// Package hugo is stack's hugo Producer (CUP-22): a real content-build
// producer that runs the actual hugo binary. It intentionally depends only
// on github.com/cleanstartup/stack/plugin (+devwatch, +stdlib) — never the
// root stack package — as the proof named in the PRD (§5) that the
// Asset/Contribution contract (CUP-26) is sufficient on a green-field
// module that only sees stack/plugin, with no back-door into stack core.
//
// Unlike tailwind/lit (D-N: app-level singleton stages, not composed args),
// hugo stays a genuine producer with args (D-L: "hugo ist Plugin (Producer,
// emittiert text/html-Tree)") — Hugo(siteDir) is a free constructor, usable
// bare or via Mount(Hugo(siteDir), "/docs").
package hugo

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cleanstartup/stack/devwatch"
	pluginpkg "github.com/cleanstartup/stack/plugin"
)

// Producer is the plugin.Builder/plugin.Dever-conformant hugo site build.
// SiteDir is a hugo project root (content/, layouts/, static/, and a config
// file all live under it) — passed to the real hugo binary as --source,
// mirroring exactly what `hugo` itself expects on the command line.
type Producer struct {
	SiteDir    string // hugo project root, passed as --source
	ConfigFile string // optional: --config passthrough (else hugo's own discovery inside SiteDir)
	BaseURL    string // optional: --baseURL passthrough
	Minify     bool   // optional: --minify on Build (never on Dev; see Dev's doc comment)
	Binary     string // optional: explicit hugo binary path (tests / override)
	Version    string // optional: pin a version instead of "latest"
}

var (
	_ pluginpkg.Builder = (*Producer)(nil)
	_ pluginpkg.Dever   = (*Producer)(nil)
)

// Hugo builds a Producer over the given hugo project root. Mirrors
// tailwind.NewProducer's free-constructor shape (see internal/tailwind/
// producer.go) — the concrete proof of D-M's `Mount(Hugo(), "/docs")`.
func Hugo(siteDir string) *Producer {
	return &Producer{SiteDir: siteDir}
}

// Build implements plugin.Builder: runs the real hugo binary once and
// returns its output tree as a single self-contained text/html Asset (the
// Asset.Path contract explicitly allows "a self-contained sub-tree dir",
// which is exactly hugo's own nature — D-L). Returns nil, nil if SiteDir is
// unset — no site configured means the stage is a no-op, not an error (same
// convention as tailwind.Producer.Build's zero-scan-paths case).
func (p *Producer) Build(ctx context.Context, stageCtx pluginpkg.StageContext) ([]pluginpkg.Asset, error) {
	if p == nil || strings.TrimSpace(p.SiteDir) == "" {
		return nil, nil
	}
	if strings.TrimSpace(stageCtx.OutputDir) == "" {
		return nil, fmt.Errorf("hugo: stageCtx.OutputDir is required to build site %q", p.SiteDir)
	}

	absSite, err := filepath.Abs(p.SiteDir)
	if err != nil {
		return nil, err
	}
	outDir := p.outputDir(stageCtx, absSite)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}

	binaryPath, err := ResolveBinary(ctx, p.config(stageCtx))
	if err != nil {
		return nil, err
	}

	args := p.baseArgs(absSite, outDir)
	if p.Minify {
		args = append(args, "--minify")
	}

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("hugo build failed via %s: %w: %s", binaryPath, err, strings.TrimSpace(string(output)))
	}

	return []pluginpkg.Asset{{Path: outDir, ContentType: "text/html"}}, nil
}

// Dev implements plugin.Dever: hugo's own watch/rebuild, surfaced as a
// single devwatch.WatchWorker.
//
// Deliberately `hugo --watch`, not `hugo server` — the produced tree is
// served by whatever target mounted it (WebApp/Website's own registrar),
// not by hugo's own embedded HTTP server; running `hugo server` as well
// would open a second, redundant port and duplicate the composed target's
// serving responsibility (D-L: the target kind owns consumption/serving,
// the producer only produces).
func (p *Producer) Dev(ctx context.Context, stageCtx pluginpkg.StageContext) ([]devwatch.WatchWorker, error) {
	if p == nil || strings.TrimSpace(p.SiteDir) == "" {
		return nil, nil
	}
	if strings.TrimSpace(stageCtx.OutputDir) == "" {
		return nil, fmt.Errorf("hugo: stageCtx.OutputDir is required for dev-watch of site %q", p.SiteDir)
	}

	absSite, err := filepath.Abs(p.SiteDir)
	if err != nil {
		return nil, err
	}
	outDir := p.outputDir(stageCtx, absSite)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}

	binaryPath, err := ResolveBinary(ctx, p.config(stageCtx))
	if err != nil {
		return nil, err
	}

	args := append(p.baseArgs(absSite, outDir), "--watch")
	worker, err := devwatch.StartCommandWatch(ctx, "hugo", "", binaryPath, args...)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(os.Stderr, "[stack] hugo watch started source=%s destination=%s\n", absSite, outDir)
	return []devwatch.WatchWorker{worker}, nil
}

func (p *Producer) baseArgs(absSite, absOut string) []string {
	args := []string{"--source", absSite, "--destination", absOut, "--cleanDestinationDir", "--gc"}
	if strings.TrimSpace(p.ConfigFile) != "" {
		args = append(args, "--config", p.ConfigFile)
	}
	if strings.TrimSpace(p.BaseURL) != "" {
		args = append(args, "--baseURL", p.BaseURL)
	}
	return args
}

func (p *Producer) config(stageCtx pluginpkg.StageContext) Config {
	return Config{Binary: p.Binary, Version: p.Version, Bin: stageCtx.Bin}
}

// outputDir keys the output path off absSite (not just a fixed "hugo"
// subdir) so two distinct Hugo(...) producers composed into the same target
// (e.g. two sites mounted at different prefixes) don't collide writing into
// the same destination tree. Takes the already-resolved absolute site dir
// rather than re-deriving it from p.SiteDir, since both Build and Dev need
// it for --source too — one filepath.Abs call per invocation, not two.
func (p *Producer) outputDir(stageCtx pluginpkg.StageContext, absSite string) string {
	return filepath.Join(stageCtx.OutputDir, "hugo", siteKey(absSite))
}

func siteKey(absSite string) string {
	sum := sha1.Sum([]byte(absSite))
	base := filepath.Base(absSite)
	if base == "" || base == string(filepath.Separator) || base == "." {
		base = "site"
	}
	return base + "-" + hex.EncodeToString(sum[:4])
}
