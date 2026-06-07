package web_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cleanstartup/way2go/activity"
	"github.com/cleanstartup/way2go/web"
)

type smokeParams struct {
	Name string
}

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

	def := activity.As[smokeParams, activity.NoInput]("smoke.page")
	app := web.New(web.Module(
		web.BindActivity(def, func(params smokeParams) activity.Instance[smokeParams, activity.NoInput] {
			return def.Take(params).Then(func(ctx activity.Context, input activity.NoInput) activity.Result {
				return "ok"
			})
		}),
		web.CSS(web.FromFile(cssPath)),
	))
	result, err := app.Build(context.Background(), web.BuildConfig{
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

func TestWebCLIHelpIncludesCommandSummaries(t *testing.T) {
	app := web.New()

	res := app.CLI().Execute([]string{"--help"})
	if res.ExitCode != 0 {
		t.Fatalf("expected help exit 0, got %d", res.ExitCode)
	}
	if !strings.Contains(res.Stdout, "run - serve the already built web app") {
		t.Fatalf("expected run help summary, got %q", res.Stdout)
	}
	if !strings.Contains(res.Stdout, "build - materialize assets into the output directory") {
		t.Fatalf("expected build help summary, got %q", res.Stdout)
	}
	if !strings.Contains(res.Stdout, "dev - run the templ supervisor and asset watch loop") {
		t.Fatalf("expected dev help summary, got %q", res.Stdout)
	}
}
