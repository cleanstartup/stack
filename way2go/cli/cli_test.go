package cli_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cleanstartup/stack/way2go/activity"
	"github.com/cleanstartup/stack/way2go/cli"
)

func TestActivityDecodeAndExecute(t *testing.T) {
	r := cli.NewRegistry()
	xy := cli.Param("xy", "x", "X")

	a := cli.Activity(
		"sample",
		func(inv *cli.Invocation) int {
			return inv.IntParam(xy, func(v int) error {
				if v <= 0 {
					return errors.New("must be > 0")
				}
				return nil
			})
		},
		func(ctx cli.Context[int]) cli.Result {
			return cli.Textf("xy=%d", ctx.Data())
		},
	)
	cli.RegisterActivity(r, a)

	res := r.Execute([]string{"sample", "--xy=7"})
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", res.ExitCode)
	}
	if res.Stdout != "xy=7" {
		t.Fatalf("unexpected stdout: %q", res.Stdout)
	}
	if res.Stderr != "" {
		t.Fatalf("unexpected stderr: %q", res.Stderr)
	}
}

func TestParamPriority(t *testing.T) {
	r := cli.NewRegistry()
	xy := cli.Param("xy", "x", "X")

	a := cli.Activity(
		"sample",
		func(inv *cli.Invocation) int {
			return inv.IntParam(xy)
		},
		func(ctx cli.Context[int]) cli.Result {
			return cli.Textf("xy=%d", ctx.Data())
		},
	)
	cli.RegisterActivity(r, a)

	res := r.Execute([]string{"sample", "-X=3", "-x=2", "--xy=1"})
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", res.ExitCode)
	}
	if res.Stdout != "xy=1" {
		t.Fatalf("expected --xy to win, got %q", res.Stdout)
	}
}

func TestMount(t *testing.T) {
	root := cli.NewRegistry()
	wallet := cli.NewRegistry()

	a := cli.Activity(
		"show",
		func(inv *cli.Invocation) struct{} { return struct{}{} },
		func(ctx cli.Context[struct{}]) cli.Result {
			return cli.Text("wallet-show")
		},
	)
	cli.RegisterActivity(wallet, a)
	root.Mount("wallet", wallet)

	res := root.Execute([]string{"wallet", "show"})
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", res.ExitCode)
	}
	if res.Stdout != "wallet-show" {
		t.Fatalf("unexpected stdout: %q", res.Stdout)
	}
}

func TestGroup(t *testing.T) {
	root := cli.NewRegistry()
	wallet := root.Group("wallet")

	a := cli.Activity(
		"show",
		func(inv *cli.Invocation) struct{} { return struct{}{} },
		func(ctx cli.Context[struct{}]) cli.Result {
			return cli.Text("wallet-show")
		},
	)
	cli.RegisterActivity(wallet, a)

	res := root.Execute([]string{"wallet", "show"})
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", res.ExitCode)
	}
	if res.Stdout != "wallet-show" {
		t.Fatalf("unexpected stdout: %q", res.Stdout)
	}
}

func TestDecodeFailureSkipsHandler(t *testing.T) {
	r := cli.NewRegistry()
	xy := cli.Param("xy")
	handlerCalled := false

	a := cli.Activity(
		"sample",
		func(inv *cli.Invocation) int {
			return inv.IntParam(xy, func(v int) error {
				if v <= 0 {
					return errors.New("must be > 0")
				}
				return nil
			})
		},
		func(ctx cli.Context[int]) cli.Result {
			handlerCalled = true
			return cli.Textf("xy=%d", ctx.Data())
		},
	)
	cli.RegisterActivity(r, a)

	res := r.Execute([]string{"sample", "--xy=0"})
	if res.ExitCode != 1 {
		t.Fatalf("expected exit 1, got %d", res.ExitCode)
	}
	if handlerCalled {
		t.Fatalf("handler must not be called on decode failure")
	}
	if !strings.Contains(res.Stderr, "must be > 0") {
		t.Fatalf("expected validation error, got %q", res.Stderr)
	}
}

func TestDuplicateRegistrationPanics(t *testing.T) {
	r := cli.NewRegistry()
	a := cli.Simple("x", func(ctx activity.Context) cli.Result { return cli.Done() })
	cli.RegisterActivity(r, a)

	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic on duplicate registration")
		}
	}()

	cli.RegisterActivity(r, a)
}

func TestUnknownCommand(t *testing.T) {
	r := cli.NewRegistry()
	res := r.Execute([]string{"missing"})
	if res.ExitCode != 1 {
		t.Fatalf("expected exit 1, got %d", res.ExitCode)
	}
	if !strings.Contains(res.Stderr, "unknown command") {
		t.Fatalf("unexpected stderr: %q", res.Stderr)
	}
	if !strings.Contains(res.Stderr, "Use --help") {
		t.Fatalf("expected help output, got %q", res.Stderr)
	}
}

func TestHelpFlagsShowUsage(t *testing.T) {
	root := cli.NewRegistry()
	wallet := root.Group("wallet")

	cli.RegisterActivity(root, cli.Simple("run", func(ctx activity.Context) cli.Result { return cli.Done() }, cli.WithHelp[struct{}]("start the server")))
	cli.RegisterActivity(wallet, cli.Simple("show", func(ctx activity.Context) cli.Result { return cli.Done() }, cli.WithHelp[struct{}]("show wallet details")))

	res := root.Execute([]string{"--help"})
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", res.ExitCode)
	}
	if !strings.Contains(res.Stdout, "Subcommands:") || !strings.Contains(res.Stdout, "wallet") {
		t.Fatalf("expected root help, got %q", res.Stdout)
	}
	if !strings.Contains(res.Stdout, "Commands:") || !strings.Contains(res.Stdout, "run - start the server") {
		t.Fatalf("expected command list, got %q", res.Stdout)
	}
	if !strings.Contains(res.Stdout, "Use --help on any command") {
		t.Fatalf("expected generic help hint, got %q", res.Stdout)
	}

	res = root.Execute([]string{"wallet", "--help"})
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", res.ExitCode)
	}
	if !strings.Contains(res.Stdout, "Usage: <command> [subcommand] [flags]") {
		t.Fatalf("expected mount help, got %q", res.Stdout)
	}

	res = root.Execute([]string{"run", "--help"})
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", res.ExitCode)
	}
	if !strings.Contains(res.Stdout, "Usage: run [flags]") {
		t.Fatalf("expected command help, got %q", res.Stdout)
	}
	if !strings.Contains(res.Stdout, "start the server") {
		t.Fatalf("expected command summary, got %q", res.Stdout)
	}
}

func TestStringParamRequired(t *testing.T) {
	r := cli.NewRegistry()
	mnemonic := cli.Param("mnemonic")

	a := cli.Activity(
		"sample",
		func(inv *cli.Invocation) string {
			return inv.StringParam(mnemonic, cli.Required())
		},
		func(ctx cli.Context[string]) cli.Result {
			return cli.Text(ctx.Data())
		},
	)
	cli.RegisterActivity(r, a)

	res := r.Execute([]string{"sample"})
	if res.ExitCode != 1 {
		t.Fatalf("expected exit 1, got %d", res.ExitCode)
	}
	if !strings.Contains(res.Stderr, "missing param 'mnemonic'") {
		t.Fatalf("unexpected stderr: %q", res.Stderr)
	}
}

func TestStringParamValidateString(t *testing.T) {
	r := cli.NewRegistry()
	mnemonic := cli.Param("mnemonic")

	a := cli.Activity(
		"sample",
		func(inv *cli.Invocation) string {
			return inv.StringParam(mnemonic, cli.ValidateString(func(value string) error {
				if value != "ok" {
					return errors.New("must be ok")
				}
				return nil
			}))
		},
		func(ctx cli.Context[string]) cli.Result {
			return cli.Text(ctx.Data())
		},
	)
	cli.RegisterActivity(r, a)

	res := r.Execute([]string{"sample", "--mnemonic=nope"})
	if res.ExitCode != 1 {
		t.Fatalf("expected exit 1, got %d", res.ExitCode)
	}
	if !strings.Contains(res.Stderr, "must be ok") {
		t.Fatalf("unexpected stderr: %q", res.Stderr)
	}
}
