package extract

import (
	"html"
	"net/url"
	"path"
	"regexp"
	"strings"
	"unicode"

	parse "github.com/tdewolff/parse/v2"
	js "github.com/tdewolff/parse/v2/js"
)

var (
	absoluteEndpointPattern = regexp.MustCompile(`(?i)(?:https?|wss?)://[^\s"'<>\\]+`)
	fallbackStringLiteral   = regexp.MustCompile("(?s)\"(?:\\\\.|[^\"\\\\])*\"|'(?:\\\\.|[^'\\\\])*'")
)

var staticEndpointExtensions = map[string]struct{}{
	".7z": {}, ".avi": {}, ".avif": {}, ".bmp": {}, ".bz2": {}, ".css": {},
	".eot": {}, ".flac": {}, ".gif": {}, ".gz": {}, ".ico": {}, ".jpeg": {},
	".jpg": {}, ".js": {}, ".jsx": {}, ".m4a": {}, ".map": {}, ".mjs": {},
	".mov": {}, ".mp3": {}, ".mp4": {}, ".mpeg": {}, ".ogg": {}, ".otf": {},
	".pdf": {}, ".png": {}, ".rar": {}, ".svg": {}, ".tar": {}, ".tif": {},
	".tiff": {}, ".ttf": {}, ".wav": {}, ".webm": {}, ".webp": {}, ".woff": {},
	".woff2": {}, ".zip": {},
}

type endpointVisitor struct {
	baseURL string
	add     func(string)
}

func (v *endpointVisitor) Enter(node js.INode) js.IVisitor {
	switch typed := node.(type) {
	case *js.LiteralExpr:
		if typed.TokenType == js.StringToken {
			endpointsInString(literalValue(typed), v.baseURL, v.add)
		}
	case *js.TemplateExpr:
		if typed.Tag == nil {
			endpointsInString(templateEndpointValue(typed), v.baseURL, v.add)
		}
	}
	return v
}

func (v *endpointVisitor) Exit(js.INode) {}

// JavaScriptEndpoints extracts high-signal HTTP(S), WebSocket, and relative
// route candidates without executing JavaScript.
func JavaScriptEndpoints(data []byte, baseURL string, add func(string), warn func(error)) {
	if parseJavaScriptEndpoints(data, baseURL, false, add) {
		return
	}

	wrapped := make([]byte, 0, len(data)+48)
	wrapped = append(wrapped, "async function __narrowmap__(){\n"...)
	wrapped = append(wrapped, data...)
	wrapped = append(wrapped, "\n}"...)
	if parseJavaScriptEndpoints(wrapped, baseURL, false, add) {
		return
	}

	_, err := js.Parse(parse.NewInputBytes(data), js.Options{})
	if warn != nil && err != nil {
		warn(err)
	}
	fallbackJavaScriptEndpoints(data, baseURL, add)
}

func parseJavaScriptEndpoints(data []byte, baseURL string, inline bool, add func(string)) bool {
	ast, err := js.Parse(parse.NewInputBytes(data), js.Options{Inline: inline})
	if err != nil {
		return false
	}
	js.Walk(&endpointVisitor{baseURL: baseURL, add: add}, ast)
	return true
}

func fallbackJavaScriptEndpoints(data []byte, baseURL string, add func(string)) {
	text := string(data)
	for _, raw := range fallbackStringLiteral.FindAllString(text, -1) {
		literal := &js.LiteralExpr{TokenType: js.StringToken, Data: []byte(raw)}
		endpointsInString(literalValue(literal), baseURL, add)
	}
	for _, candidate := range absoluteEndpointPattern.FindAllString(text, -1) {
		addNormalizedEndpoint(candidate, baseURL, add)
	}
}

func endpointsInString(value, baseURL string, add func(string)) {
	value = decodeEndpointEscapes(html.UnescapeString(strings.TrimSpace(value)))
	if value == "" || len(value) > 4096 || strings.ContainsAny(value, "\x00\r\n\t<>") {
		return
	}

	for _, candidate := range absoluteEndpointPattern.FindAllString(value, -1) {
		candidate = strings.TrimRight(candidate, "),.;]}")
		addNormalizedEndpoint(candidate, baseURL, add)
	}

	if strings.HasPrefix(value, "//") || strings.HasPrefix(value, "/") ||
		strings.HasPrefix(value, "./") || strings.HasPrefix(value, "../") {
		addNormalizedEndpoint(value, baseURL, add)
	}
}

func addNormalizedEndpoint(candidate, baseURL string, add func(string)) {
	if normalized, ok := normalizeEndpoint(candidate, baseURL); ok {
		add(normalized)
	}
}

func normalizeEndpoint(candidate, baseURL string) (string, bool) {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" || len(candidate) > 4096 || strings.ContainsAny(candidate, " \t\r\n\x00") {
		return "", false
	}

	if strings.HasPrefix(candidate, "//") {
		scheme := "https"
		if base, err := url.Parse(baseURL); err == nil && (base.Scheme == "http" || base.Scheme == "https") {
			scheme = base.Scheme
		}
		candidate = scheme + ":" + candidate
	}

	parsed, err := url.Parse(candidate)
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

	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "ws", "wss":
	default:
		return "", false
	}
	if parsed.Host == "" || strings.EqualFold(parsed.Hostname(), "web.archive.org") {
		return "", false
	}
	parsed.User = nil
	parsed.Fragment = ""
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	if parsed.Path == "/" && parsed.RawQuery == "" {
		return "", false
	}
	if _, static := staticEndpointExtensions[strings.ToLower(path.Ext(parsed.Path))]; static {
		return "", false
	}

	if parsed.RawQuery != "" {
		query := parsed.Query()
		for name := range query {
			if strings.TrimSpace(name) == "" {
				delete(query, name)
				continue
			}
			query[name] = []string{""}
		}
		parsed.RawQuery = query.Encode()
	}

	normalized := parsed.String()
	normalized = strings.ReplaceAll(normalized, "%7B", "{")
	normalized = strings.ReplaceAll(normalized, "%7D", "}")
	return normalized, true
}

func templateEndpointValue(template *js.TemplateExpr) string {
	if template == nil || template.Tag != nil {
		return ""
	}
	var builder strings.Builder
	for _, part := range template.List {
		builder.WriteString(cleanTemplateToken(part.Value))
		builder.WriteByte('{')
		builder.WriteString(templatePlaceholder(part.Expr))
		builder.WriteByte('}')
	}
	builder.WriteString(cleanTemplateToken(template.Tail))
	return builder.String()
}

func cleanTemplateToken(value []byte) string {
	chunk := string(value)
	chunk = strings.TrimPrefix(chunk, "`")
	chunk = strings.TrimPrefix(chunk, "}")
	chunk = strings.TrimSuffix(chunk, "${")
	chunk = strings.TrimSuffix(chunk, "`")
	return decodeEndpointEscapes(chunk)
}

func templatePlaceholder(expression js.IExpr) string {
	if variable, ok := expression.(*js.Var); ok {
		name := string(variable.Name())
		if len(name) > 1 {
			for _, r := range name {
				if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
					return "param"
				}
			}
			return name
		}
	}
	return "param"
}

func decodeEndpointEscapes(value string) string {
	replacer := strings.NewReplacer(
		`\/`, `/`,
		`\u002f`, `/`, `\u002F`, `/`,
		`\x2f`, `/`, `\x2F`, `/`,
		`\u003a`, `:`, `\u003A`, `:`,
		`\x3a`, `:`, `\x3A`, `:`,
	)
	return replacer.Replace(value)
}
