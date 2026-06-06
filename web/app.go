package web

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cleanstartup/way2go/cli"
)

type App struct {
	engine *BuildEngine
}

func New(mods ...Module) *App {
	return &App{engine: NewBuildEngine(mods...)}
}

func (a *App) Build(ctx context.Context, cfg BuildConfig) (*BuildResult, error) {
	if a == nil || a.engine == nil {
		return nil, fmt.Errorf("app is nil")
	}
	return a.engine.Build(ctx, cfg)
}

func (a *App) Serve(ctx context.Context, cfg ServeConfig) error {
	if a == nil || a.engine == nil {
		return fmt.Errorf("app is nil")
	}
	return a.engine.Serve(ctx, cfg)
}

func (a *App) Dev(ctx context.Context, cfg DevConfig) error {
	if a == nil || a.engine == nil {
		return fmt.Errorf("app is nil")
	}
	return a.engine.Dev(ctx, cfg)
}

func (a *App) CLI() *cli.Registry {
	r := cli.NewRegistry()

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
			return cli.Textf("built %d assets into %s", len(result.Assets), result.OutputDir)
		},
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
			}
		},
		func(ctx cli.Context[devCommandInput]) cli.Result {
			if err := a.Dev(context.Background(), DevConfig{
				Addr:         ctx.Data().Addr,
				WorkspaceDir: ctx.Data().WorkspaceDir,
				OutputDir:    ctx.Data().OutputDir,
				PollInterval: ctx.Data().PollInterval,
			}); err != nil {
				return cli.Error(err.Error())
			}
			return cli.Done()
		},
	)
	cli.RegisterActivity(r, devCmd)

	return r
}

func Serve(addr string, mods ...Module) error {
	app := New(mods...)
	return app.Serve(context.Background(), ServeConfig{
		Addr:      addr,
		OutputDir: defaultOutputDir,
	})
}

func Run(mods ...Module) error {
	app := New(mods...)
	registry := app.CLI()

	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"run"}
	}

	result := registry.Execute(args)
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
