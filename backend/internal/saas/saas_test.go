package saas

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret-key-for-unit-tests-1234"

// ── JWTValidator ────────────────────────────────────────────────────────

func makeTestToken(t *testing.T, secret string, claims Claims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign test token: %v", err)
	}
	return signed
}

func TestJWTValidator_ValidToken(t *testing.T) {
	v := NewJWTValidator(testSecret)
	expiry := time.Now().Add(time.Hour)
	tokenStr := makeTestToken(t, testSecret, Claims{
		UserID: "user-123",
		Email:  "test@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiry),
		},
	})

	claims, err := v.Validate(tokenStr)
	if err != nil {
		t.Fatalf("expected valid token, got error: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("UserID: got %q, want %q", claims.UserID, "user-123")
	}
	if claims.Email != "test@example.com" {
		t.Errorf("Email: got %q, want %q", claims.Email, "test@example.com")
	}
}

func TestJWTValidator_BearerPrefix(t *testing.T) {
	v := NewJWTValidator(testSecret)
	expiry := time.Now().Add(time.Hour)
	raw := makeTestToken(t, testSecret, Claims{
		UserID: "user-456",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiry),
		},
	})

	claims, err := v.Validate("Bearer " + raw)
	if err != nil {
		t.Fatalf("expected valid token with Bearer prefix, got error: %v", err)
	}
	if claims.UserID != "user-456" {
		t.Errorf("UserID: got %q", claims.UserID)
	}
}

func TestJWTValidator_EmptyToken(t *testing.T) {
	v := NewJWTValidator(testSecret)
	_, err := v.Validate("")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestJWTValidator_BearerOnly(t *testing.T) {
	v := NewJWTValidator(testSecret)
	_, err := v.Validate("Bearer ")
	if err == nil {
		t.Fatal("expected error for Bearer-only token")
	}
}

func TestJWTValidator_WrongSecret(t *testing.T) {
	v := NewJWTValidator(testSecret)
	expiry := time.Now().Add(time.Hour)
	tokenStr := makeTestToken(t, "wrong-secret", Claims{
		UserID: "user-789",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiry),
		},
	})

	_, err := v.Validate(tokenStr)
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestJWTValidator_ExpiredToken(t *testing.T) {
	v := NewJWTValidator(testSecret)
	pastExpiry := time.Now().Add(-time.Hour)
	tokenStr := makeTestToken(t, testSecret, Claims{
		UserID: "user-old",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(pastExpiry),
		},
	})

	_, err := v.Validate(tokenStr)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestJWTValidator_MalformedToken(t *testing.T) {
	v := NewJWTValidator(testSecret)
	_, err := v.Validate("this.is.notvalid")
	if err == nil {
		t.Fatal("expected error for malformed token")
	}
}

func TestJWTValidator_WrongAlgorithm(t *testing.T) {
	// Manually craft an RS256 token header to trigger the signing method check.
	// The validator should reject non-HMAC methods.
	v := NewJWTValidator(testSecret)
	// Use an HMAC token but tamper the header — simplest is to just use a
	// token string with a different header.
	_, err := v.Validate("eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJ1c2VySWQiOiJ4In0.")
	if err == nil {
		t.Fatal("expected error for unsupported signing method")
	}
}

// ── SetAuthContext + context helpers ────────────────────────────────────

func TestSetAuthContext_AllFields(t *testing.T) {
	tenant := &Tenant{Name: "Test Tenant", IsActive: true}
	plan := &Plan{Name: "Pro"}
	info := &AuthInfo{
		UserID:   "u-1",
		Email:    "user@example.com",
		TenantID: "t-1",
		Tenant:   tenant,
		Plan:     plan,
		Role:     "owner",
		Method:   "jwt",
	}

	ctx := SetAuthContext(context.Background(), info)

	if got := UserIDFromContext(ctx); got != "u-1" {
		t.Errorf("UserIDFromContext: got %q, want %q", got, "u-1")
	}
	if got := TenantIDFromContext(ctx); got != "t-1" {
		t.Errorf("TenantIDFromContext: got %q, want %q", got, "t-1")
	}
	if got := MemberRoleFromContext(ctx); got != "owner" {
		t.Errorf("MemberRoleFromContext: got %q, want %q", got, "owner")
	}
	if got := AuthMethodFromContext(ctx); got != "jwt" {
		t.Errorf("AuthMethodFromContext: got %q, want %q", got, "jwt")
	}
	if got := TenantFromContext(ctx); got != tenant {
		t.Errorf("TenantFromContext: got %v, want %v", got, tenant)
	}
	if got := PlanFromContext(ctx); got != plan {
		t.Errorf("PlanFromContext: got %v, want %v", got, plan)
	}
}

func TestSetAuthContext_NilTenantAndPlan(t *testing.T) {
	info := &AuthInfo{
		UserID:   "u-2",
		Email:    "b@example.com",
		TenantID: "t-2",
		Tenant:   nil,
		Plan:     nil,
		Role:     "member",
		Method:   "apikey",
	}

	ctx := SetAuthContext(context.Background(), info)

	if got := TenantFromContext(ctx); got != nil {
		t.Errorf("expected nil tenant, got %v", got)
	}
	if got := PlanFromContext(ctx); got != nil {
		t.Errorf("expected nil plan, got %v", got)
	}
}

func TestContextHelpers_EmptyContext(t *testing.T) {
	ctx := context.Background()
	if UserIDFromContext(ctx) != "" {
		t.Error("expected empty UserID from empty context")
	}
	if TenantIDFromContext(ctx) != "" {
		t.Error("expected empty TenantID from empty context")
	}
	if MemberRoleFromContext(ctx) != "" {
		t.Error("expected empty role from empty context")
	}
	if AuthMethodFromContext(ctx) != "" {
		t.Error("expected empty auth method from empty context")
	}
	if TenantFromContext(ctx) != nil {
		t.Error("expected nil tenant from empty context")
	}
	if PlanFromContext(ctx) != nil {
		t.Error("expected nil plan from empty context")
	}
}

// ── RequireJWT middleware ────────────────────────────────────────────────

// stubMiddleware creates a Middleware with a real JWTValidator but no DB.
// Only RequireJWT is usable (no DB calls).
func stubJWTMiddleware(secret string) *Middleware {
	return &Middleware{
		jwt: NewJWTValidator(secret),
	}
}

func TestRequireJWT_Valid(t *testing.T) {
	m := stubJWTMiddleware(testSecret)
	expiry := time.Now().Add(time.Hour)
	tokenStr := makeTestToken(t, testSecret, Claims{
		UserID: "uid-1",
		Email:  "jwt@test.com",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiry),
		},
	})

	var gotUID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUID = UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := m.RequireJWT(next)
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	if gotUID != "uid-1" {
		t.Errorf("UserID in context: got %q, want %q", gotUID, "uid-1")
	}
}

func TestRequireJWT_MissingHeader(t *testing.T) {
	m := stubJWTMiddleware(testSecret)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next should not be called")
	})

	handler := m.RequireJWT(next)
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestRequireJWT_InvalidToken(t *testing.T) {
	m := stubJWTMiddleware(testSecret)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next should not be called")
	})

	handler := m.RequireJWT(next)
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// ── RequireActiveBilling middleware ──────────────────────────────────────

func tenantCtx(ctx context.Context, tenant *Tenant) context.Context {
	return context.WithValue(ctx, ctxTenant, tenant)
}

func TestRequireActiveBilling_NoTenant(t *testing.T) {
	handler := RequireActiveBilling()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach handler")
	}))

	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestRequireActiveBilling_RootTenant(t *testing.T) {
	called := false
	handler := RequireActiveBilling()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest("GET", "/", nil)
	r = r.WithContext(tenantCtx(r.Context(), &Tenant{IsRoot: true, BillingStatus: BillingStatusCanceled}))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if !called {
		t.Error("root tenant should bypass billing check")
	}
}

func TestRequireActiveBilling_WaivedTenant(t *testing.T) {
	called := false
	handler := RequireActiveBilling()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest("GET", "/", nil)
	r = r.WithContext(tenantCtx(r.Context(), &Tenant{BillingWaived: true, BillingStatus: BillingStatusPastDue}))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if !called {
		t.Error("waived tenant should bypass billing check")
	}
}

func TestRequireActiveBilling_ActiveSubscription(t *testing.T) {
	called := false
	handler := RequireActiveBilling()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest("GET", "/", nil)
	r = r.WithContext(tenantCtx(r.Context(), &Tenant{BillingStatus: BillingStatusActive}))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if !called {
		t.Error("active billing should allow through")
	}
}

func TestRequireActiveBilling_FreeTier(t *testing.T) {
	called := false
	handler := RequireActiveBilling()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest("GET", "/", nil)
	r = r.WithContext(tenantCtx(r.Context(), &Tenant{BillingStatus: BillingStatusNone}))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if !called {
		t.Error("free-tier tenant (BillingStatusNone) should be allowed through")
	}
}

func TestRequireActiveBilling_InactiveBilling(t *testing.T) {
	statuses := []BillingStatus{BillingStatusPastDue, BillingStatusCanceled}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			handler := RequireActiveBilling()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("should not be called for inactive billing")
			}))

			r := httptest.NewRequest("GET", "/", nil)
			r = r.WithContext(tenantCtx(r.Context(), &Tenant{BillingStatus: status}))
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)

			if w.Code != http.StatusPaymentRequired {
				t.Errorf("expected 402 for %s, got %d", status, w.Code)
			}
			if !strings.Contains(w.Body.String(), "BILLING_INACTIVE") {
				t.Errorf("expected BILLING_INACTIVE in body, got: %s", w.Body.String())
			}
		})
	}
}
