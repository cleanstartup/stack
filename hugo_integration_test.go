package stack_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cleanstartup/stack"
	hugopkg "github.com/cleanstartup/stack/internal/hugo"
	"github.com/cleanstartup/stack/plugin"
)

// writeFakeHugoBinary stands in for the real hugo binary in these
// stack-level integration tests, without depending on internal/hugo's own
// (unexported) test helper. It writes a marker index.html with an empty
// <head> into --destination, matching real hugo's own output shape closely
// enough to prove InjectManifestLinks against it.
func writeFakeHugoBinary(t *testing.T, dir string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake shell binary helper is unix-only")
	}
	binaryPath := filepath.Join(dir, "hugo-fake.sh")
	script := "#!/bin/sh\nset -eu\ndest=\nwhile [ $# -gt 0 ]; do\n  case \"$1\" in\n    --destination) dest=\"$2\"; shift 2 ;;\n    --source|--config|--baseURL) shift 2 ;;\n    --cleanDestinationDir|--gc|--minify|--watch) shift ;;\n    *) shift ;;\n  esac\ndone\nmkdir -p \"$dest\"\nprintf '<html><head></head><body>hugo content</body></html>' > \"$dest/index.html\"\n"
	if err := os.WriteFile(binaryPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return binaryPath
}

// TestMountHugoProducerOnWebsiteTarget is CUP-22's concrete proof of PRD §6
// ("parametric producer mounting", D-M): Mount(Hugo(...), "/docs") composed
// on a real Website target, exercised end-to-end through compose.go's
// existing flattenIngredients/applyMountEdge mechanism — no new composition
// mechanics, just a real plugin.Builder producer proving the mechanism that
// already mechanically supports it (applyMountEdge's `case plugin.Builder`).
func TestMountHugoProducerOnWebsiteTarget(t *testing.T) {
	siteDir := t.TempDir()
	fakeBinary := writeFakeHugoBinary(t, t.TempDir())

	hugoProducer := hugopkg.Hugo(siteDir)
	hugoProducer.Binary = fakeBinary

	outDir := t.TempDir()
	buildDir := t.TempDir()

	target := stack.Website(stack.Bundle("hugo-integration"), stack.Mount(hugoProducer, "/docs"))
	target.OutputDir = outDir

	if err := target.Build(context.Background(), plugin.StageContext{OutputDir: buildDir}); err != nil {
		t.Fatalf("Build: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(outDir, "docs", "index.html"))
	if err != nil {
		t.Fatalf("expected hugo output copied under the /docs mount point: %v", err)
	}
	if !strings.Contains(string(got), "hugo content") {
		t.Fatalf("got %q, want hugo's own marker content", got)
	}

	manifest := target.Manifest()
	if len(manifest["text/html"]) != 1 || manifest["text/html"][0] != filepath.Join("docs", "index.html") {
		t.Fatalf("Manifest() = %+v, want text/html -> [docs/index.html]", manifest)
	}
}

// TestWebsiteTargetInjectManifestLinksIntoHugoOutput is the "relative
// <link>" half of D-L's Website policy (CUP-22, the Manifest() doc comment's
// named consumer): a hugo tree mounted at /docs alongside a CSS asset copied
// to the output root gets a real, working relative <link> injected into
// hugo's own rendered HTML — not a hand-rolled fixture, hugo's (fake)
// output tree exercised through the same Build()/Consume() path as the test
// above.
func TestWebsiteTargetInjectManifestLinksIntoHugoOutput(t *testing.T) {
	siteDir := t.TempDir()
	fakeBinary := writeFakeHugoBinary(t, t.TempDir())

	hugoProducer := hugopkg.Hugo(siteDir)
	hugoProducer.Binary = fakeBinary

	cssSrcDir := t.TempDir()
	cssPath := writeTestFile(t, cssSrcDir, "app.css", "body{color:blue}")
	cssProducer := fakeProducer{assets: []plugin.Asset{{Path: cssPath, ContentType: "text/css"}}}

	outDir := t.TempDir()
	buildDir := t.TempDir()

	target := stack.Website(stack.Bundle("hugo-integration-links"), cssProducer, stack.Mount(hugoProducer, "/docs"))
	target.OutputDir = outDir

	if err := target.Build(context.Background(), plugin.StageContext{OutputDir: buildDir}); err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := target.InjectManifestLinks(); err != nil {
		t.Fatalf("InjectManifestLinks: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(outDir, "docs", "index.html"))
	if err != nil {
		t.Fatalf("read hugo output: %v", err)
	}
	// app.css lives at the output root; docs/index.html is one level down,
	// so the injected href must be relative ("../app.css"), not absolute.
	if !strings.Contains(string(got), `<link rel="stylesheet" href="../app.css">`) {
		t.Fatalf("output %q missing relative <link> to ../app.css", got)
	}
	if !strings.Contains(string(got), "hugo content") {
		t.Fatalf("output %q lost hugo's own body content after injection", got)
	}
}
