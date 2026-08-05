package stack

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/a-h/templ"
	assetspkg "github.com/cleanstartup/stack/assets"
	buildpkg "github.com/cleanstartup/stack/internal/build"
	tailwindpkg "github.com/cleanstartup/stack/internal/tailwind"
	"github.com/cleanstartup/stack/way2go/activity"
	"github.com/cleanstartup/stack/way2go/cli"
	wayweb "github.com/cleanstartup/stack/way2go/web"
	"github.com/cleanstartup/stack/webasset"
)

const defaultAddr = ":8080"

type Part = webasset.Part
type Page = wayweb.Page
type AssetSource = webasset.AssetSource

type Module interface {
	Part
	WebApp(opts ...WebAppOption)
	CLIApp()

	namespace() string
	partsFor(features webAppFeatures) []Part
	rootDir() string
}

type WebAppOption any

type webAppFeatures struct {
	tailwind bool
	stencil  bool
}

// tailwindInclude is satisfied by assets.Tailwind().
type tailwindInclude interface {
	StackTailwindInclude()
}

// stencilInclude is satisfied by assets.Stencil().
type stencilInclude interface {
	StackStencilInclude()
}

type bundle struct {
	ns    string
	parts []Part
	root  string
}

// ActivityDef is a transport-agnostic activity definition.
// Use WithView() to register it as a web route and WithCommand() to register
// it as a CLI command. Both can be set on the same definition.
type ActivityDef struct {
	id      string
	handler func(activity.Context) activity.Result
	crossMW []activity.ContextMiddleware
	help    string
	// web
	hasView  bool
	viewOpts []wayweb.ActivityOption[struct{}]
	// cli
	hasCommand bool
	cmdPath    []string
}

// ActivityOption configures an ActivityDef.
type ActivityOption func(*ActivityDef)

// WithView registers the activity as a web route. Optional web-specific opts
// (e.g. web.WithStaticTitle) can be passed here.
func WithView(opts ...wayweb.ActivityOption[struct{}]) ActivityOption {
	return func(a *ActivityDef) {
		a.hasView = true
		a.viewOpts = append(a.viewOpts, opts...)
	}
}

// WithCommand registers the activity as a CLI command. The path segments define
// the group hierarchy and command name, e.g. WithCommand("user", "show") →
// subcommand "show" under group "user".
func WithCommand(path ...string) ActivityOption {
	return func(a *ActivityDef) {
		a.hasCommand = true
		a.cmdPath = path
	}
}

// WithMiddleware attaches cross-target middleware to the activity.
// Applied in both web and CLI targets.
func WithMiddleware(mw ...activity.ContextMiddleware) ActivityOption {
	return func(a *ActivityDef) {
		a.crossMW = append(a.crossMW, mw...)
	}
}

// WithHelp sets the help text shown in CLI --help output.
func WithHelp(text string) ActivityOption {
	return func(a *ActivityDef) { a.help = text }
}

// Apply registers the activity as a web route if WithView was set.
// Implements Part — ignored by CLIApp.
func (a *ActivityDef) Apply(app *webasset.WebApp) {
	if a == nil || !a.hasView {
		return
	}
	opts := append([]wayweb.ActivityOption[struct{}]{
		wayweb.WithCrossMiddleware[struct{}](a.crossMW...),
	}, a.viewOpts...)
	wayweb.NewActivity(a.id, a.handler, opts...).Apply(app.Registrar())
}

// ID returns the activity ID, satisfying the URIRef-compatible interface for
// use with web.URI().
func (a *ActivityDef) ID() string {
	if a == nil {
		return ""
	}
	return a.id
}

type webAppConfig struct {
	features webAppFeatures
	parts    []Part
	assetsFS fs.FS
}

type embeddedAssets struct{ fs fs.FS }

func (e embeddedAssets) Apply(_ *webasset.WebApp) {}

// Assets returns a WebAppOption that serves the provided embedded filesystem
// as the asset root at runtime. Pass the result of fs.Sub on your go:embed FS.
func Assets(f fs.FS) Part { return embeddedAssets{fs: f} }

type buildInput struct {
	WorkspaceDir string
	OutputDir    string
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
}

// Bundle creates a Module with the given namespace and parts.
// The namespace uniquely identifies the module and is used for activity IDs,
// asset namespacing, logging, and analytics.
// Namespace collisions are detected at composition time (WebApp call).
func Bundle(namespace string, parts ...Part) Module {
	return &bundle{
		ns:    strings.TrimSpace(namespace),
		parts: cloneParts(parts),
		root:  webasset.CallerDir(1),
	}
}

// Extend wraps a base Module with additional parts. Use for adding
// runtime-only concerns (e.g. HTTP mounts) that do not belong in Module().
func Extend(base Module, parts ...Part) Module {
	root := webasset.CallerDir(1)
	var merged []Part
	if base != nil {
		root = strings.TrimSpace(base.rootDir())
		merged = append(merged, base)
	}
	merged = append(merged, parts...)
	return &bundle{
		parts: cloneParts(merged),
		root:  root,
	}
}

func Compose(parts ...Part) Part { return webasset.Compose(parts...) }

// Activity defines a transport-agnostic activity. Use WithView() and/or
// WithCommand() to declare which targets it participates in.
func Activity(id string, handler func(activity.Context) activity.Result, opts ...ActivityOption) *ActivityDef {
	a := &ActivityDef{id: strings.TrimSpace(id), handler: handler}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func Screen(name string, props any) templ.Component  { return wayweb.Screen(name, props) }
func Element(name string, props any) templ.Component { return wayweb.Element(name, props) }

func NPMDependency(name, version string) Part    { return webasset.NPMDependency(name, version) }
func NPMDevDependency(name, version string) Part { return webasset.NPMDevDependency(name, version) }

func CSS(src AssetSource) Part                     { return webasset.CSS(src) }
func JS(src AssetSource) Part                      { return webasset.JS(src) }
func File(src AssetSource) Part                    { return webasset.File(src) }
func Mount(path string, handler http.Handler) Part { return webasset.Mount(path, handler) }

// cliGroup wraps a cli.Command as a Part so it can be passed to Bundle().
// The web target ignores it via the no-op Apply; CLIApp() picks it up.
type cliGroup struct{ cmd cli.Command }

func (g cliGroup) Apply(_ *webasset.WebApp) {}

// Group adds a CLI command group to a Module. Ignored by WebApp targets.
func Group(name string, cmds ...cli.Command) Part {
	return cliGroup{cli.Group(name, cmds...)}
}

func (b *bundle) Apply(app *webasset.WebApp) {
	if b == nil || app == nil {
		return
	}
	for _, part := range b.parts {
		if part == nil {
			continue
		}
		part.Apply(app)
	}
}

func (b *bundle) WebApp(opts ...WebAppOption) {
	cfg := parseWebAppOptions(opts...)
	root := strings.TrimSpace(b.rootDir())
	if root == "" {
		root = webasset.CallerDir(1)
	}
	commandDir := webasset.CallerDir(1)

	validateNamespaces(b)

	parts := append(b.partsFor(cfg.features), cfg.parts...)
	app := webasset.NewApp(parts...)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	registry := newBundleWebAppCLI(app, bundleCLIConfig{
		ctx:          ctx,
		projectDir:   root,
		commandDir:   commandDir,
		outputDir:    filepath.Join(root, ".assets"),
		workspaceDir: filepath.Join(root, ".stack", "workspace"),
		addr:         defaultAddr,
		poll:         250 * time.Millisecond,
		assetsFS:     cfg.assetsFS,
	})
	runRegistry(registry, os.Args[1:])
}

func (b *bundle) CLIApp() {
	r := cli.NewRegistry()
	b.buildCLIRegistry(r)
	runRegistry(r, os.Args[1:])
}

func (b *bundle) buildCLIRegistry(r *cli.Registry) {
	if b == nil {
		return
	}
	for _, part := range b.parts {
		switch p := part.(type) {
		case cliGroup:
			cli.RegisterCommand(r, p.cmd)
		case *ActivityDef:
			if p.hasCommand {
				registerActivityCLI(r, p)
			}
		case *bundle:
			p.buildCLIRegistry(r)
		}
	}
}

func registerActivityCLI(r *cli.Registry, a *ActivityDef) {
	target := r
	for _, group := range a.cmdPath[:len(a.cmdPath)-1] {
		target = target.GetOrCreateGroup(group)
	}
	name := a.cmdPath[len(a.cmdPath)-1]
	handler := activity.ApplyMiddlewares(a.handler, a.crossMW)
	cmd := cli.Activity(
		name,
		func(inv *cli.Invocation) struct{} { return struct{}{} },
		func(ctx cli.Context[struct{}]) cli.Result {
			result := handler(ctx)
			if r, ok := result.(cli.Result); ok {
				return r
			}
			return cli.Done()
		},
		cli.WithHelp[struct{}](a.help),
	)
	cli.RegisterActivity(target, cmd)
}

func (b *bundle) namespace() string {
	if b == nil {
		return ""
	}
	return b.ns
}

func (b *bundle) partsFor(features webAppFeatures) []Part {
	if b == nil {
		return nil
	}
	var out []Part
	for _, part := range b.parts {
		out = append(out, b.filteredPart(part, features)...)
	}
	return out
}

func (b *bundle) rootDir() string {
	if b == nil {
		return ""
	}
	return b.root
}

func parseWebAppOptions(opts ...WebAppOption) webAppConfig {
	cfg := webAppConfig{}
	for _, opt := range opts {
		switch typed := opt.(type) {
		case nil:
			continue
		case tailwindInclude:
			cfg.features.tailwind = true
		case stencilInclude:
			cfg.features.stencil = true
		case embeddedAssets:
			cfg.assetsFS = typed.fs
		case Part:
			cfg.parts = append(cfg.parts, typed)
		}
	}
	return cfg
}

// dirSourceRegistration registers a DirSource entry into the Builder so that
// Install can sync sources to .sources/<namespace>/<relPath>/.
type dirSourceRegistration struct {
	namespace string
	relPath   string
	absPath   string
}

func (r dirSourceRegistration) Apply(app *webasset.WebApp) {
	app.RegisterDirSource(r.namespace, r.relPath, r.absPath)
}

func (b *bundle) filteredPart(part Part, features webAppFeatures) []Part {
	if part == nil {
		return nil
	}
	if m, ok := part.(Module); ok {
		return m.partsFor(features)
	}
	if dir, ok := part.(assetspkg.DirSource); ok {
		return b.expandDirSource(dir, features)
	}
	return []Part{part}
}

// expandDirSource routes a DirSource to active builders and registers it for
// install-time source syncing. CSS goes to Tailwind, TSX to Stencil.
func (b *bundle) expandDirSource(dir assetspkg.DirSource, features webAppFeatures) []Part {
	root := dir.AbsPath()
	parts := []Part{
		dirSourceRegistration{namespace: b.ns, relPath: dir.RelPath, absPath: root},
	}
	if features.tailwind {
		parts = append(parts, tailwindStylesPart(root))
	}
	if features.stencil {
		parts = append(parts, stencilComponentsPart(root))
	}
	return parts
}

func cloneParts(parts []Part) []Part {
	if len(parts) == 0 {
		return nil
	}
	out := make([]Part, 0, len(parts))
	out = append(out, parts...)
	return out
}

// validateNamespaces panics if the same non-empty namespace appears in more
// than one module in the composed tree.
func validateNamespaces(root Module) {
	seen := map[string]struct{}{}
	var walk func(part Part)
	walk = func(part Part) {
		if part == nil {
			return
		}
		if m, ok := part.(Module); ok {
			if ns := strings.TrimSpace(m.namespace()); ns != "" {
				if _, exists := seen[ns]; exists {
					panic(fmt.Sprintf("stack: duplicate module namespace %q", ns))
				}
				seen[ns] = struct{}{}
			}
			// walk the module's raw parts (not filtered) to catch all children
			if b, ok := part.(*bundle); ok {
				for _, p := range b.parts {
					walk(p)
				}
			}
		}
	}
	walk(root)
}

// --- internal builder helpers ---

type lazyTailwindSource struct{ baseDir string }

func (s lazyTailwindSource) ID() string { return "lazy-styles:" + s.baseDir }

func (s lazyTailwindSource) Materialize(_ webasset.AssetWorkspace, _ webasset.AssetKind) ([]string, error) {
	return tailwindpkg.DiscoverStyles(s.baseDir), nil
}

func (s lazyTailwindSource) WatchPaths() []string {
	if s.baseDir == "" {
		return nil
	}
	return []string{s.baseDir}
}

func tailwindStylesPart(baseDir string) Part {
	if strings.TrimSpace(baseDir) == "" {
		return webasset.Compose()
	}
	return webasset.Compose(
		webasset.TailwindScan(baseDir),
		webasset.TailwindCSS(lazyTailwindSource{baseDir: baseDir}),
	)
}

func stencilComponentsPart(baseDir string) Part {
	if strings.TrimSpace(baseDir) == "" {
		return webasset.Compose()
	}
	return webasset.Compose(
		webasset.StencilScan(baseDir),
		webasset.Stencil(webasset.FromDir(baseDir)),
	)
}

// --- CLI ---

type bundleCLIConfig struct {
	ctx          context.Context
	projectDir   string
	commandDir   string
	workspaceDir string
	outputDir    string
	addr         string
	poll         time.Duration
	assetsFS     fs.FS
}

func newBundleWebAppCLI(app *webasset.WebApp, cfg bundleCLIConfig) *cli.Registry {
	r := cli.NewRegistry()
	if app == nil {
		return r
	}
	if cfg.ctx == nil {
		cfg.ctx = context.Background()
	}

	runCmd := cli.Activity(
		"run",
		func(inv *cli.Invocation) runInput {
			return runInput{
				OutputDir: stringParam(inv, cfg.outputDir, "output", "o"),
				Addr:      stringParam(inv, cfg.addr, "addr", "a"),
			}
		},
		func(ctx cli.Context[runInput]) cli.Result {
			if err := buildpkg.NewEngine(app).Serve(cfg.ctx, buildpkg.ServeConfig{
				Addr:      ctx.Data().Addr,
				OutputDir: ctx.Data().OutputDir,
				AssetsFS:  cfg.assetsFS,
			}); err != nil {
				return cli.Error(err.Error())
			}
			return cli.Done()
		},
		cli.WithHelp[runInput]("serve the already built web app"),
	)
	cli.RegisterActivity(r, runCmd)

	installCmd := cli.Activity(
		"install",
		func(inv *cli.Invocation) installInput {
			return installInput{
				WorkspaceDir: stringParam(inv, cfg.workspaceDir, "workspace", "w"),
				OutputDir:    stringParam(inv, cfg.outputDir, "output", "o"),
			}
		},
		func(ctx cli.Context[installInput]) cli.Result {
			if err := buildpkg.NewEngine(app).Install(cfg.ctx, buildpkg.BuildConfig{
				ProjectDir:   cfg.projectDir,
				WorkspaceDir: ctx.Data().WorkspaceDir,
				OutputDir:    ctx.Data().OutputDir,
			}); err != nil {
				return cli.Error(err.Error())
			}
			return cli.Done()
		},
		cli.WithHelp[installInput]("install the web app metadata and dependencies"),
	)
	cli.RegisterActivity(r, installCmd)

	buildCmd := cli.Activity(
		"build",
		func(inv *cli.Invocation) buildInput {
			return buildInput{
				WorkspaceDir: stringParam(inv, cfg.workspaceDir, "workspace", "w"),
				OutputDir:    stringParam(inv, cfg.outputDir, "output", "o"),
			}
		},
		func(ctx cli.Context[buildInput]) cli.Result {
			result, err := buildpkg.NewEngine(app).BuildAssets(cfg.ctx, buildpkg.BuildConfig{
				ProjectDir:   cfg.projectDir,
				WorkspaceDir: ctx.Data().WorkspaceDir,
				OutputDir:    ctx.Data().OutputDir,
			})
			if err != nil {
				return cli.Error(err.Error())
			}
			if err := buildCurrentCommand(cfg.ctx, cfg.commandDir); err != nil {
				return cli.Error(err.Error())
			}
			return cli.Textf("built %d assets into %s", len(result.Assets), result.OutputDir)
		},
		cli.WithHelp[buildInput]("build web app assets and the current command binary"),
	)
	cli.RegisterActivity(r, buildCmd)

	devCmd := cli.Activity(
		"dev",
		func(inv *cli.Invocation) devInput {
			poll := stringParam(inv, cfg.poll.String(), "poll", "p")
			interval, err := time.ParseDuration(poll)
			if err != nil {
				interval = cfg.poll
			}
			return devInput{
				WorkspaceDir: stringParam(inv, cfg.workspaceDir, "workspace", "w"),
				OutputDir:    stringParam(inv, cfg.outputDir, "output", "o"),
				Addr:         stringParam(inv, cfg.addr, "addr", "a"),
				PollInterval: interval,
			}
		},
		func(ctx cli.Context[devInput]) cli.Result {
			if err := runWebAppDev(cfg.ctx, app, buildpkg.DevConfig{
				ProjectDir:   cfg.projectDir,
				WorkspaceDir: ctx.Data().WorkspaceDir,
				OutputDir:    ctx.Data().OutputDir,
				Addr:         ctx.Data().Addr,
				PollInterval: ctx.Data().PollInterval,
			}); err != nil {
				return cli.Error(err.Error())
			}
			return cli.Done()
		},
		cli.WithHelp[devInput]("run web app assets and runtime in dev mode"),
	)
	cli.RegisterActivity(r, devCmd)

	return r
}

func runWebAppDev(parent context.Context, app *webasset.WebApp, cfg buildpkg.DevConfig) error {
	if app == nil {
		return errors.New("web app is nil")
	}
	return buildpkg.NewEngine(app).Dev(parent, cfg)
}

func buildCurrentCommand(ctx context.Context, commandDir string) error {
	commandDir = strings.TrimSpace(commandDir)
	if commandDir == "" {
		return errors.New("command dir is empty")
	}
	name := filepath.Base(commandDir)
	outputPath := filepath.Join(commandDir, ".bin", name)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "go", "build", "-o", outputPath, ".")
	cmd.Dir = commandDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runRegistry(registry *cli.Registry, args []string) {
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
		fmt.Fprintln(os.Stderr, message)
		os.Exit(result.ExitCode)
	}
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
