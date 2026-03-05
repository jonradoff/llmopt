package main

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// ── resolveProviderKey ──────────────────────────────────────────────────

func TestResolveProviderKey_NonSaaS_Anthropic(t *testing.T) {
	key, err := resolveProviderKey(context.Background(), nil, nil, "fallback-key", false, "anthropic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "fallback-key" {
		t.Errorf("expected fallback-key, got %q", key)
	}
}

func TestResolveProviderKey_NonSaaS_OtherProvider(t *testing.T) {
	_, err := resolveProviderKey(context.Background(), nil, nil, "fallback-key", false, "openai")
	if err == nil {
		t.Fatal("expected error for non-anthropic provider in non-SaaS mode")
	}
}

func TestResolveProviderKey_SaaS_NoTenant(t *testing.T) {
	db := testMongoDB(t)
	_, err := resolveProviderKey(context.Background(), db, testEncKey(), "", true, "anthropic")
	if err == nil {
		t.Fatal("expected error with no tenant context")
	}
}

func TestResolveProviderKey_SaaS_NoKey(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("tenant-1", "user-1")
	_, err := resolveProviderKey(ctx, db, testEncKey(), "", true, "anthropic")
	if err == nil {
		t.Fatal("expected error when no API key exists")
	}
}

func TestResolveProviderKey_SaaS_WithKey(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("tenant-1", "user-1")
	encKey := testEncKey()

	// Encrypt and store a key
	encrypted, err := encryptSecret("sk-test-api-key", encKey)
	if err != nil {
		t.Fatalf("encryptSecret failed: %v", err)
	}
	db.TenantAPIKeys().InsertOne(ctx, TenantAPIKey{
		TenantID:     "tenant-1",
		Provider:     "anthropic",
		EncryptedKey: encrypted,
		KeyPrefix:    "sk-test",
		Status:       "active",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	})

	key, err := resolveProviderKey(ctx, db, encKey, "", true, "anthropic")
	if err != nil {
		t.Fatalf("resolveProviderKey failed: %v", err)
	}
	if key != "sk-test-api-key" {
		t.Errorf("expected 'sk-test-api-key', got %q", key)
	}
}

// ── resolvePrimaryLLM ───────────────────────────────────────────────────

func TestResolvePrimaryLLM_NonSaaS(t *testing.T) {
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	provider, key, model, err := resolvePrimaryLLM(context.Background(), nil, nil, "fallback-key", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider == nil {
		t.Fatal("provider should not be nil")
	}
	if key != "fallback-key" {
		t.Errorf("key: got %q, want %q", key, "fallback-key")
	}
	if model != "" {
		t.Errorf("model should be empty in non-SaaS mode, got %q", model)
	}
}

func TestResolvePrimaryLLM_SaaS_DefaultProvider(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("tenant-1", "user-1")
	encKey := testEncKey()

	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	// Store encrypted key for anthropic
	encrypted, _ := encryptSecret("sk-ant-key", encKey)
	db.TenantAPIKeys().InsertOne(ctx, TenantAPIKey{
		TenantID:     "tenant-1",
		Provider:     "anthropic",
		EncryptedKey: encrypted,
		Status:       "active",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	})

	provider, key, _, err := resolvePrimaryLLM(ctx, db, encKey, "", true)
	if err != nil {
		t.Fatalf("resolvePrimaryLLM failed: %v", err)
	}
	if provider.ProviderID() != "anthropic" {
		t.Errorf("provider: got %q, want %q", provider.ProviderID(), "anthropic")
	}
	if key != "sk-ant-key" {
		t.Errorf("key: got %q", key)
	}
}

func TestResolvePrimaryLLM_SaaS_CustomProvider(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("tenant-1", "user-1")
	encKey := testEncKey()

	tp := newTestProvider()
	tp.id = "openai"
	withTestProviders(t, tp)

	// Set primary provider to openai
	db.TenantSettings().InsertOne(ctx, TenantSettings{
		TenantID:        "tenant-1",
		PrimaryProvider: "openai",
	})

	// Store encrypted key for openai
	encrypted, _ := encryptSecret("sk-openai-key", encKey)
	db.TenantAPIKeys().InsertOne(ctx, TenantAPIKey{
		TenantID:     "tenant-1",
		Provider:     "openai",
		EncryptedKey: encrypted,
		Status:       "active",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	})

	provider, key, _, err := resolvePrimaryLLM(ctx, db, encKey, "", true)
	if err != nil {
		t.Fatalf("resolvePrimaryLLM failed: %v", err)
	}
	if provider.ProviderID() != "openai" {
		t.Errorf("provider: got %q, want %q", provider.ProviderID(), "openai")
	}
	if key != "sk-openai-key" {
		t.Errorf("key: got %q", key)
	}
}

func TestResolvePrimaryLLM_SaaS_NoKey(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("tenant-1", "user-1")

	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	_, _, _, err := resolvePrimaryLLM(ctx, db, testEncKey(), "", true)
	if err == nil {
		t.Fatal("expected error when no API key is stored")
	}
}

// ── resolveYouTubeKey ───────────────────────────────────────────────────

func TestResolveYouTubeKey_NonSaaS_WithKey(t *testing.T) {
	key, err := resolveYouTubeKey(context.Background(), nil, nil, "yt-system-key", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "yt-system-key" {
		t.Errorf("expected 'yt-system-key', got %q", key)
	}
}

func TestResolveYouTubeKey_NonSaaS_NoKey(t *testing.T) {
	_, err := resolveYouTubeKey(context.Background(), nil, nil, "", false)
	if err == nil {
		t.Fatal("expected error when no YouTube key is available")
	}
}

func TestResolveYouTubeKey_SaaS(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("tenant-1", "user-1")
	encKey := testEncKey()

	// Store encrypted YouTube key
	encrypted, _ := encryptSecret("yt-tenant-key", encKey)
	db.TenantAPIKeys().InsertOne(ctx, TenantAPIKey{
		TenantID:     "tenant-1",
		Provider:     "youtube",
		EncryptedKey: encrypted,
		Status:       "active",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	})

	key, err := resolveYouTubeKey(ctx, db, encKey, "", true)
	if err != nil {
		t.Fatalf("resolveYouTubeKey failed: %v", err)
	}
	if key != "yt-tenant-key" {
		t.Errorf("expected 'yt-tenant-key', got %q", key)
	}
}

// ── getProvider ─────────────────────────────────────────────────────────

func TestGetProvider_Found(t *testing.T) {
	tp := newTestProvider()
	tp.id = "my-provider"
	withTestProviders(t, tp)

	got := getProvider("my-provider")
	if got == nil {
		t.Fatal("expected non-nil provider")
	}
	if got.ProviderID() != "my-provider" {
		t.Errorf("provider ID: got %q", got.ProviderID())
	}
}

func TestGetProvider_NotFound(t *testing.T) {
	withTestProviders(t) // empty providers map

	got := getProvider("nonexistent")
	if got != nil {
		t.Error("expected nil for missing provider")
	}
}

// ── lookupBrandContext ──────────────────────────────────────────────────

func TestLookupBrandContext_NoBrand(t *testing.T) {
	db := testMongoDB(t)
	info := lookupBrandContext(db, "nonexistent.com", "tenant-1")
	if info.Used {
		t.Error("expected Used=false for missing brand")
	}
	if info.ContextString != "" {
		t.Errorf("expected empty ContextString, got %q", info.ContextString)
	}
}

func TestLookupBrandContext_WithBrand(t *testing.T) {
	db := testMongoDB(t)
	dbCtx := context.Background()

	db.BrandProfiles().InsertOne(dbCtx, BrandProfile{
		TenantID:    "tenant-1",
		Domain:      "example.com",
		BrandName:   "Example Corp",
		Description: "A test company",
		Categories:  []string{"tech", "ai"},
	})

	info := lookupBrandContext(db, "example.com", "tenant-1")
	if !info.Used {
		t.Error("expected Used=true when brand exists")
	}
	if info.ContextString == "" {
		t.Fatal("expected non-empty ContextString")
	}
}


// ── stripJSONFencing ────────────────────────────────────────────────────

func TestStripJSONFencing(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{"no_fencing", `{"key":"value"}`, `{"key":"value"}`},
		{"backtick_json", "```json\n{\"key\":\"value\"}\n```", `{"key":"value"}`},
		{"backtick_plain", "```\n{\"key\":\"value\"}\n```", `{"key":"value"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripJSONFencing(tt.input)
			if got != tt.want {
				t.Errorf("stripJSONFencing(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ── trackServerEvent ────────────────────────────────────────────────────

func TestTrackServerEvent(t *testing.T) {
	db := testMongoDB(t)
	trackServerEvent(db, "test_event", "", "", map[string]any{"key": "value"})

	// trackServerEvent runs in a goroutine, give it time to complete
	time.Sleep(500 * time.Millisecond)

	ctx := context.Background()
	coll := db.Database.Collection("telemetry_events")
	count, _ := coll.CountDocuments(ctx, bson.M{"eventName": "test_event"})
	if count != 1 {
		t.Errorf("expected 1 telemetry event, got %d", count)
	}
}
