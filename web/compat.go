package web

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const siteModuleImportPath = "github.com/cleanstartup/stack/site"
const siteAssetsModuleImportPath = "github.com/cleanstartup/stack/site-assets"

func Content(baseDir string, includes ...string) Part {
	return partFunc(func(app *WebApp) { app.RegisterContent(baseDir, includes...) })
}

func Layouts(baseDir string, includes ...string) Part {
	return partFunc(func(app *WebApp) { app.RegisterLayouts(baseDir, includes...) })
}

func SiteConfig(opts SiteOptions) Part {
	return partFunc(func(app *WebApp) { app.RegisterSiteConfig(opts) })
}

func siteConfigFiles(moduleRoot string, siteConfig *SiteOptions, modules ...HugoModule) ([]string, error) {
	moduleRoot = strings.TrimSpace(moduleRoot)
	if moduleRoot == "" {
		return nil, fmt.Errorf("site config paths are required")
	}
	files := make([]string, 0, 2)
	if siteConfig != nil {
		configPath, err := writeSiteConfig(moduleRoot, *siteConfig)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(configPath) != "" {
			files = append(files, configPath)
		}
	}
	moduleConfig, err := writeSiteModuleConfig(moduleRoot, modules...)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(moduleConfig) != "" {
		files = append(files, moduleConfig)
	}
	return files, nil
}

func writeSiteConfig(moduleRoot string, opts SiteOptions) (string, error) {
	if err := os.MkdirAll(moduleRoot, 0o755); err != nil {
		return "", err
	}
	configPath := filepath.Join(moduleRoot, "site.hugo.toml")
	content, err := siteConfigToml(opts)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(content) == "" {
		_ = os.Remove(configPath)
		return "", nil
	}
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		return "", err
	}
	return configPath, nil
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
	if strings.TrimSpace(content) == "" {
		_ = os.Remove(configPath)
		return "", nil
	}
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		return "", err
	}
	return configPath, nil
}

func siteHugoModule() HugoModule {
	return HugoModule{ImportPath: siteModuleImportPath, ReplacePath: siteModuleDir()}
}

func siteAssetHugoModule(assetModuleDir string) HugoModule {
	return HugoModule{ImportPath: siteAssetsModuleImportPath, ReplacePath: strings.TrimSpace(assetModuleDir)}
}

func hugoModuleConfig(modules ...HugoModule) (string, error) {
	normalized := normalizeHugoModules(modules...)
	if len(normalized) == 0 {
		return "", nil
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
	seen := map[string]struct{}{}
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

func templProxyURL(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		addr = ":8080"
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil || strings.TrimSpace(port) == "" {
		if strings.HasPrefix(addr, ":") {
			port = strings.TrimPrefix(addr, ":")
		} else {
			port = strings.TrimSpace(addr)
		}
	}
	host = strings.TrimSpace(host)
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	if strings.TrimSpace(port) == "" {
		port = "8080"
	}
	return "http://" + host + ":" + port
}

func devChildCommand(moduleDir, baseDir string, cfg DevConfig) string {
	moduleDir = strings.TrimSpace(moduleDir)
	baseDir = strings.TrimSpace(baseDir)
	target := "."
	if moduleDir != "" && baseDir != "" {
		if rel, err := filepath.Rel(moduleDir, baseDir); err == nil && strings.TrimSpace(rel) != "" {
			target = rel
		}
	}
	targetArg := "."
	if target != "." {
		targetArg = "./" + filepath.ToSlash(target)
	}
	addr := strings.TrimSpace(cfg.Addr)
	if addr == "" {
		addr = defaultAddr
	}
	workspace := strings.TrimSpace(cfg.WorkspaceDir)
	if workspace == "" {
		workspace = defaultWorkspaceDir
	}
	output := strings.TrimSpace(cfg.OutputDir)
	if output == "" {
		output = defaultOutputDir
	}
	poll := cfg.PollInterval.String()
	if cfg.PollInterval <= 0 {
		poll = (250 * time.Millisecond).String()
	}
	return strings.Join([]string{
		"go", "run", targetArg, "dev", "--child", "--addr=" + addr, "--workspace=" + workspace, "--output=" + output, "--poll=" + poll,
	}, " ")
}

func stencilPackageSource() string {
	return projectPackageSource(false, true)
}

func stencilConfigSource(srcDir, outDir string) string {
	srcDir = strings.TrimSpace(srcDir)
	if srcDir == "" {
		srcDir = "src/assets/js"
	}
	outDir = strings.TrimSpace(outDir)
	if outDir == "" {
		outDir = "dist"
	}
	return `import type { Config } from '@stencil/core';

export const config: Config = {
  namespace: 'stack',
  srcDir: '` + filepath.ToSlash(srcDir) + `',
  outputTargets: [
    {
      type: 'dist',
      dir: '` + filepath.ToSlash(outDir) + `',
      esmLoaderPath: '../loader',
    },
  ],
};
`
}

func stencilTSConfigSource() string {
	return `{
  "compilerOptions": {
    "allowSyntheticDefaultImports": true,
    "declaration": false,
    "experimentalDecorators": true,
    "jsx": "react",
    "jsxFactory": "h",
    "lib": ["dom", "dom.iterable", "es2020"],
    "module": "esnext",
    "moduleResolution": "bundler",
    "target": "es2020"
  },
  "include": ["src/assets/js"]
}
`
}

func tailwindPackageSource() string {
	return projectPackageSource(true, false)
}

func projectPackageSource(includeTailwind, includeStencil bool) string {
	data := map[string]any{
		"name":    "stack-target-workspace",
		"private": true,
		"version": "0.0.0",
	}
	devDependencies := map[string]string{}
	dependencies := map[string]string{}

	if includeTailwind {
		devDependencies["tailwindcss"] = "^4.0.0"
		devDependencies["@tailwindcss/cli"] = "^4.0.0"
	}
	if includeStencil {
		devDependencies["@stencil/core"] = "4.43.5"
		dependencies["altcha"] = "^3.0.2"
		dependencies["embla-carousel"] = "^8.6.0"
		dependencies["embla-carousel-auto-scroll"] = "^8.6.0"
		dependencies["htmx.org"] = "^2.0.10"
		dependencies["posthog-js"] = "^1.379.2"
	}
	if len(devDependencies) > 0 {
		data["devDependencies"] = devDependencies
	}
	if len(dependencies) > 0 {
		data["dependencies"] = dependencies
	}
	buf, _ := json.MarshalIndent(data, "", "  ")
	return string(buf) + "\n"
}

func projectLockSource(includeTailwind, includeStencil bool) string {
	root := map[string]any{
		"name":    "stack-target-workspace",
		"version": "0.0.0",
	}

	devDependencies := map[string]string{}
	dependencies := map[string]string{}

	if includeTailwind {
		devDependencies["tailwindcss"] = "^4.0.0"
		devDependencies["@tailwindcss/cli"] = "^4.0.0"
	}
	if includeStencil {
		devDependencies["@stencil/core"] = "4.43.5"
		dependencies["altcha"] = "^3.0.2"
		dependencies["embla-carousel"] = "^8.6.0"
		dependencies["embla-carousel-auto-scroll"] = "^8.6.0"
		dependencies["htmx.org"] = "^2.0.10"
		dependencies["posthog-js"] = "^1.379.2"
	}
	if len(devDependencies) > 0 {
		root["devDependencies"] = devDependencies
	}
	if len(dependencies) > 0 {
		root["dependencies"] = dependencies
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
	for k, v := range devDependencies {
		merged[k] = v
	}
	for k, v := range dependencies {
		merged[k] = v
	}
	if len(merged) > 0 {
		data["dependencies"] = merged
	}
	buf, _ := json.MarshalIndent(data, "", "  ")
	return string(buf) + "\n"
}
