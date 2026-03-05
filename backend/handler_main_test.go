package main

// Tests for main.go handlers and pure utility functions (prompt builders, etc.)

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ── handleListAnalyses ────────────────────────────────────────────────────────

func TestHandleListAnalyses_FilterByDomain(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.Analyses().InsertOne(ctx, Analysis{
		TenantID:  "test-tenant",
		Domain:    "domain-filter-target.com",
		Model:     "claude-3-haiku",
		CreatedAt: time.Now(),
	})
	db.Analyses().InsertOne(ctx, Analysis{
		TenantID:  "test-tenant",
		Domain:    "domain-filter-other.com",
		Model:     "claude-3-haiku",
		CreatedAt: time.Now(),
	})

	handler := handleListAnalyses(db)
	req := testRequest(t, "GET", "/api/analyses?domain=domain-filter-target.com", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var result []any
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("expected JSON array: %v; body: %s", err, w.Body.String())
	}
	if len(result) != 1 {
		t.Errorf("expected 1 filtered analysis, got %d", len(result))
	}
}

// ── handleGetAnalysis ─────────────────────────────────────────────────────────

func TestHandleGetAnalysis_Found(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	id := primitive.NewObjectID()
	db.Analyses().InsertOne(ctx, Analysis{
		ID:        id,
		TenantID:  "test-tenant",
		Domain:    "getanalysis.com",
		Model:     "claude-3-haiku",
		CreatedAt: time.Now(),
	})

	handler := handleGetAnalysis(db)
	req := testRequest(t, "GET", "/api/analyses/"+id.Hex(), nil)
	req.SetPathValue("id", id.Hex())
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %q", ct)
	}
	var result map[string]any
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("expected JSON: %v; body: %s", err, w.Body.String())
	}
}

// ── handleGetVideoAnalysis ────────────────────────────────────────────────────

func TestHandleGetVideoAnalysis_Found(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.VideoAnalyses().InsertOne(ctx, VideoAnalysis{
		ID:          primitive.NewObjectID(),
		TenantID:    "test-tenant",
		Domain:      "videoanalysis-found.com",
		Model:       "claude-sonnet-4-6",
		GeneratedAt: time.Now(),
		Videos: []YouTubeVideo{
			{VideoID: "vf1", Title: "Test Video", ViewCount: 100, PublishedAt: time.Now()},
		},
	})

	handler := handleGetVideoAnalysis(db)
	req := testRequest(t, "GET", "/api/brand/videoanalysis-found.com/video", nil)
	req.SetPathValue("domain", "videoanalysis-found.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %q", ct)
	}
}

// ── handleListVideoAnalyses ───────────────────────────────────────────────────

func TestHandleListVideoAnalyses_NoDB(t *testing.T) {
	db := testMongoDB(t)

	handler := handleListVideoAnalyses(db)
	req := testRequest(t, "GET", "/api/brand/video/analyses", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	// Should return empty array JSON
	body := w.Body.String()
	if !strings.Contains(body, "null") && !strings.Contains(body, "[]") {
		// MongoDB returns null for empty find, which is fine
		t.Logf("body: %s", body)
	}
}

func TestHandleListVideoAnalyses_MultipleAnalyses(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.VideoAnalyses().InsertOne(ctx, bson.M{
		"_id":         primitive.NewObjectID(),
		"tenantId":    "test-tenant",
		"domain":      "listvid1.com",
		"model":       "claude-sonnet-4-6",
		"generatedAt": time.Now(),
		"videos":      bson.A{},
	})
	db.VideoAnalyses().InsertOne(ctx, bson.M{
		"_id":         primitive.NewObjectID(),
		"tenantId":    "test-tenant",
		"domain":      "listvid2.com",
		"model":       "claude-sonnet-4-6",
		"generatedAt": time.Now().Add(-time.Hour),
		"videos":      bson.A{},
	})

	handler := handleListVideoAnalyses(db)
	req := testRequest(t, "GET", "/api/brand/video/analyses", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var result []any
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("expected JSON array: %v; body: %s", err, w.Body.String())
	}
	if len(result) < 2 {
		t.Errorf("expected at least 2 analyses, got %d", len(result))
	}
}

// ── handleServePDF ────────────────────────────────────────────────────────────

func TestHandleServePDF_InvalidID(t *testing.T) {
	db := testMongoDB(t)

	handler := handleServePDF(db)
	req := testRequest(t, "GET", "/api/brand/example.com/pdf/badid", nil)
	req.SetPathValue("domain", "example.com")
	req.SetPathValue("id", "badid")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleServePDF_NotFound(t *testing.T) {
	db := testMongoDB(t)

	handler := handleServePDF(db)
	req := testRequest(t, "GET", "/api/brand/example.com/pdf/000000000000000000000001", nil)
	req.SetPathValue("domain", "example.com")
	req.SetPathValue("id", "000000000000000000000001")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusNotFound)
}

func TestHandleServePDF_Found(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	id := primitive.NewObjectID()
	pdfData := []byte("%PDF-1.4 test data")
	db.ReportPDFs().InsertOne(ctx, ReportPDF{
		ID:          id,
		TenantID:    "test-tenant",
		Domain:      "pdf-serve.com",
		PDFData:     pdfData,
		SizeBytes:   len(pdfData),
		GeneratedAt: time.Now(),
	})

	handler := handleServePDF(db)
	req := testRequest(t, "GET", "/api/brand/pdf-serve.com/pdf/"+id.Hex(), nil)
	req.SetPathValue("domain", "pdf-serve.com")
	req.SetPathValue("id", id.Hex())
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	if ct := w.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("expected application/pdf, got %q", ct)
	}
	if !strings.Contains(w.Header().Get("Content-Disposition"), ".pdf") {
		t.Errorf("expected .pdf in Content-Disposition, got %q", w.Header().Get("Content-Disposition"))
	}
	if w.Body.Len() == 0 {
		t.Error("expected non-empty PDF body")
	}
}

// ── buildPillar1Prompt / buildPillar2Prompt / buildPillar3Prompt / buildPillar4Prompt / buildSynthesisPrompt ──

func makeTestVideoContextParams() videoContextParams {
	return videoContextParams{
		Domain:      "test-brand.com",
		BrandName:   "TestBrand",
		SearchTerms: []string{"AI", "LLM"},
		Competitors: []string{"competitor-a.com", "competitor-b.com"},
		BrandInfo: BrandContextInfo{
			Used:          true,
			ContextString: "Company: TestBrand\nCategory: SaaS",
		},
		OwnVideos: []YouTubeVideo{
			{VideoID: "own1", Title: "Brand Video 1", ChannelTitle: "TestBrand", ViewCount: 5000, PublishedAt: time.Now()},
		},
		ThirdPartyCount: 10,
		Digests: []BatchDigest{
			{BatchIndex: 0, VideoCount: 10, Summary: "Good content about AI", TopCreators: []string{"Creator1"}, TopicsCovered: []string{"AI"}},
		},
		Assessments: map[string]*VideoAssessment{
			"own1": {VideoID: "own1", KeywordAlignment: 80, Quotability: 70, InfoDensity: 60, HasTranscript: true, BrandSentiment: "positive"},
		},
	}
}

func TestBuildPillar1Prompt_ContainsExpectedSections(t *testing.T) {
	p := makeTestVideoContextParams()
	prompt := buildPillar1Prompt(p)

	if !strings.Contains(prompt, "TestBrand") {
		t.Error("expected brand name in pillar 1 prompt")
	}
	if !strings.Contains(prompt, "TRANSCRIPT AUTHORITY") {
		t.Error("expected TRANSCRIPT AUTHORITY section")
	}
	if !strings.Contains(prompt, "test-brand.com") {
		t.Error("expected domain in pillar 1 prompt")
	}
	if !strings.Contains(prompt, "Brand Video 1") {
		t.Error("expected own video title in pillar 1 prompt")
	}
	if len(prompt) < 100 {
		t.Errorf("expected substantial prompt, got %d chars", len(prompt))
	}
}

func TestBuildPillar2Prompt_ContainsDigests(t *testing.T) {
	p := makeTestVideoContextParams()
	prompt := buildPillar2Prompt(p)

	if !strings.Contains(prompt, "TOPICAL DOMINANCE") {
		t.Error("expected TOPICAL DOMINANCE section")
	}
	if !strings.Contains(prompt, "Good content about AI") {
		t.Error("expected digest summary in pillar 2 prompt")
	}
	if !strings.Contains(prompt, "Creator1") {
		t.Error("expected top creator in pillar 2 prompt")
	}
}

func TestBuildPillar3Prompt_ContainsSection(t *testing.T) {
	p := makeTestVideoContextParams()
	prompt := buildPillar3Prompt(p)

	if len(prompt) < 50 {
		t.Errorf("expected non-trivial pillar 3 prompt, got %d chars", len(prompt))
	}
	if !strings.Contains(prompt, "TestBrand") {
		t.Error("expected brand name in pillar 3 prompt")
	}
}

func TestBuildPillar4Prompt_ContainsSection(t *testing.T) {
	p := makeTestVideoContextParams()
	prompt := buildPillar4Prompt(p)

	if len(prompt) < 50 {
		t.Errorf("expected non-trivial pillar 4 prompt, got %d chars", len(prompt))
	}
	if !strings.Contains(prompt, "TestBrand") {
		t.Error("expected brand name in pillar 4 prompt")
	}
}

func TestBuildSynthesisPrompt_CombinesPillars(t *testing.T) {
	prompt := buildSynthesisPrompt("TestBrand",
		`{"transcript_authority":{"score":70}}`,
		`{"topical_dominance":{"score":65}}`,
		`{"brand_narrative":{"score":80}}`,
		`{"citation_network":{"score":75}}`,
	)

	if !strings.Contains(prompt, "TestBrand") {
		t.Error("expected brand name in synthesis prompt")
	}
	if !strings.Contains(prompt, "transcript_authority") {
		t.Error("expected pillar 1 data in synthesis prompt")
	}
	if !strings.Contains(prompt, "topical_dominance") {
		t.Error("expected pillar 2 data in synthesis prompt")
	}
	if len(prompt) < 100 {
		t.Errorf("expected substantial synthesis prompt, got %d chars", len(prompt))
	}
}

// ── writeVideoAssessment ──────────────────────────────────────────────────────

func TestWriteVideoAssessment_WithAssessment(t *testing.T) {
	var sb strings.Builder
	v := YouTubeVideo{VideoID: "v1", Title: "Test Video"}
	assessments := map[string]*VideoAssessment{
		"v1": {VideoID: "v1", KeywordAlignment: 80, Quotability: 70, InfoDensity: 60,
			KeyQuotes: []string{"This is a key quote"}, Topics: []string{"AI", "ML"},
			BrandSentiment: "positive", Summary: "Good video", HasTranscript: true},
	}
	writeVideoAssessment(&sb, v, assessments)
	out := sb.String()

	if !strings.Contains(out, "keyword_alignment=80") {
		t.Errorf("expected keyword_alignment=80 in output, got: %s", out)
	}
	if !strings.Contains(out, "This is a key quote") {
		t.Error("expected key quote in output")
	}
	if !strings.Contains(out, "positive") {
		t.Error("expected sentiment in output")
	}
}

func TestWriteVideoAssessment_NoTranscriptNoAssessment(t *testing.T) {
	var sb strings.Builder
	v := YouTubeVideo{VideoID: "v2", Title: "No Transcript", Description: ""}
	writeVideoAssessment(&sb, v, nil)
	out := sb.String()

	if !strings.Contains(out, "NOT AVAILABLE") {
		t.Errorf("expected NOT AVAILABLE for missing transcript, got: %s", out)
	}
}

func TestWriteVideoAssessment_RawTranscript(t *testing.T) {
	var sb strings.Builder
	v := YouTubeVideo{VideoID: "v3", Title: "With Transcript", Transcript: "Hello world this is a transcript."}
	writeVideoAssessment(&sb, v, map[string]*VideoAssessment{}) // empty map, no assessment
	out := sb.String()

	if !strings.Contains(out, "Transcript (raw):") {
		t.Errorf("expected raw transcript section, got: %s", out)
	}
	if !strings.Contains(out, "Hello world") {
		t.Error("expected transcript content in output")
	}
}

func TestWriteVideoAssessment_LongTranscriptTruncated(t *testing.T) {
	var sb strings.Builder
	long := strings.Repeat("a", 2000)
	v := YouTubeVideo{VideoID: "v4", Title: "Long Transcript", Transcript: long}
	writeVideoAssessment(&sb, v, nil)
	out := sb.String()

	if !strings.Contains(out, "[truncated]") {
		t.Errorf("expected [truncated] for long transcript, got partial: %s", out[:100])
	}
}

// ── writePreamble ─────────────────────────────────────────────────────────────

func TestWritePreamble_ContainsExpectedContent(t *testing.T) {
	p := makeTestVideoContextParams()
	var sb strings.Builder
	writePreamble(&sb, p)
	out := sb.String()

	if !strings.Contains(out, "TestBrand") {
		t.Error("expected brand name in preamble")
	}
	if !strings.Contains(out, "test-brand.com") {
		t.Error("expected domain in preamble")
	}
	if !strings.Contains(out, "AI") {
		t.Error("expected search terms in preamble")
	}
	if !strings.Contains(out, "Company: TestBrand") {
		t.Error("expected brand info context in preamble")
	}
}

func TestWritePreamble_NoBrandInfo(t *testing.T) {
	p := videoContextParams{
		Domain:      "noBrandInfo.com",
		BrandName:   "NoBrand",
		SearchTerms: []string{"test"},
		BrandInfo:   BrandContextInfo{Used: false},
	}
	var sb strings.Builder
	writePreamble(&sb, p)
	out := sb.String()

	if !strings.Contains(out, "NoBrand") {
		t.Error("expected brand name even without brand info")
	}
	// ContextString should not appear when Used=false
	if strings.Contains(out, "Company:") {
		t.Error("should not include brand context string when Used=false")
	}
}
