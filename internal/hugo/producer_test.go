package hugo

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cleanstartup/stack/devwatch"
	pluginpkg "github.com/cleanstartup/stack/plugin"
)

// writeFakeHugoBinary writes a shell script standing in for the real hugo
// binary: it parses the same flags Producer passes (--source/--destination/
// etc.) and writes a marker index.html into --destination, closely mirroring
// tailwind's writeFakeTailwindBinary (internal/tailwind/build_test.go).
func writeFakeHugoBinary(t *testing.T, dir string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake shell binary helper is unix-only")
	}
	binaryPath := filepath.Join(dir, "hugo-fake.sh")
	script := "#!/bin/sh\nset -eu\ndest=\nsrc=\nwhile [ $# -gt 0 ]; do\n  case \"$1\" in\n    --destination) dest=\"$2\"; shift 2 ;;\n    --source) src=\"$2\"; shift 2 ;;\n    --config|--baseURL) shift 2 ;;\n    --cleanDestinationDir|--gc|--minify|--watch) shift ;;\n    *) shift ;;\n  esac\ndone\nmkdir -p \"$dest\"\necho \"<html><head></head><body>from $src</body></html>\" > \"$dest/index.html\"\n"
	if err := os.WriteFile(binaryPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return binaryPath
}

// writeFakeHugoWatchBinary writes a long-running shell script standing in
// for `hugo --watch`: it writes the marker file once, then blocks until
// killed, so Dev's WatchWorker has something real to stop.
func writeFakeHugoWatchBinary(t *testing.T, dir string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake shell binary helper is unix-only")
	}
	binaryPath := filepath.Join(dir, "hugo-watch-fake.sh")
	script := "#!/bin/sh\nset -eu\ndest=\nwhile [ $# -gt 0 ]; do\n  case \"$1\" in\n    --destination) dest=\"$2\"; shift 2 ;;\n    --source|--config|--baseURL) shift 2 ;;\n    --cleanDestinationDir|--gc|--minify|--watch) shift ;;\n    *) shift ;;\n  esac\ndone\nmkdir -p \"$dest\"\necho '<html><head></head><body>watching</body></html>' > \"$dest/index.html\"\ntrap 'exit 0' TERM INT\nwhile true; do sleep 1; done\n"
	if err := os.WriteFile(binaryPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return binaryPath
}

func TestProducerBuildNoSiteDirIsNoop(t *testing.T) {
	p := &Producer{}
	assets, err := p.Build(context.Background(), pluginpkg.StageContext{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if assets != nil {
		t.Fatalf("got %v assets, want nil for an unset SiteDir", assets)
	}
}

func TestProducerBuildRequiresOutputDir(t *testing.T) {
	p := Hugo(t.TempDir())
	if _, err := p.Build(context.Background(), pluginpkg.StageContext{}); err == nil {
		t.Fatal("want an error when stageCtx.OutputDir is unset")
	}
}

func TestProducerBuildProducesHTMLTreeAsset(t *testing.T) {
	siteDir := t.TempDir()
	outputDir := t.TempDir()
	binaryPath := writeFakeHugoBinary(t, t.TempDir())

	p := Hugo(siteDir)
	p.Binary = binaryPath

	assets, err := p.Build(context.Background(), pluginpkg.StageContext{OutputDir: outputDir})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("got %d assets, want 1: %+v", len(assets), assets)
	}
	if assets[0].ContentType != "text/html" {
		t.Fatalf("ContentType = %q, want text/html", assets[0].ContentType)
	}
	info, err := os.Stat(assets[0].Path)
	if err != nil || !info.IsDir() {
		t.Fatalf("expected asset Path to be a directory, got %q (err %v)", assets[0].Path, err)
	}
	body, err := os.ReadFile(filepath.Join(assets[0].Path, "index.html"))
	if err != nil {
		t.Fatalf("read hugo output: %v", err)
	}
	if !strings.Contains(string(body), filepath.ToSlash(siteDir)) {
		t.Fatalf("output %q missing --source site dir %q", body, siteDir)
	}
}

func TestProducerBuildTwoProducersDoNotCollide(t *testing.T) {
	outputDir := t.TempDir()
	binaryPath := writeFakeHugoBinary(t, t.TempDir())

	p1 := Hugo(t.TempDir())
	p1.Binary = binaryPath
	p2 := Hugo(t.TempDir())
	p2.Binary = binaryPath

	a1, err := p1.Build(context.Background(), pluginpkg.StageContext{OutputDir: outputDir})
	if err != nil {
		t.Fatalf("Build p1: %v", err)
	}
	a2, err := p2.Build(context.Background(), pluginpkg.StageContext{OutputDir: outputDir})
	if err != nil {
		t.Fatalf("Build p2: %v", err)
	}
	if a1[0].Path == a2[0].Path {
		t.Fatalf("expected two distinct Hugo producers to get distinct output dirs, both got %q", a1[0].Path)
	}
}

func TestProducerDevNoSiteDirIsNoop(t *testing.T) {
	p := &Producer{}
	workers, err := p.Dev(context.Background(), pluginpkg.StageContext{})
	if err != nil {
		t.Fatalf("Dev: %v", err)
	}
	if workers != nil {
		t.Fatalf("got %v workers, want nil for an unset SiteDir", workers)
	}
}

func TestProducerDevStartsWatchWorker(t *testing.T) {
	siteDir := t.TempDir()
	outputDir := t.TempDir()
	binaryPath := writeFakeHugoWatchBinary(t, t.TempDir())

	p := Hugo(siteDir)
	p.Binary = binaryPath

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	workers, err := p.Dev(ctx, pluginpkg.StageContext{OutputDir: outputDir})
	if err != nil {
		t.Fatalf("Dev: %v", err)
	}
	if len(workers) != 1 {
		t.Fatalf("got %d workers, want 1", len(workers))
	}
	if workers[0].Name != "hugo" {
		t.Fatalf("worker.Name = %q, want hugo", workers[0].Name)
	}

	absSiteDir, err := filepath.Abs(siteDir)
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	outDir := p.outputDir(pluginpkg.StageContext{OutputDir: outputDir}, absSiteDir)
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, statErr := os.Stat(filepath.Join(outDir, "index.html")); statErr == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for watch worker to write index.html")
		}
		time.Sleep(20 * time.Millisecond)
	}

	devwatch.StopWatchWorkers(workers)
}
