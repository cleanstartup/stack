package hugo

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	pluginpkg "github.com/cleanstartup/stack/plugin"
)

type fakeBinProvider struct {
	spec pluginpkg.BinarySpec
	path string
}

func (f *fakeBinProvider) Ensure(ctx context.Context, spec pluginpkg.BinarySpec) (string, error) {
	f.spec = spec
	return f.path, nil
}

func TestResolveBinaryExplicitBinaryBypassesBinProvider(t *testing.T) {
	fake := &fakeBinProvider{path: "/should/not/be/used"}
	cfg := Config{Binary: "/opt/bin/hugo", Bin: fake}

	path, err := ResolveBinary(context.Background(), cfg)
	if err != nil {
		t.Fatalf("ResolveBinary failed: %v", err)
	}
	if path != "/opt/bin/hugo" {
		t.Fatalf("expected explicit binary to short-circuit, got %q", path)
	}
	if fake.spec.URL != "" {
		t.Fatalf("expected BinProvider not to be invoked, got spec %+v", fake.spec)
	}
}

func TestResolveBinaryUsesSharedBinProviderAndExtractsArchive(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("hugo release assets are only mapped here for darwin/linux")
	}

	archivePath := writeTestTarGz(t, t.TempDir(), "hugo-fake.tar.gz", map[string]string{
		"LICENSE": "license text",
		"hugo":    "#!/bin/sh\necho fake-hugo\n",
	})
	fake := &fakeBinProvider{path: archivePath}
	cacheDir := t.TempDir()
	cfg := Config{Version: "v0.134.3", Bin: fake, CacheDir: cacheDir}

	path, err := ResolveBinary(context.Background(), cfg)
	if err != nil {
		t.Fatalf("ResolveBinary failed: %v", err)
	}

	if fake.spec.Version != "v0.134.3" {
		t.Fatalf("expected resolved tag version v0.134.3, got %q", fake.spec.Version)
	}
	if !strings.Contains(fake.spec.URL, "v0.134.3") {
		t.Fatalf("expected release URL to reference the resolved tag, got %q", fake.spec.URL)
	}
	if fake.spec.Name != "hugo-archive" {
		t.Fatalf("expected BinarySpec.Name = hugo-archive, got %q", fake.spec.Name)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected extracted hugo binary at %s: %v", path, err)
	}
	if string(data) != "#!/bin/sh\necho fake-hugo\n" {
		t.Fatalf("unexpected extracted binary contents: %q", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("expected extracted binary to be executable, got mode %v", info.Mode())
	}
}

func TestReleaseAssetNameEmbedsFileVersion(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("hugo release assets are only mapped here for darwin/linux")
	}
	name, err := releaseAssetName("0.134.3")
	if err != nil {
		t.Fatalf("releaseAssetName: %v", err)
	}
	if !strings.Contains(name, "0.134.3") {
		t.Fatalf("expected asset name to embed the file version, got %q", name)
	}
	if !strings.HasSuffix(name, ".tar.gz") {
		t.Fatalf("expected a .tar.gz asset name, got %q", name)
	}
}

func TestBuildReleaseURL(t *testing.T) {
	got := buildReleaseURL("https://github.com/gohugoio/hugo/releases", "v0.134.3", "hugo_0.134.3_linux-amd64.tar.gz")
	want := "https://github.com/gohugoio/hugo/releases/download/v0.134.3/hugo_0.134.3_linux-amd64.tar.gz"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// normalizeTagVersion is only ever called after "latest" has already been
// resolved to a concrete version (see resolveBinaryDownload) — same
// contract as tailwind's normalizeVersion (internal/tailwind/build.go), so
// a literal "latest" input isn't a real code path and isn't asserted here.
func TestNormalizeTagVersion(t *testing.T) {
	cases := map[string]string{
		"":         "latest",
		"0.134.3":  "v0.134.3",
		"v0.134.3": "v0.134.3",
	}
	for in, want := range cases {
		if got := normalizeTagVersion(in); got != want {
			t.Fatalf("normalizeTagVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCachedLatestVersionRoundTrips(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	if _, ok := loadCachedLatestVersion(dir); ok {
		t.Fatal("expected no cached version before saving one")
	}
	if err := saveCachedLatestVersion(dir, "v0.134.3"); err != nil {
		t.Fatalf("saveCachedLatestVersion: %v", err)
	}
	got, ok := loadCachedLatestVersion(dir)
	if !ok || got != "v0.134.3" {
		t.Fatalf("loadCachedLatestVersion = (%q, %v), want (v0.134.3, true)", got, ok)
	}
}
