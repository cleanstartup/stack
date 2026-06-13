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
	"github.com/cleanstartup/stack/activity"
	assetspkg "github.com/cleanstartup/stack/assets"
	"github.com/cleanstartup/stack/cli"
	tailwindpkg "github.com/cleanstartup/stack/tailwind"
	"github.com/cleanstartup/stack/web"
)

const defaultAddr = ":8080"

type Part = web.Part
type URIRef = activity.URIRef
type Page = web.Page
type AssetSource = web.AssetSource

type Module interface {
	Part
	WebApp(opts ...WebAppOption)

	namespace() string
	partsFor(features webAppFeatures) []web.Part
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
	parts []web.Part
	root  string
}

type activityPart struct {
	activity *web.WebActivity[struct{}]
}

type webAppConfig struct {
	features webAppFeatures
	parts    []web.Part
	assetsFS fs.FS
}

type embeddedAssets struct{ fs fs.FS }

func (e embeddedAssets) Apply(_ *web.WebApp) {}

// Assets returns a WebAppOption that serves the provided embedded filesystem
// as the asset root at runtime. Pass the result of fs.Sub on your go:embed FS.
func Assets(f fs.FS) web.Part { return embeddedAssets{fs: f} }

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
func Bundle(namespace string, parts ...web.Part) Module {
	return &bundle{
		ns:    strings.TrimSpace(namespace),
		parts: cloneParts(parts),
		root:  web.CallerDir(1),
	}
}

// Extend wraps a base Module with additional parts. Use for adding
// runtime-only concerns (e.g. HTTP mounts) that do not belong in Module().
func Extend(base Module, parts ...web.Part) Module {
	root := web.CallerDir(1)
	var merged []web.Part
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

func Compose(parts ...web.Part) web.Part { return web.Compose(parts...) }

func Activity(ref activity.URIRef, handler func(activity.Context) activity.Result, opts ...web.ActivityOption[struct{}]) Part {
	act := web.Activity(ref, handler, opts...)
	return activityPart{activity: act}
}

func WithStaticTitle(title string) web.ActivityOption[struct{}] { return web.WithStaticTitle(title) }
func Screen(name string, props any) templ.Component             { return web.Screen(name, props) }
func Element(name string, props any) templ.Component            { return web.Element(name, props) }

func NPMDependency(name, version string) Part    { return web.NPMDependency(name, version) }
func NPMDevDependency(name, version string) Part { return web.NPMDevDependency(name, version) }

func CSS(src AssetSource) Part                     { return web.CSS(src) }
func JS(src AssetSource) Part                      { return web.JS(src) }
func File(src AssetSource) Part                    { return web.File(src) }
func Mount(path string, handler http.Handler) Part { return web.Mount(path, handler) }

func (b *bundle) Apply(app *web.WebApp) {
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
		root = web.CallerDir(1)
	}
	commandDir := web.CallerDir(1)

	validateNamespaces(b)

	parts := append(b.partsFor(cfg.features), cfg.parts...)
	app := web.NewApp(parts...)
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

func (b *bundle) namespace() string {
	if b == nil {
		return ""
	}
	return b.ns
}

func (b *bundle) partsFor(features webAppFeatures) []web.Part {
	if b == nil {
		return nil
	}
	var out []web.Part
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

func (p activityPart) Apply(app *web.WebApp) {
	if app == nil || p.activity == nil {
		return
	}
	p.activity.Apply(app)
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
		case web.Part:
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

func (r dirSourceRegistration) Apply(app *web.WebApp) {
	app.RegisterDirSource(r.namespace, r.relPath, r.absPath)
}

func (b *bundle) filteredPart(part web.Part, features webAppFeatures) []web.Part {
	if part == nil {
		return nil
	}
	if m, ok := part.(Module); ok {
		return m.partsFor(features)
	}
	if dir, ok := part.(assetspkg.DirSource); ok {
		return b.expandDirSource(dir, features)
	}
	return []web.Part{part}
}

// expandDirSource routes a DirSource to active builders and registers it for
// install-time source syncing. CSS goes to Tailwind, TSX to Stencil.
func (b *bundle) expandDirSource(dir assetspkg.DirSource, features webAppFeatures) []web.Part {
	root := dir.AbsPath()
	parts := []web.Part{
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

func cloneParts(parts []web.Part) []web.Part {
	if len(parts) == 0 {
		return nil
	}
	out := make([]web.Part, 0, len(parts))
	out = append(out, parts...)
	return out
}

// validateNamespaces panics if the same non-empty namespace appears in more
// than one module in the composed tree.
func validateNamespaces(root Module) {
	seen := map[string]struct{}{}
	var walk func(part web.Part)
	walk = func(part web.Part) {
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

func (s lazyTailwindSource) Materialize(_ web.AssetWorkspace, _ web.AssetKind) ([]string, error) {
	return tailwindpkg.DiscoverStyles(s.baseDir), nil
}

func (s lazyTailwindSource) WatchPaths() []string {
	if s.baseDir == "" {
		return nil
	}
	return []string{s.baseDir}
}

func tailwindStylesPart(baseDir string) web.Part {
	if strings.TrimSpace(baseDir) == "" {
		return web.Compose()
	}
	return web.Compose(
		web.TailwindScan(baseDir),
		web.TailwindCSS(lazyTailwindSource{baseDir: baseDir}),
	)
}

func stencilComponentsPart(baseDir string) web.Part {
	if strings.TrimSpace(baseDir) == "" {
		return web.Compose()
	}
	return web.Compose(
		web.StencilScan(baseDir),
		web.Stencil(web.FromDir(baseDir)),
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

func newBundleWebAppCLI(app *web.WebApp, cfg bundleCLIConfig) *cli.Registry {
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
			if err := app.Engine().Serve(cfg.ctx, web.ServeConfig{
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
			if err := app.Engine().Install(cfg.ctx, web.BuildConfig{
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
			result, err := app.Engine().BuildAssets(cfg.ctx, web.BuildConfig{
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
			if err := runWebAppDev(cfg.ctx, app, cfg.commandDir, web.DevConfig{
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

func runWebAppDev(parent context.Context, app *web.WebApp, commandDir string, cfg web.DevConfig) error {
	if app == nil {
		return errors.New("web app is nil")
	}
	devCtx, cancel := context.WithCancel(parent)
	defer cancel()

	cmd, err := startCurrentCommand(devCtx, commandDir, cfg)
	if err != nil {
		return err
	}
	cmdErr := make(chan error, 1)
	go func() {
		cmdErr <- cmd.Wait()
	}()

	assetErr := make(chan error, 1)
	go func() {
		assetErr <- app.Engine().DevAssets(devCtx, cfg)
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
	case <-parent.Done():
		cancel()
		stopCommand(cmd)
		<-cmdErr
		return parent.Err()
	}
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

func startCurrentCommand(ctx context.Context, commandDir string, cfg web.DevConfig) (*exec.Cmd, error) {
	commandDir = strings.TrimSpace(commandDir)
	if commandDir == "" {
		return nil, errors.New("command dir is empty")
	}
	args := []string{
		"run",
		".",
		"run",
		"--output=" + cfg.OutputDir,
		"--addr=" + cfg.Addr,
	}
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = commandDir
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
