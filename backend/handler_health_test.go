package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// ── handleHealthCheck ────────────────────────────────────────────────────────

func TestHandleHealthCheck_Success(t *testing.T) {
	// Mock Anthropic API returning 200 for both models
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "ok"},
			},
		})
	}))
	defer server.Close()

	withAllProviderBases(t, server)
	withTestHTTPClients(t, server)

	handler := handleHealthCheck("test-api-key", nil)
	req := httptest.NewRequest("GET", "/api/health/check", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var result map[string]any
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if _, ok := result["models"]; !ok {
		t.Error("expected 'models' key in response")
	}
	if _, ok := result["checked_at"]; !ok {
		t.Error("expected 'checked_at' key in response")
	}
}

func TestHandleHealthCheck_Overloaded(t *testing.T) {
	// Mock Anthropic returning 529 overloaded
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(529)
	}))
	defer server.Close()

	withAllProviderBases(t, server)
	withTestHTTPClients(t, server)

	handler := handleHealthCheck("test-api-key", nil)
	req := httptest.NewRequest("GET", "/api/health/check", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	models, ok := result["models"].([]any)
	if !ok {
		t.Fatal("expected models array")
	}
	if len(models) != 2 {
		t.Errorf("expected 2 model results, got %d", len(models))
	}
}

func TestHandleHealthCheck_APIError(t *testing.T) {
	// Mock Anthropic returning 401
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer server.Close()

	withAllProviderBases(t, server)
	withTestHTTPClients(t, server)

	handler := handleHealthCheck("bad-key", nil)
	req := httptest.NewRequest("GET", "/api/health/check", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	// Should still return 200 with error status in models
	if _, ok := result["models"]; !ok {
		t.Error("expected 'models' key even on API error")
	}
}

// ── handleProviderHealthCheck ─────────────────────────────────────────────────

func TestHandleProviderHealthCheck_NoKeys(t *testing.T) {
	db := testMongoDB(t)

	handler := handleProviderHealthCheck(db, testEncKey())
	req := testRequest(t, "GET", "/api/health/providers", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	models, ok := result["models"].([]any)
	if !ok {
		t.Fatal("expected models array")
	}
	if len(models) != 0 {
		t.Errorf("expected empty models for tenant with no keys, got %d", len(models))
	}
}

func TestHandleProviderHealthCheck_WithAnthropicKey(t *testing.T) {
	db := testMongoDB(t)

	// Insert an encrypted Anthropic key for the test tenant
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	encKey := testEncKey()
	encryptedKey, err := encryptSecret("test-anthropic-key", encKey)
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}
	db.TenantAPIKeys().InsertOne(ctx, bson.M{
		"tenantId":       "test-tenant",
		"provider":       "anthropic",
		"encryptedKey":   encryptedKey,
		"status":         "active",
		"preferredModel": "claude-haiku-4-5-20251001",
		"createdAt":      time.Now(),
	})

	// Mock the provider API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200) // active
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	handler := handleProviderHealthCheck(db, encKey)
	req := testRequest(t, "GET", "/api/health/providers", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	models, ok := result["models"].([]any)
	if !ok {
		t.Fatal("expected models array")
	}
	if len(models) != 1 {
		t.Errorf("expected 1 model result, got %d", len(models))
	}
}

func TestHandleProviderHealthCheck_WithYouTubeKey(t *testing.T) {
	db := testMongoDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	encKey := testEncKey()
	encryptedKey, _ := encryptSecret("test-yt-key", encKey)
	db.TenantAPIKeys().InsertOne(ctx, bson.M{
		"tenantId":     "test-tenant",
		"provider":     "youtube",
		"encryptedKey": encryptedKey,
		"status":       "active",
		"createdAt":    time.Now(),
	})

	handler := handleProviderHealthCheck(db, encKey)
	req := testRequest(t, "GET", "/api/health/providers", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	models, ok := result["models"].([]any)
	if !ok {
		t.Fatal("expected models array")
	}
	// Should have youtube entry
	if len(models) == 0 {
		t.Error("expected at least 1 model result for youtube key")
	}
}

// ── handleVerifyAPIKey ────────────────────────────────────────────────────────

func TestHandleVerifyAPIKey_Forbidden(t *testing.T) {
	db := testMongoDB(t)

	handler := handleVerifyAPIKey(db, testEncKey())
	// Use non-owner role (default testRequest uses "admin" which may not have owner access)
	req := testRequest(t, "POST", "/api/keys/anthropic/verify", nil)
	req.SetPathValue("provider", "anthropic")
	// testAuthContext uses "admin" role, not "owner"
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusForbidden)
}

func TestHandleVerifyAPIKey_KeyNotFound(t *testing.T) {
	db := testMongoDB(t)

	handler := handleVerifyAPIKey(db, testEncKey())
	req := testRequestOwner(t, "POST", "/api/keys/anthropic/verify", nil)
	req.SetPathValue("provider", "anthropic")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusNotFound)
}

func TestHandleVerifyAPIKey_Success(t *testing.T) {
	db := testMongoDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	encKey := testEncKey()
	encryptedKey, _ := encryptSecret("test-key", encKey)
	db.TenantAPIKeys().InsertOne(ctx, bson.M{
		"tenantId":     "test-tenant",
		"provider":     "anthropic",
		"encryptedKey": encryptedKey,
		"status":       "active",
		"createdAt":    time.Now(),
	})

	// Mock Anthropic returning success
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer server.Close()
	withAllProviderBases(t, server)

	handler := handleVerifyAPIKey(db, encKey)
	req := testRequestOwner(t, "POST", "/api/keys/anthropic/verify", nil)
	req.SetPathValue("provider", "anthropic")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	if result["status"] != "active" {
		t.Errorf("expected status 'active', got %v", result["status"])
	}
}

func TestHandleVerifyAPIKey_UnknownProvider(t *testing.T) {
	db := testMongoDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	encKey := testEncKey()
	encryptedKey, _ := encryptSecret("test-key", encKey)
	db.TenantAPIKeys().InsertOne(ctx, bson.M{
		"tenantId":     "test-tenant",
		"provider":     "unknown-provider",
		"encryptedKey": encryptedKey,
		"status":       "active",
		"createdAt":    time.Now(),
	})

	handler := handleVerifyAPIKey(db, encKey)
	req := testRequestOwner(t, "POST", "/api/keys/unknown-provider/verify", nil)
	req.SetPathValue("provider", "unknown-provider")
	w := httptest.NewRecorder()
	handler(w, req)

	// Unknown provider without youtube → falls through to status "active"
	assertStatus(t, w, http.StatusOK)
}
