package web

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComponentsDiscoverStencilSources(t *testing.T) {
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

	mustWrite("assets/components/demo-card.tsx", "export const demo = true\n")
	mustWrite("assets/components/helpers/state.ts", "export const state = true\n")

	app := newWebApp()
	Components(baseDir).Apply(app)

	if got := len(app.builder.Components().Inputs()); got != 1 {
		t.Fatalf("expected 1 stencil input, got %d", got)
	}
	if got := len(app.builder.Components().ScanPaths()); got != 1 {
		t.Fatalf("expected 1 stencil scan path, got %d", got)
	}
	if got := app.builder.Components().ScanPaths()[0]; got != filepath.Join(baseDir, "assets", "components") {
		t.Fatalf("expected scan path %q, got %q", filepath.Join(baseDir, "assets", "components"), got)
	}
}
