package web

import (
	"os"
	"path/filepath"
	"strings"
)

const layoutsModulePrefix = "github.com/cleanstartup/stack/layouts"

type LayoutRegistry struct {
	entries []contentEntry
}

func NewLayoutRegistry() *LayoutRegistry {
	return &LayoutRegistry{entries: []contentEntry{}}
}

func (r *LayoutRegistry) Add(baseDir string, includes ...string) {
	if r == nil {
		return
	}
	baseDir = moduleRoot(baseDir)
	if strings.TrimSpace(baseDir) == "" {
		return
	}
	cleaned := cleanContentIncludes(includes...)
	if len(cleaned) == 0 {
		return
	}
	key := contentEntryKey(baseDir, cleaned)
	for _, entry := range r.entries {
		if contentEntryKey(entry.baseDir, entry.includes) == key {
			return
		}
	}
	r.entries = append(r.entries, contentEntry{baseDir: baseDir, includes: cleaned})
}

func (r *LayoutRegistry) Entries() []contentEntry {
	if r == nil {
		return nil
	}
	out := make([]contentEntry, len(r.entries))
	copy(out, r.entries)
	return out
}

func (r *LayoutRegistry) WatchPaths() []string {
	if r == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var paths []string
	for _, entry := range r.entries {
		if strings.TrimSpace(entry.baseDir) == "" {
			continue
		}
		if _, exists := seen[entry.baseDir]; exists {
			continue
		}
		seen[entry.baseDir] = struct{}{}
		paths = append(paths, entry.baseDir)
	}
	return cleanWatchPaths(paths)
}

func (r *LayoutRegistry) SourceChanged(path string) bool {
	if r == nil {
		return false
	}
	for _, entry := range r.Entries() {
		if sourcePathMatches(entry.baseDir, path) || entry.baseDir == "." {
			return true
		}
	}
	return false
}

func (r *LayoutRegistry) Modules(moduleRoot string) ([]HugoModule, error) {
	if r == nil {
		return nil, nil
	}
	moduleRoot = strings.TrimSpace(moduleRoot)
	if moduleRoot == "" {
		return nil, os.ErrInvalid
	}
	var modules []HugoModule
	for _, entry := range r.entries {
		if strings.TrimSpace(entry.baseDir) == "" {
			continue
		}
		files := discoverContentFiles(entry.baseDir, entry.includes)
		if len(files) == 0 {
			continue
		}
		mod, err := r.materialize(moduleRoot, entry.baseDir, entry.includes, files)
		if err != nil {
			return nil, err
		}
		modules = append(modules, mod)
	}
	return modules, nil
}

func (r *LayoutRegistry) materialize(moduleRoot, baseDir string, includes, files []string) (HugoModule, error) {
	id := assetID(baseDir + "\x00" + strings.Join(includes, "\x00"))
	moduleDir := filepath.Join(moduleRoot, "layouts", id)
	layoutsDir := filepath.Join(moduleDir, "layouts")

	if err := os.RemoveAll(moduleDir); err != nil {
		return HugoModule{}, err
	}
	if err := os.MkdirAll(layoutsDir, 0o755); err != nil {
		return HugoModule{}, err
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "hugo.toml"), []byte(layoutModuleConfig()), 0o644); err != nil {
		return HugoModule{}, err
	}
	for _, file := range files {
		rel, err := filepath.Rel(baseDir, file)
		if err != nil {
			rel = filepath.Base(file)
		}
		target := filepath.Join(layoutsDir, "partials", filepath.FromSlash(rel))
		if err := copyTextFileIfChanged(file, target); err != nil {
			return HugoModule{}, err
		}
	}
	return HugoModule{
		ImportPath:  layoutsModulePrefix + "/" + id,
		ReplacePath: moduleDir,
	}, nil
}

func layoutModuleConfig() string {
	return "title = \"layouts\"\n"
}

func Layouts(baseDir string, includes ...string) Part {
	patterns := cleanContentIncludes(includes...)
	return layoutAssets{baseDir: moduleRoot(baseDir), includes: patterns}
}

type layoutAssets struct {
	baseDir  string
	includes []string
}

func (a layoutAssets) Apply(app *WebApp) {
	if app == nil || strings.TrimSpace(a.baseDir) == "" {
		return
	}
	app.RegisterLayouts(a.baseDir, a.includes...)
}
