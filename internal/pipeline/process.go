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
	"sync"
	"time"

	assetspkg "github.com/cleanstartup/stack/internal/asset"
)

type WatchWorker struct {
	Name   string
	Cmd    *exec.Cmd
	DoneCh chan error

	state *watchWorkerState
}

type watchWorkerState struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	stopCh  chan struct{}
	stopped bool
}

type FileSignature struct {
	Size    int64
	ModTime int64
	IsDir   bool
}

func StartCommandWatch(ctx context.Context, name string, workDir string, binaryPath string, args ...string) (WatchWorker, error) {
	return startCommandWatch(ctx, name, workDir, binaryPath, false, args...)
}

func StartRestartingCommandWatch(ctx context.Context, name string, workDir string, binaryPath string, args ...string) (WatchWorker, error) {
	return startCommandWatch(ctx, name, workDir, binaryPath, true, args...)
}

func startCommandWatch(ctx context.Context, name string, workDir string, binaryPath string, restart bool, args ...string) (WatchWorker, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	worker := WatchWorker{
		Name:   name,
		DoneCh: make(chan error, 1),
		state:  &watchWorkerState{stopCh: make(chan struct{})},
	}
	go func() {
		worker.DoneCh <- worker.run(ctx, workDir, binaryPath, restart, args...)
	}()
	return worker, nil
}

func StartCommandWatchSpec(ctx context.Context, name string, spec assetspkg.CommandSpec) (WatchWorker, error) {
	return StartCommandWatch(ctx, name, spec.WorkDir, spec.Binary, spec.Args...)
}

func StartRestartingCommandWatchSpec(ctx context.Context, name string, spec assetspkg.CommandSpec) (WatchWorker, error) {
	return StartRestartingCommandWatch(ctx, name, spec.WorkDir, spec.Binary, spec.Args...)
}

func StopWatchWorkers(workers []WatchWorker) {
	for _, worker := range workers {
		worker.stop()
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

func (w *WatchWorker) stop() {
	if w == nil {
		return
	}
	if w.state == nil {
		return
	}
	w.state.mu.Lock()
	if !w.state.stopped {
		w.state.stopped = true
		close(w.state.stopCh)
	}
	cmd := w.state.cmd
	w.state.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

func (w *WatchWorker) run(ctx context.Context, workDir, binaryPath string, restart bool, args ...string) error {
	if w == nil {
		return nil
	}
	if w.state == nil {
		return nil
	}
	backoff := 200 * time.Millisecond
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.state.stopCh:
			return nil
		default:
		}

		cmd := exec.CommandContext(ctx, binaryPath, args...)
		if strings.TrimSpace(workDir) != "" {
			cmd.Dir = workDir
		}
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("%s watch start failed via %s: %w", w.Name, binaryPath, err)
		}
		w.state.mu.Lock()
		w.state.cmd = cmd
		w.state.mu.Unlock()

		err := cmd.Wait()
		w.state.mu.Lock()
		stopped := w.state.stopped
		w.state.mu.Unlock()
		if stopped || ctx.Err() != nil {
			return err
		}
		if !restart {
			return err
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "[stack] %s watch exited, restarting: %v\n", w.Name, err)
		} else {
			fmt.Fprintf(os.Stderr, "[stack] %s watch exited, restarting\n", w.Name)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.state.stopCh:
			return nil
		case <-time.After(backoff):
		}
		if backoff < 2*time.Second {
			backoff *= 2
			if backoff > 2*time.Second {
				backoff = 2 * time.Second
			}
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
