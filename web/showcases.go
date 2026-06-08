package web

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const showcaseModulePrefix = "github.com/cleanstartup/stack/showcases"

type showcaseEntry struct {
	baseDir string
}

type ShowcaseRegistry struct {
	entries []showcaseEntry
}

func NewShowcaseRegistry() *ShowcaseRegistry {
	return &ShowcaseRegistry{entries: []showcaseEntry{}}
}

func (r *ShowcaseRegistry) Add(baseDir string) {
	if r == nil {
		return
	}
	baseDir = moduleRoot(baseDir)
	if strings.TrimSpace(baseDir) == "" {
		return
	}
	for _, entry := range r.entries {
		if entry.baseDir == baseDir {
			return
		}
	}
	r.entries = append(r.entries, showcaseEntry{baseDir: baseDir})
}

func (r *ShowcaseRegistry) Entries() []showcaseEntry {
	if r == nil {
		return nil
	}
	out := make([]showcaseEntry, len(r.entries))
	copy(out, r.entries)
	return out
}

func (r *ShowcaseRegistry) WatchPaths() []string {
	if r == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var paths []string
	for _, entry := range r.entries {
		if strings.TrimSpace(entry.baseDir) == "" {
			continue
		}
		if _, exists := seen[entry.baseDir]; !exists {
			seen[entry.baseDir] = struct{}{}
			paths = append(paths, entry.baseDir)
		}
		files := discoverModuleFiles(entry.baseDir, ".showcase.md")
		for _, file := range files {
			if _, exists := seen[file]; exists {
				continue
			}
			seen[file] = struct{}{}
			paths = append(paths, file)
		}
	}
	return cleanWatchPaths(paths)
}

func (r *ShowcaseRegistry) Modules(moduleRoot string) ([]HugoModule, error) {
	if r == nil {
		return nil, nil
	}
	moduleRoot = strings.TrimSpace(moduleRoot)
	if moduleRoot == "" {
		return nil, fmt.Errorf("module root is required")
	}

	var modules []HugoModule
	for _, entry := range r.entries {
		if strings.TrimSpace(entry.baseDir) == "" {
			continue
		}
		files := discoverModuleFiles(entry.baseDir, ".showcase.md")
		if len(files) == 0 {
			continue
		}
		mod, err := r.materialize(moduleRoot, entry.baseDir, files)
		if err != nil {
			return nil, err
		}
		modules = append(modules, mod)
	}
	return modules, nil
}

func (r *ShowcaseRegistry) materialize(moduleRoot, baseDir string, files []string) (HugoModule, error) {
	id := assetID(baseDir)
	moduleDir := filepath.Join(moduleRoot, "showcases", id)
	contentDir := filepath.Join(moduleDir, "content")

	if err := os.RemoveAll(moduleDir); err != nil {
		return HugoModule{}, err
	}
	if err := os.MkdirAll(contentDir, 0o755); err != nil {
		return HugoModule{}, err
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "hugo.toml"), []byte(showcaseModuleConfig()), 0o644); err != nil {
		return HugoModule{}, err
	}

	for _, file := range files {
		rel, err := filepath.Rel(baseDir, file)
		if err != nil {
			rel = filepath.Base(file)
		}
		rel = strings.TrimSuffix(filepath.ToSlash(rel), ".showcase.md")
		if rel == "" {
			continue
		}
		targetDirRel := filepath.Dir(rel)
		if targetDirRel == "." {
			targetDirRel = filepath.Base(rel)
		}
		targetDir := filepath.Join(contentDir, filepath.FromSlash(targetDirRel))
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return HugoModule{}, err
		}
		if err := copyTextFileIfChanged(file, filepath.Join(targetDir, "index.md")); err != nil {
			return HugoModule{}, err
		}
	}

	return HugoModule{
		ImportPath:  showcaseModulePrefix + "/" + id,
		ReplacePath: moduleDir,
	}, nil
}

func showcaseModuleConfig() string {
	return "title = \"showcases\"\n\n[markup.goldmark.renderer]\nunsafe = true\n"
}

func copyTextFileIfChanged(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if existing, err := os.ReadFile(dst); err == nil && string(existing) == string(input) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, input, 0o644)
}

func Showcases(baseDir ...string) Part {
	if len(baseDir) > 0 && strings.TrimSpace(baseDir[0]) != "" {
		return showcaseAssets{baseDir: moduleRoot(baseDir[0])}
	}
	return showcaseAssets{baseDir: inferredShowcaseAssetsDir()}
}

type showcaseAssets struct {
	baseDir string
}

func inferredShowcaseAssetsDir() string { return CallerDir(2) }

func (a showcaseAssets) Apply(app *WebApp) {
	if app == nil || strings.TrimSpace(a.baseDir) == "" {
		return
	}
	app.RegisterShowcase(a.baseDir)
}
