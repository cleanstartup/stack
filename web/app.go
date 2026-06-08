package web

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cleanstartup/stack/cli"
)

type WebApp struct {
	builder     *Builder
	engine      *BuildEngine
	baseDir     string
	moduleDir   string
	target      TargetKind
	hugoModules []HugoModule
	siteConfig  *SiteOptions
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
	return app
}

func New(parts ...Part) *WebApp {
	app := newWebApp(TargetApp)
	app.Apply(parts...)
	return app
}

func NewSite(parts ...Part) *WebApp {
	app := newWebApp(TargetSite)
	app.Apply(parts...)
	app.RegisterTailwindScan(siteModuleDir())
	app.registerHugoModules(siteHugoModule())
	return app
}

func NewWithDefaults(baseDir string, parts ...Part) *WebApp {
	app := newWebApp(TargetApp)
	app.baseDir = strings.TrimSpace(baseDir)
	app.moduleDir = moduleRoot(baseDir)
	app.Apply(append([]Part{Styles(baseDir), Components(baseDir)}, parts...)...)
	return app
}

func NewSiteWithDefaults(baseDir string, parts ...Part) *WebApp {
	app := newWebApp(TargetSite)
	app.baseDir = strings.TrimSpace(baseDir)
	app.moduleDir = moduleRoot(baseDir)
	app.Apply(append([]Part{TailwindScan(siteModuleDir())}, parts...)...)
	app.registerHugoModules(siteHugoModule())
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

func (a *WebApp) Target() TargetKind {
	if a == nil {
		return TargetApp
	}
	if a.target == "" {
		return TargetApp
	}
	return a.target
}

func (a *WebApp) Build(ctx context.Context, cfg BuildConfig) (*BuildResult, error) {
	if a == nil || a.engine == nil {
		return nil, fmt.Errorf("target is nil")
	}
	if a.target == TargetSite {
		return a.buildSite(ctx, cfg)
	}
	return a.engine.Build(ctx, cfg)
}

func (a *WebApp) Serve(ctx context.Context, cfg ServeConfig) error {
	if a == nil || a.engine == nil {
		return fmt.Errorf("target is nil")
	}
	return a.engine.Serve(ctx, cfg)
}

func (a *WebApp) Dev(ctx context.Context, cfg DevConfig) error {
	if a == nil || a.engine == nil {
		return fmt.Errorf("target is nil")
	}
	if a.target == TargetSite {
		return a.devSite(ctx, cfg)
	}
	return a.engine.Dev(ctx, cfg)
}

func (a *WebApp) CLI() *cli.Registry {
	r := cli.NewRegistry()
	runHelp := "serve the already built web app"
	buildHelp := "materialize assets into the output directory"
	devHelp := "run the templ supervisor and asset watch loop"
	if a != nil && a.target == TargetSite {
		runHelp = "serve the already built static site"
		buildHelp = "render the site and materialize assets into the output directory"
		devHelp = "run hugo server and the asset watch loop"
	}

	runCmd := cli.Activity(
		"run",
		func(inv *cli.Invocation) runCommandInput {
			return runCommandInput{
				Addr:      stringParam(inv, defaultAddr, "addr", "a"),
				OutputDir: stringParam(inv, defaultOutputDir, "output", "o"),
			}
		},
		func(ctx cli.Context[runCommandInput]) cli.Result {
			if err := a.Serve(context.Background(), ServeConfig{
				Addr:      ctx.Data().Addr,
				OutputDir: ctx.Data().OutputDir,
			}); err != nil {
				return cli.Error(err.Error())
			}
			return cli.Done()
		},
		cli.WithHelp[runCommandInput](runHelp),
	)
	cli.RegisterActivity(r, runCmd)

	buildCmd := cli.Activity(
		"build",
		func(inv *cli.Invocation) buildCommandInput {
			return buildCommandInput{
				WorkspaceDir: stringParam(inv, defaultWorkspaceDir, "workspace", "w"),
				OutputDir:    stringParam(inv, defaultOutputDir, "output", "o"),
			}
		},
		func(ctx cli.Context[buildCommandInput]) cli.Result {
			result, err := a.Build(context.Background(), BuildConfig{
				WorkspaceDir: ctx.Data().WorkspaceDir,
				OutputDir:    ctx.Data().OutputDir,
			})
			if err != nil {
				return cli.Error(err.Error())
			}
			if a != nil && a.target == TargetSite {
				return cli.Textf("rendered %d files into %s", len(result.Assets), result.OutputDir)
			}
			return cli.Textf("built %d assets into %s", len(result.Assets), result.OutputDir)
		},
		cli.WithHelp[buildCommandInput](buildHelp),
	)
	cli.RegisterActivity(r, buildCmd)

	devCmd := cli.Activity(
		"dev",
		func(inv *cli.Invocation) devCommandInput {
			poll := stringParam(inv, "1s", "poll", "p")
			interval, err := time.ParseDuration(poll)
			if err != nil {
				interval = time.Second
			}
			return devCommandInput{
				Addr:         stringParam(inv, defaultAddr, "addr", "a"),
				WorkspaceDir: stringParam(inv, defaultWorkspaceDir, "workspace", "w"),
				OutputDir:    stringParam(inv, defaultOutputDir, "output", "o"),
				PollInterval: interval,
				Child:        boolParam(inv, false, "child"),
			}
		},
		func(ctx cli.Context[devCommandInput]) cli.Result {
			cfg := DevConfig{
				Addr:         ctx.Data().Addr,
				WorkspaceDir: ctx.Data().WorkspaceDir,
				OutputDir:    ctx.Data().OutputDir,
				PollInterval: ctx.Data().PollInterval,
			}
			var err error
			if ctx.Data().Child {
				err = a.Dev(context.Background(), cfg)
			} else if a != nil && a.target == TargetSite {
				err = a.Dev(context.Background(), cfg)
			} else {
				err = a.DevWithTempl(context.Background(), cfg)
			}
			if err != nil {
				return cli.Error(err.Error())
			}
			return cli.Done()
		},
		cli.WithHelp[devCommandInput](devHelp),
	)
	cli.RegisterActivity(r, devCmd)

	return r
}

func Serve(addr string, parts ...Part) error {
	app := NewWithDefaults(CallerDir(1), parts...)
	return app.Serve(context.Background(), ServeConfig{
		Addr:      addr,
		OutputDir: defaultOutputDir,
	})
}

func ExecuteCLI(parts ...Part) error {
	app := NewWithDefaults(CallerDir(1), parts...)
	registry := app.CLI()

	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"--help"}
	}

	result := registry.Execute(args)
	if strings.TrimSpace(result.Stdout) != "" {
		_, _ = fmt.Fprint(os.Stdout, result.Stdout)
		if !strings.HasSuffix(result.Stdout, "\n") {
			_, _ = fmt.Fprintln(os.Stdout)
		}
	}
	if result.ExitCode != 0 {
		message := strings.TrimSpace(result.Stderr)
		if message == "" {
			message = strings.TrimSpace(result.Stdout)
		}
		if message == "" {
			message = "web command failed"
		}
		return errors.New(message)
	}
	return nil
}

func App(parts ...Part) {
	app := NewWithDefaults(CallerDir(1), parts...)
	runWebCLI(app)
}

func Site(parts ...Part) {
	app := NewSiteWithDefaults(CallerDir(1), parts...)
	runWebCLI(app)
}

func SiteAt(baseDir string, parts ...Part) {
	app := NewSiteWithDefaults(baseDir, parts...)
	runWebCLI(app)
}

func runWebCLI(app *WebApp) {
	registry := app.CLI()
	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"--help"}
	}

	result := registry.Execute(args)
	if strings.TrimSpace(result.Stdout) != "" {
		_, _ = fmt.Fprint(os.Stdout, result.Stdout)
		if !strings.HasSuffix(result.Stdout, "\n") {
			_, _ = fmt.Fprintln(os.Stdout)
		}
	}
	if result.ExitCode != 0 {
		message := strings.TrimSpace(result.Stderr)
		if message == "" {
			message = strings.TrimSpace(result.Stdout)
		}
		if message == "" {
			message = "web command failed"
		}
		_, _ = fmt.Fprintln(os.Stderr, message)
		os.Exit(1)
	}
}

func Run(parts ...Part) { App(parts...) }

type runCommandInput struct {
	Addr      string
	OutputDir string
}

type buildCommandInput struct {
	WorkspaceDir string
	OutputDir    string
}

type devCommandInput struct {
	Addr         string
	WorkspaceDir string
	OutputDir    string
	PollInterval time.Duration
	Child        bool
}

func stringParam(inv *cli.Invocation, fallback string, name string, aliases ...string) string {
	if inv == nil {
		return fallback
	}
	key := cli.Param(name, aliases...)
	value := inv.StringParam(key)
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func boolParam(inv *cli.Invocation, fallback bool, name string, aliases ...string) bool {
	if inv == nil {
		return fallback
	}
	key := cli.Param(name, aliases...)
	value := strings.TrimSpace(inv.StringParam(key))
	if value == "" {
		return fallback
	}
	switch strings.ToLower(value) {
	case "1", "t", "true", "y", "yes", "on":
		return true
	case "0", "f", "false", "n", "no", "off":
		return false
	default:
		return fallback
	}
}

func (a *WebApp) DevWithTempl(ctx context.Context, cfg DevConfig) error {
	if a == nil || a.engine == nil {
		return fmt.Errorf("build engine is nil")
	}
	if a.target == TargetSite {
		return a.devSite(ctx, cfg)
	}
	moduleDir := strings.TrimSpace(a.moduleDir)
	if strings.TrimSpace(moduleDir) == "" {
		moduleDir = moduleRoot(a.baseDir)
	}
	if strings.TrimSpace(moduleDir) == "" {
		moduleDir = CallerDir(1)
	}
	if strings.TrimSpace(moduleDir) == "" {
		moduleDir = "."
	}
	childCmd := devChildCommand(moduleDir, a.baseDir, cfg)
	proxyURL := templProxyURL(cfg.Addr)
	templArgs := []string{
		"tool",
		"templ",
		"generate",
		"--watch",
		"--proxy=" + proxyURL,
		"--cmd=" + childCmd,
	}
	cmd := exec.CommandContext(ctx, "go", templArgs...)
	cmd.Dir = moduleDir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	fmt.Fprintf(os.Stderr, "[stack] templ dev supervisor start cmd=%s dir=%s proxy=%s\n", strings.Join(templArgs, " "), cmd.Dir, proxyURL)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("templ dev supervisor failed: %w", err)
	}
	return nil
}

func devChildCommand(moduleDir, baseDir string, cfg DevConfig) string {
	moduleDir = strings.TrimSpace(moduleDir)
	baseDir = strings.TrimSpace(baseDir)
	target := "."
	if moduleDir != "" && baseDir != "" {
		if rel, err := filepath.Rel(moduleDir, baseDir); err == nil && strings.TrimSpace(rel) != "" {
			target = rel
		}
	}
	targetArg := "."
	if target != "." {
		targetArg = "./" + filepath.ToSlash(target)
	}
	addr := strings.TrimSpace(cfg.Addr)
	if addr == "" {
		addr = defaultAddr
	}
	workspace := strings.TrimSpace(cfg.WorkspaceDir)
	if workspace == "" {
		workspace = defaultWorkspaceDir
	}
	output := strings.TrimSpace(cfg.OutputDir)
	if output == "" {
		output = defaultOutputDir
	}
	poll := cfg.PollInterval.String()
	if cfg.PollInterval <= 0 {
		poll = (250 * time.Millisecond).String()
	}
	return strings.Join([]string{
		"go",
		"run",
		targetArg,
		"dev",
		"--child",
		"--addr=" + addr,
		"--workspace=" + workspace,
		"--output=" + output,
		"--poll=" + poll,
	}, " ")
}

func templProxyURL(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		addr = defaultAddr
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil || strings.TrimSpace(port) == "" {
		if strings.HasPrefix(addr, ":") {
			port = strings.TrimPrefix(addr, ":")
		} else {
			port = strings.TrimSpace(addr)
		}
	}
	host = strings.TrimSpace(host)
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	if strings.TrimSpace(port) == "" {
		port = strings.TrimPrefix(defaultAddr, ":")
	}
	return "http://" + host + ":" + port
}
