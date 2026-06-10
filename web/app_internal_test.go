package web

import "testing"

func TestDefaultWebCLIArgsDefaultsToRun(t *testing.T) {
	if got := defaultWebCLIArgs(nil); len(got) != 1 || got[0] != "run" {
		t.Fatalf("expected default run args, got %#v", got)
	}
	if got := defaultWebCLIArgs([]string{"--help"}); len(got) != 1 || got[0] != "--help" {
		t.Fatalf("expected explicit args to pass through, got %#v", got)
	}
}
