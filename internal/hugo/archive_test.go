package hugo

import (
	"archive/tar"
	"compress/gzip"
	"os"
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

	path1, err := ensureExtractedBinary(archivePath, extractDir)
	if err != nil {
		t.Fatalf("first ensureExtractedBinary: %v", err)
	}

	// Overwrite the archive with different content; a cache hit must not
	// re-extract (mirrors plugin.BinProvider.Ensure's idempotency contract).
	archivePath2 := writeTestTarGz(t, dir, "hugo2.tar.gz", map[string]string{
		"hugo": "second",
	})
	path2, err := ensureExtractedBinary(archivePath2, extractDir)
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
