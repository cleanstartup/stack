package web

import (
	"context"
	"fmt"
	"time"

	"github.com/cleanstartup/stack/cli"
)

type webRuntime interface {
	Build(ctx context.Context, cfg BuildConfig) (*BuildResult, error)
	Serve(ctx context.Context, cfg ServeConfig) error
	Dev(ctx context.Context, cfg DevConfig) error
	CLI() *cli.Registry
}

type appRuntime struct {
	app *WebApp
}

func newWebRuntime(app *WebApp) webRuntime {
	return &appRuntime{app: app}
}

func (r *appRuntime) Build(ctx context.Context, cfg BuildConfig) (*BuildResult, error) {
	if r == nil || r.app == nil || r.app.engine == nil {
		return nil, fmt.Errorf("target is nil")
	}
	return r.app.engine.Build(ctx, cfg)
}

func (r *appRuntime) Serve(ctx context.Context, cfg ServeConfig) error {
	if r == nil || r.app == nil || r.app.engine == nil {
		return fmt.Errorf("target is nil")
	}
	return r.app.engine.Serve(ctx, cfg)
}

func (r *appRuntime) Dev(ctx context.Context, cfg DevConfig) error {
	if r == nil || r.app == nil || r.app.engine == nil {
		return fmt.Errorf("target is nil")
	}
	return r.app.engine.Dev(ctx, cfg)
}

func (r *appRuntime) CLI() *cli.Registry {
	return newWebCLI(r.app, webCLIConfig{
		runHelp:   "serve the already built web app",
		buildHelp: "materialize assets into the output directory",
		devHelp:   "run the templ supervisor and asset watch loop",
		useTempl:  true,
	})
}

type webCLIConfig struct {
	runHelp   string
	buildHelp string
	devHelp   string
	useTempl  bool
}

func newWebCLI(app *WebApp, opts webCLIConfig) *cli.Registry {
	r := cli.NewRegistry()
	if app == nil {
		return r
	}

	runCmd := cli.Activity(
		"run",
		func(inv *cli.Invocation) runCommandInput {
			return runCommandInput{
				Addr:      stringParam(inv, defaultAddr, "addr", "a"),
				OutputDir: stringParam(inv, DefaultOutputDir(app.baseDir), "output", "o"),
			}
		},
		func(ctx cli.Context[runCommandInput]) cli.Result {
			if err := app.Serve(context.Background(), ServeConfig{
				Addr:      ctx.Data().Addr,
				OutputDir: ctx.Data().OutputDir,
			}); err != nil {
				return cli.Error(err.Error())
			}
			return cli.Done()
		},
		cli.WithHelp[runCommandInput](opts.runHelp),
	)
	cli.RegisterActivity(r, runCmd)

	buildCmd := cli.Activity(
		"build",
		func(inv *cli.Invocation) buildCommandInput {
			return buildCommandInput{
				WorkspaceDir: stringParam(inv, DefaultWorkspaceDir(app.baseDir), "workspace", "w"),
				OutputDir:    stringParam(inv, DefaultOutputDir(app.baseDir), "output", "o"),
			}
		},
		func(ctx cli.Context[buildCommandInput]) cli.Result {
			result, err := app.Build(context.Background(), BuildConfig{
				WorkspaceDir: ctx.Data().WorkspaceDir,
				OutputDir:    ctx.Data().OutputDir,
			})
			if err != nil {
				return cli.Error(err.Error())
			}
			return cli.Textf("built %d assets into %s", len(result.Assets), result.OutputDir)
		},
		cli.WithHelp[buildCommandInput](opts.buildHelp),
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
				WorkspaceDir: stringParam(inv, DefaultWorkspaceDir(app.baseDir), "workspace", "w"),
				OutputDir:    stringParam(inv, DefaultOutputDir(app.baseDir), "output", "o"),
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
			_ = ctx.Data().Child
			_ = opts.useTempl
			err := app.Dev(context.Background(), cfg)
			if err != nil {
				return cli.Error(err.Error())
			}
			return cli.Done()
		},
		cli.WithHelp[devCommandInput](opts.devHelp),
	)
	cli.RegisterActivity(r, devCmd)

	return r
}
