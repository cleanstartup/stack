package stack

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	assetspkg "github.com/cleanstartup/stack/assets"
	"github.com/cleanstartup/stack/way2go/activity"
	"github.com/cleanstartup/stack/way2go/cli"
	"github.com/cleanstartup/stack/way2go/param"
	"github.com/cleanstartup/stack/webasset"
)

type testTailwindInclude struct{}

func (testTailwindInclude) StackTailwindInclude() {}

func TestModuleRecordsCallerPackageDir(t *testing.T) {
	bundle := Bundle("test.module")
	if bundle.rootDir() == "" {
		t.Fatal("expected module root dir")
	}
	if filepath.Base(bundle.rootDir()) != "stack" {
		t.Fatalf("expected stack package dir, got %s", bundle.rootDir())
	}
}

func TestBundleWebAppCLIHelp(t *testing.T) {
	app := webasset.NewApp(Bundle("test.module").partsFor(webAppFeatures{})...)
	registry := newBundleWebAppCLI(app, bundleCLIConfig{
		projectDir:   t.TempDir(),
		commandDir:   t.TempDir(),
		workspaceDir: filepath.Join(t.TempDir(), ".stack", "workspace"),
		outputDir:    filepath.Join(t.TempDir(), ".assets"),
		addr:         defaultAddr,
	})

	result := registry.Execute([]string{"--help"})
	if result.ExitCode != 0 {
		t.Fatalf("expected help exit 0, got %d", result.ExitCode)
	}
	for _, command := range []string{"install", "build", "dev", "run"} {
		if !strings.Contains(result.Stdout, command) {
			t.Fatalf("expected help to include %q, got %q", command, result.Stdout)
		}
	}
}

func TestBundleCLIDynamicCommandSegment(t *testing.T) {
	selectorParam := param.String("selector")
	show := Activity(
		"companies.show",
		func(ctx activity.Context) activity.Result {
			return cli.Textf("show:%s", activity.Param(ctx, selectorParam))
		},
		WithCommand("companies", "{selector}", "show"),
	)
	list := Activity(
		"companies.list",
		func(ctx activity.Context) activity.Result {
			return cli.Text("list")
		},
		WithCommand("companies", "list"),
	)

	b := Bundle("test.crm", show, list).(*bundle)
	r := cli.NewRegistry()
	b.buildCLIRegistry(r)

	res := r.Execute([]string{"companies", "21-analytics", "show"})
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", res.ExitCode, res.Stderr)
	}
	if res.Stdout != "show:21-analytics" {
		t.Fatalf("expected extracted selector, got %q", res.Stdout)
	}

	res = r.Execute([]string{"companies", "list"})
	if res.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d: %s", res.ExitCode, res.Stderr)
	}
	if res.Stdout != "list" {
		t.Fatalf("expected static command to win over dynamic segment, got %q", res.Stdout)
	}
}

func TestBundleFeatureGating(t *testing.T) {
	root := t.TempDir()
	dir := assetspkg.DirSource{CallerDir: root, RelPath: "."}
	b := &bundle{parts: []Part{dir}, root: root}

	// DirSource always emits a registration part (for install-time source sync)
	// plus one part per active builder.
	if got := b.partsFor(webAppFeatures{}); len(got) != 1 {
		t.Fatalf("expected DirSource to expand to 1 (registration only), got %d", len(got))
	}
	if got := b.partsFor(webAppFeatures{tailwind: true}); len(got) != 2 {
		t.Fatalf("expected DirSource to expand to 2 parts (registration + tailwind), got %d", len(got))
	}
	if got := b.partsFor(webAppFeatures{tailwind: true, stencil: true}); len(got) != 3 {
		t.Fatalf("expected DirSource to expand to 3 parts (registration + 2 builders), got %d", len(got))
	}

	cfg := parseWebAppOptions(testTailwindInclude{})
	if !cfg.features.tailwind {
		t.Fatal("expected tailwind include marker to enable tailwind")
	}
}

func TestBuiltAssetsRegisterTheirWebAppOutputs(t *testing.T) {
	root := t.TempDir()
	stylesDir := filepath.Join(root, "styles")
	componentsDir := filepath.Join(root, "components")
	writeTestFile(t, filepath.Join(stylesDir, "site.tailwind.css"), "@import \"tailwindcss\";\n")
	writeTestFile(t, filepath.Join(componentsDir, "demo-card.stencil.tsx"), "export class DemoCard {}\n")

	b := &bundle{
		root: root,
		parts: []Part{
			assetspkg.DirSource{CallerDir: root, RelPath: "styles"},
			assetspkg.DirSource{CallerDir: root, RelPath: "components"},
		},
	}
	app := webasset.NewApp(b.partsFor(webAppFeatures{tailwind: true, stencil: true})...)
	manifest := app.Builder().Manifest()

	if got := len(manifest.Styles); got != 1 {
		t.Fatalf("expected one registered stylesheet, got %d", got)
	}
	if got := manifest.Styles[0].URL(); got != "/assets/css/app/app.css" {
		t.Fatalf("expected tailwind output URL, got %q", got)
	}
	if got := len(manifest.Scripts); got != 1 {
		t.Fatalf("expected one registered script, got %d", got)
	}
	if got := manifest.Scripts[0].URL(); got != "/assets/js/stack/stack.esm.js" {
		t.Fatalf("expected stencil output URL, got %q", got)
	}
}

func TestBuildCurrentCommandWritesBinaryToCommandDir(t *testing.T) {
	commandDir := t.TempDir()
	writeTestFile(t, filepath.Join(commandDir, "go.mod"), "module example.com/current-command\n\ngo 1.25.2\n")
	writeTestFile(t, filepath.Join(commandDir, "main.go"), "package main\n\nfunc main() {}\n")

	if err := buildCurrentCommand(context.Background(), commandDir); err != nil {
		t.Fatalf("build current command: %v", err)
	}
	binaryPath := filepath.Join(commandDir, ".bin", filepath.Base(commandDir))
	if info, err := os.Stat(binaryPath); err != nil || info.IsDir() {
		t.Fatalf("expected command binary at %s: %v", binaryPath, err)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
