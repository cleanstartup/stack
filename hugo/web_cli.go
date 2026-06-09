package hugo

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/cleanstartup/stack/cli"
)

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
				OutputDir: stringParam(inv, defaultOutputDir, "output", "o"),
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
				WorkspaceDir: stringParam(inv, defaultWorkspaceDir, "workspace", "w"),
				OutputDir:    stringParam(inv, defaultOutputDir, "output", "o"),
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
			return cli.Textf("rendered %d files into %s", len(result.Assets), result.OutputDir)
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
			if ctx.Data().Child || !opts.useTempl {
				err = app.Dev(context.Background(), cfg)
			} else {
				err = app.DevWithTempl(context.Background(), cfg)
			}
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

