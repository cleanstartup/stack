package web

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const defaultHugoVersion = "0.162.1"
const siteModuleImportPath = "github.com/cleanstartup/stack/site"
const siteAssetsModuleImportPath = "github.com/cleanstartup/stack/site-assets"

func (a *WebApp) buildSite(ctx context.Context, cfg BuildConfig) (*BuildResult, error) {
	if a == nil || a.engine == nil {
		return nil, fmt.Errorf("target is nil")
	}
	outputDir := strings.TrimSpace(cfg.OutputDir)
	if outputDir == "" {
		outputDir = defaultOutputDir
	}
	if abs, err := filepath.Abs(outputDir); err == nil {
		outputDir = abs
	}
	sourceDir := strings.TrimSpace(a.baseDir)
	if sourceDir == "" {
		sourceDir = strings.TrimSpace(a.moduleDir)
	}
	if sourceDir == "" {
		sourceDir = CallerDir(1)
	}
	if sourceDir == "" {
		sourceDir = "."
	}

	if err := os.RemoveAll(outputDir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, err
	}
	workspaceAbs, err := filepath.Abs(cfg.WorkspaceDir)
	if err != nil {
		workspaceAbs = cfg.WorkspaceDir
	}
	assetBuildDir := filepath.Join(filepath.Dir(workspaceAbs), "site-assets")
	assetModuleDir := filepath.Join(filepath.Dir(workspaceAbs), "site-assets-module")
	if _, err := a.engine.BuildAssets(ctx, BuildConfig{
		WorkspaceDir:         cfg.WorkspaceDir,
		OutputDir:            assetBuildDir,
		TailwindBinary:       cfg.TailwindBinary,
		TailwindVersion:      cfg.TailwindVersion,
		TailwindCacheDir:     cfg.TailwindCacheDir,
		TailwindDownloadBase: cfg.TailwindDownloadBase,
		StencilBinary:        cfg.StencilBinary,
	}); err != nil {
		return nil, err
	}
	if err := mirrorSiteAssets(assetBuildDir, filepath.Join(assetModuleDir, "static", "assets")); err != nil {
		return nil, err
	}
	showcaseModules, err := a.engine.showcaseModules(filepath.Dir(outputDir))
	if err != nil {
		return nil, err
	}
	modules := append(a.HugoModules(), siteAssetHugoModule(assetModuleDir))
	modules = append(modules, showcaseModules...)
	configFiles, err := siteConfigFiles(sourceDir, filepath.Dir(outputDir), modules...)
	if err != nil {
		return nil, err
	}
	if err := runHugoBuild(ctx, sourceDir, outputDir, cfg, configFiles...); err != nil {
		return nil, err
	}
	result := &BuildResult{
		WorkspaceDir: cfg.WorkspaceDir,
		SourceDir:    sourceDir,
		OutputDir:    outputDir,
		AssetsDir:    filepath.Join(outputDir, "assets"),
	}
	if assets, err := collectAssets(outputDir); err == nil {
		result.Assets = assets
	}
	return result, nil
}

func (a *WebApp) devSite(ctx context.Context, cfg DevConfig) error {
	if a == nil || a.engine == nil {
		return fmt.Errorf("target is nil")
	}
	if cfg.DevState == nil {
		cfg.DevState = NewDevState()
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 250 * time.Millisecond
	}
	if strings.TrimSpace(cfg.OutputDir) == "" {
		cfg.OutputDir = defaultOutputDir
	}
	if strings.TrimSpace(cfg.WorkspaceDir) == "" {
		cfg.WorkspaceDir = defaultWorkspaceDir
	}
	if strings.TrimSpace(cfg.Addr) == "" {
		cfg.Addr = defaultAddr
	}
	if abs, err := filepath.Abs(cfg.OutputDir); err == nil {
		cfg.OutputDir = abs
	}

	buildCfg := BuildConfig{
		WorkspaceDir:         cfg.WorkspaceDir,
		OutputDir:            cfg.OutputDir,
		TailwindBinary:       "",
		TailwindVersion:      "",
		TailwindCacheDir:     "",
		TailwindDownloadBase: "",
		StencilBinary:        "",
		HugoBinary:           "",
		HugoVersion:          "",
		HugoCacheDir:         "",
	}
	if _, err := a.engine.BuildAssets(ctx, buildCfg); err != nil {
		return err
	}
	if cfg.DevState != nil {
		cfg.DevState.MarkBuilt()
	}

	workspaceAbs, err := filepath.Abs(cfg.WorkspaceDir)
	if err != nil {
		workspaceAbs = cfg.WorkspaceDir
	}
	tailwindCacheRoot := filepath.Join(filepath.Dir(workspaceAbs), "tailwind-cache")
	stencilCacheRoot := filepath.Join(filepath.Dir(workspaceAbs), "stencil-cache")
	assetBuildDir := filepath.Join(filepath.Dir(workspaceAbs), "site-assets")
	assetModuleDir := filepath.Join(filepath.Dir(workspaceAbs), "site-assets-module")
	sourceDir := strings.TrimSpace(a.baseDir)
	if sourceDir == "" {
		sourceDir = strings.TrimSpace(a.moduleDir)
	}
	if sourceDir == "" {
		sourceDir = CallerDir(1)
	}
	if sourceDir == "" {
		sourceDir = "."
	}
	sourceDirAbs, err := filepath.Abs(sourceDir)
	if err == nil {
		sourceDir = sourceDirAbs
	}
	assetMirrorDir := filepath.Join(assetModuleDir, "static", "assets")
	showcaseModules, err := a.engine.showcaseModules(filepath.Dir(assetModuleDir))
	if err != nil {
		return err
	}
	modules := append(a.HugoModules(), siteAssetHugoModule(assetModuleDir))
	modules = append(modules, showcaseModules...)
	configFiles, err := siteConfigFiles(sourceDir, filepath.Dir(assetModuleDir), modules...)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(assetBuildDir, 0o755); err != nil {
		return err
	}
	if _, err := a.engine.BuildAssets(ctx, BuildConfig{
		WorkspaceDir:         cfg.WorkspaceDir,
		OutputDir:            assetBuildDir,
		TailwindBinary:       "",
		TailwindVersion:      "",
		TailwindCacheDir:     "",
		TailwindDownloadBase: "",
		StencilBinary:        "",
		HugoBinary:           "",
		HugoVersion:          "",
		HugoCacheDir:         "",
	}); err != nil {
		return err
	}
	if err := mirrorSiteAssets(assetBuildDir, assetMirrorDir); err != nil {
		return err
	}
	assetDevCfg := DevConfig{
		Addr:         cfg.Addr,
		WorkspaceDir: cfg.WorkspaceDir,
		OutputDir:    assetBuildDir,
		PollInterval: cfg.PollInterval,
		DevState:     cfg.DevState,
	}
	mirrorCtx, mirrorCancel := context.WithCancel(ctx)
	defer mirrorCancel()
	mirrorDone := make(chan error, 1)
	go func() {
		mirrorDone <- watchSiteAssetMirror(mirrorCtx, assetBuildDir, assetMirrorDir, cfg.PollInterval)
	}()
	sourcePaths := a.engine.devSourceWatchPaths()
	if len(sourcePaths) > 0 {
		fmt.Fprintf(os.Stderr, "[stack] site dev mirroring %d source roots\n", len(sourcePaths))
		for _, path := range sourcePaths {
			fmt.Fprintf(os.Stderr, "[stack]   mirror %s\n", path)
		}
		sourceSnapshot, err := snapshotPaths(sourcePaths)
		if err != nil {
			mirrorCancel()
			select {
			case <-mirrorDone:
			default:
			}
			return err
		}
		go func() {
			ticker := time.NewTicker(cfg.PollInterval)
			defer ticker.Stop()
			for {
				select {
				case <-mirrorCtx.Done():
					return
				case <-ticker.C:
					currentSource, err := snapshotPaths(sourcePaths)
					if err != nil {
						fmt.Fprintln(os.Stderr, "[stack] site source snapshot failed:", err)
						continue
					}
					if snapshotsEqual(sourceSnapshot, currentSource) {
						continue
					}
					changed := diffSnapshotPaths(sourceSnapshot, currentSource)
					fmt.Fprintf(os.Stderr, "[stack] site sources changed: %s\n", strings.Join(changed, ", "))
					tailwindTouched := false
					stencilTouched := false
					showcaseTouched := false
					for _, path := range changed {
						if a.engine.devTailwindSourceChanged(path) {
							fmt.Fprintf(os.Stderr, "[stack] site tailwind source touched: %s\n", path)
							tailwindTouched = true
						}
						if a.engine.devStencilSourceChanged(path) {
							fmt.Fprintf(os.Stderr, "[stack] site stencil source touched: %s\n", path)
							stencilTouched = true
						}
						if a.engine.devShowcaseSourceChanged(path) {
							fmt.Fprintf(os.Stderr, "[stack] site showcase source touched: %s\n", path)
							showcaseTouched = true
						}
					}
					if tailwindTouched {
						if err := a.engine.rebuildTailwindBundle(ctx, assetDevCfg, newTailwindWorkspace(tailwindCacheRoot)); err != nil {
							fmt.Fprintln(os.Stderr, "[stack] site tailwind rebuild failed:", err)
							continue
						}
					}
					if stencilTouched {
						if err := a.engine.syncStencilSourceMirror(cfg.WorkspaceDir); err != nil {
							fmt.Fprintln(os.Stderr, "[stack] site stencil source sync failed:", err)
							continue
						}
					}
					if showcaseTouched {
						if _, err := a.engine.showcaseModules(filepath.Dir(assetModuleDir)); err != nil {
							fmt.Fprintln(os.Stderr, "[stack] site showcase sync failed:", err)
							continue
						}
					}
					sourceSnapshot = currentSource
				}
			}
		}()
	}

	workers, err := a.engine.startWatchWorkers(ctx, tailwindCacheRoot, stencilCacheRoot, assetBuildDir)
	if err != nil {
		mirrorCancel()
		select {
		case <-mirrorDone:
		default:
		}
		return err
	}
	outputPaths := a.engine.devOutputWatchPaths(assetBuildDir, stencilCacheRoot)
	if len(outputPaths) > 0 {
		fmt.Fprintf(os.Stderr, "[stack] site dev watching %d outputs\n", len(outputPaths))
		for _, path := range outputPaths {
			fmt.Fprintf(os.Stderr, "[stack]   output %s\n", path)
		}
		outputSnapshot, err := snapshotPaths(outputPaths)
		if err != nil {
			mirrorCancel()
			select {
			case <-mirrorDone:
			default:
			}
			return err
		}
		go func() {
			ticker := time.NewTicker(cfg.PollInterval)
			defer ticker.Stop()
			var dirty bool
			var stencilDirty bool
			var lastChange time.Time
			for {
				select {
				case <-mirrorCtx.Done():
					return
				case <-ticker.C:
					current, err := snapshotPaths(outputPaths)
					if err != nil {
						fmt.Fprintln(os.Stderr, "[stack] site output snapshot failed:", err)
						continue
					}
					if !snapshotsEqual(outputSnapshot, current) {
						changed := diffSnapshotPaths(outputSnapshot, current)
						fmt.Fprintf(os.Stderr, "[stack] site outputs changed: %s\n", strings.Join(changed, ", "))
						for _, path := range changed {
							if strings.HasPrefix(path, filepath.Join(stencilCacheRoot, "dist")) {
								stencilDirty = true
								break
							}
						}
						outputSnapshot = current
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
						if err := a.engine.syncDevOutputs(assetBuildDir, stencilCacheRoot); err != nil {
							fmt.Fprintln(os.Stderr, "[stack] site output sync failed:", err)
							continue
						}
					}
					if err := mirrorSiteAssets(assetBuildDir, assetMirrorDir); err != nil {
						fmt.Fprintln(os.Stderr, "[stack] site asset mirror failed:", err)
						continue
					}
					dirty = false
					stencilDirty = false
				}
			}
		}()
	}
	defer stopWatchWorkers(workers)
	defer func() {
		mirrorCancel()
		select {
		case <-mirrorDone:
		default:
		}
	}()

	host, port := splitAddr(cfg.Addr)
	args := []string{
		"server",
		"--bind", host,
		"--port", port,
		"--source", sourceDir,
		"--destination", cfg.OutputDir,
		"--noHTTPCache",
		"--disableFastRender",
	}
	if len(configFiles) > 0 {
		args = append(args, "--config", strings.Join(configFiles, ","))
	}
	if baseURL := baseURLFromAddr(host, port); baseURL != "" {
		args = append(args, "--baseURL", baseURL)
	}

	binary, err := resolveHugoBinary(ctx, BuildConfig{})
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = sourceDir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	fmt.Fprintf(os.Stderr, "[stack] hugo dev server start cmd=%s dir=%s\n", strings.Join(args, " "), cmd.Dir)
	return cmd.Run()
}

func watchSiteAssetMirror(ctx context.Context, sourceDir, mirrorDir string, poll time.Duration) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if poll <= 0 {
		poll = 250 * time.Millisecond
	}
	sourceAssetsDir := filepath.Join(sourceDir, "assets")
	snapshot, err := snapshotPaths([]string{sourceAssetsDir})
	if err != nil {
		return err
	}
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			current, err := snapshotPaths([]string{sourceAssetsDir})
			if err != nil {
				return err
			}
			if snapshotsEqual(snapshot, current) {
				continue
			}
			if err := mirrorSiteAssets(sourceDir, mirrorDir); err != nil {
				fmt.Fprintln(os.Stderr, "[stack] site asset mirror failed:", err)
				continue
			}
			snapshot = current
		}
	}
}

func mirrorSiteAssets(sourceDir, mirrorDir string) error {
	sourceAssetsDir := filepath.Join(sourceDir, "assets")
	if _, err := os.Stat(sourceAssetsDir); err != nil {
		return err
	}
	if err := os.RemoveAll(mirrorDir); err != nil {
		return err
	}
	if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
		return err
	}
	return copyTree(mirrorDir, sourceAssetsDir)
}

func runHugoBuild(ctx context.Context, sourceDir, outputDir string, cfg BuildConfig, configFiles ...string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	binary, err := resolveHugoBinary(ctx, cfg)
	if err != nil {
		return err
	}
	args := []string{
		"--destination", outputDir,
	}
	if len(configFiles) > 0 {
		args = append(args, "--config", strings.Join(configFiles, ","))
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = sourceDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("hugo build failed via %s: %w: %s", binary, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func resolveHugoBinary(ctx context.Context, cfg BuildConfig) (string, error) {
	if path := strings.TrimSpace(cfg.HugoBinary); path != "" {
		return path, nil
	}
	if path := strings.TrimSpace(os.Getenv("STACK_HUGO_BINARY")); path != "" {
		return path, nil
	}

	cacheDir := strings.TrimSpace(cfg.HugoCacheDir)
	if cacheDir == "" {
		cacheDir = strings.TrimSpace(os.Getenv("STACK_HUGO_CACHE_DIR"))
	}
	if cacheDir == "" {
		if userCacheDir, err := os.UserCacheDir(); err == nil && strings.TrimSpace(userCacheDir) != "" {
			cacheDir = filepath.Join(userCacheDir, "stack", "hugo")
		} else {
			cacheDir = filepath.Join(os.TempDir(), "stack", "hugo")
		}
	}

	version := strings.TrimSpace(cfg.HugoVersion)
	if version == "" {
		version = strings.TrimSpace(os.Getenv("STACK_HUGO_VERSION"))
	}
	if version == "" {
		version = defaultHugoVersion
	}
	moduleVersion := hugoModuleVersion(version)

	binDir := filepath.Join(cacheDir, moduleVersion, "bin")
	binaryName := "hugo"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(binDir, binaryName)
	if info, err := os.Stat(binaryPath); err == nil && !info.IsDir() {
		return binaryPath, nil
	}

	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", err
	}
	pkg := "github.com/gohugoio/hugo@" + moduleVersion
	if version == defaultHugoVersion {
		fmt.Fprintf(os.Stderr, "[stack] installing pinned hugo version %s via go pkg=%s bin=%s\n", moduleVersion, pkg, binDir)
	} else {
		fmt.Fprintf(os.Stderr, "[stack] installing hugo via go pkg=%s bin=%s\n", pkg, binDir)
	}
	cmd := exec.CommandContext(ctx, "go", "install", pkg)
	cmd.Env = append(os.Environ(), "GOBIN="+binDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("hugo install failed via go install: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if info, err := os.Stat(binaryPath); err == nil && !info.IsDir() {
		return binaryPath, nil
	}
	return "", fmt.Errorf("hugo install completed but binary was not found at %s", binaryPath)
}

func siteConfigFiles(sourceDir, moduleRoot string, modules ...HugoModule) ([]string, error) {
	sourceDir = strings.TrimSpace(sourceDir)
	moduleRoot = strings.TrimSpace(moduleRoot)
	if sourceDir == "" || moduleRoot == "" {
		return nil, fmt.Errorf("site config paths are required")
	}

	files := make([]string, 0, 2)
	consumerConfig := filepath.Join(sourceDir, "hugo.toml")
	if info, err := os.Stat(consumerConfig); err == nil && !info.IsDir() {
		files = append(files, consumerConfig)
	}

	moduleConfig, err := writeSiteModuleConfig(moduleRoot, modules...)
	if err != nil {
		return nil, err
	}
	files = append(files, moduleConfig)
	return files, nil
}

func writeSiteModuleConfig(moduleRoot string, modules ...HugoModule) (string, error) {
	if err := os.MkdirAll(moduleRoot, 0o755); err != nil {
		return "", err
	}
	configPath := filepath.Join(moduleRoot, "site.module.hugo.toml")
	content, err := hugoModuleConfig(modules...)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		return "", err
	}
	return configPath, nil
}

func siteHugoModule() HugoModule {
	return HugoModule{
		ImportPath:  siteModuleImportPath,
		ReplacePath: siteModuleDir(),
	}
}

func siteAssetHugoModule(assetModuleDir string) HugoModule {
	return HugoModule{
		ImportPath:  siteAssetsModuleImportPath,
		ReplacePath: strings.TrimSpace(assetModuleDir),
	}
}

func hugoModuleConfig(modules ...HugoModule) (string, error) {
	normalized := normalizeHugoModules(modules...)
	if len(normalized) == 0 {
		return "", fmt.Errorf("no hugo modules configured")
	}

	var b strings.Builder
	b.WriteString("[module]\n")
	if replacements := hugoModuleReplacements(normalized...); replacements != "" {
		b.WriteString("replacements = \"")
		b.WriteString(replacements)
		b.WriteString("\"\n")
	}
	for _, mod := range normalized {
		b.WriteString("  [[module.imports]]\n")
		b.WriteString("  path = \"")
		b.WriteString(mod.ImportPath)
		b.WriteString("\"\n")
	}
	return b.String(), nil
}

func normalizeHugoModules(modules ...HugoModule) []HugoModule {
	if len(modules) == 0 {
		return nil
	}
	out := make([]HugoModule, 0, len(modules))
	seen := make(map[string]struct{}, len(modules))
	for _, mod := range modules {
		mod.ImportPath = strings.TrimSpace(mod.ImportPath)
		mod.ReplacePath = strings.TrimSpace(mod.ReplacePath)
		if mod.ImportPath == "" {
			continue
		}
		if _, ok := seen[mod.ImportPath]; ok {
			continue
		}
		out = append(out, mod)
		seen[mod.ImportPath] = struct{}{}
	}
	return out
}

func hugoModuleReplacements(modules ...HugoModule) string {
	parts := make([]string, 0, len(modules))
	for _, mod := range modules {
		if strings.TrimSpace(mod.ReplacePath) == "" {
			continue
		}
		parts = append(parts, mod.ImportPath+" -> "+filepath.ToSlash(mod.ReplacePath))
	}
	return strings.Join(parts, ",")
}

func siteModuleDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	return filepath.Join(filepath.Dir(filepath.Dir(file)), "site")
}

func hugoModuleVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return "latest"
	}
	if version == "latest" || version == "master" {
		return version
	}
	if version[0] == 'v' {
		return version
	}
	if version[0] >= '0' && version[0] <= '9' {
		return "v" + version
	}
	return version
}

func splitAddr(addr string) (string, string) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		addr = defaultAddr
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil || strings.TrimSpace(port) == "" {
		if strings.HasPrefix(addr, ":") {
			port = strings.TrimPrefix(addr, ":")
		} else {
			port = addr
		}
	}
	host = strings.TrimSpace(host)
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	if strings.TrimSpace(port) == "" {
		port = strings.TrimPrefix(defaultAddr, ":")
	}
	return host, port
}

func baseURLFromAddr(host, port string) string {
	host = strings.TrimSpace(host)
	port = strings.TrimSpace(port)
	if host == "" {
		host = "127.0.0.1"
	}
	if port == "" {
		port = strings.TrimPrefix(defaultAddr, ":")
	}
	return "http://" + host + ":" + port + "/"
}
