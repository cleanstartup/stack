package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSiteConfigFilesInjectsModuleImports(t *testing.T) {
	tmp := t.TempDir()
	moduleRoot := filepath.Join(tmp, "workspace")
	brandingRoot := filepath.Join(tmp, "branding")
	if err := os.MkdirAll(brandingRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	unsafe := true
	files, err := siteConfigFiles(moduleRoot, &SiteOptions{
		Title:        "demo",
		BaseURL:      "http://127.0.0.1:8080/",
		DisableKinds: []string{"taxonomy", "term"},
		MarkupUnsafe: &unsafe,
		Params: map[string]string{
			"brandLead": "hello",
		},
	}, siteHugoModule(), HugoModule{ImportPath: "github.com/cleanstartup/branding", ReplacePath: brandingRoot})
	if err != nil {
		t.Fatalf("siteConfigFiles failed: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected site config plus module config, got %d", len(files))
	}
	if files[0] != filepath.Join(moduleRoot, "site.hugo.toml") {
		t.Fatalf("expected site config first, got %q", files[0])
	}
	data, err := os.ReadFile(files[1])
	if err != nil {
		t.Fatalf("read module config: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, siteModuleImportPath) {
		t.Fatalf("expected stack module import path in config, got %q", body)
	}
	if !strings.Contains(body, "github.com/cleanstartup/branding") {
		t.Fatalf("expected branding module import path in config, got %q", body)
	}
	if !strings.Contains(body, "replacements =") {
		t.Fatalf("expected module replacements in config, got %q", body)
	}
	if !strings.Contains(body, filepath.ToSlash(brandingRoot)) {
		t.Fatalf("expected branding replace path in config, got %q", body)
	}
}

func TestSiteConfigFilesWritesSiteConfig(t *testing.T) {
	tmp := t.TempDir()
	moduleRoot := filepath.Join(tmp, "workspace")
	if err := os.MkdirAll(moduleRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	unsafe := true
	files, err := siteConfigFiles(moduleRoot, &SiteOptions{
		Title:        "Branding",
		BaseURL:      "http://127.0.0.1:8080/",
		DisableKinds: []string{"taxonomy", "term"},
		MarkupUnsafe: &unsafe,
		Params: map[string]string{
			"brand": "cleanstartup",
		},
	})
	if err != nil {
		t.Fatalf("siteConfigFiles failed: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected one config file, got %d", len(files))
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	body := string(data)
	for _, want := range []string{"title = \"Branding\"", "baseURL = \"http://127.0.0.1:8080/\"", "disableKinds = [\"taxonomy\", \"term\"]", "[params]", "brand = \"cleanstartup\"", "[markup.goldmark.renderer]", "unsafe = true"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in config, got %q", want, body)
		}
	}
}
