package stack_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cleanstartup/stack"
	"github.com/cleanstartup/stack/plugin"
)

func TestWebsiteTargetCopiesToRootByDefault(t *testing.T) {
	srcDir := t.TempDir()
	cssPath := writeTestFile(t, srcDir, "app.css", "body{color:blue}")

	producer := fakeProducer{assets: []plugin.Asset{
		{Path: cssPath, ContentType: "text/css"},
	}}

	outDir := t.TempDir()
	target := stack.Website(stack.Bundle("website-test"), producer)
	target.OutputDir = outDir

	if err := target.Consume(context.Background(), []plugin.Contribution{
		{Assets: producer.assets}, // Mount == "" -> copies to output root
	}); err != nil {
		t.Fatalf("Consume: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(outDir, "app.css"))
	if err != nil {
		t.Fatalf("expected app.css at output root: %v", err)
	}
	if string(got) != "body{color:blue}" {
		t.Fatalf("got %q", got)
	}

	manifest := target.Manifest()
	if len(manifest["text/css"]) != 1 || manifest["text/css"][0] != "app.css" {
		t.Fatalf("Manifest() = %+v, want text/css -> [app.css]", manifest)
	}
}

func TestWebsiteTargetCopiesUnderExplicitMount(t *testing.T) {
	srcDir := t.TempDir()
	htmlDir := filepath.Join(srcDir, "docs")
	if err := os.Mkdir(htmlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, htmlDir, "index.html", "hugo output")

	outDir := t.TempDir()
	target := stack.Website(stack.Bundle("website-test-2"))
	target.OutputDir = outDir

	if err := target.Consume(context.Background(), []plugin.Contribution{
		{Assets: []plugin.Asset{{Path: htmlDir, ContentType: "text/html"}}, Mount: "/docs"},
	}); err != nil {
		t.Fatalf("Consume: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(outDir, "docs", "index.html"))
	if err != nil {
		t.Fatalf("expected docs/index.html under the mount point: %v", err)
	}
	if string(got) != "hugo output" {
		t.Fatalf("got %q", got)
	}

	manifest := target.Manifest()
	if len(manifest["text/html"]) != 1 || manifest["text/html"][0] != filepath.Join("docs", "index.html") {
		t.Fatalf("Manifest() = %+v, want text/html -> [docs/index.html], not the bare directory", manifest)
	}
}

func TestWebsiteTargetDestinationCollisionErrors(t *testing.T) {
	firstDir := t.TempDir()
	secondDir := t.TempDir()
	firstCSS := writeTestFile(t, firstDir, "app.css", "from first")
	secondCSS := writeTestFile(t, secondDir, "app.css", "from second")

	target := stack.Website(stack.Bundle("website-test-4"))
	target.OutputDir = t.TempDir()

	err := target.Consume(context.Background(), []plugin.Contribution{
		{Assets: []plugin.Asset{{Path: firstCSS, ContentType: "text/css"}}},
		{Assets: []plugin.Asset{{Path: secondCSS, ContentType: "text/css"}}},
	})
	if err == nil {
		t.Fatal("want an error: two producers copying distinct source files to the same destination basename")
	}
}

func TestWebsiteTargetDistinctBasenamesSucceed(t *testing.T) {
	srcDir := t.TempDir()
	firstCSS := writeTestFile(t, srcDir, "a.css", "a")
	secondCSS := writeTestFile(t, srcDir, "b.css", "b")

	target := stack.Website(stack.Bundle("website-test-5"))
	target.OutputDir = t.TempDir()

	err := target.Consume(context.Background(), []plugin.Contribution{
		{Assets: []plugin.Asset{{Path: firstCSS, ContentType: "text/css"}}},
		{Assets: []plugin.Asset{{Path: secondCSS, ContentType: "text/css"}}},
	})
	if err != nil {
		t.Fatalf("Consume: %v, want distinct destination basenames to succeed", err)
	}
}

func TestWebsiteTargetRequiresOutputDir(t *testing.T) {
	target := stack.Website(stack.Bundle("website-test-3"))
	err := target.Consume(context.Background(), []plugin.Contribution{
		{Assets: []plugin.Asset{{Path: "x.css", ContentType: "text/css"}}},
	})
	if err == nil {
		t.Fatal("want an error when OutputDir is not set")
	}
}

// TestWebsiteTargetInjectManifestLinksHonorsModuleHint pins the +module
// byte-semantics fix (D-K: unlike +head/+footer, +module is not a droppable
// placement hint) — a script contributed as application/javascript+module
// must get type="module" so ESM import/export actually parses in a browser,
// while a bare script must not carry the attribute.
func TestWebsiteTargetInjectManifestLinksHonorsModuleHint(t *testing.T) {
	srcDir := t.TempDir()
	cssPath := writeTestFile(t, srcDir, "app.css", "body{color:blue}")
	modulePath := writeTestFile(t, srcDir, "module.js", "export const x = 1;")
	classicPath := writeTestFile(t, srcDir, "classic.js", "var x = 1;")
	htmlDir := filepath.Join(srcDir, "docs")
	if err := os.Mkdir(htmlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, htmlDir, "index.html", "<html><head></head><body>hi</body></html>")

	outDir := t.TempDir()
	target := stack.Website(stack.Bundle("website-test-6"))
	target.OutputDir = outDir

	err := target.Consume(context.Background(), []plugin.Contribution{
		{Assets: []plugin.Asset{{Path: cssPath, ContentType: "text/css"}}},
		{Assets: []plugin.Asset{{Path: modulePath, ContentType: "application/javascript+module"}}},
		{Assets: []plugin.Asset{{Path: classicPath, ContentType: "application/javascript"}}},
		{Assets: []plugin.Asset{{Path: htmlDir, ContentType: "text/html"}}, Mount: "/docs"},
	})
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if err := target.InjectManifestLinks(); err != nil {
		t.Fatalf("InjectManifestLinks: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(outDir, "docs", "index.html"))
	if err != nil {
		t.Fatalf("read injected html: %v", err)
	}
	body := string(got)
	if !strings.Contains(body, `<link rel="stylesheet" href="../app.css">`) {
		t.Fatalf("output %q missing relative CSS link", body)
	}
	if !strings.Contains(body, `<script type="module" src="../module.js"></script>`) {
		t.Fatalf("output %q missing type=\"module\" on the ESM script", body)
	}
	if !strings.Contains(body, `<script src="../classic.js"></script>`) {
		t.Fatalf("output %q missing the classic script tag", body)
	}
	if strings.Contains(body, `type="module" src="../classic.js"`) {
		t.Fatalf("output %q incorrectly marked the classic script as a module", body)
	}
}
