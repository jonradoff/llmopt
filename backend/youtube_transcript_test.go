package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── Test helpers ──────────────────────────────────────────────────────────────

// withTestInnertubeURL temporarily swaps the innertubePlayerURL global.
func withTestInnertubeURL(t *testing.T, url string) {
	t.Helper()
	orig := innertubePlayerURL
	innertubePlayerURL = url
	t.Cleanup(func() { innertubePlayerURL = orig })
}

// makeCaptionServer creates a test server that:
//   - On POST: returns a valid player JSON with captionURL → server.URL+"/captions"
//   - On GET /captions: returns simple transcript XML
func makeCaptionServer(t *testing.T) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			playerResp := map[string]any{
				"playabilityStatus": map[string]any{"status": "OK"},
				"captions": map[string]any{
					"playerCaptionsTracklistRenderer": map[string]any{
						"captionTracks": []any{
							map[string]any{
								"languageCode": "en",
								"baseUrl":      srv.URL + "/captions",
							},
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(playerResp)
		} else {
			w.Header().Set("Content-Type", "text/xml")
			fmt.Fprint(w, `<transcript><text start="0.0" dur="2.0">Hello world</text><text start="2.0" dur="1.5">This is a test</text></transcript>`)
		}
	}))
	return srv
}

// ── innertubePlayerRequest ────────────────────────────────────────────────────

func TestInnertubePlayerRequest_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"playabilityStatus":{"status":"OK"},"videoDetails":{"videoId":"test123"}}`)
	}))
	defer server.Close()
	withTestInnertubeURL(t, server.URL)

	resp, err := innertubePlayerRequest("test123", []byte(`{}`), "TestUA/1.0", server.Client())
	if err != nil {
		t.Fatalf("innertubePlayerRequest failed: %v", err)
	}
	if _, ok := resp["playabilityStatus"]; !ok {
		t.Error("expected playabilityStatus in response")
	}
}

func TestInnertubePlayerRequest_403(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	withTestInnertubeURL(t, server.URL)

	_, err := innertubePlayerRequest("test123", []byte(`{}`), "TestUA/1.0", server.Client())
	if err == nil {
		t.Fatal("expected error for 403")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error should mention 403: %v", err)
	}
}

func TestInnertubePlayerRequest_429(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	withTestInnertubeURL(t, server.URL)

	_, err := innertubePlayerRequest("test123", []byte(`{}`), "TestUA/1.0", server.Client())
	if err == nil {
		t.Fatal("expected error for 429")
	}
	// Should be wrapped as errBlocked
	if !strings.Contains(err.Error(), "blocked") && !strings.Contains(err.Error(), "429") {
		t.Errorf("expected blocked/429 error, got: %v", err)
	}
}

func TestInnertubePlayerRequest_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "not-valid-json!!!")
	}))
	defer server.Close()
	withTestInnertubeURL(t, server.URL)

	_, err := innertubePlayerRequest("test123", []byte(`{}`), "TestUA/1.0", server.Client())
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestInnertubePlayerRequest_NilClient(t *testing.T) {
	// With nil client, uses default; port 0 = connection refused
	withTestInnertubeURL(t, "http://127.0.0.1:0")
	_, err := innertubePlayerRequest("test123", []byte(`{}`), "TestUA/1.0", nil)
	if err == nil {
		t.Fatal("expected error for unreachable server with nil client")
	}
}

// ── extractCaptionURL (additional cases) ─────────────────────────────────────

func TestExtractCaptionURL_Unplayable(t *testing.T) {
	playerResp := map[string]any{
		"playabilityStatus": map[string]any{
			"status": "UNPLAYABLE",
			"reason": "Video unavailable",
		},
	}
	_, err := extractCaptionURL("test123", playerResp)
	if err == nil {
		t.Fatal("expected error for UNPLAYABLE")
	}
	if !strings.Contains(err.Error(), "UNPLAYABLE") {
		t.Errorf("expected UNPLAYABLE in error, got: %v", err)
	}
}

func TestExtractCaptionURL_ErrorStatus(t *testing.T) {
	playerResp := map[string]any{
		"playabilityStatus": map[string]any{
			"status": "ERROR",
			"reason": "This video does not exist",
		},
	}
	_, err := extractCaptionURL("test123", playerResp)
	if err == nil {
		t.Fatal("expected error for ERROR status")
	}
}

func TestExtractCaptionURL_UnknownPlayabilityStatus(t *testing.T) {
	// Unknown status (not LOGIN_REQUIRED/ERROR/UNPLAYABLE) → errNoCaptions (not errBlocked)
	playerResp := map[string]any{
		"playabilityStatus": map[string]any{
			"status": "LIVE_STREAM_OFFLINE",
		},
	}
	_, err := extractCaptionURL("test123", playerResp)
	if err == nil {
		t.Fatal("expected error for LIVE_STREAM_OFFLINE")
	}
}

func TestExtractCaptionURL_NoPlayabilityStatus(t *testing.T) {
	// Missing playabilityStatus entirely — should still check captions
	playerResp := map[string]any{
		"captions": map[string]any{
			"playerCaptionsTracklistRenderer": map[string]any{
				"captionTracks": []any{
					map[string]any{
						"languageCode": "en",
						"baseUrl":      "https://example.com/captions",
					},
				},
			},
		},
	}
	url, err := extractCaptionURL("test123", playerResp)
	if err != nil {
		t.Fatalf("expected success with missing playabilityStatus: %v", err)
	}
	if url == "" {
		t.Error("expected non-empty caption URL")
	}
}

func TestExtractCaptionURL_TrackWithoutBaseURL(t *testing.T) {
	// First track has empty baseUrl, second has valid URL → should use second
	playerResp := map[string]any{
		"playabilityStatus": map[string]any{"status": "OK"},
		"captions": map[string]any{
			"playerCaptionsTracklistRenderer": map[string]any{
				"captionTracks": []any{
					map[string]any{
						"languageCode": "en",
						"baseUrl":      "", // empty — should be skipped
					},
					map[string]any{
						"languageCode": "en",
						"baseUrl":      "https://example.com/captions",
					},
				},
			},
		},
	}
	url, err := extractCaptionURL("test123", playerResp)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	if url != "https://example.com/captions" {
		t.Errorf("expected non-empty URL, got %q", url)
	}
}

// ── fetchCaptionXML ───────────────────────────────────────────────────────────

const testCaptionXML = `<?xml version="1.0" encoding="utf-8" ?><transcript><text start="0.0" dur="2.0">Hello world</text><text start="2.0" dur="1.5">This is a test</text></transcript>`

func TestFetchCaptionXML_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		fmt.Fprint(w, testCaptionXML)
	}))
	defer server.Close()

	text, err := fetchCaptionXML(server.URL, "TestUA/1.0", server.Client())
	if err != nil {
		t.Fatalf("fetchCaptionXML failed: %v", err)
	}
	if !strings.Contains(text, "Hello world") {
		t.Errorf("expected 'Hello world' in text, got: %q", text)
	}
	if !strings.Contains(text, "This is a test") {
		t.Errorf("expected 'This is a test' in text, got: %q", text)
	}
}

func TestFetchCaptionXML_404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "not found")
	}))
	defer server.Close()

	_, err := fetchCaptionXML(server.URL, "TestUA/1.0", server.Client())
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention 404: %v", err)
	}
}

func TestFetchCaptionXML_429(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, "rate limited")
	}))
	defer server.Close()

	_, err := fetchCaptionXML(server.URL, "TestUA/1.0", server.Client())
	if err == nil {
		t.Fatal("expected error for 429")
	}
}

func TestFetchCaptionXML_EmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Write nothing — empty body
	}))
	defer server.Close()

	_, err := fetchCaptionXML(server.URL, "TestUA/1.0", server.Client())
	if err == nil {
		t.Fatal("expected error for empty body")
	}
}

func TestFetchCaptionXML_NilClient(t *testing.T) {
	// nil client + unreachable URL → error
	_, err := fetchCaptionXML("http://127.0.0.1:0/captions", "TestUA/1.0", nil)
	if err == nil {
		t.Fatal("expected error for unreachable server with nil client")
	}
}

// ── fetchTranscriptAndroidWith ────────────────────────────────────────────────

func TestFetchTranscriptAndroidWith_Success(t *testing.T) {
	server := makeCaptionServer(t)
	defer server.Close()
	withTestInnertubeURL(t, server.URL)

	text, err := fetchTranscriptAndroidWith("test_video_id", server.Client())
	if err != nil {
		t.Fatalf("fetchTranscriptAndroidWith failed: %v", err)
	}
	if !strings.Contains(text, "Hello world") {
		t.Errorf("expected 'Hello world', got: %q", text)
	}
}

func TestFetchTranscriptAndroidWith_PlayerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	withTestInnertubeURL(t, server.URL)

	_, err := fetchTranscriptAndroidWith("test_video_id", server.Client())
	if err == nil {
		t.Fatal("expected error for player 403")
	}
}

func TestFetchTranscriptAndroidWith_NoCaptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return valid player response but without captions section
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"playabilityStatus": map[string]any{"status": "OK"},
		})
	}))
	defer server.Close()
	withTestInnertubeURL(t, server.URL)

	_, err := fetchTranscriptAndroidWith("test_video_id", server.Client())
	if err == nil {
		t.Fatal("expected error for no captions")
	}
}

// ── fetchTranscriptWebWith ────────────────────────────────────────────────────

func TestFetchTranscriptWebWith_Success(t *testing.T) {
	server := makeCaptionServer(t)
	defer server.Close()
	withTestInnertubeURL(t, server.URL)

	text, err := fetchTranscriptWebWith("test_video_id", server.Client())
	if err != nil {
		t.Fatalf("fetchTranscriptWebWith failed: %v", err)
	}
	if !strings.Contains(text, "Hello world") {
		t.Errorf("expected 'Hello world', got: %q", text)
	}
}

func TestFetchTranscriptWebWith_429(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	withTestInnertubeURL(t, server.URL)

	_, err := fetchTranscriptWebWith("test_video_id", server.Client())
	if err == nil {
		t.Fatal("expected error for 429")
	}
}

func TestFetchTranscriptWebWith_NoCaptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"playabilityStatus": map[string]any{"status": "OK"},
		})
	}))
	defer server.Close()
	withTestInnertubeURL(t, server.URL)

	_, err := fetchTranscriptWebWith("test_video_id", server.Client())
	if err == nil {
		t.Fatal("expected error for no captions")
	}
}

// ── parseTimedTextXML (additional cases not in youtube_test.go) ───────────────

func TestParseTimedTextXML_SRV3ParagraphText(t *testing.T) {
	// srv3 <body><p> with direct text content (no <s> children)
	xmlData := []byte(`<timedtext><body><p t="0" d="2000">Direct paragraph text</p></body></timedtext>`)
	text, err := parseTimedTextXML(xmlData)
	if err != nil {
		t.Fatalf("parseTimedTextXML failed: %v", err)
	}
	if !strings.Contains(text, "Direct paragraph text") {
		t.Errorf("expected direct paragraph text, got: %q", text)
	}
}

func TestParseTimedTextXML_WhitespaceOnly(t *testing.T) {
	// Text elements with only whitespace → should be omitted (trimmed to empty)
	xmlData := []byte(`<transcript><text>   </text><text>  </text></transcript>`)
	text, err := parseTimedTextXML(xmlData)
	if err != nil {
		t.Fatalf("parseTimedTextXML failed: %v", err)
	}
	if text != "" {
		t.Errorf("expected empty for whitespace-only text, got: %q", text)
	}
}
