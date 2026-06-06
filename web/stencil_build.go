package web

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (e *BuildEngine) buildStencilBundle(ctx context.Context, workspace *Workspace, cfg BuildConfig) error {
	if e == nil || e.builder == nil || e.builder.stencil == nil || workspace == nil {
		return nil
	}
	if len(e.builder.stencil.Inputs()) == 0 {
		return nil
	}

	stencilWorkspace := &Workspace{
		Root: filepath.Join(workspace.Temp, "stencil"),
		Src:  filepath.Join(workspace.Temp, "stencil", "src"),
		Out:  filepath.Join(workspace.Temp, "stencil", "dist"),
		Temp: filepath.Join(workspace.Temp, "stencil", "tmp"),
	}
	if err := stencilWorkspace.Prepare(); err != nil {
		return err
	}

	for _, source := range e.builder.stencil.Inputs() {
		if source == nil {
			continue
		}
		if _, err := source.Materialize(stencilWorkspace, AssetKindJS); err != nil {
			return err
		}
	}

	configPath := filepath.Join(stencilWorkspace.Root, "stencil.config.ts")
	if err := os.WriteFile(configPath, []byte(stencilConfigSource()), 0o644); err != nil {
		return err
	}

	binaryPath, err := resolveStencilBinary(cfg)
	if err != nil {
		return err
	}
	if err := runStencil(ctx, binaryPath, stencilWorkspace.Root); err != nil {
		return err
	}

	distRoot := filepath.Join(stencilWorkspace.Root, "dist")
	sourceDir := filepath.Join(distRoot, stencilBundleID)
	if info, err := os.Stat(sourceDir); err != nil || !info.IsDir() {
		return fmt.Errorf("stencil output not found at %s", sourceDir)
	}

	outputDir := workspace.OutputAssetDir(AssetKindJS, stencilBundleID)
	if err := os.RemoveAll(outputDir); err != nil {
		return err
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	if err := copyTree(outputDir, sourceDir); err != nil {
		return err
	}

	loaderDir := filepath.Join(distRoot, "loader")
	if info, err := os.Stat(loaderDir); err == nil && info.IsDir() {
		if err := copyTree(filepath.Join(outputDir, "loader"), loaderDir); err != nil {
			return err
		}
	}

	return nil
}

func resolveStencilBinary(cfg BuildConfig) (string, error) {
	if path := strings.TrimSpace(cfg.StencilBinary); path != "" {
		return path, nil
	}
	if path := strings.TrimSpace(os.Getenv("WAY2GO_STENCIL_BINARY")); path != "" {
		return path, nil
	}
	return "npm", nil
}

func runStencil(ctx context.Context, binaryPath, workDir string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	args := stencilCommandArgs(binaryPath)
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("stencil build failed via %s: %w: %s", binaryPath, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func stencilCommandArgs(binaryPath string) []string {
	base := strings.ToLower(filepath.Base(binaryPath))
	switch base {
	case "npm":
		return []string{"exec", "--yes", "--package=@stencil/core", "--", "stencil", "build"}
	case "npx":
		return []string{"--yes", "stencil", "build"}
	default:
		return []string{"build"}
	}
}

func stencilConfigSource() string {
	return `import type { Config } from '@stencil/core';

export const config: Config = {
  namespace: 'app',
  srcDir: 'src/assets/js',
  outputTargets: [
    {
      type: 'dist',
      esmLoaderPath: '../loader',
    },
  ],
};
`
}
