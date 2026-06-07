package web

import "github.com/cleanstartup/stack/activity"

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

func Module(parts ...Part) Part {
	return partFunc(func(app *WebApp) {
		for _, part := range parts {
			if part == nil {
				continue
			}
			part.Apply(app)
		}
	})
}

func Compose(parts ...Part) Part { return Module(parts...) }

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
