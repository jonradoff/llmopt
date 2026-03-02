package mcpserver

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"llmopt/internal/saas"
)

// OAuthServer implements a minimal OAuth 2.1 authorization server for MCP clients.
// It supports Dynamic Client Registration (RFC 7591), Authorization Code with PKCE,
// and refresh tokens. State is stored in-memory (resets on restart).
type OAuthServer struct {
	mu            sync.RWMutex
	clients       map[string]*oauthClient
	authCodes     map[string]*authCode
	refreshTokens map[string]*refreshToken
	jwtSecret     []byte
	baseURL       string
	sm            *saas.Middleware
}

type oauthClient struct {
	ClientID     string   `json:"client_id"`
	ClientName   string   `json:"client_name"`
	RedirectURIs []string `json:"redirect_uris"`
	CreatedAt    time.Time
}

type authCode struct {
	Code          string
	ClientID      string
	RedirectURI   string
	CodeChallenge string
	State         string
	AuthInfo      *saas.AuthInfo
	ExpiresAt     time.Time
}

type refreshToken struct {
	Token     string
	AuthInfo  *saas.AuthInfo
	APIKey    string // raw lsk_ key for re-validation
	ExpiresAt time.Time
}

// NewOAuthServer creates an OAuth server for MCP authentication.
func NewOAuthServer(sm *saas.Middleware, jwtSecret, baseURL string) *OAuthServer {
	secret := sha256.Sum256([]byte(jwtSecret))
	s := &OAuthServer{
		clients:       make(map[string]*oauthClient),
		authCodes:     make(map[string]*authCode),
		refreshTokens: make(map[string]*refreshToken),
		jwtSecret:     secret[:],
		baseURL:       baseURL,
		sm:            sm,
	}
	go s.cleanup()
	return s
}

// cleanup periodically removes expired auth codes and refresh tokens.
func (s *OAuthServer) cleanup() {
	for {
		time.Sleep(5 * time.Minute)
		s.mu.Lock()
		now := time.Now()
		for k, v := range s.authCodes {
			if now.After(v.ExpiresAt) {
				delete(s.authCodes, k)
			}
		}
		for k, v := range s.refreshTokens {
			if now.After(v.ExpiresAt) {
				delete(s.refreshTokens, k)
			}
		}
		s.mu.Unlock()
	}
}

// HandleProtectedResource serves the OAuth Protected Resource Metadata (RFC 9728).
func (s *OAuthServer) HandleProtectedResource(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"resource":                s.baseURL,
		"authorization_servers":   []string{s.baseURL},
		"bearer_methods_supported": []string{"header"},
		"scopes_supported":        []string{"mcp:read", "mcp:write"},
	})
}

// HandleServerMetadata serves the OAuth Authorization Server Metadata (RFC 8414).
func (s *OAuthServer) HandleServerMetadata(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"issuer":                                s.baseURL,
		"authorization_endpoint":                s.baseURL + "/oauth/authorize",
		"token_endpoint":                        s.baseURL + "/oauth/token",
		"registration_endpoint":                 s.baseURL + "/oauth/register",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":       []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none"},
		"scopes_supported":                      []string{"mcp:read", "mcp:write"},
	})
}

// HandleRegister implements Dynamic Client Registration (RFC 7591).
func (s *OAuthServer) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientName   string   `json:"client_name"`
		RedirectURIs []string `json:"redirect_uris"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}

	if req.ClientName == "" {
		req.ClientName = "MCP Client"
	}
	if len(req.RedirectURIs) == 0 {
		http.Error(w, `{"error":"invalid_request","error_description":"redirect_uris required"}`, http.StatusBadRequest)
		return
	}

	clientID := randomString(32)
	client := &oauthClient{
		ClientID:     clientID,
		ClientName:   req.ClientName,
		RedirectURIs: req.RedirectURIs,
		CreatedAt:    time.Now(),
	}

	s.mu.Lock()
	s.clients[clientID] = client
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"client_id":                clientID,
		"client_name":              req.ClientName,
		"redirect_uris":            req.RedirectURIs,
		"grant_types":              []string{"authorization_code", "refresh_token"},
		"response_types":           []string{"code"},
		"token_endpoint_auth_method": "none",
	})
}

// HandleAuthorize serves the authorization page where users enter their API key.
func (s *OAuthServer) HandleAuthorize(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	redirectURI := r.URL.Query().Get("redirect_uri")
	state := r.URL.Query().Get("state")
	codeChallenge := r.URL.Query().Get("code_challenge")
	codeChallengeMethod := r.URL.Query().Get("code_challenge_method")

	if clientID == "" || redirectURI == "" || codeChallenge == "" {
		http.Error(w, `{"error":"invalid_request","error_description":"client_id, redirect_uri, and code_challenge required"}`, http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	client, ok := s.clients[clientID]
	s.mu.RUnlock()
	if !ok {
		http.Error(w, `{"error":"invalid_client"}`, http.StatusBadRequest)
		return
	}

	// Validate redirect_uri
	validRedirect := false
	for _, uri := range client.RedirectURIs {
		if uri == redirectURI {
			validRedirect = true
			break
		}
	}
	if !validRedirect {
		http.Error(w, `{"error":"invalid_request","error_description":"redirect_uri not registered"}`, http.StatusBadRequest)
		return
	}

	if codeChallengeMethod != "" && codeChallengeMethod != "S256" {
		http.Error(w, `{"error":"invalid_request","error_description":"only S256 code_challenge_method supported"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, authorizePage,
		clientID, redirectURI, state, codeChallenge,
		s.baseURL, s.baseURL,
	)
}

// HandleAuthorizeSubmit processes the authorization form submission.
func (s *OAuthServer) HandleAuthorizeSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}

	clientID := r.FormValue("client_id")
	redirectURI := r.FormValue("redirect_uri")
	state := r.FormValue("state")
	codeChallenge := r.FormValue("code_challenge")
	apiKey := r.FormValue("api_key")

	if clientID == "" || redirectURI == "" || apiKey == "" {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	_, ok := s.clients[clientID]
	s.mu.RUnlock()
	if !ok {
		http.Error(w, `{"error":"invalid_client"}`, http.StatusBadRequest)
		return
	}

	// Validate the API key
	info, err := s.sm.ValidateToken(r.Context(), apiKey, "")
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, errorPage, "Invalid API key. Please check your key and try again.")
		return
	}

	// Generate auth code
	code := randomString(48)
	s.mu.Lock()
	s.authCodes[code] = &authCode{
		Code:          code,
		ClientID:      clientID,
		RedirectURI:   redirectURI,
		CodeChallenge: codeChallenge,
		State:         state,
		AuthInfo:      info,
		ExpiresAt:     time.Now().Add(10 * time.Minute),
	}
	s.mu.Unlock()

	// Redirect back to client
	u, _ := url.Parse(redirectURI)
	q := u.Query()
	q.Set("code", code)
	if state != "" {
		q.Set("state", state)
	}
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

// HandleToken exchanges authorization codes for access tokens.
func (s *OAuthServer) HandleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}

	grantType := r.FormValue("grant_type")

	switch grantType {
	case "authorization_code":
		s.handleAuthCodeExchange(w, r)
	case "refresh_token":
		s.handleRefreshTokenExchange(w, r)
	default:
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"unsupported_grant_type"}`, http.StatusBadRequest)
	}
}

func (s *OAuthServer) handleAuthCodeExchange(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	codeVerifier := r.FormValue("code_verifier")
	clientID := r.FormValue("client_id")

	if code == "" || codeVerifier == "" {
		http.Error(w, `{"error":"invalid_request","error_description":"code and code_verifier required"}`, http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	ac, ok := s.authCodes[code]
	if ok {
		delete(s.authCodes, code) // one-time use
	}
	s.mu.Unlock()

	if !ok || time.Now().After(ac.ExpiresAt) {
		http.Error(w, `{"error":"invalid_grant","error_description":"code expired or invalid"}`, http.StatusBadRequest)
		return
	}

	if clientID != "" && clientID != ac.ClientID {
		http.Error(w, `{"error":"invalid_grant","error_description":"client_id mismatch"}`, http.StatusBadRequest)
		return
	}

	// Validate PKCE S256
	verifierHash := sha256.Sum256([]byte(codeVerifier))
	expectedChallenge := base64.RawURLEncoding.EncodeToString(verifierHash[:])
	if expectedChallenge != ac.CodeChallenge {
		http.Error(w, `{"error":"invalid_grant","error_description":"code_verifier mismatch"}`, http.StatusBadRequest)
		return
	}

	// Issue access token (JWT) and refresh token
	accessToken := s.signJWT(ac.AuthInfo, time.Hour)
	refreshTok := randomString(64)

	s.mu.Lock()
	s.refreshTokens[refreshTok] = &refreshToken{
		Token:     refreshTok,
		AuthInfo:  ac.AuthInfo,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string]any{
		"access_token":  accessToken,
		"token_type":    "Bearer",
		"expires_in":    3600,
		"refresh_token": refreshTok,
		"scope":         "mcp:read mcp:write",
	})
}

func (s *OAuthServer) handleRefreshTokenExchange(w http.ResponseWriter, r *http.Request) {
	token := r.FormValue("refresh_token")
	if token == "" {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	rt, ok := s.refreshTokens[token]
	s.mu.RUnlock()

	if !ok || time.Now().After(rt.ExpiresAt) {
		http.Error(w, `{"error":"invalid_grant","error_description":"refresh token expired or invalid"}`, http.StatusBadRequest)
		return
	}

	accessToken := s.signJWT(rt.AuthInfo, time.Hour)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string]any{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   3600,
		"scope":        "mcp:read mcp:write",
	})
}

// signJWT creates an HMAC-SHA256 signed JWT with the auth info.
func (s *OAuthServer) signJWT(info *saas.AuthInfo, ttl time.Duration) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

	claims := map[string]any{
		"sub":   info.UserID,
		"tid":   info.TenantID,
		"email": info.Email,
		"role":  info.Role,
		"iss":   "llmopt-mcp",
		"exp":   time.Now().Add(ttl).Unix(),
		"iat":   time.Now().Unix(),
	}
	claimsJSON, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(claimsJSON)

	sigInput := header + "." + payload
	mac := hmac.New(sha256.New, s.jwtSecret)
	mac.Write([]byte(sigInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return sigInput + "." + sig
}

// ValidateAccessToken validates an MCP-issued JWT access token.
func (s *OAuthServer) ValidateAccessToken(tokenStr string) (*saas.AuthInfo, error) {
	parts := strings.SplitN(tokenStr, ".", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	// Verify signature
	sigInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, s.jwtSecret)
	mac.Write([]byte(sigInput))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return nil, fmt.Errorf("invalid signature")
	}

	// Decode claims
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid payload")
	}

	var claims struct {
		Sub   string `json:"sub"`
		TID   string `json:"tid"`
		Email string `json:"email"`
		Role  string `json:"role"`
		Iss   string `json:"iss"`
		Exp   int64  `json:"exp"`
	}
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("invalid claims")
	}

	if claims.Iss != "llmopt-mcp" {
		return nil, fmt.Errorf("wrong issuer")
	}
	if time.Now().Unix() > claims.Exp {
		return nil, fmt.Errorf("token expired")
	}

	return &saas.AuthInfo{
		UserID:   claims.Sub,
		Email:    claims.Email,
		TenantID: claims.TID,
		Role:     claims.Role,
		Method:   "mcp_token",
	}, nil
}

// randomString generates a cryptographically random base62 string of the given length.
func randomString(length int) string {
	const charset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

// authorizePage is the HTML template for the OAuth authorization form.
var authorizePage = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Connect to LLM Optimizer</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#0a0a0f;color:#e0e0e0;display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{background:#141420;border:1px solid #2a2a3a;border-radius:12px;padding:40px;max-width:420px;width:100%%}
h1{font-size:1.4rem;margin-bottom:8px;color:#fff}
p{color:#888;margin-bottom:24px;font-size:0.9rem;line-height:1.5}
label{display:block;font-size:0.85rem;color:#aaa;margin-bottom:6px}
input{width:100%%;padding:10px 14px;background:#1a1a2e;border:1px solid #333;border-radius:8px;color:#fff;font-size:0.95rem;margin-bottom:20px;outline:none}
input:focus{border-color:#6366f1}
button{width:100%%;padding:12px;background:#6366f1;color:#fff;border:none;border-radius:8px;font-size:1rem;cursor:pointer;font-weight:500}
button:hover{background:#5558e6}
.hint{font-size:0.8rem;color:#666;margin-top:16px;text-align:center}
</style>
</head>
<body>
<div class="card">
<h1>Connect to LLM Optimizer</h1>
<p>An MCP client is requesting access to your LLM Optimizer data. Enter your API key to authorize.</p>
<form method="POST" action="/oauth/authorize">
<input type="hidden" name="client_id" value="%s">
<input type="hidden" name="redirect_uri" value="%s">
<input type="hidden" name="state" value="%s">
<input type="hidden" name="code_challenge" value="%s">
<label for="api_key">API Key</label>
<input type="password" id="api_key" name="api_key" placeholder="lsk_..." required autocomplete="off">
<button type="submit">Authorize</button>
</form>
<p class="hint">Create API keys at <a href="%s/last/api" style="color:#6366f1">%s/last/api</a></p>
</div>
</body>
</html>`

// errorPage shown when API key validation fails during OAuth authorization.
var errorPage = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Authorization Error</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#0a0a0f;color:#e0e0e0;display:flex;justify-content:center;align-items:center;min-height:100vh}
.card{background:#141420;border:1px solid #2a2a3a;border-radius:12px;padding:40px;max-width:420px;width:100%%;text-align:center}
h1{font-size:1.3rem;margin-bottom:12px;color:#ef4444}
p{color:#888;margin-bottom:20px;font-size:0.9rem}
a{color:#6366f1;text-decoration:none}
a:hover{text-decoration:underline}
</style>
</head>
<body>
<div class="card">
<h1>Authorization Failed</h1>
<p>%s</p>
<a href="javascript:history.back()">Go back and try again</a>
</div>
</body>
</html>`
