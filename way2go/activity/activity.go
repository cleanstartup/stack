package activity

import (
	"net/http"
	"strings"
)

type Result = any

type Context interface {
	Request() *http.Request
	ResponseWriter() http.ResponseWriter
	RedirectToURI(uri string)
	RedirectToURIWithStatus(uri string, statusCode int)
	Error(err error) Result
}

type NoInput struct{}

func PathFromID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		panic("activity id must not be empty")
	}
	if strings.EqualFold(id, "root") {
		return "/"
	}
	parts := strings.Split(id, ".")
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		clean = append(clean, part)
	}
	if len(clean) == 0 {
		panic("activity id must not be empty")
	}
	return "/" + strings.Join(clean, "/")
}
