package web

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type Workspace struct {
	Root string
	Src  string
	Out  string
	Temp string
}

func NewWorkspace(root string) (*Workspace, error) {
	if stringsTrim(root) == "" {
		root = filepath.Join(".way2go", "workspace")
	}
	abs, err := filepath.Abs(root)
	if err == nil {
		root = abs
	}
	return &Workspace{
		Root: root,
		Src:  filepath.Join(root, "src"),
		Out:  filepath.Join(root, "out"),
		Temp: filepath.Join(root, "tmp"),
	}, nil
}

func (w *Workspace) Prepare() error {
	if w == nil {
		return fmt.Errorf("workspace is nil")
	}
	for _, dir := range []string{w.Root, w.Src, w.Out, w.Temp} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (w *Workspace) Clean() error {
	if w == nil {
		return fmt.Errorf("workspace is nil")
	}
	return os.RemoveAll(w.Root)
}

func (w *Workspace) AssetDir(kind AssetKind, id string) string {
	if w == nil {
		return ""
	}
	return filepath.Join(w.Src, "assets", string(kind), id)
}

func (w *Workspace) OutputAssetDir(kind AssetKind, id string) string {
	if w == nil {
		return ""
	}
	return filepath.Join(w.Out, "assets", string(kind), id)
}

func (w *Workspace) OutputAssetsRoot() string {
	if w == nil {
		return ""
	}
	return filepath.Join(w.Out, "assets")
}

func (w *Workspace) SourceAssetsRoot() string {
	if w == nil {
		return ""
	}
	return filepath.Join(w.Src, "assets")
}

func walkTree(root string, visit func(path string, entry fs.DirEntry) error) error {
	return filepath.WalkDir(root, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return visit(current, entry)
	})
}

func copyTree(dst, src string) error {
	return copyTreeExcept(dst, src, nil)
}

func copyTreeExcept(dst, src string, skip func(rel string, entry fs.DirEntry) bool) error {
	return filepath.WalkDir(src, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, current)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if skip != nil && skip(rel, entry) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(target, current)
	})
}

func stringsTrim(value string) string {
	for len(value) > 0 {
		switch value[0] {
		case ' ', '\t', '\n', '\r':
			value = value[1:]
		default:
			goto right
		}
	}
right:
	for len(value) > 0 {
		switch value[len(value)-1] {
		case ' ', '\t', '\n', '\r':
			value = value[:len(value)-1]
		default:
			return value
		}
	}
	return value
}
