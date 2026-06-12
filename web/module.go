package web

import (
	"net/http"
	"strings"

	"github.com/cleanstartup/stack/activity"
)

type Part interface {
	Apply(*WebApp)
}

type Contributor = Part

type Bundle interface {
	Part
}

type partFunc func(*WebApp)

func (f partFunc) Apply(app *WebApp) {
	if f == nil || app == nil {
		return
	}
	f(app)
}

type bundle struct {
	parts []Part
}

func Module(parts ...Part) Bundle {
	return &bundle{parts: cloneParts(parts)}
}

func (b *bundle) Apply(app *WebApp) {
	if app == nil {
		return
	}
	for _, part := range b.parts {
		if part == nil {
			continue
		}
		part.Apply(app)
	}
}

func cloneParts(parts []Part) []Part {
	if len(parts) == 0 {
		return nil
	}
	out := make([]Part, 0, len(parts))
	out = append(out, parts...)
	return out
}

func Compose(parts ...Part) Part {
	return partFunc(func(app *WebApp) {
		for _, part := range parts {
			if part == nil {
				continue
			}
			part.Apply(app)
		}
	})
}

func BindActivity[P any, I any](def *activity.Definition[P, I], resolver activity.Resolver[P, I]) Part {
	return partFunc(func(app *WebApp) { AddActivity(app.builder, def, resolver) })
}

func CSS(src AssetSource) Part {
	return partFunc(func(app *WebApp) { app.RegisterCSS(src) })
}

func TailwindCSS(src AssetSource) Part {
	return partFunc(func(app *WebApp) { app.RegisterTailwindCSS(src) })
}

func Stencil(src AssetSource) Part {
	return partFunc(func(app *WebApp) { app.RegisterStencil(src) })
}

func JS(src AssetSource) Part {
	return partFunc(func(app *WebApp) { app.RegisterJS(src) })
}

func File(src AssetSource) Part {
	return partFunc(func(app *WebApp) { app.RegisterFile(src) })
}

func TailwindScan(paths ...string) Part {
	return partFunc(func(app *WebApp) { app.RegisterTailwindScan(paths...) })
}

func StencilScan(paths ...string) Part {
	return partFunc(func(app *WebApp) { app.RegisterStencilScan(paths...) })
}

func Mount(path string, handler http.Handler) Part {
	return partFunc(func(app *WebApp) {
		if app == nil || handler == nil {
			return
		}
		if strings.TrimSpace(path) == "" {
			path = "/"
		}
		app.Mount(path, handler)
	})
}
