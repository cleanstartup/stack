package web

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewAppWithDefaultsAppliesStylesAndComponentsFromBaseDir(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "site.tailwind.css"), []byte("body { color: red; }"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "demo-card.stencil.tsx"), []byte("export const demo = true;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "demo-copy.stencil.ts"), []byte("export const demo = true;\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewAppWithDefaults(tmp)
	if got := len(app.builder.Styles().Inputs()); got != 1 {
		t.Fatalf("expected one css input, got %d", got)
	}
	paths := app.builder.Styles().ScanPaths()
	if len(paths) != 1 || paths[0] != tmp {
		t.Fatalf("expected scan path %q, got %#v", tmp, paths)
	}
	if got := len(app.builder.Assets().Refs(AssetKindCSS)); got != 0 {
		t.Fatalf("expected no direct css refs in default styles, got %d", got)
	}
	if got := len(app.builder.Components().Inputs()); got != 1 {
		t.Fatalf("expected one stencil input, got %d", got)
	}
	componentPaths := app.builder.Components().ScanPaths()
	if len(componentPaths) != 1 || componentPaths[0] != tmp {
		t.Fatalf("expected components scan path %q, got %#v", tmp, componentPaths)
	}
	if got := len(app.builder.Manifest().Scripts); got != 1 {
		t.Fatalf("expected one script bundle in manifest, got %d", got)
	}
	if got := app.builder.Manifest().Scripts[0].URL(); got != "/assets/js/stack/stack.esm.js" {
		t.Fatalf("expected stencil bundle url, got %q", got)
	}
}

func TestNewHugoWithDefaultsKeepsSiteSetupExplicit(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "site.tailwind.css"), []byte("body { color: red; }"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "demo-card.stencil.tsx"), []byte("export const demo = true;\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewHugoWithDefaults(tmp)
	if got := len(app.builder.Styles().Inputs()); got != 0 {
		t.Fatalf("expected no implicit css inputs, got %d", got)
	}
	if got := len(app.builder.Components().Inputs()); got != 0 {
		t.Fatalf("expected no implicit stencil inputs, got %d", got)
	}
	if got := app.Target(); got != TargetHugo {
		t.Fatalf("expected hugo target, got %q", got)
	}
}
