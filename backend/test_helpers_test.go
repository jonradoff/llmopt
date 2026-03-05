package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"llmopt/internal/saas"
)

// ── TestProvider: mock LLMProvider ──────────────────────────────────────

// TestProvider implements LLMProvider with configurable function fields.
// Default behavior returns canned responses suitable for most tests.
type TestProvider struct {
	id         string
	name       string
	models     []ModelDef
	smallModel string

	// Per-test overrides — set these to customize behavior.
	CallFn      func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error)
	StreamFn    func(ctx context.Context, apiKey string, body []byte, w http.ResponseWriter, flusher http.Flusher) (*StreamResult, error)
	VerifyKeyFn func(ctx context.Context, apiKey string) (string, error)
}

func newTestProvider() *TestProvider {
	return &TestProvider{
		id:         "test",
		name:       "Test Provider",
		models:     []ModelDef{{ID: "test-model", Name: "Test Model"}},
		smallModel: "test-model-small",
	}
}

func (p *TestProvider) Call(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
	if p.CallFn != nil {
		return p.CallFn(ctx, apiKey, model, prompt, maxTokens)
	}
	return `{"result": "test response"}`, nil
}

func (p *TestProvider) Stream(ctx context.Context, apiKey string, body []byte, w http.ResponseWriter, flusher http.Flusher) (*StreamResult, error) {
	if p.StreamFn != nil {
		return p.StreamFn(ctx, apiKey, body, w, flusher)
	}
	result := `{"site_summary":"Test summary","questions":[],"crawled_pages":[]}`
	sendSSE(w, flusher, "text", map[string]string{"content": "analyzing..."})
	return &StreamResult{RawText: result, ResultJSON: result}, nil
}

func (p *TestProvider) BuildStreamBody(model string, maxTokens int, prompt string, useWebSearch bool) ([]byte, error) {
	return json.Marshal(map[string]any{
		"model":      model,
		"max_tokens": maxTokens,
		"prompt":     prompt,
	})
}

func (p *TestProvider) VerifyKey(ctx context.Context, apiKey string) (string, error) {
	if p.VerifyKeyFn != nil {
		return p.VerifyKeyFn(ctx, apiKey)
	}
	return "active", nil
}

func (p *TestProvider) Models() []ModelDef     { return p.models }
func (p *TestProvider) SmallModel() string     { return p.smallModel }
func (p *TestProvider) Name() string           { return p.name }
func (p *TestProvider) ProviderID() string     { return p.id }

// ── Global state swap helpers ───────────────────────────────────────────

// withTestProviders temporarily replaces the global providers map, restoring on cleanup.
func withTestProviders(t *testing.T, tps ...*TestProvider) {
	t.Helper()
	original := providers
	providers = make(map[string]LLMProvider)
	for _, tp := range tps {
		providers[tp.id] = tp
	}
	t.Cleanup(func() { providers = original })
}

// withTestHTTPClients temporarily replaces all global HTTP clients with a test server's client.
func withTestHTTPClients(t *testing.T, server *httptest.Server) {
	t.Helper()
	client := server.Client()

	origLLM := llmHTTPClient
	origStream := llmStreamClient
	origYT := ytHTTPClient
	origReddit := redditClient

	llmHTTPClient = client
	llmStreamClient = client
	ytHTTPClient = client
	redditClient = client

	t.Cleanup(func() {
		llmHTTPClient = origLLM
		llmStreamClient = origStream
		ytHTTPClient = origYT
		redditClient = origReddit
	})
}

// withTestYouTubeBase temporarily swaps youtubeAPIBase and ytHTTPClient.
func withTestYouTubeBase(t *testing.T, server *httptest.Server) {
	t.Helper()
	origBase := youtubeAPIBase
	origClient := ytHTTPClient
	youtubeAPIBase = server.URL
	ytHTTPClient = server.Client()
	t.Cleanup(func() {
		youtubeAPIBase = origBase
		ytHTTPClient = origClient
	})
}

// withTestRedditBase temporarily swaps redditBaseURL and redditClient.
func withTestRedditBase(t *testing.T, server *httptest.Server) {
	t.Helper()
	origBase := redditBaseURL
	origClient := redditClient
	redditBaseURL = server.URL
	redditClient = server.Client()
	t.Cleanup(func() {
		redditBaseURL = origBase
		redditClient = origClient
	})
}

// ── Auth context helpers ────────────────────────────────────────────────

// testAuthContext creates a context with tenant/user auth values (role: admin).
func testAuthContext(tenantID, userID string) context.Context {
	return testAuthContextWithRole(tenantID, userID, "admin")
}

// testAuthContextWithRole creates a context with a specific role.
func testAuthContextWithRole(tenantID, userID, role string) context.Context {
	info := &saas.AuthInfo{
		UserID:   userID,
		TenantID: tenantID,
		Role:     role,
		Method:   "test",
	}
	return saas.SetAuthContext(context.Background(), info)
}

// ── Request helpers ─────────────────────────────────────────────────────

// testRequest creates an *http.Request with auth context and optional JSON body.
func testRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	req = req.WithContext(testAuthContext("test-tenant", "test-user"))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// testRequestOwner creates an *http.Request with owner role auth context.
func testRequestOwner(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	req := testRequest(t, method, path, body)
	req = req.WithContext(testAuthContextWithRole("test-tenant", "test-user", "owner"))
	return req
}

// testRequestNoAuth creates an *http.Request without auth context (non-SaaS mode).
func testRequestNoAuth(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(data)
	}
	return httptest.NewRequest(method, path, bodyReader)
}

// ── SSE response parsing ────────────────────────────────────────────────

// SSEEvent represents a parsed Server-Sent Event.
type SSEEvent struct {
	Type string
	Data json.RawMessage
}

// parseSSEResponse reads all SSE events from a response body.
func parseSSEResponse(t *testing.T, body []byte) []SSEEvent {
	t.Helper()
	var events []SSEEvent
	scanner := bufio.NewScanner(bytes.NewReader(body))
	var currentType string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			currentType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			events = append(events, SSEEvent{
				Type: currentType,
				Data: json.RawMessage(strings.TrimPrefix(line, "data: ")),
			})
			currentType = ""
		}
	}
	return events
}

// findSSEEvent returns the first SSE event of the given type, or nil.
func findSSEEvent(events []SSEEvent, eventType string) *SSEEvent {
	for _, e := range events {
		if e.Type == eventType {
			return &e
		}
	}
	return nil
}

// ── Fixture loader ──────────────────────────────────────────────────────

// loadFixture reads a file from testdata/.
func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("failed to load fixture %s: %v", name, err)
	}
	return data
}

// ── Misc helpers ────────────────────────────────────────────────────────

// testEncKey returns a deterministic 32-byte encryption key for tests.
func testEncKey() []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return key
}

// assertStatus checks the HTTP response status code.
func assertStatus(t *testing.T, w *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if w.Code != expected {
		t.Fatalf("expected status %d, got %d; body: %s", expected, w.Code, w.Body.String())
	}
}

// assertJSON unmarshals the response body and returns the decoded value.
func assertJSON[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(w.Body).Decode(&v); err != nil {
		t.Fatalf("failed to decode response JSON: %v; body: %s", err, w.Body.String())
	}
	return v
}

// ── Test for the test helpers themselves ─────────────────────────────────

func TestTestProviderImplementsInterface(t *testing.T) {
	var _ LLMProvider = newTestProvider()
}

func TestParseSSEResponse(t *testing.T) {
	raw := "event: status\ndata: {\"message\":\"working\"}\n\nevent: done\ndata: {\"result\":\"ok\"}\n\n"
	events := parseSSEResponse(t, []byte(raw))
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != "status" {
		t.Errorf("first event type: got %q, want %q", events[0].Type, "status")
	}
	if events[1].Type != "done" {
		t.Errorf("second event type: got %q, want %q", events[1].Type, "done")
	}

	e := findSSEEvent(events, "done")
	if e == nil {
		t.Fatal("findSSEEvent returned nil for 'done'")
	}

	missing := findSSEEvent(events, "nonexistent")
	if missing != nil {
		t.Fatal("findSSEEvent should return nil for missing event type")
	}
}

func TestTestProviderDefaults(t *testing.T) {
	tp := newTestProvider()

	// Call
	result, err := tp.Call(context.Background(), "key", "model", "prompt", 100)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if result == "" {
		t.Error("Call returned empty string")
	}

	// VerifyKey
	status, err := tp.VerifyKey(context.Background(), "key")
	if err != nil {
		t.Fatalf("VerifyKey failed: %v", err)
	}
	if status != "active" {
		t.Errorf("VerifyKey: got %q, want %q", status, "active")
	}

	// BuildStreamBody
	body, err := tp.BuildStreamBody("model", 100, "prompt", false)
	if err != nil {
		t.Fatalf("BuildStreamBody failed: %v", err)
	}
	if len(body) == 0 {
		t.Error("BuildStreamBody returned empty body")
	}

	// Stream
	w := httptest.NewRecorder()
	sr, err := tp.Stream(context.Background(), "key", body, w, w)
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}
	if sr.ResultJSON == "" {
		t.Error("Stream returned empty ResultJSON")
	}

	// Metadata
	if tp.ProviderID() != "test" {
		t.Errorf("ProviderID: got %q", tp.ProviderID())
	}
	if tp.Name() != "Test Provider" {
		t.Errorf("Name: got %q", tp.Name())
	}
	if len(tp.Models()) == 0 {
		t.Error("Models() is empty")
	}
	if tp.SmallModel() == "" {
		t.Error("SmallModel() is empty")
	}
}

func TestWithTestProviders(t *testing.T) {
	originalLen := len(providers)
	tp := newTestProvider()
	withTestProviders(t, tp)

	if len(providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(providers))
	}
	if getProvider("test") == nil {
		t.Error("test provider not found")
	}

	// After cleanup, providers will be restored
	_ = originalLen // verified by t.Cleanup
}

func TestTestAuthContext(t *testing.T) {
	ctx := testAuthContext("tenant-1", "user-1")
	if tid := saas.TenantIDFromContext(ctx); tid != "tenant-1" {
		t.Errorf("TenantIDFromContext: got %q, want %q", tid, "tenant-1")
	}
	if uid := saas.UserIDFromContext(ctx); uid != "user-1" {
		t.Errorf("UserIDFromContext: got %q, want %q", uid, "user-1")
	}
	if role := saas.MemberRoleFromContext(ctx); role != "admin" {
		t.Errorf("MemberRoleFromContext: got %q, want %q", role, "admin")
	}
}

func TestAssertStatus(t *testing.T) {
	w := httptest.NewRecorder()
	w.WriteHeader(http.StatusOK)
	assertStatus(t, w, http.StatusOK)
}

func TestLoadFixtureMissing(t *testing.T) {
	// We can't use loadFixture directly because it calls t.Fatalf.
	// Just verify the testdata directory concept works with a known-missing file.
	_, err := os.ReadFile(filepath.Join("testdata", "nonexistent.json"))
	if err == nil {
		t.Fatal("expected error for missing fixture")
	}
}

func TestAssertJSON(t *testing.T) {
	w := httptest.NewRecorder()
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w.Body, `{"name":"test","value":42}`)

	type resp struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	got := assertJSON[resp](t, w)
	if got.Name != "test" || got.Value != 42 {
		t.Errorf("assertJSON returned %+v", got)
	}
}
