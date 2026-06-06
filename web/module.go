package web

import "github.com/cleanstartup/way2go/activity"

type Contributor interface {
	Apply(*WebApp)
}

type contributorFunc func(*WebApp)

func (f contributorFunc) Apply(app *WebApp) {
	if f == nil || app == nil {
		return
	}
	f(app)
}

func Compose(contribs ...Contributor) Contributor {
	return contributorFunc(func(app *WebApp) {
		for _, contrib := range contribs {
			if contrib == nil {
				continue
			}
			contrib.Apply(app)
		}
	})
}

func Module(contribs ...Contributor) Contributor { return Compose(contribs...) }

func BindActivity[P any, I any](def *activity.Definition[P, I], resolver activity.Resolver[P, I]) Contributor {
	return contributorFunc(func(app *WebApp) { AddActivity(app.builder, def, resolver) })
}

func CSS(src AssetSource) Contributor {
	return contributorFunc(func(app *WebApp) { app.RegisterCSS(src) })
}

func TailwindCSS(src AssetSource) Contributor {
	return contributorFunc(func(app *WebApp) { app.RegisterTailwindCSS(src) })
}

func JS(src AssetSource) Contributor {
	return contributorFunc(func(app *WebApp) { app.RegisterJS(src) })
}

func File(src AssetSource) Contributor {
	return contributorFunc(func(app *WebApp) { app.RegisterFile(src) })
}

func TailwindScan(paths ...string) Contributor {
	return contributorFunc(func(app *WebApp) { app.RegisterTailwindScan(paths...) })
}
