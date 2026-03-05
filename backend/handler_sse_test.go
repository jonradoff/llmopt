package main

// Tests for SSE streaming handlers (handleAnalyze, handleSearchAnalyze,
// handleRedditAnalyze, handleGenerateDomainSummary, handleLLMTest).
// All tests run in non-SaaS mode (saasEnabled=false) so that resolvePrimaryLLM
// uses the fallbackKey directly without touching the DB.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// validSearchVisibilityResultJSON is a minimal valid SearchVisibilityResult.
const validSearchVisibilityResultJSON = `{
	"overall_score": 72,
	"aio_readiness": {
		"score": 70,
		"evidence": ["Good structured content"],
		"organic_presence": 60,
		"structured_data": 55,
		"content_format": 65,
		"answer_prominence": 58
	},
	"crawl_accessibility": {
		"score": 75,
		"evidence": ["Allows major AI crawlers"],
		"robots_txt_policy": "allows all",
		"ai_bot_access": 80,
		"sitemap_quality": 70,
		"render_accessibility": 75,
		"crawler_details": []
	},
	"brand_momentum": {
		"score": 68,
		"evidence": ["Growing brand mentions"],
		"brand_search_trend": "growing",
		"competitor_compare": "Ahead of mid-tier competitors",
		"web_mention_strength": 60,
		"entity_recognition": 55
	},
	"content_freshness": {
		"score": 65,
		"evidence": ["Recent updates detected"],
		"average_content_age": "6-12 months",
		"update_frequency": "moderate",
		"freshness_signals": 60,
		"content_decay_risk": 40
	},
	"executive_summary": "Test domain shows moderate AI visibility.",
	"confidence_note": "High confidence based on available signals.",
	"recommendations": []
}`

// validRedditAuthorityResultJSON is a minimal valid RedditAuthorityResult.
const validRedditAuthorityResultJSON = `{
	"overall_score": 65,
	"presence": {
		"score": 60,
		"evidence": ["Mentioned in 5 threads"],
		"total_mentions": 5,
		"unique_subreddits": 2,
		"share_of_voice": [],
		"mention_trend": "stable"
	},
	"sentiment": {
		"score": 70,
		"evidence": ["Mostly positive mentions"],
		"sentiment": {"positive": 60, "neutral": 30, "negative": 10},
		"recommendation_rate": 40,
		"top_praise": ["Good product"],
		"top_criticism": [],
		"notable_mentions": []
	},
	"competitive": {
		"score": 55,
		"evidence": ["Mentioned alongside competitors"],
		"win_rate": 45,
		"comparison_threads": 2,
		"differentiators": ["Ease of use"],
		"competitor_strengths": [],
		"head_to_head_examples": []
	},
	"training_signal": {
		"score": 70,
		"evidence": ["High-score threads present"],
		"high_score_threads": 3,
		"deep_threads": 2,
		"authority_tier": "moderate",
		"key_threads": [],
		"recommendations": []
	},
	"executive_summary": "Moderate Reddit authority detected.",
	"confidence_note": "Based on limited thread sample.",
	"recommendations": []
}`

// validDomainSummaryResultJSON is a minimal valid DomainSummaryResult.
const validDomainSummaryResultJSON = `{
	"executive_summary": "This domain has moderate LLM visibility across all analyzed dimensions.",
	"average_score": 68,
	"score_range": [55, 82],
	"themes": [],
	"action_items": [],
	"contradictions": [],
	"dimension_trends": {}
}`

// validLLMTestEvalResultJSON is valid for the eval JSON parsed in handleLLMTest.
const validLLMTestEvalResultJSON = `{
	"evaluations": [
		{
			"query_index": 0,
			"provider_id": "test",
			"mentioned": true,
			"recommended": true,
			"sentiment": "positive",
			"accuracy": "accurate",
			"score": 80
		}
	]
}`

// ── handleAnalyze ────────────────────────────────────────────────────────

func TestHandleAnalyze_MissingURL(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleAnalyze(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "POST", "/api/analyze", map[string]any{})
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatal("expected error SSE event for missing URL")
	}
}

func TestHandleAnalyze_InvalidBody(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleAnalyze(db, testEncKey(), "fake-key", false)
	req := testRequestNoAuth(t, "POST", "/api/analyze", nil)
	req = req.WithContext(testAuthContext("test-tenant", "test-user"))
	// Overwrite body with invalid JSON
	req.Body = http.NoBody
	req.ContentLength = 0
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatal("expected error SSE event for invalid body")
	}
}

func TestHandleAnalyze_Success(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	// Default Stream returns valid AnalysisResult JSON
	withTestProviders(t, tp)

	handler := handleAnalyze(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "POST", "/api/analyze", map[string]any{
		"url":   "https://example.com",
		"force": true,
	})
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	doneEvent := findSSEEvent(events, "done")
	if doneEvent == nil {
		t.Fatalf("expected done SSE event; got events: %v", events)
	}
}

func TestHandleAnalyze_CachedResult(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// Insert a fresh cached analysis
	db.Analyses().InsertOne(ctx, bson.M{
		"tenantId":  "test-tenant",
		"domain":    "cached.com",
		"model":     "test-model",
		"createdAt": time.Now(),
		"result": bson.M{
			"siteSummary": "Cached summary",
			"questions":   bson.A{},
			"crawledPages": bson.A{},
		},
		"rawText": "",
	})

	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleAnalyze(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "POST", "/api/analyze", map[string]any{
		"url":   "cached.com",
		"force": false, // don't force — use cache
	})
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	doneEvent := findSSEEvent(events, "done")
	if doneEvent == nil {
		t.Fatal("expected done SSE event for cached result")
	}
	// Verify cached=true in the done payload
	if !strings.Contains(string(doneEvent.Data), "cached") {
		t.Errorf("expected 'cached' in done event data, got: %s", doneEvent.Data)
	}
}

// ── handleSearchAnalyze ──────────────────────────────────────────────────

func TestHandleSearchAnalyze_MissingDomain(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleSearchAnalyze(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "POST", "/api/search/analyze", map[string]any{})
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatal("expected error SSE event for missing domain")
	}
}

func TestHandleSearchAnalyze_Success(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	tp.StreamFn = func(ctx context.Context, apiKey string, body []byte, w http.ResponseWriter, flusher http.Flusher) (*StreamResult, error) {
		sendSSE(w, flusher, "text", map[string]string{"content": "analyzing..."})
		return &StreamResult{RawText: validSearchVisibilityResultJSON, ResultJSON: validSearchVisibilityResultJSON}, nil
	}
	withTestProviders(t, tp)

	handler := handleSearchAnalyze(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "POST", "/api/search/analyze", map[string]any{
		"domain": "example.com",
		"force":  true,
	})
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	doneEvent := findSSEEvent(events, "done")
	if doneEvent == nil {
		t.Fatalf("expected done SSE event; got events: %v", events)
	}
}

func TestHandleSearchAnalyze_CachedResult(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// Insert a fresh cached search analysis
	db.SearchAnalyses().InsertOne(ctx, bson.M{
		"tenantId":    "test-tenant",
		"domain":      "searchcached.com",
		"model":       "test-model",
		"generatedAt": time.Now(),
		"result": bson.M{
			"overallScore":     70,
			"executiveSummary": "Cached",
			"recommendations":  bson.A{},
		},
	})

	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleSearchAnalyze(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "POST", "/api/search/analyze", map[string]any{
		"domain": "searchcached.com",
		"force":  false,
	})
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	doneEvent := findSSEEvent(events, "done")
	if doneEvent == nil {
		t.Fatal("expected done SSE event for cached search result")
	}
	if !strings.Contains(string(doneEvent.Data), "cached") {
		t.Errorf("expected 'cached' in done data, got: %s", doneEvent.Data)
	}
}

func TestHandleSearchAnalyze_ParseError(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	tp.StreamFn = func(ctx context.Context, apiKey string, body []byte, w http.ResponseWriter, flusher http.Flusher) (*StreamResult, error) {
		// Return invalid JSON — handler should send error SSE
		return &StreamResult{RawText: "not json", ResultJSON: "not json"}, nil
	}
	withTestProviders(t, tp)

	handler := handleSearchAnalyze(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "POST", "/api/search/analyze", map[string]any{
		"domain": "parseerror.com",
		"force":  true,
	})
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatal("expected error SSE event for parse failure")
	}
}

// ── handleRedditAnalyze ──────────────────────────────────────────────────

func TestHandleRedditAnalyze_NoThreads(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleRedditAnalyze(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "POST", "/api/reddit/analyze", map[string]any{
		"domain":  "example.com",
		"threads": []any{}, // empty threads → error
	})
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatal("expected error SSE event for empty threads")
	}
}

func TestHandleRedditAnalyze_Success(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	tp.StreamFn = func(ctx context.Context, apiKey string, body []byte, w http.ResponseWriter, flusher http.Flusher) (*StreamResult, error) {
		sendSSE(w, flusher, "text", map[string]string{"content": "analyzing reddit..."})
		return &StreamResult{RawText: validRedditAuthorityResultJSON, ResultJSON: validRedditAuthorityResultJSON}, nil
	}
	withTestProviders(t, tp)

	// Set up a test server that returns 404 for Reddit thread fetch (graceful fallback)
	redditServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer redditServer.Close()
	withTestRedditBase(t, redditServer)

	handler := handleRedditAnalyze(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "POST", "/api/reddit/analyze", map[string]any{
		"domain": "example.com",
		"threads": []map[string]any{
			{
				"id":          "abc123",
				"subreddit":   "testsubreddit",
				"title":       "Example thread",
				"self_text":   "Some content",
				"author":      "testuser",
				"score":       100,
				"num_comments": 5,
				"permalink":   "/r/testsubreddit/comments/abc123/example",
				"url":         "https://reddit.com/r/testsubreddit/comments/abc123",
				"created_utc": time.Now().Format(time.RFC3339),
			},
		},
	})
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	doneEvent := findSSEEvent(events, "done")
	if doneEvent == nil {
		t.Fatalf("expected done SSE event; events: %v; body: %s", events, w.Body.String())
	}
}

func TestHandleRedditAnalyze_ParseError(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	tp.StreamFn = func(ctx context.Context, apiKey string, body []byte, w http.ResponseWriter, flusher http.Flusher) (*StreamResult, error) {
		return &StreamResult{RawText: "not valid json", ResultJSON: "not valid json"}, nil
	}
	withTestProviders(t, tp)

	redditServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer redditServer.Close()
	withTestRedditBase(t, redditServer)

	handler := handleRedditAnalyze(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "POST", "/api/reddit/analyze", map[string]any{
		"domain": "example.com",
		"threads": []map[string]any{
			{
				"id":        "t1",
				"subreddit": "r/test",
				"title":     "Thread",
				"permalink": "/r/test/comments/t1/thread",
			},
		},
	})
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatal("expected error SSE event for parse failure")
	}
}

// ── handleGenerateDomainSummary ──────────────────────────────────────────

func TestHandleGenerateDomainSummary_NoData(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleGenerateDomainSummary(db, testEncKey(), "fake-key", false)
	// Use a custom request since this handler uses PathValue("domain")
	req := testRequest(t, "GET", "/api/summary/nodomain.com", nil)
	req.SetPathValue("domain", "nodomain.com")
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatal("expected error SSE event when no data for domain")
	}
}

func TestHandleGenerateDomainSummary_WithOptimizations(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// Insert an optimization so the handler has data to work with
	analysisID := primitive.NewObjectID()
	db.Optimizations().InsertOne(ctx, bson.M{
		"tenantId":      "test-tenant",
		"domain":        "summary-test.com",
		"analysisId":    analysisID,
		"questionIndex": 0,
		"question":      "How does example.com perform?",
		"model":         "test-model",
		"createdAt":     time.Now(),
		"result": bson.M{
			"overallScore": 75,
			"contentAuthority": bson.M{
				"score":        70,
				"evidence":     bson.A{},
				"improvements": bson.A{},
			},
			"structuralOptimization": bson.M{
				"score":        65,
				"evidence":     bson.A{},
				"improvements": bson.A{},
			},
			"sourceAuthority": bson.M{
				"score":        80,
				"evidence":     bson.A{},
				"improvements": bson.A{},
			},
			"knowledgePersistence": bson.M{
				"score":        72,
				"evidence":     bson.A{},
				"improvements": bson.A{},
			},
			"competitors":     bson.A{},
			"recommendations": bson.A{},
		},
	})

	tp := newTestProvider()
	tp.id = "anthropic"
	tp.StreamFn = func(ctx context.Context, apiKey string, body []byte, w http.ResponseWriter, flusher http.Flusher) (*StreamResult, error) {
		sendSSE(w, flusher, "text", map[string]string{"content": "summarizing..."})
		return &StreamResult{RawText: validDomainSummaryResultJSON, ResultJSON: validDomainSummaryResultJSON}, nil
	}
	withTestProviders(t, tp)

	handler := handleGenerateDomainSummary(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "GET", "/api/summary/summary-test.com", nil)
	req.SetPathValue("domain", "summary-test.com")
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	doneEvent := findSSEEvent(events, "done")
	if doneEvent == nil {
		t.Fatalf("expected done SSE event; body: %s", w.Body.String())
	}
}

func TestHandleGenerateDomainSummary_ParseError(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// Insert minimal data so handler proceeds to Stream()
	db.Analyses().InsertOne(ctx, bson.M{
		"tenantId":  "test-tenant",
		"domain":    "parse-err-summary.com",
		"model":     "test-model",
		"createdAt": time.Now(),
		"result": bson.M{
			"siteSummary":  "Test",
			"questions":    bson.A{},
			"crawledPages": bson.A{},
		},
	})

	tp := newTestProvider()
	tp.id = "anthropic"
	tp.StreamFn = func(ctx context.Context, apiKey string, body []byte, w http.ResponseWriter, flusher http.Flusher) (*StreamResult, error) {
		return &StreamResult{RawText: "bad json", ResultJSON: "bad json"}, nil
	}
	withTestProviders(t, tp)

	handler := handleGenerateDomainSummary(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "GET", "/api/summary/parse-err-summary.com", nil)
	req.SetPathValue("domain", "parse-err-summary.com")
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatal("expected error SSE event for parse failure")
	}
}

// ── handleLLMTest ────────────────────────────────────────────────────────

func TestHandleLLMTest_InvalidBody(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleLLMTest(db, testEncKey(), "fake-key", false)
	// Send empty body which won't decode
	req := testRequest(t, "POST", "/api/llmtest", map[string]any{
		"domain": "", // empty domain → error
	})
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatal("expected error SSE event for empty domain")
	}
}

func TestHandleLLMTest_NoProviders(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleLLMTest(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "POST", "/api/llmtest", map[string]any{
		"domain":    "example.com",
		"providers": []string{}, // empty providers → error
	})
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatal("expected error SSE event for empty providers")
	}
}

func TestHandleLLMTest_UnknownProvider(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleLLMTest(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "POST", "/api/llmtest", map[string]any{
		"domain":    "example.com",
		"providers": []string{"unknown-provider"},
		"queries":   []map[string]any{{"query": "test", "type": "brand", "priority": "high"}},
	})
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatal("expected error SSE event for unknown provider")
	}
}

func TestHandleLLMTest_NoQueries(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleLLMTest(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "POST", "/api/llmtest", map[string]any{
		"domain":    "example.com",
		"providers": []string{"test"},
		"queries":   []any{}, // no queries → error
	})
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatal("expected error SSE event for no queries")
	}
}

func TestHandleLLMTest_Success(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	// Call() is used for both querying providers and evaluating results.
	// Return different results based on call number.
	callCount := 0
	tp.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		callCount++
		if callCount == 1 {
			// First call: provider query response
			return "example.com is a great tool for testing", nil
		}
		// Second call: evaluation response
		return validLLMTestEvalResultJSON, nil
	}
	withTestProviders(t, tp)

	handler := handleLLMTest(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "POST", "/api/llmtest", map[string]any{
		"domain":    "example.com",
		"providers": []string{"anthropic"},
		"queries": []map[string]any{
			{"query": "What is example.com?", "type": "brand", "priority": "high"},
		},
	})
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	doneEvent := findSSEEvent(events, "done")
	if doneEvent == nil {
		t.Fatalf("expected done SSE event; body: %s", w.Body.String())
	}
}

// ── handleOptimize — error and success paths ─────────────────────────────

func TestHandleOptimize_InvalidAnalysisID(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleOptimize(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "POST", "/api/analyze/badid/optimize/0", map[string]any{})
	req.SetPathValue("id", "not-a-valid-objectid")
	req.SetPathValue("idx", "0")
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatal("expected error SSE event for invalid analysis ID")
	}
}

func TestHandleOptimize_InvalidQuestionIndex(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleOptimize(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "POST", "/api/analyze/id/optimize/notanumber", map[string]any{})
	req.SetPathValue("id", primitive.NewObjectID().Hex())
	req.SetPathValue("idx", "notanumber")
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatal("expected error SSE event for invalid question index")
	}
}

func TestHandleOptimize_AnalysisNotFound(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleOptimize(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "POST", "/api/analyze/id/optimize/0", map[string]any{})
	req.SetPathValue("id", primitive.NewObjectID().Hex()) // non-existent ID
	req.SetPathValue("idx", "0")
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatal("expected error SSE event for analysis not found")
	}
}

func TestHandleOptimize_QuestionIndexOutOfRange(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// Insert an analysis with 1 question
	analysisID := primitive.NewObjectID()
	db.Analyses().InsertOne(ctx, bson.M{
		"_id":       analysisID,
		"tenantId":  "test-tenant",
		"domain":    "outofrange.com",
		"model":     "test-model",
		"createdAt": time.Now(),
		"result": bson.M{
			"siteSummary": "Test site",
			"questions": bson.A{
				bson.M{
					"question":  "How good is example.com?",
					"pageUrls":  bson.A{"https://example.com"},
					"brandStatus": "",
				},
			},
			"crawledPages": bson.A{},
		},
	})

	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleOptimize(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "POST", "/api/analyze/id/optimize/99", map[string]any{})
	req.SetPathValue("id", analysisID.Hex())
	req.SetPathValue("idx", "99") // out of range
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatal("expected error SSE event for question index out of range")
	}
}

func TestHandleOptimize_CachedResult(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// Insert an analysis
	analysisID := primitive.NewObjectID()
	db.Analyses().InsertOne(ctx, bson.M{
		"_id":       analysisID,
		"tenantId":  "test-tenant",
		"domain":    "optcached.com",
		"model":     "test-model",
		"createdAt": time.Now(),
		"result": bson.M{
			"siteSummary": "Test site",
			"questions": bson.A{
				bson.M{"question": "How good is optcached.com?", "pageUrls": bson.A{}},
			},
			"crawledPages": bson.A{},
		},
	})

	// Insert a cached optimization
	db.Optimizations().InsertOne(ctx, bson.M{
		"tenantId":      "test-tenant",
		"domain":        "optcached.com",
		"analysisId":    analysisID,
		"questionIndex": 0,
		"question":      "How good is optcached.com?",
		"model":         "test-model",
		"createdAt":     time.Now(),
		"result": bson.M{
			"overallScore":           75,
			"contentAuthority":       bson.M{"score": 70, "evidence": bson.A{}, "improvements": bson.A{}},
			"structuralOptimization": bson.M{"score": 65, "evidence": bson.A{}, "improvements": bson.A{}},
			"sourceAuthority":        bson.M{"score": 80, "evidence": bson.A{}, "improvements": bson.A{}},
			"knowledgePersistence":   bson.M{"score": 72, "evidence": bson.A{}, "improvements": bson.A{}},
			"competitors":            bson.A{},
			"recommendations":        bson.A{},
		},
	})

	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	handler := handleOptimize(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "POST", "/api/analyze/id/optimize/0", map[string]any{
		"force": false, // use cache
	})
	req.SetPathValue("id", analysisID.Hex())
	req.SetPathValue("idx", "0")
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	doneEvent := findSSEEvent(events, "done")
	if doneEvent == nil {
		t.Fatalf("expected done SSE event for cached optimization; body: %s", w.Body.String())
	}
	if !strings.Contains(string(doneEvent.Data), "cached") {
		t.Errorf("expected 'cached' in done data, got: %s", doneEvent.Data)
	}
}

func TestHandleOptimize_Success(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// Insert an analysis with 1 question
	analysisID := primitive.NewObjectID()
	db.Analyses().InsertOne(ctx, bson.M{
		"_id":       analysisID,
		"tenantId":  "test-tenant",
		"domain":    "optsuccess.com",
		"model":     "test-model",
		"createdAt": time.Now(),
		"result": bson.M{
			"siteSummary": "Test site for optimization",
			"questions": bson.A{
				bson.M{
					"question":    "How can optsuccess.com improve its LLM visibility?",
					"pageUrls":    bson.A{"https://optsuccess.com"},
					"brandStatus": "",
				},
			},
			"crawledPages": bson.A{},
		},
	})

	const validOptJSON = `{
		"overall_score": 78,
		"content_authority": {"score": 75, "evidence": ["Good content"], "improvements": ["Add FAQ"]},
		"structural_optimization": {"score": 70, "evidence": ["Clean HTML"], "improvements": ["Use schema"]},
		"source_authority": {"score": 82, "evidence": ["Good links"], "improvements": ["More citations"]},
		"knowledge_persistence": {"score": 71, "evidence": ["Regular updates"], "improvements": ["More posts"]},
		"competitors": [],
		"recommendations": []
	}`

	tp := newTestProvider()
	tp.id = "anthropic"
	tp.StreamFn = func(ctx context.Context, apiKey string, body []byte, w http.ResponseWriter, flusher http.Flusher) (*StreamResult, error) {
		sendSSE(w, flusher, "text", map[string]string{"content": "optimizing..."})
		return &StreamResult{RawText: validOptJSON, ResultJSON: validOptJSON}, nil
	}
	withTestProviders(t, tp)

	handler := handleOptimize(db, testEncKey(), "fake-key", false)
	req := testRequest(t, "POST", "/api/analyze/id/optimize/0", map[string]any{
		"force": true,
	})
	req.SetPathValue("id", analysisID.Hex())
	req.SetPathValue("idx", "0")
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	doneEvent := findSSEEvent(events, "done")
	if doneEvent == nil {
		t.Fatalf("expected done SSE event for successful optimization; body: %s", w.Body.String())
	}
}
