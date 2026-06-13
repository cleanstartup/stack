package web

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cleanstartup/stack/cli"
)

type WebApp struct {
	builder *Builder
	engine  *BuildEngine
	baseDir string
	runtime webRuntime
}

func newWebApp() *WebApp {
	builder := NewBuilder()
	app := &WebApp{builder: builder}
	app.engine = &BuildEngine{builder: builder}
	app.runtime = newWebRuntime(app)
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

func (a *WebApp) Engine() *BuildEngine {
	if a == nil {
		return nil
	}
	return a.engine
}

func (a *WebApp) BaseDir() string {
	if a == nil {
		return ""
	}
	return a.baseDir
}

func (a *WebApp) Build(ctx context.Context, cfg BuildConfig) (*BuildResult, error) {
	if a == nil || a.runtime == nil {
		return nil, fmt.Errorf("target is nil")
	}
	return a.runtime.Build(ctx, cfg)
}

func (a *WebApp) Serve(ctx context.Context, cfg ServeConfig) error {
	if a == nil || a.runtime == nil {
		return fmt.Errorf("target is nil")
	}
	return a.runtime.Serve(ctx, cfg)
}

func (a *WebApp) Dev(ctx context.Context, cfg DevConfig) error {
	if a == nil || a.runtime == nil {
		return fmt.Errorf("target is nil")
	}
	return a.runtime.Dev(ctx, cfg)
}

func (a *WebApp) CLI() *cli.Registry {
	if a == nil || a.runtime == nil {
		return nil
	}
	return a.runtime.CLI()
}
