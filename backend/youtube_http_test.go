package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── YouTube Search via httptest ─────────────────────────────────────────

func TestYouTubeSearch_Mock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/search"):
			w.Header().Set("Content-Type", "application/json")
			w.Write(loadFixture(t, "youtube_search.json"))
		case strings.Contains(r.URL.Path, "/videos"):
			w.Header().Set("Content-Type", "application/json")
			w.Write(loadFixture(t, "youtube_video_details.json"))
		default:
			t.Errorf("unexpected YouTube API path: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	withTestYouTubeBase(t, server)

	videos, err := youtubeSearch("fake-key", "test brand", 5)
	if err != nil {
		t.Fatalf("youtubeSearch failed: %v", err)
	}
	// Search returns 2 IDs, but video details fixture only has 1 item
	if len(videos) != 1 {
		t.Fatalf("expected 1 video, got %d", len(videos))
	}
	if videos[0].VideoID != "test_video_001" {
		t.Errorf("video ID: got %q, want %q", videos[0].VideoID, "test_video_001")
	}
	if videos[0].Title != "Test Video One" {
		t.Errorf("title: got %q", videos[0].Title)
	}
	if videos[0].ViewCount != 15000 {
		t.Errorf("view count: got %d, want 15000", videos[0].ViewCount)
	}
	if videos[0].LikeCount != 500 {
		t.Errorf("like count: got %d, want 500", videos[0].LikeCount)
	}
}

func TestYouTubeSearch_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":{"message":"API key not valid"}}`))
	}))
	defer server.Close()
	withTestYouTubeBase(t, server)

	_, err := youtubeSearch("bad-key", "test", 5)
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error should mention 403: %v", err)
	}
}

// ── YouTube Video Details ───────────────────────────────────────────────

func TestYouTubeVideoDetails_Mock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/videos") {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(loadFixture(t, "youtube_video_details.json"))
	}))
	defer server.Close()
	withTestYouTubeBase(t, server)

	videos, err := youtubeVideoDetails("fake-key", []string{"test_video_001"})
	if err != nil {
		t.Fatalf("youtubeVideoDetails failed: %v", err)
	}
	if len(videos) != 1 {
		t.Fatalf("expected 1 video, got %d", len(videos))
	}
	v := videos[0]
	if v.VideoID != "test_video_001" {
		t.Errorf("video ID: got %q", v.VideoID)
	}
	if v.Duration != "PT10M30S" {
		t.Errorf("duration: got %q", v.Duration)
	}
	if v.CommentCount != 75 {
		t.Errorf("comment count: got %d, want 75", v.CommentCount)
	}
}

func TestYouTubeVideoDetails_Empty(t *testing.T) {
	videos, err := youtubeVideoDetails("key", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if videos != nil {
		t.Errorf("expected nil for empty video IDs, got %v", videos)
	}
}

// ── YouTube Channel Info ────────────────────────────────────────────────

func TestYouTubeChannelInfo_Mock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/channels") {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(loadFixture(t, "youtube_channel.json"))
	}))
	defer server.Close()
	withTestYouTubeBase(t, server)

	ch, err := youtubeChannelInfo("fake-key", "UCtest_channel_001")
	if err != nil {
		t.Fatalf("youtubeChannelInfo failed: %v", err)
	}
	if ch.ChannelID != "UCtest_channel_001" {
		t.Errorf("channel ID: got %q", ch.ChannelID)
	}
	if ch.Title != "Test Channel" {
		t.Errorf("title: got %q", ch.Title)
	}
	if ch.SubscriberCount != 50000 {
		t.Errorf("subscribers: got %d, want 50000", ch.SubscriberCount)
	}
	if ch.VideoCount != 120 {
		t.Errorf("video count: got %d, want 120", ch.VideoCount)
	}
	if ch.ViewCount != 5000000 {
		t.Errorf("view count: got %d, want 5000000", ch.ViewCount)
	}
}

func TestYouTubeChannelInfo_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"kind":"youtube#channelListResponse","pageInfo":{"totalResults":0},"items":[]}`))
	}))
	defer server.Close()
	withTestYouTubeBase(t, server)

	_, err := youtubeChannelInfo("fake-key", "UCnonexistent")
	if err == nil {
		t.Fatal("expected error for missing channel")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found': %v", err)
	}
}

// ── Verify YouTube Key ──────────────────────────────────────────────────

func TestVerifyYouTubeKey_Active(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()
	withTestYouTubeBase(t, server)

	status := verifyYouTubeKey(context.Background(),"good-key")
	if status != "active" {
		t.Errorf("expected 'active', got %q", status)
	}
}

func TestVerifyYouTubeKey_Invalid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	withTestYouTubeBase(t, server)

	status := verifyYouTubeKey(context.Background(),"bad-key")
	if status != "invalid" {
		t.Errorf("expected 'invalid', got %q", status)
	}
}

// ── youtubeChannelVideos ────────────────────────────────────────────────

func TestYouTubeChannelVideos_Mock(t *testing.T) {
	// The function calls /search to get video IDs, then /videos for details
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/search"):
			// Return a search result with 2 video IDs
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"items":[{"id":{"videoId":"vid001"}},{"id":{"videoId":"vid002"}}]}`))
		case strings.Contains(r.URL.Path, "/videos"):
			w.Header().Set("Content-Type", "application/json")
			w.Write(loadFixture(t, "youtube_video_details.json"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	withTestYouTubeBase(t, server)

	videos, err := youtubeChannelVideos("fake-key", "UCtest123", 10)
	if err != nil {
		t.Fatalf("youtubeChannelVideos failed: %v", err)
	}
	if len(videos) == 0 {
		t.Error("expected at least 1 video")
	}
}

func TestYouTubeChannelVideos_SearchAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"API key invalid"}`))
	}))
	defer server.Close()
	withTestYouTubeBase(t, server)

	_, err := youtubeChannelVideos("bad-key", "UCtest123", 10)
	if err == nil {
		t.Fatal("expected error for API error")
	}
}

func TestYouTubeChannelVideos_NoVideos(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return empty search results
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()
	withTestYouTubeBase(t, server)

	videos, err := youtubeChannelVideos("fake-key", "UCempty", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(videos) != 0 {
		t.Errorf("expected 0 videos for empty channel, got %d", len(videos))
	}
}

// ── resolveChannelID ────────────────────────────────────────────────────

func TestResolveChannelID_DirectChannelID(t *testing.T) {
	// /channel/UC... format returns ID directly without API call
	result, err := resolveChannelID("fake-key", "https://www.youtube.com/channel/UCtest_channel_001")
	if err != nil {
		t.Fatalf("resolveChannelID failed: %v", err)
	}
	if result != "UCtest_channel_001" {
		t.Errorf("expected UCtest_channel_001, got %q", result)
	}
}

func TestResolveChannelID_ByHandle(t *testing.T) {
	// /@handle format → calls forHandle API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return a channel with a specific ID
		w.Write([]byte(`{"items":[{"id":"UCtest_handle_001"}]}`))
	}))
	defer server.Close()
	withTestYouTubeBase(t, server)

	result, err := resolveChannelID("fake-key", "https://www.youtube.com/@testhandle")
	if err != nil {
		t.Fatalf("resolveChannelID by handle failed: %v", err)
	}
	if result != "UCtest_handle_001" {
		t.Errorf("expected UCtest_handle_001, got %q", result)
	}
}

func TestResolveChannelID_ByUsername(t *testing.T) {
	// /user/Username format → calls forUsername API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"items":[{"id":"UCtest_user_001"}]}`))
	}))
	defer server.Close()
	withTestYouTubeBase(t, server)

	result, err := resolveChannelID("fake-key", "https://www.youtube.com/user/testuser")
	if err != nil {
		t.Fatalf("resolveChannelID by username failed: %v", err)
	}
	if result != "UCtest_user_001" {
		t.Errorf("expected UCtest_user_001, got %q", result)
	}
}

func TestResolveChannelID_ByCustomPath(t *testing.T) {
	// /c/CustomName → calls forUsername
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"items":[{"id":"UCcustom_001"}]}`))
	}))
	defer server.Close()
	withTestYouTubeBase(t, server)

	result, err := resolveChannelID("fake-key", "https://www.youtube.com/c/MyChannel")
	if err != nil {
		t.Fatalf("resolveChannelID by custom path failed: %v", err)
	}
	if result != "UCcustom_001" {
		t.Errorf("expected UCcustom_001, got %q", result)
	}
}

func TestResolveChannelID_BarePath(t *testing.T) {
	// /SomeName → try as handle
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"items":[{"id":"UCbare_001"}]}`))
	}))
	defer server.Close()
	withTestYouTubeBase(t, server)

	result, err := resolveChannelID("fake-key", "https://www.youtube.com/SomeName")
	if err != nil {
		t.Fatalf("resolveChannelID by bare path failed: %v", err)
	}
	if result != "UCbare_001" {
		t.Errorf("expected UCbare_001, got %q", result)
	}
}

func TestResolveChannelID_Empty(t *testing.T) {
	_, err := resolveChannelID("fake-key", "")
	if err == nil {
		t.Error("expected error for empty URL")
	}
}

func TestResolveChannelID_NotResolvable(t *testing.T) {
	// URL that doesn't match any format
	_, err := resolveChannelID("fake-key", "https://www.youtube.com/some/deeply/nested/path")
	if err == nil {
		t.Error("expected error for non-resolvable URL")
	}
}
