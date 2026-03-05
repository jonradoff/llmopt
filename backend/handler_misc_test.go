package main

// Tests for miscellaneous handlers at 0% coverage.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ── handleServeBrandScreenshot ───────────────────────────────────────────────

func TestHandleServeBrandScreenshot_NotFound(t *testing.T) {
	db := testMongoDB(t)

	handler := handleServeBrandScreenshot(db)
	req := httptest.NewRequest("GET", "/api/share/popular/nonexistent.com/screenshot", nil)
	req.SetPathValue("domain", "nonexistent.com")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestHandleServeBrandScreenshot_Found(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	imgData := []byte{0x89, 0x50, 0x4E, 0x47} // PNG magic bytes
	db.BrandScreenshots().InsertOne(ctx, BrandScreenshot{
		ID:          primitive.NewObjectID(),
		TenantID:    "test-tenant",
		Domain:      "screenshot-found.com",
		ImageData:   imgData,
		ContentType: "image/png",
		Width:       1200,
		Height:      630,
		SizeBytes:   len(imgData),
		CapturedAt:  time.Now(),
	})

	handler := handleServeBrandScreenshot(db)
	req := httptest.NewRequest("GET", "/api/share/popular/screenshot-found.com/screenshot", nil)
	req.SetPathValue("domain", "screenshot-found.com")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Content-Type") != "image/png" {
		t.Errorf("expected Content-Type: image/png, got %q", w.Header().Get("Content-Type"))
	}
}

// ── handleShareOG ────────────────────────────────────────────────────────────

func TestHandleShareOG_NonCrawlerBrowser(t *testing.T) {
	// Non-crawler browsers get redirected to SPA (index.html)
	// http.ServeFile will 404 because there's no actual index.html, which is OK
	db := testMongoDB(t)

	handler := handleShareOG(db, "/nonexistent/static/dir")
	req := httptest.NewRequest("GET", "/s/test-share-id", nil)
	req.SetPathValue("shareId", "test-share-id")
	// Browser user agent (not a crawler)
	req.Header.Set("User-Agent", "Mozilla/5.0 Chrome/100.0")
	w := httptest.NewRecorder()
	handler(w, req)

	// Should try to serve index.html (will get 404 since dir doesn't exist, that's OK)
	// The important thing is it tries to serve file, NOT the OG HTML
	body := w.Body.String()
	if strings.Contains(body, "<meta property=\"og:type\"") {
		t.Error("expected SPA serving for non-crawler, not OG meta tags")
	}
}

func TestHandleShareOG_CrawlerNotFound(t *testing.T) {
	// Crawler with unknown shareId → serves SPA (index.html 404)
	db := testMongoDB(t)

	handler := handleShareOG(db, "/nonexistent/static/dir")
	req := httptest.NewRequest("GET", "/s/unknown-share", nil)
	req.SetPathValue("shareId", "unknown-share-xyz-404")
	req.Header.Set("User-Agent", "facebookexternalhit/1.1")
	w := httptest.NewRecorder()
	handler(w, req)

	// Should try to serve SPA (not found → ServeFile → 404)
	body := w.Body.String()
	if strings.Contains(body, "<meta property=\"og:type\"") {
		t.Error("expected SPA fallback for unknown shareId, not OG meta tags")
	}
}

func TestHandleShareOG_CrawlerFound(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	shareID := "og-test-share-abc"
	db.DomainShares().InsertOne(ctx, bson.M{
		"_id":        primitive.NewObjectID(),
		"tenantId":   "test-tenant",
		"domain":     "og-example.com",
		"shareId":    shareID,
		"visibility": "public",
		"createdAt":  time.Now(),
	})

	handler := handleShareOG(db, "/nonexistent/static/dir")
	req := httptest.NewRequest("GET", "/s/"+shareID, nil)
	req.SetPathValue("shareId", shareID)
	req.Header.Set("User-Agent", "facebookexternalhit/1.1")
	w := httptest.NewRecorder()
	handler(w, req)

	// Crawler gets OG HTML back
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OG response, got %d; body: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "og:type") {
		t.Errorf("expected OG meta tags in response, got: %s", body[:minLen(200, len(body))])
	}
	if w.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("expected text/html content-type, got %q", w.Header().Get("Content-Type"))
	}
}

func TestHandleShareOG_CrawlerFoundPopularWithBrandName(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	shareID := "og-popular-brand-xyz"
	// Seed the share
	db.DomainShares().InsertOne(ctx, bson.M{
		"_id":        primitive.NewObjectID(),
		"tenantId":   "test-tenant",
		"domain":     "popular-brand.com",
		"shareId":    shareID,
		"visibility": "popular",
		"createdAt":  time.Now(),
	})
	// Seed a brand profile to get the brand name
	db.BrandProfiles().InsertOne(ctx, BrandProfile{
		TenantID:  "test-tenant",
		Domain:    "popular-brand.com",
		BrandName: "PopularBrand",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	handler := handleShareOG(db, "/nonexistent/static/dir")
	req := httptest.NewRequest("GET", "/s/"+shareID, nil)
	req.SetPathValue("shareId", shareID)
	req.Header.Set("User-Agent", "twitterbot/1.0")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	// OG HTML should be returned with meta tags
	if !strings.Contains(body, "og:type") {
		t.Errorf("expected OG meta tags in popular OG response, got: %s", body[:minLen(400, len(body))])
	}
	// Either the brand name or domain should appear in the page
	if !strings.Contains(body, "PopularBrand") && !strings.Contains(body, "popular-brand.com") {
		t.Errorf("expected brand/domain in OG response, got: %s", body[:minLen(200, len(body))])
	}
}

func minLen(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── handleDeleteVideoAnalysis ────────────────────────────────────────────────

func TestHandleDeleteVideoAnalysis_NothingToDelete(t *testing.T) {
	// Handler deletes by domain (no ID param used), returns deleted:false when not found
	db := testMongoDB(t)

	handler := handleDeleteVideoAnalysis(db)
	req := testRequest(t, "DELETE", "/api/brand/delvid-none.com/video", nil)
	req.SetPathValue("domain", "delvid-none.com")
	w := httptest.NewRecorder()
	handler(w, req)

	// Handler returns 200 with deleted:false when nothing matched
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestHandleDeleteVideoAnalysis_Success(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.VideoAnalyses().InsertOne(ctx, bson.M{
		"tenantId":    "test-tenant",
		"domain":      "delvid-ok.com",
		"generatedAt": time.Now(),
		"config":      bson.M{},
	})

	handler := handleDeleteVideoAnalysis(db)
	req := testRequest(t, "DELETE", "/api/brand/delvid-ok.com/video", nil)
	req.SetPathValue("domain", "delvid-ok.com")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
}
