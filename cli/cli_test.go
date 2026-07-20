package cli_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/cleanstartup/stack/activity"
	"github.com/cleanstartup/stack/cli"
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

func TestDynamicSegmentExtraction(t *testing.T) {
	root := cli.NewRegistry()
	companies := root.GetOrCreateGroup("companies")
	selectorGroup := companies.GetOrCreateGroup("{selector}")

	selector := cli.Param("selector")
	show := cli.Activity(
		"show",
		func(inv *cli.Invocation) string {
			return inv.StringParam(selector)
		},
		func(ctx cli.Context[string]) cli.Result {
			return cli.Textf("show:%s", ctx.Data())
		},
	)
	cli.RegisterActivity(selectorGroup, show)

	res := root.Execute([]string{"companies", "21-analytics", "show"})
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", res.ExitCode, res.Stderr)
	}
	if res.Stdout != "show:21-analytics" {
		t.Fatalf("expected extracted selector, got %q", res.Stdout)
	}
}

func TestDynamicSegmentRemainingArgs(t *testing.T) {
	root := cli.NewRegistry()
	companies := root.GetOrCreateGroup("companies")
	selectorGroup := companies.GetOrCreateGroup("{selector}")

	selector := cli.Param("selector")
	set := cli.Activity(
		"set",
		func(inv *cli.Invocation) [3]string {
			return [3]string{inv.StringParam(selector), inv.Arg(0), inv.Arg(1)}
		},
		func(ctx cli.Context[[3]string]) cli.Result {
			d := ctx.Data()
			return cli.Textf("set:%s:%s=%s", d[0], d[1], d[2])
		},
	)
	cli.RegisterActivity(selectorGroup, set)

	res := root.Execute([]string{"companies", "21-analytics", "set", "linkedin", "https://example.com"})
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", res.ExitCode, res.Stderr)
	}
	if res.Stdout != "set:21-analytics:linkedin=https://example.com" {
		t.Fatalf("unexpected stdout: %q", res.Stdout)
	}
}

func TestStaticSegmentWinsOverDynamic(t *testing.T) {
	root := cli.NewRegistry()
	companies := root.GetOrCreateGroup("companies")
	selectorGroup := companies.GetOrCreateGroup("{selector}")

	cli.RegisterActivity(companies, cli.Simple("list", func(ctx activity.Context) cli.Result {
		return cli.Text("static-list")
	}))
	cli.RegisterActivity(selectorGroup, cli.Simple("list", func(ctx activity.Context) cli.Result {
		return cli.Text("dynamic-list")
	}))

	res := root.Execute([]string{"companies", "list"})
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", res.ExitCode, res.Stderr)
	}
	if res.Stdout != "static-list" {
		t.Fatalf("expected static command to win, got %q", res.Stdout)
	}

	res = root.Execute([]string{"companies", "_all", "list"})
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", res.ExitCode, res.Stderr)
	}
	if res.Stdout != "dynamic-list" {
		t.Fatalf("expected dynamic command for non-static selector, got %q", res.Stdout)
	}
}

func TestDynamicSegmentHelpShowsPlaceholder(t *testing.T) {
	root := cli.NewRegistry()
	companies := root.GetOrCreateGroup("companies")
	selectorGroup := companies.GetOrCreateGroup("{selector}")
	cli.RegisterActivity(selectorGroup, cli.Simple("show", func(ctx activity.Context) cli.Result { return cli.Done() }))

	res := root.Execute([]string{"companies", "--help"})
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", res.ExitCode)
	}
	if !strings.Contains(res.Stdout, "<selector>") {
		t.Fatalf("expected placeholder in help, got %q", res.Stdout)
	}
	if strings.Contains(res.Stdout, "21-analytics") {
		t.Fatalf("help must not enumerate concrete values, got %q", res.Stdout)
	}
}

func TestConflictingDynamicSegmentNamesPanic(t *testing.T) {
	root := cli.NewRegistry()
	companies := root.GetOrCreateGroup("companies")
	companies.GetOrCreateGroup("{selector}")

	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic on conflicting dynamic segment name")
		}
	}()

	companies.GetOrCreateGroup("{id}")
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

func TestStringParamsRepeatedWithEquals(t *testing.T) {
	r := cli.NewRegistry()
	with := cli.Param("with")

	a := cli.Activity(
		"sample",
		func(inv *cli.Invocation) []string {
			return inv.StringParams(with)
		},
		func(ctx cli.Context[[]string]) cli.Result {
			return cli.Text(strings.Join(ctx.Data(), ","))
		},
	)
	cli.RegisterActivity(r, a)

	res := r.Execute([]string{"sample", "--with=status:lead", "--with=country:ch"})
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", res.ExitCode, res.Stderr)
	}
	if res.Stdout != "status:lead,country:ch" {
		t.Fatalf("unexpected stdout: %q", res.Stdout)
	}
}

func TestStringParamsRepeatedWithSpace(t *testing.T) {
	r := cli.NewRegistry()
	with := cli.Param("with")

	a := cli.Activity(
		"sample",
		func(inv *cli.Invocation) []string {
			return inv.StringParams(with)
		},
		func(ctx cli.Context[[]string]) cli.Result {
			return cli.Text(strings.Join(ctx.Data(), ","))
		},
	)
	cli.RegisterActivity(r, a)

	res := r.Execute([]string{"sample", "--with", "status:lead", "--with", "country:ch"})
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", res.ExitCode, res.Stderr)
	}
	if res.Stdout != "status:lead,country:ch" {
		t.Fatalf("unexpected stdout: %q", res.Stdout)
	}
}

func TestStringParamsRepeatedAliases(t *testing.T) {
	r := cli.NewRegistry()
	with := cli.Param("with", "w")

	a := cli.Activity(
		"sample",
		func(inv *cli.Invocation) []string {
			return inv.StringParams(with)
		},
		func(ctx cli.Context[[]string]) cli.Result {
			return cli.Text(strings.Join(ctx.Data(), ","))
		},
	)
	cli.RegisterActivity(r, a)

	res := r.Execute([]string{"sample", "--w=status:lead", "--w=country:ch"})
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", res.ExitCode, res.Stderr)
	}
	if res.Stdout != "status:lead,country:ch" {
		t.Fatalf("unexpected stdout: %q", res.Stdout)
	}
}

func TestStringParamSingleValueCompatWithRepeatedFlag(t *testing.T) {
	r := cli.NewRegistry()
	with := cli.Param("with")

	a := cli.Activity(
		"sample",
		func(inv *cli.Invocation) string {
			return inv.StringParam(with)
		},
		func(ctx cli.Context[string]) cli.Result {
			return cli.Text(ctx.Data())
		},
	)
	cli.RegisterActivity(r, a)

	res := r.Execute([]string{"sample", "--with=status:lead", "--with=country:ch"})
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", res.ExitCode, res.Stderr)
	}
	if res.Stdout != "country:ch" {
		t.Fatalf("expected last value to win, got %q", res.Stdout)
	}
}

func TestStringParamsRequiredMissing(t *testing.T) {
	r := cli.NewRegistry()
	with := cli.Param("with")

	a := cli.Activity(
		"sample",
		func(inv *cli.Invocation) []string {
			return inv.StringParams(with, cli.Required())
		},
		func(ctx cli.Context[[]string]) cli.Result {
			return cli.Text(strings.Join(ctx.Data(), ","))
		},
	)
	cli.RegisterActivity(r, a)

	res := r.Execute([]string{"sample"})
	if res.ExitCode != 1 {
		t.Fatalf("expected exit 1, got %d", res.ExitCode)
	}
	if !strings.Contains(res.Stderr, "missing param 'with'") {
		t.Fatalf("unexpected stderr: %q", res.Stderr)
	}
}

func TestStringParamsOptionalMissingReturnsEmpty(t *testing.T) {
	r := cli.NewRegistry()
	with := cli.Param("with")

	a := cli.Activity(
		"sample",
		func(inv *cli.Invocation) int {
			return len(inv.StringParams(with))
		},
		func(ctx cli.Context[int]) cli.Result {
			return cli.Textf("count=%d", ctx.Data())
		},
	)
	cli.RegisterActivity(r, a)

	res := r.Execute([]string{"sample"})
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", res.ExitCode, res.Stderr)
	}
	if res.Stdout != "count=0" {
		t.Fatalf("expected empty result, got %q", res.Stdout)
	}
}

func TestStringParamsValidateStringFailsOnOneValue(t *testing.T) {
	r := cli.NewRegistry()
	with := cli.Param("with")

	a := cli.Activity(
		"sample",
		func(inv *cli.Invocation) []string {
			return inv.StringParams(with, cli.ValidateString(func(value string) error {
				if !strings.Contains(value, ":") {
					return errors.New("must contain ':'")
				}
				return nil
			}))
		},
		func(ctx cli.Context[[]string]) cli.Result {
			return cli.Text(strings.Join(ctx.Data(), ","))
		},
	)
	cli.RegisterActivity(r, a)

	res := r.Execute([]string{"sample", "--with=status:lead", "--with=broken"})
	if res.ExitCode != 1 {
		t.Fatalf("expected exit 1, got %d", res.ExitCode)
	}
	if !strings.Contains(res.Stderr, "must contain ':'") {
		t.Fatalf("unexpected stderr: %q", res.Stderr)
	}
}

func TestIntParamsRepeated(t *testing.T) {
	r := cli.NewRegistry()
	n := cli.Param("n")

	a := cli.Activity(
		"sample",
		func(inv *cli.Invocation) []int {
			return inv.IntParams(n)
		},
		func(ctx cli.Context[[]int]) cli.Result {
			d := ctx.Data()
			parts := make([]string, len(d))
			for idx, v := range d {
				parts[idx] = strconv.Itoa(v)
			}
			return cli.Text(strings.Join(parts, ","))
		},
	)
	cli.RegisterActivity(r, a)

	res := r.Execute([]string{"sample", "--n=1", "--n=2", "--n=3"})
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", res.ExitCode, res.Stderr)
	}
	if res.Stdout != "1,2,3" {
		t.Fatalf("unexpected stdout: %q", res.Stdout)
	}
}

func TestIntParamsOptionalMissingReturnsEmpty(t *testing.T) {
	r := cli.NewRegistry()
	n := cli.Param("n")

	a := cli.Activity(
		"sample",
		func(inv *cli.Invocation) int {
			return len(inv.IntParams(n))
		},
		func(ctx cli.Context[int]) cli.Result {
			return cli.Textf("count=%d", ctx.Data())
		},
	)
	cli.RegisterActivity(r, a)

	res := r.Execute([]string{"sample"})
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", res.ExitCode, res.Stderr)
	}
	if res.Stdout != "count=0" {
		t.Fatalf("expected empty result, got %q", res.Stdout)
	}
}

func TestIntParamsRequiredMissing(t *testing.T) {
	r := cli.NewRegistry()
	n := cli.Param("n")

	a := cli.Activity(
		"sample",
		func(inv *cli.Invocation) []int {
			return inv.IntParams(n, cli.RequiredInts())
		},
		func(ctx cli.Context[[]int]) cli.Result {
			return cli.Textf("count=%d", len(ctx.Data()))
		},
	)
	cli.RegisterActivity(r, a)

	res := r.Execute([]string{"sample"})
	if res.ExitCode != 1 {
		t.Fatalf("expected exit 1, got %d", res.ExitCode)
	}
	if !strings.Contains(res.Stderr, "missing param 'n'") {
		t.Fatalf("unexpected stderr: %q", res.Stderr)
	}
}

func TestIntParamsValidateIntsFailsOnOneValue(t *testing.T) {
	r := cli.NewRegistry()
	n := cli.Param("n")

	a := cli.Activity(
		"sample",
		func(inv *cli.Invocation) []int {
			return inv.IntParams(n, cli.ValidateInts(func(v int) error {
				if v <= 0 {
					return errors.New("must be > 0")
				}
				return nil
			}))
		},
		func(ctx cli.Context[[]int]) cli.Result {
			return cli.Textf("count=%d", len(ctx.Data()))
		},
	)
	cli.RegisterActivity(r, a)

	res := r.Execute([]string{"sample", "--n=1", "--n=-2"})
	if res.ExitCode != 1 {
		t.Fatalf("expected exit 1, got %d", res.ExitCode)
	}
	if !strings.Contains(res.Stderr, "must be > 0") {
		t.Fatalf("unexpected stderr: %q", res.Stderr)
	}
}
