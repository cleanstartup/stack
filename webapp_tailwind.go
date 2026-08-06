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
// declared by t.module. Returns nil, nil when no content is declared — the
// stage is a no-op, not an error, and nothing changes for a target that never
// opts in.
//
// Scoped to t.module only, not ingredient Modules: flattenIngredients
// (compose.go) doesn't yet model a Module ingredient contributing anything
// beyond routes ("Module→Sources isn't modeled yet, deferred to CUP-21/25"),
// so scanning ingredient Modules here would be speculative, untestable code
// with no reachable caller today — CUP-27's ui.Module() is what will need
// this, and it can extend this function when it lands.
func (t *WebAppTarget) buildTailwindStage(ctx context.Context, stageCtx plugin.StageContext) (*plugin.Contribution, error) {
	dirs := moduleContentDirs(t.module)
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
