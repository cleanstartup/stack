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
	"github.com/cleanstartup/stack/asset"
	"github.com/cleanstartup/stack/assets"
	"github.com/cleanstartup/stack/plugin"
)

// TestWebAppTargetRunsLitStageForDeclaredEntry pins CUP-25: a Module
// declaring component sources via assets.Dir(...) gets its own app-level lit
// bundle Contribution, discovered purely from the "*.lit.ts entry point"
// naming convention inside that dir — no ingredient needed to opt in, lit is
// a stage of the WebApp target's own nature (D-N/PRD §7), same as tailwind
// (CUP-21). The fixture is a synthetic two-"component" library (standing in
// for CUP-27's not-yet-built ui.Module()) plus one entry that imports only
// one of them, proving the served bundle is tree-shaken (D-C) end to end
// through the real Module -> Producer -> WebAppTarget.Consume wiring, not
// just internal/lit in isolation.
func TestWebAppTargetRunsLitStageForDeclaredEntry(t *testing.T) {
	componentDir := t.TempDir()
	writeTestFile(t, componentDir, "button.ts", `
export const BUTTON_MARKER = "BUTTON_COMPONENT_MARKER";
export function renderButton() { return BUTTON_MARKER; }
`)
	writeTestFile(t, componentDir, "card.ts", `
export const CARD_MARKER = "CARD_COMPONENT_MARKER";
export function renderCard() { return CARD_MARKER; }
`)
	writeTestFile(t, componentDir, "app.lit.ts", `
import { renderButton } from "./button";
console.log(renderButton());
`)

	module := stack.Bundle("webapp-lit-test", assets.Dir(componentDir))
	target := stack.WebApp(module)
	if err := target.Build(context.Background(), plugin.StageContext{OutputDir: t.TempDir()}); err != nil {
		t.Fatalf("Build: %v", err)
	}

	links := target.AssetLinks()
	if len(links.Scripts) != 1 {
		t.Fatalf("got %d script links, want 1 (the lit stage's bundle): %+v", len(links.Scripts), links)
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
	if !strings.Contains(got, "BUTTON_COMPONENT_MARKER") {
		t.Fatalf("served bundle %q missing the used component's marker (button)", got)
	}
	if strings.Contains(got, "CARD_COMPONENT_MARKER") {
		t.Fatalf("served bundle %q contains the unused component's marker (card) — tree-shaking-per-consumer (D-C) failed", got)
	}
}

// TestWebAppTargetLitStageCollisionErrors mirrors
// TestWebAppTargetTailwindStageCollisionErrors: the lit stage's Contribution
// is also appended after flattenIngredients already ran, so it needs the
// same re-checked dedup (appendSingletonContribution in target_webapp.go) —
// a manually-wired ingredient producing an Asset at the exact path the lit
// stage would write to must fail fast, not silently double-mount.
func TestWebAppTargetLitStageCollisionErrors(t *testing.T) {
	componentDir := t.TempDir()
	entryPath := writeTestFile(t, componentDir, "app.lit.ts", `console.log("hi");`)

	outputDir := t.TempDir()

	// internal/lit.Build names each bundle output deterministically from the
	// entry's absolute path (asset.AssetID(entry) + ".js") under
	// <OutputDir>/lit — compute the exact path the lit stage will write to
	// and pre-empt it with a colliding ingredient.
	collidingPath := filepath.Join(outputDir, "lit", asset.AssetID(entryPath)+".js")

	collidingProducer := fakeProducer{assets: []plugin.Asset{
		{Path: collidingPath, ContentType: "application/javascript+module"},
	}}

	module := stack.Bundle("webapp-lit-collision-test", assets.Dir(componentDir))
	target := stack.WebApp(module, collidingProducer)
	err := target.Build(context.Background(), plugin.StageContext{OutputDir: outputDir})
	if err == nil {
		t.Fatal("want an error: an ingredient producing an Asset at the lit stage's own output path must not silently double-mount")
	}
	if !strings.Contains(err.Error(), "duplicate asset path") {
		t.Fatalf("err = %v, want it to mention the duplicate asset path", err)
	}
}

func TestWebAppTargetNoLitEntryIsNoop(t *testing.T) {
	dir := t.TempDir()
	// A component library dir with no *.lit.ts entry point should not
	// produce any script link — the lit stage is a no-op, not an error,
	// same posture as tailwind's empty-content case.
	if err := os.WriteFile(filepath.Join(dir, "button.ts"), []byte(`export const X = 1;`), 0o644); err != nil {
		t.Fatal(err)
	}

	module := stack.Bundle("webapp-lit-noop-test", assets.Dir(dir))
	target := stack.WebApp(module)
	if err := target.Build(context.Background(), plugin.StageContext{OutputDir: t.TempDir()}); err != nil {
		t.Fatalf("Build: %v", err)
	}

	links := target.AssetLinks()
	if len(links.Scripts) != 0 {
		t.Fatalf("got %d script links, want 0: %+v", len(links.Scripts), links)
	}
}
