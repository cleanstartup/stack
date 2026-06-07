package web

import (
	"path/filepath"
	"strings"
)

const (
	stencilBundleID   = "stack"
	stencilBundleFile = "stack.esm.js"
)

type stencilInputEntry struct {
	source AssetSource
}

type StencilRegistry struct {
	entries []stencilInputEntry
	scans   []string
	bundle  AssetRef
}

func NewStencilRegistry() *StencilRegistry {
	return &StencilRegistry{
		entries: []stencilInputEntry{},
		scans:   []string{},
		bundle: AssetRef{
			Kind:  AssetKindJS,
			ID:    stencilBundleID,
			Files: []string{stencilBundleFile},
		},
	}
}

func (r *StencilRegistry) AddInput(src AssetSource) AssetRef {
	if r == nil || src == nil {
		return AssetRef{}
	}
	for _, entry := range r.entries {
		if entry.source != nil && entry.source.ID() == src.ID() {
			return r.bundle
		}
	}
	r.entries = append(r.entries, stencilInputEntry{source: src})
	return r.bundle
}

func (r *StencilRegistry) AddScan(paths ...string) {
	if r == nil {
		return
	}
	r.scans = cleanWatchPaths(append(r.scans, paths...))
}

func (r *StencilRegistry) BundleRef() AssetRef {
	if r == nil {
		return AssetRef{}
	}
	return r.bundle
}

func (r *StencilRegistry) Inputs() []AssetSource {
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

func (r *StencilRegistry) ScanPaths() []string {
	if r == nil {
		return nil
	}
	out := append([]string{}, r.scans...)
	return cleanWatchPaths(out)
}

func (r *StencilRegistry) WatchPaths() []string {
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
