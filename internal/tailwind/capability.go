package tailwind

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cleanstartup/stack/asset"
	"github.com/cleanstartup/stack/internal/capability"
	"github.com/cleanstartup/stack/internal/pipeline"
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
	return filepath.Join(outputRoot, "assets", "css", BundleID, BundleFile)
}

func (c Capability) Install(ctx context.Context, cfg capability.Context) error {
	_ = ctx
	if c.empty() {
		return nil
	}
	inputPath := ""
	if strings.TrimSpace(cfg.ProjectDir) != "" {
		inputPath = filepath.Join(cfg.ProjectDir, "tailwind.input.css")
	} else if cfg.Workspace != nil {
		inputPath = filepath.Join(c.cacheWorkspace(cfg).Root, "tailwind.input.css")
	}
	if strings.TrimSpace(inputPath) == "" {
		return nil
	}
	return c.writeInput(cfg, inputPath)
}

func (c Capability) Build(ctx context.Context, cfg capability.Context) error {
	if c.empty() || cfg.Workspace == nil {
		return nil
	}
	outputPath := OutputPath(cfg.Workspace.OutputDir())
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	return Build(ctx, workspaceAdapter{workspace: cfg.Workspace}, c.cacheRoot(cfg), outputPath, c.registry.Inputs(), c.registry.ScanPaths(), c.config(cfg))
}

func (c Capability) Dev(ctx context.Context, cfg capability.Context) ([]pipeline.WatchWorker, error) {
	if c.empty() || cfg.Workspace == nil {
		return nil, nil
	}
	inputPath, err := c.inputPath(cfg)
	if err != nil {
		return nil, err
	}
	if err := c.writeInput(cfg, inputPath); err != nil {
		return nil, err
	}
	outputPath := OutputPath(cfg.OutputDir)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return nil, err
	}
	spec, err := DevCommand(ctx, c.config(cfg), inputPath, outputPath)
	if err != nil {
		return nil, err
	}
	worker, err := pipeline.StartCommandWatchSpec(ctx, "tailwind", spec)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(os.Stderr, "[stack] tailwind watch started input=%s output=%s\n", inputPath, outputPath)
	return []pipeline.WatchWorker{worker}, nil
}

func (c Capability) Register(target capability.Target) {
	if target == nil || c.empty() {
		return
	}
	ref := c.registry.BundleRef()
	target.RegisterCSS(asset.AssetRef{Kind: asset.AssetKind(ref.Kind), ID: ref.ID, Files: append([]string{}, ref.Files...)})
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

func (c Capability) Rebuild(ctx context.Context, cfg capability.Context) error {
	if c.empty() || cfg.Workspace == nil {
		return nil
	}
	inputPath, err := c.inputPath(cfg)
	if err != nil {
		return err
	}
	if err := c.writeInput(cfg, inputPath); err != nil {
		return err
	}
	outputPath := OutputPath(cfg.OutputDir)
	fmt.Fprintf(os.Stderr, "[stack] dev tailwind rebuild input=%s output=%s\n", inputPath, outputPath)
	return Run(ctx, c.config(cfg), inputPath, outputPath)
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

func (c Capability) inputPath(ctx capability.Context) (string, error) {
	if strings.TrimSpace(ctx.ProjectDir) == "" {
		workspace := c.cacheWorkspace(ctx)
		if workspace == nil {
			return "", fmt.Errorf("tailwind workspace is nil")
		}
		return filepath.Join(workspace.Root, "tailwind.input.css"), nil
	}
	absProjectDir, err := filepath.Abs(ctx.ProjectDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(absProjectDir, "tailwind.input.css"), nil
}

func (c Capability) writeInput(ctx capability.Context, inputPath string) error {
	return c.registry.WriteInputFile(workspaceAdapter{workspace: ctx.Workspace}, inputPath)
}

func (c Capability) cacheRoot(ctx capability.Context) string {
	if ctx.Workspace == nil {
		return filepath.Join(".stack", "tailwind-cache")
	}
	return filepath.Join(filepath.Dir(ctx.Workspace.RootDir()), "tailwind-cache")
}

func (c Capability) cacheWorkspace(ctx capability.Context) *cacheWorkspace {
	return newCacheWorkspace(c.cacheRoot(ctx))
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
	return strings.HasSuffix(name, ".tailwind.css")
}
