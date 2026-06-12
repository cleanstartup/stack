package stencil

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func DiscoverComponents(baseDir string) []string {
	return discoverModuleFiles(baseDir, ".stencil.ts", ".stencil.tsx")
}

func discoverModuleFiles(baseDir string, extensions ...string) []string {
	if strings.TrimSpace(baseDir) == "" || len(extensions) == 0 {
		return nil
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
	_ = filepath.WalkDir(baseDir, func(path string, entry os.DirEntry, err error) error {
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
		switch name {
		case "tailwind.input.css", "stencil.config.ts", "tsconfig.json", "package.json", "package-lock.json", ".package-lock.json":
			return nil
		}
		for _, ext := range extensions {
			if strings.HasSuffix(name, ext) {
				files = append(files, path)
				break
			}
		}
		return nil
	})
	sort.Strings(files)
	return files
}
