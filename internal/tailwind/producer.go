package tailwind

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pluginpkg "github.com/cleanstartup/stack/plugin"
)

// Producer is the plugin.Builder-conformant wrapper around this package's
// existing Build logic (CUP-21): the new contract's app-level tailwind stage.
// ScanPaths are Module-declared content dirs (via assets.Dir, gathered by the
// stack package) — Producer itself does no module/composition-tree walking.
type Producer struct {
	ScanPaths []string // absolute dirs to @source-scan for tailwind class usage
	Binary    string   // optional: explicit tailwindcss binary path (tests / override)
	Version   string   // optional: pin a version instead of "latest"
}

// NewProducer builds a Producer over the given (already-resolved, absolute)
// scan dirs.
func NewProducer(scanPaths ...string) *Producer {
	return &Producer{ScanPaths: append([]string{}, scanPaths...)}
}

// Build implements plugin.Builder. Returns nil, nil if there are no scan
// paths — no content declared anywhere means the stage is a no-op, not an
// error.
func (p *Producer) Build(ctx context.Context, stageCtx pluginpkg.StageContext) ([]pluginpkg.Asset, error) {
	if p == nil || len(p.ScanPaths) == 0 {
		return nil, nil
	}
	if strings.TrimSpace(stageCtx.OutputDir) == "" {
		return nil, fmt.Errorf("tailwind: stageCtx.OutputDir is required to build %d declared content dir(s)", len(p.ScanPaths))
	}

	AddNPMDependencies(stageCtx.NPM)

	sources := make([]Source, 0, len(p.ScanPaths))
	for _, dir := range p.ScanPaths {
		sources = append(sources, contentDirSource{dir: dir})
	}

	outputPath := OutputPath(stageCtx.OutputDir)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return nil, err
	}

	cacheRoot := filepath.Join(stageCtx.OutputDir, "tailwind-cache")

	cfg := Config{Bin: stageCtx.Bin, Binary: p.Binary, Version: p.Version}
	if err := Build(ctx, newCacheWorkspace(cacheRoot), cacheRoot, outputPath, sources, p.ScanPaths, cfg); err != nil {
		return nil, err
	}

	return []pluginpkg.Asset{{Path: outputPath, ContentType: "text/css"}}, nil
}

// contentDirSource discovers *.tailwind.css fragments within dir to @import
// into the generated input — a straight port of stack.go's lazyTailwindSource,
// re-typed against this package's own Workspace/AssetKind (no adapter needed
// since Producer lives inside this package).
type contentDirSource struct{ dir string }

func (s contentDirSource) ID() string { return "content:" + s.dir }

func (s contentDirSource) Materialize(_ Workspace, _ AssetKind) ([]string, error) {
	return DiscoverStyles(s.dir), nil
}

func (s contentDirSource) WatchPaths() []string {
	if s.dir == "" {
		return nil
	}
	return []string{s.dir}
}
