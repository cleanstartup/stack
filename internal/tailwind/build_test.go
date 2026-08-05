package tailwind

import (
	"context"
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

func TestResolveBinaryUsesSharedBinProvider(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("tailwind binary release assets are only published for darwin/linux")
	}

	fake := &fakeBinProvider{path: "/cache/tailwindcss/v4.1.3/tailwindcss"}
	cfg := Config{Version: "v4.1.3", Bin: fake}

	path, err := ResolveBinary(context.Background(), cfg)
	if err != nil {
		t.Fatalf("ResolveBinary failed: %v", err)
	}
	if path != fake.path {
		t.Fatalf("expected ResolveBinary to return the shared BinProvider's path, got %q", path)
	}
	if fake.spec.Version != "v4.1.3" {
		t.Fatalf("expected resolved version v4.1.3, got %q", fake.spec.Version)
	}
	if fake.spec.Name == "" {
		t.Fatalf("expected a platform-specific binary name, got empty")
	}
	if !strings.Contains(fake.spec.URL, "v4.1.3") {
		t.Fatalf("expected release URL to reference the resolved version, got %q", fake.spec.URL)
	}
}

func TestResolveBinaryExplicitBinaryBypassesBinProvider(t *testing.T) {
	fake := &fakeBinProvider{path: "/should/not/be/used"}
	cfg := Config{Binary: "/opt/bin/tailwindcss", Bin: fake}

	path, err := ResolveBinary(context.Background(), cfg)
	if err != nil {
		t.Fatalf("ResolveBinary failed: %v", err)
	}
	if path != "/opt/bin/tailwindcss" {
		t.Fatalf("expected explicit binary to short-circuit, got %q", path)
	}
	if fake.spec.URL != "" {
		t.Fatalf("expected BinProvider not to be invoked, got spec %+v", fake.spec)
	}
}
