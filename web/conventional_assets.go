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

type styleAssets struct {
	baseDir string
}

type componentAssets struct {
	baseDir string
}

func Styles(baseDir ...string) Part {
	if len(baseDir) > 0 && strings.TrimSpace(baseDir[0]) != "" {
		return styleAssets{baseDir: moduleRoot(baseDir[0])}
	}
	return styleAssets{baseDir: inferredStyleAssetsDir()}
}

func inferredStyleAssetsDir() string { return CallerDir(2) }

func Components(baseDir ...string) Part {
	if len(baseDir) > 0 && strings.TrimSpace(baseDir[0]) != "" {
		return componentAssets{baseDir: moduleRoot(baseDir[0])}
	}
	return componentAssets{baseDir: inferredComponentAssetsDir()}
}

func inferredComponentAssetsDir() string { return CallerDir(2) }

func ConventionalAssets(baseDir ...string) Part {
	if len(baseDir) > 0 && strings.TrimSpace(baseDir[0]) != "" {
		return conventionalAssets{baseDir: moduleRoot(baseDir[0])}
	}
	return conventionalAssets{baseDir: inferredConventionalAssetsDir()}
}

func inferredConventionalAssetsDir() string { return CallerDir(2) }

func (a conventionalAssets) Apply(app *WebApp) {
	if app == nil || strings.TrimSpace(a.baseDir) == "" {
		return
	}
	app.RegisterTailwindScan(a.baseDir)

	cssFiles := discoverModuleFiles(a.baseDir, ".css")
	if len(cssFiles) > 0 {
		app.RegisterTailwindCSS(FromFiles(a.baseDir, cssFiles...))
	}
}

func (a styleAssets) Apply(app *WebApp) {
	if app == nil || strings.TrimSpace(a.baseDir) == "" {
		return
	}
	app.RegisterTailwindScan(a.baseDir)

	cssFiles := discoverModuleFiles(a.baseDir, ".css")
	if len(cssFiles) > 0 {
		app.RegisterTailwindCSS(FromFiles(a.baseDir, cssFiles...))
	}
}

func (a componentAssets) Apply(app *WebApp) {
	if app == nil || strings.TrimSpace(a.baseDir) == "" {
		return
	}
	tsFiles := discoverModuleFiles(a.baseDir, ".ts", ".tsx")
	if len(tsFiles) == 0 {
		return
	}
	app.RegisterStencilScan(a.baseDir)
	app.RegisterStencil(FromFiles(a.baseDir, tsFiles...))
}

func discoverModuleFiles(baseDir string, extensions ...string) []string {
	if strings.TrimSpace(baseDir) == "" || len(extensions) == 0 {
		return nil
	}
	skipDirs := map[string]struct{}{
		".git":         {},
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

func moduleRoot(baseDir string) string {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		return ""
	}
	current := baseDir
	for {
		if _, err := os.Stat(filepath.Join(current, "hugo.toml")); err == nil {
			return current
		}
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return baseDir
		}
		current = parent
	}
}
