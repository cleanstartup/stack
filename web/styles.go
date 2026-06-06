package web

import (
	"path/filepath"
	"strings"
)

const (
	tailwindBundleID   = "app"
	tailwindBundleFile = "app.css"
)

type tailwindInputEntry struct {
	source AssetSource
}

type TailwindRegistry struct {
	entries []tailwindInputEntry
	scans   []string
	bundle  AssetRef
}

func NewTailwindRegistry() *TailwindRegistry {
	return &TailwindRegistry{
		entries: []tailwindInputEntry{},
		scans:   []string{},
		bundle: AssetRef{
			Kind:  AssetKindCSS,
			ID:    tailwindBundleID,
			Files: []string{tailwindBundleFile},
		},
	}
}

func (r *TailwindRegistry) AddInput(src AssetSource) AssetRef {
	if r == nil || src == nil {
		return AssetRef{}
	}
	r.entries = append(r.entries, tailwindInputEntry{source: src})
	return r.bundle
}

func (r *TailwindRegistry) AddScan(paths ...string) {
	if r == nil {
		return
	}
	r.scans = cleanWatchPaths(append(r.scans, paths...))
}

func (r *TailwindRegistry) BundleRef() AssetRef {
	if r == nil {
		return AssetRef{}
	}
	return r.bundle
}

func (r *TailwindRegistry) Inputs() []AssetSource {
	if r == nil {
		return nil
	}
	out := make([]AssetSource, 0, len(r.entries))
	for _, entry := range r.entries {
		if entry.source == nil {
			continue
		}
		out = append(out, entry.source)
	}
	return out
}

func (r *TailwindRegistry) ScanPaths() []string {
	if r == nil {
		return nil
	}
	out := append([]string{}, r.scans...)
	return cleanWatchPaths(out)
}

func (r *TailwindRegistry) WatchPaths() []string {
	if r == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var paths []string
	for _, source := range r.Inputs() {
		if watcher, ok := source.(WatchPathsProvider); ok {
			for _, path := range watcher.WatchPaths() {
				path = strings.TrimSpace(path)
				if path == "" {
					continue
				}
				if _, exists := seen[path]; exists {
					continue
				}
				seen[path] = struct{}{}
				paths = append(paths, path)
			}
		}
	}
	for _, path := range r.ScanPaths() {
		path = filepath.Clean(strings.TrimSpace(path))
		if path == "" || path == "." {
			continue
		}
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	return paths
}
