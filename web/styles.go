package web

import (
	"path/filepath"
	"strings"
)

const (
	styleBundleID   = "app"
	styleBundleFile = "app.css"
)

type styleSourceEntry struct {
	source AssetSource
}

type StyleRegistry struct {
	entries []styleSourceEntry
	scans   []string
	bundle  AssetRef
}

func NewStyleRegistry() *StyleRegistry {
	return &StyleRegistry{
		entries: []styleSourceEntry{},
		scans:   []string{},
		bundle: AssetRef{
			Kind:  AssetKindCSS,
			ID:    styleBundleID,
			Files: []string{styleBundleFile},
		},
	}
}

func (r *StyleRegistry) AddCSS(src AssetSource) AssetRef {
	if r == nil || src == nil {
		return AssetRef{}
	}
	r.entries = append(r.entries, styleSourceEntry{source: src})
	return r.bundle
}

func (r *StyleRegistry) AddScan(paths ...string) {
	if r == nil {
		return
	}
	r.scans = cleanWatchPaths(append(r.scans, paths...))
}

func (r *StyleRegistry) BundleRef() AssetRef {
	if r == nil {
		return AssetRef{}
	}
	return r.bundle
}

func (r *StyleRegistry) CSSSources() []AssetSource {
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

func (r *StyleRegistry) ScanPaths() []string {
	if r == nil {
		return nil
	}
	out := append([]string{}, r.scans...)
	return cleanWatchPaths(out)
}

func (r *StyleRegistry) WatchPaths() []string {
	if r == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var paths []string
	for _, source := range r.CSSSources() {
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
