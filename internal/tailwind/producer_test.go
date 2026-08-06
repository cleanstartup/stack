package tailwind

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	pluginpkg "github.com/cleanstartup/stack/plugin"
)

func writeFakeTailwindBinary(t *testing.T, dir string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake shell binary helper is unix-only")
	}
	binaryPath := filepath.Join(dir, "tailwind-fake.sh")
	script := "#!/bin/sh\nset -eu\ninput=\noutput=\nwhile [ $# -gt 0 ]; do\n  case \"$1\" in\n    -i) input=\"$2\"; shift 2 ;;\n    -o) output=\"$2\"; shift 2 ;;\n    --minify|--watch) shift ;;\n    *) shift ;;\n  esac\ndone\ncp \"$input\" \"$output\"\n"
	if err := os.WriteFile(binaryPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return binaryPath
}

func TestProducerBuildNoScanPathsIsNoop(t *testing.T) {
	p := NewProducer()
	assets, err := p.Build(context.Background(), pluginpkg.StageContext{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if assets != nil {
		t.Fatalf("got %v assets, want nil for zero scan paths", assets)
	}
}

func TestProducerBuildProducesCSSAssetFromScannedContent(t *testing.T) {
	contentDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(contentDir, "extra.tailwind.css"), []byte(".extra{color:blue}"), 0o644); err != nil {
		t.Fatal(err)
	}

	outputDir := t.TempDir()
	binaryPath := writeFakeTailwindBinary(t, t.TempDir())

	p := NewProducer(contentDir)
	p.Binary = binaryPath

	assets, err := p.Build(context.Background(), pluginpkg.StageContext{OutputDir: outputDir})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("got %d assets, want 1: %+v", len(assets), assets)
	}
	if assets[0].ContentType != "text/css" {
		t.Fatalf("ContentType = %q, want text/css", assets[0].ContentType)
	}
	body, err := os.ReadFile(assets[0].Path)
	if err != nil {
		t.Fatalf("read output asset: %v", err)
	}
	got := string(body)
	if !strings.Contains(got, `@import "tailwindcss";`) {
		t.Fatalf("output %q missing tailwindcss import", got)
	}
	if !strings.Contains(got, filepath.ToSlash(contentDir)) {
		t.Fatalf("output %q missing @source for scanned content dir %q", got, contentDir)
	}
}
