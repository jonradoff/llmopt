package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// ── Primary Provider ────────────────────────────────────────────────────

func TestHandleGetPrimaryProvider_Default(t *testing.T) {
	db := testMongoDB(t)
	handler := handleGetPrimaryProvider(db)

	req := testRequest(t, "GET", "/api/provider/primary", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["primary_provider"] != "anthropic" {
		t.Errorf("default primary provider: got %q, want %q", resp["primary_provider"], "anthropic")
	}
}

func TestHandleGetPrimaryProvider_Custom(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.TenantSettings().InsertOne(ctx, TenantSettings{
		TenantID:        "test-tenant",
		PrimaryProvider: "openai",
	})

	handler := handleGetPrimaryProvider(db)
	req := testRequest(t, "GET", "/api/provider/primary", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["primary_provider"] != "openai" {
		t.Errorf("custom primary provider: got %q, want %q", resp["primary_provider"], "openai")
	}
}

func TestHandleSetPrimaryProvider(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	tp := newTestProvider()
	tp.id = "openai"
	withTestProviders(t, tp)

	// Must have an active key for the provider
	db.TenantAPIKeys().InsertOne(ctx, TenantAPIKey{
		TenantID:  "test-tenant",
		Provider:  "openai",
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	handler := handleSetPrimaryProvider(db)
	body := map[string]string{"provider": "openai"}
	req := testRequestOwner(t, "POST", "/api/provider/primary", body)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["primary_provider"] != "openai" {
		t.Errorf("set primary provider: got %q", resp["primary_provider"])
	}

	// Verify it persisted
	var settings TenantSettings
	db.TenantSettings().FindOne(ctx, bson.M{"tenantId": "test-tenant"}).Decode(&settings)
	if settings.PrimaryProvider != "openai" {
		t.Errorf("persisted provider: got %q", settings.PrimaryProvider)
	}
}

func TestHandleSetPrimaryProvider_NotOwner(t *testing.T) {
	db := testMongoDB(t)
	handler := handleSetPrimaryProvider(db)

	body := map[string]string{"provider": "openai"}
	req := testRequest(t, "POST", "/api/provider/primary", body) // admin, not owner
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusForbidden)
}

func TestHandleSetPrimaryProvider_InvalidProvider(t *testing.T) {
	db := testMongoDB(t)
	withTestProviders(t) // empty providers map

	handler := handleSetPrimaryProvider(db)
	body := map[string]string{"provider": "nonexistent"}
	req := testRequestOwner(t, "POST", "/api/provider/primary", body)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleSetPrimaryProvider_NoKey(t *testing.T) {
	db := testMongoDB(t)

	tp := newTestProvider()
	tp.id = "openai"
	withTestProviders(t, tp)

	handler := handleSetPrimaryProvider(db)
	body := map[string]string{"provider": "openai"}
	req := testRequestOwner(t, "POST", "/api/provider/primary", body)
	w := httptest.NewRecorder()
	handler(w, req)

	// Should fail because no API key is stored
	assertStatus(t, w, http.StatusBadRequest)
}

// ── Domain Share ────────────────────────────────────────────────────────

func TestHandleGetDomainShare_NoShare(t *testing.T) {
	db := testMongoDB(t)
	handler := handleGetDomainShare(db)

	req := testRequest(t, "GET", "/api/share/example.com", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	// Should return public: false by default
	if resp["public"] == true {
		t.Error("expected public=false for non-existent share")
	}
}

func TestHandleSetDomainShare(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// Handler requires existing analysis data for the domain
	db.Analyses().InsertOne(ctx, Analysis{
		TenantID: "test-tenant",
		Domain:   "example.com",
	})

	handler := handleSetDomainShare(db)
	body := map[string]string{"visibility": "public"}
	req := testRequest(t, "PUT", "/api/share/example.com", body)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestHandleSetDomainShare_InvalidVisibility(t *testing.T) {
	db := testMongoDB(t)
	handler := handleSetDomainShare(db)

	body := map[string]string{"visibility": "invalid"}
	req := testRequest(t, "PUT", "/api/share/example.com", body)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

// ── Bulk Todo Archive ───────────────────────────────────────────────────

func TestHandleBulkArchiveTodos(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// Handler archives todos with status "todo" or "backlogged" (not "completed")
	db.Todos().InsertMany(ctx, []any{
		bson.M{"tenantId": "test-tenant", "status": "todo", "domain": "example.com", "action": "Pending 1"},
		bson.M{"tenantId": "test-tenant", "status": "backlogged", "domain": "example.com", "action": "Backlogged 1"},
		bson.M{"tenantId": "test-tenant", "status": "completed", "domain": "example.com", "action": "Completed 1"},
	})

	handler := handleBulkArchiveTodos(db)
	body := map[string]string{"domain": "example.com"}
	req := testRequest(t, "POST", "/api/todos/archive", body)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)

	// Only "todo" and "backlogged" should be archived; "completed" should remain
	archivedCount, _ := db.Todos().CountDocuments(ctx, bson.M{"tenantId": "test-tenant", "status": "archived"})
	if archivedCount != 2 {
		t.Errorf("expected 2 archived todos, got %d", archivedCount)
	}
	completedCount, _ := db.Todos().CountDocuments(ctx, bson.M{"tenantId": "test-tenant", "status": "completed"})
	if completedCount != 1 {
		t.Errorf("expected 1 completed todo to remain, got %d", completedCount)
	}
}
