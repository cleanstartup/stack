package stack_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cleanstartup/stack"
	"github.com/cleanstartup/stack/assets"
	"github.com/cleanstartup/stack/plugin"
)

// TestWebAppTargetLitStageResolvesBareSpecifierViaProjectDir pins CUP-27's
// slice of the StageContext contract gap CUP-25 flagged (internal/lit/npm.go):
// stageCtx.ProjectDir now reaches esbuild's AbsWorkingDir (internal/lit/
// build.go), so a real `import ... from "lit"` bare specifier resolves
// against an actual node_modules tree instead of only relative-path sources
// bundling successfully.
//
// It also pins the other half of the gap CUP-25/CUP-26 left open: a Module
// contributing component sources composed as an *Ingredient* — the exact
// shape ui.Module() needs (`stack.WebApp(module, ui.Module())`, D-M/D-N) —
// now has its assets.Dir(...) content dirs picked up by the lit/tailwind
// stages via ingredientModules, not just t.module's own dirs.
//
// The fixture stands in for CUP-27's ui.Module(): a hand-built node_modules/
// lit (no real npm install / network access needed — proves the resolution
// mechanism itself, same "no network in tests" posture CUP-25 used for
// tree-shaking) plus a small two-file "component library" where only one
// file is reachable from the entry, so tree-shaking-per-consumer (D-C)
// still holds for a Module composed as an ingredient, not just as the
// WebApp's own module (which is all CUP-25's proof covered).
func TestWebAppTargetLitStageResolvesBareSpecifierViaProjectDir(t *testing.T) {
	projectDir := t.TempDir()
	litPkgDir := filepath.Join(projectDir, "node_modules", "lit")
	if err := os.MkdirAll(litPkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, litPkgDir, "package.json", `{"name":"lit","version":"3.0.0-test","main":"index.js"}`)
	writeTestFile(t, litPkgDir, "index.js", `export const LIT_MARKER = "LIT_RESOLVED_MARKER";`)

	componentDir := t.TempDir()
	writeTestFile(t, componentDir, "cup-button.ts", `
import { LIT_MARKER } from "lit";
export const BUTTON_MARKER = "BUTTON_COMPONENT:" + LIT_MARKER;
`)
	writeTestFile(t, componentDir, "cup-card.ts", `
export const CARD_MARKER = "CARD_COMPONENT_MARKER";
`)
	writeTestFile(t, componentDir, "index.lit.ts", `
import { BUTTON_MARKER } from "./cup-button";
console.log(BUTTON_MARKER);
`)

	// uiModule stands in for ui.Module(): a Module whose only purpose is to
	// contribute component sources, composed as an Ingredient — not as the
	// WebApp's own module.
	uiModule := stack.Bundle("cup-ui-projectdir-test", assets.Dir(componentDir))
	appModule := stack.Bundle("app-projectdir-test")

	target := stack.WebApp(appModule, uiModule)
	err := target.Build(context.Background(), plugin.StageContext{
		OutputDir:  t.TempDir(),
		ProjectDir: projectDir,
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	links := target.AssetLinks()
	if len(links.Scripts) != 1 {
		t.Fatalf("got %d script links, want 1 (ui.Module()'s ingredient-contributed entry): %+v", len(links.Scripts), links)
	}

	server := httptest.NewServer(target.Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + links.Scripts[0])
	if err != nil {
		t.Fatalf("GET %s: %v", links.Scripts[0], err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	got := string(body)
	if !strings.Contains(got, "LIT_RESOLVED_MARKER") {
		t.Fatalf("served bundle %q missing lit's marker — bare `from \"lit\"` specifier did not resolve via ProjectDir/AbsWorkingDir", got)
	}
	if !strings.Contains(got, "BUTTON_COMPONENT") {
		t.Fatalf("served bundle %q missing the used component's marker (cup-button)", got)
	}
	if strings.Contains(got, "CARD_COMPONENT_MARKER") {
		t.Fatalf("served bundle %q contains the unused component's marker (cup-card) — tree-shaking for an ingredient-composed Module failed", got)
	}
}

// TestWebAppTargetLitStageWithoutProjectDirFailsOnBareSpecifier is the
// negative control for the test above: without a ProjectDir pointing at a
// real node_modules tree (and with the WebApp's own module carrying no
// useful rootDir() default here, since this test file's own directory has
// no node_modules/lit), esbuild must fail to resolve the bare `from "lit"`
// specifier — proving the ProjectDir plumbing is load-bearing, not a no-op
// that happened to work anyway.
func TestWebAppTargetLitStageWithoutProjectDirFailsOnBareSpecifier(t *testing.T) {
	componentDir := t.TempDir()
	writeTestFile(t, componentDir, "cup-button.lit.ts", `
import { LIT_MARKER } from "lit";
console.log(LIT_MARKER);
`)

	uiModule := stack.Bundle("cup-ui-noprojectdir-test", assets.Dir(componentDir))
	appModule := stack.Bundle("app-noprojectdir-test")

	target := stack.WebApp(appModule, uiModule)
	// stageCtx.ProjectDir left empty; Build defaults it from appModule's
	// rootDir() (this _test.go file's own directory), which has no
	// node_modules/lit — the import must still fail to resolve.
	err := target.Build(context.Background(), plugin.StageContext{OutputDir: t.TempDir()})
	if err == nil {
		t.Fatal("want a build error: a bare `from \"lit\"` specifier has no node_modules to resolve against")
	}
}
