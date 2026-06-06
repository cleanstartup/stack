package web

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConventionalAssetsDiscoverTailwindFragments(t *testing.T) {
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
	mustWrite("assets/css/site.tailwind.css", "@layer components {.title {@apply text-3xl font-bold;}}")
	mustWrite("assets/js/site.js", "console.log('ok')")

	b := NewBuilder()
	ConventionalAssets(baseDir).apply(b)

	if got := len(b.Assets().Entries()); got != 2 {
		t.Fatalf("expected 2 direct asset entries, got %d", got)
	}
	if got := len(b.Styles().Inputs()); got != 1 {
		t.Fatalf("expected 1 tailwind input, got %d", got)
	}
	if got := len(b.Styles().ScanPaths()); got != 1 {
		t.Fatalf("expected 1 tailwind scan path, got %d", got)
	}
	if got := b.Styles().ScanPaths()[0]; got != baseDir {
		t.Fatalf("expected scan path %q, got %q", baseDir, got)
	}
}
