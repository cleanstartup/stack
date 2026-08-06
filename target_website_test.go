package stack_test

import (
	"context"
	"os"
	"path/filepath"
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
