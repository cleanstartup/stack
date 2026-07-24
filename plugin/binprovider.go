package plugin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// NewBinProvider returns a BinProvider that caches downloaded binaries under
// cacheRoot/<Name>/<Version>/<Name>. This is the reusable core extracted from
// tailwind's former downloadBinary: URL/platform/latest-version resolution
// stays plugin-specific and is baked into BinarySpec.URL by the caller.
func NewBinProvider(cacheRoot string) BinProvider {
	return &binProvider{cacheRoot: cacheRoot}
}

type binProvider struct {
	cacheRoot string
}

func (b *binProvider) Ensure(ctx context.Context, spec BinarySpec) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return "", fmt.Errorf("plugin: binary spec name is empty")
	}
	url := strings.TrimSpace(spec.URL)
	if url == "" {
		return "", fmt.Errorf("plugin: binary spec url is empty")
	}
	version := strings.TrimSpace(spec.Version)
	if version == "" {
		version = "latest"
	}

	targetDir := filepath.Join(b.cacheRoot, name, version)
	targetPath := filepath.Join(targetDir, name)
	if info, err := os.Stat(targetPath); err == nil && !info.IsDir() {
		return targetPath, nil
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
		return "", fmt.Errorf("plugin: binary download failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	tmpPath := targetPath + ".download"
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
