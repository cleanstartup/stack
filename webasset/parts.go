package webasset

import (
	"net/http"
	"strings"
)

// Part is the composition primitive Bundle()/WebApp() operate on. It covers
// both asset contributions (bound to Builder) and runtime contributions
// (bound to the app's Registrar) — WebApp is the single root that delegates
// to whichever target a given Part actually needs.
type Part interface {
	Apply(*WebApp)
}

type Contributor = Part

type partFunc func(*WebApp)

func (f partFunc) Apply(app *WebApp) {
	if f == nil || app == nil {
		return
	}
	f(app)
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

func NPMDependency(name, version string) Part {
	return partFunc(func(app *WebApp) { app.builder.AddNPMDependency(name, version, false) })
}

func NPMDevDependency(name, version string) Part {
	return partFunc(func(app *WebApp) { app.builder.AddNPMDependency(name, version, true) })
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
