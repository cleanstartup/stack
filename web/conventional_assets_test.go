package web

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConventionalAssetsDiscoverStyles(t *testing.T) {
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

	mustWrite("site.tailwind.css", "body{color:red}")

	app := newWebApp()
	ConventionalAssets(baseDir).Apply(app)

	if got := len(app.builder.Assets().Entries()); got != 0 {
		t.Fatalf("expected 0 direct asset entries, got %d", got)
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

func TestConventionalAssetsIgnoreGeneratedStaticAssets(t *testing.T) {
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

	mustWrite("site.tailwind.css", "body{color:red}")
	mustWrite("static/assets/css/app/app.css", "body{color:blue}")

	app := newWebApp()
	ConventionalAssets(baseDir).Apply(app)

	if got := len(app.builder.Styles().Inputs()); got != 1 {
		t.Fatalf("expected generated static assets to be ignored, got %d css inputs", got)
	}
}

func TestComponentsPreferHugoSiteRootOverParentModuleRoot(t *testing.T) {
	tmp := t.TempDir()
	parent := filepath.Join(tmp, "branding")
	siteDir := filepath.Join(parent, "site")
	if err := os.MkdirAll(siteDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "go.mod"), []byte("module github.com/cleanstartup/branding\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(siteDir, "hugo.toml"), []byte("title = \"branding\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "outside.stencil.tsx"), []byte("export const outside = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(siteDir, "inside.stencil.tsx"), []byte("export const inside = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := newWebApp()
	Components(siteDir).Apply(app)

	if got := len(app.builder.Components().Inputs()); got != 1 {
		t.Fatalf("expected 1 component input from site root, got %d", got)
	}
	if got := len(app.builder.Components().ScanPaths()); got != 1 {
		t.Fatalf("expected 1 scan path from site root, got %d", got)
	}
	if got := app.builder.Components().ScanPaths()[0]; got != siteDir {
		t.Fatalf("expected scan path %q, got %q", siteDir, got)
	}
}
