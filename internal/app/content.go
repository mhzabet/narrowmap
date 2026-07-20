package app

import (
	"bytes"
	"fmt"
	"mime"
	"path/filepath"
	"strings"

	"narrowmap/internal/model"
)

func detectKind(name, contentType string, body []byte) (model.Kind, error) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".html", ".htm", ".xhtml":
		return model.KindHTML, nil
	case ".js", ".mjs", ".cjs", ".jsx":
		return model.KindJS, nil
	case ".json":
		return model.KindJSON, nil
	}

	if mediaType, _, err := mime.ParseMediaType(contentType); err == nil {
		mediaType = strings.ToLower(mediaType)
		switch {
		case mediaType == "text/html", mediaType == "application/xhtml+xml":
			return model.KindHTML, nil
		case strings.Contains(mediaType, "javascript"), mediaType == "application/ecmascript":
			return model.KindJS, nil
		case mediaType == "application/json", strings.HasSuffix(mediaType, "+json"):
			return model.KindJSON, nil
		}
	}

	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return "", fmt.Errorf("empty content")
	}
	if trimmed[0] == '<' {
		return model.KindHTML, nil
	}
	if trimmed[0] == '{' || trimmed[0] == '[' {
		return model.KindJSON, nil
	}
	return model.KindJS, nil
}

func supportedFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".html", ".htm", ".xhtml", ".js", ".mjs", ".cjs", ".jsx", ".json":
		return true
	default:
		return false
	}
}
