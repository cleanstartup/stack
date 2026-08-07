package stack

import (
	"context"

	assetspkg "github.com/cleanstartup/stack/assets"
	tailwindpkg "github.com/cleanstartup/stack/internal/tailwind"
	"github.com/cleanstartup/stack/plugin"
)

// moduleContentDirs walks module's Part tree (same shape validateNamespaces
// in stack.go already uses for nested *bundles) collecting every
// assetspkg.DirSource declared anywhere in it, deduped by AbsPath. Reuses the
// existing assets.Dir(...) module-facing API as-is (CUP-21) — no new
// module-facing mechanism for declaring tailwind-scannable content.
//
// Shared with the lit stage (webapp_lit.go, CUP-25): both stages need "every
// module-declared source dir", differing only in which files within each dir
// they care about (an internal, per-plugin naming-convention filter applied
// downstream, not a difference in how the dir itself is declared) — so
// there's one collector, not two.
func moduleContentDirs(module Module) []string {
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
	walk(module)
	return dirs
}

// buildTailwindStage runs tailwind as the WebApp target's app-level singleton
// build stage (D-N/PRD §7): one pass over every assets.Dir(...) content dir
// declared by t.module. dirs is t.module's moduleContentDirs, computed once
// by Build and shared with buildLitStage so the module's Part tree is walked
// a single time per build, not once per stage. Returns nil, nil when no
// content is declared — the stage is a no-op, not an error, and nothing
// changes for a target that never opts in.
//
// Scoped to t.module only, not ingredient Modules: flattenIngredients
// (compose.go) doesn't yet model a Module ingredient contributing anything
// beyond routes ("Module→Sources isn't modeled yet, deferred to CUP-21/25"),
// so scanning ingredient Modules here would be speculative, untestable code
// with no reachable caller today — CUP-27's ui.Module() is what will need
// this, and it can extend this function when it lands.
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
