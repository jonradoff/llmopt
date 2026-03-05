package main

// Tests for the spaHandler.ServeHTTP method (SPA static file serving).

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSpaHandler_ServeHTTP_FileNotFound(t *testing.T) {
	// Create temp dir with only index.html
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>SPA</html>"), 0644); err != nil {
		t.Fatalf("failed to write index.html: %v", err)
	}

	h := &spaHandler{staticPath: dir, indexPath: "index.html"}
	req := httptest.NewRequest("GET", "/nonexistent-path", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	// Should serve index.html for missing paths (SPA fallback)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for missing path (SPA fallback), got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "SPA") {
		t.Errorf("expected index.html content in response, got: %s", w.Body.String())
	}
	// index.html should have no-cache headers
	cc := w.Header().Get("Cache-Control")
	if !strings.Contains(cc, "no-cache") {
		t.Errorf("expected no-cache header for index.html, got %q", cc)
	}
}

func TestSpaHandler_ServeHTTP_FileExists(t *testing.T) {
	// Create temp dir with a static file
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>SPA</html>"), 0644); err != nil {
		t.Fatalf("failed to write index.html: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "robots.txt"), []byte("User-agent: *\nAllow: /"), 0644); err != nil {
		t.Fatalf("failed to write robots.txt: %v", err)
	}

	h := &spaHandler{staticPath: dir, indexPath: "index.html"}
	req := httptest.NewRequest("GET", "/robots.txt", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for existing file, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "User-agent") {
		t.Errorf("expected robots.txt content, got: %s", w.Body.String())
	}
}

func TestSpaHandler_ServeHTTP_AssetsCacheHeader(t *testing.T) {
	// Create temp dir with an assets directory
	dir := t.TempDir()
	assetsDir := filepath.Join(dir, "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("failed to create assets dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>SPA</html>"), 0644); err != nil {
		t.Fatalf("failed to write index.html: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "app.abc123.js"), []byte("var x=1;"), 0644); err != nil {
		t.Fatalf("failed to write asset: %v", err)
	}

	h := &spaHandler{staticPath: dir, indexPath: "index.html"}
	req := httptest.NewRequest("GET", "/assets/app.abc123.js", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for asset file, got %d", w.Code)
	}
	cc := w.Header().Get("Cache-Control")
	if !strings.Contains(cc, "max-age=31536000") {
		t.Errorf("expected long-term cache header for asset, got %q", cc)
	}
}
