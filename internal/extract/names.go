package extract

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

var ignoredJSNames = map[string]struct{}{
	"arguments":                  {},
	"cancelanimationframe":       {},
	"cloneelement":               {},
	"console":                    {},
	"constructor":                {},
	"createelement":              {},
	"createpages":                {},
	"createresolvers":            {},
	"createschemacustomization":  {},
	"document":                   {},
	"exports":                    {},
	"forwardref":                 {},
	"fragment":                   {},
	"global":                     {},
	"globalthis":                 {},
	"isvalidelement":             {},
	"length":                     {},
	"module":                     {},
	"oncliententry":              {},
	"oncreatepage":               {},
	"onpostprefetchpathname":     {},
	"onprerouteupdate":           {},
	"onrouteupdate":              {},
	"pageinfo":                   {},
	"replacehydratefunction":     {},
	"replacerenderer":            {},
	"requestanimationframe":      {},
	"require":                    {},
	"setfieldsongraphqlnodetype": {},
	"setstate":                   {},
	"sourcenodes":                {},
	"suspense":                   {},
	"this":                       {},
	"undefined":                  {},
	"usecallback":                {},
	"usecontext":                 {},
	"useeffect":                  {},
	"usememo":                    {},
	"usereducer":                 {},
	"useref":                     {},
	"usestate":                   {},
	"window":                     {},
	"__narrowmap__":              {},
}

var highSignalJSFragments = []string{
	"account", "amount", "api", "auth", "billing", "body", "callback", "code",
	"content", "cookie", "coupon", "csrf", "cursor", "description",
	"download", "email", "export", "file", "filter", "format", "header",
	"id", "import", "invoice", "key", "limit", "locale", "message", "name",
	"nonce", "offset", "order", "org", "page", "param", "password", "path",
	"payload", "payment", "permission", "phone", "plan", "price", "project",
	"quantity", "query", "redirect", "refund", "request", "return", "role", "search", "secret",
	"session", "size", "slug", "sort", "state", "status", "team", "token",
	"type", "upload", "url", "uri", "user", "value", "version", "webhook",
}

var genericJSSuffixes = []string{
	"config", "context", "element", "options", "props", "provider",
	"wrapper", "wrapperprops",
}

func normalizeCandidate(value string) (string, bool) {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "\"'`")
	if value == "" || len(value) > 128 || !utf8.ValidString(value) {
		return "", false
	}

	hasNameRune := false
	for _, r := range value {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			if unicode.IsLetter(r) {
				hasNameRune = true
			}
		case strings.ContainsRune("_-$.:[]@", r):
			if r == '_' || r == '$' {
				hasNameRune = true
			}
		default:
			return "", false
		}
	}
	if !hasNameRune {
		return "", false
	}
	return value, true
}

func normalizeJSName(value string) (string, bool) {
	return normalizeJSNameWithMode(value, false)
}

func normalizeJSNameWithMode(value string, includeLowSignal bool) (string, bool) {
	value, ok := normalizeCandidate(value)
	if !ok || utf8.RuneCountInString(value) < 2 {
		return "", false
	}
	if _, ignored := ignoredJSNames[strings.ToLower(value)]; ignored {
		return "", false
	}
	if !includeLowSignal && !isUsefulJSName(value) {
		return "", false
	}
	return value, true
}

func isUsefulJSName(value string) bool {
	lower := strings.ToLower(value)

	if strings.HasPrefix(lower, "wrap") ||
		strings.HasPrefix(lower, "gatsby") ||
		strings.HasPrefix(lower, "webpack") ||
		strings.HasPrefix(lower, "component") ||
		strings.HasPrefix(lower, "layout") ||
		strings.HasPrefix(lower, "provider") ||
		strings.HasPrefix(lower, "render") ||
		strings.HasPrefix(lower, "root") ||
		strings.HasPrefix(lower, "style") ||
		strings.HasPrefix(lower, "theme") {
		return false
	}
	for _, suffix := range genericJSSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return false
		}
	}
	switch lower {
	case "data", "error", "fetch", "json", "request", "response", "result":
		return false
	}

	for _, fragment := range highSignalJSFragments {
		if fragment == "id" {
			if lower == "id" ||
				strings.HasSuffix(lower, "_id") ||
				strings.HasSuffix(lower, "-id") ||
				strings.HasSuffix(value, "Id") ||
				strings.HasSuffix(value, "ID") {
				return true
			}
			continue
		}
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
}
