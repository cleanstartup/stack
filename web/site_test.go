package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSiteConfigFilesInjectsModuleImport(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "consumer")
	moduleRoot := filepath.Join(tmp, "workspace")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "hugo.toml"), []byte("title = \"demo\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := siteConfigFiles(sourceDir, moduleRoot)
	if err != nil {
		t.Fatalf("siteConfigFiles failed: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected consumer config plus module config, got %d", len(files))
	}
	if files[0] != filepath.Join(sourceDir, "hugo.toml") {
		t.Fatalf("expected consumer config first, got %q", files[0])
	}
	data, err := os.ReadFile(files[1])
	if err != nil {
		t.Fatalf("read module config: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, siteModuleImportPath) {
		t.Fatalf("expected module import path in config, got %q", body)
	}
	if !strings.Contains(body, "replacements =") {
		t.Fatalf("expected module replacements in config, got %q", body)
	}
}
