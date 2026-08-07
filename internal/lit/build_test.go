package lit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBuildResolvesBareSpecifierViaNodePaths pins CUP-27's fix to the
// StageContext contract gap CUP-25 flagged: a bare specifier import (e.g.
// `from "lit"`) from a source file that is *not* inside absWorkingDir's own
// directory tree (the realistic shape for CUP-27's ui.Module() — its
// component sources live wherever its Go module was checked out, not inside
// the consuming app's project directory) must still resolve against
// absWorkingDir's node_modules.
//
// This specifically exercises the case AbsWorkingDir alone does not cover:
// esbuild's node_modules resolution walks upward from the *importing file's
// own directory*, not from AbsWorkingDir — confirmed by writing this test
// first against an AbsWorkingDir-only implementation, which failed with
// "Could not resolve \"lit\"" despite absWorkingDir being set correctly.
// NodePaths (this package's Build, set to absWorkingDir/node_modules) is
// what actually closes the gap for sources located outside the project
// tree.
func TestBuildResolvesBareSpecifierViaNodePaths(t *testing.T) {
	projectDir := t.TempDir()
	litPkgDir := filepath.Join(projectDir, "node_modules", "lit")
	if err := os.MkdirAll(litPkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, litPkgDir, "package.json", `{"name":"lit","version":"3.0.0-test","main":"index.js"}`)
	writeTestFile(t, litPkgDir, "index.js", `export const LIT_MARKER = "LIT_RESOLVED_MARKER";`)

	// componentDir is deliberately a *separate* temp dir from projectDir —
	// simulating ui.Module()'s sources living outside the app's own project
	// tree (a Go module checkout path), the case NodePaths exists to cover.
	componentDir := t.TempDir()
	entry := writeTestFile(t, componentDir, "app.lit.ts", `
import { LIT_MARKER } from "lit";
console.log(LIT_MARKER);
`)

	outDir := t.TempDir()
	assets, err := Build(context.Background(), []string{entry}, outDir, projectDir)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("got %d assets, want 1", len(assets))
	}

	got, err := os.ReadFile(assets[0].Path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(got), "LIT_RESOLVED_MARKER") {
		t.Fatalf("output %q missing lit's marker — bare specifier did not resolve via NodePaths", got)
	}
}

// TestBuildWithoutAbsWorkingDirFailsBareSpecifier is the negative control:
// an empty absWorkingDir must behave exactly as before this field existed —
// a bare specifier fails to resolve, it doesn't silently succeed some other
// way.
func TestBuildWithoutAbsWorkingDirFailsBareSpecifier(t *testing.T) {
	componentDir := t.TempDir()
	entry := writeTestFile(t, componentDir, "app.lit.ts", `
import { LIT_MARKER } from "lit";
console.log(LIT_MARKER);
`)

	outDir := t.TempDir()
	_, err := Build(context.Background(), []string{entry}, outDir, "")
	if err == nil {
		t.Fatal("want an error: no absWorkingDir means no node_modules to resolve \"lit\" against")
	}
}
