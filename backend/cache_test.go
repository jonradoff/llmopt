package main

// Tests for the YouTube cache layer: getCache, setCache, setCacheWithTTL,
// cachedYouTubeSearch (cache-hit path), cachedTranscript (cache-hit path),
// cachedVideoAssessment / setCachedVideoAssessment.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ── getCache / setCache / setCacheWithTTL ─────────────────────────────────────

func TestGetCache_Miss(t *testing.T) {
	db := testMongoDB(t)
	_, ok := getCache(db, "cache-test-miss-"+time.Now().String())
	if ok {
		t.Error("expected cache miss for unknown key")
	}
}

func TestSetCache_GetCache_RoundTrip(t *testing.T) {
	db := testMongoDB(t)
	key := "roundtrip-test-key-abc"
	data := `{"score":99,"label":"test"}`

	setCache(db, key, data)
	got, ok := getCache(db, key)
	if !ok {
		t.Error("expected cache hit after setCache")
	}
	if got != data {
		t.Errorf("expected %q, got %q", data, got)
	}
}

func TestSetCacheWithTTL_Hit(t *testing.T) {
	db := testMongoDB(t)
	key := "ttl-cache-test-key-xyz"
	data := "cached with TTL"

	setCacheWithTTL(db, key, data, 10*time.Minute)
	got, ok := getCache(db, key)
	if !ok {
		t.Error("expected cache hit after setCacheWithTTL")
	}
	if got != data {
		t.Errorf("expected %q, got %q", data, got)
	}
}

func TestSetCache_Overwrite(t *testing.T) {
	db := testMongoDB(t)
	key := "overwrite-test-key"

	setCache(db, key, "first value")
	setCache(db, key, "second value") // should overwrite
	got, ok := getCache(db, key)
	if !ok {
		t.Error("expected cache hit")
	}
	if got != "second value" {
		t.Errorf("expected 'second value', got %q", got)
	}
}

// ── cachedVideoAssessment / setCachedVideoAssessment ──────────────────────────

func TestCachedVideoAssessment_MissAndSet(t *testing.T) {
	db := testMongoDB(t)

	// Initially not in cache
	_, ok := cachedVideoAssessment(db, "new-vid-abc", "brand-x.com", []string{"AI", "ML"})
	if ok {
		t.Error("expected cache miss for new assessment")
	}

	assessment := &VideoAssessment{
		VideoID:          "new-vid-abc",
		KeywordAlignment: 85,
		Quotability:      70,
		InfoDensity:      60,
		BrandSentiment:   "positive",
		HasTranscript:    true,
		KeyQuotes:        []string{"AI is important"},
		Topics:           []string{"AI", "ML"},
		Summary:          "Great AI content",
	}
	setCachedVideoAssessment(db, "new-vid-abc", "brand-x.com", []string{"AI", "ML"}, assessment)

	got, ok := cachedVideoAssessment(db, "new-vid-abc", "brand-x.com", []string{"AI", "ML"})
	if !ok {
		t.Error("expected cache hit after setCachedVideoAssessment")
	}
	if got.KeywordAlignment != 85 {
		t.Errorf("expected KeywordAlignment=85, got %d", got.KeywordAlignment)
	}
	if got.BrandSentiment != "positive" {
		t.Errorf("expected BrandSentiment=positive, got %q", got.BrandSentiment)
	}
}

func TestCachedVideoAssessment_DifferentSearchTermsMiss(t *testing.T) {
	db := testMongoDB(t)

	// Seed with search terms ["AI"]
	assessment := &VideoAssessment{
		VideoID:       "search-terms-vid",
		BrandSentiment: "neutral",
		HasTranscript: true,
	}
	setCachedVideoAssessment(db, "search-terms-vid", "brand.com", []string{"AI"}, assessment)

	// Different search terms → different cache key → cache miss
	_, ok := cachedVideoAssessment(db, "search-terms-vid", "brand.com", []string{"LLM", "GPT"})
	if ok {
		t.Error("expected cache miss for different search terms")
	}

	// Same search terms → cache hit
	_, ok = cachedVideoAssessment(db, "search-terms-vid", "brand.com", []string{"AI"})
	if !ok {
		t.Error("expected cache hit for same search terms")
	}
}

// ── cachedYouTubeSearch (cache-hit path) ──────────────────────────────────────

func TestCachedYouTubeSearch_CacheHit(t *testing.T) {
	db := testMongoDB(t)

	// Pre-seed the cache manually
	videos := []YouTubeVideo{
		{VideoID: "cached-search-vid1", Title: "AI Tutorial", ChannelTitle: "TechChan", ViewCount: 1000, PublishedAt: time.Now()},
	}
	data, _ := json.Marshal(videos)
	setCache(db, "search:ai+tutorial:10", string(data))

	// Now call cachedYouTubeSearch with an empty API key — it should return from cache
	// without calling the YouTube API (which would fail with empty key)
	result, err := cachedYouTubeSearch(db, "", "ai+tutorial", 10)
	if err != nil {
		t.Fatalf("expected cache hit without API call, got error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 video from cache, got %d", len(result))
	}
	if result[0].VideoID != "cached-search-vid1" {
		t.Errorf("expected 'cached-search-vid1', got %q", result[0].VideoID)
	}
}

// ── cachedVideoDetails (cache-hit path) ──────────────────────────────────────

func TestCachedVideoDetails_AllCached(t *testing.T) {
	db := testMongoDB(t)

	vid1 := YouTubeVideo{VideoID: "cache-vid-det-1", Title: "Cached Video 1", ChannelTitle: "Chan1", ViewCount: 500, PublishedAt: time.Now()}
	vid2 := YouTubeVideo{VideoID: "cache-vid-det-2", Title: "Cached Video 2", ChannelTitle: "Chan2", ViewCount: 300, PublishedAt: time.Now()}
	for _, vid := range []YouTubeVideo{vid1, vid2} {
		data, _ := json.Marshal(vid)
		setCache(db, "video:"+vid.VideoID, string(data))
	}

	// Call with no API key (would fail for real YouTube) — all should come from cache
	result, err := cachedVideoDetails(db, "", []string{"cache-vid-det-1", "cache-vid-det-2"})
	if err != nil {
		t.Fatalf("expected all-cached result, got error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 videos from cache, got %d", len(result))
	}
}

func TestCachedVideoDetails_EmptyList(t *testing.T) {
	db := testMongoDB(t)
	result, err := cachedVideoDetails(db, "", []string{})
	if err != nil {
		t.Fatalf("expected no error for empty list, got: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d", len(result))
	}
}

// ── cachedRedditSearch (cache-hit path) ──────────────────────────────────────

func TestCachedRedditSearch_CacheHit(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// Pre-seed the RedditCache collection with a fresh entry
	threads := []RedditThread{
		{ID: "abc123", Subreddit: "r/golang", Title: "Test thread", Score: 100, NumComments: 25},
	}
	data, _ := json.Marshal(threads)
	key := redditCacheKey("search", "golang", "unit testing", "year")
	db.RedditCache().InsertOne(ctx, map[string]any{
		"cacheKey": key,
		"data":     string(data),
		"cachedAt": time.Now(), // fresh → within TTL
	})

	// Should return cached threads without hitting Reddit API
	result, fromCache, err := cachedRedditSearch(db, "golang", "unit testing", "year", 10)
	if err != nil {
		t.Fatalf("expected cache hit, got error: %v", err)
	}
	if !fromCache {
		t.Error("expected fromCache=true for pre-seeded Reddit cache")
	}
	if len(result) != 1 {
		t.Errorf("expected 1 thread from cache, got %d", len(result))
	}
	if result[0].ID != "abc123" {
		t.Errorf("expected thread ID 'abc123', got %q", result[0].ID)
	}
}

func TestCachedRedditSearch_ExpiredCache(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// Seed with an EXPIRED entry (cachedAt > 7 days ago)
	threads := []RedditThread{{ID: "old123", Title: "Old thread"}}
	data, _ := json.Marshal(threads)
	key := redditCacheKey("search", "expired-sub", "stale term", "all")
	db.RedditCache().InsertOne(ctx, map[string]any{
		"cacheKey": key,
		"data":     string(data),
		"cachedAt": time.Now().Add(-8 * 24 * time.Hour), // 8 days old → expired
	})

	// Set up mock Reddit server that returns empty results
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"children":[]}}`))
	}))
	defer server.Close()
	withTestRedditBase(t, server)

	// Cache is expired → should go to Reddit API (returns empty from mock server)
	result, fromCache, err := cachedRedditSearch(db, "expired-sub", "stale term", "all", 5)
	if err != nil {
		t.Fatalf("expected no error even on expired cache, got: %v", err)
	}
	if fromCache {
		t.Error("expected fromCache=false for expired cache")
	}
	if len(result) != 0 {
		t.Errorf("expected empty from mock server, got %d threads", len(result))
	}
}

// ── cachedTranscript (cache-hit path) ────────────────────────────────────────

func TestCachedTranscript_CacheHit(t *testing.T) {
	db := testMongoDB(t)

	// Pre-seed the transcript cache
	videoID := "cached-transcript-vid"
	transcriptText := "This is a pre-cached transcript for the video."
	setCache(db, "transcript:"+videoID, transcriptText)

	// Now call cachedTranscript — should return from cache without calling YouTube
	got, fromCache, method, err := cachedTranscript(db, videoID)
	if err != nil {
		t.Fatalf("expected cache hit, got error: %v", err)
	}
	if !fromCache {
		t.Error("expected fromCache=true for cached transcript")
	}
	if got != transcriptText {
		t.Errorf("expected %q, got %q", transcriptText, got)
	}
	if method != "" {
		t.Errorf("expected empty method for cache hit, got %q", method)
	}
}
