package web

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	stencilpkg "github.com/cleanstartup/stack/stencil"
	tailwindpkg "github.com/cleanstartup/stack/tailwind"
)

const stencilBundleID = stencilpkg.BundleID
const stencilBundleFile = stencilpkg.BundleFile

func (e *BuildEngine) buildStencilBundle(ctx context.Context, workspace *Workspace, cfg BuildConfig) error {
	if e == nil || e.builder == nil || e.builder.stencil == nil || workspace == nil {
		return nil
	}
	workRoot := filepath.Join(filepath.Dir(workspace.Root), "stencil-workspace")
	return stencilpkg.Build(ctx, stencilWorkspaceAdapter{workspace: workspace}, workRoot, workspace.Out, e.builder.stencil.Inputs(), stencilpkg.Config{
		Binary:     cfg.StencilBinary,
		ProjectDir: cfg.ProjectDir,
	})
}

func (e *BuildEngine) syncTailwindInput(workspace *Workspace, inputPath string) error {
	if workspace == nil {
		return fmt.Errorf("tailwind workspace is nil")
	}
	if e == nil || e.builder == nil || e.builder.tailwind == nil {
		return nil
	}
	return e.builder.tailwind.WriteInputFile(tailwindBuildWorkspaceAdapter{workspace: workspace}, inputPath)
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
	outputPath := tailwindOutputPath(cfg.OutputDir)
	fmt.Fprintf(os.Stderr, "[stack] dev tailwind rebuild input=%s output=%s\n", inputPath, outputPath)
	return tailwindpkg.Run(ctx, tailwindpkg.Config{ProjectDir: cfg.ProjectDir}, inputPath, outputPath)
}

func sourcePathsFromTailwind(sources []tailwindpkg.Source) []string {
	var paths []string
	for _, source := range sources {
		sourcePaths, err := tailwindpkg.SourcePaths(source)
		if err != nil {
			continue
		}
		paths = append(paths, sourcePaths...)
	}
	return paths
}

func sourcePathsFromStencil(sources []stencilpkg.Source) []string {
	var paths []string
	for _, source := range sources {
		sourcePaths, err := stencilpkg.SourcePaths(source)
		if err != nil {
			continue
		}
		paths = append(paths, sourcePaths...)
	}
	return paths
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

func (e *BuildEngine) devOutputWatchPaths(outputDir string) []string {
	var paths []string
	tailwindOutput := tailwindOutputPath(outputDir)
	if _, err := os.Stat(tailwindOutput); err == nil {
		paths = append(paths, tailwindOutput)
	}
	stencilOutput := filepath.Join(outputDir, "assets", "js", stencilBundleID)
	if _, err := os.Stat(stencilOutput); err == nil {
		paths = append(paths, stencilOutput)
	}
	sort.Strings(paths)
	return paths
}
