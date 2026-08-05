package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

func TestBinProviderEnsureDownloadsIntoCache(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable bit assertion is unix-only")
	}

	var requests int32
	payload := "#!/bin/sh\necho hi\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		_, _ = w.Write([]byte(payload))
	}))
	defer server.Close()

	cacheRoot := filepath.Join(t.TempDir(), "bin-cache")
	provider := NewBinProvider(cacheRoot)

	spec := BinarySpec{Name: "fakebin", Version: "v1.2.3", URL: server.URL}

	path, err := provider.Ensure(context.Background(), spec)
	if err != nil {
		t.Fatalf("first Ensure failed: %v", err)
	}
	wantPath := filepath.Join(cacheRoot, "fakebin", "v1.2.3", "fakebin")
	if path != wantPath {
		t.Fatalf("expected cached path %q, got %q", wantPath, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected downloaded binary at %s: %v", path, err)
	}
	if string(data) != payload {
		t.Fatalf("unexpected binary contents: %q", string(data))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat cached binary: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("expected cached binary to be executable, got mode %v", info.Mode())
	}
	if atomic.LoadInt32(&requests) != 1 {
		t.Fatalf("expected exactly one download request, got %d", requests)
	}

	// Second Ensure call must be idempotent: cache hit, no new download.
	path2, err := provider.Ensure(context.Background(), spec)
	if err != nil {
		t.Fatalf("second Ensure failed: %v", err)
	}
	if path2 != wantPath {
		t.Fatalf("expected same cached path on second call, got %q", path2)
	}
	if atomic.LoadInt32(&requests) != 1 {
		t.Fatalf("expected cache hit on second call, got %d requests", requests)
	}
}

func TestBinProviderEnsureRejectsEmptySpec(t *testing.T) {
	provider := NewBinProvider(t.TempDir())

	if _, err := provider.Ensure(context.Background(), BinarySpec{URL: "https://example.invalid/bin"}); err == nil {
		t.Fatal("expected error for empty name")
	}
	if _, err := provider.Ensure(context.Background(), BinarySpec{Name: "fakebin"}); err == nil {
		t.Fatal("expected error for empty url")
	}
}

func TestDefaultBinCacheDirPriority(t *testing.T) {
	binDir := filepath.Join(t.TempDir(), "bin-override")
	tailwindDir := filepath.Join(t.TempDir(), "tailwind-override")

	t.Run("STACK_BIN_CACHE_DIR wins over everything", func(t *testing.T) {
		t.Setenv("STACK_BIN_CACHE_DIR", binDir)
		t.Setenv("STACK_TAILWIND_CACHE_DIR", tailwindDir)
		if got := DefaultBinCacheDir(); got != binDir {
			t.Fatalf("expected STACK_BIN_CACHE_DIR to win, got %q", got)
		}
	})

	t.Run("STACK_TAILWIND_CACHE_DIR is honored as a BC fallback", func(t *testing.T) {
		t.Setenv("STACK_BIN_CACHE_DIR", "")
		t.Setenv("STACK_TAILWIND_CACHE_DIR", tailwindDir)
		if got := DefaultBinCacheDir(); got != tailwindDir {
			t.Fatalf("expected STACK_TAILWIND_CACHE_DIR fallback, got %q", got)
		}
	})

	t.Run("falls back to the default when nothing is set", func(t *testing.T) {
		t.Setenv("STACK_BIN_CACHE_DIR", "")
		t.Setenv("STACK_TAILWIND_CACHE_DIR", "")
		got := DefaultBinCacheDir()
		if !strings.HasSuffix(filepath.ToSlash(got), "/stack/bin") {
			t.Fatalf("expected default to end in /stack/bin, got %q", got)
		}
	})
}

// TestBinProviderEnsureRespectsConfiguredCacheDir reproduces the F1 fix: the
// shared BinProvider built the way BuildEngine builds it
// (NewBinProvider(DefaultBinCacheDir())) must honor STACK_TAILWIND_CACHE_DIR,
// so a freshly downloaded binary lands exactly where that variable points —
// not under the fixed $UserCacheDir/stack/bin default.
func TestBinProviderEnsureRespectsConfiguredCacheDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable bit assertion is unix-only")
	}

	configuredDir := filepath.Join(t.TempDir(), "configured-tailwind-cache")
	t.Setenv("STACK_BIN_CACHE_DIR", "")
	t.Setenv("STACK_TAILWIND_CACHE_DIR", configuredDir)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("#!/bin/sh\necho hi\n"))
	}))
	defer server.Close()

	provider := NewBinProvider(DefaultBinCacheDir())
	path, err := provider.Ensure(context.Background(), BinarySpec{Name: "tailwindcss", Version: "v4.1.3", URL: server.URL})
	if err != nil {
		t.Fatalf("Ensure failed: %v", err)
	}
	wantPath := filepath.Join(configuredDir, "tailwindcss", "v4.1.3", "tailwindcss")
	if path != wantPath {
		t.Fatalf("expected download under configured STACK_TAILWIND_CACHE_DIR, got %q, want %q", path, wantPath)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected binary at %s: %v", path, err)
	}
}
