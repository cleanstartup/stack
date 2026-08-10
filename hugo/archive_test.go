package hugo

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func writeTestTarGz(t *testing.T, dir, filename string, entries map[string]string) string {
	t.Helper()
	archivePath := filepath.Join(dir, filename)
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, body := range entries {
		hdr := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return archivePath
}

func TestExtractTarGzEntryFindsRootEntry(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable bit assertion is unix-only")
	}
	dir := t.TempDir()
	archivePath := writeTestTarGz(t, dir, "hugo.tar.gz", map[string]string{
		"LICENSE": "license text",
		"hugo":    "#!/bin/sh\necho fake-hugo\n",
		"README":  "readme text",
	})

	destPath := filepath.Join(dir, "extracted", "hugo")
	if err := extractTarGzEntry(archivePath, "hugo", destPath); err != nil {
		t.Fatalf("extractTarGzEntry: %v", err)
	}

	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read extracted binary: %v", err)
	}
	if string(data) != "#!/bin/sh\necho fake-hugo\n" {
		t.Fatalf("unexpected extracted contents: %q", data)
	}
	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("expected extracted binary to be executable, got mode %v", info.Mode())
	}
}

func TestExtractTarGzEntryFindsNestedEntry(t *testing.T) {
	dir := t.TempDir()
	archivePath := writeTestTarGz(t, dir, "hugo.tar.gz", map[string]string{
		"hugo_0.134.3_linux-amd64/hugo": "nested binary",
	})

	destPath := filepath.Join(dir, "extracted", "hugo")
	if err := extractTarGzEntry(archivePath, "hugo", destPath); err != nil {
		t.Fatalf("extractTarGzEntry: %v", err)
	}
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "nested binary" {
		t.Fatalf("got %q", data)
	}
}

func TestExtractTarGzEntryMissingEntryErrors(t *testing.T) {
	dir := t.TempDir()
	archivePath := writeTestTarGz(t, dir, "hugo.tar.gz", map[string]string{
		"LICENSE": "license text",
	})

	destPath := filepath.Join(dir, "extracted", "hugo")
	err := extractTarGzEntry(archivePath, "hugo", destPath)
	if err == nil {
		t.Fatal("want an error when the archive has no entry matching entryName")
	}
	if _, statErr := os.Stat(destPath); statErr == nil {
		t.Fatal("want no partial file left behind on extraction failure")
	}
}

func TestEnsureExtractedBinaryIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	archivePath := writeTestTarGz(t, dir, "hugo.tar.gz", map[string]string{
		"hugo": "first",
	})
	extractDir := filepath.Join(dir, "extracted")

	path1, err := ensureExtractedBinary(archivePath, extractDir, "hugo_0.134.3_linux-amd64.tar.gz")
	if err != nil {
		t.Fatalf("first ensureExtractedBinary: %v", err)
	}

	// Overwrite the archive with different content; a cache hit must not
	// re-extract (mirrors plugin.BinProvider.Ensure's idempotency contract).
	archivePath2 := writeTestTarGz(t, dir, "hugo2.tar.gz", map[string]string{
		"hugo": "second",
	})
	path2, err := ensureExtractedBinary(archivePath2, extractDir, "hugo_0.134.3_linux-amd64.tar.gz")
	if err != nil {
		t.Fatalf("second ensureExtractedBinary: %v", err)
	}
	if path1 != path2 {
		t.Fatalf("expected same extracted path, got %q vs %q", path1, path2)
	}
	data, err := os.ReadFile(path2)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "first" {
		t.Fatalf("expected cache hit to skip re-extraction, got %q", data)
	}
}

func requirePkgTools(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("pkgutil/pkgbuild are macOS-only tools")
	}
	if _, err := exec.LookPath("pkgutil"); err != nil {
		t.Skip("pkgutil not found in PATH")
	}
	if _, err := exec.LookPath("pkgbuild"); err != nil {
		t.Skip("pkgbuild not found in PATH")
	}
}

// writeTestPkg builds a real, genuine .pkg (via pkgbuild, the same Apple
// tool a real hugo release would have been built with) so extractPkgEntry is
// exercised against actual xar/cpio/pbzx layers, not a hand-rolled fake —
// mirrors writeTestTarGz's role for the tar.gz path.
func writeTestPkg(t *testing.T, dir, filename string, entries map[string]string) string {
	t.Helper()
	root := filepath.Join(dir, "pkgroot")
	for name, body := range entries {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	pkgPath := filepath.Join(dir, filename)
	cmd := exec.Command("pkgbuild",
		"--root", root,
		"--identifier", "com.cleanstartup.stack.hugo-test",
		"--version", "1.0",
		"--install-location", "/tmp/hugo-test-install",
		pkgPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("pkgbuild: %v: %s", err, out)
	}
	return pkgPath
}

func TestExtractPkgEntryFindsNestedEntry(t *testing.T) {
	requirePkgTools(t)
	dir := t.TempDir()
	pkgPath := writeTestPkg(t, dir, "hugo.pkg", map[string]string{
		"hugo_0.164.0_darwin-universal/hugo": "#!/bin/sh\necho fake-hugo\n",
	})

	destPath := filepath.Join(dir, "extracted", "hugo")
	if err := extractPkgEntry(pkgPath, "hugo", destPath); err != nil {
		t.Fatalf("extractPkgEntry: %v", err)
	}

	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read extracted binary: %v", err)
	}
	if string(data) != "#!/bin/sh\necho fake-hugo\n" {
		t.Fatalf("unexpected extracted contents: %q", data)
	}
	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("expected extracted binary to be executable, got mode %v", info.Mode())
	}
}

func TestExtractPkgEntryMissingEntryErrors(t *testing.T) {
	requirePkgTools(t)
	dir := t.TempDir()
	pkgPath := writeTestPkg(t, dir, "hugo.pkg", map[string]string{
		"LICENSE": "license text",
	})

	destPath := filepath.Join(dir, "extracted", "hugo")
	err := extractPkgEntry(pkgPath, "hugo", destPath)
	if err == nil {
		t.Fatal("want an error when the pkg has no entry matching entryName")
	}
	if _, statErr := os.Stat(destPath); statErr == nil {
		t.Fatal("want no partial file left behind on extraction failure")
	}
}

func TestEnsureExtractedBinaryDispatchesOnAssetSuffix(t *testing.T) {
	requirePkgTools(t)
	dir := t.TempDir()
	pkgPath := writeTestPkg(t, dir, "hugo.pkg", map[string]string{
		"hugo": "pkg contents",
	})
	extractDir := filepath.Join(dir, "extracted")

	path, err := ensureExtractedBinary(pkgPath, extractDir, "hugo_0.164.0_darwin-universal.pkg")
	if err != nil {
		t.Fatalf("ensureExtractedBinary: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "pkg contents" {
		t.Fatalf("expected .pkg asset suffix to route through extractPkgEntry, got %q", data)
	}
}
