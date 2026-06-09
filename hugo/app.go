package hugo

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/cleanstartup/stack/cli"
	"github.com/cleanstartup/stack/web"
)

func NewApp(parts ...Part) *WebApp {
	app := &WebApp{
		core:  web.NewApp(),
		target: "hugo",
	}
	app.Apply(parts...)
	return app
}

func NewAppWithDefaults(baseDir string, parts ...Part) *WebApp {
	app := NewApp()
	app.baseDir = strings.TrimSpace(baseDir)
	app.moduleDir = moduleRoot(baseDir)
	app.Apply(append([]Part{Styles(baseDir), Components(baseDir)}, parts...)...)
	return app
}

func (a *WebApp) coreApp() *web.WebApp {
	if a == nil {
		return nil
	}
	if a.core == nil {
		a.core = web.NewApp()
	}
	return a.core
}

func (a *WebApp) Core() *web.WebApp {
	return a.coreApp()
}

func (a *WebApp) builder() *web.Builder {
	if a == nil || a.coreApp() == nil || a.coreApp().Engine() == nil {
		return nil
	}
	return a.coreApp().Engine().Builder()
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

func (a *WebApp) registerHugoModules(mods ...HugoModule) {
	if a == nil || len(mods) == 0 {
		return
	}
	a.hugoModules = appendUniqueHugoModules(a.hugoModules, mods...)
}

func (a *WebApp) HugoModules() []HugoModule {
	if a == nil || len(a.hugoModules) == 0 {
		return nil
	}
	out := make([]HugoModule, 0, len(a.hugoModules))
	out = append(out, a.hugoModules...)
	return out
}

func (a *WebApp) RegisterCSS(src AssetSource) AssetRef {
	if a == nil || a.coreApp() == nil {
		return AssetRef{}
	}
	ref := a.coreApp().RegisterCSS(src)
	return AssetRef{Kind: ref.Kind, ID: ref.ID, Files: append([]string{}, ref.Files...)}
}

func (a *WebApp) RegisterTailwindCSS(src AssetSource) AssetRef {
	if a == nil || a.coreApp() == nil {
		return AssetRef{}
	}
	ref := a.coreApp().RegisterTailwindCSS(src)
	return AssetRef{Kind: ref.Kind, ID: ref.ID, Files: append([]string{}, ref.Files...)}
}

func (a *WebApp) RegisterStencil(src AssetSource) AssetRef {
	if a == nil || a.coreApp() == nil {
		return AssetRef{}
	}
	ref := a.coreApp().RegisterStencil(src)
	return AssetRef{Kind: ref.Kind, ID: ref.ID, Files: append([]string{}, ref.Files...)}
}

func (a *WebApp) RegisterJS(src AssetSource) AssetRef {
	if a == nil || a.coreApp() == nil {
		return AssetRef{}
	}
	ref := a.coreApp().RegisterJS(src)
	return AssetRef{Kind: ref.Kind, ID: ref.ID, Files: append([]string{}, ref.Files...)}
}

func (a *WebApp) RegisterFile(src AssetSource) AssetRef {
	if a == nil || a.coreApp() == nil {
		return AssetRef{}
	}
	ref := a.coreApp().RegisterFile(src)
	return AssetRef{Kind: ref.Kind, ID: ref.ID, Files: append([]string{}, ref.Files...)}
}

func (a *WebApp) Mount(path string, handler http.Handler) {
	if a == nil || a.coreApp() == nil {
		return
	}
	a.coreApp().Mount(path, handler)
}

func (a *WebApp) RegisterTailwindScan(paths ...string) {
	if a == nil || a.coreApp() == nil {
		return
	}
	a.coreApp().RegisterTailwindScan(paths...)
}

func (a *WebApp) RegisterStencilScan(paths ...string) {
	if a == nil || a.coreApp() == nil {
		return
	}
	a.coreApp().RegisterStencilScan(paths...)
}

func (a *WebApp) RegisterContent(baseDir string, includes ...string) {
	if a == nil || a.coreApp() == nil {
		return
	}
	a.coreApp().RegisterContent(baseDir, includes...)
}

func (a *WebApp) RegisterLayouts(baseDir string, includes ...string) {
	if a == nil || a.coreApp() == nil {
		return
	}
	a.coreApp().RegisterLayouts(baseDir, includes...)
}

func (a *WebApp) RegisterSiteConfig(opts SiteOptions) {
	a.registerSiteConfig(opts)
}

func (a *WebApp) Build(ctx context.Context, cfg BuildConfig) (*BuildResult, error) {
	if a == nil {
		return nil, fmt.Errorf("target is nil")
	}
	return a.buildSite(ctx, cfg)
}

func (a *WebApp) Serve(ctx context.Context, cfg ServeConfig) error {
	if a == nil {
		return fmt.Errorf("target is nil")
	}
	if a.coreApp() == nil || a.coreApp().Engine() == nil {
		return fmt.Errorf("build engine is nil")
	}
	return a.coreApp().Engine().Serve(ctx, cfg)
}

func (a *WebApp) Dev(ctx context.Context, cfg DevConfig) error {
	if a == nil {
		return fmt.Errorf("target is nil")
	}
	return a.devSite(ctx, cfg)
}

func (a *WebApp) DevWithTempl(ctx context.Context, cfg DevConfig) error {
	if a == nil {
		return fmt.Errorf("target is nil")
	}
	return a.devWithTempl(ctx, cfg)
}

func (a *WebApp) CLI() *cli.Registry {
	if a == nil {
		return nil
	}
	return newWebCLI(a, webCLIConfig{
		runHelp:   "serve the already built hugo site",
		buildHelp: "render the hugo site and materialize assets into the output directory",
		devHelp:   "run hugo server and the asset watch loop",
		useTempl:  false,
	})
}

func (a *WebApp) buildEngine() *BuildEngine {
	if a == nil {
		return &BuildEngine{}
	}
	if a.coreApp() == nil || a.coreApp().Engine() == nil {
		return &BuildEngine{}
	}
	return &BuildEngine{core: a.coreApp().Engine(), builder: a.coreApp().Engine().Builder()}
}

func (a *WebApp) Engine() *BuildEngine {
	return a.buildEngine()
}
