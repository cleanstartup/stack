package hugo

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

// ensureExtractedBinary returns the local, executable "hugo" binary path
// under extractDir, extracting it out of the release archive at archivePath
// if it isn't already there. Idempotent, same posture as
// plugin.BinProvider.Ensure (stat-first, skip work on a cache hit) — this is
// the piece BinProvider itself deliberately doesn't do: it downloads-and-
// chmods a raw file at a URL, it does not know archives exist (see
// plugin/binprovider.go). assetFile is the release asset name computed by
// releaseAssetName (BinProvider's cached archivePath itself has no
// extension, see plugin.binProvider.Ensure) and picks the extraction format:
// hugo's linux releases ship .tar.gz, its darwin releases ship a .pkg
// installer (see CUP-33 / releaseAssetName's darwin case).
func ensureExtractedBinary(archivePath, extractDir, assetFile string) (string, error) {
	binaryPath := filepath.Join(extractDir, "hugo")
	if info, err := os.Stat(binaryPath); err == nil && !info.IsDir() {
		return binaryPath, nil
	}
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return "", err
	}
	if strings.HasSuffix(assetFile, ".pkg") {
		if err := extractPkgEntry(archivePath, "hugo", binaryPath); err != nil {
			return "", err
		}
	} else {
		if err := extractTarGzEntry(archivePath, "hugo", binaryPath); err != nil {
			return "", err
		}
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

		return writeExecutableAtomic(destPath, tr)
	}
	return fmt.Errorf("hugo: entry %q not found in archive %q (first entries seen: %v)", entryName, archivePath, seen)
}

// extractPkgEntry extracts the single regular-file entry named entryName
// (matched by base name) from the macOS installer package (.pkg, xar format)
// at archivePath into destPath. hugo's darwin releases dropped .tar.gz and
// now only ship .pkg (see releaseAssetName / CUP-33), and a .pkg's Payload is
// itself a compressed cpio (often pbzx-chunked on modern macOS) — rather than
// reimplement that format, this shells out to pkgutil --expand-full, Apple's
// own tool for exploding a .pkg's xar/cpio/pbzx layers into plain files on
// disk, then walks the result the same way extractTarGzEntry walks a tarball.
func extractPkgEntry(archivePath, entryName, destPath string) error {
	expandRoot, err := os.MkdirTemp("", "hugo-pkg-expand-*")
	if err != nil {
		return fmt.Errorf("hugo: create pkg expand dir: %w", err)
	}
	defer os.RemoveAll(expandRoot)

	// pkgutil refuses to expand into a directory that already exists, so
	// target a not-yet-created subdirectory of our own temp dir.
	expandDir := filepath.Join(expandRoot, "expanded")
	if out, err := exec.Command("pkgutil", "--expand-full", archivePath, expandDir).CombinedOutput(); err != nil {
		return fmt.Errorf("hugo: pkgutil --expand-full %q: %w: %s", archivePath, err, strings.TrimSpace(string(out)))
	}

	var foundPath string
	var seen []string
	walkErr := filepath.WalkDir(expandDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || foundPath != "" {
			return nil
		}
		if d.Name() == entryName {
			foundPath = p
			return nil
		}
		if len(seen) < 10 {
			rel, relErr := filepath.Rel(expandDir, p)
			if relErr != nil {
				rel = p
			}
			seen = append(seen, rel)
		}
		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("hugo: walk expanded pkg %q: %w", archivePath, walkErr)
	}
	if foundPath == "" {
		return fmt.Errorf("hugo: entry %q not found in pkg %q (first entries seen: %v)", entryName, archivePath, seen)
	}

	src, err := os.Open(foundPath)
	if err != nil {
		return fmt.Errorf("hugo: open expanded pkg entry %q: %w", foundPath, err)
	}
	defer src.Close()
	return writeExecutableAtomic(destPath, src)
}

// writeExecutableAtomic copies r into destPath via a uniquely-named temp
// file in destPath's directory, chmods it executable, then renames
// atomically into place. Shared by both archive formats hugo release assets
// come in (extractTarGzEntry, extractPkgEntry) — a fixed temp-file suffix
// would let two concurrent extractions (e.g. two Hugo(...) producers racing
// to warm the same version's cache) clobber each other's temp file mid-write,
// mirroring plugin.BinProvider.Ensure's own download-then-rename pattern.
func writeExecutableAtomic(destPath string, r io.Reader) error {
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	out, err := os.CreateTemp(destDir, filepath.Base(destPath)+".*.extract")
	if err != nil {
		return err
	}
	tmpPath := out.Name()
	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("hugo: write extracted binary %q: %w", destPath, err)
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
