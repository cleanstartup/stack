package web

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCSSReturnsTailwindBundleRef(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cssPath := filepath.Join(tmp, "site.css")
	if err := os.WriteFile(cssPath, []byte("body { color: red; }"), 0o644); err != nil {
		t.Fatal(err)
	}

	builder := NewBuilder()
	ref := builder.CSS(FromFile(cssPath))

	if ref.Kind != AssetKindCSS {
		t.Fatalf("expected css bundle kind, got %v", ref.Kind)
	}
	if ref.URL() != "/assets/css/app/app.css" {
		t.Fatalf("expected bundled css url, got %q", ref.URL())
	}
}

func TestBuildWritesTailwindBundle(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "module-assets")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cssPath := filepath.Join(sourceDir, "style.css")
	if err := os.WriteFile(cssPath, []byte("body { color: red; }"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := New(ModuleFunc(func(b *Builder) {
		b.CSS(FromFile(cssPath))
	}))
	result, err := app.Build(context.Background(), BuildConfig{
		WorkspaceDir: filepath.Join(tmp, "workspace"),
		OutputDir:    filepath.Join(tmp, "public"),
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	bundlePath := filepath.Join(result.OutputDir, "assets", "css", "app", "app.css")
	content, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatalf("expected css bundle at %s: %v", bundlePath, err)
	}
	if len(content) == 0 {
		t.Fatalf("expected non-empty css bundle")
	}
	if _, err := os.Stat(filepath.Join(result.OutputDir, "assets", "css", "style")); err == nil {
		t.Fatalf("expected raw css source not to be copied into output")
	}
}
