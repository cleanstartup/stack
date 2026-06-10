package web

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/cleanstartup/stack/activity"
)

type Part interface {
	Apply(*WebApp)
}

type HugoModule struct {
	ImportPath  string
	ReplacePath string
}

type Contributor = Part

type Bundle interface {
	Part
	WithHugo(mods ...HugoModule) Bundle
	HugoModules() []HugoModule
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

func AssetFS(source fs.FS, root string) Part {
	return partFunc(func(app *WebApp) {
		if app == nil {
			return
		}
		app.assetsFS = source
		app.assetRoot = strings.TrimSpace(root)
	})
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
