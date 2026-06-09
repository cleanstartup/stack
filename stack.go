package stack

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cleanstartup/stack/activity"
	"github.com/cleanstartup/stack/cli"
	"github.com/cleanstartup/stack/hugo"
	"github.com/cleanstartup/stack/web"
)

const (
	defaultWorkspaceDir = ".stack/workspace"
	defaultOutputDir    = ".stack/public"
	defaultAddr         = ":8080"
)

type Mode string

const (
	ModeBuild Mode = "build"
	ModeDev   Mode = "dev"
)

type TargetKind string

const (
	TargetApp       TargetKind = "app"
	TargetHugo      TargetKind = "hugo"
	TargetTailwind  TargetKind = "tailwind"
	TargetStencil   TargetKind = "stencil"
	TargetComposite TargetKind = "composite"
)

type Target struct {
	Kind TargetKind
	Name string

	BaseDir      string
	WorkspaceDir string
	OutputDir    string
	Addr         string
	PollInterval time.Duration

	Assets   []hugo.Part
	Parts    []hugo.Part
	Children []*Target
}

type TargetOption func(*Target)

type Context struct {
	ProjectDir   string
	Mode         Mode
	WorkspaceDir string
	OutputDir    string
	Addr         string
	PollInterval time.Duration
}

type buildInput struct {
	WorkspaceDir string
	OutputDir    string
}

type devInput struct {
	WorkspaceDir string
	OutputDir    string
	Addr         string
	PollInterval time.Duration
}

func App(opts ...TargetOption) *Target {
	return newTarget(TargetApp, opts...)
}

func Hugo(opts ...TargetOption) *Target {
	return newTarget(TargetHugo, opts...)
}

func Tailwind(opts ...TargetOption) *Target {
	return newTarget(TargetTailwind, opts...)
}

func Stencil(opts ...TargetOption) *Target {
	return newTarget(TargetStencil, opts...)
}

func Composite(opts ...TargetOption) *Target {
	return newTarget(TargetComposite, opts...)
}

func newTarget(kind TargetKind, opts ...TargetOption) *Target {
	t := &Target{
		Kind:         kind,
		Name:         string(kind),
		BaseDir:      web.CallerDir(2),
		WorkspaceDir: defaultWorkspaceDir,
		OutputDir:    defaultOutputDir,
		Addr:         defaultAddr,
		PollInterval: 250 * time.Millisecond,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(t)
	}
	if strings.TrimSpace(t.Name) == "" {
		t.Name = string(kind)
	}
	return t
}

func WithName(name string) TargetOption {
	return func(t *Target) {
		if t == nil {
			return
		}
		t.Name = strings.TrimSpace(name)
	}
}

func WithBaseDir(baseDir string) TargetOption {
	return func(t *Target) {
		if t == nil {
			return
		}
		baseDir = strings.TrimSpace(baseDir)
		if baseDir == "" {
			return
		}
		t.BaseDir = baseDir
	}
}

func WithWorkspaceDir(workspaceDir string) TargetOption {
	return func(t *Target) {
		if t == nil {
			return
		}
		workspaceDir = strings.TrimSpace(workspaceDir)
		if workspaceDir == "" {
			return
		}
		t.WorkspaceDir = workspaceDir
	}
}

func WithOutputDir(outputDir string) TargetOption {
	return func(t *Target) {
		if t == nil {
			return
		}
		outputDir = strings.TrimSpace(outputDir)
		if outputDir == "" {
			return
		}
		t.OutputDir = outputDir
	}
}

func WithAddr(addr string) TargetOption {
	return func(t *Target) {
		if t == nil {
			return
		}
		addr = strings.TrimSpace(addr)
		if addr == "" {
			return
		}
		t.Addr = addr
	}
}

func WithPollInterval(interval time.Duration) TargetOption {
	return func(t *Target) {
		if t == nil || interval <= 0 {
			return
		}
		t.PollInterval = interval
	}
}

func WithParts(parts ...hugo.Part) TargetOption {
	return func(t *Target) {
		if t == nil || len(parts) == 0 {
			return
		}
		t.Parts = append(t.Parts, parts...)
	}
}

func WithAssets(parts ...hugo.Part) TargetOption {
	return func(t *Target) {
		if t == nil || len(parts) == 0 {
			return
		}
		t.Assets = append(t.Assets, parts...)
	}
}

func WithChildren(children ...*Target) TargetOption {
	return func(t *Target) {
		if t == nil || len(children) == 0 {
			return
		}
		for _, child := range children {
			if child == nil {
				continue
			}
			t.Children = append(t.Children, child)
		}
	}
}

func WithSiteConfig(opts hugo.SiteOptions) TargetOption {
	return WithParts(hugo.SiteConfig(opts))
}

func WithHugoModules(mods ...hugo.HugoModule) TargetOption {
	return func(t *Target) {
		if t == nil || len(mods) == 0 {
			return
		}
		bundle := hugo.Module()
		t.Parts = append(t.Parts, bundle.WithHugo(mods...))
	}
}

type Bundle = hugo.Bundle
type Part = hugo.Part
type URIRef = activity.URIRef
type SiteOptions = hugo.SiteOptions
type HugoModule = hugo.HugoModule

func Module(parts ...hugo.Part) hugo.Bundle { return hugo.Module(parts...) }
func Compose(parts ...hugo.Part) hugo.Part  { return hugo.Compose(parts...) }
func Activity(ref activity.URIRef, handler func(activity.Context) activity.Result, opts ...hugo.ActivityOption[struct{}]) Part {
	act := hugo.Activity(ref, handler, opts...)
	return activityPart{activity: act}
}
func Ref(id string) activity.URIRef                              { return activity.Ref(id) }
func RootRef() activity.URIRef                                   { return activity.RootRef() }
func WithStaticTitle(title string) hugo.ActivityOption[struct{}] { return hugo.WithStaticTitle(title) }

func Styles(baseDir ...string) Part                   { return hugo.Styles(baseDir...) }
func Components(baseDir ...string) Part               { return hugo.Components(baseDir...) }
func ConventionalAssets(baseDir ...string) Part       { return hugo.ConventionalAssets(baseDir...) }
func Content(baseDir string, includes ...string) Part { return hugo.Content(baseDir, includes...) }
func Layouts(baseDir string, includes ...string) Part { return hugo.Layouts(baseDir, includes...) }
func SiteConfig(opts SiteOptions) Part                { return hugo.SiteConfig(opts) }
func CSS(src hugo.AssetSource) Part                   { return hugo.CSS(src) }
func TailwindCSS(src hugo.AssetSource) Part           { return hugo.TailwindCSS(src) }
func JS(src hugo.AssetSource) Part                    { return hugo.JS(src) }
func File(src hugo.AssetSource) Part                  { return hugo.File(src) }
func TailwindScan(paths ...string) Part               { return hugo.TailwindScan(paths...) }
func StencilScan(paths ...string) Part                { return hugo.StencilScan(paths...) }
func Mount(path string, handler http.Handler) Part    { return hugo.Mount(path, handler) }

type activityPart struct {
	activity *hugo.WebActivity[struct{}]
}

func (p activityPart) Apply(app *hugo.WebApp) {
	if app == nil || p.activity == nil || app.Core() == nil {
		return
	}
	app.Core().Apply(p.activity)
}

func Execute(target *Target) error {
	return ExecuteContext(context.Background(), target)
}

func ExecuteContext(ctx context.Context, target *Target) error {
	if target == nil {
		return errors.New("target is nil")
	}
	registry := target.registry(ctx)
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
			message = "stack command failed"
		}
		return errors.New(message)
	}
	return nil
}

func (t *Target) registry(parentCtx context.Context) *cli.Registry {
	registry := cli.NewRegistry()
	if t == nil {
		return registry
	}

	buildCmd := cli.Activity(
		"build",
		func(inv *cli.Invocation) buildInput {
			return buildInput{
				WorkspaceDir: stringParam(inv, t.WorkspaceDir, "workspace", "w"),
				OutputDir:    stringParam(inv, t.OutputDir, "output", "o"),
			}
		},
		func(ctx cli.Context[buildInput]) cli.Result {
			cfg := Context{
				Mode:         ModeBuild,
				WorkspaceDir: ctx.Data().WorkspaceDir,
				OutputDir:    ctx.Data().OutputDir,
			}
			if err := t.execute(parentCtx, cfg); err != nil {
				return cli.Error(err.Error())
			}
			return cli.Done()
		},
		cli.WithHelp[buildInput]("materialize the target graph"),
	)
	cli.RegisterActivity(registry, buildCmd)

	devCmd := cli.Activity(
		"dev",
		func(inv *cli.Invocation) devInput {
			poll := stringParam(inv, t.pollIntervalString(), "poll", "p")
			interval, err := time.ParseDuration(poll)
			if err != nil {
				interval = t.PollInterval
			}
			return devInput{
				WorkspaceDir: stringParam(inv, t.WorkspaceDir, "workspace", "w"),
				OutputDir:    stringParam(inv, t.OutputDir, "output", "o"),
				Addr:         stringParam(inv, t.Addr, "addr", "a"),
				PollInterval: interval,
			}
		},
		func(ctx cli.Context[devInput]) cli.Result {
			cfg := Context{
				Mode:         ModeDev,
				WorkspaceDir: ctx.Data().WorkspaceDir,
				OutputDir:    ctx.Data().OutputDir,
				Addr:         ctx.Data().Addr,
				PollInterval: ctx.Data().PollInterval,
			}
			if err := t.execute(parentCtx, cfg); err != nil {
				return cli.Error(err.Error())
			}
			return cli.Done()
		},
		cli.WithHelp[devInput]("run the target graph in dev mode"),
	)
	cli.RegisterActivity(registry, devCmd)

	return registry
}

func (t *Target) execute(ctx context.Context, cfg Context) error {
	if t == nil {
		return errors.New("target is nil")
	}
	resolved := t.resolveExecution(cfg)
	switch t.Kind {
	case TargetHugo:
		return t.executeHugo(ctx, resolved)
	case TargetTailwind, TargetStencil:
		return t.executeAssetTarget(ctx, resolved)
	default:
		return t.executeApp(ctx, resolved)
	}
}

func (t *Target) resolveExecution(cfg Context) Context {
	if t == nil {
		return cfg
	}
	if strings.TrimSpace(cfg.WorkspaceDir) == "" {
		cfg.WorkspaceDir = t.WorkspaceDir
	}
	if strings.TrimSpace(cfg.ProjectDir) == "" {
		cfg.ProjectDir = t.baseDir()
	}
	if strings.TrimSpace(cfg.OutputDir) == "" {
		cfg.OutputDir = t.OutputDir
	}
	if strings.TrimSpace(cfg.Addr) == "" {
		cfg.Addr = t.Addr
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = t.PollInterval
	}
	if strings.TrimSpace(cfg.WorkspaceDir) == "" {
		cfg.WorkspaceDir = defaultWorkspaceDir
	}
	if strings.TrimSpace(cfg.ProjectDir) == "" {
		cfg.ProjectDir = t.baseDir()
	}
	if strings.TrimSpace(cfg.OutputDir) == "" {
		cfg.OutputDir = defaultOutputDir
	}
	if strings.TrimSpace(cfg.Addr) == "" {
		cfg.Addr = defaultAddr
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 250 * time.Millisecond
	}
	return cfg
}

func (t *Target) executeApp(ctx context.Context, cfg Context) error {
	parts := t.collectParts()
	app := hugo.NewAppWithDefaults(t.baseDir(), parts...)
	engine := app.Engine()
	switch cfg.Mode {
	case ModeDev:
		return engine.Dev(ctx, hugo.DevConfig{
			ProjectDir:   cfg.ProjectDir,
			Addr:         cfg.Addr,
			WorkspaceDir: cfg.WorkspaceDir,
			OutputDir:    cfg.OutputDir,
			PollInterval: cfg.PollInterval,
		})
	default:
		_, err := engine.Build(ctx, hugo.BuildConfig{
			ProjectDir:   cfg.ProjectDir,
			WorkspaceDir: cfg.WorkspaceDir,
			OutputDir:    cfg.OutputDir,
		})
		return err
	}
}

func (t *Target) executeHugo(ctx context.Context, cfg Context) error {
	parts := t.collectParts()
	app := hugo.NewAppWithDefaults(t.baseDir(), parts...)
	switch cfg.Mode {
	case ModeDev:
		return app.Dev(ctx, hugo.DevConfig{
			ProjectDir:   cfg.ProjectDir,
			Addr:         cfg.Addr,
			WorkspaceDir: cfg.WorkspaceDir,
			OutputDir:    cfg.OutputDir,
			PollInterval: cfg.PollInterval,
		})
	default:
		_, err := app.Build(ctx, hugo.BuildConfig{
			ProjectDir:   cfg.ProjectDir,
			WorkspaceDir: cfg.WorkspaceDir,
			OutputDir:    cfg.OutputDir,
		})
		return err
	}
}

func (t *Target) executeAssetTarget(ctx context.Context, cfg Context) error {
	app := hugo.NewAppWithDefaults(t.baseDir(), t.collectParts()...)
	engine := app.Engine()
	switch cfg.Mode {
	case ModeDev:
		return engine.Dev(ctx, hugo.DevConfig{
			ProjectDir:   cfg.ProjectDir,
			Addr:         cfg.Addr,
			WorkspaceDir: cfg.WorkspaceDir,
			OutputDir:    cfg.OutputDir,
			PollInterval: cfg.PollInterval,
		})
	default:
		_, err := engine.BuildAssets(ctx, hugo.BuildConfig{
			ProjectDir:   cfg.ProjectDir,
			WorkspaceDir: cfg.WorkspaceDir,
			OutputDir:    cfg.OutputDir,
		})
		return err
	}
}

func (t *Target) collectParts() []hugo.Part {
	if t == nil {
		return nil
	}
	var parts []hugo.Part
	parts = append(parts, t.Assets...)
	parts = append(parts, t.Parts...)
	for _, child := range t.Children {
		if child == nil {
			continue
		}
		parts = append(parts, child.collectParts()...)
	}
	return parts
}

func (t *Target) baseDir() string {
	if t == nil {
		return ""
	}
	baseDir := strings.TrimSpace(t.BaseDir)
	if baseDir == "" {
		return web.CallerDir(2)
	}
	return baseDir
}

func (t *Target) pollIntervalString() string {
	if t == nil || t.PollInterval <= 0 {
		return "250ms"
	}
	return t.PollInterval.String()
}

func stringParam(inv *cli.Invocation, fallback string, name string, aliases ...string) string {
	if inv == nil {
		return fallback
	}
	value := strings.TrimSpace(inv.StringParam(cli.Param(name, aliases...)))
	if value == "" {
		return fallback
	}
	return value
}

func boolParam(inv *cli.Invocation, fallback bool, name string, aliases ...string) bool {
	if inv == nil {
		return fallback
	}
	raw := strings.TrimSpace(inv.StringParam(cli.Param(name, aliases...)))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return true
	}
	return parsed
}
