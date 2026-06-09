package stencil

import (
	"path/filepath"
	"strings"
)

type AssetKind string

const AssetJS AssetKind = "js"

const (
	BundleID   = "stack"
	BundleFile = "stack.esm.js"
)

type Workspace interface {
	AssetDir(kind AssetKind, id string) string
}

type Source interface {
	ID() string
	Materialize(Workspace, AssetKind) ([]string, error)
}

type WatchPathsProvider interface {
	WatchPaths() []string
}

type SourcePathsProvider interface {
	SourcePaths() []string
}

type AssetRef struct {
	Kind  AssetKind
	ID    string
	Files []string
}

type inputEntry struct {
	source Source
}

type Registry struct {
	entries []inputEntry
	scans   []string
	bundle  AssetRef
}

func NewRegistry() *Registry {
	return &Registry{
		entries: []inputEntry{},
		scans:   []string{},
		bundle: AssetRef{
			Kind:  AssetJS,
			ID:    BundleID,
			Files: []string{BundleFile},
		},
	}
}

func (r *Registry) AddInput(src Source) AssetRef {
	if r == nil || src == nil {
		return AssetRef{}
	}
	for _, entry := range r.entries {
		if entry.source != nil && entry.source.ID() == src.ID() {
			return r.bundle
		}
	}
	r.entries = append(r.entries, inputEntry{source: src})
	return r.bundle
}

func (r *Registry) AddScan(paths ...string) {
	if r == nil {
		return
	}
	r.scans = cleanWatchPaths(append(r.scans, paths...))
}

func (r *Registry) BundleRef() AssetRef {
	if r == nil {
		return AssetRef{}
	}
	return r.bundle
}

func (r *Registry) Inputs() []Source {
	if r == nil {
		return nil
	}
	out := make([]Source, 0, len(r.entries))
	for _, entry := range r.entries {
		if entry.source == nil {
			continue
		}
		out = append(out, entry.source)
	}
	return out
}

func (r *Registry) ScanPaths() []string {
	if r == nil {
		return nil
	}
	out := append([]string{}, r.scans...)
	return cleanWatchPaths(out)
}

func (r *Registry) WatchPaths() []string {
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

func cleanWatchPaths(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	out := make([]string, 0, len(paths))
	seen := map[string]struct{}{}
	for _, path := range paths {
		cleaned := strings.TrimSpace(path)
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

func SourcePaths(source Source) ([]string, error) {
	if source == nil {
		return nil, nil
	}
	if provider, ok := source.(SourcePathsProvider); ok {
		return append([]string{}, provider.SourcePaths()...), nil
	}
	if watcher, ok := source.(WatchPathsProvider); ok {
		return append([]string{}, watcher.WatchPaths()...), nil
	}
	return nil, nil
}
