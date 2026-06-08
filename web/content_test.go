package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContentRegistryMaterializesHugoModule(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	moduleRoot := filepath.Join(t.TempDir(), "module")
	if err := os.MkdirAll(filepath.Join(baseDir, "components", "button"), 0o755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "components", "button", "showcase.md"), []byte("---\ntitle: Button\nlayout: documentation\ntype: showcase\n---\n\n<button>demo</button>\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	app := New()
	Content(baseDir, "components/**/showcase.md").Apply(app)

	mods, err := app.engine.contentModules(moduleRoot)
	if err != nil {
		t.Fatalf("contentModules failed: %v", err)
	}
	if len(mods) != 1 {
		t.Fatalf("expected 1 content module, got %d", len(mods))
	}
	if !strings.Contains(mods[0].ImportPath, "stack/content") {
		t.Fatalf("expected content import path, got %q", mods[0].ImportPath)
	}

	contentPath := filepath.Join(moduleRoot, "content")
	entries, err := os.ReadDir(contentPath)
	if err != nil {
		t.Fatalf("read content root: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one content module directory, got %d", len(entries))
	}
	moduleDir := filepath.Join(moduleRoot, "content", entries[0].Name())
	data, err := os.ReadFile(filepath.Join(moduleDir, "content", "components", "button", "showcase.md"))
	if err != nil {
		t.Fatalf("read materialized content: %v", err)
	}
	if !strings.Contains(string(data), "<button>demo</button>") {
		t.Fatalf("expected content to be copied, got %q", string(data))
	}
	cfg, err := os.ReadFile(filepath.Join(moduleDir, "hugo.toml"))
	if err != nil {
		t.Fatalf("read module config: %v", err)
	}
	if !strings.Contains(string(cfg), "unsafe = true") {
		t.Fatalf("expected module config to enable unsafe rendering, got %q", string(cfg))
	}
}

func TestContentRegistrySourceChangedMatchesRepoRoot(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(baseDir, "index.md"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	r := NewContentRegistry()
	r.Add(baseDir, "**/*.md")

	if !r.SourceChanged(filepath.Join(baseDir, "index.md")) {
		t.Fatalf("expected repo-root markdown change to be detected")
	}
}
