package main

// Tests for video-related handlers and pure functions.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ── newVideoContextParams ─────────────────────────────────────────────────────

func TestNewVideoContextParams_NoBrandInfo(t *testing.T) {
	// Without brandInfo.Used, brandName should fall back to domain
	params := newVideoContextParams(
		"example.com",
		nil, 0, nil,
		[]string{"competitor.com"},
		[]string{"AI tools"},
		BrandContextInfo{Used: false},
		nil,
	)
	if params.Domain != "example.com" {
		t.Errorf("expected domain=example.com, got %q", params.Domain)
	}
	if params.BrandName != "example.com" {
		t.Errorf("expected brandName=example.com (fallback), got %q", params.BrandName)
	}
	if len(params.Competitors) != 1 || params.Competitors[0] != "competitor.com" {
		t.Errorf("unexpected competitors: %v", params.Competitors)
	}
	if len(params.SearchTerms) != 1 || params.SearchTerms[0] != "AI tools" {
		t.Errorf("unexpected searchTerms: %v", params.SearchTerms)
	}
}

func TestNewVideoContextParams_WithBrandInfo(t *testing.T) {
	// brandInfo.Used=true with "Company: MyBrand" line → BrandName=MyBrand
	params := newVideoContextParams(
		"example.com",
		nil, 0, nil,
		nil, nil,
		BrandContextInfo{
			Used:          true,
			ContextString: "Company: MyBrand\nSome other info",
		},
		nil,
	)
	if params.BrandName != "MyBrand" {
		t.Errorf("expected BrandName=MyBrand, got %q", params.BrandName)
	}
}

func TestNewVideoContextParams_WithBrandInfoNoCompanyLine(t *testing.T) {
	// brandInfo.Used=true but no "Company: " prefix → fall back to domain
	params := newVideoContextParams(
		"fallback.com",
		nil, 0, nil,
		nil, nil,
		BrandContextInfo{
			Used:          true,
			ContextString: "Some info without company line",
		},
		nil,
	)
	if params.BrandName != "fallback.com" {
		t.Errorf("expected BrandName=fallback.com (fallback), got %q", params.BrandName)
	}
}

func TestNewVideoContextParams_WithVideosAndAssessments(t *testing.T) {
	videos := []YouTubeVideo{
		{VideoID: "vid1", Title: "Video 1", Transcript: "Hello world"},
	}
	assessments := map[string]*VideoAssessment{
		"vid1": {VideoID: "vid1", KeywordAlignment: 75, HasTranscript: true},
	}
	params := newVideoContextParams(
		"example.com",
		videos, 3,
		[]BatchDigest{},
		nil, nil,
		BrandContextInfo{},
		assessments,
	)
	if len(params.OwnVideos) != 1 {
		t.Errorf("expected 1 own video, got %d", len(params.OwnVideos))
	}
	if params.ThirdPartyCount != 3 {
		t.Errorf("expected ThirdPartyCount=3, got %d", params.ThirdPartyCount)
	}
	if params.Assessments["vid1"] == nil {
		t.Error("expected assessment for vid1")
	}
}

// ── handleGetVideoDetails ─────────────────────────────────────────────────────

func TestHandleGetVideoDetails_NotFound(t *testing.T) {
	db := testMongoDB(t)

	handler := handleGetVideoDetails(db)
	req := testRequest(t, "GET", "/api/brand/nonexistent.com/video/details", nil)
	req.SetPathValue("domain", "nonexistent.com")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestHandleGetVideoDetails_Found(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// Seed a video analysis
	db.VideoAnalyses().InsertOne(ctx, bson.M{
		"_id":         primitive.NewObjectID(),
		"tenantId":    "test-tenant",
		"domain":      "video-details.com",
		"generatedAt": time.Now(),
		"videos": bson.A{
			bson.M{
				"videoId":    "vid-det-1",
				"title":      "Detail Video",
				"transcript": "This is a test transcript for detail",
			},
		},
		"config": bson.M{
			"searchTerms": bson.A{"AI"},
		},
	})

	handler := handleGetVideoDetails(db)
	req := testRequest(t, "GET", "/api/brand/video-details.com/video/details", nil)
	req.SetPathValue("domain", "video-details.com")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	var result []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("expected JSON array: %v; body: %s", err, w.Body.String())
	}
	if len(result) != 1 {
		t.Errorf("expected 1 video detail, got %d", len(result))
	}
}

func TestHandleGetVideoDetails_LongTranscript(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// Seed with a long transcript (>500 chars)
	longTranscript := ""
	for i := 0; i < 600; i++ {
		longTranscript += "x"
	}

	db.VideoAnalyses().InsertOne(ctx, bson.M{
		"_id":         primitive.NewObjectID(),
		"tenantId":    "test-tenant",
		"domain":      "long-transcript.com",
		"generatedAt": time.Now(),
		"videos": bson.A{
			bson.M{
				"videoId":    "vid-long-1",
				"title":      "Long Transcript Video",
				"transcript": longTranscript,
			},
		},
		"config": bson.M{
			"searchTerms": bson.A{},
		},
	})

	handler := handleGetVideoDetails(db)
	req := testRequest(t, "GET", "/api/brand/long-transcript.com/video/details", nil)
	req.SetPathValue("domain", "long-transcript.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)

	var result []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("expected JSON: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result))
	}
	// Transcript snippet should be truncated to 500 chars
	snippet, _ := result[0]["transcript"].(string)
	if len(snippet) > 500 {
		t.Errorf("expected transcript snippet <= 500 chars, got %d", len(snippet))
	}
	// transcript_length should be the full length
	fullLen, _ := result[0]["transcript_length"].(float64)
	if int(fullLen) != 600 {
		t.Errorf("expected transcript_length=600, got %v", fullLen)
	}
}

// ── handleGenerateTestQueries (with full brand data) ─────────────────────────

func TestHandleGenerateTestQueries_FullBrand(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// Seed a full brand profile that exercises all query generation paths
	db.BrandProfiles().InsertOne(ctx, BrandProfile{
		TenantID:        "test-tenant",
		Domain:          "fullbrand.com",
		BrandName:       "FullBrand",
		Description:     "A test brand",
		Categories:      []string{"SaaS", "Analytics", "Data"},
		Products:        []string{"FullProduct", "FullAnalytics"},
		PrimaryAudience: "Developers",
		KeyUseCases:     []string{"API testing", "Coverage tracking"},
		Competitors: []BrandCompetitor{
			{Name: "Competitor A", URL: "competitor-a.com"},
			{Name: "Competitor B", URL: "competitor-b.com"},
			{Name: "Competitor C", URL: "competitor-c.com"},
		},
		TargetQueries: []TargetQuery{
			{Query: "What is FullBrand?", Type: "brand", Priority: "high"},
		},
		KeyMessages: []KeyMessage{
			{Claim: "FullBrand is the best tool", Priority: "high"},
		},
		Differentiators: []string{"Unique feature X"},
		Presence: BrandPresence{
			Subreddits: []string{"r/programming"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	handler := handleGenerateTestQueries(db)
	req := testRequest(t, "POST", "/api/llmtest/queries", map[string]any{"domain": "fullbrand.com"})
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)

	var result map[string]any
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("expected JSON: %v; body: %s", err, w.Body.String())
	}
	brandName, _ := result["brand_name"].(string)
	if brandName != "FullBrand" {
		t.Errorf("expected brand_name=FullBrand, got %q", brandName)
	}
	queries, _ := result["queries"].([]any)
	if len(queries) == 0 {
		t.Error("expected non-empty queries from full brand")
	}
}

func TestHandleGenerateTestQueries_BrandNoName(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// BrandName is empty → should fall back to domain
	db.BrandProfiles().InsertOne(ctx, BrandProfile{
		TenantID:   "test-tenant",
		Domain:     "noname-brand.com",
		BrandName:  "", // empty
		Categories: []string{"SaaS"},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	})

	handler := handleGenerateTestQueries(db)
	req := testRequest(t, "POST", "/api/llmtest/queries", map[string]any{"domain": "noname-brand.com"})
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)

	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	brandName, _ := result["brand_name"].(string)
	if brandName != "noname-brand.com" {
		t.Errorf("expected brand_name=noname-brand.com, got %q", brandName)
	}
}

func TestHandleGenerateTestQueries_InvalidBody(t *testing.T) {
	db := testMongoDB(t)

	handler := handleGenerateTestQueries(db)
	req := httptest.NewRequest("POST", "/api/llmtest/queries", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for nil body, got %d", w.Code)
	}
}

// ── handleAPIv1GetScore (with data) ──────────────────────────────────────────

func TestHandleAPIv1GetScore_WithData(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// Seed an optimization with overallScore
	db.Optimizations().InsertOne(ctx, bson.M{
		"tenantId":  "test-tenant",
		"domain":    "score-with-data.com",
		"createdAt": time.Now(),
		"result": bson.M{
			"overallScore": 75,
		},
	})

	handler := handleAPIv1GetScore(db)
	req := testRequest(t, "GET", "/api/v1/domains/score-with-data.com/score", nil)
	req.SetPathValue("domain", "score-with-data.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)

	var result map[string]any
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("expected JSON: %v", err)
	}
	// At least the optimization component should be available
	available, _ := result["available"].(float64)
	if available == 0 {
		t.Error("expected at least 1 component to be available")
	}
}
