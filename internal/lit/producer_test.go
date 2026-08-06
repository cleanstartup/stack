package lit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pluginpkg "github.com/cleanstartup/stack/plugin"
)

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestProducerBuildNoSourceDirsIsNoop(t *testing.T) {
	p := NewProducer()
	assets, err := p.Build(context.Background(), pluginpkg.StageContext{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if assets != nil {
		t.Fatalf("got %v assets, want nil for zero source dirs", assets)
	}
}

func TestProducerBuildNoEntryPointsIsNoop(t *testing.T) {
	dir := t.TempDir()
	// A plain, non-entry .ts file: a component library with no entry that
	// imports it should not produce a bundle on its own.
	writeTestFile(t, dir, "button.ts", `export const BUTTON = "BUTTON_COMPONENT_MARKER";`)

	p := NewProducer(dir)
	assets, err := p.Build(context.Background(), pluginpkg.StageContext{OutputDir: t.TempDir()})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if assets != nil {
		t.Fatalf("got %v assets, want nil: a bare component library dir with no *.lit.ts entry point should build nothing", assets)
	}
}

// TestProducerBuildTreeShakesUnusedComponent is the load-bearing test for
// D-C: "the lit bundler tree-shakes per consumer to just the components
// actually used". The fixture is a synthetic two-component "library"
// (button.ts, card.ts — standing in for CUP-27's not-yet-built ui.Module()
// sources) plus one entry point (app.lit.ts) that imports only button.ts.
// The bundled output must contain button's marker and must NOT contain
// card's marker — proving the unused component was dropped, not just that
// a bundle was produced.
func TestProducerBuildTreeShakesUnusedComponent(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "button.ts", `
export const BUTTON_MARKER = "BUTTON_COMPONENT_MARKER";
export function renderButton() { return BUTTON_MARKER; }
`)
	writeTestFile(t, dir, "card.ts", `
export const CARD_MARKER = "CARD_COMPONENT_MARKER";
export function renderCard() { return CARD_MARKER; }
`)
	writeTestFile(t, dir, "app.lit.ts", `
import { renderButton } from "./button";
console.log(renderButton());
`)

	outputDir := t.TempDir()
	p := NewProducer(dir)
	assets, err := p.Build(context.Background(), pluginpkg.StageContext{OutputDir: outputDir})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("got %d assets, want 1 (one bundle for the one entry point): %+v", len(assets), assets)
	}
	if assets[0].ContentType != "application/javascript+module" {
		t.Fatalf("ContentType = %q, want application/javascript+module", assets[0].ContentType)
	}

	body, err := os.ReadFile(assets[0].Path)
	if err != nil {
		t.Fatalf("read output asset: %v", err)
	}
	got := string(body)
	if !strings.Contains(got, "BUTTON_COMPONENT_MARKER") {
		t.Fatalf("output bundle %q missing the used component's marker (button)", got)
	}
	if strings.Contains(got, "CARD_COMPONENT_MARKER") {
		t.Fatalf("output bundle %q contains the unused component's marker (card) — it should have been tree-shaken out", got)
	}
}

// TestProducerBuildBundlesEachEntryIndependently pins the "no shared chunk"
// design call (PRD non-goal: code-splitting entry-vs-chunk convention is
// deferred): two entries in the same source dir, each importing a distinct
// component, must each get their own self-contained bundle — entry A's
// output must not leak entry B's component and vice versa.
func TestProducerBuildBundlesEachEntryIndependently(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "button.ts", `export const BUTTON_MARKER = "BUTTON_COMPONENT_MARKER";`)
	writeTestFile(t, dir, "card.ts", `export const CARD_MARKER = "CARD_COMPONENT_MARKER";`)
	writeTestFile(t, dir, "buttons.lit.ts", `import { BUTTON_MARKER } from "./button"; console.log(BUTTON_MARKER);`)
	writeTestFile(t, dir, "cards.lit.ts", `import { CARD_MARKER } from "./card"; console.log(CARD_MARKER);`)

	outputDir := t.TempDir()
	p := NewProducer(dir)
	assets, err := p.Build(context.Background(), pluginpkg.StageContext{OutputDir: outputDir})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(assets) != 2 {
		t.Fatalf("got %d assets, want 2 (one bundle per entry point): %+v", len(assets), assets)
	}

	var buttonsBody, cardsBody string
	for _, a := range assets {
		body, err := os.ReadFile(a.Path)
		if err != nil {
			t.Fatalf("read %s: %v", a.Path, err)
		}
		switch {
		case strings.Contains(string(body), "BUTTON_COMPONENT_MARKER"):
			buttonsBody = string(body)
		case strings.Contains(string(body), "CARD_COMPONENT_MARKER"):
			cardsBody = string(body)
		}
	}
	if buttonsBody == "" {
		t.Fatalf("no bundle contained the button entry's marker: %+v", assets)
	}
	if cardsBody == "" {
		t.Fatalf("no bundle contained the card entry's marker: %+v", assets)
	}
	if strings.Contains(buttonsBody, "CARD_COMPONENT_MARKER") {
		t.Fatalf("buttons.lit.ts bundle leaked the card entry's component: %q", buttonsBody)
	}
	if strings.Contains(cardsBody, "BUTTON_COMPONENT_MARKER") {
		t.Fatalf("cards.lit.ts bundle leaked the button entry's component: %q", cardsBody)
	}
}

func TestProducerBuildRequiresOutputDir(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "app.lit.ts", `console.log("hi");`)

	p := NewProducer(dir)
	_, err := p.Build(context.Background(), pluginpkg.StageContext{})
	if err == nil {
		t.Fatal("want an error: OutputDir is required once an entry point is discovered")
	}
}

// TestProducerBuildDeclaresNPMDependency pins the fix for the missing
// stageCtx.NPM wiring: once Producer.Build finds real work to do, it must
// declare "lit" on the shared npm registry the same way tailwind declares
// its own tooling — even though (see npm.go's doc comment) nothing yet
// turns that declaration into a resolvable node_modules tree.
func TestProducerBuildDeclaresNPMDependency(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "app.lit.ts", `console.log("hi");`)

	fake := &fakeNPM{}
	p := NewProducer(dir)
	_, err := p.Build(context.Background(), pluginpkg.StageContext{OutputDir: t.TempDir(), NPM: fake})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if _, ok := fake.deps["lit"]; !ok {
		t.Fatalf("expected Build to declare \"lit\" on stageCtx.NPM, got %+v", fake.deps)
	}
}

// TestProducerBuildPropagatesDiscoveryError pins the fix for the discarded
// filepath.WalkDir error (discovery.go): a broken declared SourceDir must
// fail Producer.Build, not be silently treated as "no entry points found".
func TestProducerBuildPropagatesDiscoveryError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	p := NewProducer(missing)
	_, err := p.Build(context.Background(), pluginpkg.StageContext{OutputDir: t.TempDir()})
	if err == nil {
		t.Fatal("want an error: a non-existent declared source dir must fail the build, not silently produce zero entries")
	}
}
