package web

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCSSReturnsDirectAssetRef(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cssPath := filepath.Join(tmp, "site.css")
	if err := os.WriteFile(cssPath, []byte("body { color: red; }"), 0o644); err != nil {
		t.Fatal(err)
	}

	builder := NewBuilder()
	ref := builder.CSS(FromFile(cssPath))

	if ref.Kind != AssetKindCSS {
		t.Fatalf("expected css kind, got %v", ref.Kind)
	}
	if ref.URL() != "/assets/css/"+assetID(cssPath)+"/site.css" {
		t.Fatalf("expected direct css asset url, got %q", ref.URL())
	}
}

func TestTailwindReturnsBundleRef(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cssPath := filepath.Join(tmp, "tailwind.css")
	if err := os.WriteFile(cssPath, []byte("@import \"tailwindcss\";\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	builder := NewBuilder()
	ref := builder.TailwindCSS(FromFile(cssPath))

	if ref.Kind != AssetKindCSS {
		t.Fatalf("expected css kind, got %v", ref.Kind)
	}
	if ref.URL() != "/assets/css/app/app.css" {
		t.Fatalf("expected tailwind bundle url, got %q", ref.URL())
	}
}

func TestTailwindRegistryTracksWatchPaths(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	builder := NewBuilder()
	builder.TailwindScan(tmp)
	builder.TailwindCSS(FromFS(os.DirFS(tmp), ".", tmp))

	paths := builder.Styles().WatchPaths()
	if len(paths) == 0 {
		t.Fatalf("expected watch paths")
	}
}
