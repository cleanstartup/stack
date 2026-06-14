package build

import (
	"encoding/json"
	"net"
	"path/filepath"
	"strings"
	"time"

	npmpkg "github.com/cleanstartup/stack/internal/npm"
	stencilpkg "github.com/cleanstartup/stack/internal/stencil"
	tailwindpkg "github.com/cleanstartup/stack/internal/tailwind"
)

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
		workspace = DefaultWorkspaceDir(baseDir)
	}
	output := strings.TrimSpace(cfg.OutputDir)
	if output == "" {
		output = DefaultOutputDir(baseDir)
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

func stencilTSConfigSource(include ...string) string {
	if len(include) == 0 {
		include = []string{"src/assets/js"}
	}
	for idx, path := range include {
		include[idx] = filepath.ToSlash(strings.TrimSpace(path))
	}
	includeJSON, err := json.Marshal(include)
	if err != nil {
		includeJSON = []byte(`["src/assets/js"]`)
	}
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
  "include": ` + string(includeJSON) + `
}
`
}

func tailwindPackageSource() string {
	return projectPackageSource(true, false)
}

func projectPackageSource(includeTailwind, includeStencil bool) string {
	project := newNPMProject(includeTailwind, includeStencil)
	return project.PackageJSON()
}

func projectLockSource(includeTailwind, includeStencil bool) string {
	project := newNPMProject(includeTailwind, includeStencil)
	return project.PackageLockJSON()
}

func newNPMProject(includeTailwind, includeStencil bool) *npmpkg.Project {
	project := npmpkg.NewProject()
	if includeTailwind {
		tailwindpkg.AddNPMDependencies(project)
	}
	if includeStencil {
		stencilpkg.AddNPMDependencies(project)
	}
	return project
}
