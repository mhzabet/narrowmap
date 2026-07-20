package model

import "net/http"

type Kind string

const (
	KindHTML Kind = "html"
	KindJS   Kind = "javascript"
	KindJSON Kind = "json"
)

type Document struct {
	Name         string
	BaseURL      string
	ObservedURLs []string
	ContentType  string
	Kind         Kind
	Body         []byte
	Headers      http.Header
}
