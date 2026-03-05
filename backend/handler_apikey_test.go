package main

// Additional API key handler tests to improve coverage on missing branches.

import (
	"net/http/httptest"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// ── handleSetAPIKey (missing branches) ───────────────────────────────────────

func TestHandleSetAPIKey_Forbidden(t *testing.T) {
	db := testMongoDB(t)
	handler := handleSetAPIKey(db, testEncKey())

	// testRequest uses "admin" role, not "owner" → should be forbidden
	req := testRequest(t, "PUT", "/api/keys/anthropic", map[string]string{"key": "sk-ant-test"})
	req.SetPathValue("provider", "anthropic")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, 403)
}

func TestHandleSetAPIKey_NoKeyNoModel(t *testing.T) {
	// Empty key and empty preferred_model → 400
	db := testMongoDB(t)
	handler := handleSetAPIKey(db, testEncKey())

	req := testRequestOwner(t, "PUT", "/api/keys/anthropic", map[string]string{
		"key":             "",
		"preferred_model": "",
	})
	req.SetPathValue("provider", "anthropic")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, 400)
}

func TestHandleSetAPIKey_UpdatePreferredModel(t *testing.T) {
	// Empty key + preferred_model → update model on existing key
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// First seed a key
	db.TenantAPIKeys().InsertOne(ctx, bson.M{
		"tenantId":       "test-tenant",
		"provider":       "anthropic",
		"encryptedKey":   "some-encrypted-value",
		"keyPrefix":      "sk-ant-...",
		"status":         "active",
		"preferredModel": "claude-3-sonnet",
		"createdAt":      time.Now(),
		"updatedAt":      time.Now(),
	})

	handler := handleSetAPIKey(db, testEncKey())
	req := testRequestOwner(t, "PUT", "/api/keys/anthropic", map[string]any{
		"key":             "",
		"preferred_model": "claude-opus-4",
	})
	req.SetPathValue("provider", "anthropic")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, 200)
}

func TestHandleSetAPIKey_UpdatePreferredModel_NoExistingKey(t *testing.T) {
	// Empty key + preferred_model but no existing key → 400
	db := testMongoDB(t)
	handler := handleSetAPIKey(db, testEncKey())

	req := testRequestOwner(t, "PUT", "/api/keys/openai", map[string]any{
		"key":             "",
		"preferred_model": "gpt-4",
	})
	req.SetPathValue("provider", "openai")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, 400)
}

func TestHandleSetAPIKey_YouTubeProvider(t *testing.T) {
	// YouTube provider → verifyYouTubeKey path (no LLM provider lookup)
	db := testMongoDB(t)
	handler := handleSetAPIKey(db, testEncKey())

	req := testRequestOwner(t, "PUT", "/api/keys/youtube", map[string]string{
		"key": "AIzaSyTest12345678",
	})
	req.SetPathValue("provider", "youtube")
	w := httptest.NewRecorder()
	handler(w, req)

	// Should succeed (YouTube key verification is best-effort)
	assertStatus(t, w, 200)
}

// ── handleDeleteAPIKey (missing branches) ─────────────────────────────────────

func TestHandleDeleteAPIKey_Forbidden(t *testing.T) {
	db := testMongoDB(t)
	handler := handleDeleteAPIKey(db)

	// testRequest uses "admin" role, not "owner" → forbidden
	req := testRequest(t, "DELETE", "/api/keys/anthropic", nil)
	req.SetPathValue("provider", "anthropic")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, 403)
}

func TestHandleDeleteAPIKey_NotFound(t *testing.T) {
	// Deleting a key that doesn't exist → 404
	db := testMongoDB(t)
	handler := handleDeleteAPIKey(db)

	req := testRequestOwner(t, "DELETE", "/api/keys/openai", nil)
	req.SetPathValue("provider", "openai")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, 404)
}

// ── handleListAPIKeys (additional coverage) ───────────────────────────────────

func TestHandleListAPIKeys_AdminCanList(t *testing.T) {
	// handleListAPIKeys has no role restriction - admin can list
	db := testMongoDB(t)
	handler := handleListAPIKeys(db)

	req := testRequest(t, "GET", "/api/keys", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, 200)
}
