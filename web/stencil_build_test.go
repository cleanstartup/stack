package web

import (
	"strings"
	"testing"
)

func TestStencilWorkspacePackageIncludesBrandDependencies(t *testing.T) {
	t.Parallel()

	src := stencilPackageSource()
	for _, want := range []string{
		"\"altcha\"",
		"\"embla-carousel\"",
		"\"embla-carousel-auto-scroll\"",
		"\"htmx.org\"",
		"\"posthog-js\"",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("expected package source to include %s, got %s", want, src)
		}
	}
}

func TestStencilWorkspaceTsConfigUsesModernLibs(t *testing.T) {
	t.Parallel()

	src := stencilTSConfigSource()
	for _, want := range []string{
		"\"dom.iterable\"",
		"\"es2020\"",
		"\"moduleResolution\": \"bundler\"",
		"\"target\": \"es2020\"",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("expected tsconfig source to include %s, got %s", want, src)
		}
	}
}
