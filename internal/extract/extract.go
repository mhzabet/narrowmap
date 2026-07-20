package extract

import (
	"fmt"
	"strings"

	"narrowmap/internal/model"
)

type Options struct {
	IncludeLowSignal bool
}

func Document(doc model.Document, options Options, add func(string), warn func(error)) error {
	filteredAdd := func(candidate string) {
		if normalized, ok := normalizeCandidate(candidate); ok {
			add(normalized)
		}
	}

	for _, observedURL := range doc.ObservedURLs {
		URLParameters(observedURL, doc.BaseURL, filteredAdd)
	}
	extractResponseHeaders(doc, filteredAdd)

	switch doc.Kind {
	case model.KindHTML:
		return HTML(doc.Body, doc.BaseURL, options, filteredAdd, warn)
	case model.KindJS:
		JavaScript(doc.Body, doc.BaseURL, options, filteredAdd, warn)
		return nil
	case model.KindJSON:
		return JSON(doc.Body, doc.BaseURL, filteredAdd)
	default:
		return fmt.Errorf("unsupported content type for %s", doc.Name)
	}
}

func extractResponseHeaders(doc model.Document, add func(string)) {
	for _, header := range []string{"Location", "Content-Location", "Link", "Refresh"} {
		for _, value := range doc.Headers.Values(header) {
			URLsInText(value, doc.BaseURL, add)
		}
	}
	for _, cookie := range doc.Headers.Values("Set-Cookie") {
		pair := strings.TrimSpace(strings.SplitN(cookie, ";", 2)[0])
		if name, _, ok := strings.Cut(pair, "="); ok {
			add(name)
		}
	}
}
