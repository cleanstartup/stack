package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLayoutRegistryMaterializesHugoModule(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	moduleRoot := filepath.Join(t.TempDir(), "module")
	if err := os.MkdirAll(filepath.Join(baseDir, "layouts", "site"), 0o755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "layouts", "site", "content.hugo.html"), []byte("layout"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	app := New()
	Layouts(baseDir, "layouts/**/*.hugo.html").Apply(app)

	mods, err := app.engine.layoutModules(moduleRoot)
	if err != nil {
		t.Fatalf("layoutModules failed: %v", err)
	}
	if len(mods) != 1 {
		t.Fatalf("expected 1 layout module, got %d", len(mods))
	}
	if !strings.Contains(mods[0].ImportPath, "stack/layouts") {
		t.Fatalf("expected layout import path, got %q", mods[0].ImportPath)
	}
	entries, err := os.ReadDir(filepath.Join(moduleRoot, "layouts"))
	if err != nil {
		t.Fatalf("read materialized layouts root: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one layout module dir, got %d", len(entries))
	}
	data, err := os.ReadFile(filepath.Join(moduleRoot, "layouts", entries[0].Name(), "layouts", "partials", "layouts", "site", "content.hugo.html"))
	if err != nil {
		t.Fatalf("read materialized layout: %v", err)
	}
	if string(data) != "layout" {
		t.Fatalf("expected layout content to be copied, got %q", string(data))
	}
}

func TestLayoutRegistrySourceChangedMatchesRepoRoot(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(baseDir, "layouts", "site"), 0o755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "layouts", "site", "content.hugo.html"), []byte("layout"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	r := NewLayoutRegistry()
	r.Add(baseDir, "layouts/**/*.hugo.html")

	if !r.SourceChanged(filepath.Join(baseDir, "layouts", "site", "content.hugo.html")) {
		t.Fatalf("expected repo-root layout change to be detected")
	}
}
