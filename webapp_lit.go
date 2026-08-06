package stack

import (
	"context"

	litpkg "github.com/cleanstartup/stack/internal/lit"
	"github.com/cleanstartup/stack/plugin"
)

// buildLitStage runs lit as the WebApp target's app-level singleton build
// stage (D-N/PRD §7), mirroring buildTailwindStage (webapp_tailwind.go)
// exactly: one pass over every assets.Dir(...) source dir declared by
// t.module. Returns nil, nil when no *.lit.ts/*.lit.tsx entry point is
// discovered anywhere in those dirs — the stage is a no-op, not an error.
//
// Source-declaration mechanism: reuses assets.Dir(...) as-is, the same
// DirSource moduleContentDirs already collects for tailwind — no new
// module-facing API. tailwind and lit both need "a module-declared
// directory, filtered by an internal naming convention"; they differ only
// in which files within that directory matter (tailwind: any
// "*.tailwind.css"; lit: files ending "*.lit.ts"/"*.lit.tsx" are bundle
// entry points — see internal/lit/discovery.go's doc comment for why plain,
// non-entry .ts files in the same declared dir are not a second concept
// needing their own API, just importable-but-not-an-entry modules).
//
// Same scope fence as buildTailwindStage: scoped to t.module only, not
// ingredient Modules — flattenIngredients doesn't yet model a Module
// ingredient contributing sources beyond routes, so walking ingredient
// Modules here would be speculative, untestable code with no reachable
// caller today. CUP-27's ui.Module() is what will need this.
func (t *WebAppTarget) buildLitStage(ctx context.Context, stageCtx plugin.StageContext) (*plugin.Contribution, error) {
	dirs := moduleContentDirs(t.module)
	if len(dirs) == 0 {
		return nil, nil
	}

	producer := litpkg.NewProducer(dirs...)
	assets, err := producer.Build(ctx, stageCtx)
	if err != nil {
		return nil, err
	}
	if len(assets) == 0 {
		return nil, nil
	}

	return &plugin.Contribution{Assets: assets}, nil
}
