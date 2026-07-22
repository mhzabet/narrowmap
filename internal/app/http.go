package app

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"narrowmap/internal/model"
)

type fetchResult struct {
	document  model.Document
	status    int
	truncated bool
	err       error
}

type startLimiter struct {
	mu    sync.Mutex
	next  time.Time
	delay time.Duration
}

func (l *startLimiter) Wait(ctx context.Context) error {
	if l.delay <= 0 {
		return nil
	}

	l.mu.Lock()
	now := time.Now()
	start := now
	if l.next.After(now) {
		start = l.next
	}
	l.next = start.Add(l.delay)
	l.mu.Unlock()

	wait := time.Until(start)
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func loadLinks(path string, stdin io.Reader) ([]string, error) {
	if path == "-" {
		return loadLinksReader(stdin, "stdin")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return loadLinksReader(file, path)
}

func loadURLInput(value string, stdin io.Reader) ([]string, error) {
	if value == "-" {
		return loadLinksReader(stdin, "stdin")
	}
	normalized, err := normalizeHTTPURL(value)
	if err != nil {
		return nil, err
	}
	return []string{normalized}, nil
}

func loadLinksReader(reader io.Reader, source string) ([]string, error) {
	seen := make(map[string]struct{})
	var links []string
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		normalized, err := normalizeHTTPURL(line)
		if err != nil {
			return nil, fmt.Errorf("invalid HTTP(S) URL in %s: %q", source, line)
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		links = append(links, normalized)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(links) == 0 {
		return nil, fmt.Errorf("%s contains no HTTP(S) URLs", source)
	}
	return links, nil
}

func normalizeHTTPURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	switch {
	case strings.HasPrefix(value, "//"):
		value = "https:" + value
	case !strings.Contains(value, "://"):
		value = "https://" + value
	}

	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", fmt.Errorf("invalid HTTP(S) URL: %q", value)
	}
	return parsed.String(), nil
}

func parseHeaders(values []string) (http.Header, error) {
	headers := make(http.Header)
	for _, value := range values {
		name, headerValue, ok := strings.Cut(value, ":")
		if !ok {
			return nil, fmt.Errorf("invalid header %q", value)
		}
		name = textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(name))
		headerValue = strings.TrimSpace(headerValue)
		if name == "" || headerValue == "" {
			return nil, fmt.Errorf("invalid header %q", value)
		}
		headers.Add(name, headerValue)
	}
	return headers, nil
}

func fetchLinks(
	ctx context.Context,
	links []string,
	headers http.Header,
	concurrency int,
	delay time.Duration,
	timeout time.Duration,
	maxBody int64,
) <-chan fetchResult {
	jobs := make(chan string)
	results := make(chan fetchResult)
	limiter := &startLimiter{delay: delay}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          concurrency * 2,
		MaxIdleConnsPerHost:   concurrency,
		MaxConnsPerHost:       concurrency,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}

	var workers sync.WaitGroup
	for range concurrency {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for link := range jobs {
				if err := limiter.Wait(ctx); err != nil {
					results <- fetchResult{err: err}
					continue
				}
				results <- fetchOne(ctx, client, link, headers, maxBody)
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, link := range links {
			select {
			case <-ctx.Done():
				return
			case jobs <- link:
			}
		}
	}()

	go func() {
		workers.Wait()
		close(results)
		transport.CloseIdleConnections()
	}()

	return results
}

func fetchOne(ctx context.Context, client *http.Client, link string, headers http.Header, maxBody int64) fetchResult {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return fetchResult{err: err}
	}
	request.Header.Set("User-Agent", "narrowmap/"+version)
	for name, values := range headers {
		for _, value := range values {
			request.Header.Add(name, value)
		}
	}

	response, err := client.Do(request)
	if err != nil {
		return fetchResult{err: fmt.Errorf("%s: %w", link, err)}
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, maxBody+1))
	if err != nil {
		return fetchResult{err: fmt.Errorf("%s: %w", link, err)}
	}
	truncated := int64(len(body)) > maxBody
	if truncated {
		body = body[:maxBody]
	}

	finalURL := response.Request.URL.String()
	kind, err := detectKind(finalURL, response.Header.Get("Content-Type"), body)
	if err != nil {
		return fetchResult{status: response.StatusCode, err: fmt.Errorf("%s: %w", link, err)}
	}

	return fetchResult{
		status:    response.StatusCode,
		truncated: truncated,
		document: model.Document{
			Name:         link,
			BaseURL:      finalURL,
			ObservedURLs: []string{link, finalURL},
			ContentType:  response.Header.Get("Content-Type"),
			Kind:         kind,
			Body:         body,
			Headers:      response.Header.Clone(),
		},
	}
}
