package web

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConventionalAssetsDiscoverStylesAndScripts(t *testing.T) {
	baseDir := t.TempDir()

	mustWrite := func(rel string, content string) {
		path := filepath.Join(baseDir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mustWrite("assets/css/site.css", "body{color:red}")
	mustWrite("assets/js/site.js", "console.log('ok')")

	app := newWebApp()
	ConventionalAssets(baseDir).Apply(app)

	if got := len(app.builder.Assets().Entries()); got != 1 {
		t.Fatalf("expected 1 direct asset entry, got %d", got)
	}
	if got := len(app.builder.Styles().Inputs()); got != 1 {
		t.Fatalf("expected 1 css input, got %d", got)
	}
	if got := len(app.builder.Styles().ScanPaths()); got != 1 {
		t.Fatalf("expected 1 tailwind scan path, got %d", got)
	}
	if got := app.builder.Styles().ScanPaths()[0]; got != baseDir {
		t.Fatalf("expected scan path %q, got %q", baseDir, got)
	}
}
