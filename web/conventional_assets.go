package web

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type conventionalAssets struct {
	baseDir string
}

func ConventionalAssets(baseDir ...string) Part {
	if len(baseDir) > 0 && strings.TrimSpace(baseDir[0]) != "" {
		return conventionalAssets{baseDir: baseDir[0]}
	}
	return conventionalAssets{baseDir: inferredConventionalAssetsDir()}
}

func inferredConventionalAssetsDir() string { return CallerDir(2) }

func (a conventionalAssets) Apply(app *WebApp) {
	if app == nil || strings.TrimSpace(a.baseDir) == "" {
		return
	}
	app.RegisterTailwindScan(a.baseDir)

	cssFiles, tailwindFiles, jsFiles := discoverConventionalAssets(a.baseDir)
	for _, path := range cssFiles {
		app.RegisterCSS(FromFile(path))
	}
	for _, path := range tailwindFiles {
		app.RegisterTailwindCSS(FromFile(path))
	}
	for _, path := range jsFiles {
		app.RegisterJS(FromFile(path))
	}
}

func discoverConventionalAssets(baseDir string) (cssFiles []string, tailwindFiles []string, jsFiles []string) {
	for _, relDir := range []string{
		filepath.Join("assets", "css"),
		filepath.Join("assets", "js"),
	} {
		root := filepath.Join(baseDir, relDir)
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
