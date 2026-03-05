package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── Reddit Search via httptest ──────────────────────────────────────────

func TestRedditSearch_Mock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/search.json") {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(loadFixture(t, "reddit_search.json"))
	}))
	defer server.Close()
	withTestRedditBase(t, server)

	threads, err := redditSearch("technology", "test brand", "year", 25)
	if err != nil {
		t.Fatalf("redditSearch failed: %v", err)
	}
	if len(threads) != 2 {
		t.Fatalf("expected 2 threads, got %d", len(threads))
	}
	if threads[0].ID != "abc123" {
		t.Errorf("first thread ID: got %q, want %q", threads[0].ID, "abc123")
	}
	if threads[0].Subreddit != "technology" {
		t.Errorf("subreddit: got %q", threads[0].Subreddit)
	}
	if threads[0].Score != 42 {
		t.Errorf("score: got %d, want 42", threads[0].Score)
	}
	if threads[1].ID != "def456" {
		t.Errorf("second thread ID: got %q", threads[1].ID)
	}
}

func TestRedditSearch_AllSubreddits(t *testing.T) {
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write(loadFixture(t, "reddit_search.json"))
	}))
	defer server.Close()
	withTestRedditBase(t, server)

	_, err := redditSearch("", "test brand", "year", 25)
	if err != nil {
		t.Fatalf("redditSearch all failed: %v", err)
	}
	// When subreddit is empty, should search /search.json (not /r/xxx/search.json)
	if strings.Contains(requestedPath, "/r/") {
		t.Errorf("all-subreddit search should not use /r/ path, got: %s", requestedPath)
	}
}

func TestRedditSearch_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()
	withTestRedditBase(t, server)

	_, err := redditSearch("test", "query", "year", 25)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

// ── Reddit Fetch Thread via httptest ────────────────────────────────────

func TestRedditFetchThread_Mock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, ".json") {
			t.Errorf("expected .json path, got: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(loadFixture(t, "reddit_thread.json"))
	}))
	defer server.Close()
	withTestRedditBase(t, server)

	thread, err := redditFetchThread("/r/technology/comments/abc123/discussion_about_test_brand/")
	if err != nil {
		t.Fatalf("redditFetchThread failed: %v", err)
	}
	if thread.ID != "abc123" {
		t.Errorf("thread ID: got %q, want %q", thread.ID, "abc123")
	}
	if thread.Title != "Discussion about test brand" {
		t.Errorf("title: got %q", thread.Title)
	}
	if len(thread.TopComments) != 2 {
		t.Fatalf("expected 2 top comments, got %d", len(thread.TopComments))
	}
	if thread.TopComments[0].Author != "commenter1" {
		t.Errorf("first comment author: got %q", thread.TopComments[0].Author)
	}
	if thread.TopComments[0].Score != 25 {
		t.Errorf("first comment score: got %d, want 25", thread.TopComments[0].Score)
	}
}

func TestRedditFetchThread_NormalizePermalink(t *testing.T) {
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write(loadFixture(t, "reddit_thread.json"))
	}))
	defer server.Close()
	withTestRedditBase(t, server)

	// Permalink without leading slash — should be normalized
	_, err := redditFetchThread("r/test/comments/xyz789/test_thread/")
	if err != nil {
		t.Fatalf("redditFetchThread failed: %v", err)
	}
	// Should have prepended / and removed trailing /
	if !strings.HasPrefix(requestedPath, "/r/test") {
		t.Errorf("path should start with /r/test, got: %s", requestedPath)
	}
}

func TestRedditFetchThread_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer server.Close()
	withTestRedditBase(t, server)

	_, err := redditFetchThread("/r/test/comments/abc123/test/")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}
