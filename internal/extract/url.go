package extract

import (
	"net/url"
	"regexp"
	"strings"
)

var embeddedURLPattern = regexp.MustCompile(`(?i)(?:https?://|//|/)[^\s"'<>]+\?[^\s"'<>]+`)

func URLParameters(rawURL, baseURL string, add func(string)) {
	rawURL = strings.TrimSpace(strings.Trim(rawURL, "<>\"'"))
	if rawURL == "" {
		return
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return
	}
	if baseURL != "" && !parsed.IsAbs() {
		if base, baseErr := url.Parse(baseURL); baseErr == nil {
			parsed = base.ResolveReference(parsed)
		}
	}
	for name := range parsed.Query() {
		add(name)
	}
}

func URLsInText(text, baseURL string, add func(string)) {
	URLParameters(text, baseURL, add)
	for _, candidate := range embeddedURLPattern.FindAllString(text, -1) {
		URLParameters(strings.TrimRight(candidate, "),.;]"), baseURL, add)
	}
}
