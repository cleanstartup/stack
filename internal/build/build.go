package build

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

	devwatchpkg "github.com/cleanstartup/stack/devwatch"
	npmpkg "github.com/cleanstartup/stack/internal/npm"
	stencilpkg "github.com/cleanstartup/stack/internal/stencil"
	tailwindpkg "github.com/cleanstartup/stack/internal/tailwind"
	"github.com/cleanstartup/stack/plugin"
	way2goweb "github.com/cleanstartup/stack/way2go/web"
	"github.com/cleanstartup/stack/webasset"
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
}

type ServeConfig struct {
	Addr      string
	OutputDir string
	AssetsFS  fs.FS
	AssetRoot string
	DevState  *way2goweb.DevState
}

type DevConfig struct {
	ProjectDir   string
	Addr         string
	WorkspaceDir string
	OutputDir    string
	AssetsFS     fs.FS
	AssetRoot    string
	PollInterval time.Duration
	DevState     *way2goweb.DevState
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
	app     *webasset.WebApp
	builder *webasset.Builder
	npm     *npmpkg.Project
	bin     plugin.BinProvider
}

func NewEngine(app *webasset.WebApp) *BuildEngine {
	return &BuildEngine{
		app:     app,
		builder: app.Builder(),
		npm:     npmpkg.NewProject(),
		bin:     plugin.NewBinProvider(plugin.DefaultBinCacheDir()),
	}
}

func (e *BuildEngine) Builder() *webasset.Builder {
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
	if err := e.Install(ctx, cfg); err != nil {
		return nil, err
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

	if err := os.RemoveAll(stagingOut); err != nil {
		return nil, err
	}
	if e.needsWorkspaceMaterialization() {
		if err := os.RemoveAll(workspace.Root); err != nil {
			return nil, err
		}
		if err := workspace.Prepare(); err != nil {
			return nil, err
		}
	}
	installCfg := cfg
	installCfg.OutputDir = workspace.Out
	if err := e.installCapabilities(ctx, installCfg, workspace); err != nil {
		return nil, err
	}
	if cfg.ProjectDir != "" {
		defer func() {
			publishWorkspace := *workspace
			publishWorkspace.Out = publishDir
			publishCfg := cfg
			publishCfg.OutputDir = publishDir
			_ = e.installCapabilities(ctx, publishCfg, &publishWorkspace)
		}()
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
	if info, err := os.Stat(workspace.Src); err == nil && info.IsDir() {
		if err := copyTreeExcept(workspace.Out, workspace.Src, func(rel string, entry fs.DirEntry) bool {
			rel = filepath.ToSlash(rel)
			tailwindPrefix := "tailwind/"
			return rel == "tailwind" || strings.HasPrefix(rel, tailwindPrefix)
		}); err != nil {
			return nil, err
		}
	}
	if err := e.buildCapabilities(ctx, cfg, workspace); err != nil {
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
	if err := e.Install(ctx, cfg); err != nil {
		return nil, err
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
	if e.needsWorkspaceMaterialization() {
		if err := os.RemoveAll(workspace.Root); err != nil {
			return nil, err
		}
		if err := workspace.Prepare(); err != nil {
			return nil, err
		}
	}
	installCfg := cfg
	installCfg.OutputDir = workspace.Out
	if err := e.installCapabilities(ctx, installCfg, workspace); err != nil {
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
	if err := e.buildCapabilities(ctx, cfg, workspace); err != nil {
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

func (e *BuildEngine) Install(ctx context.Context, cfg BuildConfig) error {
	if e == nil || e.builder == nil {
		return fmt.Errorf("build engine is nil")
	}
	if strings.TrimSpace(cfg.WorkspaceDir) == "" {
		cfg.WorkspaceDir = DefaultWorkspaceDir(cfg.ProjectDir)
	}
	if strings.TrimSpace(cfg.OutputDir) == "" {
		cfg.OutputDir = DefaultOutputDir(cfg.ProjectDir)
	}

	workspace, err := NewWorkspace(cfg.WorkspaceDir)
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.OutputDir) != "" {
		outputDir, err := filepath.Abs(cfg.OutputDir)
		if err == nil {
			workspace.Out = outputDir
		} else {
			workspace.Out = cfg.OutputDir
		}
	}
	if e.needsWorkspaceMaterialization() {
		if err := workspace.Prepare(); err != nil {
			return err
		}
	}
	if err := e.syncDirSources(cfg); err != nil {
		return err
	}
	installCfg := cfg
	installCfg.OutputDir = workspace.Out
	return e.installCapabilities(ctx, installCfg, workspace)
}

// syncDirSources copies each registered DirSource into
// .sources/<namespace>/<relPath>/ relative to the project directory.
func (e *BuildEngine) syncDirSources(cfg BuildConfig) error {
	if e == nil || e.builder == nil {
		return nil
	}
	entries := e.builder.DirSources()
	if len(entries) == 0 || strings.TrimSpace(cfg.ProjectDir) == "" {
		return nil
	}
	sourcesDir := filepath.Join(cfg.ProjectDir, ".sources")
	for _, entry := range entries {
		nsPart := strings.ReplaceAll(entry.Namespace, ".", string(filepath.Separator))
		dst := filepath.Join(sourcesDir, nsPart)
		rel := strings.TrimSpace(entry.RelPath)
		if rel != "" && rel != "." {
			dst = filepath.Join(dst, filepath.FromSlash(rel))
		}
		if err := os.MkdirAll(dst, 0o755); err != nil {
			return fmt.Errorf("sync sources %s: %w", entry.Namespace, err)
		}
		if _, err := webasset.CopyDir(dst, entry.AbsPath); err != nil {
			return fmt.Errorf("sync sources %s: %w", entry.Namespace, err)
		}
	}
	return nil
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

	registry := e.app.Handler()
	registry.SetDevState(cfg.DevState)

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
		cfg.DevState = way2goweb.NewDevState()
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

	workspace, err := NewWorkspace(cfg.WorkspaceDir)
	if err != nil {
		return err
	}
	workers, err := e.startCapabilityDevWorkers(watchCtx, cfg, workspace)
	if err != nil {
		return err
	}
	defer devwatchpkg.StopWatchWorkers(workers)

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

	paths := e.devOutputWatchPaths(cfg.OutputDir)
	fmt.Fprintf(os.Stderr, "[stack] dev watching %d roots\n", len(paths))
	for _, path := range paths {
		fmt.Fprintf(os.Stderr, "[stack]   watch %s\n", path)
	}
	snapshot, err := devwatchpkg.SnapshotPaths(paths)
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
	sourceSnapshot, err := devwatchpkg.SnapshotPaths(sourcePaths)
	if err != nil {
		return err
	}

	goSnapshot, err := devwatchpkg.SnapshotPaths([]string{cfg.ProjectDir})
	if err != nil {
		return err
	}

	dirty := false
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
			if cfg.ProjectDir != "" {
				currentGo, err := devwatchpkg.SnapshotPaths([]string{cfg.ProjectDir})
				if err != nil {
					return err
				}
				if !devwatchpkg.SnapshotsEqual(goSnapshot, currentGo) {
					changed := devwatchpkg.DiffSnapshotPaths(goSnapshot, currentGo)
					changed = filterGoSourcePaths(changed)
					if len(changed) > 0 {
						fmt.Fprintf(os.Stderr, "[stack] Go sources changed (%s) — restart dev to apply\n", strings.Join(changed, ", "))
					}
					goSnapshot = currentGo
				}
			}
			if len(sourcePaths) > 0 {
				currentSource, err := devwatchpkg.SnapshotPaths(sourcePaths)
				if err != nil {
					return err
				}
				if !devwatchpkg.SnapshotsEqual(sourceSnapshot, currentSource) {
					changed := devwatchpkg.DiffSnapshotPaths(sourceSnapshot, currentSource)
					changed = FilterGeneratedProjectPaths(cfg.ProjectDir, changed)
					if len(changed) == 0 {
						sourceSnapshot = currentSource
						continue
					}
					fmt.Fprintf(os.Stderr, "[stack] dev sources changed: %s\n", strings.Join(changed, ", "))
					e.rebuildChangedCapabilities(ctx, cfg, workspace, changed)
					sourceSnapshot = currentSource
				}
			}
			current, err := devwatchpkg.SnapshotPaths(paths)
			if err != nil {
				return err
			}
			if !devwatchpkg.SnapshotsEqual(snapshot, current) {
				changed := devwatchpkg.DiffSnapshotPaths(snapshot, current)
				fmt.Fprintf(os.Stderr, "[stack] dev output changed: %s\n", strings.Join(changed, ", "))
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
			fmt.Fprintf(os.Stderr, "[stack] dev output settled, broadcasting reload\n")
			if cfg.DevState != nil {
				cfg.DevState.Broadcast()
			}
			dirty = false
		}
	}
}

func (e *BuildEngine) DevAssets(ctx context.Context, cfg DevConfig) error {
	if e == nil || e.builder == nil {
		return fmt.Errorf("build engine is nil")
	}
	if cfg.DevState == nil {
		cfg.DevState = way2goweb.NewDevState()
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
	if _, err := e.BuildAssets(ctx, BuildConfig{
		ProjectDir:   cfg.ProjectDir,
		WorkspaceDir: cfg.WorkspaceDir,
		OutputDir:    cfg.OutputDir,
	}); err != nil {
		return err
	}
	cfg.DevState.MarkBuilt()

	watchCtx, watchCancel := context.WithCancel(ctx)
	defer watchCancel()

	workspace, err := NewWorkspace(cfg.WorkspaceDir)
	if err != nil {
		return err
	}
	workers, err := e.startCapabilityDevWorkers(watchCtx, cfg, workspace)
	if err != nil {
		return err
	}
	defer devwatchpkg.StopWatchWorkers(workers)

	paths := e.devOutputWatchPaths(cfg.OutputDir)
	fmt.Fprintf(os.Stderr, "[stack] dev watching %d roots\n", len(paths))
	for _, path := range paths {
		fmt.Fprintf(os.Stderr, "[stack]   watch %s\n", path)
	}
	snapshot, err := devwatchpkg.SnapshotPaths(paths)
	if err != nil {
		return err
	}
	sourcePaths := e.devSourceWatchPaths()
	if len(sourcePaths) > 0 {
		fmt.Fprintf(os.Stderr, "[stack] dev watching %d source roots\n", len(sourcePaths))
		for _, path := range sourcePaths {
			fmt.Fprintf(os.Stderr, "[stack]   source %s\n", path)
		}
	}
	sourceSnapshot, err := devwatchpkg.SnapshotPaths(sourcePaths)
	if err != nil {
		return err
	}
	dirty := false
	var lastChange time.Time

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if len(sourcePaths) > 0 {
				currentSource, err := devwatchpkg.SnapshotPaths(sourcePaths)
				if err != nil {
					return err
				}
				if !devwatchpkg.SnapshotsEqual(sourceSnapshot, currentSource) {
					changed := devwatchpkg.DiffSnapshotPaths(sourceSnapshot, currentSource)
					changed = FilterGeneratedProjectPaths(cfg.ProjectDir, changed)
					if len(changed) == 0 {
						sourceSnapshot = currentSource
						continue
					}
					fmt.Fprintf(os.Stderr, "[stack] dev sources changed: %s\n", strings.Join(changed, ", "))
					e.rebuildChangedCapabilities(ctx, cfg, workspace, changed)
					sourceSnapshot = currentSource
				}
			}
			current, err := devwatchpkg.SnapshotPaths(paths)
			if err != nil {
				return err
			}
			if !devwatchpkg.SnapshotsEqual(snapshot, current) {
				changed := devwatchpkg.DiffSnapshotPaths(snapshot, current)
				fmt.Fprintf(os.Stderr, "[stack] dev output changed: %s\n", strings.Join(changed, ", "))
				snapshot = current
				dirty = true
				lastChange = time.Now()
				continue
			}
			if !dirty || time.Since(lastChange) < cfg.PollInterval {
				continue
			}
			fmt.Fprintf(os.Stderr, "[stack] dev output settled\n")
			cfg.DevState.Broadcast()
			dirty = false
		}
	}
}

func (e *BuildEngine) devSourceWatchPaths() []string {
	if e == nil || e.builder == nil {
		return nil
	}
	seen := map[string]struct{}{}
	paths := e.capabilitySourceWatchPaths()
	for _, path := range paths {
		seen[path] = struct{}{}
	}
	sort.Strings(paths)
	return paths
}

func (e *BuildEngine) materializeAssets(workspace *Workspace) error {
	if e == nil || e.builder == nil || workspace == nil {
		return nil
	}
	for _, entry := range e.builder.Assets().Entries() {
		if entry.Source == nil {
			continue
		}
		if _, err := entry.Source.Materialize(workspace, entry.Kind); err != nil {
			return err
		}
	}
	return nil
}

func (e *BuildEngine) needsWorkspaceMaterialization() bool {
	if e == nil || e.builder == nil {
		return false
	}
	assets := e.builder.Assets()
	if assets != nil && len(assets.Entries()) > 0 {
		return true
	}
	if e.builder.Styles() != nil {
		for _, source := range e.builder.Styles().Inputs() {
			if source == nil {
				continue
			}
			if _, ok := source.(tailwindpkg.SourcePathsProvider); !ok {
				return true
			}
		}
	}
	if e.builder.Components() != nil {
		for _, source := range e.builder.Components().Inputs() {
			if source == nil {
				continue
			}
			if _, ok := source.(stencilpkg.SourcePathsProvider); !ok {
				return true
			}
		}
	}
	return false
}

func (e *BuildEngine) materializeProjectFiles(projectDir, workspaceRoot, outputDir string) error {
	workspace, err := NewWorkspace(workspaceRoot)
	if err != nil {
		return err
	}
	workspace.Out = outputDir
	return e.installCapabilities(context.Background(), BuildConfig{
		ProjectDir:   projectDir,
		WorkspaceDir: workspaceRoot,
		OutputDir:    outputDir,
	}, workspace)
}

func (e *BuildEngine) stencilSourceFiles() []string {
	if e == nil || e.builder == nil || e.builder.Components() == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var files []string
	for _, source := range e.builder.Components().Inputs() {
		paths, err := stencilpkg.SourcePaths(source)
		if err != nil {
			continue
		}
		for _, path := range paths {
			path = strings.TrimSpace(path)
			if path == "" {
				continue
			}
			path = filepath.Clean(path)
			if _, exists := seen[path]; exists {
				continue
			}
			seen[path] = struct{}{}
			files = append(files, path)
		}
	}
	sort.Strings(files)
	return files
}

func (e *BuildEngine) stencilSourceDir() string {
	files := e.stencilSourceFiles()
	if len(files) == 0 {
		return ""
	}
	common := filepath.Dir(files[0])
	for _, file := range files[1:] {
		dir := filepath.Dir(file)
		for {
			rel, err := filepath.Rel(common, dir)
			if err == nil && !strings.HasPrefix(rel, "..") {
				break
			}
			next := filepath.Dir(common)
			if next == common {
				return common
			}
			common = next
		}
	}
	return common
}

func (e *BuildEngine) watchPaths() []string {
	if e == nil || e.builder == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var paths []string
	if e.builder.Styles() != nil {
		for _, p := range e.builder.Styles().WatchPaths() {
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
	if e.builder.Components() != nil {
		for _, p := range e.builder.Components().WatchPaths() {
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
	assets := e.builder.Assets()
	if assets == nil {
		sort.Strings(paths)
		return paths
	}
	for _, entry := range assets.Entries() {
		if watcher, ok := entry.Source.(webasset.WatchPathsProvider); ok {
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

func filterGoSourcePaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if IsGeneratedLocalPath(p) {
			continue
		}
		skip := false
		for _, seg := range strings.Split(filepath.ToSlash(p), "/") {
			if seg == "vendor" || seg == "node_modules" {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		if filepath.Ext(p) == ".go" {
			out = append(out, filepath.Base(p))
		}
	}
	return out
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
