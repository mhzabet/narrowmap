package extract

import (
	"bufio"
	"bytes"
	"net/url"
	"strings"
)

// RobotsEndpoints extracts historical Allow, Disallow, Noindex, Sitemap, and
// Clean-param paths as absolute endpoints rooted at the original robots URL.
func RobotsEndpoints(data []byte, baseURL string, add func(string)) {
	scanner := bufio.NewScanner(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if comment := strings.IndexByte(line, '#'); comment >= 0 {
			line = strings.TrimSpace(line[:comment])
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		name = strings.ToLower(strings.TrimSpace(name))
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}

		switch name {
		case "allow", "disallow", "noindex", "sitemap":
			if endpoint, ok := normalizeRobotsEndpoint(value, baseURL); ok {
				add(endpoint)
			}
		case "clean-param":
			fields := strings.Fields(value)
			if len(fields) > 1 {
				if endpoint, ok := normalizeRobotsEndpoint(fields[len(fields)-1], baseURL); ok {
					add(endpoint)
				}
			}
		}
	}
}

func normalizeRobotsEndpoint(value, baseURL string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 4096 || strings.ContainsAny(value, " \t\r\n\x00") {
		return "", false
	}
	if !strings.Contains(value, "://") && !strings.HasPrefix(value, "/") {
		value = "/" + value
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return "", false
	}
	if !parsed.IsAbs() {
		base, baseErr := url.Parse(baseURL)
		if baseErr != nil || base.Scheme == "" || base.Host == "" {
			return "", false
		}
		parsed = base.ResolveReference(parsed)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Host == "" {
		return "", false
	}
	parsed.User = nil
	parsed.Fragment = ""
	if parsed.Path == "" {
		return "", false
	}
	return parsed.String(), true
}
