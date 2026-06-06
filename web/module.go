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

type LegacyModule interface {
	register(*Registry)
}

type legacyModuleFunc func(*Registry)

func (f legacyModuleFunc) register(r *Registry) {
	if f == nil || r == nil {
		return
	}
	f(r)
}

type legacyGroup struct {
	path    string
	modules []LegacyModule
}

func (g *legacyGroup) register(r *Registry) {
	if g == nil || r == nil {
		return
	}
	target := r
	if g.path != "" {
		target = r.Group(g.path)
	}
	for _, module := range g.modules {
		if module == nil {
			continue
		}
		module.register(target)
	}
}

func Bind(path string, modules ...LegacyModule) LegacyModule {
	return &legacyGroup{path: path, modules: modules}
}

func Compose(modules ...LegacyModule) LegacyModule { return Bind("", modules...) }

func Module(modules ...LegacyModule) LegacyModule { return Compose(modules...) }

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
