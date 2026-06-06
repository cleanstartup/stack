package web

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ConventionalAssets struct {
	BaseDir   string
	ScanPaths []string
}

func NewConventionalAssets(baseDir string) ConventionalAssets {
	return ConventionalAssets{
		BaseDir:   baseDir,
		ScanPaths: []string{baseDir},
	}
}

func (a ConventionalAssets) Register(b *Builder) {
	if b == nil {
		return
	}
	for _, path := range a.ScanPaths {
		b.TailwindScan(path)
	}

	cssFiles, tailwindFiles, jsFiles := a.discover()
	for _, path := range cssFiles {
		b.CSS(FromFile(path))
	}
	for _, path := range tailwindFiles {
		b.TailwindCSS(FromFile(path))
	}
	for _, path := range jsFiles {
		b.JS(FromFile(path))
	}
}

func (a ConventionalAssets) discover() (cssFiles []string, tailwindFiles []string, jsFiles []string) {
	if strings.TrimSpace(a.BaseDir) == "" {
		return nil, nil, nil
	}
	for _, relDir := range []string{
		filepath.Join("assets", "css"),
		filepath.Join("assets", "js"),
	} {
		root := filepath.Join(a.BaseDir, relDir)
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			continue
		}
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry == nil || entry.IsDir() {
				return err
			}
			name := filepath.Base(path)
			switch {
			case strings.HasSuffix(name, ".tailwind.css"):
				tailwindFiles = append(tailwindFiles, path)
			case strings.HasSuffix(name, ".css"):
				cssFiles = append(cssFiles, path)
			case strings.HasSuffix(name, ".js"):
				jsFiles = append(jsFiles, path)
			}
			return nil
		})
	}
	sort.Strings(cssFiles)
	sort.Strings(tailwindFiles)
	sort.Strings(jsFiles)
	return cssFiles, tailwindFiles, jsFiles
}
