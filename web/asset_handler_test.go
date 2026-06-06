package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssetHandlerServesBuiltFiles(t *testing.T) {
	tmp := t.TempDir()
	assetPath := filepath.Join(tmp, "assets", "css", "site_css_4d8efec8")
	if err := os.MkdirAll(assetPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetPath, "site.css"), []byte("body{color:red}"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := assetHandler(os.DirFS(tmp), "assets")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/assets/css/site_css_4d8efec8/site.css", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "body{color:red}") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}
