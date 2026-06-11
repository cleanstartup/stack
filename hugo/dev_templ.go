package hugo

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cleanstartup/stack/web"
)

func (a *WebApp) devWithTempl(ctx context.Context, cfg DevConfig) error {
	if a == nil {
		return fmt.Errorf("build engine is nil")
	}
	if strings.EqualFold(strings.TrimSpace(a.target), "hugo") {
		return a.devSite(ctx, cfg)
	}
	moduleDir := strings.TrimSpace(a.moduleDir)
	if moduleDir == "" {
		moduleDir = moduleRoot(a.baseDir)
	}
	if moduleDir == "" {
		moduleDir = web.CallerDir(1)
	}
	if moduleDir == "" {
		moduleDir = "."
	}
	childCmd := devChildCommand(moduleDir, a.baseDir, cfg)
	proxyURL := templProxyURL(cfg.Addr)
	templArgs := []string{
		"run",
		"github.com/a-h/templ/cmd/templ@v0.3.943",
		"generate",
		"--watch",
		"--proxy=" + proxyURL,
		"--cmd=" + childCmd,
	}
	cmd := exec.CommandContext(ctx, "go", templArgs...)
	cmd.Dir = moduleDir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	fmt.Fprintf(os.Stderr, "[stack] templ dev supervisor start cmd=%s dir=%s proxy=%s\n", strings.Join(templArgs, " "), cmd.Dir, proxyURL)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("templ dev supervisor failed: %w", err)
	}
	return nil
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
		workspace = web.DefaultWorkspaceDir(baseDir)
	}
	output := strings.TrimSpace(cfg.OutputDir)
	if output == "" {
		output = web.DefaultOutputDir(baseDir)
	}
	poll := cfg.PollInterval.String()
	if cfg.PollInterval <= 0 {
		poll = (250 * time.Millisecond).String()
	}
	return strings.Join([]string{
		"go",
		"run",
		targetArg,
		"dev",
		"--child",
		"--addr=" + addr,
		"--workspace=" + workspace,
		"--output=" + output,
		"--poll=" + poll,
	}, " ")
}

func templProxyURL(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		addr = defaultAddr
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
		port = strings.TrimPrefix(defaultAddr, ":")
	}
	return "http://" + host + ":" + port
}
