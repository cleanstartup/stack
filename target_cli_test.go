package stack_test

import (
	"context"
	"testing"

	"github.com/cleanstartup/stack"
	"github.com/cleanstartup/stack/plugin"
)

// TestCLIAppTargetNeverTouchesAssetBytes proves Consume is a genuine no-op —
// not just "happens to work" — by pointing an Asset at a file that doesn't
// exist and asserting success anyway.
func TestCLIAppTargetNeverTouchesAssetBytes(t *testing.T) {
	target := stack.CLIApp(stack.Bundle("cli-test"))
	err := target.Consume(context.Background(), []plugin.Contribution{
		{Assets: []plugin.Asset{{Path: "/nonexistent/does/not/exist.css", ContentType: "text/css"}}},
		{Assets: []plugin.Asset{{Path: "/nonexistent/does/not/exist.html", ContentType: "text/html"}}, Mount: "/docs"},
	})
	if err != nil {
		t.Fatalf("Consume: %v, want a CLI target to ignore assets entirely regardless of whether they exist", err)
	}
}

func TestCLIAppTargetBuildIgnoresProducerOutput(t *testing.T) {
	producer := fakeProducer{assets: []plugin.Asset{{Path: "/nonexistent.css", ContentType: "text/css"}}}
	target := stack.CLIApp(stack.Bundle("cli-test-2"), producer)
	if err := target.Build(context.Background(), plugin.StageContext{}); err != nil {
		t.Fatalf("Build: %v, want a CLI target to never fail on asset content it never reads", err)
	}
}
