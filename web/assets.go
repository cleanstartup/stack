package web

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type AssetKind string

const (
	AssetKindCSS  AssetKind = "css"
	AssetKindJS   AssetKind = "js"
	AssetKindFile AssetKind = "file"
)

type AssetSource interface {
	ID() string
	Materialize(*Workspace, AssetKind) ([]string, error)
}

type AssetNamer interface {
	AssetNames() []string
}

type AssetRef struct {
	Kind  AssetKind
	ID    string
	Files []string
}

func (r AssetRef) URLs() []string {
	if len(r.Files) == 0 {
		return []string{AssetURL(r.Kind, r.ID)}
	}
	out := make([]string, 0, len(r.Files))
	for _, file := range r.Files {
		if strings.TrimSpace(file) == "" {
			continue
		}
		out = append(out, AssetURL(r.Kind, r.ID, file))
	}
	return out
}

func (r AssetRef) URL() string {
	urls := r.URLs()
	if len(urls) == 0 {
		return ""
	}
	return urls[0]
}

type WatchPathsProvider interface {
	WatchPaths() []string
}

type AssetEntry struct {
	Kind   AssetKind
	Source AssetSource
}

type AssetRegistry struct {
	entries []AssetEntry
}

func NewAssetRegistry() *AssetRegistry {
	return &AssetRegistry{entries: []AssetEntry{}}
}

func (r *AssetRegistry) Add(kind AssetKind, src AssetSource) {
	if r == nil || src == nil {
		return
	}
	r.entries = append(r.entries, AssetEntry{Kind: kind, Source: src})
}

func (r *AssetRegistry) Entries() []AssetEntry {
	if r == nil {
		return nil
	}
	out := make([]AssetEntry, len(r.entries))
	copy(out, r.entries)
	return out
}

type fileAssetSource struct {
	id   string
	path string
}

func FromFile(path string) AssetSource {
	return fileAssetSource{
		id:   assetID(path),
		path: path,
	}
}

func (s fileAssetSource) ID() string { return s.id }

func (s fileAssetSource) AssetNames() []string { return []string{filepath.Base(s.path)} }

func (s fileAssetSource) Materialize(ws *Workspace, kind AssetKind) ([]string, error) {
	if ws == nil {
		return nil, fmt.Errorf("workspace is nil")
	}
	dstDir := ws.AssetDir(kind, s.ID())
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return nil, err
	}
	name := filepath.Base(s.path)
	if err := copyFile(filepath.Join(dstDir, name), s.path); err != nil {
		return nil, err
	}
	return []string{name}, nil
}

func (s fileAssetSource) WatchPaths() []string { return []string{s.path} }

type dirAssetSource struct {
	id   string
	path string
}

func FromDir(path string) AssetSource {
	return dirAssetSource{
		id:   assetID(path),
		path: path,
	}
}

func (s dirAssetSource) ID() string { return s.id }

func (s dirAssetSource) AssetNames() []string { return []string{filepath.Base(s.path)} }

func (s dirAssetSource) Materialize(ws *Workspace, kind AssetKind) ([]string, error) {
	if ws == nil {
		return nil, fmt.Errorf("workspace is nil")
	}
	dstDir := ws.AssetDir(kind, s.ID())
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return nil, err
	}
	return copyDir(dstDir, s.path)
}

func (s dirAssetSource) WatchPaths() []string { return []string{s.path} }

type fsAssetSource struct {
	id     string
	source fs.FS
	root   string
}

func FromFS(source fs.FS, root string) AssetSource {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	return fsAssetSource{
		id:     assetID(root),
		source: source,
		root:   root,
	}
}

func (s fsAssetSource) ID() string { return s.id }

func (s fsAssetSource) AssetNames() []string { return []string{filepath.Base(s.root)} }

func (s fsAssetSource) Materialize(ws *Workspace, kind AssetKind) ([]string, error) {
	if ws == nil {
		return nil, fmt.Errorf("workspace is nil")
	}
	dstDir := ws.AssetDir(kind, s.ID())
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return nil, err
	}
	sub, err := fs.Sub(s.source, s.root)
	if err != nil {
		return nil, err
	}
	return copyFS(dstDir, sub)
}

type generatedAssetSource struct {
	id    string
	write func(dst string) error
}

func Generated(id string, write func(dst string) error) AssetSource {
	return generatedAssetSource{
		id:    assetID(id),
		write: write,
	}
}

func (s generatedAssetSource) ID() string { return s.id }

func (s generatedAssetSource) AssetNames() []string { return []string{filepath.Base(s.id)} }

func (s generatedAssetSource) Materialize(ws *Workspace, kind AssetKind) ([]string, error) {
	if ws == nil {
		return nil, fmt.Errorf("workspace is nil")
	}
	if s.write == nil {
		return nil, nil
	}
	dstDir := ws.AssetDir(kind, s.ID())
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return nil, err
	}
	if err := s.write(dstDir); err != nil {
		return nil, err
	}
	return listFiles(dstDir)
}

func AssetURL(kind AssetKind, id string, parts ...string) string {
	segments := append([]string{"/assets", string(kind), id}, parts...)
	return path.Join(segments...)
}

func assetID(value string) string {
	cleaned := strings.TrimSpace(value)
	if cleaned == "" {
		return "asset"
	}
	sum := sha1.Sum([]byte(cleaned))
	suffix := hex.EncodeToString(sum[:4])
	base := filepath.Base(cleaned)
	base = strings.TrimSpace(base)
	if base == "." || base == string(filepath.Separator) || base == "" {
		base = "asset"
	}
	base = strings.ReplaceAll(base, string(filepath.Separator), "_")
	base = strings.ReplaceAll(base, " ", "_")
	return sanitizeSegment(base) + "_" + suffix
}

func sanitizeSegment(value string) string {
	value = strings.ToLower(value)
	var out strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			out.WriteRune(r)
		case r >= '0' && r <= '9':
			out.WriteRune(r)
		case r == '-' || r == '_':
			out.WriteRune(r)
		default:
			out.WriteRune('_')
		}
	}
	result := strings.Trim(out.String(), "_")
	if result == "" {
		return "asset"
	}
	return result
}

func copyFile(dst, src string) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	output, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer output.Close()

	_, err = io.Copy(output, input)
	return err
}

func copyDir(dst, src string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(src, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, current)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := copyFile(target, current); err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	return files, err
}

func copyFS(dst string, source fs.FS) ([]string, error) {
	var files []string
	err := fs.WalkDir(source, ".", func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if current == "." {
			return os.MkdirAll(dst, 0o755)
		}
		target := filepath.Join(dst, current)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, err := fs.ReadFile(source, current)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, content, 0o644); err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(current))
		return nil
	})
	return files, err
}

func listFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	return files, err
}
