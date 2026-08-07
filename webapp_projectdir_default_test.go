package stack_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cleanstartup/stack"
	"github.com/cleanstartup/stack/internal/testfixture/fakelib"
	"github.com/cleanstartup/stack/plugin"
)

// projectDirSpy is a plugin.Builder that records the StageContext it was
// actually invoked with, so a test can observe what WebAppTarget.Build
// handed downstream after applying its ProjectDir default — Contribution
// itself carries no StageContext, so this is the only way to observe it
// from outside the stack package.
type projectDirSpy struct {
	captured *plugin.StageContext
}

func (s *projectDirSpy) Build(_ context.Context, stageCtx plugin.StageContext) ([]plugin.Asset, error) {
	*s.captured = stageCtx
	return nil, nil
}

// thisFileDir returns this test file's own directory — the same directory
// stack.Bundle's webasset.CallerDir captures as rootDir() for any
// stack.Bundle(...) call made directly inside a function in this file (see
// stack_bundle_test.go's TestModuleRecordsCallerPackageDir for the existing
// precedent pinning that behavior).
func thisFileDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

// TestWebAppTargetDefaultsProjectDirFromOwnModuleNotIngredient pins a
// deliberate design choice CUP-27 code review asked to have verified rather
// than assumed: StageContext.ProjectDir, when the caller leaves it empty,
// defaults from the WebApp's *own* module's rootDir() — never from any
// ingredient Module's rootDir(), even though ingredientModules
// (webapp_tailwind.go) does fold an ingredient Module's *content dirs* into
// the tailwind/lit scan.
//
// This is intentional, not an oversight: in the shape this whole mechanism
// exists for — `stack.WebApp(myModule, ui.Module())` — ProjectDir must
// point at the *consuming app's* project root, because that's where a real
// npm project (package.json/node_modules with lit + Web Awesome installed)
// is expected to live. ui.Module()'s own rootDir() points at wherever the
// cleanstartup/ui Go module was checked out (GOPATH/pkg/mod, or a local
// replace) — a directory that structurally can never be the app's npm
// project root. Defaulting ProjectDir from an ingredient's rootDir instead
// of (or in addition to) the app's own would be actively wrong for the
// primary use case this mechanism was built for, not just an edge case left
// out.
func TestWebAppTargetDefaultsProjectDirFromOwnModuleNotIngredient(t *testing.T) {
	appModule := stack.Bundle("app-projectdir-default-test")
	lib := fakelib.Module()

	appRoot := thisFileDir()
	libRoot := fakelib.Dir()
	if appRoot == "" || libRoot == "" {
		t.Fatalf("test fixture invalid: both dirs must be non-empty (app=%q lib=%q)", appRoot, libRoot)
	}
	if appRoot == libRoot {
		t.Fatalf("test fixture invalid: app dir (%q) and fakelib dir (%q) must differ — they're different .go files in different directories, so this indicates a fixture bug, not a real pass", appRoot, libRoot)
	}

	var captured plugin.StageContext
	spy := &projectDirSpy{captured: &captured}

	target := stack.WebApp(appModule, lib, spy)
	if err := target.Build(context.Background(), plugin.StageContext{OutputDir: t.TempDir()}); err != nil {
		t.Fatalf("Build: %v", err)
	}

	if captured.ProjectDir != appRoot {
		t.Fatalf("stageCtx.ProjectDir = %q, want the WebApp's own module's rootDir %q", captured.ProjectDir, appRoot)
	}
	if captured.ProjectDir == libRoot {
		t.Fatal("stageCtx.ProjectDir defaulted from the ingredient Module's rootDir instead of the WebApp's own module — wrong for the ui.Module() use case this exists for")
	}
}
