package web

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewWithDefaultsAppliesStylesFromBaseDir(t *testing.T) {
	tmp := t.TempDir()
	cssDir := filepath.Join(tmp, "assets", "css")
	if err := os.MkdirAll(cssDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cssDir, "site.css"), []byte("body { color: red; }"), 0o644); err != nil {
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
}
