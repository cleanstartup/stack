package web

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tailwindpkg "github.com/cleanstartup/stack/tailwind"
)

const tailwindBundleFile = tailwindpkg.BundleFile

func tailwindOutputPath(outputRoot string) string {
	return filepath.Join(outputRoot, "assets", "css", tailwindpkg.BundleID, tailwindBundleFile)
}

func (e *BuildEngine) buildStyleBundle(ctx context.Context, workspace *Workspace, cfg BuildConfig) error {
	if e == nil || e.builder == nil || e.builder.tailwind == nil || workspace == nil {
		return nil
	}
	outputPath := tailwindOutputPath(workspace.Out)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	cacheRoot := filepath.Join(filepath.Dir(workspace.Root), "tailwind-cache")
	return tailwindpkg.Build(ctx, tailwindBuildWorkspaceAdapter{workspace: workspace}, cacheRoot, outputPath, e.builder.tailwind.Inputs(), e.builder.tailwind.ScanPaths(), tailwindpkg.Config{
		Binary:       cfg.TailwindBinary,
		Version:      cfg.TailwindVersion,
		CacheDir:     cfg.TailwindCacheDir,
		DownloadBase: cfg.TailwindDownloadBase,
		ProjectDir:   cfg.ProjectDir,
	})
}

func (e *BuildEngine) tailwindInput(workspace *Workspace) (string, error) {
	if e == nil || e.builder == nil || e.builder.tailwind == nil {
		return "", nil
	}
	if workspace == nil {
		return "", fmt.Errorf("tailwind workspace is nil")
	}
	cacheRoot := filepath.Join(filepath.Dir(workspace.Root), "tailwind-cache")
	cacheWorkspace := &Workspace{
		Root: cacheRoot,
		Src:  filepath.Join(cacheRoot, "src"),
		Out:  filepath.Join(cacheRoot, "dist"),
		Temp: filepath.Join(cacheRoot, "tmp"),
	}
	return e.builder.tailwind.Input(tailwindBuildWorkspaceAdapter{workspace: cacheWorkspace})
}

func newTailwindWorkspace(cacheRoot string) *Workspace {
	cacheRoot = strings.TrimSpace(cacheRoot)
	if cacheRoot == "" {
		cacheRoot = filepath.Join(".stack", "tailwind-cache")
	}
	return &Workspace{
		Root: cacheRoot,
		Src:  filepath.Join(cacheRoot, "src"),
		Out:  filepath.Join(cacheRoot, "dist"),
		Temp: filepath.Join(cacheRoot, "tmp"),
	}
}
