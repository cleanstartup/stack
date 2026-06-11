package tailwind

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	assetspkg "github.com/cleanstartup/stack/asset"
)

const (
	defaultDownloadBase = "https://github.com/tailwindlabs/tailwindcss/releases"
	defaultAPIBase      = "https://api.github.com/repos/tailwindlabs/tailwindcss"
)

type Config struct {
	Binary       string
	Version      string
	CacheDir     string
	DownloadBase string
	ProjectDir   string
}

func (r *Registry) Input(workspace Workspace) (string, error) {
	if r == nil {
		return "", nil
	}
	var out bytes.Buffer
	out.WriteString(`@import "tailwindcss";`)
	out.WriteString("\n")

	for _, scanPath := range r.ScanPaths() {
		scanPath = strings.TrimSpace(scanPath)
		if scanPath == "" {
			continue
		}
		out.WriteString(`@source "`)
		out.WriteString(filepath.ToSlash(scanPath))
		out.WriteString(`";`)
		out.WriteString("\n")
	}

	for _, source := range r.Inputs() {
		if source == nil {
			continue
		}
		paths, err := materializeSource(source, workspace)
		if err != nil {
			return "", err
		}
		baseDir := workspace.AssetDir(AssetCSS, source.ID())
		for _, sourcePath := range paths {
			if strings.TrimSpace(sourcePath) == "" {
				continue
			}
			contentPath := filepath.Clean(filepath.FromSlash(sourcePath))
			if !filepath.IsAbs(contentPath) {
				if _, err := os.Stat(contentPath); err != nil {
					contentPath = filepath.Join(baseDir, filepath.FromSlash(sourcePath))
				} else if !strings.HasPrefix(filepath.ToSlash(contentPath), "./") && !strings.HasPrefix(filepath.ToSlash(contentPath), "../") {
					contentPath = "." + string(filepath.Separator) + contentPath
				}
			}
			out.WriteString("\n@import \"")
			out.WriteString(filepath.ToSlash(contentPath))
			out.WriteString("\";\n")
		}
	}

	return out.String(), nil
}

func (r *Registry) WriteInputFile(workspace Workspace, dst string) error {
	if workspace == nil {
		return fmt.Errorf("tailwind workspace is nil")
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	input, err := r.Input(workspace)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, []byte(input), 0o644)
}

func Build(ctx context.Context, materializationWorkspace Workspace, cacheRoot, outputPath string, sources []Source, scans []string, cfg Config) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if materializationWorkspace == nil {
		return fmt.Errorf("tailwind workspace is nil")
	}
	if len(sources) == 0 {
		return nil
	}

	cacheWorkspace := newCacheWorkspace(cacheRoot)
	inputPath := filepath.Join(cacheWorkspace.Root, "tailwind.input.css")
	projectDir := strings.TrimSpace(cfg.ProjectDir)
	if projectDir != "" {
		absProjectDir, err := filepath.Abs(projectDir)
		if err != nil {
			return err
		}
		inputPath = filepath.Join(absProjectDir, "tailwind.input.css")
	}
	reg := &Registry{
		entries: make([]inputEntry, 0, len(sources)),
		scans:   append([]string{}, scans...),
		bundle:  AssetRef{Kind: AssetCSS, ID: BundleID, Files: []string{BundleFile}},
	}
	for _, src := range sources {
		reg.entries = append(reg.entries, inputEntry{source: src})
	}
	if projectDir == "" {
		if err := os.MkdirAll(cacheWorkspace.Root, 0o755); err != nil {
			return err
		}
		if err := os.MkdirAll(cacheWorkspace.Src, 0o755); err != nil {
			return err
		}
	}
	input, err := reg.Input(materializationWorkspace)
	if err != nil {
		return err
	}
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		return err
	}

	return run(ctx, cfg, inputPath, outputPath)
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
		cacheRoot = filepath.Join(".stack", "tailwind-cache")
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

func materializeSource(source Source, workspace Workspace) ([]string, error) {
	if source == nil {
		return nil, nil
	}
	if paths, err := SourcePaths(source); err == nil && len(paths) > 0 {
		return paths, nil
	}
	return source.Materialize(workspace, AssetCSS)
}

func run(ctx context.Context, cfg Config, inputPath, outputPath string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	spec, err := commandSpec(ctx, cfg, inputPath, outputPath, false)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, spec.Binary, spec.Args...)
	if strings.TrimSpace(spec.WorkDir) != "" {
		cmd.Dir = spec.WorkDir
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tailwind build failed via %s: %w: %s", spec.Binary, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func Run(ctx context.Context, cfg Config, inputPath, outputPath string) error {
	return run(ctx, cfg, inputPath, outputPath)
}

func WatchArgs(cfg Config, inputPath, outputPath string) []string {
	return []string{"-i", inputPath, "-o", outputPath, "--watch", "--minify"}
}

func DevCommand(ctx context.Context, cfg Config, inputPath, outputPath string) (assetspkg.CommandSpec, error) {
	return commandSpec(ctx, cfg, inputPath, outputPath, true)
}

func ResolveBinary(ctx context.Context, cfg Config) (string, error) {
	if path := strings.TrimSpace(cfg.Binary); path != "" {
		return path, nil
	}
	if path := strings.TrimSpace(os.Getenv("STACK_TAILWIND_BINARY")); path != "" {
		return path, nil
	}
	if strings.TrimSpace(cfg.ProjectDir) != "" {
		return "npm", nil
	}

	version := strings.TrimSpace(cfg.Version)
	if version == "" {
		version = strings.TrimSpace(os.Getenv("STACK_TAILWIND_VERSION"))
	}
	if version == "" {
		version = "latest"
	}

	cacheDir := strings.TrimSpace(cfg.CacheDir)
	if cacheDir == "" {
		cacheDir = strings.TrimSpace(os.Getenv("STACK_TAILWIND_CACHE_DIR"))
	}
	if cacheDir == "" {
		if userCacheDir, err := os.UserCacheDir(); err == nil && strings.TrimSpace(userCacheDir) != "" {
			cacheDir = filepath.Join(userCacheDir, "stack", "tailwind")
		} else {
			cacheDir = filepath.Join(os.TempDir(), "stack", "tailwind")
		}
	}

	downloadBase := strings.TrimSpace(cfg.DownloadBase)
	if downloadBase == "" {
		downloadBase = strings.TrimSpace(os.Getenv("STACK_TAILWIND_DOWNLOAD_BASE"))
	}
	if downloadBase == "" {
		downloadBase = defaultDownloadBase
	}

	return downloadBinary(ctx, cacheDir, version, downloadBase)
}

func commandSpec(ctx context.Context, cfg Config, inputPath, outputPath string, watch bool) (assetspkg.CommandSpec, error) {
	projectDir := strings.TrimSpace(cfg.ProjectDir)
	if projectDir != "" && strings.TrimSpace(cfg.Binary) == "" && strings.TrimSpace(os.Getenv("STACK_TAILWIND_BINARY")) == "" {
		if err := ensureProjectDependencies(ctx, projectDir); err != nil {
			return assetspkg.CommandSpec{}, err
		}
		absProjectDir, err := filepath.Abs(projectDir)
		if err != nil {
			return assetspkg.CommandSpec{}, err
		}
		binaryPath := filepath.Join(absProjectDir, "node_modules", ".bin", "tailwindcss")
		args := []string{"-i", inputPath, "-o", outputPath}
		if watch {
			args = append(args, "--watch")
		}
		args = append(args, "--minify")
		return assetspkg.CommandSpec{
			Binary:  binaryPath,
			Args:    args,
			WorkDir: absProjectDir,
		}, nil
	}
	binaryPath, err := ResolveBinary(ctx, cfg)
	if err != nil {
		return assetspkg.CommandSpec{}, err
	}
	args := []string{"-i", inputPath, "-o", outputPath}
	if watch {
		args = append(args, "--watch")
	}
	args = append(args, "--minify")
	return assetspkg.CommandSpec{
		Binary: binaryPath,
		Args:   args,
	}, nil
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
	if marker := filepath.Join(absProjectDir, "node_modules", ".package-lock.json"); fileExists(marker) {
		if fileExists(filepath.Join(absProjectDir, "node_modules", ".bin", "tailwindcss")) {
			return nil
		}
	}
	cmd := exec.CommandContext(ctx, "npm", "install", "--ignore-scripts", "--no-audit", "--no-fund")
	cmd.Dir = absProjectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tailwind dependency install failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func EnsureProjectDependencies(ctx context.Context, projectDir string) error {
	return ensureProjectDependencies(ctx, projectDir)
}

func downloadBinary(ctx context.Context, cacheDir, version, downloadBase string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	assetName, err := assetName()
	if err != nil {
		return "", err
	}
	if version == "latest" {
		if cachedVersion, ok := loadCachedLatestVersion(cacheDir); ok {
			version = cachedVersion
		} else {
			version, err = resolveLatestVersion(ctx)
			if err != nil {
				return "", err
			}
			if err := saveCachedLatestVersion(cacheDir, version); err != nil {
				return "", err
			}
		}
	}
	version = normalizeVersion(version)

	targetDir := filepath.Join(cacheDir, version)
	targetPath := filepath.Join(targetDir, assetName)
	if info, err := os.Stat(targetPath); err == nil && !info.IsDir() {
		return targetPath, nil
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", err
	}

	releaseURL := buildReleaseURL(downloadBase, version, assetName)
	tmpPath := targetPath + ".download"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("tailwind download failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	file, err := os.Create(tmpPath)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(file, resp.Body); err != nil {
		file.Close()
		_ = os.Remove(tmpPath)
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	return targetPath, nil
}

func assetName() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		switch runtime.GOARCH {
		case "amd64":
			return "tailwindcss-macos-x64", nil
		case "arm64":
			return "tailwindcss-macos-arm64", nil
		}
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return "tailwindcss-linux-x64", nil
		case "arm64":
			return "tailwindcss-linux-arm64", nil
		}
	}
	return "", fmt.Errorf("unsupported tailwind platform %s/%s", runtime.GOOS, runtime.GOARCH)
}

func buildReleaseURL(base, version, assetName string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = defaultDownloadBase
	}
	version = strings.TrimSpace(version)
	if version == "" {
		version = "latest"
	}
	return fmt.Sprintf("%s/download/%s/%s", strings.TrimRight(base, "/"), version, assetName)
}

func resolveLatestVersion(ctx context.Context) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/releases/latest", defaultAPIBase), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("tailwind latest version lookup failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.TagName == "" {
		return "", fmt.Errorf("tailwind latest version lookup returned empty tag")
	}
	return payload.TagName, nil
}

func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return "latest"
	}
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}

func loadCachedLatestVersion(cacheDir string) (string, bool) {
	path := filepath.Join(cacheDir, "latest.version")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	version := strings.TrimSpace(string(data))
	if version == "" {
		return "", false
	}
	return version, true
}

func saveCachedLatestVersion(cacheDir, version string) error {
	if strings.TrimSpace(cacheDir) == "" || strings.TrimSpace(version) == "" {
		return nil
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cacheDir, "latest.version"), []byte(strings.TrimSpace(version)+"\n"), 0o644)
}

func CopyAsset(src, dst string) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	output, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer output.Close()
	if _, err := io.Copy(output, input); err != nil {
		return err
	}
	return output.Close()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
