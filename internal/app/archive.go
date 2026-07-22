package app

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"narrowmap/internal/extract"
)

type archiveMode uint8

const (
	archiveRobofinder archiveMode = iota + 1
	archiveOJS
)

type archiveResponse struct {
	body         []byte
	status       int
	contentType  string
	truncated    bool
	retryAfter   time.Duration
	requestedURL string
}

type archiveBackend interface {
	Inventory(context.Context, string, bool) ([]string, error)
	Versions(context.Context, string) ([]string, error)
	Fetch(context.Context, string, int64) (archiveResponse, error)
	Close()
}

type defaultArchiveBackend struct {
	binary  string
	timeout time.Duration
	client  *http.Client
	close   func()
}

var replayModifierPattern = regexp.MustCompile(`/web/(\d+)(?:if_|id_)/`)

func runArchiveFeature(
	ctx context.Context,
	cfg config,
	stdin io.Reader,
	findings *collector,
	log *progress,
	warn func(string, error),
) error {
	mode := archiveRobofinder
	input := cfg.robofinder
	if cfg.ojs != "" {
		mode = archiveOJS
		input = cfg.ojs
	}
	targets, err := loadArchiveTargets(input, stdin)
	if err != nil {
		return err
	}

	backend, err := newDefaultArchiveBackend(cfg)
	if err != nil {
		return err
	}
	defer backend.Close()
	return runArchiveWorkflow(ctx, cfg, mode, targets, backend, findings, log, warn)
}

func runArchiveWorkflow(
	ctx context.Context,
	cfg config,
	mode archiveMode,
	targets []string,
	backend archiveBackend,
	findings *collector,
	log *progress,
	warn func(string, error),
) error {
	limiter := &startLimiter{delay: cfg.archiveDelay}
	inventorySet := make(map[string]struct{})
	for index, target := range targets {
		log.Stage("waybackurls inventory %d/%d: %s", index+1, len(targets), target)
		values, err := archiveURLsWithRetry(ctx, limiter, cfg, func(callCtx context.Context) ([]string, error) {
			return backend.Inventory(callCtx, target, cfg.archiveNoSubs)
		})
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			warn(target, fmt.Errorf("waybackurls inventory: %w", err))
			continue
		}
		if len(values) == 0 {
			warn(target, errors.New("waybackurls returned no URLs; the archive may be empty or temporarily unavailable"))
		}
		for _, value := range values {
			if normalized, ok := normalizeArchivedOriginal(value); ok {
				inventorySet[normalized] = struct{}{}
			}
		}
	}

	inventory := sortedSet(inventorySet)
	log.Stage("waybackurls returned %d unique archived URLs", len(inventory))

	var originals []string
	switch mode {
	case archiveRobofinder:
		originals = robotsCandidates(targets, inventory)
		log.Stage("found %d robots.txt histories to inspect", len(originals))
	case archiveOJS:
		originals = javascriptCandidates(inventory)
		log.Stage("found %d archived JavaScript assets to inspect", len(originals))
	default:
		return errors.New("unknown archive mode")
	}

	missingVersions := 0
	fetched := 0
	for index, original := range originals {
		if err := ctx.Err(); err != nil {
			return err
		}
		label := "robots.txt"
		if mode == archiveOJS {
			label = "JavaScript"
		}
		log.Stage("archived %s %d/%d: %s", label, index+1, len(originals), original)

		versions, err := archiveURLsWithRetry(ctx, limiter, cfg, func(callCtx context.Context) ([]string, error) {
			return backend.Versions(callCtx, original)
		})
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			warn(original, fmt.Errorf("waybackurls -get-versions: %w", err))
			continue
		}
		versions = selectArchiveVersions(versions, cfg.archiveMaxVersions)
		if len(versions) == 0 {
			missingVersions++
			continue
		}

		for _, replayURL := range versions {
			response, err := archiveFetchWithRetry(ctx, limiter, cfg, backend, replayURL)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				warn(replayURL, err)
				continue
			}
			if response.status < http.StatusOK || response.status >= http.StatusMultipleChoices {
				warn(replayURL, fmt.Errorf("archive replay returned HTTP status %d", response.status))
				continue
			}
			if response.truncated {
				warn(replayURL, fmt.Errorf("archived response truncated at %d bytes", cfg.maxBody))
			}
			if looksLikeHTML(response.contentType, response.body) {
				warn(replayURL, errors.New("archive replay returned HTML instead of raw archived content"))
				continue
			}

			fetched++
			switch mode {
			case archiveRobofinder:
				extract.RobotsEndpoints(response.body, original, findings.Add)
			case archiveOJS:
				extract.JavaScriptEndpoints(response.body, original, findings.Add, func(parseErr error) {
					warn(replayURL, fmt.Errorf("JavaScript parse: %w", parseErr))
				})
			}
		}
	}

	if missingVersions > 0 {
		log.Stage("%d archived files had no Wayback versions", missingVersions)
	}
	log.Stage("analyzed %d archived response contexts without saving files", fetched)
	return nil
}

func newDefaultArchiveBackend(cfg config) (*defaultArchiveBackend, error) {
	binary, err := exec.LookPath(cfg.waybackBin)
	if err != nil {
		return nil, fmt.Errorf(
			"tomnomnom/waybackurls was not found (%s); install it with: go install github.com/tomnomnom/waybackurls@latest",
			cfg.waybackBin,
		)
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: cfg.archiveTimeout, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          2,
		MaxIdleConnsPerHost:   1,
		MaxConnsPerHost:       1,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   cfg.archiveTimeout,
		ResponseHeaderTimeout: cfg.archiveTimeout,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   cfg.archiveTimeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("stopped after 10 archive redirects")
			}
			if !isWaybackURL(request.URL) {
				return fmt.Errorf("archive replay attempted to redirect outside web.archive.org to %s", request.URL.Redacted())
			}
			return nil
		},
	}
	return &defaultArchiveBackend{
		binary:  binary,
		timeout: cfg.archiveTimeout,
		client:  client,
		close:   transport.CloseIdleConnections,
	}, nil
}

func (backend *defaultArchiveBackend) Inventory(ctx context.Context, target string, noSubs bool) ([]string, error) {
	args := make([]string, 0, 2)
	if noSubs {
		args = append(args, "-no-subs")
	}
	args = append(args, target)
	return backend.runWayback(ctx, args...)
}

func (backend *defaultArchiveBackend) Versions(ctx context.Context, original string) ([]string, error) {
	return backend.runWayback(ctx, "-get-versions", original)
}

func (backend *defaultArchiveBackend) runWayback(ctx context.Context, args ...string) ([]string, error) {
	callCtx, cancel := context.WithTimeout(ctx, backend.timeout)
	defer cancel()

	command := exec.CommandContext(callCtx, backend.binary, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		if errors.Is(callCtx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("timed out after %s", backend.timeout)
		}
		message := strings.TrimSpace(stderr.String())
		if message != "" {
			return nil, fmt.Errorf("%w: %s", err, message)
		}
		return nil, err
	}
	return parseWaybackOutput(stdout.Bytes())
}

func (backend *defaultArchiveBackend) Fetch(ctx context.Context, replayURL string, maxBody int64) (archiveResponse, error) {
	replayURL = rawReplayURL(replayURL)
	parsed, err := url.Parse(replayURL)
	if err != nil || !isWaybackURL(parsed) {
		return archiveResponse{}, fmt.Errorf("refusing non-Wayback replay URL %q", replayURL)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, replayURL, nil)
	if err != nil {
		return archiveResponse{}, err
	}
	request.Header.Set("User-Agent", "narrowmap/"+version)
	request.Header.Set("Accept", "*/*")

	response, err := backend.client.Do(request)
	if err != nil {
		return archiveResponse{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBody+1))
	if err != nil {
		return archiveResponse{}, err
	}
	truncated := int64(len(body)) > maxBody
	if truncated {
		body = body[:maxBody]
	}
	return archiveResponse{
		body:         body,
		status:       response.StatusCode,
		contentType:  response.Header.Get("Content-Type"),
		truncated:    truncated,
		retryAfter:   parseRetryAfter(response.Header.Get("Retry-After")),
		requestedURL: replayURL,
	}, nil
}

func (backend *defaultArchiveBackend) Close() {
	if backend.close != nil {
		backend.close()
	}
}

func loadArchiveTargets(value string, stdin io.Reader) ([]string, error) {
	if value != "-" {
		target, err := normalizeArchiveTarget(value)
		if err != nil {
			return nil, err
		}
		return []string{target}, nil
	}

	seen := make(map[string]struct{})
	var targets []string
	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		target, err := normalizeArchiveTarget(line)
		if err != nil {
			return nil, fmt.Errorf("invalid archive target %q: %w", line, err)
		}
		if _, exists := seen[target]; exists {
			continue
		}
		seen[target] = struct{}{}
		targets = append(targets, target)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, errors.New("stdin contains no archive targets")
	}
	sort.Strings(targets)
	return targets, nil
}

func normalizeArchiveTarget(value string) (string, error) {
	value = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(value), "*."))
	normalized, err := normalizeHTTPURL(value)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(normalized)
	if err != nil || parsed.Hostname() == "" {
		return "", fmt.Errorf("invalid target %q", value)
	}
	return strings.ToLower(parsed.Hostname()), nil
}

func parseWaybackOutput(data []byte) ([]string, error) {
	seen := make(map[string]struct{})
	var values []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		candidate := fields[len(fields)-1]
		parsed, err := url.Parse(candidate)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			continue
		}
		parsed.Fragment = ""
		candidate = parsed.String()
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		values = append(values, candidate)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.Strings(values)
	return values, nil
}

func normalizeArchivedOriginal(value string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", false
	}
	parsed.User = nil
	parsed.Fragment = ""
	return parsed.String(), true
}

func robotsCandidates(targets, inventory []string) []string {
	set := make(map[string]struct{})
	for _, target := range targets {
		set["http://"+target+"/robots.txt"] = struct{}{}
		set["https://"+target+"/robots.txt"] = struct{}{}
	}
	for _, value := range inventory {
		parsed, err := url.Parse(value)
		if err == nil && strings.EqualFold(path.Base(parsed.Path), "robots.txt") {
			set[value] = struct{}{}
		}
	}
	return sortedSet(set)
}

func javascriptCandidates(inventory []string) []string {
	set := make(map[string]struct{})
	for _, value := range inventory {
		parsed, err := url.Parse(value)
		if err != nil {
			continue
		}
		switch strings.ToLower(path.Ext(parsed.Path)) {
		case ".js", ".mjs", ".cjs", ".jsx":
			set[value] = struct{}{}
		}
	}
	return sortedSet(set)
}

func selectArchiveVersions(values []string, maximum int) []string {
	set := make(map[string]struct{})
	for _, value := range values {
		value = rawReplayURL(value)
		parsed, err := url.Parse(value)
		if err != nil || !isWaybackURL(parsed) {
			continue
		}
		set[value] = struct{}{}
	}
	ordered := sortedSet(set)
	if maximum <= 0 || len(ordered) <= maximum {
		return ordered
	}
	if maximum == 1 {
		return []string{ordered[len(ordered)-1]}
	}

	selected := make([]string, 0, maximum)
	for index := 0; index < maximum; index++ {
		position := index * (len(ordered) - 1) / (maximum - 1)
		selected = append(selected, ordered[position])
	}
	return selected
}

func rawReplayURL(value string) string {
	return replayModifierPattern.ReplaceAllString(value, `/web/${1}id_/`)
}

func isWaybackURL(parsed *url.URL) bool {
	return parsed != nil && (parsed.Scheme == "https" || parsed.Scheme == "http") &&
		strings.EqualFold(parsed.Hostname(), "web.archive.org")
}

func archiveURLsWithRetry(
	ctx context.Context,
	limiter *startLimiter,
	cfg config,
	operation func(context.Context) ([]string, error),
) ([]string, error) {
	var lastErr error
	for attempt := 0; attempt <= cfg.archiveRetries; attempt++ {
		if err := limiter.Wait(ctx); err != nil {
			return nil, err
		}
		values, err := operation(ctx)
		if err == nil {
			return values, nil
		}
		lastErr = err
		if attempt == cfg.archiveRetries {
			break
		}
		if err := waitArchiveBackoff(ctx, cfg.archiveDelay, attempt, 0); err != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

func archiveFetchWithRetry(
	ctx context.Context,
	limiter *startLimiter,
	cfg config,
	backend archiveBackend,
	replayURL string,
) (archiveResponse, error) {
	var lastErr error
	for attempt := 0; attempt <= cfg.archiveRetries; attempt++ {
		if err := limiter.Wait(ctx); err != nil {
			return archiveResponse{}, err
		}
		response, err := backend.Fetch(ctx, rawReplayURL(replayURL), cfg.maxBody)
		if err == nil && !retryableArchiveStatus(response.status) {
			return response, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("archive replay returned retryable HTTP status %d", response.status)
		}
		if attempt == cfg.archiveRetries {
			break
		}
		if err := waitArchiveBackoff(ctx, cfg.archiveDelay, attempt, response.retryAfter); err != nil {
			return archiveResponse{}, err
		}
	}
	return archiveResponse{}, lastErr
}

func retryableArchiveStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func waitArchiveBackoff(ctx context.Context, base time.Duration, attempt int, retryAfter time.Duration) error {
	wait := base
	for count := 0; count < attempt && wait < 30*time.Second; count++ {
		wait *= 2
	}
	if retryAfter > wait {
		wait = retryAfter
	}
	if wait > time.Minute {
		wait = time.Minute
	}
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

func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if deadline, err := http.ParseTime(value); err == nil {
		if wait := time.Until(deadline); wait > 0 {
			return wait
		}
	}
	return 0
}

func looksLikeHTML(contentType string, body []byte) bool {
	prefix := strings.ToLower(strings.TrimSpace(string(body[:min(len(body), 512)])))
	if strings.HasPrefix(prefix, "<!doctype html") || strings.HasPrefix(prefix, "<html") {
		return true
	}
	return strings.Contains(strings.ToLower(contentType), "text/html") && strings.Contains(prefix, "<html")
}

func sortedSet(set map[string]struct{}) []string {
	values := make([]string, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}
