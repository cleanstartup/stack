package web

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const contentModulePrefix = "github.com/cleanstartup/stack/content"

type contentEntry struct {
	baseDir  string
	includes []string
}

type ContentRegistry struct {
	entries []contentEntry
}

func NewContentRegistry() *ContentRegistry {
	return &ContentRegistry{entries: []contentEntry{}}
}

func (r *ContentRegistry) Add(baseDir string, includes ...string) {
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

func (r *ContentRegistry) Entries() []contentEntry {
	if r == nil {
		return nil
	}
	out := make([]contentEntry, len(r.entries))
	copy(out, r.entries)
	return out
}

func (r *ContentRegistry) WatchPaths() []string {
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

func (r *ContentRegistry) SourceChanged(path string) bool {
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

func (r *ContentRegistry) Modules(moduleRoot string) ([]HugoModule, error) {
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

func (r *ContentRegistry) materialize(moduleRoot, baseDir string, includes, files []string) (HugoModule, error) {
	id := assetID(baseDir + "\x00" + strings.Join(includes, "\x00"))
	moduleDir := filepath.Join(moduleRoot, "content", id)
	contentDir := filepath.Join(moduleDir, "content")

	if err := os.RemoveAll(moduleDir); err != nil {
		return HugoModule{}, err
	}
	if err := os.MkdirAll(contentDir, 0o755); err != nil {
		return HugoModule{}, err
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "hugo.toml"), []byte(contentModuleConfig()), 0o644); err != nil {
		return HugoModule{}, err
	}

	for _, file := range files {
		rel, err := filepath.Rel(baseDir, file)
		if err != nil {
			rel = filepath.Base(file)
		}
		target := filepath.Join(contentDir, filepath.FromSlash(rel))
		if err := copyTextFileIfChanged(file, target); err != nil {
			return HugoModule{}, err
		}
	}

	return HugoModule{
		ImportPath:  contentModulePrefix + "/" + id,
		ReplacePath: moduleDir,
	}, nil
}

func contentModuleConfig() string {
	return "title = \"content\"\n\n[markup.goldmark.renderer]\nunsafe = true\n"
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

func contentEntryKey(baseDir string, includes []string) string {
	return strings.TrimSpace(baseDir) + "\x00" + strings.Join(includes, "\x00")
}

func cleanContentIncludes(patterns ...string) []string {
	if len(patterns) == 0 {
		return nil
	}
	out := make([]string, 0, len(patterns))
	seen := map[string]struct{}{}
	for _, pattern := range patterns {
		cleaned := strings.TrimSpace(pattern)
		if cleaned == "" {
			continue
		}
		if _, exists := seen[cleaned]; exists {
			continue
		}
		seen[cleaned] = struct{}{}
		out = append(out, cleaned)
	}
	return out
}

func discoverContentFiles(baseDir string, includes []string) []string {
	if strings.TrimSpace(baseDir) == "" || len(includes) == 0 {
		return nil
	}
	skipDirs := map[string]struct{}{
		".git":         {},
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
	_ = filepath.WalkDir(baseDir, func(current string, entry os.DirEntry, err error) error {
		if err != nil || entry == nil {
			return err
		}
		if entry.IsDir() {
			if current != baseDir {
				if _, skip := skipDirs[filepath.Base(current)]; skip {
					return filepath.SkipDir
				}
			}
			return nil
		}
		rel, err := filepath.Rel(baseDir, current)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !matchesAnyContentPattern(includes, rel) {
			return nil
		}
		files = append(files, current)
		return nil
	})
	sort.Strings(files)
	return files
}

func matchesAnyContentPattern(patterns []string, name string) bool {
	for _, pattern := range patterns {
		if matchContentPattern(pattern, name) {
			return true
		}
	}
	return false
}

func matchContentPattern(pattern, name string) bool {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	name = filepath.ToSlash(strings.TrimSpace(name))
	if pattern == "" || name == "" {
		return false
	}
	return matchContentSegments(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

func matchContentSegments(patterns, names []string) bool {
	if len(patterns) == 0 {
		return len(names) == 0
	}
	if patterns[0] == "**" {
		if len(patterns) == 1 {
			return true
		}
		for i := 0; i <= len(names); i++ {
			if matchContentSegments(patterns[1:], names[i:]) {
				return true
			}
		}
		return false
	}
	if len(names) == 0 {
		return false
	}
	ok, err := path.Match(patterns[0], names[0])
	if err != nil || !ok {
		return false
	}
	return matchContentSegments(patterns[1:], names[1:])
}

func Content(baseDir string, includes ...string) Part {
	patterns := cleanContentIncludes(includes...)
	return contentAssets{baseDir: moduleRoot(baseDir), includes: patterns}
}

type contentAssets struct {
	baseDir  string
	includes []string
}

func (a contentAssets) Apply(app *WebApp) {
	if app == nil || strings.TrimSpace(a.baseDir) == "" {
		return
	}
	app.RegisterContent(a.baseDir, a.includes...)
}
