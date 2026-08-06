package lit

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	pluginpkg "github.com/cleanstartup/stack/plugin"
)

// Producer is the plugin.Builder-conformant wrapper around this package's
// Build logic (CUP-25): the new contract's app-level lit stage. SourceDirs
// are Module-declared source dirs (via assets.Dir, gathered by the stack
// package — same DirSource mechanism CUP-21 reused for tailwind, see
// webapp_lit.go's doc comment for why no new module-facing API was needed)
// — Producer itself does no module/composition-tree walking.
type Producer struct {
	SourceDirs []string // absolute dirs to discover *.lit.ts/*.lit.tsx entry points in
}

// NewProducer builds a Producer over the given (already-resolved, absolute)
// source dirs.
func NewProducer(sourceDirs ...string) *Producer {
	return &Producer{SourceDirs: append([]string{}, sourceDirs...)}
}

// Build implements plugin.Builder. Returns nil, nil if no *.lit.ts/*.lit.tsx
// entry point is found anywhere in SourceDirs — no declared entry means the
// stage is a no-op, not an error (same posture as tailwind's Producer.Build
// with zero scan paths).
func (p *Producer) Build(ctx context.Context, stageCtx pluginpkg.StageContext) ([]pluginpkg.Asset, error) {
	if p == nil || len(p.SourceDirs) == 0 {
		return nil, nil
	}

	var entries []string
	seen := map[string]bool{}
	for _, dir := range p.SourceDirs {
		found, err := DiscoverEntries(dir)
		if err != nil {
			return nil, err
		}
		for _, entry := range found {
			if !seen[entry] {
				seen[entry] = true
				entries = append(entries, entry)
			}
		}
	}
	if len(entries) == 0 {
		return nil, nil
	}
	if strings.TrimSpace(stageCtx.OutputDir) == "" {
		return nil, fmt.Errorf("lit: stageCtx.OutputDir is required to build %d discovered entry point(s)", len(entries))
	}

	// See AddNPMDependencies' doc comment (npm.go): this records the
	// intended dependency for whatever eventually installs it — it does not
	// itself make esbuild able to resolve a bare `from "lit"` specifier.
	AddNPMDependencies(stageCtx.NPM)

	outDir := filepath.Join(stageCtx.OutputDir, "lit")
	return Build(ctx, entries, outDir)
}
