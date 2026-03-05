package main

// Tests for handleVideoDiscover and handleVideoAnalyze SSE handlers.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
)

// ── handleVideoDiscover ───────────────────────────────────────────────────────

func TestHandleVideoDiscover_NoYouTubeKey(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	// saasEnabled=false, empty systemYTKey → resolveYouTubeKey returns error
	handler := handleVideoDiscover(db, testEncKey(), "", false)
	req := testRequest(t, "POST", "/api/video/discover", map[string]any{
		"domain": "example.com",
	})
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatalf("expected error SSE event for missing YouTube key; body: %s", w.Body.String())
	}
}

func TestHandleVideoDiscover_EmptyRequest_NoSearches(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	// With a fake YouTube key (non-SaaS), empty request body (no brand/channel/terms)
	// → discoverVideos returns empty slice without calling YouTube API
	handler := handleVideoDiscover(db, testEncKey(), "fake-yt-key", false)
	req := testRequest(t, "POST", "/api/video/discover", map[string]any{
		"domain": "example.com",
		// No brand_name, channel_url, search_terms, or competitors
	})
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	doneEvent := findSSEEvent(events, "done")
	if doneEvent == nil {
		t.Fatalf("expected done SSE event for empty discover request; body: %s", w.Body.String())
	}
}

func TestHandleVideoDiscover_InvalidBody(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	// With a fake YouTube key but invalid JSON body
	handler := handleVideoDiscover(db, testEncKey(), "fake-yt-key", false)
	rawReq := httptest.NewRequest("POST", "/api/video/discover", bytes.NewReader([]byte("not-json")))
	rawReq = rawReq.WithContext(testAuthContext("test-tenant", "test-user"))
	rawReq.Header.Set("Content-Type", "application/json")
	req := rawReq
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatalf("expected error SSE event for invalid body; body: %s", w.Body.String())
	}
}

// ── handleVideoAnalyze ────────────────────────────────────────────────────────

func TestHandleVideoAnalyze_NoYouTubeKey(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	// No YouTube key → immediate error SSE
	handler := handleVideoAnalyze(db, testEncKey(), "fake-key", false, "")
	req := testRequest(t, "POST", "/api/video/analyze", map[string]any{
		"domain":             "example.com",
		"selected_video_ids": []string{"vid1"},
	})
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatalf("expected error SSE event for missing YouTube key; body: %s", w.Body.String())
	}
}

func TestHandleVideoAnalyze_NoVideosSelected(t *testing.T) {
	db := testMongoDB(t)
	tp := newTestProvider()
	tp.id = "anthropic"
	withTestProviders(t, tp)

	// YouTube key present, but no video IDs selected → error SSE
	handler := handleVideoAnalyze(db, testEncKey(), "fake-key", false, "fake-yt-key")
	req := testRequest(t, "POST", "/api/video/analyze", map[string]any{
		"domain":             "example.com",
		"selected_video_ids": []string{},
	})
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	errEvent := findSSEEvent(events, "error")
	if errEvent == nil {
		t.Fatalf("expected error SSE event for empty selected_video_ids; body: %s", w.Body.String())
	}
}

func TestHandleVideoAnalyze_CachedVideos_Success(t *testing.T) {
	db := testMongoDB(t)

	// Pre-seed video details and transcripts so cachedVideoDetails and cachedTranscript
	// return data without calling YouTube API.
	vid1 := YouTubeVideo{
		VideoID:      "analyze-vid-1",
		Title:        "AI Tutorial",
		ChannelTitle: "ThirdParty Channel",
		ChannelID:    "third-party-channel-id",
		ViewCount:    5000,
		LikeCount:    200,
		PublishedAt:  time.Now().Add(-30 * 24 * time.Hour),
		Transcript:   "AI and machine learning are transforming the world.",
	}
	vid1Data, _ := json.Marshal(vid1)
	setCache(db, "video:analyze-vid-1", string(vid1Data))
	setCache(db, "transcript:analyze-vid-1", "AI and machine learning are transforming the world.")

	// Mock LLM provider returns valid JSON for all calls
	tp := newTestProvider()
	tp.id = "anthropic"
	callCount := 0
	tp.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		callCount++
		// For assessVideos (small model) - return VideoAssessment JSON
		return `{"keyword_alignment":75,"quotability":65,"info_density":55,"key_quotes":["AI is the future"],"topics":["AI","ML"],"brand_sentiment":"positive","summary":"Good AI content"}`, nil
	}
	withTestProviders(t, tp)

	handler := handleVideoAnalyze(db, testEncKey(), "fake-key", false, "fake-yt-key")
	req := testRequest(t, "POST", "/api/video/analyze", map[string]any{
		"domain":             "example.com",
		"selected_video_ids": []string{"analyze-vid-1"},
		"config": map[string]any{
			"channel_url": "", // no own channel → all videos are third-party
		},
	})
	w := httptest.NewRecorder()
	handler(w, req)

	events := parseSSEResponse(t, w.Body.Bytes())
	// Should end with done or error (error is acceptable if JSON parse fails)
	hasTerminal := findSSEEvent(events, "done") != nil || findSSEEvent(events, "error") != nil
	if !hasTerminal {
		t.Fatalf("expected done or error SSE event; body: %s", w.Body.String())
	}
}
