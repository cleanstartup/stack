package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cleanstartup/stack/web"
)

func TestStencilWorkspacePackageIncludesBrandDependencies(t *testing.T) {
	t.Parallel()

	src := stencilPackageSource()
	for _, want := range []string{
		"\"altcha\"",
		"\"embla-carousel\"",
		"\"embla-carousel-auto-scroll\"",
		"\"htmx.org\"",
		"\"posthog-js\"",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("expected package source to include %s, got %s", want, src)
		}
	}
}

func TestTailwindWorkspacePackageIncludesCliDependency(t *testing.T) {
	t.Parallel()

	src := tailwindPackageSource()
	for _, want := range []string{
		"\"@tailwindcss/cli\"",
		"\"tailwindcss\"",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("expected package source to include %s, got %s", want, src)
		}
	}
}

func TestStencilWorkspaceTsConfigUsesModernLibs(t *testing.T) {
	t.Parallel()

	src := stencilTSConfigSource()
	for _, want := range []string{
		"\"dom.iterable\"",
		"\"es2020\"",
		"\"moduleResolution\": \"bundler\"",
		"\"target\": \"es2020\"",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("expected tsconfig source to include %s, got %s", want, src)
		}
	}
}

func TestMaterializeProjectFilesWritesTargetWorkspaceFiles(t *testing.T) {
	tmp := t.TempDir()
	projectDir := filepath.Join(tmp, "cmd", "site")
	workspaceRoot := filepath.Join(tmp, ".stack", "workspace")
	outputDir := filepath.Join(tmp, ".assets")

	cssPath := filepath.Join(tmp, "tailwind.css")
	if err := os.WriteFile(cssPath, []byte("@import \"tailwindcss\";\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tsPath := filepath.Join(tmp, "demo-card.tsx")
	if err := os.WriteFile(tsPath, []byte("export const Demo = () => null;\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := web.NewApp(web.TailwindCSS(web.FromFile(cssPath)), web.Stencil(web.FromFile(tsPath)))
	engine := NewEngine(app.Builder())
	if err := engine.materializeProjectFiles(projectDir, workspaceRoot, outputDir); err != nil {
		t.Fatalf("materialize failed: %v", err)
	}

	for _, name := range []string{"package.json", "package-lock.json", "tailwind.input.css", "stencil.config.ts", "tsconfig.json"} {
		if _, err := os.Stat(filepath.Join(projectDir, name)); err != nil {
			t.Fatalf("expected %s to exist: %v", name, err)
		}
	}

	tailwindBytes, err := os.ReadFile(filepath.Join(projectDir, "tailwind.input.css"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(tailwindBytes), filepath.ToSlash(cssPath)) {
		t.Fatalf("expected tailwind input to reference source css, got %s", string(tailwindBytes))
	}

	configBytes, err := os.ReadFile(filepath.Join(projectDir, "stencil.config.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(configBytes), "srcDir: '../..'") {
		t.Fatalf("expected config to point at source root, got %s", string(configBytes))
	}
	if !strings.Contains(string(configBytes), "dir: '../../.assets/assets/js'") {
		t.Fatalf("expected config to point output at asset dir, got %s", string(configBytes))
	}
	tsconfigBytes, err := os.ReadFile(filepath.Join(projectDir, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(tsconfigBytes), `"../.."`) {
		t.Fatalf("expected tsconfig to include source root, got %s", string(tsconfigBytes))
	}

	lockBytes, err := os.ReadFile(filepath.Join(projectDir, "package-lock.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"lockfileVersion": 3`, `"tailwindcss"`, `"@tailwindcss/cli"`, `"@stencil/core"`} {
		if !strings.Contains(string(lockBytes), want) {
			t.Fatalf("expected lockfile to include %s, got %s", want, string(lockBytes))
		}
	}
}
