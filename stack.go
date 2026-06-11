package stack

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/a-h/templ"
	"github.com/cleanstartup/stack/activity"
	"github.com/cleanstartup/stack/cli"
	"github.com/cleanstartup/stack/hugo"
	"github.com/cleanstartup/stack/web"
)

const defaultAddr = ":8080"

type Mode string

const (
	ModeRun     Mode = "run"
	ModeInstall Mode = "install"
	ModeBuild   Mode = "build"
	ModeDev     Mode = "dev"
)

type TargetKind string

const (
	TargetWebApp   TargetKind = "webapp"
	TargetSite     TargetKind = "site"
	TargetTailwind TargetKind = "tailwind"
	TargetStencil  TargetKind = "stencil"
	TargetArtifact TargetKind = "artifact"

	// Deprecated: use TargetWebApp.
	TargetApp = TargetWebApp
	// Deprecated: use TargetSite.
	TargetHugo = TargetSite
	// Deprecated: use TargetArtifact.
	TargetComposite = TargetArtifact
)

type Target struct {
	Kind TargetKind
	Name string

	BaseDir      string
	ProjectDir   string
	WorkspaceDir string
	OutputDir    string
	Addr         string
	PollInterval time.Duration

	workspaceDirSet bool
	outputDirSet    bool

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
	Command      string
}

type buildInput struct {
	WorkspaceDir string
	OutputDir    string
	Command      string
}

type installInput struct {
	WorkspaceDir string
	OutputDir    string
}

type runInput struct {
	OutputDir string
	Addr      string
}

type devInput struct {
	WorkspaceDir string
	OutputDir    string
	Addr         string
	PollInterval time.Duration
	Command      string
}

// Deprecated: use WebApp.
func App(opts ...TargetOption) *Target {
	return WebApp(opts...)
}

// Deprecated: use Site.
func Hugo(opts ...TargetOption) *Target {
	return Site(opts...)
}

func Tailwind(opts ...TargetOption) *Target {
	return newTarget(TargetTailwind, opts...)
}

func Stencil(opts ...TargetOption) *Target {
	return newTarget(TargetStencil, opts...)
}

// Deprecated: use Artifact.
func Composite(opts ...TargetOption) *Target {
	return Artifact(opts...)
}

func Artifact(opts ...TargetOption) *Target {
	return newTarget(TargetArtifact, opts...)
}

func WebApp(opts ...TargetOption) *Target {
	return newTarget(TargetWebApp, opts...)
}

func Site(opts ...TargetOption) *Target {
	return newTarget(TargetSite, opts...)
}

func newTarget(kind TargetKind, opts ...TargetOption) *Target {
	t := &Target{
		Kind:         kind,
		Name:         defaultTargetName(kind),
		BaseDir:      web.CallerDir(2),
		ProjectDir:   web.CallerDir(2),
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
		t.Name = defaultTargetName(kind)
	}
	t.applyPathDefaults()
	return t
}

func defaultTargetName(kind TargetKind) string {
	switch kind {
	case TargetWebApp:
		return "app"
	case TargetSite:
		return "site"
	case TargetArtifact:
		return "artifact"
	case TargetTailwind:
		return "tailwind"
	case TargetStencil:
		return "stencil"
	default:
		return string(kind)
	}
}

func Name(name string) TargetOption {
	return WithName(name)
}

// Deprecated: use Name.
func WithName(name string) TargetOption {
	return func(t *Target) {
		if t == nil {
			return
		}
		t.Name = strings.TrimSpace(name)
	}
}

func BaseDir(baseDir string) TargetOption {
	return WithBaseDir(baseDir)
}

// Deprecated: use BaseDir.
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
		t.applyPathDefaults()
	}
}

func ProjectDir(projectDir string) TargetOption {
	return WithProjectDir(projectDir)
}

// Deprecated: use ProjectDir.
func WithProjectDir(projectDir string) TargetOption {
	return func(t *Target) {
		if t == nil {
			return
		}
		projectDir = strings.TrimSpace(projectDir)
		if projectDir == "" {
			return
		}
		t.ProjectDir = projectDir
	}
}

func WorkspaceDir(workspaceDir string) TargetOption {
	return WithWorkspaceDir(workspaceDir)
}

// Deprecated: use WorkspaceDir.
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
		t.workspaceDirSet = true
	}
}

func OutputDir(outputDir string) TargetOption {
	return WithOutputDir(outputDir)
}

// Deprecated: use OutputDir.
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
		t.outputDirSet = true
	}
}

func Addr(addr string) TargetOption {
	return WithAddr(addr)
}

// Deprecated: use Addr.
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

func PollInterval(interval time.Duration) TargetOption {
	return WithPollInterval(interval)
}

// Deprecated: use PollInterval.
func WithPollInterval(interval time.Duration) TargetOption {
	return func(t *Target) {
		if t == nil || interval <= 0 {
			return
		}
		t.PollInterval = interval
	}
}

func Parts(parts ...hugo.Part) TargetOption {
	return WithParts(parts...)
}

// Deprecated: use Parts.
func WithParts(parts ...hugo.Part) TargetOption {
	return func(t *Target) {
		if t == nil || len(parts) == 0 {
			return
		}
		t.Parts = append(t.Parts, parts...)
	}
}

func Assets(parts ...hugo.Part) TargetOption {
	return WithAssets(parts...)
}

// Deprecated: use Assets.
func WithAssets(parts ...hugo.Part) TargetOption {
	return func(t *Target) {
		if t == nil || len(parts) == 0 {
			return
		}
		t.Assets = append(t.Assets, parts...)
	}
}

func Children(children ...*Target) TargetOption {
	return WithChildren(children...)
}

// Deprecated: use Children.
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

// Deprecated: use SiteConfig.
func WithSiteConfig(opts hugo.SiteOptions) TargetOption {
	return WithParts(hugo.SiteConfig(opts))
}

// Deprecated: use WithParts and hugo.Module().WithHugo(...).
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
type Page = web.Page
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
func Screen(name string, props any) templ.Component              { return web.Screen(name, props) }
func Element(name string, props any) templ.Component             { return web.Element(name, props) }

func Styles(baseDir ...string) Part                   { return hugo.Styles(baseDir...) }
func Components(baseDir ...string) Part               { return hugo.Components(baseDir...) }
func ConventionalAssets(baseDir ...string) Part       { return hugo.ConventionalAssets(baseDir...) }
func Content(baseDir string, includes ...string) Part { return hugo.Content(baseDir, includes...) }
func Layouts(baseDir string, includes ...string) Part { return hugo.Layouts(baseDir, includes...) }
func SiteConfig(opts SiteOptions) Part                { return hugo.SiteConfig(opts) }
func CSS(src hugo.AssetSource) Part                   { return hugo.CSS(src) }
func TailwindCSS(src hugo.AssetSource) Part           { return hugo.TailwindCSS(src) }
func JS(src hugo.AssetSource) Part                    { return hugo.JS(src) }
func WithCSS(names ...string) Part                    { return hugo.WithCSS(names...) }
func WithJS(names ...string) Part                     { return hugo.WithJS(names...) }
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return ExecuteContext(ctx, target)
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
	if t != nil && t.Kind == TargetArtifact {
		return t.artifactRegistry(parentCtx)
	}
	registry := cli.NewRegistry()
	if t == nil {
		return registry
	}

	runCmd := cli.Activity(
		"run",
		func(inv *cli.Invocation) runInput {
			return runInput{
				OutputDir: stringParam(inv, t.outputDir(), "output", "o"),
				Addr:      stringParam(inv, t.Addr, "addr", "a"),
			}
		},
		func(ctx cli.Context[runInput]) cli.Result {
			cfg := Context{
				Mode:      ModeRun,
				OutputDir: ctx.Data().OutputDir,
				Addr:      ctx.Data().Addr,
			}
			if err := t.execute(parentCtx, cfg); err != nil {
				return cli.Error(err.Error())
			}
			return cli.Done()
		},
		cli.WithHelp[runInput]("serve the already built target"),
	)
	cli.RegisterActivity(registry, runCmd)

	buildCmd := cli.Activity(
		"build",
		func(inv *cli.Invocation) buildInput {
			return buildInput{
				WorkspaceDir: stringParam(inv, t.workspaceDir(), "workspace", "w"),
				OutputDir:    stringParam(inv, t.outputDir(), "output", "o"),
				Command:      strings.TrimSpace(inv.Arg(0)),
			}
		},
		func(ctx cli.Context[buildInput]) cli.Result {
			cfg := Context{
				Mode:         ModeBuild,
				WorkspaceDir: ctx.Data().WorkspaceDir,
				OutputDir:    ctx.Data().OutputDir,
				Command:      ctx.Data().Command,
			}
			if err := t.execute(parentCtx, cfg); err != nil {
				return cli.Error(err.Error())
			}
			return cli.Done()
		},
		cli.WithHelp[buildInput]("materialize the target graph"),
	)
	cli.RegisterActivity(registry, buildCmd)

	installCmd := cli.Activity(
		"install",
		func(inv *cli.Invocation) installInput {
			return installInput{
				WorkspaceDir: stringParam(inv, t.workspaceDir(), "workspace", "w"),
				OutputDir:    stringParam(inv, t.outputDir(), "output", "o"),
			}
		},
		func(ctx cli.Context[installInput]) cli.Result {
			cfg := Context{
				Mode:         ModeInstall,
				WorkspaceDir: ctx.Data().WorkspaceDir,
				OutputDir:    ctx.Data().OutputDir,
			}
			if err := t.execute(parentCtx, cfg); err != nil {
				return cli.Error(err.Error())
			}
			return cli.Done()
		},
		cli.WithHelp[installInput]("materialize target metadata and dependencies"),
	)
	cli.RegisterActivity(registry, installCmd)

	devCmd := cli.Activity(
		"dev",
		func(inv *cli.Invocation) devInput {
			poll := stringParam(inv, t.pollIntervalString(), "poll", "p")
			interval, err := time.ParseDuration(poll)
			if err != nil {
				interval = t.PollInterval
			}
			return devInput{
				WorkspaceDir: stringParam(inv, t.workspaceDir(), "workspace", "w"),
				OutputDir:    stringParam(inv, t.outputDir(), "output", "o"),
				Addr:         stringParam(inv, t.Addr, "addr", "a"),
				PollInterval: interval,
				Command:      strings.TrimSpace(inv.Arg(0)),
			}
		},
		func(ctx cli.Context[devInput]) cli.Result {
			cfg := Context{
				Mode:         ModeDev,
				WorkspaceDir: ctx.Data().WorkspaceDir,
				OutputDir:    ctx.Data().OutputDir,
				Addr:         ctx.Data().Addr,
				PollInterval: ctx.Data().PollInterval,
				Command:      ctx.Data().Command,
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

func (t *Target) artifactRegistry(parentCtx context.Context) *cli.Registry {
	registry := cli.NewRegistry()
	if t == nil {
		return registry
	}

	installCmd := cli.Activity(
		"install",
		func(inv *cli.Invocation) installInput {
			return installInput{
				WorkspaceDir: stringParam(inv, t.workspaceDir(), "workspace", "w"),
				OutputDir:    stringParam(inv, t.outputDir(), "output", "o"),
			}
		},
		func(ctx cli.Context[installInput]) cli.Result {
			cfg := Context{
				Mode:         ModeInstall,
				WorkspaceDir: ctx.Data().WorkspaceDir,
				OutputDir:    ctx.Data().OutputDir,
			}
			if err := t.execute(parentCtx, cfg); err != nil {
				return cli.Error(err.Error())
			}
			return cli.Done()
		},
		cli.WithHelp[installInput]("install the artifact metadata and dependencies"),
	)
	cli.RegisterActivity(registry, installCmd)

	buildCmd := cli.Activity(
		"build",
		func(inv *cli.Invocation) buildInput {
			return buildInput{
				WorkspaceDir: stringParam(inv, t.workspaceDir(), "workspace", "w"),
				OutputDir:    stringParam(inv, t.outputDir(), "output", "o"),
				Command:      strings.TrimSpace(inv.Arg(0)),
			}
		},
		func(ctx cli.Context[buildInput]) cli.Result {
			cfg := Context{
				Mode:         ModeBuild,
				WorkspaceDir: ctx.Data().WorkspaceDir,
				OutputDir:    ctx.Data().OutputDir,
				Command:      ctx.Data().Command,
			}
			if err := t.execute(parentCtx, cfg); err != nil {
				return cli.Error(err.Error())
			}
			return cli.Done()
		},
		cli.WithHelp[buildInput]("build the artifact assets"),
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
				WorkspaceDir: stringParam(inv, t.workspaceDir(), "workspace", "w"),
				OutputDir:    stringParam(inv, t.outputDir(), "output", "o"),
				Addr:         stringParam(inv, t.Addr, "addr", "a"),
				PollInterval: interval,
				Command:      strings.TrimSpace(inv.Arg(0)),
			}
		},
		func(ctx cli.Context[devInput]) cli.Result {
			cfg := Context{
				Mode:         ModeDev,
				WorkspaceDir: ctx.Data().WorkspaceDir,
				OutputDir:    ctx.Data().OutputDir,
				Addr:         ctx.Data().Addr,
				PollInterval: ctx.Data().PollInterval,
				Command:      ctx.Data().Command,
			}
			if err := t.execute(parentCtx, cfg); err != nil {
				return cli.Error(err.Error())
			}
			return cli.Done()
		},
		cli.WithHelp[devInput]("watch and rebuild the artifact assets"),
	)
	cli.RegisterActivity(registry, devCmd)

	return registry
}

func (t *Target) modeRegistry(parentCtx context.Context, mode Mode) *cli.Registry {
	registry := cli.NewRegistry()
	if t == nil {
		return registry
	}
	for _, child := range t.Children {
		if child == nil {
			continue
		}
		childTarget := child.withInheritedParts(t.directParts(), t.projectDir())
		if childTarget == nil {
			continue
		}
		commandName := strings.TrimSpace(childTarget.Name)
		if commandName == "" {
			commandName = defaultTargetName(childTarget.Kind)
		}
		selected := childTarget
		cmdMode := mode
		switch mode {
		case ModeDev:
			activity := cli.Activity(
				commandName,
				func(inv *cli.Invocation) devInput {
					poll := stringParam(inv, selected.pollIntervalString(), "poll", "p")
					interval, err := time.ParseDuration(poll)
					if err != nil {
						interval = selected.PollInterval
					}
					return devInput{
						WorkspaceDir: stringParam(inv, selected.workspaceDir(), "workspace", "w"),
						OutputDir:    stringParam(inv, selected.outputDir(), "output", "o"),
						Addr:         stringParam(inv, selected.Addr, "addr", "a"),
						PollInterval: interval,
					}
				},
				func(ctx cli.Context[devInput]) cli.Result {
					execCtx := Context{
						Mode:         cmdMode,
						WorkspaceDir: ctx.Data().WorkspaceDir,
						OutputDir:    ctx.Data().OutputDir,
						Addr:         ctx.Data().Addr,
						PollInterval: ctx.Data().PollInterval,
					}
					if err := selected.execute(parentCtx, execCtx); err != nil {
						return cli.Error(err.Error())
					}
					return cli.Done()
				},
				cli.WithHelp[devInput](modeHelp(mode, commandName)),
			)
			cli.RegisterActivity(registry, activity)
		default:
			activity := cli.Activity(
				commandName,
				func(inv *cli.Invocation) buildInput {
					return buildInput{
						WorkspaceDir: stringParam(inv, selected.workspaceDir(), "workspace", "w"),
						OutputDir:    stringParam(inv, selected.outputDir(), "output", "o"),
					}
				},
				func(ctx cli.Context[buildInput]) cli.Result {
					execCtx := Context{
						Mode:         cmdMode,
						WorkspaceDir: ctx.Data().WorkspaceDir,
						OutputDir:    ctx.Data().OutputDir,
					}
					if err := selected.execute(parentCtx, execCtx); err != nil {
						return cli.Error(err.Error())
					}
					return cli.Done()
				},
				cli.WithHelp[buildInput](modeHelp(mode, commandName)),
			)
			cli.RegisterActivity(registry, activity)
		}
	}
	return registry
}

func modeHelp(mode Mode, targetName string) string {
	switch mode {
	case ModeInstall:
		return "install the artifact metadata and dependencies"
	case ModeDev:
		return "run " + targetName + " in dev mode"
	default:
		return "build " + targetName
	}
}

func (t *Target) directParts() []hugo.Part {
	if t == nil {
		return nil
	}
	var parts []hugo.Part
	parts = append(parts, t.Assets...)
	parts = append(parts, t.Parts...)
	return parts
}

func (t *Target) withInheritedParts(inherited []hugo.Part, projectDir string) *Target {
	if t == nil {
		return nil
	}
	clone := *t
	clone.Assets = append([]hugo.Part{}, t.Assets...)
	clone.Parts = append(append([]hugo.Part{}, inherited...), t.Parts...)
	if strings.TrimSpace(clone.ProjectDir) == "" && strings.TrimSpace(projectDir) != "" {
		clone.ProjectDir = projectDir
	}
	if len(t.Children) > 0 {
		clone.Children = make([]*Target, 0, len(t.Children))
		nextInherited := append([]hugo.Part{}, clone.Assets...)
		nextInherited = append(nextInherited, clone.Parts...)
		for _, child := range t.Children {
			clone.Children = append(clone.Children, child.withInheritedParts(nextInherited, clone.projectDir()))
		}
	}
	return &clone
}

func (t *Target) execute(ctx context.Context, cfg Context) error {
	if t == nil {
		return errors.New("target is nil")
	}
	resolved := t.resolveExecution(cfg)
	switch t.Kind {
	case TargetArtifact:
		if cfg.Mode == ModeInstall {
			return t.installArtifact(ctx, resolved)
		}
		if cfg.Mode == ModeBuild {
			return t.executeArtifactBuild(ctx, resolved)
		}
		if cfg.Mode == ModeDev {
			return t.executeArtifactDev(ctx, resolved)
		}
		return errors.New("artifact targets are orchestrators and must be executed through their root commands")
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
		cfg.WorkspaceDir = t.workspaceDir()
	}
	if strings.TrimSpace(cfg.ProjectDir) == "" {
		cfg.ProjectDir = t.projectDir()
	}
	if strings.TrimSpace(cfg.OutputDir) == "" {
		cfg.OutputDir = t.outputDir()
	}
	if strings.TrimSpace(cfg.Addr) == "" {
		cfg.Addr = t.Addr
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = t.PollInterval
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
	app := hugo.NewApp(parts...)
	engine := app.Engine()
	switch cfg.Mode {
	case ModeRun:
		return engine.Serve(ctx, hugo.ServeConfig{
			Addr:      cfg.Addr,
			OutputDir: cfg.OutputDir,
		})
	case ModeInstall:
		return engine.Install(ctx, hugo.BuildConfig{
			ProjectDir:   cfg.ProjectDir,
			WorkspaceDir: cfg.WorkspaceDir,
			OutputDir:    cfg.OutputDir,
		})
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

func (t *Target) installArtifact(ctx context.Context, cfg Context) error {
	parts := t.collectParts()
	app := hugo.NewApp(parts...)
	engine := app.Engine()
	return engine.Install(ctx, hugo.BuildConfig{
		ProjectDir:   cfg.ProjectDir,
		WorkspaceDir: cfg.WorkspaceDir,
		OutputDir:    cfg.OutputDir,
	})
}

func (t *Target) executeArtifactBuild(ctx context.Context, cfg Context) error {
	parts := t.collectParts()
	app := hugo.NewApp(parts...)
	engine := app.Engine()
	_, err := engine.BuildAssets(ctx, hugo.BuildConfig{
		ProjectDir:   cfg.ProjectDir,
		WorkspaceDir: cfg.WorkspaceDir,
		OutputDir:    cfg.OutputDir,
	})
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.Command) == "" {
		return nil
	}
	return t.buildCommand(ctx, cfg.Command)
}

func (t *Target) executeArtifactDev(ctx context.Context, cfg Context) error {
	parts := t.collectParts()
	app := hugo.NewApp(parts...)
	engine := app.Engine()
	if strings.TrimSpace(cfg.Command) == "" {
		return engine.DevAssets(ctx, hugo.DevConfig{
			ProjectDir:   cfg.ProjectDir,
			Addr:         cfg.Addr,
			WorkspaceDir: cfg.WorkspaceDir,
			OutputDir:    cfg.OutputDir,
			PollInterval: cfg.PollInterval,
		})
	}

	devCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	cmd, err := t.startCommand(devCtx, cfg)
	if err != nil {
		return err
	}
	cmdErr := make(chan error, 1)
	go func() {
		cmdErr <- cmd.Wait()
	}()

	assetErr := make(chan error, 1)
	go func() {
		assetErr <- engine.DevAssets(devCtx, hugo.DevConfig{
			ProjectDir:   cfg.ProjectDir,
			Addr:         cfg.Addr,
			WorkspaceDir: cfg.WorkspaceDir,
			OutputDir:    cfg.OutputDir,
			PollInterval: cfg.PollInterval,
		})
	}()

	select {
	case err := <-cmdErr:
		cancel()
		if err != nil {
			return err
		}
		return nil
	case err := <-assetErr:
		cancel()
		stopCommand(cmd)
		<-cmdErr
		return err
	case <-ctx.Done():
		cancel()
		stopCommand(cmd)
		<-cmdErr
		return ctx.Err()
	}
}

func (t *Target) buildCommand(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	projectDir := t.projectDir()
	outputPath := filepath.Join(projectDir, "cmd", name, ".bin", name)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "go", "build", "-o", outputPath, "./"+filepath.ToSlash(filepath.Join("cmd", name)))
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (t *Target) startCommand(ctx context.Context, cfg Context) (*exec.Cmd, error) {
	name := strings.TrimSpace(cfg.Command)
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	args := []string{
		"run",
		"./" + filepath.ToSlash(filepath.Join("cmd", name)),
		"run",
		"--output=" + cfg.OutputDir,
		"--addr=" + cfg.Addr,
	}
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = t.projectDir()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func stopCommand(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM); err != nil {
		_ = cmd.Process.Kill()
	}
}

func (t *Target) executeHugo(ctx context.Context, cfg Context) error {
	parts := t.collectParts()
	app := hugo.NewAppWithDefaults(t.baseDir(), parts...)
	engine := app.Engine()
	switch cfg.Mode {
	case ModeInstall:
		return engine.Install(ctx, hugo.BuildConfig{
			ProjectDir:   cfg.ProjectDir,
			WorkspaceDir: cfg.WorkspaceDir,
			OutputDir:    cfg.OutputDir,
		})
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
	case ModeInstall:
		return engine.Install(ctx, hugo.BuildConfig{
			ProjectDir:   cfg.ProjectDir,
			WorkspaceDir: cfg.WorkspaceDir,
			OutputDir:    cfg.OutputDir,
		})
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

func (t *Target) projectDir() string {
	if t == nil {
		return ""
	}
	projectDir := strings.TrimSpace(t.ProjectDir)
	if projectDir != "" {
		return projectDir
	}
	return t.baseDir()
}

func (t *Target) applyPathDefaults() {
	if t == nil {
		return
	}
	if !t.workspaceDirSet {
		t.WorkspaceDir = web.DefaultWorkspaceDir(t.baseDir())
	}
	if !t.outputDirSet {
		t.OutputDir = web.DefaultOutputDir(t.baseDir())
	}
}

func (t *Target) workspaceDir() string {
	if t == nil {
		return ""
	}
	if strings.TrimSpace(t.WorkspaceDir) != "" {
		return t.WorkspaceDir
	}
	return web.DefaultWorkspaceDir(t.baseDir())
}

func (t *Target) outputDir() string {
	if t == nil {
		return ""
	}
	if strings.TrimSpace(t.OutputDir) != "" {
		return t.OutputDir
	}
	return web.DefaultOutputDir(t.baseDir())
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
