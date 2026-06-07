package web

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func (e *BuildEngine) buildStencilBundle(ctx context.Context, workspace *Workspace, cfg BuildConfig) error {
	if e == nil || e.builder == nil || e.builder.stencil == nil || workspace == nil {
		return nil
	}
	if len(e.builder.stencil.Inputs()) == 0 {
		return nil
	}

	cacheRoot := filepath.Join(filepath.Dir(workspace.Root), "stencil-cache")
	start := time.Now()
	fmt.Fprintf(os.Stderr, "[stack] stencil build start inputs=%d cache=%s\n", len(e.builder.stencil.Inputs()), cacheRoot)
	stencilWorkspace := &Workspace{
		Root: cacheRoot,
		Src:  filepath.Join(cacheRoot, "src"),
		Out:  filepath.Join(cacheRoot, "dist"),
		Temp: filepath.Join(cacheRoot, "tmp"),
	}
	if err := os.MkdirAll(stencilWorkspace.Root, 0o755); err != nil {
		return err
	}
	for _, dir := range []string{stencilWorkspace.Src, stencilWorkspace.Out, stencilWorkspace.Temp} {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
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
	tsconfigPath := filepath.Join(stencilWorkspace.Root, "tsconfig.json")
	if err := os.WriteFile(tsconfigPath, []byte(stencilTSConfigSource()), 0o644); err != nil {
		return err
	}
	packagePath := filepath.Join(stencilWorkspace.Root, "package.json")
	if err := os.WriteFile(packagePath, []byte(stencilPackageSource()), 0o644); err != nil {
		return err
	}

	if err := ensureStencilDependencies(ctx, stencilWorkspace.Root); err != nil {
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
	fmt.Fprintf(os.Stderr, "[stack] stencil build complete duration=%s output=%s\n", time.Since(start).Round(time.Millisecond), outputDir)

	return nil
}

func resolveStencilBinary(cfg BuildConfig) (string, error) {
	if path := strings.TrimSpace(cfg.StencilBinary); path != "" {
		return path, nil
	}
	if path := strings.TrimSpace(os.Getenv("STACK_STENCIL_BINARY")); path != "" {
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
  namespace: 'stack',
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

func stencilTSConfigSource() string {
	return `{
  "compilerOptions": {
    "allowSyntheticDefaultImports": true,
    "declaration": false,
    "experimentalDecorators": true,
    "jsx": "react",
    "jsxFactory": "h",
    "lib": ["dom", "es2017"],
    "module": "esnext",
    "moduleResolution": "node",
    "target": "es2017"
  },
  "include": ["src/assets/js"]
}
`
}

func stencilPackageSource() string {
	data, _ := json.MarshalIndent(map[string]any{
		"name":    "stack-stencil-workspace",
		"private": true,
		"version": "0.0.0",
		"devDependencies": map[string]string{
			"@stencil/core": "4.43.5",
		},
	}, "", "  ")
	return string(data) + "\n"
}

func ensureStencilDependencies(ctx context.Context, workDir string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	marker := filepath.Join(workDir, "node_modules", "@stencil", "core", "package.json")
	if info, err := os.Stat(marker); err == nil && !info.IsDir() {
		fmt.Fprintf(os.Stderr, "[stack] stencil deps cache hit %s\n", workDir)
		return nil
	}
	fmt.Fprintf(os.Stderr, "[stack] stencil deps install %s\n", workDir)
	cmd := exec.CommandContext(ctx, "npm", "install", "--no-package-lock", "--ignore-scripts")
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("stencil dependency install failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
