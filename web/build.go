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
	"path/filepath"
	"sort"
	"strings"
	"time"

	pipelinepkg "github.com/cleanstartup/stack/pipeline"
	stencilpkg "github.com/cleanstartup/stack/stencil"
	tailwindpkg "github.com/cleanstartup/stack/tailwind"
)

const (
	defaultAddr      = ":8080"
	defaultAssetRoot = "assets"
)

type BuildConfig struct {
	ProjectDir           string
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
	ProjectDir   string
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
	return NewApp(parts...).engine
}

func (e *BuildEngine) Builder() *Builder {
	if e == nil {
		return nil
	}
	return e.builder
}

func (e *BuildEngine) Build(ctx context.Context, cfg BuildConfig) (*BuildResult, error) {
	_ = ctx
	if e == nil || e.builder == nil {
		return nil, fmt.Errorf("build engine is nil")
	}
	if strings.TrimSpace(cfg.WorkspaceDir) == "" {
		cfg.WorkspaceDir = DefaultWorkspaceDir(cfg.ProjectDir)
	}
	if strings.TrimSpace(cfg.OutputDir) == "" {
		cfg.OutputDir = DefaultOutputDir(cfg.ProjectDir)
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
	if err := e.materializeProjectFiles(cfg.ProjectDir, workspace.Root); err != nil {
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
	if strings.TrimSpace(cfg.WorkspaceDir) == "" {
		cfg.WorkspaceDir = DefaultWorkspaceDir(cfg.ProjectDir)
	}
	if strings.TrimSpace(cfg.OutputDir) == "" {
		cfg.OutputDir = DefaultOutputDir(cfg.ProjectDir)
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
	if err := e.materializeProjectFiles(cfg.ProjectDir, workspace.Root); err != nil {
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
		outputDir = DefaultOutputDir("")
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
		cfg.OutputDir = DefaultOutputDir(cfg.ProjectDir)
	}
	if abs, err := filepath.Abs(cfg.OutputDir); err == nil {
		cfg.OutputDir = abs
	}
	if cfg.WorkspaceDir == "" {
		cfg.WorkspaceDir = DefaultWorkspaceDir(cfg.ProjectDir)
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
		ProjectDir:   cfg.ProjectDir,
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

	workers, err := e.startWatchWorkers(watchCtx, cfg, tailwindCacheRoot, stencilCacheRoot, cfg.OutputDir)
	if err != nil {
		return err
	}
	defer pipelinepkg.StopWatchWorkers(workers)

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
	snapshot, err := pipelinepkg.SnapshotPaths(paths)
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
	sourceSnapshot, err := pipelinepkg.SnapshotPaths(sourcePaths)
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
				currentSource, err := pipelinepkg.SnapshotPaths(sourcePaths)
				if err != nil {
					return err
				}
				if !pipelinepkg.SnapshotsEqual(sourceSnapshot, currentSource) {
					changed := pipelinepkg.DiffSnapshotPaths(sourceSnapshot, currentSource)
					changed = FilterGeneratedProjectPaths(cfg.ProjectDir, changed)
					if len(changed) == 0 {
						sourceSnapshot = currentSource
						continue
					}
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
							sourceSnapshot = currentSource
							continue
						}
					}
					if stencilTouched {
						if err := e.syncStencilSourceMirror(cfg.WorkspaceDir); err != nil {
							fmt.Fprintln(os.Stderr, "[stack] dev stencil source sync failed:", err)
							sourceSnapshot = currentSource
							continue
						}
					}
					sourceSnapshot = currentSource
				}
			}
			current, err := pipelinepkg.SnapshotPaths(paths)
			if err != nil {
				return err
			}
			if !pipelinepkg.SnapshotsEqual(snapshot, current) {
				changed := pipelinepkg.DiffSnapshotPaths(snapshot, current)
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

func (e *BuildEngine) startWatchWorkers(ctx context.Context, cfg DevConfig, tailwindCacheRoot, stencilCacheRoot, outputDir string) ([]pipelinepkg.WatchWorker, error) {
	var workers []pipelinepkg.WatchWorker

	if e.builder != nil && e.builder.tailwind != nil && len(e.builder.tailwind.Inputs()) > 0 {
		inputPath := filepath.Join(tailwindCacheRoot, "tailwind.input.css")
		if strings.TrimSpace(cfg.ProjectDir) != "" {
			absProjectDir, err := filepath.Abs(cfg.ProjectDir)
			if err != nil {
				return nil, err
			}
			inputPath = filepath.Join(absProjectDir, "tailwind.input.css")
		}
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
		worker, err := pipelinepkg.StartCommandWatchSpec(ctx, "tailwind", spec)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(os.Stderr, "[stack] tailwind watch started input=%s output=%s\n", inputPath, outputPath)
		workers = append(workers, worker)
	}

	if e.builder != nil && e.builder.stencil != nil && len(e.builder.stencil.Inputs()) > 0 {
		spec, err := stencilpkg.DevCommand(stencilpkg.Config{ProjectDir: cfg.ProjectDir})
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Join(outputDir, "assets", "js", stencilBundleID), 0o755); err != nil {
			return nil, err
		}
		spec.WorkDir = stencilCacheRoot
		worker, err := pipelinepkg.StartRestartingCommandWatchSpec(ctx, "stencil", spec)
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
			sourcePaths, err := tailwindpkg.SourcePaths(source)
			if err != nil {
				continue
			}
			for _, path := range sourcePaths {
				path = strings.TrimSpace(path)
				if path == "" || IsGeneratedLocalPath(path) {
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
			sourcePaths, err := stencilpkg.SourcePaths(source)
			if err != nil {
				continue
			}
			for _, path := range sourcePaths {
				path = strings.TrimSpace(path)
				if path == "" || IsGeneratedLocalPath(path) {
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
	if e.builder.content != nil {
		for _, path := range e.builder.content.WatchPaths() {
			path = strings.TrimSpace(path)
			if path == "" || IsGeneratedLocalPath(path) {
				continue
			}
			if _, exists := seen[path]; exists {
				continue
			}
			seen[path] = struct{}{}
			paths = append(paths, path)
		}
	}
	if e.builder.layouts != nil {
		for _, path := range e.builder.layouts.WatchPaths() {
			path = strings.TrimSpace(path)
			if path == "" || IsGeneratedLocalPath(path) {
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

func (e *BuildEngine) contentModules(moduleRoot string) ([]HugoModule, error) {
	if e == nil || e.builder == nil || e.builder.content == nil {
		return nil, nil
	}
	mods, err := e.builder.content.Modules(moduleRoot)
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
	if e == nil || e.builder == nil || e.builder.layouts == nil {
		return nil, nil
	}
	mods, err := e.builder.layouts.Modules(moduleRoot)
	if err != nil {
		return nil, err
	}
	out := make([]HugoModule, 0, len(mods))
	for _, mod := range mods {
		out = append(out, HugoModule{ImportPath: mod.ImportPath, ReplacePath: mod.ReplacePath})
	}
	return out, nil
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

func (e *BuildEngine) materializeProjectFiles(projectDir, workspaceRoot string) error {
	if e == nil || e.builder == nil {
		return nil
	}
	projectDir = strings.TrimSpace(projectDir)
	if projectDir == "" {
		return nil
	}
	hasTailwind := e.builder.tailwind != nil && len(e.builder.tailwind.Inputs()) > 0
	hasStencil := e.builder.stencil != nil && len(e.builder.stencil.Inputs()) > 0
	if !hasTailwind && !hasStencil {
		return nil
	}
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(projectPackageSource(hasTailwind, hasStencil)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(projectDir, "package-lock.json"), []byte(projectLockSource(hasTailwind, hasStencil)), 0o644); err != nil {
		return err
	}
	if hasStencil {
		stencilCacheRoot := filepath.Join(filepath.Dir(strings.TrimSpace(workspaceRoot)), "stencil-cache")
		srcDir := filepath.Join(stencilCacheRoot, "src", "assets", "js")
		outDir := filepath.Join(stencilCacheRoot, "dist")
		if rel, err := filepath.Rel(projectDir, srcDir); err == nil && strings.TrimSpace(rel) != "" {
			srcDir = rel
		}
		if rel, err := filepath.Rel(projectDir, outDir); err == nil && strings.TrimSpace(rel) != "" {
			outDir = rel
		}
		if err := os.WriteFile(filepath.Join(projectDir, "stencil.config.ts"), []byte(stencilConfigSource(srcDir, outDir)), 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(projectDir, "tsconfig.json"), []byte(stencilTSConfigSource()), 0o644); err != nil {
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
		if e.builder.content != nil {
			for _, p := range e.builder.content.WatchPaths() {
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
	if e.builder.content != nil {
		for _, p := range e.builder.content.WatchPaths() {
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
	sort.Strings(paths)
	return paths
}

func FilterGeneratedProjectPaths(projectDir string, paths []string) []string {
	projectDir = strings.TrimSpace(projectDir)
	if projectDir == "" || len(paths) == 0 {
		return append([]string{}, paths...)
	}
	absProjectDir, err := filepath.Abs(projectDir)
	if err != nil {
		absProjectDir = projectDir
	}
	absProjectDir = filepath.Clean(absProjectDir)

	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if isGeneratedProjectFile(absProjectDir, path) || IsGeneratedLocalPath(path) {
			continue
		}
		out = append(out, path)
	}
	return out
}

func IsGeneratedLocalPath(path string) bool {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." {
		return false
	}
	needle := string(filepath.Separator) + ".stack" + string(filepath.Separator)
	if strings.Contains(path, needle) {
		return true
	}
	return filepath.Base(path) == ".stack"
}

func isGeneratedProjectFile(projectDir, path string) bool {
	projectDir = filepath.Clean(strings.TrimSpace(projectDir))
	path = filepath.Clean(strings.TrimSpace(path))
	if projectDir == "" || path == "" {
		return false
	}
	rel, err := filepath.Rel(projectDir, path)
	if err != nil {
		return false
	}
	rel = filepath.Clean(rel)
	if rel == "." {
		return false
	}
	if rel == "package.json" ||
		rel == "package-lock.json" ||
		rel == ".package-lock.json" ||
		rel == "tailwind.input.css" ||
		rel == "stencil.config.ts" ||
		rel == "tsconfig.json" {
		return true
	}
	if rel == "node_modules" || strings.HasPrefix(rel, "node_modules"+string(filepath.Separator)) {
		return true
	}
	return false
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
