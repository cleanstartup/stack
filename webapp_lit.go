package stack

import (
	"context"

	litpkg "github.com/cleanstartup/stack/internal/lit"
	"github.com/cleanstartup/stack/plugin"
)

// buildLitStage runs lit as the WebApp target's app-level singleton build
// stage (D-N/PRD §7), mirroring buildTailwindStage (webapp_tailwind.go)
// exactly: one pass over every assets.Dir(...) source dir declared by
// t.module and every Module composed as an Ingredient (CUP-27 —
// moduleContentDirs/ingredientModules). dirs is that combined set, computed
// once by Build and shared with buildTailwindStage so the composed tree is
// walked a single time per build, not once per stage. Returns nil, nil when
// no *.lit.ts/*.lit.tsx entry point is discovered anywhere in those dirs —
// the stage is a no-op, not an error.
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
// Ingredient Modules now included (CUP-27, closing the gap this doc comment
// used to describe as deferred): stack.WebApp(module, ui.Module(), ...)
// composes ui.Module() as an Ingredient, not as the WebApp's own module — a
// real Module meant to contribute component sources without being "the"
// app module. See ingredientModules (webapp_tailwind.go) for how those
// Modules are found among t.ingredients; Mount(Module, at)'s "at" is
// ignored for this purpose (build-time source scanning, not runtime
// routing).
func (t *WebAppTarget) buildLitStage(ctx context.Context, stageCtx plugin.StageContext, dirs []string) (*plugin.Contribution, error) {
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
