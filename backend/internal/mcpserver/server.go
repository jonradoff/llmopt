package mcpserver

import (
	"net/http"
	"strings"

	"github.com/mark3labs/mcp-go/server"
	"go.mongodb.org/mongo-driver/mongo"

	"llmopt/internal/saas"
)

// New creates the MCP server and returns it as an http.Handler.
// The handler wraps the streamable HTTP server with bearer token auth.
func New(sm *saas.Middleware, db *mongo.Database, oauth *OAuthServer, baseURL string) http.Handler {
	s := server.NewMCPServer(
		"LLM Optimizer",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	h := &handlers{sm: sm, db: db}

	s.AddTool(listDomainsTool(), h.listDomains)
	s.AddTool(getReportTool(), h.getReport)
	s.AddTool(getVisibilityScoreTool(), h.getVisibilityScore)
	s.AddTool(listTodosTool(), h.listTodos)
	s.AddTool(updateTodoTool(), h.updateTodo)

	streamable := server.NewStreamableHTTPServer(s,
		server.WithStateLess(true),
	)

	return authMiddleware(sm, oauth, baseURL, streamable)
}

// authMiddleware wraps the MCP handler with bearer token validation.
// Returns 401 with WWW-Authenticate header when no valid auth is provided.
func authMiddleware(sm *saas.Middleware, oauth *OAuthServer, baseURL string, next http.Handler) http.Handler {
	// In non-SaaS mode, no auth required
	if sm == nil {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow DELETE and GET without auth (MCP session management / SSE)
		if r.Method != "POST" {
			next.ServeHTTP(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+baseURL+`/.well-known/oauth-protected-resource"`)
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		tenantIDHint := r.Header.Get("X-Tenant-ID")

		var info *saas.AuthInfo
		var err error

		if strings.HasPrefix(token, "lsk_") {
			// Direct LastSaaS API key
			info, err = sm.ValidateToken(r.Context(), token, tenantIDHint)
		} else if strings.HasPrefix(token, "lok_") && oauth != nil {
			// Direct user access key (lok_ prefix) — no OAuth flow required
			info, err = oauth.validateLokKey(r.Context(), token)
		} else if oauth != nil {
			// Try as MCP-issued JWT first
			info, err = oauth.ValidateAccessToken(token)
			if err != nil {
				// Fall back to LastSaaS JWT
				info, err = sm.ValidateToken(r.Context(), token, tenantIDHint)
			}
		} else {
			info, err = sm.ValidateToken(r.Context(), token, tenantIDHint)
		}

		if err != nil {
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+baseURL+`/.well-known/oauth-protected-resource"`)
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		ctx := saas.SetAuthContext(r.Context(), info)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

