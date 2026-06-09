package hugo

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	stencilpkg "github.com/cleanstartup/stack/stencil"
	tailwindpkg "github.com/cleanstartup/stack/tailwind"
	"github.com/cleanstartup/stack/web"
)

func (e *BuildEngine) BuildAssets(ctx context.Context, cfg BuildConfig) (*BuildResult, error) {
	if e == nil || e.core == nil {
		return nil, fmt.Errorf("build engine is nil")
	}
	return e.core.BuildAssets(ctx, cfg)
}

func (e *BuildEngine) Build(ctx context.Context, cfg BuildConfig) (*BuildResult, error) {
	if e == nil || e.core == nil {
		return nil, fmt.Errorf("build engine is nil")
	}
	return e.core.Build(ctx, cfg)
}

func (e *BuildEngine) Serve(ctx context.Context, cfg ServeConfig) error {
	if e == nil || e.core == nil {
		return fmt.Errorf("build engine is nil")
	}
	return e.core.Serve(ctx, cfg)
}

func (e *BuildEngine) Dev(ctx context.Context, cfg DevConfig) error {
	if e == nil || e.core == nil {
		return fmt.Errorf("build engine is nil")
	}
	return e.core.Dev(ctx, cfg)
}

func (e *BuildEngine) contentModules(moduleRoot string) ([]HugoModule, error) {
	if e == nil || e.builder == nil || e.builder.ContentRegistry() == nil {
		return nil, nil
	}
	mods, err := e.builder.ContentRegistry().Modules(moduleRoot)
	if err != nil {
		return nil, err
	}
	out := make([]HugoModule, 0, len(mods))
	for _, mod := range mods {
		out = append(out, HugoModule{ImportPath: mod.ImportPath, ReplacePath: mod.ReplacePath})
	}
	return out, nil
}

func (e *BuildEngine) layoutModules(moduleRoot string) ([]HugoModule, error) {
	if e == nil || e.builder == nil || e.builder.LayoutRegistry() == nil {
		return nil, nil
	}
	mods, err := e.builder.LayoutRegistry().Modules(moduleRoot)
	if err != nil {
		return nil, err
	}
	out := make([]HugoModule, 0, len(mods))
	for _, mod := range mods {
		out = append(out, HugoModule{ImportPath: mod.ImportPath, ReplacePath: mod.ReplacePath})
	}
	return out, nil
}

func (e *BuildEngine) devSourceWatchPaths() []string {
	if e == nil || e.builder == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var paths []string
	if e.builder.Styles() != nil {
		for _, source := range e.builder.Styles().Inputs() {
			sourcePaths, err := tailwindSourcePaths(source)
			if err != nil {
				continue
			}
			for _, path := range sourcePaths {
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
	if e.builder.Components() != nil {
		for _, source := range e.builder.Components().Inputs() {
			sourcePaths, err := tailwindSourcePaths(source)
			if err != nil {
				continue
			}
			for _, path := range sourcePaths {
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
	if e.builder.ContentRegistry() != nil {
		for _, path := range e.builder.ContentRegistry().WatchPaths() {
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
	if e.builder.LayoutRegistry() != nil {
		for _, path := range e.builder.LayoutRegistry().WatchPaths() {
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
	sort.Strings(paths)
	return paths
}

type watchWorker struct {
	name   string
	cmd    *exec.Cmd
	doneCh chan error
}

func startCommandWatch(ctx context.Context, name string, workDir string, binaryPath string, args ...string) (watchWorker, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	if strings.TrimSpace(workDir) != "" {
		cmd.Dir = workDir
	}
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return watchWorker{}, fmt.Errorf("%s watch start failed via %s: %w", name, binaryPath, err)
	}
	worker := watchWorker{
		name:   name,
		cmd:    cmd,
		doneCh: make(chan error, 1),
	}
	go func() {
		worker.doneCh <- cmd.Wait()
	}()
	return worker, nil
}

func stopWatchWorkers(workers []watchWorker) {
	for _, worker := range workers {
		if worker.cmd != nil && worker.cmd.Process != nil {
			_ = worker.cmd.Process.Kill()
		}
	}
	for _, worker := range workers {
		if worker.doneCh == nil {
			continue
		}
		select {
		case <-worker.doneCh:
		default:
		}
	}
}

func (e *BuildEngine) startWatchWorkers(ctx context.Context, cfg BuildConfig, tailwindCacheRoot, stencilCacheRoot, outputDir string) ([]watchWorker, error) {
	var workers []watchWorker
	if e == nil || e.builder == nil {
		return workers, nil
	}
	if e.builder.Styles() != nil && len(e.builder.Styles().Inputs()) > 0 {
		inputPath := filepath.Join(tailwindCacheRoot, "tailwind.input.css")
		if err := e.syncTailwindInput(newTailwindWorkspace(tailwindCacheRoot), inputPath); err != nil {
			return nil, err
		}
		outputPath := filepath.Join(outputDir, "assets", "css", "app", tailwindBundleFile)
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return nil, err
		}
		spec, err := tailwindpkg.DevCommand(ctx, tailwindpkg.Config{ProjectDir: cfg.ProjectDir}, inputPath, outputPath)
		if err != nil {
			return nil, err
		}
		worker, err := startCommandWatch(ctx, "tailwind", spec.WorkDir, spec.Binary, spec.Args...)
		if err != nil {
			return nil, err
		}
		workers = append(workers, worker)
	}
	if e.builder.Components() != nil && len(e.builder.Components().Inputs()) > 0 {
		if err := os.MkdirAll(filepath.Join(outputDir, "assets", "js", stencilBundleID), 0o755); err != nil {
			return nil, err
		}
		spec, err := stencilpkg.DevCommand(stencilpkg.Config{ProjectDir: cfg.ProjectDir})
		if err != nil {
			return nil, err
		}
		worker, err := startCommandWatch(ctx, "stencil", spec.WorkDir, spec.Binary, spec.Args...)
		if err != nil {
			return nil, err
		}
		workers = append(workers, worker)
	}
	return workers, nil
}

type fileSignature struct {
	Size    int64
	ModTime int64
	IsDir   bool
}

func snapshotPaths(paths []string) (map[string]fileSignature, error) {
	snapshot := map[string]fileSignature{}
	for _, root := range paths {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info == nil {
				return nil
			}
			snapshot[path] = fileSignature{Size: info.Size(), ModTime: info.ModTime().UnixNano(), IsDir: info.IsDir()}
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	return snapshot, nil
}

func snapshotsEqual(before, after map[string]fileSignature) bool {
	if len(before) != len(after) {
		return false
	}
	for path, sig := range before {
		if after[path] != sig {
			return false
		}
	}
	return true
}

func diffSnapshotPaths(before, after map[string]fileSignature) []string {
	seen := map[string]struct{}{}
	var changed []string
	for path, left := range before {
		right, ok := after[path]
		if !ok || right != left {
			if _, exists := seen[path]; !exists {
				seen[path] = struct{}{}
				changed = append(changed, path)
			}
		}
	}
	for path, right := range after {
		left, ok := before[path]
		if !ok || right != left {
			if _, exists := seen[path]; !exists {
				seen[path] = struct{}{}
				changed = append(changed, path)
			}
		}
	}
	sort.Strings(changed)
	return changed
}

func collectAssets(root string) ([]web.MaterializedAsset, error) {
	var assets []web.MaterializedAsset
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info == nil || info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		assets = append(assets, web.MaterializedAsset{Path: rel, Size: info.Size()})
		return nil
	})
	return assets, err
}

func copyTree(dst, src string) error {
	return copyTreeExcept(dst, src, nil)
}

func copyTreeExcept(dst, src string, skip func(rel string, entry fs.DirEntry) bool) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if skip != nil && skip(rel, entry) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func publishBuiltOutput(stagingDir, publishDir string) error {
	if err := os.RemoveAll(publishDir); err != nil {
		return err
	}
	return os.Rename(stagingDir, publishDir)
}
