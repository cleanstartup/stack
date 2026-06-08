package web

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func TestTailwindInputUsesMirroredSourcePaths(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cssPath := filepath.Join(tmp, "site.css")
	if err := os.WriteFile(cssPath, []byte("@apply text-slate-900;\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := New(TailwindCSS(FromFile(cssPath)))
	cache := newTailwindWorkspace(filepath.Join(tmp, "tailwind-cache"))

	input, err := app.engine.tailwindInput(cache)
	if err != nil {
		t.Fatalf("tailwind input failed: %v", err)
	}

	mirroredPath := filepath.Join(cache.AssetDir(AssetKindCSS, assetID(cssPath)), filepath.Base(cssPath))
	if _, err := os.Stat(mirroredPath); err != nil {
		t.Fatalf("expected mirrored css source at %s: %v", mirroredPath, err)
	}
	if !strings.Contains(input, "@import \""+filepath.ToSlash(mirroredPath)+"\";") {
		t.Fatalf("expected mirrored css import in input, got %q", input)
	}
	if strings.Contains(input, "/* stack:") {
		t.Fatalf("did not expect inline stack marker in input, got %q", input)
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

func TestBuildUsesExplicitTailwindBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake shell binary helper is unix-only")
	}

	tmp := t.TempDir()
	cssPath := filepath.Join(tmp, "tailwind.css")
	if err := os.WriteFile(cssPath, []byte("@import \"tailwindcss\";\nbody { color: red; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	binaryPath := filepath.Join(tmp, "tailwind-fake.sh")
	script := "#!/bin/sh\nset -eu\ninput=\noutput=\nwhile [ $# -gt 0 ]; do\n  case \"$1\" in\n    -i) input=\"$2\"; shift 2 ;;\n    -o) output=\"$2\"; shift 2 ;;\n    --minify|--watch) shift ;;\n    *) shift ;;\n  esac\ndone\ncp \"$input\" \"$output\"\n"
	if err := os.WriteFile(binaryPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	app := New(TailwindCSS(FromFile(cssPath)))
	result, err := app.Build(context.Background(), BuildConfig{
		WorkspaceDir:   filepath.Join(tmp, "workspace"),
		OutputDir:      filepath.Join(tmp, "public"),
		TailwindBinary: binaryPath,
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	bundlePath := filepath.Join(result.OutputDir, "assets", "css", "app", "app.css")
	if _, err := os.Stat(bundlePath); err != nil {
		t.Fatalf("expected tailwind bundle at %s: %v", bundlePath, err)
	}
}
