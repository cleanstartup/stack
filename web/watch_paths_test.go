package web

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFromFSCarriesWatchPaths(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "site.css"), "body { color: red; }")

	source := FromFS(os.DirFS(root), ".", root)
	watcher, ok := source.(WatchPathsProvider)
	if !ok {
		t.Fatalf("expected FromFS source to expose watch paths")
	}

	got := watcher.WatchPaths()
	if len(got) != 1 {
		t.Fatalf("expected one watch path, got %v", got)
	}
	if got[0] != root {
		t.Fatalf("expected watch path %q, got %q", root, got[0])
	}
}

func TestBuildEngineCollectsDependencyWatchPaths(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "site.css"), "body { color: blue; }")

	engine := NewBuildEngine(ModuleFunc(func(b *Builder) {
		b.CSS(FromFS(os.DirFS(root), ".", root))
	}))

	got := engine.watchPaths()
	if len(got) != 1 {
		t.Fatalf("expected one watch path, got %v", got)
	}
	if got[0] != root {
		t.Fatalf("expected watch path %q, got %q", root, got[0])
	}
}

func TestWithWatchPathsAddsExtraWatchRoots(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	inner := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "site.css"), "body { color: green; }")

	source := WithWatchPaths(FromFS(os.DirFS(root), ".", root), inner)
	watcher, ok := source.(WatchPathsProvider)
	if !ok {
		t.Fatalf("expected wrapped source to expose watch paths")
	}

	got := watcher.WatchPaths()
	if len(got) != 2 {
		t.Fatalf("expected two watch paths, got %v", got)
	}
	if got[0] != inner && got[1] != inner {
		t.Fatalf("expected watch paths to include %q, got %v", inner, got)
	}
	if got[0] != root && got[1] != root {
		t.Fatalf("expected watch paths to include %q, got %v", root, got)
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
