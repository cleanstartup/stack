package stack_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/cleanstartup/stack"
	"github.com/cleanstartup/stack/asset"
	"github.com/cleanstartup/stack/plugin"
	"github.com/cleanstartup/stack/way2go/activity"
)

type fakeProducer struct {
	assets []plugin.Asset
	err    error
}

func (f fakeProducer) Build(context.Context, plugin.StageContext) ([]plugin.Asset, error) {
	return f.assets, f.err
}

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestWebAppTargetServesAndLinksCSSAndJS(t *testing.T) {
	dir := t.TempDir()
	cssPath := writeTestFile(t, dir, "app.css", "body{color:red}")
	jsPath := writeTestFile(t, dir, "app.js", "console.log('hi')")

	producer := fakeProducer{assets: []plugin.Asset{
		{Path: cssPath, ContentType: "text/css"},
		{Path: jsPath, ContentType: "application/javascript+module"},
	}}

	target := stack.WebApp(stack.Bundle("webapp-test"), producer)
	if err := target.Build(context.Background(), plugin.StageContext{}); err != nil {
		t.Fatalf("Build: %v", err)
	}

	links := target.AssetLinks()
	if len(links.Styles) != 1 {
		t.Fatalf("got %d style links, want 1: %+v", len(links.Styles), links)
	}
	if len(links.Scripts) != 1 {
		t.Fatalf("got %d script links, want 1: %+v", len(links.Scripts), links)
	}

	server := httptest.NewServer(target.Handler())
	defer server.Close()

	assertBodyEquals(t, server.URL+links.Styles[0], "body{color:red}")
	assertBodyEquals(t, server.URL+links.Scripts[0], "console.log('hi')")
}

func TestWebAppTargetUnmountedForeignContentTypeErrors(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "docs"), 0o755)
	writeTestFile(t, filepath.Join(dir, "docs"), "index.html", "<html></html>")

	producer := fakeProducer{assets: []plugin.Asset{
		{Path: filepath.Join(dir, "docs"), ContentType: "text/html"},
	}}

	target := stack.WebApp(stack.Bundle("webapp-test-2"), producer)
	if err := target.Build(context.Background(), plugin.StageContext{}); err == nil {
		t.Fatal("want an error: a bare (unmounted) text/html tree has no sensible default root in a WebApp")
	}
}

func TestWebAppTargetMountedForeignContentTypeServes(t *testing.T) {
	dir := t.TempDir()
	docsDir := filepath.Join(dir, "docs")
	os.Mkdir(docsDir, 0o755)
	writeTestFile(t, docsDir, "index.html", "hugo output")

	producer := fakeProducer{assets: []plugin.Asset{
		{Path: docsDir, ContentType: "text/html"},
	}}

	target := stack.WebApp(stack.Bundle("webapp-test-3"), stack.Mount(producer, "/docs"))
	if err := target.Build(context.Background(), plugin.StageContext{}); err != nil {
		t.Fatalf("Build: %v", err)
	}

	server := httptest.NewServer(target.Handler())
	defer server.Close()

	assertBodyEquals(t, server.URL+"/docs/index.html", "hugo output")
}

func TestWebAppTargetMountedHandlerServesDirectly(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("auth ok"))
	})

	target := stack.WebApp(stack.Bundle("webapp-test-4"), stack.Mount(handler, "/auth"))
	if err := target.Build(context.Background(), plugin.StageContext{}); err != nil {
		t.Fatalf("Build: %v", err)
	}

	server := httptest.NewServer(target.Handler())
	defer server.Close()

	assertBodyEquals(t, server.URL+"/auth", "auth ok")
}

func TestWebAppTargetSharedMountPointErrors(t *testing.T) {
	dir := t.TempDir()
	firstPage := writeTestFile(t, dir, "first.html", "first")
	secondPage := writeTestFile(t, dir, "second.html", "second")

	producer := fakeProducer{assets: []plugin.Asset{
		{Path: firstPage, ContentType: "text/html"},
		{Path: secondPage, ContentType: "text/html"},
	}}

	target := stack.WebApp(stack.Bundle("webapp-test-5"), stack.Mount(producer, "/docs"))
	err := target.Build(context.Background(), plugin.StageContext{})
	if err == nil {
		t.Fatal("want an error: two foreign-content-type assets sharing one mount point would silently shadow each other")
	}
}

func TestWebAppTargetIgnoresLegacyModuleParts(t *testing.T) {
	dir := t.TempDir()
	cssPath := writeTestFile(t, dir, "legacy.css", "legacy{color:green}")

	module := stack.Bundle("webapp-test-6", stack.CSS(asset.FromFile(cssPath)))
	target := stack.WebApp(module) // no producers/ingredients at all
	if err := target.Build(context.Background(), plugin.StageContext{}); err != nil {
		t.Fatalf("Build: %v", err)
	}

	links := target.AssetLinks()
	if len(links.Styles) != 0 {
		t.Fatalf("AssetLinks().Styles = %v, want empty: legacy Module CSS Parts must not leak into the new contract's links", links.Styles)
	}
}

func assertBodyEquals(t *testing.T, url, want string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != want {
		t.Fatalf("GET %s: body = %q, want %q", url, body, want)
	}
}

// TestModuleEdgeMountOnWebAppIsFenced pins CUP-26's deliberate scope fence:
// D-M's Module-edge Mount ("routes+assets under a prefix, namespaced") isn't
// implemented yet — mounting a raw sub-handler without prefix-stripping or
// namespacing would silently 404 every route under the prefix instead of
// doing what the nature promises. A loud "not yet" beats that silent gap.
func TestModuleEdgeMountOnWebAppIsFenced(t *testing.T) {
	sub := stack.Bundle("docstest", stack.Activity("index", func(ctx activity.Context) activity.Result {
		return "docs-index"
	}, stack.WithView()))

	target := stack.WebApp(stack.Bundle("roottest"), stack.Mount(sub, "/docs"))
	err := target.Build(context.Background(), plugin.StageContext{})
	if err == nil {
		t.Fatal("want an error: Module-edge Mount on a WebApp target is not yet supported (CUP-21)")
	}
}
