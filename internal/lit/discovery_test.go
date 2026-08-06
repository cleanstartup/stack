package lit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverEntriesFindsLitSuffixedFiles(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "app.lit.ts", "")
	writeTestFile(t, dir, "widget.lit.tsx", "")
	writeTestFile(t, dir, "button.ts", "")    // library file, not an entry
	writeTestFile(t, dir, "helper.tsx", "")   // library file, not an entry
	writeTestFile(t, dir, "notes.lit.md", "") // wrong extension family entirely

	nested := filepath.Join(dir, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, nested, "nested.lit.ts", "")

	skip := filepath.Join(dir, "node_modules")
	if err := os.MkdirAll(skip, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, skip, "ignored.lit.ts", "")

	got, err := DiscoverEntries(dir)
	if err != nil {
		t.Fatalf("DiscoverEntries: %v", err)
	}
	want := []string{
		filepath.Join(dir, "app.lit.ts"),
		filepath.Join(nested, "nested.lit.ts"),
		filepath.Join(dir, "widget.lit.tsx"),
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	gotSet := map[string]bool{}
	for _, g := range got {
		gotSet[g] = true
	}
	for _, w := range want {
		if !gotSet[w] {
			t.Fatalf("missing expected entry %q in %v", w, got)
		}
	}
}

func TestDiscoverEntriesEmptyBaseDir(t *testing.T) {
	got, err := DiscoverEntries("")
	if err != nil {
		t.Fatalf("DiscoverEntries: %v", err)
	}
	if got != nil {
		t.Fatalf("got %v, want nil for empty baseDir", got)
	}
}

// TestDiscoverEntriesMissingDirErrors pins the fix for the discarded
// filepath.WalkDir error: a Module declaring a typo'd or deleted
// assets.Dir(...) path must fail the build loudly, not be silently
// indistinguishable from "this dir legitimately has no lit source in it".
func TestDiscoverEntriesMissingDirErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	got, err := DiscoverEntries(missing)
	if err == nil {
		t.Fatalf("want an error for a non-existent baseDir, got entries %v", got)
	}
}
