package lit

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DiscoverEntries finds every lit bundle entry point under baseDir: files
// named "*.lit.ts" or "*.lit.tsx". This is the naming convention that plays
// the same role as tailwind's ".tailwind.css" (internal/tailwind/discovery.go)
// and stencil's ".stencil.ts"/".stencil.tsx" (internal/stencil/discovery.go)
// suffixes — a plugin-internal filter over a generically-declared
// assets.Dir(...), not a new module-facing API.
//
// The distinction this convention encodes: a directory declared via
// assets.Dir(...) may contain both entry files (bundled as their own output,
// discovered here) and plain, non-entry .ts/.tsx modules (a component
// library — e.g. CUP-27's ui.Module() sources) that are only pulled into an
// entry's bundle if that entry actually imports them, and dropped otherwise.
// That's the mechanical shape of D-C's "tree-shaken per consumer": each
// discovered entry is bundled independently (see Producer.Build), so a
// component only reachable from one entry never leaks into another.
//
// Higher stakes than tailwind's analogous convention: misnaming a tailwind
// content file (missing ".tailwind.css") just drops some class scanning —
// a purging miss that shows up visually. Misnaming a lit entry (or naming a
// library file "*.lit.ts" by mistake) changes what gets bundled and served
// as JS at all — a silent build-shape change, not just a styling gap. No
// mitigation beyond this doc comment today; CUP-27's ui.Module() is the
// first real consumer where getting this convention wrong would bite.
// DiscoverEntries returns an error if baseDir can't be walked (e.g. it
// doesn't exist — a Module declaring a typo'd or deleted assets.Dir(...)
// path) rather than silently reporting zero entries: a broken declared
// source dir should fail the build loudly, not be indistinguishable from
// "this dir legitimately has no lit source in it".
func DiscoverEntries(baseDir string) ([]string, error) {
	return discoverModuleFiles(baseDir, ".lit.ts", ".lit.tsx")
}

func discoverModuleFiles(baseDir string, extensions ...string) ([]string, error) {
	if strings.TrimSpace(baseDir) == "" || len(extensions) == 0 {
		return nil, nil
	}
	skipDirs := map[string]struct{}{
		".git":         {},
		".assets":      {},
		"deps":         {},
		".stack":       {},
		"node_modules": {},
		"dist":         {},
		"build":        {},
		"coverage":     {},
		"vendor":       {},
		"public":       {},
		"static":       {},
		"resources":    {},
	}
	var files []string
	err := filepath.WalkDir(baseDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry == nil {
			return err
		}
		if entry.IsDir() {
			if path != baseDir {
				if _, skip := skipDirs[filepath.Base(path)]; skip {
					return filepath.SkipDir
				}
			}
			return nil
		}
		name := strings.ToLower(filepath.Base(path))
		for _, ext := range extensions {
			if strings.HasSuffix(name, ext) {
				files = append(files, path)
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("lit: scanning %s: %w", baseDir, err)
	}
	sort.Strings(files)
	return files, nil
}
