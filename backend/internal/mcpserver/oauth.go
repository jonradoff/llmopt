package mcpserver

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"llmopt/internal/saas"
)

// OAuthServer implements a minimal OAuth 2.1 authorization server for MCP clients.
// It supports Dynamic Client Registration (RFC 7591), Authorization Code with PKCE,
// and refresh tokens. Refresh tokens are persisted to MongoDB when a DB is provided
// (so they survive server restarts); otherwise they fall back to in-memory storage.
type OAuthServer struct {
	mu            sync.RWMutex
	clients       map[string]*oauthClient
	authCodes     map[string]*authCode
	refreshTokens map[string]*refreshToken // fallback when refreshColl == nil
	refreshColl   *mongo.Collection
	clientsColl   *mongo.Collection // persists dynamic client registrations across restarts
	db            *mongo.Database   // for lok_ user access key validation
	jwtSecret     []byte
	baseURL       string
	sm            *saas.Middleware
}

// refreshTokenDoc is the MongoDB document for persisted refresh tokens.
type refreshTokenDoc struct {
	Token     string    `bson:"_id"`
	UserID    string    `bson:"user_id"`
	TenantID  string    `bson:"tenant_id"`
	Email     string    `bson:"email"`
	Role      string    `bson:"role"`
	ExpiresAt time.Time `bson:"expires_at"`
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
// When db is non-nil, refresh tokens are persisted to MongoDB (collection
// "mcp_refresh_tokens") so they survive server restarts.
func NewOAuthServer(sm *saas.Middleware, jwtSecret, baseURL string, db *mongo.Database) *OAuthServer {
	secret := sha256.Sum256([]byte(jwtSecret))
	s := &OAuthServer{
		clients:       make(map[string]*oauthClient),
		authCodes:     make(map[string]*authCode),
		refreshTokens: make(map[string]*refreshToken),
		jwtSecret:     secret[:],
		baseURL:       baseURL,
		sm:            sm,
	}
	if db != nil {
		s.db = db
		s.refreshColl = db.Collection("mcp_refresh_tokens")
		s.clientsColl = db.Collection("mcp_oauth_clients")
		go s.ensureTTLIndex()
		go s.loadClientsFromDB()
	}
	go s.cleanup()
	return s
}

// ensureTTLIndex creates a TTL index on expires_at so MongoDB auto-expires old tokens.
func (s *OAuthServer) ensureTTLIndex() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := s.refreshColl.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "expires_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(0),
	})
	if err != nil {
		log.Printf("MCP: failed to create TTL index on mcp_refresh_tokens: %v", err)
	}
}

// loadClientsFromDB pre-populates the in-memory client map from MongoDB on startup.
func (s *OAuthServer) loadClientsFromDB() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cursor, err := s.clientsColl.Find(ctx, bson.M{})
	if err != nil {
		log.Printf("MCP: failed to load clients from DB: %v", err)
		return
	}
	defer cursor.Close(ctx)
	var loaded int
	for cursor.Next(ctx) {
		var c oauthClient
		if err := cursor.Decode(&c); err == nil {
			s.mu.Lock()
			s.clients[c.ClientID] = &c
			s.mu.Unlock()
			loaded++
		}
	}
	log.Printf("MCP: loaded %d OAuth clients from DB", loaded)
}

// persistClient saves a newly registered client to MongoDB.
func (s *OAuthServer) persistClient(client *oauthClient) {
	if s.clientsColl == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Upsert so re-registration of the same clientID doesn't error.
	_, err := s.clientsColl.ReplaceOne(ctx,
		bson.M{"_id": client.ClientID},
		bson.M{
			"_id":          client.ClientID,
			"client_name":  client.ClientName,
			"redirect_uris": client.RedirectURIs,
			"created_at":   client.CreatedAt,
		},
		options.Replace().SetUpsert(true),
	)
	if err != nil {
		log.Printf("MCP: failed to persist OAuth client %s: %v", client.ClientID, err)
	}
}

// lookupClient returns a client by ID, checking memory first then MongoDB.
func (s *OAuthServer) lookupClient(clientID string) (*oauthClient, bool) {
	s.mu.RLock()
	c, ok := s.clients[clientID]
	s.mu.RUnlock()
	if ok {
		return c, true
	}
	// Not in memory — try DB (handles post-restart case).
	if s.clientsColl == nil {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var doc struct {
		ID           string   `bson:"_id"`
		ClientName   string   `bson:"client_name"`
		RedirectURIs []string `bson:"redirect_uris"`
		CreatedAt    time.Time `bson:"created_at"`
	}
	if err := s.clientsColl.FindOne(ctx, bson.M{"_id": clientID}).Decode(&doc); err != nil {
		return nil, false
	}
	c = &oauthClient{
		ClientID:     doc.ID,
		ClientName:   doc.ClientName,
		RedirectURIs: doc.RedirectURIs,
		CreatedAt:    doc.CreatedAt,
	}
	// Cache it back in memory.
	s.mu.Lock()
	s.clients[clientID] = c
	s.mu.Unlock()
	return c, true
}

// storeRefreshToken persists a refresh token to MongoDB or in-memory.
func (s *OAuthServer) storeRefreshToken(token string, info *saas.AuthInfo, expiresAt time.Time) {
	if s.refreshColl != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		doc := refreshTokenDoc{
			Token:     token,
			UserID:    info.UserID,
			TenantID:  info.TenantID,
			Email:     info.Email,
			Role:      info.Role,
			ExpiresAt: expiresAt,
		}
		if _, err := s.refreshColl.InsertOne(ctx, doc); err != nil {
			log.Printf("MCP: failed to store refresh token: %v", err)
		}
		return
	}
	// In-memory fallback (used when no DB, e.g. tests).
	s.mu.Lock()
	s.refreshTokens[token] = &refreshToken{
		Token:     token,
		AuthInfo:  info,
		ExpiresAt: expiresAt,
	}
	s.mu.Unlock()
}

// lookupRefreshToken retrieves a refresh token from MongoDB or in-memory.
func (s *OAuthServer) lookupRefreshToken(token string) (*refreshToken, bool) {
	if s.refreshColl != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var doc refreshTokenDoc
		if err := s.refreshColl.FindOne(ctx, bson.M{"_id": token}).Decode(&doc); err != nil {
			return nil, false
		}
		if time.Now().After(doc.ExpiresAt) {
			return nil, false
		}
		return &refreshToken{
			Token: doc.Token,
			AuthInfo: &saas.AuthInfo{
				UserID:   doc.UserID,
				TenantID: doc.TenantID,
				Email:    doc.Email,
				Role:     doc.Role,
				Method:   "mcp_refresh",
			},
			ExpiresAt: doc.ExpiresAt,
		}, true
	}
	// In-memory fallback.
	s.mu.RLock()
	rt, ok := s.refreshTokens[token]
	s.mu.RUnlock()
	return rt, ok
}

// cleanup periodically removes expired auth codes and refresh tokens.
func (s *OAuthServer) cleanup() {
	for {
		time.Sleep(5 * time.Minute)
		now := time.Now()
		s.mu.Lock()
		for k, v := range s.authCodes {
			if now.After(v.ExpiresAt) {
				delete(s.authCodes, k)
			}
		}
		// Clean in-memory refresh tokens when not using MongoDB.
		if s.refreshColl == nil {
			for k, v := range s.refreshTokens {
				if now.After(v.ExpiresAt) {
					delete(s.refreshTokens, k)
				}
			}
		}
		s.mu.Unlock()
		// MongoDB TTL index handles expiry automatically, but we also delete proactively.
		if s.refreshColl != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, _ = s.refreshColl.DeleteMany(ctx, bson.M{"expires_at": bson.M{"$lt": now}})
			cancel()
		}
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
	go s.persistClient(client)

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

	client, ok := s.lookupClient(clientID)
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

	if _, ok := s.lookupClient(clientID); !ok {
		http.Error(w, `{"error":"invalid_client"}`, http.StatusBadRequest)
		return
	}

	// Validate the API key — supports both lsk_ (LastSaaS) and lok_ (llmopt user access keys).
	var info *saas.AuthInfo
	var err error
	if strings.HasPrefix(apiKey, "lok_") {
		info, err = s.validateLokKey(r.Context(), apiKey)
	} else {
		info, err = s.sm.ValidateToken(r.Context(), apiKey, "")
	}
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
	s.storeRefreshToken(refreshTok, ac.AuthInfo, time.Now().Add(30*24*time.Hour))

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

	rt, ok := s.lookupRefreshToken(token)

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

// userAccessKeyDoc is the minimal shape we need from the user_access_keys collection.
type userAccessKeyDoc struct {
	UserID   string `bson:"userId"`
	TenantID string `bson:"tenantId"`
}

// validateLokKey validates a lok_ user access key against the user_access_keys collection
// and returns a fresh AuthInfo by resolving the user and tenant from the database.
func (s *OAuthServer) validateLokKey(ctx context.Context, rawKey string) (*saas.AuthInfo, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not available")
	}
	sum := sha256.Sum256([]byte(rawKey))
	hash := hex.EncodeToString(sum[:])

	coll := s.db.Collection("user_access_keys")
	var doc userAccessKeyDoc
	if err := coll.FindOne(ctx, bson.M{"keyHash": hash, "isActive": true}).Decode(&doc); err != nil {
		return nil, fmt.Errorf("invalid access key")
	}

	info, err := s.sm.ResolveAuthInfo(ctx, doc.UserID, doc.TenantID, "lok_key")
	if err != nil {
		return nil, err
	}

	// Update lastUsedAt asynchronously
	go func() {
		bg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = coll.UpdateOne(bg, bson.M{"keyHash": hash}, bson.M{"$set": bson.M{"lastUsedAt": time.Now()}})
	}()

	return info, nil
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
