package stack

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cleanstartup/stack/web"
)

type testTailwindInclude struct{}

func (testTailwindInclude) StackTailwindInclude() {}

func TestModuleRecordsCallerPackageDir(t *testing.T) {
	bundle := Bundle()
	if bundle.rootDir() == "" {
		t.Fatal("expected module root dir")
	}
	if filepath.Base(bundle.rootDir()) != "stack" {
		t.Fatalf("expected stack package dir, got %s", bundle.rootDir())
	}
}

func TestBundleWebAppCLIHelp(t *testing.T) {
	app := web.NewApp(Bundle().partsFor(webAppFeatures{})...)
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

func TestBundleFeatureGating(t *testing.T) {
	part := gatedPart{part: CSS(nil), tailwind: true}
	bundle := &bundle{parts: []web.Part{part}, root: t.TempDir()}

	if got := bundle.partsFor(webAppFeatures{}); len(got) != 0 {
		t.Fatalf("expected gated tailwind part to be skipped, got %d parts", len(got))
	}
	if got := bundle.partsFor(webAppFeatures{tailwind: true}); len(got) != 1 {
		t.Fatalf("expected gated tailwind part to be included, got %d parts", len(got))
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

	bundle := &bundle{
		root: root,
		parts: []web.Part{
			WithTailwindStyles(stylesDir),
			WithStencilComponents(componentsDir),
		},
	}
	app := web.NewApp(bundle.partsFor(webAppFeatures{tailwind: true, stencil: true})...)
	manifest := app.Engine().Builder().Manifest()

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
