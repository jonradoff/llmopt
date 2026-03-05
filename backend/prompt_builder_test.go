package main

// Tests for prompt builder functions and assessVideos orchestrator.

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ── assessVideos ──────────────────────────────────────────────────────────────

func TestAssessVideos_EmptyVideos(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	w := httptest.NewRecorder()

	result := assessVideos(context.Background(), tp, "fake-key", nil, "example.com", nil, db, w, w)
	if len(result) != 0 {
		t.Errorf("expected empty map for empty videos, got %d items", len(result))
	}
}

func TestAssessVideos_NoTranscript_Skipped(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	// provider should never be called for videos without transcripts
	tp.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		t.Error("provider should not be called for video without transcript")
		return "", nil
	}
	w := httptest.NewRecorder()

	videos := []YouTubeVideo{
		{VideoID: "skip1", Title: "No Transcript", Transcript: ""},
	}
	result := assessVideos(context.Background(), tp, "fake-key", videos, "example.com", nil, db, w, w)
	if len(result) != 1 {
		t.Errorf("expected 1 result entry (nil assessment), got %d", len(result))
	}
	if result["skip1"] != nil {
		t.Error("expected nil assessment for video without transcript")
	}
}

func TestAssessVideos_WithTranscript_Success(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		return `{"keyword_alignment":75,"quotability":65,"info_density":55,"key_quotes":["test quote"],"topics":["AI"],"brand_sentiment":"positive","summary":"Good video"}`, nil
	}
	w := httptest.NewRecorder()

	videos := []YouTubeVideo{
		{VideoID: "assessed1", Title: "AI Video", ChannelTitle: "TestChan", ViewCount: 1000, PublishedAt: time.Now(),
			Transcript: "This is a transcript about AI and machine learning."},
	}
	result := assessVideos(context.Background(), tp, "fake-key", videos, "example.com", []string{"AI"}, db, w, w)
	if len(result) != 1 {
		t.Errorf("expected 1 assessment, got %d", len(result))
	}
	if a := result["assessed1"]; a == nil {
		t.Error("expected non-nil assessment for video with transcript")
	} else if a.KeywordAlignment != 75 {
		t.Errorf("expected KeywordAlignment=75, got %d", a.KeywordAlignment)
	}
}

func TestAssessVideos_FromCache(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// Pre-seed a cached assessment
	cached := &VideoAssessment{
		VideoID:          "cache-vid",
		KeywordAlignment: 90,
		Quotability:      85,
		InfoDensity:      80,
		BrandSentiment:   "positive",
		HasTranscript:    true,
	}
	setCachedVideoAssessment(db, "cache-vid", "cached-brand.com", nil, cached)
	_ = ctx // used by testMongoDB cleanup

	tp := newTestProvider()
	tp.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		t.Error("provider should not be called when assessment is cached")
		return "", nil
	}
	w := httptest.NewRecorder()

	videos := []YouTubeVideo{
		{VideoID: "cache-vid", Title: "Cached Video", Transcript: "some transcript"},
	}
	result := assessVideos(context.Background(), tp, "fake-key", videos, "cached-brand.com", nil, db, w, w)
	if a := result["cache-vid"]; a == nil {
		t.Error("expected non-nil cached assessment")
	} else if a.KeywordAlignment != 90 {
		t.Errorf("expected KeywordAlignment=90 from cache, got %d", a.KeywordAlignment)
	}
}

// ── writeDigests ──────────────────────────────────────────────────────────────

func TestWriteDigests_Empty(t *testing.T) {
	p := videoContextParams{
		Domain:    "test.com",
		BrandName: "Test",
		Digests:   nil,
	}
	var sb strings.Builder
	writeDigests(&sb, p)
	if sb.String() != "" {
		t.Errorf("expected empty output for nil digests, got: %s", sb.String())
	}
}

func TestWriteDigests_WithData(t *testing.T) {
	p := videoContextParams{
		Domain:          "test.com",
		BrandName:       "Test",
		ThirdPartyCount: 15,
		Digests: []BatchDigest{
			{
				BatchIndex:    0,
				VideoCount:    15,
				Summary:       "Excellent AI content landscape",
				TopCreators:   []string{"Creator A", "Creator B"},
				TopicsCovered: []string{"AI", "ML", "LLM"},
				SentimentTally: map[string]int{
					"positive": 10, "neutral": 3, "negative": 2, "none": 0,
				},
				NotableQuotes: []string{"AI is the future"},
				ContentGaps:   []string{"Missing: practical tutorials"},
			},
		},
	}
	var sb strings.Builder
	writeDigests(&sb, p)
	out := sb.String()

	if !strings.Contains(out, "THIRD-PARTY VIDEO DIGESTS") {
		t.Error("expected THIRD-PARTY VIDEO DIGESTS header")
	}
	if !strings.Contains(out, "Excellent AI content landscape") {
		t.Error("expected digest summary")
	}
	if !strings.Contains(out, "Creator A") {
		t.Error("expected top creator")
	}
	if !strings.Contains(out, "AI is the future") {
		t.Error("expected notable quote")
	}
	if !strings.Contains(out, "Missing: practical tutorials") {
		t.Error("expected content gap")
	}
}

// ── writeOwnChannelVideos ─────────────────────────────────────────────────────

func TestWriteOwnChannelVideos_Empty(t *testing.T) {
	p := videoContextParams{
		Domain:    "test.com",
		BrandName: "Test",
		OwnVideos: nil,
	}
	var sb strings.Builder
	writeOwnChannelVideos(&sb, p)
	if sb.String() != "" {
		t.Errorf("expected empty output for no own videos, got: %s", sb.String())
	}
}

func TestWriteOwnChannelVideos_WithVideo(t *testing.T) {
	p := videoContextParams{
		Domain:    "test.com",
		BrandName: "TestBrand",
		OwnVideos: []YouTubeVideo{
			{
				VideoID:      "own-v1",
				Title:        "Brand Feature Demo",
				ChannelTitle: "TestBrand",
				ViewCount:    5000,
				LikeCount:    200,
				CommentCount: 50,
				Duration:     "PT5M30S",
				PublishedAt:  time.Now(),
				Description:  "A comprehensive demo of our product.",
				Tags:         []string{"demo", "tutorial"},
				Transcript:   "Welcome to TestBrand. Today we will show you our amazing feature.",
			},
		},
		Assessments: map[string]*VideoAssessment{},
	}
	var sb strings.Builder
	writeOwnChannelVideos(&sb, p)
	out := sb.String()

	if !strings.Contains(out, "OWN CHANNEL VIDEOS") {
		t.Error("expected OWN CHANNEL VIDEOS header")
	}
	if !strings.Contains(out, "Brand Feature Demo") {
		t.Error("expected video title")
	}
	if !strings.Contains(out, "5000") {
		t.Error("expected view count")
	}
	if !strings.Contains(out, "demo") {
		t.Error("expected tags")
	}
	if !strings.Contains(out, "Welcome to TestBrand") {
		t.Error("expected transcript content")
	}
}

func TestWriteOwnChannelVideos_WithDescription(t *testing.T) {
	p := videoContextParams{
		Domain:    "test.com",
		BrandName: "TestBrand",
		OwnVideos: []YouTubeVideo{
			{
				VideoID:     "desc-v1",
				Title:       "Desc Only Video",
				ViewCount:   100,
				PublishedAt: time.Now(),
				Description: "This video has a description but no transcript.",
				// No Transcript
			},
		},
		Assessments: nil,
	}
	var sb strings.Builder
	writeOwnChannelVideos(&sb, p)
	out := sb.String()

	if !strings.Contains(out, "This video has a description") {
		t.Error("expected description in output")
	}
}

// ── buildRedditAuthorityPrompt ────────────────────────────────────────────────

func TestBuildRedditAuthorityPrompt_Basic(t *testing.T) {
	threads := []RedditThread{
		{
			ID:        "thread1",
			Subreddit: "r/programming",
			Title:     "How does TestBrand compare to competitors?",
			SelfText:  "I've been using TestBrand for a while and want to know opinions.",
			Score:     150,
			NumComments: 45,
			CreatedUTC: time.Now().Add(-24 * time.Hour),
		},
	}
	brandInfo := BrandContextInfo{
		Used:          true,
		ContextString: "Company: TestBrand\nCategory: SaaS",
	}

	prompt := buildRedditAuthorityPrompt("testbrand.com", threads, []string{"competitor-a.com"}, []string{"AI SaaS"}, brandInfo)

	if !strings.Contains(prompt, "TestBrand") {
		t.Error("expected brand name in reddit prompt")
	}
	if !strings.Contains(prompt, "r/programming") || !strings.Contains(prompt, "testbrand.com") {
		t.Error("expected domain/subreddit in reddit prompt")
	}
	if len(prompt) < 100 {
		t.Errorf("expected substantial prompt, got %d chars", len(prompt))
	}
}

func TestBuildRedditAuthorityPrompt_NoBrandInfo(t *testing.T) {
	prompt := buildRedditAuthorityPrompt("nodomain.com", nil, nil, nil, BrandContextInfo{Used: false})
	if !strings.Contains(prompt, "nodomain.com") {
		t.Error("expected domain as brand name when no brand info")
	}
	if len(prompt) < 50 {
		t.Errorf("expected non-trivial prompt, got %d chars", len(prompt))
	}
}

// ── buildSearchVisibilityPrompt ───────────────────────────────────────────────

func TestBuildSearchVisibilityPrompt_WithBrandInfo(t *testing.T) {
	brandInfo := BrandContextInfo{
		Used:          true,
		ContextString: "Company: SearchBrand\nCategory: SEO Tools",
	}
	prompt := buildSearchVisibilityPrompt("searchbrand.com", brandInfo,
		[]string{"competitor-seo.com"},
		[]string{"AI SEO", "search optimization"},
		[]string{"How does SearchBrand work?"},
	)

	if !strings.Contains(prompt, "SearchBrand") {
		t.Error("expected brand name in search visibility prompt")
	}
	if !strings.Contains(prompt, "searchbrand.com") {
		t.Error("expected domain in prompt")
	}
	if !strings.Contains(prompt, "competitor-seo.com") {
		t.Error("expected competitor in prompt")
	}
	if len(prompt) < 100 {
		t.Errorf("expected substantial prompt, got %d chars", len(prompt))
	}
}

func TestBuildSearchVisibilityPrompt_NoBrandInfo(t *testing.T) {
	prompt := buildSearchVisibilityPrompt("basic-search.com", BrandContextInfo{Used: false}, nil, nil, nil)
	if !strings.Contains(prompt, "basic-search.com") {
		t.Error("expected domain in prompt")
	}
}

// ── handleServeSharedPDF ──────────────────────────────────────────────────────

func TestHandleServeSharedPDF_NotFound(t *testing.T) {
	db := testMongoDB(t)

	handler := handleServeSharedPDF(db)
	req := testRequest(t, "GET", "/s/share-nopdf/pdf", nil)
	req.SetPathValue("shareId", "nonexistent-share-abc")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, 404)
}

func TestHandleServeSharedPDF_ShareExistsNoPDF(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	shareID := "shared-pdf-no-doc-xyz"
	db.DomainShares().InsertOne(ctx, DomainShare{
		ID:         primitive.NewObjectID(),
		TenantID:   "test-tenant",
		Domain:     "no-pdf-domain.com",
		ShareID:    shareID,
		Visibility: "public",
		CreatedAt:  time.Now(),
	})

	handler := handleServeSharedPDF(db)
	req := testRequest(t, "GET", "/s/"+shareID+"/pdf", nil)
	req.SetPathValue("shareId", shareID)
	w := httptest.NewRecorder()
	handler(w, req)

	// No PDF exists for this share → 404
	assertStatus(t, w, 404)
}

func TestHandleServeSharedPDF_Success(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	shareID := "shared-pdf-success-xyz"
	db.DomainShares().InsertOne(ctx, DomainShare{
		ID:         primitive.NewObjectID(),
		TenantID:   "test-tenant",
		Domain:     "pdf-share-success.com",
		ShareID:    shareID,
		Visibility: "public",
		CreatedAt:  time.Now(),
	})

	pdfData := []byte("%PDF-1.4 shared report data")
	db.ReportPDFs().InsertOne(ctx, ReportPDF{
		ID:          primitive.NewObjectID(),
		TenantID:    "test-tenant",
		Domain:      "pdf-share-success.com",
		PDFData:     pdfData,
		SizeBytes:   len(pdfData),
		GeneratedAt: time.Now(),
	})

	handler := handleServeSharedPDF(db)
	req := testRequest(t, "GET", "/s/"+shareID+"/pdf", nil)
	req.SetPathValue("shareId", shareID)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, 200)
	if ct := w.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("expected application/pdf, got %q", ct)
	}
	if w.Body.Len() == 0 {
		t.Error("expected non-empty PDF body")
	}
}
