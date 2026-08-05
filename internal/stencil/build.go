package stencil

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	assetspkg "github.com/cleanstartup/stack/asset"
)

type Config struct {
	Binary     string
	ProjectDir string
}

func Build(ctx context.Context, materializationWorkspace Workspace, cacheRoot, outputDir string, sources []Source, cfg Config) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(sources) == 0 {
		return nil
	}

	cacheWorkspace := newCacheWorkspace(cacheRoot)
	outputRoot := filepath.Join(outputDir, "assets", "js")
	start := time.Now()
	fmt.Fprintf(os.Stderr, "[stack] stencil build start inputs=%d output=%s\n", len(sources), outputRoot)
	if err := os.MkdirAll(outputRoot, 0o755); err != nil {
		return err
	}
	projectDir := strings.TrimSpace(cfg.ProjectDir)
	if projectDir == "" {
		if materializationWorkspace == nil {
			return fmt.Errorf("stencil workspace is nil")
		}
		if err := os.MkdirAll(cacheWorkspace.Root, 0o755); err != nil {
			return err
		}
		for _, dir := range []string{cacheWorkspace.Src, cacheWorkspace.Out, cacheWorkspace.Temp} {
			if err := os.RemoveAll(dir); err != nil {
				return err
			}
		}
		if err := cacheWorkspace.Prepare(); err != nil {
			return err
		}
		sourceFiles := sourceFiles(sources)
		srcDir := commonSourceDir(sourceFiles)
		include := []string{srcDir}
		if strings.TrimSpace(srcDir) == "" {
			include = nil
		}
		if err := os.WriteFile(filepath.Join(cacheWorkspace.Root, "stencil.config.ts"), []byte(configSource(srcDir, outputRoot)), 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(cacheWorkspace.Root, "tsconfig.json"), []byte(tsconfigSource(include...)), 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(cacheWorkspace.Root, "package.json"), []byte(packageSource()), 0o644); err != nil {
			return err
		}
		if err := ensureDependencies(ctx, cacheWorkspace.Root); err != nil {
			return err
		}
	}
	if err := run(ctx, cfg, cacheWorkspace.Root); err != nil {
		return err
	}
	outputBundleDir := filepath.Join(outputRoot, BundleID)
	if info, err := os.Stat(outputBundleDir); err != nil || !info.IsDir() {
		return fmt.Errorf("stencil output not found at %s", outputBundleDir)
	}
	fmt.Fprintf(os.Stderr, "[stack] stencil build complete duration=%s output=%s\n", time.Since(start).Round(time.Millisecond), outputBundleDir)
	return nil
}

func ResolveBinary(cfg Config) (string, error) {
	if path := strings.TrimSpace(cfg.Binary); path != "" {
		return path, nil
	}
	if path := strings.TrimSpace(os.Getenv("STACK_STENCIL_BINARY")); path != "" {
		return path, nil
	}
	if strings.TrimSpace(cfg.ProjectDir) != "" {
		return "npm", nil
	}
	return "npm", nil
}

func Run(ctx context.Context, cfg Config, workDir string) error {
	return run(ctx, cfg, workDir)
}

func WatchArgs(cfg Config) []string {
	binaryPath, _ := ResolveBinary(cfg)
	return commandArgs(binaryPath)
}

func DevCommand(cfg Config) (assetspkg.CommandSpec, error) {
	return commandSpec(cfg, true)
}

func run(ctx context.Context, cfg Config, workDir string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	spec, err := commandSpec(cfg, false)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, spec.Binary, spec.Args...)
	cmd.Dir = workDir
	if strings.TrimSpace(spec.WorkDir) != "" {
		cmd.Dir = spec.WorkDir
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("stencil build failed via %s: %w: %s", spec.Binary, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func CommandArgs(binaryPath string) []string {
	return commandArgs(binaryPath)
}

func commandSpec(cfg Config, watch bool) (assetspkg.CommandSpec, error) {
	projectDir := strings.TrimSpace(cfg.ProjectDir)
	if projectDir != "" && strings.TrimSpace(cfg.Binary) == "" && strings.TrimSpace(os.Getenv("STACK_STENCIL_BINARY")) == "" {
		if err := ensureProjectDependencies(context.Background(), projectDir); err != nil {
			return assetspkg.CommandSpec{}, err
		}
		absProjectDir, err := filepath.Abs(projectDir)
		if err != nil {
			return assetspkg.CommandSpec{}, err
		}
		binaryPath, err := filepath.Abs(filepath.Join(absProjectDir, "node_modules", ".bin", "stencil"))
		if err != nil {
			return assetspkg.CommandSpec{}, err
		}
		args := []string{"build"}
		if watch {
			args = append(args, "--watch")
		}
		args = append(args, "--config", "stencil.config.ts")
		return assetspkg.CommandSpec{
			Binary:  binaryPath,
			Args:    args,
			WorkDir: absProjectDir,
		}, nil
	}
	binaryPath, err := ResolveBinary(cfg)
	if err != nil {
		return assetspkg.CommandSpec{}, err
	}
	args := commandArgs(binaryPath)
	if watch {
		args = append(args, "--watch")
	}
	return assetspkg.CommandSpec{
		Binary: binaryPath,
		Args:   args,
	}, nil
}

func commandArgs(binaryPath string) []string {
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

func ConfigSource() string {
	return configSource("src/assets/js", "dist")
}

func configSource(srcDir, outDir string) string {
	srcDir = strings.TrimSpace(srcDir)
	if srcDir == "" {
		srcDir = "src/assets/js"
	}
	outDir = strings.TrimSpace(outDir)
	if outDir == "" {
		outDir = "dist"
	}
	// Derive the top-level output directory (e.g. "../../.assets/assets/js" → ".assets")
	// and build a regex that excludes it from the file watcher to prevent rebuild loops.
	outTopDir := filepath.ToSlash(outDir)
	for strings.HasPrefix(outTopDir, "../") {
		outTopDir = strings.TrimPrefix(outTopDir, "../")
	}
	if idx := strings.Index(outTopDir, "/"); idx >= 0 {
		outTopDir = outTopDir[:idx]
	}
	watchIgnored := ""
	if outTopDir != "" {
		watchIgnored = `
  watchIgnoredRegex: [/` + regexp.QuoteMeta(outTopDir) + `(\/|$)/],`
	}
	return `import type { Config } from '@stencil/core';

export const config: Config = {
  namespace: 'stack',
  srcDir: '` + filepath.ToSlash(srcDir) + `',` + watchIgnored + `
  outputTargets: [
    {
      type: 'dist',
      dir: '` + filepath.ToSlash(outDir) + `',
      esmLoaderPath: '../loader',
    },
  ],
};
`
}

func TSConfigSource() string {
	return tsconfigSource("src/assets/js")
}

func tsconfigSource(include ...string) string {
	if len(include) == 0 {
		include = []string{"src/assets/js"}
	}
	for idx, path := range include {
		include[idx] = filepath.ToSlash(strings.TrimSpace(path))
	}
	includeJSON, err := json.Marshal(include)
	if err != nil {
		includeJSON = []byte(`["src/assets/js"]`)
	}
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
  "include": ` + string(includeJSON) + `
}
`
}

func sourceFiles(sources []Source) []string {
	seen := map[string]struct{}{}
	var files []string
	for _, source := range sources {
		paths, err := SourcePaths(source)
		if err != nil {
			continue
		}
		for _, path := range paths {
			path = strings.TrimSpace(path)
			if path == "" {
				continue
			}
			path = filepath.Clean(path)
			if _, exists := seen[path]; exists {
				continue
			}
			seen[path] = struct{}{}
			files = append(files, path)
		}
	}
	return files
}

func commonSourceDir(files []string) string {
	if len(files) == 0 {
		return "src/assets/js"
	}
	common := filepath.Dir(files[0])
	for _, file := range files[1:] {
		dir := filepath.Dir(file)
		for common != "." && common != string(filepath.Separator) {
			rel, err := filepath.Rel(common, dir)
			if err == nil && !strings.HasPrefix(rel, "..") {
				break
			}
			next := filepath.Dir(common)
			if next == common {
				break
			}
			common = next
		}
	}
	return common
}

func PackageSource() string {
	return packageSource()
}

func packageSource() string {
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

func EnsureDependencies(ctx context.Context, workDir string) error {
	return ensureDependencies(ctx, workDir)
}

func ensureDependencies(ctx context.Context, workDir string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	marker := filepath.Join(workDir, "node_modules", "@stencil", "core", "package.json")
	if info, err := os.Stat(marker); err == nil && !info.IsDir() {
		fmt.Fprintf(os.Stderr, "[stack] stencil deps cache hit %s\n", workDir)
		return nil
	}
	fmt.Fprintf(os.Stderr, "[stack] stencil deps install %s\n", workDir)
	cmd := exec.CommandContext(ctx, "npm", "install", "--ignore-scripts")
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("stencil dependency install failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func ensureProjectDependencies(ctx context.Context, projectDir string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	projectDir = strings.TrimSpace(projectDir)
	if projectDir == "" {
		return nil
	}
	absProjectDir, err := filepath.Abs(projectDir)
	if err != nil {
		return err
	}
	if fileExists(filepath.Join(absProjectDir, "node_modules", ".package-lock.json")) &&
		fileExists(filepath.Join(absProjectDir, "node_modules", ".bin", "stencil")) {
		return nil
	}
	cmd := exec.CommandContext(ctx, "npm", "install", "--ignore-scripts", "--no-audit", "--no-fund")
	cmd.Dir = absProjectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("stencil project dependency install failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func EnsureProjectDependencies(ctx context.Context, projectDir string) error {
	return ensureProjectDependencies(ctx, projectDir)
}

type cacheWorkspace struct {
	Root string
	Src  string
	Out  string
	Temp string
}

func newCacheWorkspace(cacheRoot string) *cacheWorkspace {
	cacheRoot = strings.TrimSpace(cacheRoot)
	if cacheRoot == "" {
		cacheRoot = filepath.Join(".stack", "stencil-workspace")
	}
	return &cacheWorkspace{
		Root: cacheRoot,
		Src:  filepath.Join(cacheRoot, "src"),
		Out:  filepath.Join(cacheRoot, "dist"),
		Temp: filepath.Join(cacheRoot, "tmp"),
	}
}

func (w *cacheWorkspace) AssetDir(kind AssetKind, id string) string {
	if w == nil {
		return ""
	}
	return filepath.Join(w.Src, "assets", string(kind), id)
}

func (w *cacheWorkspace) Prepare() error {
	if w == nil {
		return fmt.Errorf("workspace is nil")
	}
	for _, dir := range []string{w.Root, w.Src, w.Out, w.Temp} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
