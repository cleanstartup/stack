package stack

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	npmpkg "github.com/cleanstartup/stack/internal/npm"
	"github.com/cleanstartup/stack/plugin"
	"github.com/cleanstartup/stack/stackruntime"
	"github.com/cleanstartup/stack/webasset"
	"github.com/theway2go/way2go/cli"
)

// embedGenFileName is the generated file build writes into the consumer's
// command dir (CUP-30). Its content is deliberately fixed by Main itself
// (not templated per-caller): the `//go:build release` tag plus
// `stackruntime.Register` call are the entire contract between build and a
// release binary's Main — see stackruntime's own doc comment.
const embedGenFileName = "embed_gen.go"

// MainOption configures (*WebAppTarget).Main. Kept deliberately small
// (CUP-30's design doc, "möglichst deklarativ"): a bare .Main() must run
// with no options at all.
type MainOption func(*mainConfig)

type mainConfig struct {
	outputDir string
	addr      string
	npmDeps   map[string]string
}

// WithOutputDir overrides Main's asset scratch dir (dev's build output and
// build's `.assets` embed source are the same dir — see build's own doc
// comment for why that has to be true). Default: "<command dir>/.assets".
func WithOutputDir(dir string) MainOption {
	return func(c *mainConfig) { c.outputDir = strings.TrimSpace(dir) }
}

// WithAddr overrides the listen address Main's run verb (dev or release)
// binds to. Default: --addr flag, else $PORT, else ":8080".
func WithAddr(addr string) MainOption {
	return func(c *mainConfig) { c.addr = strings.TrimSpace(addr) }
}

// WithNPMDependency declares a real npm package Main must install into the
// WebApp's own module's project root before building — the smallest-first
// fix for the "lit stage declares its own npm dependency too late to inform
// an install that must finish before Build runs esbuild" gap (CUP-30's
// handoff notes): a caller composing e.g. ui.Module() (whose lit/ sources
// bare-import "lit" and Web Awesome) declares those same two packages here,
// exactly as cmd/showcase/main.go used to do by hand against its own local
// npmProject. Ignored under a release binary (Main never installs or
// builds there).
func WithNPMDependency(name, version string) MainOption {
	return func(c *mainConfig) {
		name, version = strings.TrimSpace(name), strings.TrimSpace(version)
		if name == "" || version == "" {
			return
		}
		if c.npmDeps == nil {
			c.npmDeps = map[string]string{}
		}
		c.npmDeps[name] = version
	}
}

type mainCliInput struct {
	OutputDir string
	Addr      string
}

// Main is the WebApp target's terminal, os.Args-driven lifecycle CLI
// (CUP-30 — unlike Build/Consume, deliberately in the os.Args-driven shape
// Build's own doc comment said was out of CUP-26's scope). A dev-compiled
// binary (the default, no build tag) gets "run" (npm install, in-process
// Build, serve from OutputDir) and "build" (Build once, freeze the result
// into a manifest, generate embed_gen.go, `go build -tags release` itself
// into a release binary). A release binary — recognized not by a build tag
// Main inspects, but by stackruntime.Registered() reporting an embedded
// asset tree, which only a release binary's generated init() ever populates
// — gets only "run": rehydrate the frozen manifest against the embedded
// tree and serve, no npm/StageContext/Stage involved.
//
// commandDir is resolved via webasset.CallerDir the same way the old
// bundle.WebApp/Extend already do (stack.go) — the directory of whatever
// file calls .Main(), i.e. the consumer's cmd/<name> package. build's
// `.assets`/embed_gen.go must live there, not the module root: `//go:embed`
// is package-local and can't reach outside its own package's directory
// (CUP-30's handoff notes).
func (t *WebAppTarget) Main(opts ...MainOption) {
	cfg := mainConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	commandDir := webasset.CallerDir(1)
	if cfg.outputDir == "" {
		cfg.outputDir = filepath.Join(commandDir, ".assets")
	}
	cfg.addr = resolveAddr(cfg.addr)

	if fsys, ok := stackruntime.Registered(); ok {
		runRegistry(t.releaseRegistry(fsys, cfg), os.Args[1:])
		return
	}
	runRegistry(t.devRegistry(commandDir, cfg), os.Args[1:])
}

func resolveAddr(explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	if v := strings.TrimSpace(os.Getenv("PORT")); v != "" {
		return ":" + v
	}
	return defaultAddr
}

// devRegistry offers run+build, the dev-compiled binary's full lifecycle
// (CUP-30's Subcommand-Definition: "Dev-Binary (Default-Compile, kein
// Tag)").
func (t *WebAppTarget) devRegistry(commandDir string, cfg mainConfig) *cli.Registry {
	r := cli.NewRegistry()

	runCmd := cli.Activity(
		"run",
		func(inv *cli.Invocation) mainCliInput {
			return mainCliInput{
				OutputDir: stringParam(inv, cfg.outputDir, "output", "o"),
				Addr:      stringParam(inv, cfg.addr, "addr", "a"),
			}
		},
		func(ctx cli.Context[mainCliInput]) cli.Result {
			data := ctx.Data()
			if err := t.serveDev(context.Background(), data.OutputDir, data.Addr, cfg.npmDeps); err != nil {
				return cli.Error(err.Error())
			}
			return cli.Done()
		},
		cli.WithHelp[mainCliInput]("ensure npm dependencies, build in-process (tailwind/lit), and serve"),
	)
	cli.RegisterActivity(r, runCmd)

	buildCmd := cli.Activity(
		"build",
		func(inv *cli.Invocation) mainCliInput {
			return mainCliInput{
				OutputDir: stringParam(inv, cfg.outputDir, "output", "o"),
			}
		},
		func(ctx cli.Context[mainCliInput]) cli.Result {
			outputDir := ctx.Data().OutputDir
			binPath, err := t.runBuild(context.Background(), commandDir, outputDir, cfg.npmDeps)
			if err != nil {
				return cli.Error(err.Error())
			}
			return cli.Textf("built release binary: %s", binPath)
		},
		cli.WithHelp[mainCliInput]("build assets and a self-contained release binary (go build -tags release)"),
	)
	cli.RegisterActivity(r, buildCmd)

	return r
}

// releaseRegistry offers only run (CUP-30's Subcommand-Definition:
// "Release-Binary (-tags release, von build erzeugt) ... build wird hier
// nicht angeboten") — the release binary never re-runs build, structurally:
// its registry has no build command compiled in for that verb to dispatch
// to, not a runtime check.
func (t *WebAppTarget) releaseRegistry(fsys fs.FS, cfg mainConfig) *cli.Registry {
	r := cli.NewRegistry()

	runCmd := cli.Activity(
		"run",
		func(inv *cli.Invocation) mainCliInput {
			return mainCliInput{Addr: stringParam(inv, cfg.addr, "addr", "a")}
		},
		func(ctx cli.Context[mainCliInput]) cli.Result {
			if err := t.serveRelease(fsys, ctx.Data().Addr); err != nil {
				return cli.Error(err.Error())
			}
			return cli.Done()
		},
		cli.WithHelp[mainCliInput]("serve the embedded, pre-built web app"),
	)
	cli.RegisterActivity(r, runCmd)

	return r
}

// serveDev is the dev "run" verb's body: ensure npm, Build in-process, then
// serve straight off OutputDir — the everyday inner-loop command (CUP-30
// Kernentscheidung #1: "In-Process-Build statt Dev-Zeit-Zweischritt").
func (t *WebAppTarget) serveDev(ctx context.Context, outputDir, addr string, npmDeps map[string]string) error {
	project, err := t.ensureNPM(ctx, npmDeps)
	if err != nil {
		return fmt.Errorf("stack: npm install: %w", err)
	}
	stageCtx := plugin.StageContext{
		OutputDir: outputDir,
		Mode:      plugin.ModeBuild,
		NPM:       project,
		Bin:       plugin.NewBinProvider(plugin.DefaultBinCacheDir()),
	}
	if err := t.Build(ctx, stageCtx); err != nil {
		return fmt.Errorf("stack: build: %w", err)
	}
	log.Printf("stack: serving on %s (%d style link(s), %d script link(s))", addr, len(t.links.Styles), len(t.links.Scripts))
	return http.ListenAndServe(addr, t.Handler())
}

// runBuild is the "build" verb's body: Build once, freeze the result into a
// manifest, generate embed_gen.go beside the consumer's main package, then
// compile a release binary from it — the host-native orchestrator step that
// makes "pre-built prod binary" real (CUP-30 Kernentscheidung #2/#5).
// Deliberately never invoked again by the binary it produces (PRD
// invariant, "keine Self-Recursion"): the release binary's registry
// (releaseRegistry) has no build command to call back into this.
func (t *WebAppTarget) runBuild(ctx context.Context, commandDir, outputDir string, npmDeps map[string]string) (string, error) {
	project, err := t.ensureNPM(ctx, npmDeps)
	if err != nil {
		return "", fmt.Errorf("stack: npm install: %w", err)
	}
	stageCtx := plugin.StageContext{
		OutputDir: outputDir,
		Mode:      plugin.ModeBuild,
		NPM:       project,
		Bin:       plugin.NewBinProvider(plugin.DefaultBinCacheDir()),
	}
	if err := t.Build(ctx, stageCtx); err != nil {
		return "", fmt.Errorf("stack: build: %w", err)
	}
	if err := writeManifest(outputDir, t.manifest); err != nil {
		return "", fmt.Errorf("stack: writing manifest: %w", err)
	}
	if err := writeEmbedGen(commandDir); err != nil {
		return "", fmt.Errorf("stack: generating %s: %w", embedGenFileName, err)
	}
	binPath, err := buildReleaseBinary(ctx, commandDir)
	if err != nil {
		return "", fmt.Errorf("stack: go build -tags release: %w", err)
	}
	return binPath, nil
}

// serveRelease is the release binary's only verb: rehydrate the manifest
// frozen into fsys by build's own Consume pass, then serve — no npm,
// StageContext, or Stage runs here (CUP-30 Subcommand-Definition).
func (t *WebAppTarget) serveRelease(fsys fs.FS, addr string) error {
	manifest, err := readManifest(fsys)
	if err != nil {
		return fmt.Errorf("stack: reading manifest: %w", err)
	}
	if err := t.rehydrate(fsys, manifest); err != nil {
		return fmt.Errorf("stack: rehydrating assets: %w", err)
	}
	log.Printf("stack: serving embedded build on %s (%d style link(s), %d script link(s))", addr, len(t.links.Styles), len(t.links.Scripts))
	return http.ListenAndServe(addr, t.Handler())
}

// ensureNPM writes package.json/package-lock.json for whatever
// WithNPMDependency declared into the WebApp's own module's project root
// (never an ingredient's — mirrors Build's own ProjectDir default, see its
// doc comment) and runs a real `npm install` there when needed, returning
// the same *npm.Project as stageCtx.NPM so a stage that declares its own
// dependency during Build (e.g. the lit stage's AddNPMDependencies) has
// somewhere real to record it, even though — as today — that's too late to
// affect this install.
func (t *WebAppTarget) ensureNPM(ctx context.Context, deps map[string]string) (*npmpkg.Project, error) {
	project := npmpkg.NewProject()
	for name, version := range deps {
		project.AddDependency(name, version)
	}
	if project.Empty() {
		return project, nil
	}
	projectDir := ""
	if t.module != nil {
		projectDir = strings.TrimSpace(t.module.rootDir())
	}
	if projectDir == "" {
		return project, nil
	}
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(project.PackageJSON()), 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(projectDir, "package-lock.json"), []byte(project.PackageLockJSON()), 0o644); err != nil {
		return nil, err
	}
	if err := npmpkg.EnsureDependencies(ctx, projectDir, nil); err != nil {
		return nil, err
	}
	return project, nil
}

// writeEmbedGen generates the file whose presence at compile time is the
// entire release-mode signal (stackruntime's doc comment): tagged
// `release`, so a default (tag-free) `go build` never sees it, it embeds
// build's own OutputDir (".assets", named to match the design's Manifest-
// Schema note) and registers it before main() ever runs.
func writeEmbedGen(commandDir string) error {
	const content = `// Code generated by (*stack.WebAppTarget).Main's build verb. DO NOT EDIT.

//go:build release

package main

import (
	"embed"
	"io/fs"

	"github.com/cleanstartup/stack/stackruntime"
)

//go:embed all:.assets
var stackAssets embed.FS

func init() {
	sub, err := fs.Sub(stackAssets, ".assets")
	if err != nil {
		panic(err)
	}
	stackruntime.Register(sub)
}
`
	return os.WriteFile(filepath.Join(commandDir, embedGenFileName), []byte(content), 0o644)
}

// buildReleaseBinary shells `go build -tags release`, plain compile with no
// self-recursion (the release binary's own registry never offers build
// back) — precedent: stack.go's buildCurrentCommand, same .bin/<name>
// output convention.
func buildReleaseBinary(ctx context.Context, commandDir string) (string, error) {
	name := filepath.Base(commandDir)
	outputPath := filepath.Join(commandDir, ".bin", name)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "go", "build", "-tags", "release", "-trimpath", "-o", outputPath, ".")
	cmd.Dir = commandDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return outputPath, nil
}
