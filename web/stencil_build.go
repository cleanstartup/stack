package web

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	pipelinepkg "github.com/cleanstartup/stack/pipeline"
	stencilpkg "github.com/cleanstartup/stack/stencil"
	tailwindpkg "github.com/cleanstartup/stack/tailwind"
)

const stencilBundleID = stencilpkg.BundleID
const stencilBundleFile = stencilpkg.BundleFile

func (e *BuildEngine) buildStencilBundle(ctx context.Context, workspace *Workspace, cfg BuildConfig) error {
	if e == nil || e.builder == nil || e.builder.stencil == nil || workspace == nil {
		return nil
	}
	cacheRoot := filepath.Join(filepath.Dir(workspace.Root), "stencil-cache")
	return stencilpkg.Build(ctx, stencilWorkspaceAdapter{workspace: workspace}, cacheRoot, workspace.Out, e.builder.stencil.Inputs(), stencilpkg.Config{
		Binary:     cfg.StencilBinary,
		ProjectDir: cfg.ProjectDir,
	})
}

func (e *BuildEngine) syncStencilSourceMirror(workspaceDir string) error {
	if e == nil || e.builder == nil || e.builder.stencil == nil {
		return nil
	}
	workspaceAbs, err := filepath.Abs(workspaceDir)
	if err != nil {
		workspaceAbs = workspaceDir
	}
	cacheRoot := filepath.Join(filepath.Dir(workspaceAbs), "stencil-cache")
	stencilWorkspace := &Workspace{
		Root: cacheRoot,
		Src:  filepath.Join(cacheRoot, "src"),
		Out:  filepath.Join(cacheRoot, "dist"),
		Temp: filepath.Join(cacheRoot, "tmp"),
	}
	if err := os.MkdirAll(stencilWorkspace.Root, 0o755); err != nil {
		return err
	}
	for _, source := range e.builder.stencil.Inputs() {
		if source == nil {
			continue
		}
		sourceDir := stencilWorkspace.AssetDir(AssetKind(stencilpkg.AssetJS), source.ID())
		if err := os.RemoveAll(sourceDir); err != nil {
			return err
		}
		if _, err := source.Materialize(stencilWorkspaceAdapter{workspace: stencilWorkspace}, stencilpkg.AssetJS); err != nil {
			return err
		}
	}
	return nil
}

func (e *BuildEngine) syncTailwindInput(workspace *Workspace, inputPath string) error {
	if workspace == nil {
		return fmt.Errorf("tailwind workspace is nil")
	}
	if e == nil || e.builder == nil || e.builder.tailwind == nil {
		return nil
	}
	cacheRoot := filepath.Join(filepath.Dir(workspace.Root), "tailwind-cache")
	cacheWorkspace := &Workspace{
		Root: cacheRoot,
		Src:  filepath.Join(cacheRoot, "src"),
		Out:  filepath.Join(cacheRoot, "dist"),
		Temp: filepath.Join(cacheRoot, "tmp"),
	}
	if err := os.MkdirAll(cacheWorkspace.Src, 0o755); err != nil {
		return err
	}
	return e.builder.tailwind.WriteInputFile(tailwindBuildWorkspaceAdapter{workspace: cacheWorkspace}, inputPath)
}

func (e *BuildEngine) rebuildTailwindBundle(ctx context.Context, cfg DevConfig, workspace *Workspace) error {
	if e == nil || e.builder == nil || e.builder.tailwind == nil || workspace == nil {
		return nil
	}
	if len(e.builder.tailwind.Inputs()) == 0 {
		return nil
	}
	inputPath := filepath.Join(workspace.Root, "tailwind.input.css")
	if strings.TrimSpace(cfg.ProjectDir) != "" {
		absProjectDir, err := filepath.Abs(cfg.ProjectDir)
		if err != nil {
			return err
		}
		inputPath = filepath.Join(absProjectDir, "tailwind.input.css")
	}
	if err := e.syncTailwindInput(workspace, inputPath); err != nil {
		return err
	}
	outputPath := filepath.Join(cfg.OutputDir, "assets", "css", "app", tailwindBundleFile)
	fmt.Fprintf(os.Stderr, "[stack] dev tailwind rebuild input=%s output=%s\n", inputPath, outputPath)
	return tailwindpkg.Run(ctx, tailwindpkg.Config{ProjectDir: cfg.ProjectDir}, inputPath, outputPath)
}

func (e *BuildEngine) devTailwindSourceChanged(path string) bool {
	if e == nil || e.builder == nil || e.builder.tailwind == nil {
		return false
	}
	if !isTailwindSourceFile(path) {
		return false
	}
	for _, source := range e.builder.tailwind.Inputs() {
		paths, err := tailwindpkg.SourcePaths(source)
		if err != nil {
			continue
		}
		for _, candidate := range paths {
			if pipelinepkg.SourcePathMatches(candidate, path) {
				return true
			}
		}
	}
	return false
}

func (e *BuildEngine) devStencilSourceChanged(path string) bool {
	if e == nil || e.builder == nil || e.builder.stencil == nil {
		return false
	}
	if !isStencilSourceFile(path) {
		return false
	}
	for _, source := range e.builder.stencil.Inputs() {
		paths, err := stencilpkg.SourcePaths(source)
		if err != nil {
			continue
		}
		for _, candidate := range paths {
			if pipelinepkg.SourcePathMatches(candidate, path) {
				return true
			}
		}
	}
	return false
}

func isTailwindSourceFile(path string) bool {
	name := strings.ToLower(filepath.Base(strings.TrimSpace(path)))
	return strings.HasSuffix(name, ".tailwind.css")
}

func isStencilSourceFile(path string) bool {
	name := strings.ToLower(filepath.Base(strings.TrimSpace(path)))
	return strings.HasSuffix(name, ".stencil.ts") || strings.HasSuffix(name, ".stencil.tsx")
}

func (e *BuildEngine) devContentSourceChanged(path string) bool {
	if e == nil || e.builder == nil || e.builder.content == nil {
		return false
	}
	return e.builder.content.SourceChanged(path)
}

func (e *BuildEngine) devLayoutSourceChanged(path string) bool {
	if e == nil || e.builder == nil || e.builder.layouts == nil {
		return false
	}
	return e.builder.layouts.SourceChanged(path)
}

func (e *BuildEngine) devOutputWatchPaths(outputDir, stencilCacheRoot string) []string {
	var paths []string
	tailwindOutput := filepath.Join(outputDir, "assets", "css", "app", tailwindBundleFile)
	if _, err := os.Stat(tailwindOutput); err == nil {
		paths = append(paths, tailwindOutput)
	}
	stencilOutput := filepath.Join(stencilCacheRoot, "dist")
	if _, err := os.Stat(stencilOutput); err == nil {
		paths = append(paths, stencilOutput)
	}
	sort.Strings(paths)
	return paths
}

func (e *BuildEngine) syncDevOutputs(outputDir, stencilCacheRoot string) error {
	stencilSource := filepath.Join(stencilCacheRoot, "dist", stencilBundleID)
	stencilDest := filepath.Join(outputDir, "assets", "js", stencilBundleID)
	if info, err := os.Stat(stencilSource); err == nil && info.IsDir() {
		if err := os.RemoveAll(stencilDest); err != nil {
			return err
		}
		if err := os.MkdirAll(stencilDest, 0o755); err != nil {
			return err
		}
		if err := copyTree(stencilDest, stencilSource); err != nil {
			return err
		}
	}

	loaderSource := filepath.Join(stencilCacheRoot, "dist", "loader")
	loaderDest := filepath.Join(outputDir, "assets", "js", stencilBundleID, "loader")
	if info, err := os.Stat(loaderSource); err == nil && info.IsDir() {
		if err := os.RemoveAll(loaderDest); err != nil {
			return err
		}
		if err := os.MkdirAll(loaderDest, 0o755); err != nil {
			return err
		}
		if err := copyTree(loaderDest, loaderSource); err != nil {
			return err
		}
	}
	return nil
}
