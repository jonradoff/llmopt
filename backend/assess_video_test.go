package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// ── assessVideo ──────────────────────────────────────────────────────────────

func TestAssessVideo_EmptyTranscript(t *testing.T) {
	tp := newTestProvider()
	video := YouTubeVideo{VideoID: "test123", Title: "Test Video", Transcript: ""}
	result, err := assessVideo(context.Background(), tp, "fake-key", video, "example.com", nil)
	if err != nil {
		t.Fatalf("expected no error for empty transcript, got: %v", err)
	}
	if result != nil {
		t.Error("expected nil result for empty transcript")
	}
}

func TestAssessVideo_Success(t *testing.T) {
	tp := newTestProvider()
	tp.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		return `{"keyword_alignment":80,"quotability":70,"info_density":60,"key_quotes":["Great quote here"],"topics":["AI","LLM"],"brand_sentiment":"positive","summary":"Excellent AI content"}`, nil
	}
	video := YouTubeVideo{
		VideoID:      "vid001",
		Title:        "AI Explained",
		ChannelTitle: "TechChannel",
		ViewCount:    1000,
		PublishedAt:  time.Now(),
		Transcript:   "Today we discuss artificial intelligence and large language models.",
	}
	result, err := assessVideo(context.Background(), tp, "fake-key", video, "example.com", []string{"AI", "LLM"})
	if err != nil {
		t.Fatalf("assessVideo failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.VideoID != "vid001" {
		t.Errorf("expected VideoID=vid001, got %q", result.VideoID)
	}
	if result.Title != "AI Explained" {
		t.Errorf("expected Title='AI Explained', got %q", result.Title)
	}
	if !result.HasTranscript {
		t.Error("expected HasTranscript=true")
	}
	if result.KeywordAlignment != 80 {
		t.Errorf("expected KeywordAlignment=80, got %d", result.KeywordAlignment)
	}
	if result.BrandSentiment != "positive" {
		t.Errorf("expected BrandSentiment=positive, got %q", result.BrandSentiment)
	}
}

func TestAssessVideo_JSONWithMarkdownWrapper(t *testing.T) {
	// Provider wraps JSON in markdown code fence — should still parse
	tp := newTestProvider()
	tp.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		return "```json\n{\"keyword_alignment\":50,\"quotability\":40,\"info_density\":30,\"key_quotes\":[],\"topics\":[\"topic1\"],\"brand_sentiment\":\"neutral\",\"summary\":\"A summary\"}\n```", nil
	}
	video := YouTubeVideo{
		VideoID:    "vid002",
		Title:      "Test",
		Transcript: "Some transcript content here.",
	}
	result, err := assessVideo(context.Background(), tp, "fake-key", video, "example.com", nil)
	if err != nil {
		t.Fatalf("assessVideo failed with markdown JSON: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.KeywordAlignment != 50 {
		t.Errorf("expected KeywordAlignment=50, got %d", result.KeywordAlignment)
	}
}

func TestAssessVideo_InvalidJSON(t *testing.T) {
	tp := newTestProvider()
	tp.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		return "this is not json at all", nil
	}
	video := YouTubeVideo{
		VideoID:    "vid003",
		Title:      "Test",
		Transcript: "Some content.",
	}
	_, err := assessVideo(context.Background(), tp, "fake-key", video, "example.com", nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected 'parse' in error, got: %v", err)
	}
}

func TestAssessVideo_ProviderError_NonRetryable(t *testing.T) {
	tp := newTestProvider()
	sentinel := errors.New("permanent API error")
	tp.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		return "", sentinel
	}
	video := YouTubeVideo{
		VideoID:    "vid004",
		Title:      "Test",
		Transcript: "Some content.",
	}
	_, err := assessVideo(context.Background(), tp, "fake-key", video, "example.com", nil)
	if err == nil {
		t.Fatal("expected error from provider")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got: %v", err)
	}
}

func TestAssessVideo_OverloadedThenSuccess(t *testing.T) {
	// ErrOverloaded on first call, success on second
	tp := newTestProvider()
	calls := 0
	tp.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		calls++
		if calls == 1 {
			return "", ErrOverloaded
		}
		return `{"keyword_alignment":55,"quotability":45,"info_density":35,"key_quotes":[],"topics":[],"brand_sentiment":"neutral","summary":"Retry worked"}`, nil
	}
	video := YouTubeVideo{
		VideoID:    "vid005",
		Title:      "Test",
		Transcript: "Content here.",
	}
	// Use a context with timeout to prevent long waits on backoff
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := assessVideo(ctx, tp, "fake-key", video, "example.com", nil)
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if calls < 2 {
		t.Errorf("expected at least 2 calls for retry, got %d", calls)
	}
}

func TestAssessVideo_ContextCancelled(t *testing.T) {
	tp := newTestProvider()
	tp.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		return "", ErrOverloaded
	}
	video := YouTubeVideo{
		VideoID:    "vid006",
		Title:      "Test",
		Transcript: "Content here.",
	}
	// Cancel context before calling
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancelled
	_, err := assessVideo(ctx, tp, "fake-key", video, "example.com", nil)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestAssessVideo_EmptySearchTerms(t *testing.T) {
	// Nil searchTerms should work fine
	tp := newTestProvider()
	tp.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		return `{"keyword_alignment":0,"quotability":0,"info_density":0,"key_quotes":[],"topics":[],"brand_sentiment":"none","summary":"No terms"}`, nil
	}
	video := YouTubeVideo{
		VideoID:    "vid007",
		Title:      "No Terms Test",
		Transcript: "Just some transcript.",
	}
	result, err := assessVideo(context.Background(), tp, "fake-key", video, "example.com", nil)
	if err != nil {
		t.Fatalf("assessVideo failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}
