package web

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/cleanstartup/stack/cli"
)

type SiteOptions struct {
	Title        string
	BaseURL      string
	DisableKinds []string
	Params       map[string]string
	MarkupUnsafe *bool
}

type WebApp struct {
	builder     *Builder
	engine      *BuildEngine
	baseDir     string
	moduleDir   string
	target      TargetKind
	hugoModules []HugoModule
	siteConfig  *SiteOptions
	runtime     webRuntime
}

type hugoModuleProvider interface {
	HugoModules() []HugoModule
}

func newWebApp(target ...TargetKind) *WebApp {
	appTarget := TargetApp
	if len(target) > 0 && target[0] != "" {
		appTarget = target[0]
	}
	builder := NewBuilder()
	app := &WebApp{builder: builder, target: appTarget}
	app.engine = &BuildEngine{builder: builder}
	app.runtime = newWebRuntime(app, appTarget)
	return app
}

func NewApp(parts ...Part) *WebApp {
	app := newWebApp(TargetApp)
	app.Apply(parts...)
	return app
}

func NewAppWithDefaults(baseDir string, parts ...Part) *WebApp {
	app := newWebApp(TargetApp)
	app.baseDir = strings.TrimSpace(baseDir)
	app.moduleDir = moduleRoot(baseDir)
	app.Apply(append([]Part{Styles(baseDir), Components(baseDir)}, parts...)...)
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
		if module, ok := part.(hugoModuleProvider); ok {
			a.registerHugoModules(module.HugoModules()...)
		}
		part.Apply(a)
	}
}

func (a *WebApp) registerHugoModules(mods ...HugoModule) {
	if a == nil || len(mods) == 0 {
		return
	}
	existing := make(map[string]struct{}, len(a.hugoModules))
	for _, mod := range a.hugoModules {
		if strings.TrimSpace(mod.ImportPath) == "" {
			continue
		}
		existing[mod.ImportPath] = struct{}{}
	}
	for _, mod := range mods {
		if strings.TrimSpace(mod.ImportPath) == "" {
			continue
		}
		if _, ok := existing[mod.ImportPath]; ok {
			continue
		}
		a.hugoModules = append(a.hugoModules, mod)
		existing[mod.ImportPath] = struct{}{}
	}
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

func (a *WebApp) RegisterTailwindScan(paths ...string) {
	a.builder.TailwindScan(paths...)
}

func (a *WebApp) RegisterStencilScan(paths ...string) {
	a.builder.StencilScan(paths...)
}

func (a *WebApp) RegisterContent(baseDir string, includes ...string) {
	a.builder.Content(baseDir, includes...)
}

func (a *WebApp) RegisterLayouts(baseDir string, includes ...string) {
	a.builder.Layouts(baseDir, includes...)
}

func (a *WebApp) RegisterSiteConfig(opts SiteOptions) {
	a.registerSiteConfig(opts)
}

func (a *WebApp) registerSiteConfig(opts SiteOptions) {
	if a == nil {
		return
	}
	if a.siteConfig == nil {
		a.siteConfig = &SiteOptions{}
	}
	if strings.TrimSpace(opts.Title) != "" {
		a.siteConfig.Title = strings.TrimSpace(opts.Title)
	}
	if strings.TrimSpace(opts.BaseURL) != "" {
		a.siteConfig.BaseURL = strings.TrimSpace(opts.BaseURL)
	}
	if len(opts.DisableKinds) > 0 {
		a.siteConfig.DisableKinds = mergeStrings(a.siteConfig.DisableKinds, opts.DisableKinds)
	}
	if len(opts.Params) > 0 {
		if a.siteConfig.Params == nil {
			a.siteConfig.Params = map[string]string{}
		}
		for key, value := range opts.Params {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			a.siteConfig.Params[key] = value
		}
	}
	if opts.MarkupUnsafe != nil {
		if a.siteConfig.MarkupUnsafe == nil {
			a.siteConfig.MarkupUnsafe = new(bool)
		}
		*a.siteConfig.MarkupUnsafe = *opts.MarkupUnsafe
	}
}

func (a *WebApp) SiteConfig() *SiteOptions {
	if a == nil || a.siteConfig == nil {
		return nil
	}
	out := *a.siteConfig
	if len(out.DisableKinds) > 0 {
		out.DisableKinds = append([]string{}, out.DisableKinds...)
	}
	if len(out.Params) > 0 {
		out.Params = make(map[string]string, len(out.Params))
		for key, value := range a.siteConfig.Params {
			out.Params[key] = value
		}
	}
	if a.siteConfig.MarkupUnsafe != nil {
		val := *a.siteConfig.MarkupUnsafe
		out.MarkupUnsafe = &val
	}
	return &out
}

func (a *WebApp) Target() TargetKind {
	if a == nil {
		return TargetApp
	}
	if a.target == "" {
		return TargetApp
	}
	return a.target
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

func (a *WebApp) ModuleDir() string {
	if a == nil {
		return ""
	}
	return a.moduleDir
}

func mergeStrings(dst []string, src []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(dst)+len(src))
	for _, value := range dst {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	for _, value := range src {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func tomlString(value string) string { return strconv.Quote(value) }

func siteConfigToml(opts SiteOptions) (string, error) {
	var b strings.Builder
	if strings.TrimSpace(opts.Title) != "" {
		fmt.Fprintf(&b, "title = %s\n", tomlString(opts.Title))
	}
	if strings.TrimSpace(opts.BaseURL) != "" {
		fmt.Fprintf(&b, "baseURL = %s\n", tomlString(opts.BaseURL))
	}
	if len(opts.DisableKinds) > 0 {
		b.WriteString("disableKinds = [")
		for i, kind := range mergeStrings(nil, opts.DisableKinds) {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(tomlString(kind))
		}
		b.WriteString("]\n")
	}
	if len(opts.Params) > 0 {
		keys := make([]string, 0, len(opts.Params))
		for key := range opts.Params {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("[params]\n")
		for _, key := range keys {
			value := strings.TrimSpace(opts.Params[key])
			fmt.Fprintf(&b, "%s = %s\n", key, tomlString(value))
		}
	}
	if opts.MarkupUnsafe != nil {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("[markup.goldmark.renderer]\n")
		if *opts.MarkupUnsafe {
			b.WriteString("unsafe = true\n")
		} else {
			b.WriteString("unsafe = false\n")
		}
	}
	return b.String(), nil
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
