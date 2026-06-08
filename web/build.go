package web

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	defaultWorkspaceDir = ".stack/workspace"
	defaultOutputDir    = ".stack/public"
	defaultAddr         = ":8080"
	defaultAssetRoot    = "assets"
)

type BuildConfig struct {
	WorkspaceDir         string
	OutputDir            string
	TailwindBinary       string
	TailwindVersion      string
	TailwindCacheDir     string
	TailwindDownloadBase string
	StencilBinary        string
	HugoBinary           string
	HugoVersion          string
	HugoCacheDir         string
}

type ServeConfig struct {
	Addr      string
	OutputDir string
	AssetsFS  fs.FS
	AssetRoot string
	DevState  *DevState
}

type DevConfig struct {
	Addr         string
	WorkspaceDir string
	OutputDir    string
	AssetsFS     fs.FS
	AssetRoot    string
	PollInterval time.Duration
	DevState     *DevState
}

type MaterializedAsset struct {
	Path string
	Size int64
}

type BuildResult struct {
	WorkspaceDir string
	SourceDir    string
	OutputDir    string
	AssetsDir    string
	Assets       []MaterializedAsset
}

type BuildEngine struct {
	builder *Builder
}

func NewBuildEngine(parts ...Part) *BuildEngine {
	return New(parts...).engine
}

func (e *BuildEngine) Build(ctx context.Context, cfg BuildConfig) (*BuildResult, error) {
	_ = ctx
	if e == nil || e.builder == nil {
		return nil, fmt.Errorf("build engine is nil")
	}

	workspace, err := NewWorkspace(cfg.WorkspaceDir)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.OutputDir) != "" {
		outputDir, err := filepath.Abs(cfg.OutputDir)
		if err == nil {
			workspace.Out = outputDir
		} else {
			workspace.Out = cfg.OutputDir
		}
	}
	publishDir := workspace.Out
	stagingOut := workspace.Out + ".next"
	workspace.Out = stagingOut

	start := time.Now()
	fmt.Fprintf(os.Stderr, "[stack] build start workspace=%s output=%s\n", workspace.Root, publishDir)

	if err := os.RemoveAll(workspace.Root); err != nil {
		return nil, err
	}
	if err := os.RemoveAll(stagingOut); err != nil {
		return nil, err
	}
	if err := workspace.Prepare(); err != nil {
		return nil, err
	}

	if err := e.materializeAssets(workspace); err != nil {
		return nil, err
	}

	if err := os.RemoveAll(workspace.Out); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(workspace.Out, 0o755); err != nil {
		return nil, err
	}
	if err := copyTreeExcept(workspace.Out, workspace.Src, func(rel string, entry fs.DirEntry) bool {
		rel = filepath.ToSlash(rel)
		tailwindPrefix := "tailwind/"
		return rel == "tailwind" || strings.HasPrefix(rel, tailwindPrefix)
	}); err != nil {
		return nil, err
	}
	if err := e.buildStyleBundle(ctx, workspace, cfg); err != nil {
		return nil, err
	}
	if err := e.buildStencilBundle(ctx, workspace, cfg); err != nil {
		return nil, err
	}

	assets, err := collectAssets(workspace.Out)
	if err != nil {
		return nil, err
	}
	if err := publishBuiltOutput(workspace.Out, publishDir); err != nil {
		return nil, err
	}
	workspace.Out = publishDir
	fmt.Fprintf(os.Stderr, "[stack] build complete assets=%d duration=%s output=%s\n", len(assets), time.Since(start).Round(time.Millisecond), publishDir)

	return &BuildResult{
		WorkspaceDir: workspace.Root,
		SourceDir:    workspace.Src,
		OutputDir:    publishDir,
		AssetsDir:    filepath.Join(publishDir, "assets"),
		Assets:       assets,
	}, nil
}

func (e *BuildEngine) BuildAssets(ctx context.Context, cfg BuildConfig) (*BuildResult, error) {
	_ = ctx
	if e == nil || e.builder == nil {
		return nil, fmt.Errorf("build engine is nil")
	}

	workspace, err := NewWorkspace(cfg.WorkspaceDir)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.OutputDir) != "" {
		outputDir, err := filepath.Abs(cfg.OutputDir)
		if err == nil {
			workspace.Out = outputDir
		} else {
			workspace.Out = cfg.OutputDir
		}
	}
	if err := workspace.Prepare(); err != nil {
		return nil, err
	}

	assetsRoot := filepath.Join(workspace.Out, "assets")
	if err := os.RemoveAll(assetsRoot); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(assetsRoot, 0o755); err != nil {
		return nil, err
	}
	if err := e.materializeAssets(workspace); err != nil {
		return nil, err
	}
	if info, err := os.Stat(workspace.SourceAssetsRoot()); err == nil && info.IsDir() {
		if err := copyTreeExcept(assetsRoot, workspace.SourceAssetsRoot(), nil); err != nil {
			return nil, err
		}
	}
	if err := e.buildStyleBundle(ctx, workspace, cfg); err != nil {
		return nil, err
	}
	if err := e.buildStencilBundle(ctx, workspace, cfg); err != nil {
		return nil, err
	}

	assets, err := collectAssets(workspace.Out)
	if err != nil {
		return nil, err
	}
	return &BuildResult{
		WorkspaceDir: workspace.Root,
		SourceDir:    workspace.Src,
		OutputDir:    workspace.Out,
		AssetsDir:    assetsRoot,
		Assets:       assets,
	}, nil
}

func (e *BuildEngine) Serve(ctx context.Context, cfg ServeConfig) error {
	if e == nil || e.builder == nil {
		return fmt.Errorf("build engine is nil")
	}
	outputDir := strings.TrimSpace(cfg.OutputDir)
	if outputDir == "" {
		outputDir = defaultOutputDir
	}
	addr := strings.TrimSpace(cfg.Addr)
	if addr == "" {
		addr = defaultAddr
	}

	registry := NewRegistry()
	registry.SetAssets(e.builder.Manifest())
	registry.SetDevState(cfg.DevState)
	e.builder.registerRoutes(registry)

	assetFS := cfg.AssetsFS
	assetRoot := strings.TrimSpace(cfg.AssetRoot)
	if assetRoot == "" {
		assetRoot = defaultAssetRoot
	}
	if assetFS == nil {
		if _, err := os.Stat(filepath.Join(outputDir, defaultAssetRoot)); err != nil {
			return fmt.Errorf("built assets not found in %s: %w", filepath.Join(outputDir, defaultAssetRoot), err)
		}
		assetFS = os.DirFS(outputDir)
	}

	registry.Mount("/assets", assetHandler(assetFS, assetRoot))
	registry.RegisterDevEndpoints(cfg.DevState)

	server := &http.Server{
		Addr:    addr,
		Handler: registry.Handler(),
	}

	errCh := make(chan error, 1)
	go func() {
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		_ = server.Shutdown(context.Background())
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func assetHandler(assetFS fs.FS, assetRoot string) http.Handler {
	if assetFS == nil {
		return http.NotFoundHandler()
	}
	assetRoot = strings.TrimSpace(assetRoot)
	if assetRoot == "" {
		assetRoot = defaultAssetRoot
	}
	if assetRoot == "." {
		assetRoot = ""
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath := strings.TrimPrefix(r.URL.Path, "/assets")
		requestPath = strings.TrimPrefix(requestPath, "/")
		if requestPath == "" {
			http.NotFound(w, r)
			return
		}
		if assetRoot != "" {
			requestPath = filepath.Join(assetRoot, requestPath)
		}
		requestPath = filepath.Clean(requestPath)
		requestPath = strings.TrimPrefix(requestPath, string(filepath.Separator))
		file, err := assetFS.Open(requestPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			http.NotFound(w, r)
			return
		}
		content, err := io.ReadAll(file)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, info.Name(), info.ModTime(), bytes.NewReader(content))
	})
}

func (e *BuildEngine) Dev(ctx context.Context, cfg DevConfig) error {
	if e == nil || e.builder == nil {
		return fmt.Errorf("build engine is nil")
	}
	if cfg.DevState == nil {
		cfg.DevState = NewDevState()
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 250 * time.Millisecond
	}
	if cfg.OutputDir == "" {
		cfg.OutputDir = defaultOutputDir
	}
	if cfg.WorkspaceDir == "" {
		cfg.WorkspaceDir = defaultWorkspaceDir
	}
	if cfg.Addr == "" {
		cfg.Addr = defaultAddr
	}
	workspaceAbs, err := filepath.Abs(cfg.WorkspaceDir)
	if err != nil {
		workspaceAbs = cfg.WorkspaceDir
	}
	tailwindCacheRoot := filepath.Join(filepath.Dir(workspaceAbs), "tailwind-cache")
	tailwindWorkspace := newTailwindWorkspace(tailwindCacheRoot)
	stencilCacheRoot := filepath.Join(filepath.Dir(workspaceAbs), "stencil-cache")

	if _, err := e.Build(ctx, BuildConfig{
		WorkspaceDir: cfg.WorkspaceDir,
		OutputDir:    cfg.OutputDir,
	}); err != nil {
		return err
	}
	if cfg.DevState != nil {
		cfg.DevState.MarkBuilt()
	}

	watchCtx, watchCancel := context.WithCancel(ctx)
	defer watchCancel()

	workers, err := e.startWatchWorkers(watchCtx, tailwindCacheRoot, stencilCacheRoot, cfg.OutputDir)
	if err != nil {
		return err
	}
	defer stopWatchWorkers(workers)

	serveCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- e.Serve(serveCtx, ServeConfig{
			Addr:      cfg.Addr,
			OutputDir: cfg.OutputDir,
			AssetsFS:  cfg.AssetsFS,
			AssetRoot: cfg.AssetRoot,
			DevState:  cfg.DevState,
		})
	}()

	paths := e.devOutputWatchPaths(cfg.OutputDir, stencilCacheRoot)
	fmt.Fprintf(os.Stderr, "[stack] dev watching %d roots\n", len(paths))
	for _, path := range paths {
		fmt.Fprintf(os.Stderr, "[stack]   watch %s\n", path)
	}
	snapshot, err := snapshotPaths(paths)
	if err != nil {
		return err
	}
	sourcePaths := e.devSourceWatchPaths()
	if len(sourcePaths) > 0 {
		fmt.Fprintf(os.Stderr, "[stack] dev mirroring %d source roots\n", len(sourcePaths))
		for _, path := range sourcePaths {
			fmt.Fprintf(os.Stderr, "[stack]   mirror %s\n", path)
		}
	}
	sourceSnapshot, err := snapshotPaths(sourcePaths)
	if err != nil {
		return err
	}
	dirty := false
	stencilDirty := false
	var lastChange time.Time

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			cancel()
			return ctx.Err()
		case err := <-errCh:
			return err
		case <-ticker.C:
			if len(sourcePaths) > 0 {
				currentSource, err := snapshotPaths(sourcePaths)
				if err != nil {
					return err
				}
				if !snapshotsEqual(sourceSnapshot, currentSource) {
					changed := diffSnapshotPaths(sourceSnapshot, currentSource)
					fmt.Fprintf(os.Stderr, "[stack] dev sources changed: %s\n", strings.Join(changed, ", "))
					tailwindTouched := false
					stencilTouched := false
					for _, path := range changed {
						if e.devTailwindSourceChanged(path) {
							fmt.Fprintf(os.Stderr, "[stack] dev tailwind source touched: %s\n", path)
							tailwindTouched = true
						}
						if e.devStencilSourceChanged(path) {
							fmt.Fprintf(os.Stderr, "[stack] dev stencil source touched: %s\n", path)
							stencilTouched = true
						}
					}
					if tailwindTouched {
						if err := e.rebuildTailwindBundle(ctx, cfg, tailwindWorkspace); err != nil {
							fmt.Fprintln(os.Stderr, "[stack] dev tailwind rebuild failed:", err)
							continue
						}
					}
					if stencilTouched {
						if err := e.syncStencilSourceMirror(cfg.WorkspaceDir); err != nil {
							fmt.Fprintln(os.Stderr, "[stack] dev stencil source sync failed:", err)
							continue
						}
					}
					sourceSnapshot = currentSource
				}
			}
			current, err := snapshotPaths(paths)
			if err != nil {
				return err
			}
			if !snapshotsEqual(snapshot, current) {
				changed := diffSnapshotPaths(snapshot, current)
				fmt.Fprintf(os.Stderr, "[stack] dev output changed: %s\n", strings.Join(changed, ", "))
				for _, path := range changed {
					if strings.HasPrefix(path, filepath.Join(stencilCacheRoot, "dist")) {
						stencilDirty = true
						break
					}
				}
				snapshot = current
				dirty = true
				lastChange = time.Now()
				continue
			}
			if !dirty {
				continue
			}
			if time.Since(lastChange) < cfg.PollInterval {
				continue
			}
			if stencilDirty {
				if err := e.syncDevOutputs(cfg.OutputDir, stencilCacheRoot); err != nil {
					fmt.Fprintln(os.Stderr, "[stack] dev output sync failed:", err)
					continue
				}
			}
			fmt.Fprintf(os.Stderr, "[stack] dev output settled, broadcasting reload\n")
			if cfg.DevState != nil {
				cfg.DevState.Broadcast()
			}
			dirty = false
			stencilDirty = false
		}
	}
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

func (e *BuildEngine) startWatchWorkers(ctx context.Context, tailwindCacheRoot, stencilCacheRoot, outputDir string) ([]watchWorker, error) {
	var workers []watchWorker

	if e.builder != nil && e.builder.tailwind != nil && len(e.builder.tailwind.Inputs()) > 0 {
		inputPath := filepath.Join(tailwindCacheRoot, "tailwind.input.css")
		if err := e.syncTailwindInput(newTailwindWorkspace(tailwindCacheRoot), inputPath); err != nil {
			return nil, err
		}
		outputPath := filepath.Join(outputDir, "assets", "css", "app", tailwindBundleFile)
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return nil, err
		}
		binaryPath, err := resolveTailwindBinary(ctx, BuildConfig{})
		if err != nil {
			return nil, err
		}
		worker, err := startCommandWatch(ctx, "tailwind", "", binaryPath, tailwindWatchArgs(inputPath, outputPath)...)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(os.Stderr, "[stack] tailwind watch started input=%s output=%s\n", inputPath, outputPath)
		workers = append(workers, worker)
	}

	if e.builder != nil && e.builder.stencil != nil && len(e.builder.stencil.Inputs()) > 0 {
		binaryPath, err := resolveStencilBinary(BuildConfig{})
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Join(outputDir, "assets", "js", stencilBundleID), 0o755); err != nil {
			return nil, err
		}
		worker, err := startCommandWatch(ctx, "stencil", stencilCacheRoot, binaryPath, stencilWatchArgs(binaryPath)...)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(os.Stderr, "[stack] stencil watch started cache=%s\n", stencilCacheRoot)
		workers = append(workers, worker)
	}

	return workers, nil
}

func (e *BuildEngine) devSourceWatchPaths() []string {
	if e == nil || e.builder == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var paths []string
	if e.builder.tailwind != nil {
		for _, source := range e.builder.tailwind.Inputs() {
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
	if e.builder.stencil != nil {
		for _, source := range e.builder.stencil.Inputs() {
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
	sort.Strings(paths)
	return paths
}

func (e *BuildEngine) syncStencilSourceMirror(workspaceDir string) error {
	if e == nil || e.builder == nil || e.builder.stencil == nil {
		return nil
	}
	workspaceAbs, err := filepath.Abs(workspaceDir)
	if err != nil {
		workspaceAbs = workspaceDir
	}
	cacheRoot := filepath.Join(filepath.Dir(workspaceAbs), "stencil-cache")
	stencilWorkspace := &Workspace{
		Root: cacheRoot,
		Src:  filepath.Join(cacheRoot, "src"),
		Out:  filepath.Join(cacheRoot, "dist"),
		Temp: filepath.Join(cacheRoot, "tmp"),
	}
	if err := os.MkdirAll(stencilWorkspace.Root, 0o755); err != nil {
		return err
	}
	if err := os.RemoveAll(stencilWorkspace.Src); err != nil {
		return err
	}
	if err := os.MkdirAll(stencilWorkspace.Src, 0o755); err != nil {
		return err
	}
	for _, source := range e.builder.stencil.Inputs() {
		if source == nil {
			continue
		}
		if _, err := source.Materialize(stencilWorkspace, AssetKindJS); err != nil {
			return err
		}
	}
	return nil
}

func (e *BuildEngine) syncTailwindInput(workspace *Workspace, inputPath string) error {
	if workspace == nil {
		return fmt.Errorf("tailwind workspace is nil")
	}
	if err := os.MkdirAll(workspace.Src, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(inputPath), 0o755); err != nil {
		return err
	}
	input, err := e.tailwindInput(workspace)
	if err != nil {
		return err
	}
	return os.WriteFile(inputPath, []byte(input), 0o644)
}

func (e *BuildEngine) rebuildTailwindBundle(ctx context.Context, cfg DevConfig, workspace *Workspace) error {
	if e == nil || e.builder == nil || e.builder.tailwind == nil || workspace == nil {
		return nil
	}
	if len(e.builder.tailwind.Inputs()) == 0 {
		return nil
	}
	inputPath := filepath.Join(workspace.Root, "tailwind.input.css")
	if err := e.syncTailwindInput(workspace, inputPath); err != nil {
		return err
	}
	binaryPath, err := resolveTailwindBinary(ctx, BuildConfig{})
	if err != nil {
		return err
	}
	outputPath := filepath.Join(cfg.OutputDir, "assets", "css", "app", tailwindBundleFile)
	fmt.Fprintf(os.Stderr, "[stack] dev tailwind rebuild input=%s output=%s\n", inputPath, outputPath)
	return runTailwind(ctx, binaryPath, inputPath, outputPath)
}

func (e *BuildEngine) devTailwindSourceChanged(path string) bool {
	if e == nil || e.builder == nil || e.builder.tailwind == nil {
		return false
	}
	if strings.EqualFold(filepath.Ext(path), ".css") {
		return true
	}
	for _, source := range e.builder.tailwind.Inputs() {
		paths, err := tailwindSourcePaths(source)
		if err != nil {
			continue
		}
		for _, candidate := range paths {
			if sourcePathMatches(candidate, path) {
				return true
			}
		}
	}
	return false
}

func (e *BuildEngine) devStencilSourceChanged(path string) bool {
	if e == nil || e.builder == nil || e.builder.stencil == nil {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".ts" || ext == ".tsx" {
		return true
	}
	for _, source := range e.builder.stencil.Inputs() {
		paths, err := tailwindSourcePaths(source)
		if err != nil {
			continue
		}
		for _, candidate := range paths {
			if sourcePathMatches(candidate, path) {
				return true
			}
		}
	}
	return false
}

func sourcePathMatches(candidate, changed string) bool {
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

func tailwindWatchArgs(inputPath, outputPath string) []string {
	return []string{"-i", inputPath, "-o", outputPath, "--watch", "--minify"}
}

func stencilWatchArgs(binaryPath string) []string {
	base := strings.ToLower(filepath.Base(binaryPath))
	switch base {
	case "npm":
		return []string{"exec", "--yes", "--package=@stencil/core", "--", "stencil", "build", "--watch"}
	case "npx":
		return []string{"--yes", "stencil", "build", "--watch"}
	default:
		return []string{"build", "--watch"}
	}
}

func (e *BuildEngine) devOutputWatchPaths(outputDir, stencilCacheRoot string) []string {
	var paths []string
	tailwindOutput := filepath.Join(outputDir, "assets", "css", "app", tailwindBundleFile)
	if _, err := os.Stat(tailwindOutput); err == nil {
		paths = append(paths, tailwindOutput)
	}
	stencilOutput := filepath.Join(stencilCacheRoot, "dist")
	if _, err := os.Stat(stencilOutput); err == nil {
		paths = append(paths, stencilOutput)
	}
	sort.Strings(paths)
	return paths
}

func (e *BuildEngine) syncDevOutputs(outputDir, stencilCacheRoot string) error {
	stencilSource := filepath.Join(stencilCacheRoot, "dist", stencilBundleID)
	stencilDest := filepath.Join(outputDir, "assets", "js", stencilBundleID)
	if info, err := os.Stat(stencilSource); err == nil && info.IsDir() {
		if err := os.RemoveAll(stencilDest); err != nil {
			return err
		}
		if err := os.MkdirAll(stencilDest, 0o755); err != nil {
			return err
		}
		if err := copyTree(stencilDest, stencilSource); err != nil {
			return err
		}
	}

	loaderSource := filepath.Join(stencilCacheRoot, "dist", "loader")
	loaderDest := filepath.Join(outputDir, "assets", "js", stencilBundleID, "loader")
	if info, err := os.Stat(loaderSource); err == nil && info.IsDir() {
		if err := os.RemoveAll(loaderDest); err != nil {
			return err
		}
		if err := os.MkdirAll(loaderDest, 0o755); err != nil {
			return err
		}
		if err := copyTree(loaderDest, loaderSource); err != nil {
			return err
		}
	}
	return nil
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

func (e *BuildEngine) materializeAssets(workspace *Workspace) error {
	if e == nil || e.builder == nil || workspace == nil {
		return nil
	}
	for _, entry := range e.builder.assets.Entries() {
		if entry.Source == nil {
			continue
		}
		if _, err := entry.Source.Materialize(workspace, entry.Kind); err != nil {
			return err
		}
	}
	return nil
}

func (e *BuildEngine) watchPaths() []string {
	if e == nil || e.builder == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var paths []string
	if e.builder.tailwind != nil {
		for _, p := range e.builder.tailwind.WatchPaths() {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if _, exists := seen[p]; exists {
				continue
			}
			seen[p] = struct{}{}
			paths = append(paths, p)
		}
	}
	if e.builder.stencil != nil {
		for _, p := range e.builder.stencil.WatchPaths() {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if _, exists := seen[p]; exists {
				continue
			}
			seen[p] = struct{}{}
			paths = append(paths, p)
		}
	}
	if e.builder.assets == nil {
		sort.Strings(paths)
		return paths
	}
	for _, entry := range e.builder.assets.Entries() {
		if watcher, ok := entry.Source.(WatchPathsProvider); ok {
			for _, p := range watcher.WatchPaths() {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				if _, exists := seen[p]; exists {
					continue
				}
				seen[p] = struct{}{}
				paths = append(paths, p)
			}
		}
	}
	sort.Strings(paths)
	return paths
}

func collectAssets(root string) ([]MaterializedAsset, error) {
	var assets []MaterializedAsset
	err := filepath.WalkDir(root, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		assets = append(assets, MaterializedAsset{Path: rel, Size: info.Size()})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(assets, func(i, j int) bool { return assets[i].Path < assets[j].Path })
	return assets, nil
}

func publishBuiltOutput(stagingDir, publishDir string) error {
	stagingDir = strings.TrimSpace(stagingDir)
	publishDir = strings.TrimSpace(publishDir)
	if stagingDir == "" || publishDir == "" {
		return nil
	}
	if stagingDir == publishDir {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(publishDir), 0o755); err != nil {
		return err
	}

	backupDir := publishDir + ".old"
	_ = os.RemoveAll(backupDir)
	if _, err := os.Stat(publishDir); err == nil {
		if err := os.Rename(publishDir, backupDir); err != nil {
			return err
		}
	}
	if err := os.Rename(stagingDir, publishDir); err != nil {
		if _, statErr := os.Stat(backupDir); statErr == nil {
			_ = os.Rename(backupDir, publishDir)
		}
		return err
	}
	_ = os.RemoveAll(backupDir)
	return nil
}

type fileSignature struct {
	Size    int64
	ModTime int64
}

func snapshotPaths(paths []string) (map[string]fileSignature, error) {
	snapshot := map[string]fileSignature{}
	skipDirs := map[string]struct{}{
		".git":         {},
		".stack":       {},
		"node_modules": {},
		"dist":         {},
		"build":        {},
		"coverage":     {},
		"vendor":       {},
	}
	for _, root := range paths {
		info, err := os.Stat(root)
		if err != nil {
			continue
		}
		if info.IsDir() {
			err = filepath.WalkDir(root, func(current string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() && current != root {
					if _, skip := skipDirs[filepath.Base(current)]; skip {
						return filepath.SkipDir
					}
				}
				if entry.IsDir() {
					return nil
				}
				info, err := entry.Info()
				if err != nil {
					return err
				}
				snapshot[current] = fileSignature{Size: info.Size(), ModTime: info.ModTime().UnixNano()}
				return nil
			})
			if err != nil {
				return nil, err
			}
			continue
		}
		snapshot[root] = fileSignature{Size: info.Size(), ModTime: info.ModTime().UnixNano()}
	}
	return snapshot, nil
}

func snapshotsEqual(a, b map[string]fileSignature) bool {
	if len(a) != len(b) {
		return false
	}
	for key, left := range a {
		right, ok := b[key]
		if !ok || right != left {
			return false
		}
	}
	return true
}
