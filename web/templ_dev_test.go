package web

import (
	"strings"
	"testing"
	"time"
)

func TestTemplProxyURLDefaultsToLocalhost(t *testing.T) {
	t.Parallel()

	if got := templProxyURL(":8080"); got != "http://127.0.0.1:8080" {
		t.Fatalf("expected localhost proxy url, got %q", got)
	}
	if got := templProxyURL("0.0.0.0:9090"); got != "http://127.0.0.1:9090" {
		t.Fatalf("expected loopback proxy url, got %q", got)
	}
}

func TestDevChildCommandIncludesChildFlag(t *testing.T) {
	t.Parallel()

	got := devChildCommand("/repo", "/repo/cmd/demo", DevConfig{
		Addr:         ":8081",
		PollInterval: 500 * time.Millisecond,
	})

	if !strings.Contains(got, "go run ./cmd/demo dev --child") {
		t.Fatalf("expected child command, got %q", got)
	}
	if !strings.Contains(got, "--addr=:8081") {
		t.Fatalf("expected addr flag, got %q", got)
	}
	if !strings.Contains(got, "--workspace=/repo/cmd/demo/.stack/workspace") {
		t.Fatalf("expected workspace flag, got %q", got)
	}
	if !strings.Contains(got, "--output=/repo/cmd/demo/.stack/public") {
		t.Fatalf("expected output flag, got %q", got)
	}
	if !strings.Contains(got, "--poll=500ms") {
		t.Fatalf("expected poll flag, got %q", got)
	}
}
