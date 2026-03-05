package main

// Additional tests to push coverage on partially-covered handlers.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"llmopt/internal/saas"
)

// ── handleListVideoAnalyses — with data ──────────────────────────────────

func TestHandleListVideoAnalyses_WithData(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.VideoAnalyses().InsertMany(ctx, []any{
		bson.M{
			"tenantId":    "test-tenant",
			"domain":      "example.com",
			"model":       "test-model",
			"generatedAt": time.Now(),
			"videos": bson.A{
				bson.M{"videoId": "v1", "title": "Video 1"},
				bson.M{"videoId": "v2", "title": "Video 2"},
			},
			"result": bson.M{"overallScore": 75},
		},
		bson.M{
			"tenantId":    "test-tenant",
			"domain":      "other.com",
			"model":       "test-model",
			"generatedAt": time.Now().Add(-time.Hour),
			// No videos field — exercises video_count=0 branch
		},
	})

	handler := handleListVideoAnalyses(db)
	req := testRequest(t, "GET", "/api/video", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var summaries []map[string]any
	json.NewDecoder(w.Body).Decode(&summaries)
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}
	// First result should have video_count=2
	vc, ok := summaries[0]["video_count"]
	if !ok {
		t.Fatal("expected video_count in summary")
	}
	// JSON numbers come back as float64
	if int(vc.(float64)) != 2 {
		t.Errorf("expected video_count=2, got %v", vc)
	}
}

// ── handleListRedditAnalyses — with data ────────────────────────────────

func TestHandleListRedditAnalyses_WithData(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.RedditAnalyses().InsertMany(ctx, []any{
		bson.M{
			"tenantId":    "test-tenant",
			"domain":      "example.com",
			"model":       "test-model",
			"generatedAt": time.Now(),
			"threads": bson.A{
				bson.M{"id": "t1", "title": "Thread 1"},
				bson.M{"id": "t2", "title": "Thread 2"},
				bson.M{"id": "t3", "title": "Thread 3"},
			},
			"result": bson.M{"overallScore": 65},
		},
		bson.M{
			"tenantId":    "test-tenant",
			"domain":      "other.com",
			"model":       "test-model",
			"generatedAt": time.Now().Add(-time.Hour),
			// No threads field — exercises thread_count=0 branch
		},
	})

	handler := handleListRedditAnalyses(db)
	req := testRequest(t, "GET", "/api/reddit", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var summaries []map[string]any
	json.NewDecoder(w.Body).Decode(&summaries)
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}
	// Sorted by generatedAt desc, so example.com should be first
	if int(summaries[0]["thread_count"].(float64)) != 3 {
		t.Errorf("expected thread_count=3, got %v", summaries[0]["thread_count"])
	}
	if int(summaries[1]["thread_count"].(float64)) != 0 {
		t.Errorf("expected thread_count=0 for no-threads doc, got %v", summaries[1]["thread_count"])
	}
}

// ── handleListSearchAnalyses — with data ────────────────────────────────

func TestHandleListSearchAnalyses_WithData(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.SearchAnalyses().InsertMany(ctx, []any{
		bson.M{
			"tenantId":    "test-tenant",
			"domain":      "example.com",
			"model":       "test-model",
			"generatedAt": time.Now(),
			"result":      bson.M{"overallScore": 85},
		},
		bson.M{
			"tenantId":    "test-tenant",
			"domain":      "notscore.com",
			"model":       "test-model",
			"generatedAt": time.Now().Add(-time.Hour),
			// No result — exercises nil result branch
		},
	})

	handler := handleListSearchAnalyses(db)
	req := testRequest(t, "GET", "/api/search", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var summaries []map[string]any
	json.NewDecoder(w.Body).Decode(&summaries)
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}
	// Verify expected fields are present
	if summaries[0]["domain"] == nil {
		t.Error("expected domain field in summary")
	}
	if summaries[1]["domain"] == nil {
		t.Error("expected domain field in second summary")
	}
}

// ── handleGetCompetitorTests ─────────────────────────────────────────────

func TestHandleGetCompetitorTests_Empty(t *testing.T) {
	db := testMongoDB(t)
	handler := handleGetCompetitorTests(db)

	req := testRequest(t, "GET", "/api/llm-tests/example.com/competitors", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var tests []LLMTest
	json.NewDecoder(w.Body).Decode(&tests)
	if len(tests) != 0 {
		t.Errorf("expected 0 competitor tests, got %d", len(tests))
	}
}

func TestHandleGetCompetitorTests_WithData(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// Two competitor test runs for "comp.com" competing with "example.com"
	db.LLMTests().InsertMany(ctx, []any{
		bson.M{
			"tenantId":     "test-tenant",
			"domain":       "comp.com",
			"competitorOf": "example.com",
			"generatedAt":  time.Now(),
		},
		bson.M{
			"tenantId":     "test-tenant",
			"domain":       "comp.com",
			"competitorOf": "example.com",
			"generatedAt":  time.Now().Add(-time.Hour),
		},
	})

	handler := handleGetCompetitorTests(db)
	req := testRequest(t, "GET", "/api/llm-tests/example.com/competitors", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var tests []LLMTest
	json.NewDecoder(w.Body).Decode(&tests)
	// Should be deduped to 1 (latest per competitor domain)
	if len(tests) != 1 {
		t.Errorf("expected 1 deduplicated competitor test, got %d", len(tests))
	}
}

func TestHandleGetCompetitorTests_EmptyDomain(t *testing.T) {
	db := testMongoDB(t)
	handler := handleGetCompetitorTests(db)

	req := testRequest(t, "GET", "/api/llm-tests//competitors", nil)
	req.SetPathValue("domain", "")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

// ── handleGetLLMTestHistory edge cases ──────────────────────────────────

func TestHandleGetLLMTestHistory_Empty(t *testing.T) {
	db := testMongoDB(t)
	handler := handleGetLLMTestHistory(db)

	req := testRequest(t, "GET", "/api/llm-tests/example.com/history", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var tests []LLMTest
	json.NewDecoder(w.Body).Decode(&tests)
	if len(tests) != 0 {
		t.Errorf("expected 0 tests, got %d", len(tests))
	}
}

func TestHandleGetLLMTestHistory_EmptyDomain(t *testing.T) {
	db := testMongoDB(t)
	handler := handleGetLLMTestHistory(db)

	req := testRequest(t, "GET", "/api/llm-tests//history", nil)
	req.SetPathValue("domain", "")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

// ── handleDeleteLLMTest edge cases ───────────────────────────────────────

func TestHandleDeleteLLMTest_EmptyDomain(t *testing.T) {
	db := testMongoDB(t)
	handler := handleDeleteLLMTest(db)

	req := testRequest(t, "DELETE", "/api/llm-tests/", nil)
	req.SetPathValue("domain", "")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

// ── handleDeleteFailedAnalysis edge cases ────────────────────────────────

func TestHandleDeleteFailedAnalysis_InvalidID(t *testing.T) {
	db := testMongoDB(t)
	handler := handleDeleteFailedAnalysis(db)

	req := testRequest(t, "DELETE", "/api/failed-analyses/not-an-oid", nil)
	req.SetPathValue("id", "not-an-oid")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleDeleteFailedAnalysis_NotFound(t *testing.T) {
	db := testMongoDB(t)
	handler := handleDeleteFailedAnalysis(db)

	fakeOID := primitive.NewObjectID().Hex()
	req := testRequest(t, "DELETE", "/api/failed-analyses/"+fakeOID, nil)
	req.SetPathValue("id", fakeOID)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["deleted"].(bool) != false {
		t.Error("expected deleted=false for non-existent ID")
	}
}

func TestHandleListFailedAnalyses_WithDomainFilter(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.FailedAnalyses().InsertMany(ctx, []any{
		bson.M{"tenantId": "test-tenant", "domain": "example.com", "feedType": "site", "failedAt": time.Now()},
		bson.M{"tenantId": "test-tenant", "domain": "other.com", "feedType": "video", "failedAt": time.Now()},
	})

	handler := handleListFailedAnalyses(db)
	req := testRequest(t, "GET", "/api/failed-analyses?domain=example.com", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var results []FailedAnalysis
	json.NewDecoder(w.Body).Decode(&results)
	if len(results) != 1 {
		t.Fatalf("expected 1 result with domain filter, got %d", len(results))
	}
	if results[0].Domain != "example.com" {
		t.Errorf("expected domain=example.com, got %q", results[0].Domain)
	}
}

// ── handleDomainSummaryStatus — with existing summary ───────────────────

func TestHandleDomainSummaryStatus_Found_FreshSummary(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	now := time.Now()

	// Insert a summary with no newer data (so it's fresh)
	db.DomainSummaries().InsertOne(ctx, bson.M{
		"tenantId":    "test-tenant",
		"domain":      "example.com",
		"generatedAt": now,
		"reportCount": 3,
	})

	handler := handleDomainSummaryStatus(db)
	req := testRequest(t, "GET", "/api/summary/status/example.com", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["exists"].(bool) != true {
		t.Error("expected exists=true")
	}
}

func TestHandleDomainSummaryStatus_Found_StaleByOptimization(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	summaryTime := time.Now().Add(-time.Hour)

	db.DomainSummaries().InsertOne(ctx, bson.M{
		"tenantId":    "test-tenant",
		"domain":      "stale.com",
		"generatedAt": summaryTime,
		"reportCount": 1,
	})

	// Insert an optimization created AFTER the summary
	db.Optimizations().InsertOne(ctx, bson.M{
		"tenantId":  "test-tenant",
		"domain":    "stale.com",
		"createdAt": time.Now(),
	})

	handler := handleDomainSummaryStatus(db)
	req := testRequest(t, "GET", "/api/summary/status/stale.com", nil)
	req.SetPathValue("domain", "stale.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["exists"].(bool) != true {
		t.Error("expected exists=true")
	}
	if resp["stale"].(bool) != true {
		t.Errorf("expected stale=true, got %v", resp["stale"])
	}
}

// ── resolveOwnerName ────────────────────────────────────────────────────

func TestResolveOwnerName_InvalidHex(t *testing.T) {
	db := testMongoDB(t)
	result := resolveOwnerName(context.Background(), db, "not-a-valid-hex")
	if result != "" {
		t.Errorf("expected empty string for invalid hex, got %q", result)
	}
}

func TestResolveOwnerName_NotFound(t *testing.T) {
	db := testMongoDB(t)
	result := resolveOwnerName(context.Background(), db, primitive.NewObjectID().Hex())
	if result != "" {
		t.Errorf("expected empty string when no membership found, got %q", result)
	}
}

// ── isRootShareAdmin ─────────────────────────────────────────────────────

func TestIsRootShareAdmin_NoTenant(t *testing.T) {
	ctx := testAuthContextWithRole("tenant-1", "user-1", "admin")
	// ctx has no Tenant object, only role
	result := isRootShareAdmin(ctx)
	if result {
		t.Error("expected false when no tenant object in context")
	}
}

func TestIsRootShareAdmin_NonRootTenant(t *testing.T) {
	info := &saas.AuthInfo{
		UserID:   "u-1",
		TenantID: "t-1",
		Role:     "admin",
		Tenant:   &saas.Tenant{IsRoot: false, IsActive: true},
		Method:   "jwt",
	}
	ctx := saas.SetAuthContext(context.Background(), info)
	result := isRootShareAdmin(ctx)
	if result {
		t.Error("expected false for non-root tenant")
	}
}

func TestIsRootShareAdmin_RootTenant(t *testing.T) {
	info := &saas.AuthInfo{
		UserID:   "u-1",
		TenantID: "t-1",
		Role:     "admin",
		Tenant:   &saas.Tenant{IsRoot: true, IsActive: true},
		Method:   "jwt",
	}
	ctx := saas.SetAuthContext(context.Background(), info)
	result := isRootShareAdmin(ctx)
	if !result {
		t.Error("expected true for root tenant admin")
	}
}

func TestIsRootShareAdmin_NonAdmin(t *testing.T) {
	info := &saas.AuthInfo{
		UserID:   "u-1",
		TenantID: "t-1",
		Role:     "member",
		Tenant:   &saas.Tenant{IsRoot: true, IsActive: true},
		Method:   "jwt",
	}
	ctx := saas.SetAuthContext(context.Background(), info)
	result := isRootShareAdmin(ctx)
	if result {
		t.Error("expected false for non-admin even with root tenant")
	}
}

// ── handleGetLLMTest edge cases ──────────────────────────────────────────

func TestHandleGetLLMTest_EmptyDomain(t *testing.T) {
	db := testMongoDB(t)
	handler := handleGetLLMTest(db)

	req := testRequest(t, "GET", "/api/llm-tests/", nil)
	req.SetPathValue("domain", "")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

// ── lookupBrandContext ────────────────────────────────────────────────────────

func TestLookupBrandContext_NoBrandProfile(t *testing.T) {
	db := testMongoDB(t)

	info := lookupBrandContext(db, "nobrand.com", "test-tenant")
	if info.Used {
		t.Error("expected Used=false when no brand profile exists")
	}
	if info.ContextString != "" {
		t.Errorf("expected empty ContextString, got %q", info.ContextString)
	}
}

func TestLookupBrandContext_FullBrandProfile(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// Insert a rich brand profile covering all branches
	db.BrandProfiles().InsertOne(ctx, BrandProfile{
		TenantID:        "test-tenant",
		Domain:          "fullbrand.com",
		BrandName:       "FullBrand Inc",
		Description:     "The best brand for everything",
		Categories:      []string{"SaaS", "Analytics"},
		Products:        []string{"AnalyticsPro", "DataHub"},
		PrimaryAudience: "Enterprise data teams",
		KeyUseCases:     []string{"Data visualization", "Report automation"},
		Competitors: []BrandCompetitor{
			{Name: "Competitor Alpha", URL: "comp-alpha.com"},
			{Name: "Competitor Beta", URL: "comp-beta.com"},
		},
		Differentiators: []string{"Best-in-class UX", "30% faster than alternatives"},
		KeyMessages: []KeyMessage{
			{Claim: "Industry leader in analytics", Priority: "high", EvidenceURL: "https://fullbrand.com/report"},
			{Claim: "Loved by 10,000 teams", Priority: "medium"},
		},
		TargetQueries: []TargetQuery{
			{Query: "best analytics platform", Priority: "high", Type: "brand"},
			{Query: "how to automate reports", Priority: "medium", Type: "category"},
		},
		UpdatedAt: time.Now(),
	})

	info := lookupBrandContext(db, "fullbrand.com", "test-tenant")
	if !info.Used {
		t.Error("expected Used=true for full brand profile")
	}
	if info.ContextString == "" {
		t.Error("expected non-empty ContextString")
	}

	// Verify key fields appear in context
	for _, expected := range []string{
		"FullBrand Inc", "best brand", "SaaS", "AnalyticsPro",
		"Enterprise data teams", "Data visualization",
		"Competitor Alpha", "Best-in-class UX",
		"Industry leader", "https://fullbrand.com/report",
		"best analytics platform",
	} {
		if !containsStr(info.ContextString, expected) {
			t.Errorf("expected %q in ContextString, but not found", expected)
		}
	}
}

func TestLookupBrandContext_EmptyParts(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// Insert a minimal brand profile with only updatedAt set (no name/desc/etc.)
	db.BrandProfiles().InsertOne(ctx, BrandProfile{
		TenantID:  "test-tenant",
		Domain:    "minimalparts.com",
		UpdatedAt: time.Now(),
		// All other fields empty → len(parts) == 0 branch
	})

	info := lookupBrandContext(db, "minimalparts.com", "test-tenant")
	if info.Used {
		t.Error("expected Used=false for profile with no meaningful fields")
	}
	if info.ProfileUpdatedAt == nil {
		t.Error("expected non-nil ProfileUpdatedAt even for empty profile")
	}
}
