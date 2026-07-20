package extract

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	parse "github.com/tdewolff/parse/v2"
	js "github.com/tdewolff/parse/v2/js"
)

var (
	fallbackDeclaration = regexp.MustCompile(`(?m)\b(?:var|let|const)\s+([A-Za-z_$][A-Za-z0-9_$]*)`)
	fallbackObjectKey   = regexp.MustCompile(`(?:\{|,)\s*["']?([A-Za-z_$][A-Za-z0-9_$.-]*)["']?\s*:`)
)

type jsVisitor struct {
	baseURL    string
	options    Options
	add        func(string)
	classNames map[*js.Var]struct{}
}

func (v *jsVisitor) Enter(node js.INode) js.IVisitor {
	switch typed := node.(type) {
	case *js.ClassDecl:
		if typed.Name != nil {
			v.classNames[typed.Name] = struct{}{}
		}
	case *js.Var:
		if _, isClassName := v.classNames[typed]; isClassName {
			break
		}
		switch typed.Decl {
		case js.VariableDecl, js.LexicalDecl, js.ArgumentDecl, js.CatchDecl:
			v.addVariable(string(typed.Name()))
		}
	case *js.ObjectExpr:
		for _, property := range typed.List {
			if property.Name != nil && !isFunctionExpression(property.Value) {
				v.addProperty(property.Name)
			}
		}
	case *js.BindingObject:
		for _, item := range typed.List {
			if item.Key != nil {
				v.addProperty(item.Key)
			}
			v.addBinding(item.Value.Binding)
		}
		if typed.Rest != nil {
			v.addVariable(string(typed.Rest.Name()))
		}
	case *js.LiteralExpr:
		if typed.TokenType == js.StringToken {
			URLsInText(literalValue(typed), v.baseURL, v.add)
		}
	}
	return v
}

func newJSVisitor(baseURL string, options Options, add func(string)) *jsVisitor {
	return &jsVisitor{
		baseURL:    baseURL,
		options:    options,
		add:        add,
		classNames: make(map[*js.Var]struct{}),
	}
}

func (v *jsVisitor) addBinding(binding js.IBinding) {
	switch typed := binding.(type) {
	case *js.Var:
		v.addVariable(string(typed.Name()))
	case *js.BindingObject:
		for _, item := range typed.List {
			if item.Key != nil {
				v.addProperty(item.Key)
			}
			v.addBinding(item.Value.Binding)
		}
		if typed.Rest != nil {
			v.addVariable(string(typed.Rest.Name()))
		}
	case *js.BindingArray:
		for _, item := range typed.List {
			v.addBinding(item.Binding)
		}
		v.addBinding(typed.Rest)
	}
}

func (v *jsVisitor) Exit(js.INode) {}

func (v *jsVisitor) addVariable(value string) {
	if value, ok := normalizeJSVariableNameWithMode(value, v.options.IncludeLowSignal); ok {
		v.add(value)
	}
}

func (v *jsVisitor) addProperty(name *js.PropertyName) {
	if name == nil || name.IsComputed() {
		return
	}
	if name.Literal.TokenType == js.StringToken {
		v.addObjectKey(literalValue(&name.Literal))
		return
	}
	v.addObjectKey(string(name.Literal.Data))
}

func (v *jsVisitor) addObjectKey(value string) {
	if value, ok := normalizeJSObjectKeyWithMode(value, v.options.IncludeLowSignal); ok {
		v.add(value)
	}
}

func JavaScript(data []byte, baseURL string, options Options, add func(string), warn func(error)) {
	parseJavaScript(data, baseURL, false, options, add, warn)
}

func JavaScriptInline(data []byte, baseURL string, options Options, add func(string), warn func(error)) {
	parseJavaScript(data, baseURL, true, options, add, warn)
}

func parseJavaScript(data []byte, baseURL string, inline bool, options Options, add func(string), warn func(error)) {
	ast, err := js.Parse(parse.NewInputBytes(data), js.Options{Inline: inline})
	if err == nil {
		js.Walk(newJSVisitor(baseURL, options, add), ast)
		return
	}
	if !inline {
		wrapped := make([]byte, 0, len(data)+48)
		wrapped = append(wrapped, "async function __narrowmap__(){\n"...)
		wrapped = append(wrapped, data...)
		wrapped = append(wrapped, "\n}"...)
		if wrappedAST, wrappedErr := js.Parse(parse.NewInputBytes(wrapped), js.Options{}); wrappedErr == nil {
			js.Walk(newJSVisitor(baseURL, options, add), wrappedAST)
			return
		}
	}
	if warn != nil {
		warn(err)
	}
	fallbackJavaScript(data, baseURL, options, add)
}

func fallbackJavaScript(data []byte, baseURL string, options Options, add func(string)) {
	text := string(data)
	for _, match := range fallbackDeclaration.FindAllStringSubmatch(text, -1) {
		if value, ok := normalizeJSVariableNameWithMode(match[1], options.IncludeLowSignal); ok {
			add(value)
		}
	}
	for _, match := range fallbackObjectKey.FindAllStringSubmatch(text, -1) {
		if value, ok := normalizeJSObjectKeyWithMode(match[1], options.IncludeLowSignal); ok {
			add(value)
		}
	}
	URLsInText(text, baseURL, add)
}

func isFunctionExpression(expression js.IExpr) bool {
	switch expression.(type) {
	case *js.FuncDecl, *js.ArrowFunc:
		return true
	default:
		return false
	}
}

func literalValue(literal *js.LiteralExpr) string {
	if literal == nil {
		return ""
	}
	if literal.TokenType != js.StringToken {
		return string(literal.Data)
	}

	raw := string(literal.Data)
	if len(raw) >= 2 {
		if value, err := strconv.Unquote(raw); err == nil {
			return value
		}
		if raw[0] == '\'' && raw[len(raw)-1] == '\'' {
			if value, err := strconv.Unquote("\"" + strings.ReplaceAll(raw[1:len(raw)-1], "\"", `\"`) + "\""); err == nil {
				return value
			}
		}
		var value string
		if json.Unmarshal([]byte(raw), &value) == nil {
			return value
		}
	}
	return strings.Trim(raw, "\"'")
}
