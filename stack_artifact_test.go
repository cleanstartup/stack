package stack

import (
	"context"
	"path/filepath"
	"os"
	"strings"
	"testing"
)

func TestArtifactBuildIsRootCommand(t *testing.T) {
	rootDir := t.TempDir()
	stylesDir := filepath.Join(rootDir, "styles")
	componentsDir := filepath.Join(rootDir, "components")
	writeTestFile(t, filepath.Join(stylesDir, "site.tailwind.css"), "@import \"tailwindcss\";\n")
	writeTestFile(t, filepath.Join(componentsDir, "demo-card.stencil.tsx"), "export function DemoCard() { return <div>demo-card</div>; }\n")

	target := Artifact(
		BaseDir(rootDir),
		OutputDir(filepath.Join(rootDir, ".assets")),
		Parts(
			Styles("styles"),
			Components("components"),
		),
	)

	registry := target.registry(context.Background())

	help := registry.Execute([]string{"build", "--help"})
	if help.ExitCode != 0 {
		t.Fatalf("expected help exit 0, got %d", help.ExitCode)
	}
	if !strings.Contains(help.Stdout, "build the artifact assets") {
		t.Fatalf("expected root build help, got %q", help.Stdout)
	}

}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
