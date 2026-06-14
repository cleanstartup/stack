package web

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cleanstartup/stack/internal/capability"
	npmpkg "github.com/cleanstartup/stack/internal/npm"
	pipelinepkg "github.com/cleanstartup/stack/internal/pipeline"
	stencilpkg "github.com/cleanstartup/stack/internal/stencil"
	tailwindpkg "github.com/cleanstartup/stack/internal/tailwind"
)

func (e *BuildEngine) capabilities() []capability.Capability {
	project := npmpkg.NewProject()
	var caps []capability.Capability

	if e != nil && e.builder != nil && e.builder.tailwind != nil && len(e.builder.tailwind.Inputs()) > 0 {
		tailwindpkg.AddNPMDependencies(project)
		caps = append(caps, tailwindpkg.NewCapability(e.builder.tailwind, tailwindConfig))
	}
	if e != nil && e.builder != nil && e.builder.stencil != nil && len(e.builder.stencil.Inputs()) > 0 {
		stencilpkg.AddNPMDependencies(project)
		caps = append(caps, stencilpkg.NewCapability(e.builder.stencil, stencilConfig))
	}
	if e != nil && e.builder != nil {
		for _, dep := range e.builder.npm {
			if dep.dev {
				project.AddDevDependency(dep.name, dep.version)
			} else {
				project.AddDependency(dep.name, dep.version)
			}
		}
	}
	if !project.Empty() {
		caps = append([]capability.Capability{npmpkg.NewCapability(project)}, caps...)
	}
	return caps
}

func (b *Builder) registrationCapabilities() []capability.Capability {
	if b == nil {
		return nil
	}
	var caps []capability.Capability
	if b.tailwind != nil && len(b.tailwind.Inputs()) > 0 {
		caps = append(caps, tailwindpkg.NewCapability(b.tailwind, nil))
	}
	if b.stencil != nil && len(b.stencil.Inputs()) > 0 {
		caps = append(caps, stencilpkg.NewCapability(b.stencil, nil))
	}
	return caps
}

func (e *BuildEngine) installCapabilities(ctx context.Context, cfg BuildConfig, workspace *Workspace) error {
	capabilityContext := e.capabilityContext(cfg, DevConfig{}, workspace)
	for _, cap := range e.capabilities() {
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
	capabilityContext := e.capabilityContext(cfg, DevConfig{}, workspace)
	for _, cap := range e.capabilities() {
		if cap == nil {
			continue
		}
		if err := cap.Build(ctx, capabilityContext); err != nil {
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
	for _, cap := range e.capabilities() {
		if cap == nil {
			continue
		}
		capabilityWorkers, err := cap.Dev(ctx, capabilityContext)
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
	for _, cap := range e.capabilities() {
		source, ok := cap.(capability.Source)
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
	for _, cap := range e.capabilities() {
		source, ok := cap.(capability.Source)
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

func (e *BuildEngine) capabilityContext(buildCfg BuildConfig, devCfg DevConfig, workspace *Workspace) capability.Context {
	outputDir := strings.TrimSpace(buildCfg.OutputDir)
	if outputDir == "" {
		outputDir = strings.TrimSpace(devCfg.OutputDir)
	}
	projectDir := strings.TrimSpace(buildCfg.ProjectDir)
	if projectDir == "" {
		projectDir = strings.TrimSpace(devCfg.ProjectDir)
	}
	return capability.Context{
		ProjectDir:  projectDir,
		Workspace:   workspace,
		OutputDir:   outputDir,
		BuildConfig: buildCfg,
		DevConfig:   devCfg,
	}
}

func tailwindConfig(ctx capability.Context) tailwindpkg.Config {
	cfg, _ := ctx.BuildConfig.(BuildConfig)
	return tailwindpkg.Config{
		Binary:       cfg.TailwindBinary,
		Version:      cfg.TailwindVersion,
		CacheDir:     cfg.TailwindCacheDir,
		DownloadBase: cfg.TailwindDownloadBase,
		ProjectDir:   ctx.ProjectDir,
	}
}

func stencilConfig(ctx capability.Context) stencilpkg.Config {
	cfg, _ := ctx.BuildConfig.(BuildConfig)
	return stencilpkg.Config{
		Binary:     cfg.StencilBinary,
		ProjectDir: ctx.ProjectDir,
	}
}
