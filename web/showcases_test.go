package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShowcaseRegistryMaterializesHugoModule(t *testing.T) {
	tmp := t.TempDir()
	baseDir := filepath.Join(tmp, "branding")
	moduleRoot := filepath.Join(tmp, "workspace")
	if err := os.MkdirAll(filepath.Join(baseDir, "components", "button"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "go.mod"), []byte("module github.com/cleanstartup/branding\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "components", "button", "button.showcase.md"), []byte("---\ntitle: Button\nlayout: documentation\ntype: article\n---\n\n<button>demo</button>\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := newWebApp(TargetSite)
	Showcases(baseDir).Apply(app)

	mods, err := app.engine.showcaseModules(moduleRoot)
	if err != nil {
		t.Fatalf("showcaseModules failed: %v", err)
	}
	if len(mods) != 1 {
		t.Fatalf("expected 1 showcase module, got %d", len(mods))
	}
	if !strings.Contains(mods[0].ImportPath, "stack/showcases") {
		t.Fatalf("expected showcase import path, got %q", mods[0].ImportPath)
	}

	contentPath := filepath.Join(moduleRoot, "showcases")
	entries, err := os.ReadDir(contentPath)
	if err != nil {
		t.Fatalf("read showcase root: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected showcase module directory")
	}

	moduleDir := filepath.Join(moduleRoot, "showcases", entries[0].Name())
	data, err := os.ReadFile(filepath.Join(moduleDir, "content", "components", "button", "index.md"))
	if err != nil {
		t.Fatalf("read materialized showcase: %v", err)
	}
	if !strings.Contains(string(data), "<button>demo</button>") {
		t.Fatalf("expected showcase content to be copied, got %q", string(data))
	}
	cfg, err := os.ReadFile(filepath.Join(moduleDir, "hugo.toml"))
	if err != nil {
		t.Fatalf("read module config: %v", err)
	}
	if !strings.Contains(string(cfg), "unsafe = true") {
		t.Fatalf("expected showcase module config to enable unsafe rendering, got %q", string(cfg))
	}
}
