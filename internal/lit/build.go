// Package lit is the generic TS/lit component build plugin (CUP-25),
// analogous to internal/tailwind (CUP-21) on the same plugin.Builder
// contract (CUP-26). Per D-N it is an app-level singleton bundler pass over
// the union of contributed component sources, tree-shaken per consumer —
// see webapp_lit.go (stack package root) for how it's wired into the
// WebApp target's own build stage.
//
// Bundler choice: github.com/evanw/esbuild's Go API (pkg/api), not a
// shelled-out binary. This is the one genuinely open fork this task called
// out (mirrors tailwind's Bin/NPM choices, but lit doesn't reuse them):
//   - esbuild ships as an ordinary Go module (MVS-resolved, no separate
//     binary-provisioning/download dance like tailwind's BinProvider).
//   - api.Build runs in-process and synchronously, so the tree-shaking
//     behavior this plugin's whole raison d'être depends on (D-C) is
//     directly and deterministically testable with no network access, no
//     downloaded binary, and no npm install step required in CI.
//   - No node/npm toolchain needs to exist on the host for esbuild itself,
//     unlike tailwind (which still shells out to a real tailwindcss
//     binary/npm). stageCtx.Bin is therefore unused here. stageCtx.NPM is
//     still used (producer.go, npm.go) to declare the "lit" package as an
//     intended dependency, same shape as tailwind's AddNPMDependencies — but
//     see npm.go's doc comment for why that alone doesn't yet make a real
//     `import ... from "lit"` resolvable; today only relative-import sources
//     bundle successfully.
//
// Trade-off accepted: esbuild's Go implementation becomes a compile-time
// dependency of the stack module itself (not just an npm devDependency),
// same as this package's own code. That's consistent with today's !release-
// gated toolchain story (D-H) once that tagging lands (not yet threaded
// through webapp_tailwind.go either — out of scope here, same as CUP-21).
package lit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/evanw/esbuild/pkg/api"

	assetspkg "github.com/cleanstartup/stack/asset"
	pluginpkg "github.com/cleanstartup/stack/plugin"
)

// Build bundles each entry in entries independently into outDir, one output
// file per entry (no shared-chunk code splitting — that's PRD non-goal #2,
// "code-splitting convention for lit entry vs. lazy chunk", deliberately
// deferred). Bundling each entry on its own esbuild call, rather than one
// call across all entries with Splitting enabled, is what makes tree-
// shaking-per-consumer (D-C) fall out mechanically: an entry's bundle only
// ever contains what that entry actually imports, regardless of what any
// other entry imports.
//
// ctx is checked for cancellation between entries; esbuild's api.Build call
// itself is synchronous and has no context parameter of its own.
func Build(ctx context.Context, entries []string, outDir string) ([]pluginpkg.Asset, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(outDir) == "" {
		return nil, fmt.Errorf("lit: outDir is required to build %d entry point(s)", len(entries))
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}

	assets := make([]pluginpkg.Asset, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		outPath := filepath.Join(outDir, assetspkg.AssetID(entry)+".js")
		result := api.Build(api.BuildOptions{
			EntryPoints:       []string{entry},
			Bundle:            true,
			Write:             true,
			Outfile:           outPath,
			Format:            api.FormatESModule,
			Platform:          api.PlatformBrowser,
			Target:            api.ESNext,
			TreeShaking:       api.TreeShakingTrue,
			MinifyWhitespace:  true,
			MinifyIdentifiers: true,
			MinifySyntax:      true,
			LogLevel:          api.LogLevelSilent,
		})
		if len(result.Errors) > 0 {
			return nil, fmt.Errorf("lit: esbuild failed for %s: %s", entry, strings.Join(api.FormatMessages(result.Errors, api.FormatMessagesOptions{Kind: api.ErrorMessage}), "\n"))
		}
		// application/javascript+module (D-K): Format: ESM above is real byte
		// semantics (the output uses import/export), not just a placement
		// hint — a <script> tag serving this asset must carry type="module".
		assets = append(assets, pluginpkg.Asset{Path: outPath, ContentType: "application/javascript+module"})
	}
	return assets, nil
}
