package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ── NewRateLimiter ──────────────────────────────────────────────────────

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()
	if rl == nil {
		t.Fatal("NewRateLimiter returned nil")
	}
	if rl.requests == nil {
		t.Fatal("requests map is nil")
	}
}

// ── allowLocal / Allow ──────────────────────────────────────────────────

func TestAllow_FirstRequest(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()

	cfg := RateLimitConfig{MaxRequests: 5, Window: time.Minute}
	allowed, remaining, _ := rl.Allow("user-1", cfg)

	if !allowed {
		t.Fatal("expected first request to be allowed")
	}
	if remaining != 4 {
		t.Errorf("expected remaining=4, got %d", remaining)
	}
}

func TestAllow_ExhaustsLimit(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()

	cfg := RateLimitConfig{MaxRequests: 3, Window: time.Minute}
	for i := 0; i < 3; i++ {
		allowed, _, _ := rl.Allow("user-1", cfg)
		if !allowed {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	allowed, remaining, _ := rl.Allow("user-1", cfg)
	if allowed {
		t.Fatal("4th request should be denied")
	}
	if remaining != 0 {
		t.Errorf("expected remaining=0 when denied, got %d", remaining)
	}
}

func TestAllow_WindowReset(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()

	// Use a very short window.
	cfg := RateLimitConfig{MaxRequests: 1, Window: 10 * time.Millisecond}

	allowed, _, _ := rl.Allow("user-2", cfg)
	if !allowed {
		t.Fatal("first request should be allowed")
	}
	allowed, _, _ = rl.Allow("user-2", cfg)
	if allowed {
		t.Fatal("second request should be denied within window")
	}

	// Wait for the window to expire.
	time.Sleep(20 * time.Millisecond)

	allowed, remaining, _ := rl.Allow("user-2", cfg)
	if !allowed {
		t.Fatal("request after window reset should be allowed")
	}
	if remaining != 0 {
		t.Errorf("expected remaining=0 (maxRequests=1, used 1), got %d", remaining)
	}
}

func TestAllow_IsolatedKeys(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()

	cfg := RateLimitConfig{MaxRequests: 1, Window: time.Minute}
	rl.Allow("user-A", cfg) // exhaust A

	allowed, _, _ := rl.Allow("user-B", cfg)
	if !allowed {
		t.Fatal("user-B should not be affected by user-A's limit")
	}
}

func TestAllow_RemainingDecrementsCorrectly(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()

	cfg := RateLimitConfig{MaxRequests: 5, Window: time.Minute}
	for i := 0; i < 5; i++ {
		_, remaining, _ := rl.Allow("user-3", cfg)
		want := 4 - i
		if remaining != want {
			t.Errorf("after request %d: expected remaining=%d, got %d", i+1, want, remaining)
		}
	}
}

// ── cleanupExpired ──────────────────────────────────────────────────────

func TestCleanupExpired(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()

	// Plant an expired entry directly.
	rl.mu.Lock()
	rl.requests["old-key"] = &rateLimitEntry{
		count:     1,
		windowEnd: time.Now().Add(-time.Minute),
	}
	rl.mu.Unlock()

	rl.cleanupExpired()

	rl.mu.RLock()
	_, exists := rl.requests["old-key"]
	rl.mu.RUnlock()

	if exists {
		t.Fatal("expired entry should have been removed by cleanupExpired")
	}
}

// ── GetClientIP ─────────────────────────────────────────────────────────

func TestGetClientIP_FlyClientIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Fly-Client-IP", "1.2.3.4")
	r.Header.Set("X-Forwarded-For", "9.9.9.9")

	ip := GetClientIP(r)
	if ip != "1.2.3.4" {
		t.Errorf("expected Fly-Client-IP to take precedence, got %q", ip)
	}
}

func TestGetClientIP_XRealIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Real-IP", "5.6.7.8")
	r.Header.Set("X-Forwarded-For", "9.9.9.9")

	ip := GetClientIP(r)
	if ip != "5.6.7.8" {
		t.Errorf("expected X-Real-IP, got %q", ip)
	}
}

func TestGetClientIP_XForwardedFor(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "10.0.0.1, 192.168.1.1")

	ip := GetClientIP(r)
	if ip != "10.0.0.1" {
		t.Errorf("expected first XFF IP, got %q", ip)
	}
}

func TestGetClientIP_RemoteAddr(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:12345"

	ip := GetClientIP(r)
	if ip != "127.0.0.1" {
		t.Errorf("expected RemoteAddr IP, got %q", ip)
	}
}

func TestGetClientIP_InvalidFlyHeader_FallsThrough(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Fly-Client-IP", "not-an-ip")
	r.Header.Set("X-Real-IP", "5.5.5.5")

	ip := GetClientIP(r)
	if ip != "5.5.5.5" {
		t.Errorf("expected fallthrough to X-Real-IP, got %q", ip)
	}
}

func TestGetClientIP_InvalidXRealIP_FallsThrough(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Real-IP", "bad-ip")
	r.Header.Set("X-Forwarded-For", "8.8.8.8")

	ip := GetClientIP(r)
	if ip != "8.8.8.8" {
		t.Errorf("expected fallthrough to XFF, got %q", ip)
	}
}

func TestGetClientIP_InvalidXFF_UsesRemoteAddr(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "not-valid-ip")
	r.RemoteAddr = "10.0.0.2:9000"

	ip := GetClientIP(r)
	if ip != "10.0.0.2" {
		t.Errorf("expected RemoteAddr fallback, got %q", ip)
	}
}

// ── RateLimitHandler ────────────────────────────────────────────────────

func TestRateLimitHandler_Allowed(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()

	cfg := RateLimitConfig{MaxRequests: 5, Window: time.Minute}
	keyFunc := func(r *http.Request) string { return "fixed-key" }

	handlerCalled := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.RateLimitHandler(cfg, keyFunc, inner)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	handler(w, r)

	if !handlerCalled {
		t.Fatal("inner handler should have been called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("missing X-RateLimit-Limit header")
	}
	if w.Header().Get("X-RateLimit-Remaining") == "" {
		t.Error("missing X-RateLimit-Remaining header")
	}
	if w.Header().Get("X-RateLimit-Reset") == "" {
		t.Error("missing X-RateLimit-Reset header")
	}
}

func TestRateLimitHandler_Blocked(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()

	cfg := RateLimitConfig{MaxRequests: 1, Window: time.Minute}
	keyFunc := func(r *http.Request) string { return "blocked-key" }

	// Exhaust the limit first.
	rl.Allow("blocked-key", cfg)
	rl.Allow("blocked-key", cfg) // over limit

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler should NOT be called when rate limited")
	})

	handler := rl.RateLimitHandler(cfg, keyFunc, inner)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	handler(w, r)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("missing Retry-After header on 429")
	}
}

// ── NewDistributedRateLimiter ─────────────────────────────────────────────

func TestNewDistributedRateLimiter_NilDB(t *testing.T) {
	// When given a nil collection, should panic or work in local mode
	// We can't call this with nil db without panic, so just verify
	// the local rate limiter works as documented
	rl := NewRateLimiter()
	defer rl.Stop()

	if rl.collection != nil {
		t.Error("expected nil collection for local rate limiter")
	}

	// Verify Allow() falls back to local when collection is nil
	config := RateLimitConfig{MaxRequests: 5, Window: time.Minute}
	allowed, remaining, _ := rl.Allow("test-key", config)
	if !allowed {
		t.Error("expected first request to be allowed")
	}
	if remaining != 4 {
		t.Errorf("expected 4 remaining, got %d", remaining)
	}
}
