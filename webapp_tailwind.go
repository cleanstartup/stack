package stack

import (
	"context"

	assetspkg "github.com/cleanstartup/stack/assets"
	tailwindpkg "github.com/cleanstartup/stack/internal/tailwind"
	"github.com/cleanstartup/stack/plugin"
)

// moduleContentDirs walks each of modules' Part trees (same shape
// validateNamespaces in stack.go already uses for nested *bundles)
// collecting every assetspkg.DirSource declared anywhere in any of them,
// deduped by AbsPath across all of them together. Reuses the existing
// assets.Dir(...) module-facing API as-is (CUP-21) — no new module-facing
// mechanism for declaring tailwind/lit-scannable content.
//
// Shared with the lit stage (webapp_lit.go, CUP-25): both stages need "every
// module-declared source dir", differing only in which files within each dir
// they care about (an internal, per-plugin naming-convention filter applied
// downstream, not a difference in how the dir itself is declared) — so
// there's one collector, not two.
//
// Takes multiple roots (CUP-27): t.module plus any Module composed as an
// Ingredient (e.g. `stack.WebApp(module, ui.Module())` — see
// ingredientModules below) all contribute to the same content-dir set. Before
// CUP-27 this only ever walked t.module — ingredient Modules were
// deliberately unscanned because nothing exercised that path (see prior
// history of this function); ui.Module() is the first real Module meant to
// be composed as an ingredient rather than as the WebApp's own module, so
// leaving it unscanned would silently drop its lit/tailwind sources.
func moduleContentDirs(modules ...Module) []string {
	var dirs []string
	seen := map[string]bool{}
	var walk func(part Part)
	walk = func(part Part) {
		if part == nil {
			return
		}
		if dir, ok := part.(assetspkg.DirSource); ok {
			if abs := dir.AbsPath(); abs != "" && !seen[abs] {
				seen[abs] = true
				dirs = append(dirs, abs)
			}
		}
		if b, ok := part.(*bundle); ok {
			for _, p := range b.parts {
				walk(p)
			}
		}
	}
	for _, module := range modules {
		walk(module)
	}
	return dirs
}

// ingredientModules extracts every Module found among ingredients so
// callers can fold their content dirs into moduleContentDirs alongside
// t.module. In practice today that means only a *bare* Module ingredient —
// `stack.WebApp(module, ui.Module())`, the shape CUP-27's ui.Module()
// actually uses. The `case mountEdge` branch below is dead code on a
// WebAppTarget, not a second supported path, and should not be read as one:
// applyMountEdge (compose.go) unconditionally errors on `Mount(Module, at)`
// for a non-nil app, so flattenIngredients (called before moduleContentDirs
// in WebAppTarget.Build) already returns that error before this function
// ever runs — pinned by TestModuleEdgeMountOnWebAppIsFenced
// (target_webapp_test.go). Kept rather than deleted because it costs
// nothing and is already correct for the day Mount(Module, at) routing is
// implemented for real (D-M's "routes+assets under a prefix, namespaced" —
// still unscheduled, see applyMountEdge's own doc comment): content-dir
// scanning for that Module will already work with no further change here.
func ingredientModules(ingredients []Ingredient) []Module {
	var modules []Module
	for _, ingredient := range ingredients {
		switch v := ingredient.(type) {
		case Module:
			modules = append(modules, v)
		case mountEdge:
			if m, ok := v.subject.(Module); ok {
				modules = append(modules, m)
			}
		}
	}
	return modules
}

// buildTailwindStage runs tailwind as the WebApp target's app-level singleton
// build stage (D-N/PRD §7): one pass over every assets.Dir(...) content dir
// declared by t.module and every Module composed as an Ingredient (D-C —
// see moduleContentDirs/ingredientModules, CUP-27). dirs is that combined
// set, computed once by Build and shared with buildLitStage so the composed
// tree is walked a single time per build, not once per stage. Returns nil,
// nil when no content is declared — the stage is a no-op, not an error, and
// nothing changes for a target that never opts in.
func (t *WebAppTarget) buildTailwindStage(ctx context.Context, stageCtx plugin.StageContext, dirs []string) (*plugin.Contribution, error) {
	if len(dirs) == 0 {
		return nil, nil
	}

	producer := tailwindpkg.NewProducer(dirs...)
	assets, err := producer.Build(ctx, stageCtx)
	if err != nil {
		return nil, err
	}
	if len(assets) == 0 {
		return nil, nil
	}

	return &plugin.Contribution{Assets: assets}, nil
}
