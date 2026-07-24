package npm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cleanstartup/stack/plugin"
	"github.com/cleanstartup/stack/devwatch"
)

type Project struct {
	dependencies    map[string]string
	devDependencies map[string]string
	requiredBins    []string
}

type Capability struct {
	project *Project
}

func NewProject() *Project {
	return &Project{
		dependencies:    map[string]string{},
		devDependencies: map[string]string{},
	}
}

func NewCapability(project *Project) Capability {
	return Capability{project: project}
}

func (p *Project) AddDependency(name, version string) {
	if p == nil || strings.TrimSpace(name) == "" || strings.TrimSpace(version) == "" {
		return
	}
	p.dependencies[strings.TrimSpace(name)] = strings.TrimSpace(version)
}

func (p *Project) AddDevDependency(name, version string) {
	if p == nil || strings.TrimSpace(name) == "" || strings.TrimSpace(version) == "" {
		return
	}
	p.devDependencies[strings.TrimSpace(name)] = strings.TrimSpace(version)
}

func (p *Project) RequireBin(name string) {
	if p == nil || strings.TrimSpace(name) == "" {
		return
	}
	name = strings.TrimSpace(name)
	for _, existing := range p.requiredBins {
		if existing == name {
			return
		}
	}
	p.requiredBins = append(p.requiredBins, name)
}

func (p *Project) Empty() bool {
	if p == nil {
		return true
	}
	return len(p.dependencies) == 0 && len(p.devDependencies) == 0
}

func (p *Project) PackageJSON() string {
	data := map[string]any{
		"name":    "stack-target-workspace",
		"private": true,
		"version": "0.0.0",
	}
	if p != nil && len(p.devDependencies) > 0 {
		data["devDependencies"] = p.devDependencies
	}
	if p != nil && len(p.dependencies) > 0 {
		data["dependencies"] = p.dependencies
	}
	buf, _ := json.MarshalIndent(data, "", "  ")
	return string(buf) + "\n"
}

func (p *Project) PackageLockJSON() string {
	root := map[string]any{
		"name":    "stack-target-workspace",
		"version": "0.0.0",
	}
	if p != nil && len(p.devDependencies) > 0 {
		root["devDependencies"] = p.devDependencies
	}
	if p != nil && len(p.dependencies) > 0 {
		root["dependencies"] = p.dependencies
	}
	data := map[string]any{
		"name":            "stack-target-workspace",
		"lockfileVersion": 3,
		"requires":        true,
		"packages": map[string]any{
			"": root,
		},
		"version": "0.0.0",
	}
	merged := map[string]string{}
	if p != nil {
		for k, v := range p.devDependencies {
			merged[k] = v
		}
		for k, v := range p.dependencies {
			merged[k] = v
		}
	}
	if len(merged) > 0 {
		data["dependencies"] = merged
	}
	buf, _ := json.MarshalIndent(data, "", "  ")
	return string(buf) + "\n"
}

func (c Capability) Install(ctx context.Context, cfg plugin.Context) error {
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
	if err := os.WriteFile(filepath.Join(projectDir, "package.json"), []byte(project.PackageJSON()), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(projectDir, "package-lock.json"), []byte(project.PackageLockJSON()), 0o644); err != nil {
		return err
	}
	return EnsureDependencies(ctx, projectDir, project.requiredBins)
}

func (c Capability) Build(context.Context, plugin.Context) error { return nil }
func (c Capability) Dev(context.Context, plugin.Context) ([]devwatch.WatchWorker, error) {
	return nil, nil
}
func (c Capability) Register(plugin.Target) {}

func EnsureDependencies(ctx context.Context, projectDir string, requiredBins []string) error {
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
