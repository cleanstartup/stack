package stencil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cleanstartup/stack/asset"
	"github.com/cleanstartup/stack/capability"
	"github.com/cleanstartup/stack/pipeline"
)

type ConfigResolver func(capability.Context) Config

type Capability struct {
	registry      *Registry
	resolveConfig ConfigResolver
}

func NewCapability(registry *Registry, resolveConfig ConfigResolver) Capability {
	return Capability{registry: registry, resolveConfig: resolveConfig}
}

func OutputPath(outputRoot string) string {
	return filepath.Join(outputRoot, "assets", "js", BundleID)
}

func (c Capability) Install(ctx context.Context, cfg capability.Context) error {
	_ = ctx
	if c.empty() {
		return nil
	}
	projectDir := strings.TrimSpace(cfg.ProjectDir)
	if projectDir == "" {
		return nil
	}
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return err
	}

	srcDir, realDirs, err := c.buildSymlinkSrcDir(projectDir)
	if err != nil {
		return err
	}

	outDir := filepath.Join(cfg.OutputDir, "assets", "js")
	if rel, err := filepath.Rel(projectDir, outDir); err == nil && strings.TrimSpace(rel) != "" {
		outDir = rel
	}
	if err := os.WriteFile(filepath.Join(projectDir, "stencil.config.ts"), []byte(configSource(srcDir, outDir)), 0o644); err != nil {
		return err
	}
	// tsconfig includes both the symlink dir (for Stencil's native watch/discovery)
	// and the real source dirs (so TypeScript plugin processes files after Rollup resolves symlinks).
	includes := append([]string{srcDir}, realDirs...)
	return os.WriteFile(filepath.Join(projectDir, "tsconfig.json"), []byte(tsconfigSource(includes...)), 0o644)
}

// buildSymlinkSrcDir creates .stack/stencil-src/ inside projectDir with one
// symlink per registered source pointing at its original directory. This gives
// Stencil a narrow srcDir so --watch never scans node_modules or unrelated trees.
// Returns the symlink dir (relative to projectDir) and the absolute real source dirs.
func (c Capability) buildSymlinkSrcDir(projectDir string) (string, []string, error) {
	linkRoot := filepath.Join(projectDir, ".stack", "stencil-src")
	if err := os.RemoveAll(linkRoot); err != nil {
		return "", nil, err
	}
	if err := os.MkdirAll(linkRoot, 0o755); err != nil {
		return "", nil, err
	}
	var realDirs []string
	for _, source := range c.registry.Inputs() {
		dirs, err := SourcePaths(source)
		if err != nil || len(dirs) == 0 {
			continue
		}
		dir := strings.TrimSpace(dirs[0])
		if dir == "" {
			continue
		}
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return "", nil, err
		}
		linkName := filepath.Join(linkRoot, source.ID())
		if err := os.Symlink(absDir, linkName); err != nil {
			return "", nil, fmt.Errorf("stencil symlink %s -> %s: %w", linkName, absDir, err)
		}
		realDirs = append(realDirs, filepath.ToSlash(absDir))
	}
	rel, err := filepath.Rel(projectDir, linkRoot)
	if err != nil {
		return filepath.ToSlash(linkRoot), realDirs, nil
	}
	return filepath.ToSlash(rel), realDirs, nil
}

func (c Capability) Build(ctx context.Context, cfg capability.Context) error {
	if c.empty() || cfg.Workspace == nil {
		return nil
	}
	return Build(ctx, workspaceAdapter{workspace: cfg.Workspace}, c.cacheRoot(cfg), cfg.Workspace.OutputDir(), c.registry.Inputs(), c.config(cfg))
}

func (c Capability) Dev(ctx context.Context, cfg capability.Context) ([]pipeline.WatchWorker, error) {
	if c.empty() {
		return nil, nil
	}
	spec, err := DevCommand(c.config(cfg))
	if err != nil {
		return nil, err
	}
	outputPath := OutputPath(cfg.OutputDir)
	if err := os.MkdirAll(outputPath, 0o755); err != nil {
		return nil, err
	}
	worker, err := pipeline.StartRestartingCommandWatchSpec(ctx, "stencil", spec)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(os.Stderr, "[stack] stencil watch started output=%s\n", outputPath)
	return []pipeline.WatchWorker{worker}, nil
}

func (c Capability) Register(target capability.Target) {
	if target == nil || c.empty() {
		return
	}
	ref := c.registry.BundleRef()
	target.RegisterJS(asset.AssetRef{Kind: asset.AssetKind(ref.Kind), ID: ref.ID, Files: append([]string{}, ref.Files...)})
}

func (c Capability) SourcePaths() []string {
	if c.registry == nil {
		return nil
	}
	var paths []string
	for _, source := range c.registry.Inputs() {
		sourcePaths, err := SourcePaths(source)
		if err != nil {
			continue
		}
		paths = append(paths, sourcePaths...)
	}
	return paths
}

func (c Capability) SourceChanged(path string) bool {
	if !isSourceFile(path) {
		return false
	}
	for _, candidate := range c.SourcePaths() {
		if pipeline.SourcePathMatches(candidate, path) {
			return true
		}
	}
	return false
}

func (c Capability) Rebuild(context.Context, capability.Context) error {
	return nil
}

func (c Capability) empty() bool {
	return c.registry == nil || len(c.registry.Inputs()) == 0
}

func (c Capability) config(ctx capability.Context) Config {
	if c.resolveConfig == nil {
		return Config{ProjectDir: ctx.ProjectDir}
	}
	cfg := c.resolveConfig(ctx)
	if strings.TrimSpace(cfg.ProjectDir) == "" {
		cfg.ProjectDir = ctx.ProjectDir
	}
	return cfg
}

func (c Capability) cacheRoot(ctx capability.Context) string {
	if ctx.Workspace == nil {
		return filepath.Join(".stack", "stencil-workspace")
	}
	return filepath.Join(filepath.Dir(ctx.Workspace.RootDir()), "stencil-workspace")
}

type workspaceAdapter struct {
	workspace capability.Workspace
}

func (a workspaceAdapter) AssetDir(kind AssetKind, id string) string {
	if a.workspace == nil {
		return ""
	}
	return a.workspace.AssetDir(asset.AssetKind(kind), id)
}

func isSourceFile(path string) bool {
	name := strings.ToLower(filepath.Base(strings.TrimSpace(path)))
	return strings.HasSuffix(name, ".stencil.ts") || strings.HasSuffix(name, ".stencil.tsx")
}
