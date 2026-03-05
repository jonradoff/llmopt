package main

// Tests for utility functions (loadEnvFile, digestVideoBatch, digestThirdPartyVideos, etc.)

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ── loadEnvFile ──────────────────────────────────────────────────────────────

func TestLoadEnvFile_FileNotFound(t *testing.T) {
	// loadEnvFile silently ignores missing files
	loadEnvFile("/nonexistent/path/.env.test.nope")
	// No panic or error
}

func TestLoadEnvFile_BasicKeyValue(t *testing.T) {
	// Create a temp env file
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env.test")
	os.WriteFile(envFile, []byte("TEST_LOAD_ENV_KEY1=hello_world\nTEST_LOAD_ENV_KEY2=test_value\n"), 0600)
	defer os.Unsetenv("TEST_LOAD_ENV_KEY1")
	defer os.Unsetenv("TEST_LOAD_ENV_KEY2")

	loadEnvFile(envFile)

	if os.Getenv("TEST_LOAD_ENV_KEY1") != "hello_world" {
		t.Errorf("expected TEST_LOAD_ENV_KEY1=hello_world, got %q", os.Getenv("TEST_LOAD_ENV_KEY1"))
	}
	if os.Getenv("TEST_LOAD_ENV_KEY2") != "test_value" {
		t.Errorf("expected TEST_LOAD_ENV_KEY2=test_value, got %q", os.Getenv("TEST_LOAD_ENV_KEY2"))
	}
}

func TestLoadEnvFile_IgnoresComments(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	os.WriteFile(envFile, []byte("# This is a comment\nTEST_ENV_COMMENT_KEY=val\n# Another comment\n"), 0600)
	defer os.Unsetenv("TEST_ENV_COMMENT_KEY")

	loadEnvFile(envFile)

	if os.Getenv("TEST_ENV_COMMENT_KEY") != "val" {
		t.Errorf("expected TEST_ENV_COMMENT_KEY=val, got %q", os.Getenv("TEST_ENV_COMMENT_KEY"))
	}
}

func TestLoadEnvFile_IgnoresEmptyLines(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	os.WriteFile(envFile, []byte("\n\n\nTEST_ENV_EMPTY_KEY=nonempty\n\n"), 0600)
	defer os.Unsetenv("TEST_ENV_EMPTY_KEY")

	loadEnvFile(envFile)

	if os.Getenv("TEST_ENV_EMPTY_KEY") != "nonempty" {
		t.Errorf("expected TEST_ENV_EMPTY_KEY=nonempty, got %q", os.Getenv("TEST_ENV_EMPTY_KEY"))
	}
}

func TestLoadEnvFile_QuotedValues(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	content := `TEST_ENV_QUOTED_DOUBLE="double quoted value"
TEST_ENV_QUOTED_SINGLE='single quoted value'
`
	os.WriteFile(envFile, []byte(content), 0600)
	defer os.Unsetenv("TEST_ENV_QUOTED_DOUBLE")
	defer os.Unsetenv("TEST_ENV_QUOTED_SINGLE")

	loadEnvFile(envFile)

	if os.Getenv("TEST_ENV_QUOTED_DOUBLE") != "double quoted value" {
		t.Errorf("expected unquoted value, got %q", os.Getenv("TEST_ENV_QUOTED_DOUBLE"))
	}
	if os.Getenv("TEST_ENV_QUOTED_SINGLE") != "single quoted value" {
		t.Errorf("expected unquoted value, got %q", os.Getenv("TEST_ENV_QUOTED_SINGLE"))
	}
}

func TestLoadEnvFile_DoesNotOverwriteExistingEnv(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	os.WriteFile(envFile, []byte("TEST_ENV_EXISTING=from_file\n"), 0600)

	// Set the env var before loading
	os.Setenv("TEST_ENV_EXISTING", "from_env")
	defer os.Unsetenv("TEST_ENV_EXISTING")

	loadEnvFile(envFile)

	// Should NOT be overwritten
	if os.Getenv("TEST_ENV_EXISTING") != "from_env" {
		t.Errorf("expected existing env var to be preserved, got %q", os.Getenv("TEST_ENV_EXISTING"))
	}
}

func TestLoadEnvFile_NoEqualsSign(t *testing.T) {
	// Lines without '=' should be silently skipped
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	os.WriteFile(envFile, []byte("NO_EQUALS_SIGN\nVALID_KEY=valid\n"), 0600)
	defer os.Unsetenv("VALID_KEY")

	loadEnvFile(envFile)

	if os.Getenv("VALID_KEY") != "valid" {
		t.Errorf("expected VALID_KEY=valid, got %q", os.Getenv("VALID_KEY"))
	}
}

// ── digestVideoBatch ──────────────────────────────────────────────────────────

func TestDigestVideoBatch_Success(t *testing.T) {
	tp := newTestProvider()
	tp.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		return `{"top_creators":["TestChannel"],"topics_covered":["AI"],"sentiment_tally":{"positive":1,"neutral":0,"negative":0,"none":0},"notable_quotes":[],"content_gaps":[],"summary":"A good video landscape"}`, nil
	}

	videos := []YouTubeVideo{
		{VideoID: "vid1", Title: "AI Basics", ChannelTitle: "TestChannel", ViewCount: 500, PublishedAt: time.Now()},
	}
	assessments := map[string]*VideoAssessment{}

	digest, err := digestVideoBatch(context.Background(), tp, "fake-key", videos, assessments, "example.com", []string{"AI"}, 0)
	if err != nil {
		t.Fatalf("digestVideoBatch failed: %v", err)
	}
	if digest == nil {
		t.Fatal("expected non-nil digest")
	}
	if len(digest.TopCreators) == 0 {
		t.Error("expected non-empty TopCreators")
	}
	if digest.BatchIndex != 0 {
		t.Errorf("expected BatchIndex=0, got %d", digest.BatchIndex)
	}
	if digest.VideoCount != 1 {
		t.Errorf("expected VideoCount=1, got %d", digest.VideoCount)
	}
}

func TestDigestVideoBatch_WithAssessments(t *testing.T) {
	tp := newTestProvider()
	tp.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		// Verify prompt includes the assessment data
		if !strings.Contains(prompt, "keyword=80") {
			return "", fmt.Errorf("expected keyword score in prompt, got: %s", prompt[:100])
		}
		return `{"top_creators":[],"topics_covered":["test"],"sentiment_tally":{"positive":1,"neutral":0,"negative":0,"none":0},"notable_quotes":["great quote"],"content_gaps":[],"summary":"Summary"}`, nil
	}

	videos := []YouTubeVideo{
		{VideoID: "vid2", Title: "Test Video", ChannelTitle: "Chan", ViewCount: 1000, PublishedAt: time.Now()},
	}
	assessments := map[string]*VideoAssessment{
		"vid2": {VideoID: "vid2", KeywordAlignment: 80, Quotability: 70, InfoDensity: 60, KeyQuotes: []string{"great quote"}, BrandSentiment: "positive", HasTranscript: true},
	}

	digest, err := digestVideoBatch(context.Background(), tp, "fake-key", videos, assessments, "example.com", nil, 1)
	if err != nil {
		t.Fatalf("digestVideoBatch failed: %v", err)
	}
	if digest.BatchIndex != 1 {
		t.Errorf("expected BatchIndex=1, got %d", digest.BatchIndex)
	}
}

func TestDigestVideoBatch_ProviderError(t *testing.T) {
	tp := newTestProvider()
	sentinel := errors.New("provider error")
	tp.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		return "", sentinel
	}

	videos := []YouTubeVideo{{VideoID: "vid3", Title: "Test", PublishedAt: time.Now()}}
	_, err := digestVideoBatch(context.Background(), tp, "fake-key", videos, nil, "example.com", nil, 0)
	if err == nil {
		t.Fatal("expected error from provider")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got: %v", err)
	}
}

func TestDigestVideoBatch_InvalidJSON(t *testing.T) {
	tp := newTestProvider()
	tp.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		return "this is not JSON", nil
	}

	videos := []YouTubeVideo{{VideoID: "vid4", Title: "Test", PublishedAt: time.Now()}}
	_, err := digestVideoBatch(context.Background(), tp, "fake-key", videos, nil, "example.com", nil, 0)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestDigestVideoBatch_ContextCancelled(t *testing.T) {
	tp := newTestProvider()
	tp.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		return "", ErrOverloaded // triggers retry
	}

	videos := []YouTubeVideo{{VideoID: "vid5", Title: "Test", PublishedAt: time.Now()}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	_, err := digestVideoBatch(ctx, tp, "fake-key", videos, nil, "example.com", nil, 0)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestDigestVideoBatch_JSONWithMarkdown(t *testing.T) {
	tp := newTestProvider()
	tp.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		return "```json\n{\"top_creators\":[],\"topics_covered\":[],\"sentiment_tally\":{\"positive\":0,\"neutral\":0,\"negative\":0,\"none\":0},\"notable_quotes\":[],\"content_gaps\":[],\"summary\":\"Test\"}\n```", nil
	}

	videos := []YouTubeVideo{{VideoID: "vid6", Title: "Test", PublishedAt: time.Now()}}
	digest, err := digestVideoBatch(context.Background(), tp, "fake-key", videos, nil, "example.com", nil, 2)
	if err != nil {
		t.Fatalf("expected success with markdown JSON: %v", err)
	}
	if digest.Summary != "Test" {
		t.Errorf("expected Summary=Test, got %q", digest.Summary)
	}
}

// ── digestThirdPartyVideos ─────────────────────────────────────────────────────

func TestDigestThirdPartyVideos_EmptyVideos(t *testing.T) {
	tp := newTestProvider()
	w := httptest.NewRecorder()

	result, err := digestThirdPartyVideos(context.Background(), tp, "fake-key", nil, nil, "example.com", nil, w, w)
	if err != nil {
		t.Fatalf("expected no error for empty videos, got: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result for empty videos, got %d items", len(result))
	}
}

func TestDigestThirdPartyVideos_Success(t *testing.T) {
	tp := newTestProvider()
	tp.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		return `{"top_creators":["Chan"],"topics_covered":["AI"],"sentiment_tally":{"positive":1,"neutral":0,"negative":0,"none":0},"notable_quotes":[],"content_gaps":[],"summary":"Good content"}`, nil
	}
	w := httptest.NewRecorder()

	videos := []YouTubeVideo{
		{VideoID: "tpv1", Title: "Third-Party Video", ChannelTitle: "Chan", ViewCount: 200, PublishedAt: time.Now()},
		{VideoID: "tpv2", Title: "Another Video", ChannelTitle: "Chan2", ViewCount: 300, PublishedAt: time.Now()},
	}

	result, err := digestThirdPartyVideos(context.Background(), tp, "fake-key", videos, nil, "example.com", []string{"AI"}, w, w)
	if err != nil {
		t.Fatalf("digestThirdPartyVideos failed: %v", err)
	}
	if len(result) == 0 {
		t.Error("expected at least one BatchDigest in result")
	}
	if result[0].VideoCount == 0 {
		t.Error("expected VideoCount > 0")
	}
}

func TestDigestThirdPartyVideos_AllBatchesFail(t *testing.T) {
	tp := newTestProvider()
	tp.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		return "", errors.New("permanent failure")
	}
	w := httptest.NewRecorder()

	videos := []YouTubeVideo{
		{VideoID: "fail1", Title: "Fail Video", PublishedAt: time.Now()},
	}

	_, err := digestThirdPartyVideos(context.Background(), tp, "fake-key", videos, nil, "example.com", nil, w, w)
	if err == nil {
		t.Fatal("expected error when all batches fail")
	}
}

func TestDigestThirdPartyVideos_WithAssessments(t *testing.T) {
	tp := newTestProvider()
	tp.CallFn = func(ctx context.Context, apiKey, model, prompt string, maxTokens int) (string, error) {
		return `{"top_creators":[],"topics_covered":["test"],"sentiment_tally":{"positive":1,"neutral":0,"negative":0,"none":0},"notable_quotes":[],"content_gaps":[],"summary":"Assessed"}`, nil
	}
	w := httptest.NewRecorder()

	videos := []YouTubeVideo{
		{VideoID: "av1", Title: "Assessed Video", ChannelTitle: "Chan", ViewCount: 100, PublishedAt: time.Now()},
	}
	assessments := map[string]*VideoAssessment{
		"av1": {VideoID: "av1", KeywordAlignment: 75, Quotability: 60, InfoDensity: 50, BrandSentiment: "positive", HasTranscript: true},
	}

	result, err := digestThirdPartyVideos(context.Background(), tp, "fake-key", videos, assessments, "example.com", nil, w, w)
	if err != nil {
		t.Fatalf("digestThirdPartyVideos with assessments failed: %v", err)
	}
	if len(result) == 0 {
		t.Error("expected non-empty result with assessments")
	}
}
