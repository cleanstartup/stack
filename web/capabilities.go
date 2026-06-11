package web

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	pipelinepkg "github.com/cleanstartup/stack/pipeline"
	stencilpkg "github.com/cleanstartup/stack/stencil"
	tailwindpkg "github.com/cleanstartup/stack/tailwind"
)

type Capability interface {
	Install(context.Context, CapabilityContext) error
	Build(context.Context, CapabilityContext) error
	Dev(context.Context, CapabilityContext) ([]pipelinepkg.WatchWorker, error)
	Register(Target)
}

type CapabilityContext struct {
	ProjectDir  string
	Workspace   *Workspace
	OutputDir   string
	BuildConfig BuildConfig
	DevConfig   DevConfig
}

type Target interface {
	RegisterCSS(AssetRef)
	RegisterJS(AssetRef)
}

type nodeProject struct {
	dependencies    map[string]string
	devDependencies map[string]string
	requiredBins    []string
}

func newNodeProject() *nodeProject {
	return &nodeProject{
		dependencies:    map[string]string{},
		devDependencies: map[string]string{},
	}
}

func (p *nodeProject) AddDependency(name, version string) {
	if p == nil || strings.TrimSpace(name) == "" || strings.TrimSpace(version) == "" {
		return
	}
	p.dependencies[strings.TrimSpace(name)] = strings.TrimSpace(version)
}

func (p *nodeProject) AddDevDependency(name, version string) {
	if p == nil || strings.TrimSpace(name) == "" || strings.TrimSpace(version) == "" {
		return
	}
	p.devDependencies[strings.TrimSpace(name)] = strings.TrimSpace(version)
}

func (p *nodeProject) RequireBin(name string) {
	if p == nil || strings.TrimSpace(name) == "" {
		return
	}
	p.requiredBins = appendUniqueStrings(p.requiredBins, strings.TrimSpace(name))
}

func (p *nodeProject) Empty() bool {
	if p == nil {
		return true
	}
	return len(p.dependencies) == 0 && len(p.devDependencies) == 0
}

type npmCapability struct {
	project *nodeProject
}

type tailwindCapability struct {
	engine  *BuildEngine
	builder *Builder
}

type stencilCapability struct {
	engine  *BuildEngine
	builder *Builder
}

type sourceCapability interface {
	SourcePaths() []string
	SourceChanged(string) bool
	Rebuild(context.Context, CapabilityContext) error
}

func (c npmCapability) Install(ctx context.Context, cfg CapabilityContext) error {
	project := c.project
	if project == nil || project.Empty() {
		return nil
	}
	projectDir := strings.TrimSpace(cfg.ProjectDir)
	if projectDir == "" {
		return nil
	}
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(projectPackageSourceFromNodeProject(project)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(projectDir, "package-lock.json"), []byte(projectLockSourceFromNodeProject(project)), 0o644); err != nil {
		return err
	}
	return ensureNPMDependencies(ctx, projectDir, project.requiredBins)
}

func (c npmCapability) Build(context.Context, CapabilityContext) error { return nil }
func (c npmCapability) Dev(context.Context, CapabilityContext) ([]pipelinepkg.WatchWorker, error) {
	return nil, nil
}
func (c npmCapability) Register(Target) {}

func (c tailwindCapability) Install(ctx context.Context, cfg CapabilityContext) error {
	_ = ctx
	if c.engine == nil || c.engine.builder == nil || c.engine.builder.tailwind == nil || len(c.engine.builder.tailwind.Inputs()) == 0 {
		return nil
	}
	inputPath := ""
	if strings.TrimSpace(cfg.ProjectDir) != "" {
		inputPath = filepath.Join(cfg.ProjectDir, "tailwind.input.css")
	} else if cfg.Workspace != nil {
		inputPath = filepath.Join(cfg.Workspace.Root, "tailwind.input.css")
	}
	if strings.TrimSpace(inputPath) == "" {
		return nil
	}
	return c.engine.syncTailwindInput(cfg.Workspace, inputPath)
}

func (c tailwindCapability) Build(ctx context.Context, cfg CapabilityContext) error {
	if c.engine == nil || c.activeBuilder() == nil || c.activeBuilder().tailwind == nil || cfg.Workspace == nil {
		return nil
	}
	return c.engine.buildStyleBundle(ctx, cfg.Workspace, cfg.BuildConfig)
}

func (c tailwindCapability) Dev(ctx context.Context, cfg CapabilityContext) ([]pipelinepkg.WatchWorker, error) {
	builder := c.activeBuilder()
	if c.engine == nil || builder == nil || builder.tailwind == nil || len(builder.tailwind.Inputs()) == 0 || cfg.Workspace == nil {
		return nil, nil
	}
	inputPath, err := c.tailwindInputPath(cfg)
	if err != nil {
		return nil, err
	}
	if err := c.engine.syncTailwindInput(c.tailwindWorkspace(cfg), inputPath); err != nil {
		return nil, err
	}
	outputPath := tailwindOutputPath(cfg.OutputDir)
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
	return []pipelinepkg.WatchWorker{worker}, nil
}

func (c tailwindCapability) Register(target Target) {
	builder := c.activeBuilder()
	if target == nil || builder == nil || builder.tailwind == nil || len(builder.tailwind.Inputs()) == 0 {
		return
	}
	ref := builder.tailwind.BundleRef()
	target.RegisterCSS(AssetRef{Kind: AssetKind(ref.Kind), ID: ref.ID, Files: append([]string{}, ref.Files...)})
}

func (c tailwindCapability) SourcePaths() []string {
	builder := c.activeBuilder()
	if builder == nil || builder.tailwind == nil {
		return nil
	}
	return sourcePathsFromTailwind(builder.tailwind.Inputs())
}

func (c tailwindCapability) SourceChanged(path string) bool {
	if !isTailwindSourceFile(path) {
		return false
	}
	for _, candidate := range c.SourcePaths() {
		if pipelinepkg.SourcePathMatches(candidate, path) {
			return true
		}
	}
	return false
}

func (c tailwindCapability) Rebuild(ctx context.Context, cfg CapabilityContext) error {
	if c.engine == nil || cfg.Workspace == nil {
		return nil
	}
	return c.engine.rebuildTailwindBundle(ctx, cfg.DevConfig, c.tailwindWorkspace(cfg))
}

func (c stencilCapability) Install(ctx context.Context, cfg CapabilityContext) error {
	_ = ctx
	if c.engine == nil || c.engine.builder == nil || c.engine.builder.stencil == nil || len(c.engine.builder.stencil.Inputs()) == 0 {
		return nil
	}
	projectDir := strings.TrimSpace(cfg.ProjectDir)
	if projectDir == "" {
		return nil
	}
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return err
	}
	srcDir := c.engine.stencilSourceDir()
	if strings.TrimSpace(srcDir) == "" && cfg.Workspace != nil {
		srcDir = filepath.Join(strings.TrimSpace(cfg.Workspace.Root), "src", "assets", "js")
	}
	outputDir := strings.TrimSpace(cfg.OutputDir)
	if outputDir == "" {
		outputDir = DefaultOutputDir(projectDir)
	}
	outDir := filepath.Join(outputDir, "assets", "js")
	if rel, err := filepath.Rel(projectDir, srcDir); err == nil && strings.TrimSpace(rel) != "" {
		srcDir = rel
	}
	if rel, err := filepath.Rel(projectDir, outDir); err == nil && strings.TrimSpace(rel) != "" {
		outDir = rel
	}
	if err := os.WriteFile(filepath.Join(projectDir, "stencil.config.ts"), []byte(stencilConfigSource(srcDir, outDir)), 0o644); err != nil {
		return err
	}
	includes := []string{srcDir}
	if strings.TrimSpace(srcDir) == "" {
		includes = nil
	}
	return os.WriteFile(filepath.Join(projectDir, "tsconfig.json"), []byte(stencilTSConfigSource(includes...)), 0o644)
}

func (c stencilCapability) Build(ctx context.Context, cfg CapabilityContext) error {
	if c.engine == nil || c.activeBuilder() == nil || c.activeBuilder().stencil == nil || cfg.Workspace == nil {
		return nil
	}
	return c.engine.buildStencilBundle(ctx, cfg.Workspace, cfg.BuildConfig)
}

func (c stencilCapability) Dev(ctx context.Context, cfg CapabilityContext) ([]pipelinepkg.WatchWorker, error) {
	builder := c.activeBuilder()
	if builder == nil || builder.stencil == nil || len(builder.stencil.Inputs()) == 0 {
		return nil, nil
	}
	spec, err := stencilpkg.DevCommand(stencilpkg.Config{ProjectDir: cfg.ProjectDir})
	if err != nil {
		return nil, err
	}
	outputPath := filepath.Join(cfg.OutputDir, "assets", "js", stencilBundleID)
	if err := os.MkdirAll(outputPath, 0o755); err != nil {
		return nil, err
	}
	worker, err := pipelinepkg.StartRestartingCommandWatchSpec(ctx, "stencil", spec)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(os.Stderr, "[stack] stencil watch started output=%s\n", outputPath)
	return []pipelinepkg.WatchWorker{worker}, nil
}

func (c stencilCapability) Register(target Target) {
	builder := c.activeBuilder()
	if target == nil || builder == nil || builder.stencil == nil || len(builder.stencil.Inputs()) == 0 {
		return
	}
	ref := builder.stencil.BundleRef()
	target.RegisterJS(AssetRef{Kind: AssetKind(ref.Kind), ID: ref.ID, Files: append([]string{}, ref.Files...)})
}

func (c stencilCapability) SourcePaths() []string {
	builder := c.activeBuilder()
	if builder == nil || builder.stencil == nil {
		return nil
	}
	return sourcePathsFromStencil(builder.stencil.Inputs())
}

func (c stencilCapability) SourceChanged(path string) bool {
	if !isStencilSourceFile(path) {
		return false
	}
	for _, candidate := range c.SourcePaths() {
		if pipelinepkg.SourcePathMatches(candidate, path) {
			return true
		}
	}
	return false
}

func (c stencilCapability) Rebuild(context.Context, CapabilityContext) error {
	return nil
}

func (c tailwindCapability) activeBuilder() *Builder {
	if c.builder != nil {
		return c.builder
	}
	if c.engine != nil {
		return c.engine.builder
	}
	return nil
}

func (c stencilCapability) activeBuilder() *Builder {
	if c.builder != nil {
		return c.builder
	}
	if c.engine != nil {
		return c.engine.builder
	}
	return nil
}

func (c tailwindCapability) tailwindWorkspace(cfg CapabilityContext) *Workspace {
	if cfg.Workspace == nil {
		return nil
	}
	return newTailwindWorkspace(filepath.Join(filepath.Dir(cfg.Workspace.Root), "tailwind-cache"))
}

func (c tailwindCapability) tailwindInputPath(cfg CapabilityContext) (string, error) {
	if strings.TrimSpace(cfg.ProjectDir) == "" {
		if cfg.Workspace == nil {
			return "", fmt.Errorf("tailwind workspace is nil")
		}
		return filepath.Join(c.tailwindWorkspace(cfg).Root, "tailwind.input.css"), nil
	}
	absProjectDir, err := filepath.Abs(cfg.ProjectDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(absProjectDir, "tailwind.input.css"), nil
}

func (e *BuildEngine) capabilities() []Capability {
	project := newNodeProject()
	var caps []Capability

	if e != nil && e.builder != nil && e.builder.tailwind != nil && len(e.builder.tailwind.Inputs()) > 0 {
		project.AddDevDependency("tailwindcss", "^4.0.0")
		project.AddDevDependency("@tailwindcss/cli", "^4.0.0")
		project.RequireBin("tailwindcss")
		caps = append(caps, tailwindCapability{engine: e})
	}
	if e != nil && e.builder != nil && e.builder.stencil != nil && len(e.builder.stencil.Inputs()) > 0 {
		project.AddDevDependency("@stencil/core", "4.43.5")
		project.AddDependency("altcha", "^3.0.2")
		project.AddDependency("embla-carousel", "^8.6.0")
		project.AddDependency("embla-carousel-auto-scroll", "^8.6.0")
		project.AddDependency("htmx.org", "^2.0.10")
		project.AddDependency("posthog-js", "^1.379.2")
		project.RequireBin("stencil")
		caps = append(caps, stencilCapability{engine: e})
	}
	if !project.Empty() {
		caps = append([]Capability{npmCapability{project: project}}, caps...)
	}
	return caps
}

func (b *Builder) registrationCapabilities() []Capability {
	if b == nil {
		return nil
	}
	var caps []Capability
	if b.tailwind != nil && len(b.tailwind.Inputs()) > 0 {
		caps = append(caps, tailwindCapability{builder: b})
	}
	if b.stencil != nil && len(b.stencil.Inputs()) > 0 {
		caps = append(caps, stencilCapability{builder: b})
	}
	return caps
}

func (e *BuildEngine) installCapabilities(ctx context.Context, cfg BuildConfig, workspace *Workspace) error {
	capabilityContext := e.capabilityContext(cfg, DevConfig{}, workspace)
	for _, capability := range e.capabilities() {
		if capability == nil {
			continue
		}
		if err := capability.Install(ctx, capabilityContext); err != nil {
			return err
		}
	}
	return nil
}

func (e *BuildEngine) buildCapabilities(ctx context.Context, cfg BuildConfig, workspace *Workspace) error {
	capabilityContext := e.capabilityContext(cfg, DevConfig{}, workspace)
	for _, capability := range e.capabilities() {
		if capability == nil {
			continue
		}
		if err := capability.Build(ctx, capabilityContext); err != nil {
			return err
		}
	}
	return nil
}

func (e *BuildEngine) startCapabilityDevWorkers(ctx context.Context, cfg DevConfig, workspace *Workspace) ([]pipelinepkg.WatchWorker, error) {
	capabilityContext := e.capabilityContext(BuildConfig{
		ProjectDir:   cfg.ProjectDir,
		WorkspaceDir: cfg.WorkspaceDir,
		OutputDir:    cfg.OutputDir,
	}, cfg, workspace)
	var workers []pipelinepkg.WatchWorker
	for _, capability := range e.capabilities() {
		if capability == nil {
			continue
		}
		capabilityWorkers, err := capability.Dev(ctx, capabilityContext)
		if err != nil {
			pipelinepkg.StopWatchWorkers(workers)
			return nil, err
		}
		workers = append(workers, capabilityWorkers...)
	}
	return workers, nil
}

func (e *BuildEngine) rebuildChangedCapabilities(ctx context.Context, cfg DevConfig, workspace *Workspace, changed []string) {
	capabilityContext := e.capabilityContext(BuildConfig{
		ProjectDir:   cfg.ProjectDir,
		WorkspaceDir: cfg.WorkspaceDir,
		OutputDir:    cfg.OutputDir,
	}, cfg, workspace)
	for _, capability := range e.capabilities() {
		source, ok := capability.(sourceCapability)
		if !ok {
			continue
		}
		touched := false
		for _, path := range changed {
			if source.SourceChanged(path) {
				fmt.Fprintf(os.Stderr, "[stack] dev asset source touched: %s\n", path)
				touched = true
			}
		}
		if !touched {
			continue
		}
		if err := source.Rebuild(ctx, capabilityContext); err != nil {
			fmt.Fprintln(os.Stderr, "[stack] dev asset rebuild failed:", err)
		}
	}
}

func (e *BuildEngine) capabilitySourceWatchPaths() []string {
	seen := map[string]struct{}{}
	var paths []string
	for _, capability := range e.capabilities() {
		source, ok := capability.(sourceCapability)
		if !ok {
			continue
		}
		for _, path := range source.SourcePaths() {
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
	return paths
}

func (e *BuildEngine) capabilityContext(buildCfg BuildConfig, devCfg DevConfig, workspace *Workspace) CapabilityContext {
	outputDir := strings.TrimSpace(buildCfg.OutputDir)
	if outputDir == "" {
		outputDir = strings.TrimSpace(devCfg.OutputDir)
	}
	projectDir := strings.TrimSpace(buildCfg.ProjectDir)
	if projectDir == "" {
		projectDir = strings.TrimSpace(devCfg.ProjectDir)
	}
	return CapabilityContext{
		ProjectDir:  projectDir,
		Workspace:   workspace,
		OutputDir:   outputDir,
		BuildConfig: buildCfg,
		DevConfig:   devCfg,
	}
}

func ensureNPMDependencies(ctx context.Context, projectDir string, requiredBins []string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	projectDir = strings.TrimSpace(projectDir)
	if projectDir == "" {
		return nil
	}
	absProjectDir, err := filepath.Abs(projectDir)
	if err != nil {
		return err
	}
	if fileExists(filepath.Join(absProjectDir, "node_modules", ".package-lock.json")) {
		missing := false
		for _, bin := range requiredBins {
			if strings.TrimSpace(bin) == "" {
				continue
			}
			if !fileExists(filepath.Join(absProjectDir, "node_modules", ".bin", strings.TrimSpace(bin))) {
				missing = true
				break
			}
		}
		if !missing {
			return nil
		}
	}
	cmd := exec.CommandContext(ctx, "npm", "install", "--ignore-scripts", "--no-audit", "--no-fund")
	cmd.Dir = absProjectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("npm dependency install failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
