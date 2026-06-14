package web

import (
	"net/http"
)

type WebApp struct {
	builder *Builder
}

func newWebApp() *WebApp {
	builder := NewBuilder()
	app := &WebApp{builder: builder}
	return app
}

func NewApp(parts ...Part) *WebApp {
	app := newWebApp()
	app.Apply(parts...)
	return app
}

func (a *WebApp) Apply(parts ...Part) {
	if a == nil {
		return
	}
	for _, part := range parts {
		if part == nil {
			continue
		}
		part.Apply(a)
	}
}

func (a *WebApp) Builder() *Builder {
	if a == nil {
		return nil
	}
	return a.builder
}

func (a *WebApp) RegisterCSS(src AssetSource) AssetRef {
	return a.builder.CSS(src)
}

func (a *WebApp) RegisterTailwindCSS(src AssetSource) AssetRef {
	return a.builder.TailwindCSS(src)
}

func (a *WebApp) RegisterStencil(src AssetSource) AssetRef {
	return a.builder.Stencil(src)
}

func (a *WebApp) RegisterJS(src AssetSource) AssetRef {
	return a.builder.JS(src)
}

func (a *WebApp) RegisterFile(src AssetSource) AssetRef {
	return a.builder.File(src)
}

func (a *WebApp) Mount(path string, handler http.Handler) {
	if a == nil || a.builder == nil || handler == nil {
		return
	}
	a.builder.AddMount(path, handler)
}

func (a *WebApp) RegisterDirSource(namespace, relPath, absPath string) {
	if a == nil || a.builder == nil {
		return
	}
	a.builder.RegisterDirSource(namespace, relPath, absPath)
}

func (a *WebApp) RegisterTailwindScan(paths ...string) {
	a.builder.TailwindScan(paths...)
}

func (a *WebApp) RegisterStencilScan(paths ...string) {
	a.builder.StencilScan(paths...)
}
