package pipeline

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	assetspkg "github.com/cleanstartup/stack/asset"
)

type WatchWorker struct {
	Name   string
	Cmd    *exec.Cmd
	DoneCh chan error
}

type FileSignature struct {
	Size    int64
	ModTime int64
	IsDir   bool
}

func StartCommandWatch(ctx context.Context, name string, workDir string, binaryPath string, args ...string) (WatchWorker, error) {
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
		return WatchWorker{}, fmt.Errorf("%s watch start failed via %s: %w", name, binaryPath, err)
	}
	worker := WatchWorker{
		Name:   name,
		Cmd:    cmd,
		DoneCh: make(chan error, 1),
	}
	go func() {
		worker.DoneCh <- cmd.Wait()
	}()
	return worker, nil
}

func StartCommandWatchSpec(ctx context.Context, name string, spec assetspkg.CommandSpec) (WatchWorker, error) {
	return StartCommandWatch(ctx, name, spec.WorkDir, spec.Binary, spec.Args...)
}

func StopWatchWorkers(workers []WatchWorker) {
	for _, worker := range workers {
		if worker.Cmd != nil && worker.Cmd.Process != nil {
			_ = worker.Cmd.Process.Kill()
		}
	}
	for _, worker := range workers {
		if worker.DoneCh == nil {
			continue
		}
		select {
		case <-worker.DoneCh:
		default:
		}
	}
}

func SourcePathMatches(candidate, changed string) bool {
	candidate = filepath.Clean(strings.TrimSpace(candidate))
	changed = filepath.Clean(strings.TrimSpace(changed))
	if candidate == "" || changed == "" {
		return false
	}
	if candidate == changed {
		return true
	}
	prefix := candidate + string(filepath.Separator)
	return strings.HasPrefix(changed, prefix)
}

func SnapshotPaths(paths []string) (map[string]FileSignature, error) {
	snapshot := map[string]FileSignature{}
	for _, root := range paths {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		info, err := os.Stat(root)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			snapshot[root] = FileSignature{Size: info.Size(), ModTime: info.ModTime().UnixNano()}
			continue
		}
		err = filepath.WalkDir(root, func(current string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			snapshot[current] = FileSignature{Size: info.Size(), ModTime: info.ModTime().UnixNano(), IsDir: entry.IsDir()}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return snapshot, nil
}

func SnapshotsEqual(a, b map[string]FileSignature) bool {
	if len(a) != len(b) {
		return false
	}
	for path, left := range a {
		right, ok := b[path]
		if !ok || right != left {
			return false
		}
	}
	return true
}

func DiffSnapshotPaths(before, after map[string]FileSignature) []string {
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
