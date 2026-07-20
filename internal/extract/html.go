package extract

import (
	"bytes"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

var URLAttributes = map[string]struct{}{
	"action":     {},
	"data":       {},
	"formaction": {},
	"href":       {},
	"poster":     {},
	"src":        {},
}

func HTML(data []byte, baseURL string, options Options, add func(string), warn func(error)) error {
	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return err
	}

	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if node.Type == html.ElementNode {
			attrs := make(map[string]string, len(node.Attr))
			for _, attr := range node.Attr {
				name := strings.ToLower(attr.Key)
				attrs[name] = attr.Val
				if _, ok := URLAttributes[name]; ok {
					URLsInText(attr.Val, baseURL, add)
				}
			}

			if options.IncludeLowSignal || isFormElement(node.Data) {
				add(attrs["name"])
				add(attrs["id"])
			}

			if strings.EqualFold(node.Data, "script") && attrs["src"] == "" {
				content := textContent(node)
				if strings.TrimSpace(content) != "" {
					typeName := strings.ToLower(strings.TrimSpace(attrs["type"]))
					if strings.Contains(typeName, "json") {
						if err := JSON([]byte(content), baseURL, add); err != nil && warn != nil {
							warn(err)
						}
					} else {
						JavaScript([]byte(content), baseURL, options, add, warn)
					}
				}
			}

			for name, value := range attrs {
				if strings.HasPrefix(name, "on") && strings.TrimSpace(value) != "" {
					JavaScriptInline([]byte(value), baseURL, options, add, warn)
				}
			}
		}

		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(doc)
	return nil
}

func isFormElement(name string) bool {
	switch strings.ToLower(name) {
	case "button", "fieldset", "form", "input", "object", "option", "output", "select", "textarea":
		return true
	default:
		return false
	}
}

// HTMLScriptURLs returns same-origin external script URLs referenced by an HTML document.
// Cross-origin resources stay opt-in through --input-links to avoid unexpected scope expansion.
func HTMLScriptURLs(data []byte, baseURL string) []string {
	base, err := url.Parse(baseURL)
	if err != nil || base.Host == "" {
		return nil
	}

	seen := make(map[string]struct{})
	var scripts []string
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if node.Type == html.ElementNode && strings.EqualFold(node.Data, "script") {
			for _, attr := range node.Attr {
				if !strings.EqualFold(attr.Key, "src") || strings.TrimSpace(attr.Val) == "" {
					continue
				}
				parsed, err := url.Parse(strings.TrimSpace(attr.Val))
				if err != nil {
					continue
				}
				resolved := base.ResolveReference(parsed)
				if resolved.Scheme != "http" && resolved.Scheme != "https" {
					continue
				}
				if !strings.EqualFold(resolved.Host, base.Host) {
					continue
				}
				value := resolved.String()
				if _, exists := seen[value]; !exists {
					seen[value] = struct{}{}
					scripts = append(scripts, value)
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}

	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	visit(doc)
	return scripts
}

func textContent(node *html.Node) string {
	var builder strings.Builder
	var visit func(*html.Node)
	visit = func(current *html.Node) {
		if current.Type == html.TextNode {
			builder.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(node)
	return builder.String()
}
