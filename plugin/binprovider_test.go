package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
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
