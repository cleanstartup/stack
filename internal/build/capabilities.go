package build

import (
	"context"
	"fmt"
	"os"
	"strings"

	devwatchpkg "github.com/cleanstartup/stack/devwatch"
	npmpkg "github.com/cleanstartup/stack/internal/npm"
	stencilpkg "github.com/cleanstartup/stack/internal/stencil"
	tailwindpkg "github.com/cleanstartup/stack/internal/tailwind"
	"github.com/cleanstartup/stack/plugin"
	"github.com/cleanstartup/stack/web"
)

func (e *BuildEngine) capabilities(cfg BuildConfig) []plugin.Capability {
	if e == nil {
		return nil
	}
	project := e.npm
	var caps []plugin.Capability
	needsNPM := false

	if e.builder != nil && e.builder.Styles() != nil && len(e.builder.Styles().Inputs()) > 0 {
		needsNPM = true
		caps = append(caps, tailwindpkg.NewCapability(e.builder.Styles(), func(ctx plugin.Context) tailwindpkg.Config {
			return tailwindpkg.Config{
				Binary:       cfg.TailwindBinary,
				Version:      cfg.TailwindVersion,
				CacheDir:     cfg.TailwindCacheDir,
				DownloadBase: cfg.TailwindDownloadBase,
				ProjectDir:   ctx.ProjectDir,
				Bin:          ctx.Bin,
			}
		}))
	}
	if e.builder != nil && e.builder.Components() != nil && len(e.builder.Components().Inputs()) > 0 {
		needsNPM = true
		caps = append(caps, stencilpkg.NewCapability(e.builder.Components(), func(ctx plugin.Context) stencilpkg.Config {
			return stencilpkg.Config{
				Binary:     cfg.StencilBinary,
				ProjectDir: ctx.ProjectDir,
			}
		}))
	}
	if e.builder != nil {
		for _, dep := range e.builder.NPMDeps() {
			if dep.Dev {
				project.AddDevDependency(dep.Name, dep.Version)
			} else {
				project.AddDependency(dep.Name, dep.Version)
			}
		}
	}
	// npm.Capability.Install writes package.json, so it must run after
	// tailwind/stencil have had a chance to register their deps into the
	// shared project via ctx.NPM inside their own Install() — hence appended
	// last rather than prepended. project.Empty() alone isn't enough to
	// decide inclusion: at this point tailwind/stencil haven't run Install()
	// yet, so their deps aren't registered.
	if needsNPM || !project.Empty() {
		caps = append(caps, npmpkg.NewCapability(project))
	}
	return caps
}

func registrationCapabilities(b *web.Builder) []plugin.Capability {
	if b == nil {
		return nil
	}
	var caps []plugin.Capability
	if b.Styles() != nil && len(b.Styles().Inputs()) > 0 {
		caps = append(caps, tailwindpkg.NewCapability(b.Styles(), nil))
	}
	if b.Components() != nil && len(b.Components().Inputs()) > 0 {
		caps = append(caps, stencilpkg.NewCapability(b.Components(), nil))
	}
	return caps
}

func (e *BuildEngine) installCapabilities(ctx context.Context, cfg BuildConfig, workspace *Workspace) error {
	capabilityContext := e.capabilityContext(cfg, DevConfig{}, workspace, plugin.ModeBuild)
	for _, cap := range e.capabilities(cfg) {
		if cap == nil {
			continue
		}
		if err := cap.Install(ctx, capabilityContext); err != nil {
			return err
		}
	}
	return nil
}

func (e *BuildEngine) buildCapabilities(ctx context.Context, cfg BuildConfig, workspace *Workspace) error {
	capabilityContext := e.capabilityContext(cfg, DevConfig{}, workspace, plugin.ModeBuild)
	for _, cap := range e.capabilities(cfg) {
		if cap == nil {
			continue
		}
		if err := cap.Build(ctx, capabilityContext); err != nil {
			return err
		}
	}
	return nil
}

func (e *BuildEngine) startCapabilityDevWorkers(ctx context.Context, cfg DevConfig, workspace *Workspace) ([]devwatchpkg.WatchWorker, error) {
	buildCfg := BuildConfig{
		ProjectDir:   cfg.ProjectDir,
		WorkspaceDir: cfg.WorkspaceDir,
		OutputDir:    cfg.OutputDir,
	}
	capabilityContext := e.capabilityContext(buildCfg, cfg, workspace, plugin.ModeDev)
	var workers []devwatchpkg.WatchWorker
	for _, cap := range e.capabilities(buildCfg) {
		if cap == nil {
			continue
		}
		capabilityWorkers, err := cap.Dev(ctx, capabilityContext)
		if err != nil {
			devwatchpkg.StopWatchWorkers(workers)
			return nil, err
		}
		workers = append(workers, capabilityWorkers...)
	}
	return workers, nil
}

func (e *BuildEngine) rebuildChangedCapabilities(ctx context.Context, cfg DevConfig, workspace *Workspace, changed []string) {
	buildCfg := BuildConfig{
		ProjectDir:   cfg.ProjectDir,
		WorkspaceDir: cfg.WorkspaceDir,
		OutputDir:    cfg.OutputDir,
	}
	capabilityContext := e.capabilityContext(buildCfg, cfg, workspace, plugin.ModeDev)
	for _, cap := range e.capabilities(buildCfg) {
		source, ok := cap.(plugin.Source)
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
	for _, cap := range e.capabilities(BuildConfig{}) {
		source, ok := cap.(plugin.Source)
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

func (e *BuildEngine) capabilityContext(buildCfg BuildConfig, devCfg DevConfig, workspace *Workspace, mode plugin.Mode) plugin.Context {
	outputDir := strings.TrimSpace(buildCfg.OutputDir)
	if outputDir == "" {
		outputDir = strings.TrimSpace(devCfg.OutputDir)
	}
	projectDir := strings.TrimSpace(buildCfg.ProjectDir)
	if projectDir == "" {
		projectDir = strings.TrimSpace(devCfg.ProjectDir)
	}
	return plugin.Context{
		ProjectDir: projectDir,
		Workspace:  workspace,
		OutputDir:  outputDir,
		Mode:       mode,
		NPM:        e.npm,
		Bin:        e.bin,
	}
}

// keep registrationCapabilities accessible but suppress unused warning
var _ = registrationCapabilities
