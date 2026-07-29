package webasset

import (
	"net/http"

	way2goweb "github.com/cleanstartup/stack/way2go/web"
)

// WebApp is the composition root: it bridges way2go's runtime registrar
// (routes, activities, mounts) with stack's own asset Builder (CSS/JS/
// Stencil/File/DirSource). Parts bind here via Apply(*WebApp).
type WebApp struct {
	runtime *way2goweb.RouteAccumulator
	builder *Builder
}

func newWebApp() *WebApp {
	return &WebApp{
		runtime: way2goweb.NewRouteAccumulator(),
		builder: NewBuilder(),
	}
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

// Registrar exposes the runtime target so way2go-native Parts (activities,
// mounts) and their stack-level wrappers can bind to it.
func (a *WebApp) Registrar() way2goweb.Registrar {
	if a == nil {
		return nil
	}
	return a.runtime
}

// AddActivity registers a way2go route/activity with this app's runtime.
// Used by stack's ActivityDef.Apply.
func (a *WebApp) AddActivity(act way2goweb.RouteActivity) {
	if a == nil || a.runtime == nil {
		return
	}
	a.runtime.AddActivity(act)
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
	if a == nil || a.runtime == nil || handler == nil {
		return
	}
	a.runtime.AddMount(path, handler)
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

// Handler builds the runtime HTTP handler once composition is complete,
// converting the asset manifest into way2go's AssetLinks at the boundary —
// way2go never sees an AssetRef.
func (a *WebApp) Handler() *way2goweb.Registry {
	reg := a.runtime.Handler()
	reg.SetAssets(toAssetLinks(a.builder.Manifest()))
	return reg
}

func toAssetLinks(m AssetManifest) way2goweb.AssetLinks {
	var links way2goweb.AssetLinks
	for _, ref := range m.Styles {
		links.Styles = append(links.Styles, ref.URLs()...)
	}
	for _, ref := range m.Scripts {
		links.Scripts = append(links.Scripts, ref.URLs()...)
	}
	return links
}
