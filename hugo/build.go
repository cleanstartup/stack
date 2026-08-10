package hugo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	pluginpkg "github.com/cleanstartup/stack/plugin"
)

const (
	defaultDownloadBase = "https://github.com/gohugoio/hugo/releases"
	defaultAPIBase      = "https://api.github.com/repos/gohugoio/hugo"
)

// Config resolves the concrete hugo binary to run. URL/platform/version
// resolution and archive extraction stay here in this package; Bin (when
// set) only does download -> cache -> chmod -> atomic rename, exactly the
// same split tailwind uses (see internal/tailwind/build.go's Config/
// ResolveBinary) — the difference is hugo's GitHub release assets are
// .tar.gz archives, not a raw executable, so this package also extracts.
type Config struct {
	Binary       string
	Version      string
	CacheDir     string
	DownloadBase string
	// APIBase overrides the GitHub API host the "latest" version lookup
	// queries (STACK_HUGO_API_BASE). Independent from DownloadBase since
	// they're different hosts (api.github.com vs github.com) — a mirror or
	// hermetic test that redirects DownloadBase must also redirect this to
	// avoid a live github.com/api.github.com call slipping through.
	APIBase string
	// Bin is the shared binary-provisioning service (StageContext.Bin).
	// Falls back to a hugo-local BinProvider over CacheDir when nil (e.g.
	// direct package use outside the plugin.StageContext wiring).
	Bin pluginpkg.BinProvider
}

// ResolveBinary returns a local, executable hugo binary path: an explicit
// Config.Binary/STACK_HUGO_BINARY short-circuits everything else; otherwise
// a version is resolved (pinned or "latest", GitHub API + on-disk cache of
// the resolved "latest" the same way tailwind caches it), the matching
// platform release archive is downloaded via the shared BinProvider, and
// the "hugo" binary is extracted from it into a local cache dir.
func ResolveBinary(ctx context.Context, cfg Config) (string, error) {
	if path := strings.TrimSpace(cfg.Binary); path != "" {
		return path, nil
	}
	if path := strings.TrimSpace(os.Getenv("STACK_HUGO_BINARY")); path != "" {
		return path, nil
	}

	version := strings.TrimSpace(cfg.Version)
	if version == "" {
		version = strings.TrimSpace(os.Getenv("STACK_HUGO_VERSION"))
	}
	if version == "" {
		version = "latest"
	}

	cacheDir := strings.TrimSpace(cfg.CacheDir)
	if cacheDir == "" {
		cacheDir = strings.TrimSpace(os.Getenv("STACK_HUGO_CACHE_DIR"))
	}
	if cacheDir == "" {
		cacheDir = pluginpkg.DefaultBinCacheDir()
	}

	downloadBase := strings.TrimSpace(cfg.DownloadBase)
	if downloadBase == "" {
		downloadBase = strings.TrimSpace(os.Getenv("STACK_HUGO_DOWNLOAD_BASE"))
	}
	if downloadBase == "" {
		downloadBase = defaultDownloadBase
	}

	apiBase := strings.TrimSpace(cfg.APIBase)
	if apiBase == "" {
		apiBase = strings.TrimSpace(os.Getenv("STACK_HUGO_API_BASE"))
	}
	if apiBase == "" {
		apiBase = defaultAPIBase
	}

	return resolveBinaryDownload(ctx, cfg.Bin, cacheDir, version, downloadBase, apiBase)
}

// resolveBinaryDownload resolves the concrete release tag + archive URL,
// delegates the archive download to the shared BinProvider (download ->
// cache -> chmod -> atomic rename, identical contract tailwind uses for its
// raw binary), then extracts the "hugo" executable out of that archive into
// a local, idempotent extraction cache.
func resolveBinaryDownload(ctx context.Context, bin pluginpkg.BinProvider, cacheDir, version, downloadBase, apiBase string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if version == "latest" {
		if cached, ok := loadCachedLatestVersion(cacheDir); ok {
			version = cached
		} else {
			resolved, err := resolveLatestVersion(ctx, apiBase)
			if err != nil {
				return "", err
			}
			if err := saveCachedLatestVersion(cacheDir, resolved); err != nil {
				return "", err
			}
			version = resolved
		}
	}
	tagVersion := normalizeTagVersion(version)
	fileVersion := strings.TrimPrefix(tagVersion, "v")

	assetFile, err := releaseAssetName(fileVersion)
	if err != nil {
		return "", err
	}

	if bin == nil {
		bin = pluginpkg.NewBinProvider(cacheDir)
	}

	archiveURL := buildReleaseURL(downloadBase, tagVersion, assetFile)
	archivePath, err := bin.Ensure(ctx, pluginpkg.BinarySpec{Name: "hugo-archive", Version: tagVersion, URL: archiveURL})
	if err != nil {
		return "", err
	}

	extractDir := filepath.Join(cacheDir, "hugo", tagVersion, "extracted")
	return ensureExtractedBinary(archivePath, extractDir)
}

// releaseAssetName mirrors tailwind's assetName() (internal/tailwind/
// build.go) but, unlike tailwind's platform-only names, hugo's release
// filenames embed the (unprefixed) version. Same platform scope as
// tailwind's own release matrix (darwin universal + linux amd64/arm64) —
// windows is left unsupported for the same reason tailwind's assetName
// leaves it unsupported: no released binary name mapped here yet, not a
// fundamental limitation of the approach.
func releaseAssetName(fileVersion string) (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return fmt.Sprintf("hugo_%s_darwin-universal.tar.gz", fileVersion), nil
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return fmt.Sprintf("hugo_%s_linux-amd64.tar.gz", fileVersion), nil
		case "arm64":
			return fmt.Sprintf("hugo_%s_linux-arm64.tar.gz", fileVersion), nil
		}
	}
	return "", fmt.Errorf("hugo: unsupported platform %s/%s", runtime.GOOS, runtime.GOARCH)
}

func buildReleaseURL(base, tagVersion, assetName string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = defaultDownloadBase
	}
	tagVersion = strings.TrimSpace(tagVersion)
	if tagVersion == "" {
		tagVersion = "latest"
	}
	return fmt.Sprintf("%s/download/%s/%s", strings.TrimRight(base, "/"), tagVersion, assetName)
}

func resolveLatestVersion(ctx context.Context, apiBase string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	apiBase = strings.TrimSpace(apiBase)
	if apiBase == "" {
		apiBase = defaultAPIBase
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/releases/latest", apiBase), nil)
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
		return "", fmt.Errorf("hugo latest version lookup failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.TagName == "" {
		return "", fmt.Errorf("hugo latest version lookup returned empty tag")
	}
	return payload.TagName, nil
}

func normalizeTagVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return "latest"
	}
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}

// loadCachedLatestVersion/saveCachedLatestVersion use a hugo-specific
// filename (not tailwind's "latest.version") because DefaultBinCacheDir is
// a shared root across plugins (see plugin.DefaultBinCacheDir) — a shared
// filename would split-brain the two plugins' independently-resolved
// "latest" pointers into the same file.
func loadCachedLatestVersion(cacheDir string) (string, bool) {
	path := filepath.Join(cacheDir, "hugo-latest.version")
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
	return os.WriteFile(filepath.Join(cacheDir, "hugo-latest.version"), []byte(strings.TrimSpace(version)+"\n"), 0o644)
}
