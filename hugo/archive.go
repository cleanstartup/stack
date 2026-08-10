package hugo

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
)

// ensureExtractedBinary returns the local, executable "hugo" binary path
// under extractDir, extracting it out of the .tar.gz at archivePath if it
// isn't already there. Idempotent, same posture as plugin.BinProvider.Ensure
// (stat-first, skip work on a cache hit) — this is the piece BinProvider
// itself deliberately doesn't do: it downloads-and-chmods a raw file at a
// URL, it does not know archives exist (see plugin/binprovider.go).
func ensureExtractedBinary(archivePath, extractDir string) (string, error) {
	binaryPath := filepath.Join(extractDir, "hugo")
	if info, err := os.Stat(binaryPath); err == nil && !info.IsDir() {
		return binaryPath, nil
	}
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return "", err
	}
	if err := extractTarGzEntry(archivePath, "hugo", binaryPath); err != nil {
		return "", err
	}
	return binaryPath, nil
}

// extractTarGzEntry extracts the single regular-file entry named entryName
// (matched by base name, since release tarballs may nest it under a
// version-named directory) from the .tar.gz at archivePath into destPath.
// Writes to a uniquely-named temp file in destPath's directory, then renames
// atomically into place, mirroring plugin.BinProvider.Ensure's
// download-then-rename pattern — a fixed ".extract" suffix would let two
// concurrent extractions (e.g. two Hugo(...) producers racing to warm the
// same version's cache) clobber each other's temp file mid-write.
func extractTarGzEntry(archivePath, entryName, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("hugo: open archive %q: %w", archivePath, err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("hugo: gzip reader for %q: %w", archivePath, err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var seen []string
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("hugo: read tar entry in %q: %w", archivePath, err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		base := path.Base(filepath.ToSlash(header.Name))
		if base != entryName {
			if len(seen) < 10 {
				seen = append(seen, header.Name)
			}
			continue
		}

		destDir := filepath.Dir(destPath)
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return err
		}
		out, err := os.CreateTemp(destDir, filepath.Base(destPath)+".*.extract")
		if err != nil {
			return err
		}
		tmpPath := out.Name()
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("hugo: write extracted %q: %w", entryName, err)
		}
		if err := out.Close(); err != nil {
			_ = os.Remove(tmpPath)
			return err
		}
		if err := os.Chmod(tmpPath, 0o755); err != nil {
			_ = os.Remove(tmpPath)
			return err
		}
		if err := os.Rename(tmpPath, destPath); err != nil {
			_ = os.Remove(tmpPath)
			return err
		}
		return nil
	}
	return fmt.Errorf("hugo: entry %q not found in archive %q (first entries seen: %v)", entryName, archivePath, seen)
}
