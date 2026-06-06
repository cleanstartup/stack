package templ

import (
	"context"
	"io"
	"net/http"
)

type Component interface {
	Render(context.Context, io.Writer) error
}

type componentFunc func(context.Context, io.Writer) error

func (f componentFunc) Render(ctx context.Context, w io.Writer) error {
	if f == nil {
		return nil
	}
	return f(ctx, w)
}

var NopComponent Component = componentFunc(func(context.Context, io.Writer) error { return nil })

func Handler(c Component) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c == nil {
			return
		}
		_ = c.Render(r.Context(), w)
	})
}
