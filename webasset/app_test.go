package webasset_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	buildpkg "github.com/cleanstartup/stack/internal/build"
	"github.com/cleanstartup/stack/webasset"
)

func TestBuildMaterializesAssets(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "module-assets")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cssPath := filepath.Join(sourceDir, "style.css")
	if err := os.WriteFile(cssPath, []byte("body{color:red}"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := webasset.NewApp(
		webasset.CSS(webasset.FromFile(cssPath)),
	)
	result, err := buildpkg.NewEngine(app).Build(context.Background(), buildpkg.BuildConfig{
		WorkspaceDir: filepath.Join(tmp, "workspace"),
		OutputDir:    filepath.Join(tmp, "public"),
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if len(result.Assets) == 0 {
		t.Fatalf("expected built assets")
	}

	if _, err := os.Stat(filepath.Join(result.OutputDir, "assets", "css")); err != nil {
		t.Fatalf("expected output assets directory: %v", err)
	}
}
