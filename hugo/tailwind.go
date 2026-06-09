package hugo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tailwindpkg "github.com/cleanstartup/stack/tailwind"
	"github.com/cleanstartup/stack/web"
)

const tailwindBundleFile = tailwindpkg.BundleFile

func (e *BuildEngine) buildStyleBundle(ctx context.Context, workspace *Workspace, cfg BuildConfig) error {
	if e == nil || e.builder == nil || e.builder.Styles() == nil || workspace == nil {
		return nil
	}
	bundle := e.builder.Styles().BundleRef()
	outputDir := workspace.OutputAssetDir(web.AssetKind(bundle.Kind), bundle.ID)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	cacheRoot := filepath.Join(filepath.Dir(workspace.Root), "tailwind-cache")
	outputPath := filepath.Join(outputDir, tailwindBundleFile)
	return tailwindpkg.Build(ctx, tailwindBuildWorkspaceAdapter{workspace: workspace}, cacheRoot, outputPath, e.builder.Styles().Inputs(), e.builder.Styles().ScanPaths(), tailwindpkg.Config{
		Binary:       cfg.TailwindBinary,
		Version:      cfg.TailwindVersion,
		CacheDir:     cfg.TailwindCacheDir,
		DownloadBase: cfg.TailwindDownloadBase,
		ProjectDir:   cfg.ProjectDir,
	})
}

func (e *BuildEngine) tailwindInput(workspace *Workspace) (string, error) {
	if e == nil || e.builder == nil || e.builder.Styles() == nil {
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
	return e.builder.Styles().Input(tailwindBuildWorkspaceAdapter{workspace: cacheWorkspace})
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

func tailwindSourcePaths(source any) ([]string, error) {
	if source == nil {
		return nil, nil
	}
	if provider, ok := source.(interface{ SourcePaths() []string }); ok {
		return append([]string{}, provider.SourcePaths()...), nil
	}
	if provider, ok := source.(interface{ WatchPaths() []string }); ok {
		return append([]string{}, provider.WatchPaths()...), nil
	}
	return nil, fmt.Errorf("unsupported tailwind source %T", source)
}

func runTailwind(ctx context.Context, cfg tailwindpkg.Config, inputPath, outputPath string) error {
	return tailwindpkg.Run(ctx, cfg, inputPath, outputPath)
}

func resolveTailwindBinary(ctx context.Context, cfg BuildConfig) (string, error) {
	return tailwindpkg.ResolveBinary(ctx, tailwindpkg.Config{
		Binary:       cfg.TailwindBinary,
		Version:      cfg.TailwindVersion,
		CacheDir:     cfg.TailwindCacheDir,
		DownloadBase: cfg.TailwindDownloadBase,
	})
}

func downloadTailwindBinary(ctx context.Context, cacheDir, version, downloadBase string) (string, error) {
	return tailwindpkg.ResolveBinary(ctx, tailwindpkg.Config{
		Version:      version,
		CacheDir:     cacheDir,
		DownloadBase: downloadBase,
	})
}
