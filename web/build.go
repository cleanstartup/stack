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
)

const (
	defaultWorkspaceDir = ".way2go/workspace"
	defaultOutputDir    = ".way2go/public"
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

func NewBuildEngine(mods ...Module) *BuildEngine {
	builder := NewBuilder()
	builder.Apply(mods...)
	return &BuildEngine{builder: builder}
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

	if err := os.RemoveAll(workspace.Root); err != nil {
		return nil, err
	}
	if workspace.Out != filepath.Join(workspace.Root, "out") {
		if err := os.RemoveAll(workspace.Out); err != nil {
			return nil, err
		}
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

	assets, err := collectAssets(workspace.Out)
	if err != nil {
		return nil, err
	}

	return &BuildResult{
		WorkspaceDir: workspace.Root,
		SourceDir:    workspace.Src,
		OutputDir:    workspace.Out,
		AssetsDir:    workspace.OutputAssetsRoot(),
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
		cfg.PollInterval = time.Second
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

	if _, err := e.Build(ctx, BuildConfig{
		WorkspaceDir: cfg.WorkspaceDir,
		OutputDir:    cfg.OutputDir,
	}); err != nil {
		return err
	}
	if cfg.DevState != nil {
		cfg.DevState.MarkBuilt()
	}

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

	paths := e.watchPaths()
	snapshot, err := snapshotPaths(paths)
	if err != nil {
		return err
	}

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
			current, err := snapshotPaths(paths)
			if err != nil {
				return err
			}
			if snapshotsEqual(snapshot, current) {
				continue
			}
			if _, err := e.Build(ctx, BuildConfig{
				WorkspaceDir: cfg.WorkspaceDir,
				OutputDir:    cfg.OutputDir,
			}); err != nil {
				fmt.Fprintln(os.Stderr, "web dev build failed:", err)
				continue
			}
			if cfg.DevState != nil {
				cfg.DevState.Broadcast()
			}
			snapshot = current
		}
	}
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

type fileSignature struct {
	Size    int64
	ModTime int64
}

func snapshotPaths(paths []string) (map[string]fileSignature, error) {
	snapshot := map[string]fileSignature{}
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
