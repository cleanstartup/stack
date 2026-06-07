package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const (
	tailwindDefaultDownloadBase = "https://github.com/tailwindlabs/tailwindcss/releases"
	tailwindDefaultAPIBase      = "https://api.github.com/repos/tailwindlabs/tailwindcss"
)

var tailwindBinaryResolveMu sync.Mutex

func (e *BuildEngine) buildStyleBundle(ctx context.Context, workspace *Workspace, cfg BuildConfig) error {
	if e == nil || e.builder == nil || e.builder.tailwind == nil || workspace == nil {
		return nil
	}
	if len(e.builder.tailwind.Inputs()) == 0 {
		return nil
	}

	bundle := e.builder.tailwind.BundleRef()
	outputDir := workspace.OutputAssetDir(bundle.Kind, bundle.ID)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	outputPath := filepath.Join(outputDir, tailwindBundleFile)
	tailwindWorkspace := newTailwindWorkspace(filepath.Join(filepath.Dir(workspace.Root), "tailwind-cache"))
	inputPath := filepath.Join(tailwindWorkspace.Root, "tailwind.input.css")
	if err := e.syncTailwindInput(tailwindWorkspace, inputPath); err != nil {
		return err
	}

	binaryPath, err := resolveTailwindBinary(ctx, cfg)
	if err != nil {
		return err
	}
	return runTailwind(ctx, binaryPath, inputPath, outputPath)
}

func (e *BuildEngine) tailwindInput(workspace *Workspace) (string, error) {
	if e == nil || e.builder == nil || e.builder.tailwind == nil {
		return "", nil
	}
	if workspace == nil {
		return "", fmt.Errorf("tailwind workspace is nil")
	}
	var out bytes.Buffer
	out.WriteString(`@import "tailwindcss";`)
	out.WriteString("\n")

	for _, scanPath := range e.builder.tailwind.ScanPaths() {
		scanPath = strings.TrimSpace(scanPath)
		if scanPath == "" {
			continue
		}
		out.WriteString(`@source "`)
		out.WriteString(filepath.ToSlash(scanPath))
		out.WriteString(`";`)
		out.WriteString("\n")
	}

	for _, source := range e.builder.tailwind.Inputs() {
		if source == nil {
			continue
		}
		paths, err := materializeTailwindSource(source, workspace, AssetKindCSS)
		if err != nil {
			return "", err
		}
		for _, sourcePath := range paths {
			if strings.TrimSpace(sourcePath) == "" {
				continue
			}
			contentPath := filepath.Join(workspace.AssetDir(AssetKindCSS, source.ID()), filepath.FromSlash(sourcePath))
			content, err := os.ReadFile(contentPath)
			if err != nil {
				return "", err
			}
			out.WriteString("\n/* way2go: ")
			out.WriteString(filepath.ToSlash(contentPath))
			out.WriteString(" */\n")
			out.Write(content)
			if len(content) == 0 || content[len(content)-1] != '\n' {
				out.WriteString("\n")
			}
		}
	}

	return out.String(), nil
}

func newTailwindWorkspace(cacheRoot string) *Workspace {
	cacheRoot = strings.TrimSpace(cacheRoot)
	if cacheRoot == "" {
		cacheRoot = filepath.Join(".way2go", "tailwind-cache")
	}
	return &Workspace{
		Root: cacheRoot,
		Src:  filepath.Join(cacheRoot, "src"),
		Out:  filepath.Join(cacheRoot, "dist"),
		Temp: filepath.Join(cacheRoot, "tmp"),
	}
}

func tailwindSourcePaths(source AssetSource) ([]string, error) {
	if source == nil {
		return nil, nil
	}
	switch typed := source.(type) {
	case watchedAssetSource:
		return tailwindSourcePaths(typed.source)
	case fileAssetSource:
		return []string{typed.path}, nil
	case dirAssetSource:
		return []string{typed.path}, nil
	case fileSetAssetSource:
		return append([]string{}, typed.files...), nil
	case fsAssetSource:
		if len(typed.watchPaths) > 0 {
			return append([]string{}, typed.watchPaths...), nil
		}
		return nil, nil
	case generatedAssetSource:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported tailwind source %T", source)
	}
}

func runTailwind(ctx context.Context, binaryPath, inputPath, outputPath string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, binaryPath, "-i", inputPath, "-o", outputPath, "--minify")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tailwind build failed via %s: %w: %s", binaryPath, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func resolveTailwindBinary(ctx context.Context, cfg BuildConfig) (string, error) {
	if path := strings.TrimSpace(cfg.TailwindBinary); path != "" {
		return path, nil
	}
	if path := strings.TrimSpace(os.Getenv("WAY2GO_TAILWIND_BINARY")); path != "" {
		return path, nil
	}

	version := strings.TrimSpace(cfg.TailwindVersion)
	if version == "" {
		version = strings.TrimSpace(os.Getenv("WAY2GO_TAILWIND_VERSION"))
	}
	if version == "" {
		version = "latest"
	}

	cacheDir := strings.TrimSpace(cfg.TailwindCacheDir)
	if cacheDir == "" {
		cacheDir = strings.TrimSpace(os.Getenv("WAY2GO_TAILWIND_CACHE_DIR"))
	}
	if cacheDir == "" {
		if userCacheDir, err := os.UserCacheDir(); err == nil && strings.TrimSpace(userCacheDir) != "" {
			cacheDir = filepath.Join(userCacheDir, "way2go", "tailwind")
		} else {
			cacheDir = filepath.Join(os.TempDir(), "way2go", "tailwind")
		}
	}

	downloadBase := strings.TrimSpace(cfg.TailwindDownloadBase)
	if downloadBase == "" {
		downloadBase = strings.TrimSpace(os.Getenv("WAY2GO_TAILWIND_DOWNLOAD_BASE"))
	}
	if downloadBase == "" {
		downloadBase = tailwindDefaultDownloadBase
	}

	return downloadTailwindBinary(ctx, cacheDir, version, downloadBase)
}

func downloadTailwindBinary(ctx context.Context, cacheDir, version, downloadBase string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	assetName, err := tailwindAssetName()
	if err != nil {
		return "", err
	}
	if version == "latest" {
		if cachedVersion, ok := loadCachedLatestTailwindVersion(cacheDir); ok {
			version = cachedVersion
		} else {
			version, err = resolveLatestTailwindVersion(ctx)
			if err != nil {
				return "", err
			}
			if err := saveCachedLatestTailwindVersion(cacheDir, version); err != nil {
				return "", err
			}
		}
	}
	version = normalizeTailwindVersion(version)

	targetDir := filepath.Join(cacheDir, version)
	targetPath := filepath.Join(targetDir, assetName)
	if info, err := os.Stat(targetPath); err == nil && !info.IsDir() {
		return targetPath, nil
	}

	tailwindBinaryResolveMu.Lock()
	defer tailwindBinaryResolveMu.Unlock()

	if info, err := os.Stat(targetPath); err == nil && !info.IsDir() {
		return targetPath, nil
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", err
	}

	downloadURL := fmt.Sprintf("%s/download/%s/%s", strings.TrimRight(downloadBase, "/"), version, assetName)
	if version == "latest" {
		downloadURL = fmt.Sprintf("%s/latest/download/%s", strings.TrimRight(downloadBase, "/"), assetName)
	}

	tempFile, err := os.CreateTemp(targetDir, assetName+".*.tmp")
	if err != nil {
		return "", err
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
	}()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", "way2go-web")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return "", fmt.Errorf("tailwind download failed: %s", response.Status)
	}

	if _, err := io.Copy(tempFile, response.Body); err != nil {
		return "", err
	}
	if err := tempFile.Chmod(0o755); err != nil {
		return "", err
	}
	if err := tempFile.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		return "", err
	}
	return targetPath, nil
}

func resolveLatestTailwindVersion(ctx context.Context) (string, error) {
	apiBase := strings.TrimSpace(os.Getenv("WAY2GO_TAILWIND_API_BASE"))
	if apiBase == "" {
		apiBase = tailwindDefaultAPIBase
	}
	apiURL := strings.TrimRight(apiBase, "/") + "/releases/latest"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", "way2go-web")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return "", fmt.Errorf("resolve latest tailwind version failed: %s", response.Status)
	}

	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", err
	}
	version := strings.TrimSpace(payload.TagName)
	if version == "" {
		return "", fmt.Errorf("latest tailwind release has no tag")
	}
	return version, nil
}

func loadCachedLatestTailwindVersion(cacheDir string) (string, bool) {
	cacheDir = strings.TrimSpace(cacheDir)
	if cacheDir == "" {
		return "", false
	}
	content, err := os.ReadFile(filepath.Join(cacheDir, "latest.version"))
	if err != nil {
		return "", false
	}
	version := strings.TrimSpace(string(content))
	if version == "" {
		return "", false
	}
	return version, true
}

func saveCachedLatestTailwindVersion(cacheDir, version string) error {
	cacheDir = strings.TrimSpace(cacheDir)
	version = strings.TrimSpace(version)
	if cacheDir == "" || version == "" {
		return nil
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cacheDir, "latest.version"), []byte(version), 0o644)
}

func tailwindAssetName() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		switch runtime.GOARCH {
		case "arm64":
			return "tailwindcss-macos-arm64", nil
		case "amd64":
			return "tailwindcss-macos-x64", nil
		}
	case "linux":
		switch runtime.GOARCH {
		case "arm64":
			return "tailwindcss-linux-arm64", nil
		case "amd64":
			return "tailwindcss-linux-x64", nil
		}
	case "windows":
		switch runtime.GOARCH {
		case "amd64":
			return "tailwindcss-windows-x64.exe", nil
		}
	}
	return "", fmt.Errorf("unsupported platform for tailwind: %s/%s", runtime.GOOS, runtime.GOARCH)
}

func normalizeTailwindVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" || version == "latest" {
		return "latest"
	}
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}

func materializeTailwindSource(source AssetSource, workspace *Workspace, kind AssetKind) ([]string, error) {
	if source == nil {
		return nil, nil
	}
	if workspace == nil {
		return nil, fmt.Errorf("workspace is nil")
	}

	switch typed := source.(type) {
	case watchedAssetSource:
		return materializeTailwindSource(typed.source, workspace, kind)
	case fileAssetSource:
		dstDir := workspace.AssetDir(kind, typed.ID())
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			return nil, err
		}
		name := filepath.Base(typed.path)
		if err := copyFile(filepath.Join(dstDir, name), typed.path); err != nil {
			return nil, err
		}
		return []string{name}, nil
	case dirAssetSource:
		dstDir := workspace.AssetDir(kind, typed.ID())
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			return nil, err
		}
		return copyDir(dstDir, typed.path)
	case fsAssetSource:
		dstDir := workspace.AssetDir(kind, typed.ID())
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			return nil, err
		}
		sub, err := fs.Sub(typed.source, typed.root)
		if err != nil {
			return nil, err
		}
		return copyFS(dstDir, sub)
	case generatedAssetSource:
		dstDir := workspace.AssetDir(kind, typed.ID())
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			return nil, err
		}
		if typed.write == nil {
			return nil, nil
		}
		if err := typed.write(dstDir); err != nil {
			return nil, err
		}
		return listFiles(dstDir)
	default:
		return source.Materialize(workspace, kind)
	}
}
