package web

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewWithDefaultsAppliesStylesAndComponentsFromBaseDir(t *testing.T) {
	tmp := t.TempDir()
	cssDir := filepath.Join(tmp, "assets", "css")
	componentsDir := filepath.Join(tmp, "assets", "components")
	if err := os.MkdirAll(cssDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(componentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cssDir, "site.css"), []byte("body { color: red; }"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(componentsDir, "demo-card.tsx"), []byte("export const demo = true;\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewWithDefaults(tmp)
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
	if len(componentPaths) != 1 || componentPaths[0] != filepath.Join(tmp, "assets", "components") {
		t.Fatalf("expected components scan path %q, got %#v", filepath.Join(tmp, "assets", "components"), componentPaths)
	}
	if got := len(app.builder.Manifest().Scripts); got != 1 {
		t.Fatalf("expected one script bundle in manifest, got %d", got)
	}
	if got := app.builder.Manifest().Scripts[0].URL(); got != "/assets/js/way2go/way2go.esm.js" {
		t.Fatalf("expected stencil bundle url, got %q", got)
	}
}
