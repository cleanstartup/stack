package hugo

import (
	"net/http"
	"strings"

	"github.com/cleanstartup/stack/activity"
	hugosupportpkg "github.com/cleanstartup/stack/hugosupport"
	"github.com/cleanstartup/stack/web"
)

const defaultAddr = ":8080"

type BuildConfig = web.BuildConfig
type BuildResult = web.BuildResult
type DevConfig = web.DevConfig
type ServeConfig = web.ServeConfig
type Workspace = web.Workspace
type AssetSource = web.AssetSource
type AssetRef = web.AssetRef
type ContentRegistry = hugosupportpkg.ContentRegistry
type LayoutRegistry = hugosupportpkg.LayoutRegistry
type HugoModule = hugosupportpkg.Module

var NewDevState = web.NewDevState
var NewContentRegistry = hugosupportpkg.NewContentRegistry
var NewLayoutRegistry = hugosupportpkg.NewLayoutRegistry

type ActivityOption[C any] = web.ActivityOption[C]
type WebActivity[C any] = web.WebActivity[C]

type SiteOptions struct {
	Title        string
	BaseURL      string
	DisableKinds []string
	Params       map[string]string
	MarkupUnsafe *bool
}

type Part interface {
	Apply(*WebApp)
}

type Bundle interface {
	Part
	WithHugo(mods ...HugoModule) Bundle
	HugoModules() []HugoModule
}

type WebApp struct {
	core        *web.WebApp
	baseDir     string
	moduleDir   string
	target      string
	hugoModules []HugoModule
	siteConfig  *SiteOptions
}

type BuildEngine struct {
	core    *web.BuildEngine
	builder *web.Builder
}

func Module(parts ...Part) Bundle { return &bundle{parts: cloneParts(parts)} }

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

func Activity(ref activity.URIRef, handler func(activity.Context) activity.Result, opts ...web.ActivityOption[struct{}]) *web.WebActivity[struct{}] {
	return web.Activity(ref, handler, opts...)
}

func Ref(id string) activity.URIRef { return activity.Ref(id) }

func RootRef() activity.URIRef { return activity.RootRef() }

func WithStaticTitle(title string) web.ActivityOption[struct{}] { return web.WithStaticTitle(title) }

func CSS(src AssetSource) Part {
	return partFunc(func(app *WebApp) { app.RegisterCSS(src) })
}

func TailwindCSS(src AssetSource) Part {
	return partFunc(func(app *WebApp) { app.RegisterTailwindCSS(src) })
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
	return partFunc(func(app *WebApp) { app.Mount(path, handler) })
}

func SiteConfig(opts SiteOptions) Part {
	return partFunc(func(app *WebApp) { app.registerSiteConfig(opts) })
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
	hugo  []HugoModule
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
	if len(b.hugo) > 0 {
		app.registerHugoModules(b.hugo...)
	}
}

func (b *bundle) WithHugo(mods ...HugoModule) Bundle {
	if b == nil || len(mods) == 0 {
		return b
	}
	b.hugo = appendUniqueHugoModules(b.hugo, mods...)
	return b
}

func (b *bundle) HugoModules() []HugoModule {
	if b == nil || len(b.hugo) == 0 {
		return nil
	}
	out := make([]HugoModule, 0, len(b.hugo))
	out = append(out, b.hugo...)
	return out
}

func cloneParts(parts []Part) []Part {
	if len(parts) == 0 {
		return nil
	}
	out := make([]Part, 0, len(parts))
	out = append(out, parts...)
	return out
}

func appendUniqueHugoModules(dst []HugoModule, mods ...HugoModule) []HugoModule {
	if len(mods) == 0 {
		return dst
	}
	if dst == nil {
		dst = make([]HugoModule, 0, len(mods))
	}
	existing := make(map[string]struct{}, len(dst))
	for _, mod := range dst {
		if strings.TrimSpace(mod.ImportPath) == "" {
			continue
		}
		existing[mod.ImportPath] = struct{}{}
	}
	for _, mod := range mods {
		mod.ImportPath = strings.TrimSpace(mod.ImportPath)
		mod.ReplacePath = strings.TrimSpace(mod.ReplacePath)
		if mod.ImportPath == "" {
			continue
		}
		if _, ok := existing[mod.ImportPath]; ok {
			continue
		}
		dst = append(dst, mod)
		existing[mod.ImportPath] = struct{}{}
	}
	return dst
}
