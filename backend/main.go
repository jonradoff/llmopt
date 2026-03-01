package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"llmopt/internal/saas"
)

// normalizeDomain strips protocol, trailing slashes, and lowercases.
// "https://Anthropic.AI/" → "anthropic.ai"
func normalizeDomain(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimRight(s, "/")
	return s
}

func loadEnvFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		// Strip surrounding quotes
		if len(value) >= 2 &&
			((value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		// Don't overwrite explicitly set env vars
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}

func main() {
	// Load .env from project root (one dir up from backend/) then current dir as fallback
	loadEnvFile("../.env")
	loadEnvFile(".env")

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY environment variable is required. Copy .env.dev.example to .env and fill in your key.")
	}

	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		log.Fatal("MONGODB_URI environment variable is required.")
	}

	ytKey := os.Getenv("YOUTUBE_API_KEY") // optional — video tab disabled if missing

	// Encryption key for API key storage (required in SaaS mode)
	var encryptionKey []byte
	if encKeyHex := os.Getenv("LLMOPT_ENCRYPTION_KEY"); encKeyHex != "" {
		var err error
		encryptionKey, err = parseEncryptionKey(encKeyHex)
		if err != nil {
			log.Fatalf("Invalid LLMOPT_ENCRYPTION_KEY: %v", err)
		}
		log.Println("API key encryption enabled")
	}

	dbName := os.Getenv("DATABASE_NAME")
	if dbName == "" {
		dbName = "llmopt"
	}

	mongoDB, err := NewMongoDB(mongoURI, dbName)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer mongoDB.Close(context.Background())
	log.Printf("Connected to MongoDB (database: %s)", dbName)

	// One-time migration: normalize domain fields (strip protocol)
	mongoDB.migrateDomains()
	if ytKey != "" {
		log.Println("YouTube API key configured — Video Authority enabled")
	}

	// SaaS mode: multi-tenant auth via shared JWT with LastSaaS
	saasEnabled := os.Getenv("LLMOPT_SAAS_ENABLED") == "true"
	var sm *saas.Middleware
	if saasEnabled {
		jwtSecret := os.Getenv("LLMOPT_JWT_ACCESS_SECRET")
		if jwtSecret == "" {
			log.Fatal("LLMOPT_JWT_ACCESS_SECRET is required when LLMOPT_SAAS_ENABLED=true")
		}
		if encryptionKey == nil {
			log.Fatal("LLMOPT_ENCRYPTION_KEY is required when LLMOPT_SAAS_ENABLED=true (for API key encryption)")
		}
		jv := saas.NewJWTValidator(jwtSecret)
		sm = saas.NewMiddleware(jv, mongoDB.Database)
		log.Println("SaaS mode enabled — JWT auth active")

		// One-time migration: assign root tenant to existing data
		mongoDB.migrateTenantIDs()

		// One-time migration: convert per-record public flags to domain shares
		mongoDB.migratePublicToDomainShares()
	}

	// Capture missing screenshots for popular brands (non-blocking)
	go ensurePopularScreenshots(mongoDB)

	// withAuth wraps a handler with JWT + tenant middleware (SaaS mode only).
	// In non-SaaS mode, returns the handler unwrapped.
	withAuth := func(h http.HandlerFunc) http.HandlerFunc {
		if sm == nil {
			return h
		}
		return func(w http.ResponseWriter, r *http.Request) {
			sm.RequireJWT(sm.RequireTenant(http.HandlerFunc(h))).ServeHTTP(w, r)
		}
	}

	mux := http.NewServeMux()

	// SaaS config endpoint — tells the frontend whether auth is required
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"saas_enabled": saasEnabled})
	})
	mux.HandleFunc("OPTIONS /api/config", handleOptions)

	// Tenant-scoped routes (wrapped with auth in SaaS mode)
	mux.HandleFunc("POST /api/analyze", withAuth(handleAnalyze(mongoDB, encryptionKey, apiKey, saasEnabled)))
	mux.HandleFunc("GET /api/analyses", withAuth(handleListAnalyses(mongoDB)))
	mux.HandleFunc("GET /api/analyses/{id}", withAuth(handleGetAnalysis(mongoDB)))
	mux.HandleFunc("DELETE /api/analyses/{id}", withAuth(handleDeleteAnalysis(mongoDB)))
	mux.HandleFunc("DELETE /api/optimizations/{id}", withAuth(handleDeleteOptimization(mongoDB)))
	mux.HandleFunc("POST /api/analyses/{id}/questions/{idx}/optimize", withAuth(handleOptimize(mongoDB, encryptionKey, apiKey, saasEnabled)))
	mux.HandleFunc("GET /api/analyses/{id}/questions/{idx}/optimization", withAuth(handleGetOptimization(mongoDB)))
	mux.HandleFunc("GET /api/optimizations", withAuth(handleListOptimizations(mongoDB)))
	mux.HandleFunc("GET /api/optimizations/{id}", withAuth(handleGetOptimizationByID(mongoDB)))
	mux.HandleFunc("GET /api/domains/{domain}/share", withAuth(handleGetDomainShare(mongoDB)))
	mux.HandleFunc("PUT /api/domains/{domain}/share", withAuth(handleSetDomainShare(mongoDB)))
	mux.HandleFunc("GET /api/todos", withAuth(handleListTodos(mongoDB)))
	mux.HandleFunc("PATCH /api/todos/{id}", withAuth(handleUpdateTodo(mongoDB)))
	mux.HandleFunc("POST /api/todos/archive", withAuth(handleBulkArchiveTodos(mongoDB)))
	mux.HandleFunc("POST /api/domains/{domain}/summary", withAuth(handleGenerateDomainSummary(mongoDB, encryptionKey, apiKey, saasEnabled)))
	mux.HandleFunc("GET /api/domains/{domain}/summary", withAuth(handleGetDomainSummary(mongoDB)))
	mux.HandleFunc("GET /api/domains/{domain}/summary/status", withAuth(handleDomainSummaryStatus(mongoDB)))
	mux.HandleFunc("GET /api/brands", withAuth(handleListBrands(mongoDB)))
	mux.HandleFunc("GET /api/brands/{domain}", withAuth(handleGetBrand(mongoDB)))
	mux.HandleFunc("PUT /api/brands/{domain}", withAuth(handleSaveBrand(mongoDB)))
	mux.HandleFunc("DELETE /api/brands/{domain}", withAuth(handleDeleteBrand(mongoDB)))
	mux.HandleFunc("POST /api/brands/{domain}/discover-competitors", withAuth(handleDiscoverCompetitors(mongoDB, encryptionKey, apiKey, saasEnabled)))
	mux.HandleFunc("POST /api/brands/{domain}/suggest-queries", withAuth(handleSuggestQueries(mongoDB, encryptionKey, apiKey, saasEnabled)))
	mux.HandleFunc("POST /api/brands/{domain}/generate-description", withAuth(handleGenerateDescription(mongoDB, encryptionKey, apiKey, saasEnabled)))
	mux.HandleFunc("POST /api/brands/{domain}/predict-audience", withAuth(handlePredictAudience(mongoDB, encryptionKey, apiKey, saasEnabled)))
	mux.HandleFunc("POST /api/brands/{domain}/suggest-claims", withAuth(handleSuggestClaims(mongoDB, encryptionKey, apiKey, saasEnabled)))
	mux.HandleFunc("POST /api/brands/{domain}/predict-differentiators", withAuth(handlePredictDifferentiators(mongoDB, encryptionKey, apiKey, saasEnabled)))
	mux.HandleFunc("POST /api/video/discover", withAuth(handleVideoDiscover(ytKey, mongoDB)))
	mux.HandleFunc("POST /api/video/analyze", withAuth(handleVideoAnalyze(mongoDB, encryptionKey, apiKey, saasEnabled, ytKey)))
	mux.HandleFunc("GET /api/video/analyses/{domain}/details", withAuth(handleGetVideoDetails(mongoDB)))
	mux.HandleFunc("GET /api/video/analyses/{domain}", withAuth(handleGetVideoAnalysis(mongoDB)))
	mux.HandleFunc("GET /api/video/analyses", withAuth(handleListVideoAnalyses(mongoDB)))
	mux.HandleFunc("DELETE /api/video/analyses/{domain}", withAuth(handleDeleteVideoAnalysis(mongoDB)))

	// Reddit Authority Analyzer
	mux.HandleFunc("POST /api/reddit/discover", withAuth(handleRedditDiscover(mongoDB)))
	mux.HandleFunc("POST /api/reddit/analyze", withAuth(handleRedditAnalyze(mongoDB, encryptionKey, apiKey, saasEnabled)))
	mux.HandleFunc("GET /api/reddit/analyses/{domain}", withAuth(handleGetRedditAnalysis(mongoDB)))
	mux.HandleFunc("GET /api/reddit/analyses", withAuth(handleListRedditAnalyses(mongoDB)))
	mux.HandleFunc("DELETE /api/reddit/analyses/{domain}", withAuth(handleDeleteRedditAnalysis(mongoDB)))

	// Search Visibility Analyzer
	mux.HandleFunc("POST /api/search/analyze", withAuth(handleSearchAnalyze(mongoDB, encryptionKey, apiKey, saasEnabled)))
	mux.HandleFunc("GET /api/search/analyses/{domain}", withAuth(handleGetSearchAnalysis(mongoDB)))
	mux.HandleFunc("GET /api/search/analyses", withAuth(handleListSearchAnalyses(mongoDB)))
	mux.HandleFunc("DELETE /api/search/analyses/{domain}", withAuth(handleDeleteSearchAnalysis(mongoDB)))

	// Visibility Score
	mux.HandleFunc("GET /api/visibility-score/{domain}", withAuth(handleVisibilityScore(mongoDB)))

	// LLM Test
	mux.HandleFunc("GET /api/providers/models", withAuth(handleListProviderModels()))
	mux.HandleFunc("POST /api/test", withAuth(handleLLMTest(mongoDB, encryptionKey, apiKey, saasEnabled)))
	mux.HandleFunc("GET /api/test/{domain}/history", withAuth(handleGetLLMTestHistory(mongoDB)))
	mux.HandleFunc("GET /api/test/{domain}/competitors", withAuth(handleGetCompetitorTests(mongoDB)))
	mux.HandleFunc("GET /api/test/{domain}", withAuth(handleGetLLMTest(mongoDB)))
	mux.HandleFunc("DELETE /api/test/{domain}", withAuth(handleDeleteLLMTest(mongoDB)))
	mux.HandleFunc("POST /api/test/generate-queries", withAuth(handleGenerateTestQueries(mongoDB)))

	// PDF Report
	mux.HandleFunc("POST /api/domains/{domain}/report/pdf", withAuth(handleGeneratePDF(mongoDB)))
	mux.HandleFunc("GET /api/domains/{domain}/report/pdf/{id}", withAuth(handleServePDF(mongoDB)))

	// API Key Management
	mux.HandleFunc("GET /api/settings/api-keys", withAuth(handleListAPIKeys(mongoDB)))
	mux.HandleFunc("PUT /api/settings/api-keys/{provider}", withAuth(handleSetAPIKey(mongoDB, encryptionKey)))
	mux.HandleFunc("DELETE /api/settings/api-keys/{provider}", withAuth(handleDeleteAPIKey(mongoDB)))
	mux.HandleFunc("POST /api/settings/api-keys/{provider}/verify", withAuth(handleVerifyAPIKey(mongoDB, encryptionKey)))
	mux.HandleFunc("GET /api/settings/api-keys/status", withAuth(handleAPIKeyStatus(mongoDB)))
	mux.HandleFunc("GET /api/settings/primary-provider", withAuth(handleGetPrimaryProvider(mongoDB)))
	mux.HandleFunc("PUT /api/settings/primary-provider", withAuth(handleSetPrimaryProvider(mongoDB)))

	// Public routes (no auth required)
	mux.HandleFunc("GET /api/health/claude", handleHealthCheck(apiKey, mongoDB))
	mux.HandleFunc("GET /api/health/history", handleHealthHistory(mongoDB))
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /api/share/popular", handleGetPopularDomains(mongoDB))
	mux.HandleFunc("GET /api/share/popular/{domain}/screenshot", handleServeBrandScreenshot(mongoDB))
	mux.HandleFunc("GET /api/share/{shareId}", handleGetSharedDomain(mongoDB))

	// OPTIONS handlers (CORS preflight — no auth)
	// Note: OPTIONS /api/config already registered above with the GET handler
	mux.HandleFunc("OPTIONS /api/health/claude", handleOptions)
	mux.HandleFunc("OPTIONS /api/health", handleOptions)
	mux.HandleFunc("OPTIONS /api/analyze", handleOptions)
	mux.HandleFunc("OPTIONS /api/analyses", handleOptions)
	mux.HandleFunc("OPTIONS /api/analyses/{id}", handleOptions)
	mux.HandleFunc("OPTIONS /api/analyses/{id}/questions/{idx}/optimize", handleOptions)
	mux.HandleFunc("OPTIONS /api/analyses/{id}/questions/{idx}/optimization", handleOptions)
	mux.HandleFunc("OPTIONS /api/optimizations", handleOptions)
	mux.HandleFunc("OPTIONS /api/optimizations/{id}", handleOptions)
	mux.HandleFunc("OPTIONS /api/domains/{domain}/share", handleOptions)
	mux.HandleFunc("OPTIONS /api/share/popular", handleOptions)
	mux.HandleFunc("OPTIONS /api/share/popular/{domain}/screenshot", handleOptions)
	mux.HandleFunc("OPTIONS /api/share/{shareId}", handleOptions)
	mux.HandleFunc("OPTIONS /api/todos", handleOptions)
	mux.HandleFunc("OPTIONS /api/todos/{id}", handleOptions)
	mux.HandleFunc("OPTIONS /api/health/history", handleOptions)
	mux.HandleFunc("OPTIONS /api/domains/{domain}/summary", handleOptions)
	mux.HandleFunc("OPTIONS /api/domains/{domain}/summary/status", handleOptions)
	mux.HandleFunc("OPTIONS /api/brands", handleOptions)
	mux.HandleFunc("OPTIONS /api/brands/{domain}", handleOptions)
	mux.HandleFunc("OPTIONS /api/brands/{domain}/discover-competitors", handleOptions)
	mux.HandleFunc("OPTIONS /api/brands/{domain}/suggest-queries", handleOptions)
	mux.HandleFunc("OPTIONS /api/brands/{domain}/generate-description", handleOptions)
	mux.HandleFunc("OPTIONS /api/brands/{domain}/predict-audience", handleOptions)
	mux.HandleFunc("OPTIONS /api/brands/{domain}/suggest-claims", handleOptions)
	mux.HandleFunc("OPTIONS /api/brands/{domain}/predict-differentiators", handleOptions)
	mux.HandleFunc("OPTIONS /api/video/discover", handleOptions)
	mux.HandleFunc("OPTIONS /api/video/analyze", handleOptions)
	mux.HandleFunc("OPTIONS /api/video/analyses/{domain}/details", handleOptions)
	mux.HandleFunc("OPTIONS /api/video/analyses/{domain}", handleOptions)
	mux.HandleFunc("OPTIONS /api/video/analyses", handleOptions)
	mux.HandleFunc("OPTIONS /api/todos/archive", handleOptions)
	mux.HandleFunc("OPTIONS /api/reddit/discover", handleOptions)
	mux.HandleFunc("OPTIONS /api/reddit/analyze", handleOptions)
	mux.HandleFunc("OPTIONS /api/reddit/analyses/{domain}", handleOptions)
	mux.HandleFunc("OPTIONS /api/reddit/analyses", handleOptions)
	mux.HandleFunc("OPTIONS /api/search/analyze", handleOptions)
	mux.HandleFunc("OPTIONS /api/search/analyses/{domain}", handleOptions)
	mux.HandleFunc("OPTIONS /api/search/analyses", handleOptions)
	mux.HandleFunc("OPTIONS /api/domains/{domain}/report/pdf", handleOptions)
	mux.HandleFunc("OPTIONS /api/domains/{domain}/report/pdf/{id}", handleOptions)
	mux.HandleFunc("OPTIONS /api/visibility-score/{domain}", handleOptions)
	mux.HandleFunc("OPTIONS /api/providers/models", handleOptions)
	mux.HandleFunc("OPTIONS /api/test", handleOptions)
	mux.HandleFunc("OPTIONS /api/test/{domain}/history", handleOptions)
	mux.HandleFunc("OPTIONS /api/test/{domain}/competitors", handleOptions)
	mux.HandleFunc("OPTIONS /api/test/{domain}", handleOptions)
	mux.HandleFunc("OPTIONS /api/test/generate-queries", handleOptions)
	mux.HandleFunc("OPTIONS /api/settings/api-keys", handleOptions)
	mux.HandleFunc("OPTIONS /api/settings/api-keys/{provider}", handleOptions)
	mux.HandleFunc("OPTIONS /api/settings/api-keys/{provider}/verify", handleOptions)
	mux.HandleFunc("OPTIONS /api/settings/api-keys/status", handleOptions)

	// SaaS frontend: serve LastSaaS auth/admin pages when configured
	if saasFrontendDir := os.Getenv("LLMOPT_FRONTEND_DIR"); saasFrontendDir != "" {
		if info, statErr := os.Stat(saasFrontendDir); statErr == nil && info.IsDir() {
			log.Printf("Serving SaaS frontend from %s", saasFrontendDir)
			saasSPA := &spaHandler{staticPath: saasFrontendDir, indexPath: "index.html"}
			// Auth pages at root level
			for _, p := range []string{"/login", "/signup", "/forgot-password", "/reset-password", "/verify-email", "/auth/callback", "/auth/mfa", "/auth/magic-link", "/setup"} {
				mux.Handle(p, saasSPA)
			}
			// Admin/team/settings under /last/ — StripPrefix so asset paths
			// (e.g. /last/assets/index-ABC.js) resolve correctly in the SPA dir
			mux.Handle("/last/", http.StripPrefix("/last", saasSPA))
		}
	}

	// Serve main frontend static files if available
	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "../frontend/dist"
	}
	if info, err := os.Stat(staticDir); err == nil && info.IsDir() {
		log.Printf("Serving frontend from %s", staticDir)
		mux.Handle("/", &spaHandler{staticPath: staticDir, indexPath: "index.html"})
	} else {
		log.Printf("No frontend directory at %s, API-only mode", staticDir)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("LLM Optimizer backend starting on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, withCORS(mux)))
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Tenant-ID")
		next.ServeHTTP(w, r)
	})
}

// tenantFilter appends a tenantId filter element when running in SaaS mode.
// In non-SaaS mode (no tenant in context), returns the filter unchanged.
func tenantFilter(ctx context.Context, filter bson.D) bson.D {
	if tid := saas.TenantIDFromContext(ctx); tid != "" {
		return append(filter, bson.E{Key: "tenantId", Value: tid})
	}
	return filter
}

func handleOptions(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// generateShareID returns a 12-character base62 string using crypto/rand.
func generateShareID() string {
	const charset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 12)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

// isShareAdmin checks if the current user is an owner or admin of their tenant.
func isShareAdmin(ctx context.Context) bool {
	role := saas.MemberRoleFromContext(ctx)
	return role == "owner" || role == "admin"
}

// isRootShareAdmin checks if the user is an owner/admin of the root tenant.
func isRootShareAdmin(ctx context.Context) bool {
	if !isShareAdmin(ctx) {
		return false
	}
	tenant := saas.TenantFromContext(ctx)
	return tenant != nil && tenant.IsRoot
}

// spaHandler serves static files with SPA fallback to index.html.
type spaHandler struct {
	staticPath string
	indexPath  string
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(h.staticPath, filepath.Clean(r.URL.Path))

	fi, err := os.Stat(path)
	if os.IsNotExist(err) || (err == nil && fi.IsDir()) {
		// Never cache index.html — ensures new JS bundles are picked up
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		http.ServeFile(w, r, filepath.Join(h.staticPath, h.indexPath))
		return
	}
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Hashed assets (Vite bundles) can be cached long-term
	if strings.Contains(r.URL.Path, "/assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	http.FileServer(http.Dir(h.staticPath)).ServeHTTP(w, r)
}

func sendSSE(w http.ResponseWriter, f http.Flusher, eventType string, data any) {
	jsonData, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, jsonData)
	f.Flush()
}

// saveAndSendDone parses the result, saves to MongoDB, and sends the done SSE event.
func saveAndSendDone(w http.ResponseWriter, flusher http.Flusher, ctx context.Context, mongoDB *MongoDB, domain string, rawText string, resultJSON string, model string, brandInfo BrandContextInfo) {
	resultJSON = stripJSONFencing(resultJSON)
	var analysisResult AnalysisResult
	if err := json.Unmarshal([]byte(resultJSON), &analysisResult); err == nil {
		analysis := Analysis{
			Domain:                domain,
			TenantID:              saas.TenantIDFromContext(ctx),
			RawText:               rawText,
			Result:                analysisResult,
			Model:                 model,
			BrandContextUsed:      brandInfo.Used,
			BrandProfileUpdatedAt: brandInfo.ProfileUpdatedAt,
			CreatedAt:             time.Now(),
		}
		insertResult, insertErr := mongoDB.Analyses().InsertOne(ctx, analysis)
		var savedID string
		if insertErr != nil {
			log.Printf("Failed to save analysis: %v", insertErr)
		} else if oid, ok := insertResult.InsertedID.(primitive.ObjectID); ok {
			savedID = oid.Hex()
		}
		sendSSE(w, flusher, "done", map[string]any{
			"result":                   resultJSON,
			"id":                       savedID,
			"cached":                   false,
			"model":                    model,
			"created_at":               analysis.CreatedAt,
			"brand_context_used":       brandInfo.Used,
			"brand_profile_updated_at": brandInfo.ProfileUpdatedAt,
		})
	} else {
		log.Printf("Failed to parse analysis result for saving: %v", err)
		sendSSE(w, flusher, "done", map[string]string{"result": resultJSON})
	}
}

// errOverloaded is kept as an alias for backward compatibility within this file.
var errOverloaded = ErrOverloaded

// stripJSONFencing removes markdown code fences (```json ... ```) that Claude
// sometimes wraps around JSON responses.
func stripJSONFencing(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		// Remove opening fence line
		if idx := strings.Index(s, "\n"); idx != -1 {
			s = s[idx+1:]
		}
		// Remove closing fence
		if idx := strings.LastIndex(s, "```"); idx != -1 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}
	return s
}


// assessVideo calls the provider's small model to assess a single video's transcript for LLM authority signals.
func assessVideo(ctx context.Context, provider LLMProvider, apiKey string, video YouTubeVideo, domain string, searchTerms []string) (*VideoAssessment, error) {
	if video.Transcript == "" {
		return nil, nil
	}

	prompt := fmt.Sprintf(`Assess this YouTube video transcript for LLM authority signals.

Brand/Domain: %s
Target Search Terms: %s
Video: %s by %s
Views: %d | Published: %s

Transcript:
%s

Assess:
1. keyword_alignment (0-100): How well do target search terms appear naturally in spoken words?
2. quotability (0-100): Are there standalone, citable statements an LLM could extract?
3. info_density (0-100): Focused expert content vs. filler/rambling?
4. key_quotes: 2-3 most citable sentences (exact from transcript)
5. topics: Main topics covered (3-5 items)
6. brand_sentiment: How is %s discussed? (positive/negative/neutral/none)
7. summary: 1-2 sentence summary of what an LLM would extract from this video

Return ONLY valid JSON: {"keyword_alignment":N,"quotability":N,"info_density":N,"key_quotes":["..."],"topics":["..."],"brand_sentiment":"...","summary":"..."}`,
		domain,
		strings.Join(searchTerms, ", "),
		video.Title,
		video.ChannelTitle,
		video.ViewCount,
		video.PublishedAt.Format("2006-01-02"),
		video.Transcript,
		domain,
	)

	const maxRetries = 2
	backoff := 2 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			backoff *= 2
		}

		text, err := provider.Call(ctx, apiKey, provider.SmallModel(), prompt, 1024)
		if err == errOverloaded {
			if attempt < maxRetries {
				continue
			}
			return nil, fmt.Errorf("Haiku overloaded after %d retries", maxRetries)
		}
		if err != nil {
			return nil, err
		}

		// Extract JSON from response (may have markdown wrapping)
		text = strings.TrimSpace(text)
		if idx := strings.Index(text, "{"); idx >= 0 {
			if end := strings.LastIndex(text, "}"); end > idx {
				text = text[idx : end+1]
			}
		}

		var a VideoAssessment
		if err := json.Unmarshal([]byte(text), &a); err != nil {
			return nil, fmt.Errorf("failed to parse assessment: %w", err)
		}
		a.VideoID = video.VideoID
		a.Title = video.Title
		a.HasTranscript = true
		return &a, nil
	}
	return nil, fmt.Errorf("exhausted retries")
}

// assessVideos runs Phase 1: concurrent per-video assessments with the provider's small model.
// Returns a map of videoID -> assessment. Nil values mean no transcript or assessment failed.
func assessVideos(ctx context.Context, provider LLMProvider, apiKey string, videos []YouTubeVideo, domain string, searchTerms []string, mongoDB *MongoDB, w http.ResponseWriter, flusher http.Flusher) map[string]*VideoAssessment {
	results := make(map[string]*VideoAssessment)
	var mu sync.Mutex

	type assessResult struct {
		videoID    string
		assessment *VideoAssessment
		fromCache  bool
		err        error
	}

	resultsCh := make(chan assessResult, len(videos))
	sem := make(chan struct{}, 8) // 8 concurrent assessments

	for _, v := range videos {
		go func(video YouTubeVideo) {
			sem <- struct{}{}
			defer func() { <-sem }()

			// Check cache first
			if cached, ok := cachedVideoAssessment(mongoDB, video.VideoID, domain, searchTerms); ok {
				resultsCh <- assessResult{videoID: video.VideoID, assessment: cached, fromCache: true}
				return
			}

			// Skip videos without transcripts
			if video.Transcript == "" {
				resultsCh <- assessResult{videoID: video.VideoID}
				return
			}

			a, err := assessVideo(ctx, provider, apiKey, video, domain, searchTerms)
			if err != nil {
				log.Printf("Warning: assessment failed for %s: %v", video.VideoID, err)
				resultsCh <- assessResult{videoID: video.VideoID, err: err}
				return
			}

			// Cache the result
			if a != nil {
				setCachedVideoAssessment(mongoDB, video.VideoID, domain, searchTerms, a)
			}
			resultsCh <- assessResult{videoID: video.VideoID, assessment: a}
		}(v)
	}

	cachedCount, assessedCount, skippedCount, failedCount := 0, 0, 0, 0
	for i := 0; i < len(videos); i++ {
		r := <-resultsCh
		mu.Lock()
		results[r.videoID] = r.assessment
		if r.fromCache {
			cachedCount++
		} else if r.assessment != nil {
			assessedCount++
		} else if r.err != nil {
			failedCount++
		} else {
			skippedCount++
		}
		mu.Unlock()

		sendSSE(w, flusher, "progress", map[string]string{
			"message": fmt.Sprintf("Assessing transcripts (%d/%d)... [%d cached, %d assessed, %d skipped, %d failed]",
				i+1, len(videos), cachedCount, assessedCount, skippedCount, failedCount),
		})
	}

	return results
}

func handleAnalyze(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		var req struct {
			URL   string `json:"url"`
			Force bool   `json:"force"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Invalid request body"})
			return
		}
		if req.URL == "" {
			sendSSE(w, flusher, "error", map[string]string{"message": "URL is required"})
			return
		}
		req.URL = normalizeDomain(req.URL)

		// Check for cached analysis (< 30 days old) unless force refresh
		if !req.Force {
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			var cached Analysis
			cacheFilter := tenantFilter(r.Context(), bson.D{
				{Key: "domain", Value: req.URL},
				{Key: "createdAt", Value: bson.D{{Key: "$gt", Value: time.Now().AddDate(0, 0, -30)}}},
			})
			err := mongoDB.Analyses().FindOne(ctx, cacheFilter, options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}})).Decode(&cached)
			cancel()

			if err == nil {
				sendSSE(w, flusher, "status", map[string]string{
					"message": "Found cached analysis...",
				})
				resultJSON, _ := json.Marshal(cached.Result)
				sendSSE(w, flusher, "done", map[string]any{
					"result":                   string(resultJSON),
					"id":                       cached.ID.Hex(),
					"cached":                   true,
					"model":                    cached.Model,
					"created_at":               cached.CreatedAt,
					"brand_context_used":       cached.BrandContextUsed,
					"brand_profile_updated_at": cached.BrandProfileUpdatedAt,
				})
				return
			}
		}

		// Resolve primary LLM provider and API key for this tenant
		provider, apiKey, _, err := resolvePrimaryLLM(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Configure an API key in Settings", "code": "api_key_required"})
			return
		}

		sendSSE(w, flusher, "status", map[string]string{
			"message": "Starting analysis of " + req.URL + "...",
		})

		brandInfo := lookupBrandContext(mongoDB, req.URL, saas.TenantIDFromContext(r.Context()))

		brandInstructions := ""
		if brandInfo.Used {
			brandInstructions = `

IMPORTANT — Brand Aspiration Cross-Reference:
The brand intelligence context below includes "Target Queries" and "Brand Claims". These represent aspirations — questions the brand WANTS people to ask about them, and claims they WANT to make — whether or not the site currently supports them.

After generating the questions you discover organically from the site, cross-reference every Target Query and Brand Claim against what you found:

1. If the site handles this well with strong, relevant content → include as a question with brand_status: "normal"
2. If the site mentions the topic but weakly, tangentially, or without dedicated content → brand_status: "aspirational"
3. If the site does NOT appear to address this at all → brand_status: "missing" with page_urls: []

Questions discovered organically (not from brand aspirations) should omit brand_status or use "normal".

The JSON format for questions is:
{
  "question": "...",
  "relevance": "...",
  "category": "...",
  "page_urls": [...],
  "brand_status": "normal" | "aspirational" | "missing"
}`
		}

		prompt := fmt.Sprintf(`You are a website content analyzer. Your task is to understand what a website is about and determine what questions people would likely ask that this website can answer.

Website to analyze: %s

Please:
1. Search for and visit this website to understand its content, purpose, and offerings
2. Browse multiple pages on the site — the homepage, key subpages, product/service pages, about pages, blog posts, etc.
3. Analyze what the website provides — its products, services, information, etc.
4. Think about what questions a user might type into a search engine or AI assistant that this website would be well-suited to answer
5. Track which specific pages you visited and which pages are relevant to each question

Return your analysis as JSON in exactly this format (no markdown code fences, just raw JSON):
{
  "site_summary": "Brief description of what the website is about",
  "crawled_pages": [
    {
      "url": "Full URL of a page you visited",
      "title": "Page title or short description"
    }
  ],
  "questions": [
    {
      "question": "The question a user might ask",
      "relevance": "Why this website is relevant to this question",
      "category": "Category like Product, Pricing, How-to, Comparison, General, etc.",
      "page_urls": ["https://example.com/page1", "https://example.com/page2"]
    }
  ]
}

For crawled_pages, list every distinct page on the site you visited or found during your search.
For page_urls in each question, list the specific page URL(s) from the site that answer or are most relevant to that question. Use the actual URLs you found, not fabricated ones.

Generate 15-20 diverse questions across different categories. Include questions at different levels of specificity — from broad queries to very specific ones.%s%s`, req.URL, brandInstructions, brandInfo.ContextString)

		models := provider.Models()

		for mi, model := range models {
			if mi > 0 {
				sendSSE(w, flusher, "status", map[string]string{
					"message": fmt.Sprintf("%s unavailable, falling back to %s...", models[mi-1].Name, model.Name),
				})
			}

			claudeBody, _ := provider.BuildStreamBody(model.ID, 16384, prompt, true)

			const maxRetries = 3
			backoff := 2 * time.Second
			var lastErr error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					sendSSE(w, flusher, "status", map[string]string{
						"message": fmt.Sprintf("%s overloaded, retrying in %ds (attempt %d/%d)...", model.Name, int(backoff.Seconds()), attempt, maxRetries),
					})
					select {
					case <-time.After(backoff):
					case <-r.Context().Done():
						return
					}
					backoff *= 2
				}

				result, err := provider.Stream(r.Context(), apiKey, claudeBody, w, flusher)
				if err == errOverloaded {
					lastErr = err
					if attempt < maxRetries {
						log.Printf("%s API (%s) overloaded, will retry (attempt %d/%d)", provider.Name(), model.ID, attempt+1, maxRetries)
						continue
					}
					break // exhausted retries, try next model
				}
				if err != nil {
					sendSSE(w, flusher, "error", map[string]string{"message": err.Error()})
					return
				}

				saveAndSendDone(w, flusher, r.Context(), mongoDB, req.URL, result.RawText, result.ResultJSON, model.Name, brandInfo)
				return
			}

			log.Printf("%s API (%s) exhausted retries: %v", provider.Name(), model.ID, lastErr)
		}

		sendSSE(w, flusher, "error", map[string]string{
			"message": "All models are currently overloaded. Please try again later.",
		})
	}
}

func handleListAnalyses(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{})
		sort := bson.D{{Key: "createdAt", Value: -1}}
		if domain := r.URL.Query().Get("domain"); domain != "" {
			filter = append(filter, bson.E{Key: "domain", Value: normalizeDomain(domain)})
			// When filtering by domain, sort brand-intel reports first, then by date
			sort = bson.D{{Key: "brandContextUsed", Value: -1}, {Key: "createdAt", Value: -1}}
		}

		opts := options.Find().
			SetSort(sort).
			SetLimit(50).
			SetProjection(bson.D{
				{Key: "domain", Value: 1},
				{Key: "createdAt", Value: 1},
				{Key: "model", Value: 1},
				{Key: "result.siteSummary", Value: 1},
				{Key: "result.questions", Value: 1},
				{Key: "result.crawledPages", Value: 1},
				{Key: "brandContextUsed", Value: 1},
				{Key: "brandProfileUpdatedAt", Value: 1},
			})

		cursor, err := mongoDB.Analyses().Find(ctx, filter, opts)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)

		var analyses []Analysis
		if err := cursor.All(ctx, &analyses); err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		summaries := make([]AnalysisSummary, len(analyses))
		for i, a := range analyses {
			summaries[i] = AnalysisSummary{
				ID:                    a.ID,
				Domain:                a.Domain,
				SiteSummary:           a.Result.SiteSummary,
				QuestionCount:         len(a.Result.Questions),
				PageCount:             len(a.Result.CrawledPages),
				Model:                 a.Model,
				BrandContextUsed:      a.BrandContextUsed,
				BrandProfileUpdatedAt: a.BrandProfileUpdatedAt,
				CreatedAt:             a.CreatedAt,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(summaries)
	}
}

func handleGetAnalysis(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		oid, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		var analysis Analysis
		err = mongoDB.Analyses().FindOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "_id", Value: oid}})).Decode(&analysis)
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(analysis)
	}
}

type modelCheckResult struct {
	Model      string `json:"model"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	HTTPStatus int    `json:"http_status,omitempty"`
	LatencyMs  int64  `json:"latency_ms,omitempty"`
	Error      string `json:"error,omitempty"`
}

func handleHealthCheck(apiKey string, mongoDB *MongoDB) http.HandlerFunc {
	type modelDef struct {
		id, name string
	}
	models := []modelDef{
		{"claude-sonnet-4-6", "Sonnet 4.6"},
		{"claude-haiku-4-5-20251001", "Haiku 4.5"},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		results := make([]modelCheckResult, len(models))
		var wg sync.WaitGroup

		for i, model := range models {
			wg.Add(1)
			go func(idx int, m modelDef) {
				defer wg.Done()

				ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
				defer cancel()

				body, _ := json.Marshal(map[string]any{
					"model":      m.id,
					"max_tokens": 10,
					"messages": []map[string]any{
						{"role": "user", "content": "Reply with just the word 'ok'."},
					},
				})

				httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
				if err != nil {
					results[idx] = modelCheckResult{Model: m.id, Name: m.name, Status: "error", Error: "Failed to create request"}
					return
				}
				httpReq.Header.Set("Content-Type", "application/json")
				httpReq.Header.Set("x-api-key", apiKey)
				httpReq.Header.Set("anthropic-version", "2023-06-01")

				start := time.Now()
				resp, err := http.DefaultClient.Do(httpReq)
				latency := time.Since(start)

				if err != nil {
					results[idx] = modelCheckResult{Model: m.id, Name: m.name, Status: "error", Error: err.Error()}
					return
				}
				defer resp.Body.Close()

				check := modelCheckResult{
					Model:      m.id,
					Name:       m.name,
					HTTPStatus: resp.StatusCode,
					LatencyMs:  latency.Milliseconds(),
				}

				switch {
				case resp.StatusCode == 200:
					check.Status = "available"
				case resp.StatusCode == 529:
					check.Status = "overloaded"
					errBody, _ := io.ReadAll(resp.Body)
					check.Error = string(errBody)
				default:
					check.Status = "error"
					errBody, _ := io.ReadAll(resp.Body)
					check.Error = string(errBody)
				}

				results[idx] = check
			}(i, model)
		}

		wg.Wait()

		checkedAt := time.Now()

		// Persist to DB
		var modelRecords []ModelStatusRecord
		for _, r := range results {
			modelRecords = append(modelRecords, ModelStatusRecord{
				Model:      r.Model,
				Name:       r.Name,
				Status:     r.Status,
				LatencyMs:  r.LatencyMs,
				HTTPStatus: r.HTTPStatus,
			})
		}
		record := HealthRecord{
			Models:    modelRecords,
			CheckedAt: checkedAt,
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if _, err := mongoDB.HealthChecks().InsertOne(ctx, record); err != nil {
				log.Printf("Failed to save health check: %v", err)
			}
		}()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"models":     results,
			"checked_at": checkedAt,
		})
	}
}

func handleHealthHistory(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Default: last 24 hours
		hours := 24
		if h := r.URL.Query().Get("hours"); h != "" {
			if parsed, err := strconv.Atoi(h); err == nil && parsed > 0 && parsed <= 168 {
				hours = parsed
			}
		}

		since := time.Now().Add(-time.Duration(hours) * time.Hour)
		opts := options.Find().
			SetSort(bson.D{{Key: "checkedAt", Value: 1}}).
			SetLimit(500)

		cursor, err := mongoDB.HealthChecks().Find(ctx, bson.D{
			{Key: "checkedAt", Value: bson.D{{Key: "$gte", Value: since}}},
		}, opts)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)

		var records []HealthRecord
		if err := cursor.All(ctx, &records); err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		if records == nil {
			records = []HealthRecord{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(records)
	}
}

// --- API Key Management Handlers ---

func handleListAPIKeys(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := saas.TenantIDFromContext(r.Context())

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		cursor, err := mongoDB.TenantAPIKeys().Find(ctx, bson.M{"tenantId": tenantID})
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)

		var keys []TenantAPIKey
		if err := cursor.All(ctx, &keys); err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		if keys == nil {
			keys = []TenantAPIKey{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"keys": keys})
	}
}

func handleSetAPIKey(mongoDB *MongoDB, encKey []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if role := saas.MemberRoleFromContext(r.Context()); role != "owner" {
			http.Error(w, `{"error":"forbidden","message":"Only the team owner can manage API keys"}`, http.StatusForbidden)
			return
		}
		provider := r.PathValue("provider")
		validProviders := map[string]bool{"anthropic": true, "openai": true, "grok": true, "gemini": true}
		if !validProviders[provider] {
			http.Error(w, `{"error":"invalid provider"}`, http.StatusBadRequest)
			return
		}

		var req struct {
			Key            string `json:"key"`
			PreferredModel string `json:"preferred_model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		if req.Key == "" {
			http.Error(w, `{"error":"key is required"}`, http.StatusBadRequest)
			return
		}

		tenantID := saas.TenantIDFromContext(r.Context())

		// Verify key with the provider
		status := "active"
		if p := getProvider(provider); p != nil {
			var err error
			status, err = p.VerifyKey(r.Context(), req.Key)
			if err != nil {
				log.Printf("API key verification error for tenant %s (%s): %v", tenantID, provider, err)
			}
		}

		// Encrypt the key
		encrypted, err := encryptSecret(req.Key, encKey)
		if err != nil {
			log.Printf("Failed to encrypt API key for tenant %s: %v", tenantID, err)
			http.Error(w, `{"error":"encryption failed"}`, http.StatusInternalServerError)
			return
		}

		// Build key prefix for display (first 8 chars or less)
		prefix := req.Key
		if len(prefix) > 8 {
			prefix = prefix[:8]
		}
		prefix += "..."

		now := time.Now()
		doc := TenantAPIKey{
			TenantID:       tenantID,
			Provider:       provider,
			EncryptedKey:   encrypted,
			KeyPrefix:      prefix,
			PreferredModel: req.PreferredModel,
			Status:         status,
			LastVerifiedAt: &now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := bson.M{"tenantId": tenantID, "provider": provider}
		update := bson.M{
			"$set": bson.M{
				"encryptedKey":   encrypted,
				"keyPrefix":      prefix,
				"preferredModel": req.PreferredModel,
				"status":         status,
				"lastVerifiedAt": now,
				"updatedAt":      now,
			},
			"$setOnInsert": bson.M{
				"tenantId":  tenantID,
				"provider":  provider,
				"createdAt": now,
			},
		}
		_, err = mongoDB.TenantAPIKeys().UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
		if err != nil {
			log.Printf("Failed to store API key for tenant %s: %v", tenantID, err)
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		// Return the result without the encrypted key
		doc.EncryptedKey = ""
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(doc)
	}
}

func handleDeleteAPIKey(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if role := saas.MemberRoleFromContext(r.Context()); role != "owner" {
			http.Error(w, `{"error":"forbidden","message":"Only the team owner can manage API keys"}`, http.StatusForbidden)
			return
		}
		provider := r.PathValue("provider")
		tenantID := saas.TenantIDFromContext(r.Context())

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		result, err := mongoDB.TenantAPIKeys().DeleteOne(ctx, bson.M{
			"tenantId": tenantID,
			"provider": provider,
		})
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		if result.DeletedCount == 0 {
			http.Error(w, `{"error":"key not found"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"message":"API key removed"}`))
	}
}

func handleVerifyAPIKey(mongoDB *MongoDB, encKey []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if role := saas.MemberRoleFromContext(r.Context()); role != "owner" {
			http.Error(w, `{"error":"forbidden","message":"Only the team owner can manage API keys"}`, http.StatusForbidden)
			return
		}
		provider := r.PathValue("provider")
		tenantID := saas.TenantIDFromContext(r.Context())

		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()

		var doc TenantAPIKey
		err := mongoDB.TenantAPIKeys().FindOne(ctx, bson.M{
			"tenantId": tenantID,
			"provider": provider,
		}).Decode(&doc)
		if err != nil {
			http.Error(w, `{"error":"key not found"}`, http.StatusNotFound)
			return
		}

		plainKey, err := decryptSecret(doc.EncryptedKey, encKey)
		if err != nil {
			http.Error(w, `{"error":"decryption failed"}`, http.StatusInternalServerError)
			return
		}

		status := "active"
		if p := getProvider(provider); p != nil {
			status, err = p.VerifyKey(ctx, plainKey)
			if err != nil {
				log.Printf("API key re-verify error for tenant %s (%s): %v", tenantID, provider, err)
			}
		}

		now := time.Now()
		mongoDB.TenantAPIKeys().UpdateOne(ctx, bson.M{
			"tenantId": tenantID,
			"provider": provider,
		}, bson.M{"$set": bson.M{
			"status":         status,
			"lastVerifiedAt": now,
			"updatedAt":      now,
		}})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":          status,
			"last_verified_at": now,
		})
	}
}

// resolveOwnerName looks up the display name of the tenant's owner.
func resolveOwnerName(ctx context.Context, mongoDB *MongoDB, tenantID string) string {
	oid, err := primitive.ObjectIDFromHex(tenantID)
	if err != nil {
		return ""
	}

	// Find the owner membership
	var membership struct {
		UserID primitive.ObjectID `bson:"userId"`
	}
	err = mongoDB.Database.Collection("tenant_memberships").FindOne(ctx, bson.M{
		"tenantId": oid,
		"role":     "owner",
	}).Decode(&membership)
	if err != nil {
		return ""
	}

	// Look up the owner's display name
	var user struct {
		DisplayName string `bson:"displayName"`
	}
	err = mongoDB.Database.Collection("users").FindOne(ctx, bson.M{
		"_id": membership.UserID,
	}).Decode(&user)
	if err != nil {
		return ""
	}
	return user.DisplayName
}

func handleAPIKeyStatus(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := saas.TenantIDFromContext(r.Context())
		role := saas.MemberRoleFromContext(r.Context())

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Resolve owner name for non-owners
		ownerName := ""
		if role != "owner" {
			ownerName = resolveOwnerName(ctx, mongoDB, tenantID)
		}

		// Check if ANY provider has an active key
		cursor, err := mongoDB.TenantAPIKeys().Find(ctx, bson.M{"tenantId": tenantID})
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"has_key":    false,
				"status":     "unconfigured",
				"role":       role,
				"owner_name": ownerName,
			})
			return
		}
		defer cursor.Close(ctx)

		var keys []TenantAPIKey
		if err := cursor.All(ctx, &keys); err != nil || len(keys) == 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"has_key":    false,
				"status":     "unconfigured",
				"role":       role,
				"owner_name": ownerName,
			})
			return
		}

		// Determine primary provider
		var settings TenantSettings
		primaryProvider := "anthropic"
		if err := mongoDB.TenantSettings().FindOne(ctx, bson.M{"tenantId": tenantID}).Decode(&settings); err == nil && settings.PrimaryProvider != "" {
			primaryProvider = settings.PrimaryProvider
		}

		// has_key is true if any key exists; status is "active" if any key is active
		hasActive := false
		for _, k := range keys {
			if k.Status == "active" {
				hasActive = true
				break
			}
		}

		overallStatus := "inactive"
		if hasActive {
			overallStatus = "active"
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"has_key":          true,
			"status":           overallStatus,
			"primary_provider": primaryProvider,
			"role":             role,
			"owner_name":       ownerName,
		})
	}
}

func handleGetPrimaryProvider(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := saas.TenantIDFromContext(r.Context())

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		var settings TenantSettings
		primaryProvider := "anthropic"
		if err := mongoDB.TenantSettings().FindOne(ctx, bson.M{"tenantId": tenantID}).Decode(&settings); err == nil && settings.PrimaryProvider != "" {
			primaryProvider = settings.PrimaryProvider
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"primary_provider": primaryProvider,
		})
	}
}

func handleSetPrimaryProvider(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if role := saas.MemberRoleFromContext(r.Context()); role != "owner" {
			http.Error(w, `{"error":"forbidden","message":"Only the team owner can change the primary provider"}`, http.StatusForbidden)
			return
		}

		var req struct {
			Provider string `json:"provider"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}

		if getProvider(req.Provider) == nil {
			http.Error(w, `{"error":"invalid provider"}`, http.StatusBadRequest)
			return
		}

		tenantID := saas.TenantIDFromContext(r.Context())

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Verify the tenant has an active key for this provider
		var doc TenantAPIKey
		err := mongoDB.TenantAPIKeys().FindOne(ctx, bson.M{
			"tenantId": tenantID,
			"provider": req.Provider,
		}).Decode(&doc)
		if err != nil {
			http.Error(w, `{"error":"no_key","message":"You must configure an API key for this provider first"}`, http.StatusBadRequest)
			return
		}

		now := time.Now()
		_, err = mongoDB.TenantSettings().UpdateOne(ctx,
			bson.M{"tenantId": tenantID},
			bson.M{
				"$set": bson.M{
					"primaryProvider": req.Provider,
					"updatedAt":       now,
				},
				"$setOnInsert": bson.M{
					"tenantId": tenantID,
				},
			},
			options.Update().SetUpsert(true),
		)
		if err != nil {
			log.Printf("Failed to set primary provider for tenant %s: %v", tenantID, err)
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"primary_provider": req.Provider,
		})
	}
}

func handleOptimize(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		// Parse path params
		analysisIDStr := r.PathValue("id")
		questionIdxStr := r.PathValue("idx")

		analysisOID, err := primitive.ObjectIDFromHex(analysisIDStr)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Invalid analysis ID"})
			return
		}
		questionIdx, err := strconv.Atoi(questionIdxStr)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Invalid question index"})
			return
		}

		// Parse optional force flag
		var req struct {
			Force bool `json:"force"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		// Load parent analysis
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		var analysis Analysis
		err = mongoDB.Analyses().FindOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "_id", Value: analysisOID}})).Decode(&analysis)
		cancel()
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Analysis not found"})
			return
		}

		if questionIdx < 0 || questionIdx >= len(analysis.Result.Questions) {
			sendSSE(w, flusher, "error", map[string]string{"message": "Question index out of range"})
			return
		}

		question := analysis.Result.Questions[questionIdx]

		// Cache check
		if !req.Force {
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			var cached Optimization
			optCacheFilter := tenantFilter(r.Context(), bson.D{
				{Key: "analysisId", Value: analysisOID},
				{Key: "questionIndex", Value: questionIdx},
				{Key: "createdAt", Value: bson.D{{Key: "$gt", Value: time.Now().AddDate(0, 0, -30)}}},
			})
			err := mongoDB.Optimizations().FindOne(ctx, optCacheFilter, options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}})).Decode(&cached)
			cancel()

			if err == nil {
				sendSSE(w, flusher, "status", map[string]string{"message": "Found cached optimization..."})
				resultJSON, _ := json.Marshal(cached.Result)
				sendSSE(w, flusher, "done", map[string]any{
					"result":                   string(resultJSON),
					"id":                       cached.ID.Hex(),
					"cached":                   true,
					"model":                    cached.Model,
					"created_at":               cached.CreatedAt,
					"brand_context_used":       cached.BrandContextUsed,
					"brand_profile_updated_at": cached.BrandProfileUpdatedAt,
					"brand_status":             cached.BrandStatus,
				})
				return
			}
		}

		// Resolve primary LLM provider and API key for this tenant
		provider, apiKey, _, err := resolvePrimaryLLM(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Configure an API key in Settings", "code": "api_key_required"})
			return
		}

		sendSSE(w, flusher, "status", map[string]string{
			"message": "Starting optimization analysis for: " + question.Question,
		})

		optBrandInfo := lookupBrandContext(mongoDB, analysis.Domain, saas.TenantIDFromContext(r.Context()))

		pageURLsList := strings.Join(question.PageURLs, "\n- ")
		if pageURLsList != "" {
			pageURLsList = "\n- " + pageURLsList
		}

		brandStatusNote := ""
		if question.BrandStatus == "aspirational" {
			brandStatusNote = `

NOTE — ASPIRATIONAL QUESTION: This question is a brand aspiration — the brand wants to be found for this topic, but the site only weakly addresses it. Recommendations should emphasize creating or significantly enhancing content for this topic. Score dimensions realistically based on the current weak coverage.`
		} else if question.BrandStatus == "missing" {
			brandStatusNote = `

NOTE — MISSING COVERAGE: This question is a brand aspiration NOT currently addressed on the site at all. Recommendations should focus on creating new content from scratch. All dimension scores will likely be very low. Consider what competitors do for this topic as a benchmark for what content to create.`
		}

		prompt := fmt.Sprintf(`You are an LLM visibility analyst. Your task is to assess how likely a large language model (like ChatGPT, Claude, Perplexity, or Gemini with web search) is to surface and cite a website's answer to a specific question.

Website domain: %s
Question: %s
Relevant page URLs from this site:%s

Your analysis must evaluate four dimensions, each scored 0-100:

## Dimension 1: Content Authority (30%% weight)
Visit the page URL(s) above and evaluate:
- Does the content include quotations from authoritative sources? (Research shows +41%% visibility)
- Does it include specific statistics and data points? (+33%% visibility)
- Does it cite external references/sources inline? (+28%% visibility)
- Is the writing fluent and well-structured? (+29%% visibility)
- Does it use appropriate technical/domain terminology? (+19%% visibility)
- Is there keyword stuffing or marketing fluff? (-9%% visibility — harmful)

## Dimension 2: Structural Optimization (20%% weight)
- Is the answer to the question front-loaded (prominent) or buried deep in the page?
- Is the content concise and focused, or sprawling and padded?
- Are there machine-readable structures (Schema.org, comparison tables, FAQ blocks, bulleted lists)?
- Does the content explain "why" not just "what" — providing justification language?

## Dimension 3: Source Authority (30%% weight)
Search the web for third-party coverage:
- Is this site mentioned by independent review sites, industry publications, or analysts?
- Is this earned media coverage (independent third parties) rather than brand-owned or social content?
- AI search engines cite earned media 72-92%% of the time and virtually ignore social content.
- How does this site's authority compare to other sites answering the same question?

## Dimension 4: Knowledge Persistence (20%% weight)
Search the web to assess:
- How widely does this answer/information appear across the web? (Higher frequency = better)
- Is the content written in a clear, educational, didactic style?
- Would this content be effective as RAG context — clear, self-contained answer passages?
- Information widely repeated across high-quality sources is far more likely encoded in LLM weights.

Also identify:
- 3-5 competing websites that answer this same question (search for the question)
- For each competitor: estimate their overall score and note their key strengths
- 3-5 prioritized recommendations (high/medium/low) for improving the site's LLM visibility for this question

Return your analysis as JSON in exactly this format (no markdown code fences, just raw JSON):
{
  "overall_score": <0-100 weighted average>,
  "content_authority": {
    "score": <0-100>,
    "evidence": ["specific finding 1", "specific finding 2"],
    "improvements": ["actionable improvement 1", "actionable improvement 2"]
  },
  "structural_optimization": {
    "score": <0-100>,
    "evidence": ["specific finding 1", "specific finding 2"],
    "improvements": ["actionable improvement 1", "actionable improvement 2"]
  },
  "source_authority": {
    "score": <0-100>,
    "evidence": ["specific finding 1", "specific finding 2"],
    "improvements": ["actionable improvement 1", "actionable improvement 2"]
  },
  "knowledge_persistence": {
    "score": <0-100>,
    "evidence": ["specific finding 1", "specific finding 2"],
    "improvements": ["actionable improvement 1", "actionable improvement 2"]
  },
  "competitors": [
    {
      "domain": "competitor-site.com",
      "score_estimate": <0-100>,
      "strengths": "Brief description of why they rank well"
    }
  ],
  "recommendations": [
    {
      "priority": "high",
      "action": "Specific actionable step",
      "expected_impact": "What improvement to expect",
      "dimension": "content_authority"
    }
  ]
}

Be specific and evidence-based in your scoring. Reference actual content you found on the pages. The overall_score should be the weighted average: content_authority*0.30 + structural_optimization*0.20 + source_authority*0.30 + knowledge_persistence*0.20.%s%s`, analysis.Domain, question.Question, pageURLsList, brandStatusNote, optBrandInfo.ContextString)

		models := provider.Models()

		for mi, model := range models {
			if mi > 0 {
				sendSSE(w, flusher, "status", map[string]string{
					"message": fmt.Sprintf("%s unavailable, falling back to %s...", models[mi-1].Name, model.Name),
				})
			}

			claudeBody, _ := provider.BuildStreamBody(model.ID, 16384, prompt, true)

			const maxRetries = 3
			backoff := 2 * time.Second
			var lastErr error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					sendSSE(w, flusher, "status", map[string]string{
						"message": fmt.Sprintf("%s overloaded, retrying in %ds (attempt %d/%d)...", model.Name, int(backoff.Seconds()), attempt, maxRetries),
					})
					select {
					case <-time.After(backoff):
					case <-r.Context().Done():
						return
					}
					backoff *= 2
				}

				result, err := provider.Stream(r.Context(), apiKey, claudeBody, w, flusher)
				if err == errOverloaded {
					lastErr = err
					if attempt < maxRetries {
						log.Printf("%s API (%s) overloaded for optimization, will retry (attempt %d/%d)", provider.Name(), model.ID, attempt+1, maxRetries)
						continue
					}
					break
				}
				if err != nil {
					sendSSE(w, flusher, "error", map[string]string{"message": err.Error()})
					return
				}

				// Parse and save
				cleanJSON := stripJSONFencing(result.ResultJSON)
				var optResult OptimizationResult
				if err := json.Unmarshal([]byte(cleanJSON), &optResult); err != nil {
					log.Printf("Failed to parse optimization result: %v", err)
					sendSSE(w, flusher, "done", map[string]string{"result": result.ResultJSON})
					return
				}

				opt := Optimization{
					AnalysisID:            analysisOID,
					QuestionIndex:         questionIdx,
					Question:              question.Question,
					Domain:                analysis.Domain,
					TenantID:              saas.TenantIDFromContext(r.Context()),
					PageURLs:              question.PageURLs,
					Result:                optResult,
					RawText:               result.RawText,
					BrandStatus:           question.BrandStatus,
					Model:                 model.Name,
					BrandContextUsed:      optBrandInfo.Used,
					BrandProfileUpdatedAt: optBrandInfo.ProfileUpdatedAt,
					CreatedAt:             time.Now(),
				}
				insertResult, insertErr := mongoDB.Optimizations().InsertOne(r.Context(), opt)
				var savedID string
				if insertErr != nil {
					log.Printf("Failed to save optimization: %v", insertErr)
				} else if oid, ok := insertResult.InsertedID.(primitive.ObjectID); ok {
					savedID = oid.Hex()
					go createTodosFromOptimization(mongoDB, oid, analysisOID, analysis.Domain, question.Question, saas.TenantIDFromContext(r.Context()), optResult)
				}

				sendSSE(w, flusher, "done", map[string]any{
					"result":                   result.ResultJSON,
					"id":                       savedID,
					"cached":                   false,
					"model":                    model.Name,
					"created_at":               opt.CreatedAt,
					"brand_context_used":       optBrandInfo.Used,
					"brand_profile_updated_at": optBrandInfo.ProfileUpdatedAt,
					"brand_status":             question.BrandStatus,
				})
				return
			}

			log.Printf("%s API (%s) exhausted retries for optimization: %v", provider.Name(), model.ID, lastErr)
		}

		sendSSE(w, flusher, "error", map[string]string{
			"message": "All models are currently overloaded. Please try again later.",
		})
	}
}

func handleGetOptimization(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		analysisIDStr := r.PathValue("id")
		questionIdxStr := r.PathValue("idx")

		analysisOID, err := primitive.ObjectIDFromHex(analysisIDStr)
		if err != nil {
			http.Error(w, "Invalid analysis ID", http.StatusBadRequest)
			return
		}
		questionIdx, err := strconv.Atoi(questionIdxStr)
		if err != nil {
			http.Error(w, "Invalid question index", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		var opt Optimization
		err = mongoDB.Optimizations().FindOne(ctx, tenantFilter(r.Context(), bson.D{
			{Key: "analysisId", Value: analysisOID},
			{Key: "questionIndex", Value: questionIdx},
		}), options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}})).Decode(&opt)
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(opt)
	}
}

func handleListOptimizations(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		opts := options.Find().
			SetSort(bson.D{{Key: "createdAt", Value: -1}}).
			SetLimit(100).
			SetProjection(bson.D{
				{Key: "domain", Value: 1},
				{Key: "question", Value: 1},
				{Key: "questionIndex", Value: 1},
				{Key: "result.overallScore", Value: 1},
				{Key: "model", Value: 1},
				{Key: "public", Value: 1},
				{Key: "brandStatus", Value: 1},
				{Key: "brandContextUsed", Value: 1},
				{Key: "brandProfileUpdatedAt", Value: 1},
				{Key: "createdAt", Value: 1},
			})

		cursor, err := mongoDB.Optimizations().Find(ctx, tenantFilter(r.Context(), bson.D{}), opts)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)

		var optimizations []Optimization
		if err := cursor.All(ctx, &optimizations); err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		summaries := make([]OptimizationSummary, len(optimizations))
		for i, o := range optimizations {
			summaries[i] = OptimizationSummary{
				ID:                    o.ID,
				Domain:                o.Domain,
				Question:              o.Question,
				QuestionIndex:         o.QuestionIndex,
				OverallScore:          o.Result.OverallScore,
				Model:                 o.Model,
				Public:                o.Public,
				BrandStatus:           o.BrandStatus,
				BrandContextUsed:      o.BrandContextUsed,
				BrandProfileUpdatedAt: o.BrandProfileUpdatedAt,
				CreatedAt:             o.CreatedAt,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(summaries)
	}
}

func handleGetOptimizationByID(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		oid, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		var opt Optimization
		err = mongoDB.Optimizations().FindOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "_id", Value: oid}})).Decode(&opt)
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(opt)
	}
}

// todoSummary creates a 1-2 line summary from action and expected impact.
func todoSummary(action, impact string) string {
	// Truncate action to first sentence or 120 chars
	a := action
	if idx := strings.Index(a, ". "); idx > 0 && idx < 120 {
		a = a[:idx+1]
	} else if len(a) > 120 {
		a = a[:117] + "..."
	}
	// Append abbreviated impact if room
	if impact != "" {
		imp := impact
		if len(imp) > 80 {
			imp = imp[:77] + "..."
		}
		return a + " → " + imp
	}
	return a
}

func createTodosFromOptimization(mongoDB *MongoDB, optimizationID, analysisID primitive.ObjectID, domain, question, tenantID string, result OptimizationResult) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if len(result.Recommendations) == 0 {
		return
	}

	var todos []any
	now := time.Now()
	for _, rec := range result.Recommendations {
		todos = append(todos, TodoItem{
			OptimizationID: optimizationID,
			AnalysisID:     analysisID,
			Domain:         domain,
			TenantID:       tenantID,
			Question:       question,
			Action:         rec.Action,
			Summary:        todoSummary(rec.Action, rec.ExpectedImpact),
			ExpectedImpact: rec.ExpectedImpact,
			Dimension:      rec.Dimension,
			Priority:       rec.Priority,
			Status:         "todo",
			CreatedAt:      now,
		})
	}

	_, err := mongoDB.Todos().InsertMany(ctx, todos)
	if err != nil {
		log.Printf("Failed to create todos from optimization: %v", err)
	}

	// Deduplicate todos for this domain
	go deduplicateTodos(mongoDB, domain, tenantID)
}

func createTodosFromVideoAnalysis(mongoDB *MongoDB, videoAnalysisID primitive.ObjectID, domain, tenantID string, recommendations []VideoRecommendation) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if len(recommendations) == 0 {
		return
	}

	// Delete previous video todos for this domain to avoid duplicates on re-run
	mongoDB.Todos().DeleteMany(ctx, bson.D{
		{Key: "videoAnalysisId", Value: videoAnalysisID},
	})

	dimLabels := map[string]string{
		"transcript_authority": "Video: Transcript Authority",
		"topical_dominance":   "Video: Topical Dominance",
		"citation_network":    "Video: Citation Network",
		"brand_narrative":     "Video: Brand Narrative",
	}

	var todos []any
	now := time.Now()
	for _, rec := range recommendations {
		question := dimLabels[rec.Dimension]
		if question == "" {
			question = "Video: LLM Authority"
		}
		todos = append(todos, TodoItem{
			VideoAnalysisID: &videoAnalysisID,
			SourceType:      "video",
			Domain:          domain,
			TenantID:        tenantID,
			Question:        question,
			Action:          rec.Action,
			Summary:         todoSummary(rec.Action, rec.ExpectedImpact),
			ExpectedImpact:  rec.ExpectedImpact,
			Dimension:       rec.Dimension,
			Priority:        rec.Priority,
			Status:          "todo",
			CreatedAt:       now,
		})
	}

	_, err := mongoDB.Todos().InsertMany(ctx, todos)
	if err != nil {
		log.Printf("Failed to create todos from video analysis: %v", err)
	}

	// Deduplicate todos for this domain
	go deduplicateTodos(mongoDB, domain, tenantID)
}

// deduplicateTodos finds semantically similar open todos for a domain and archives duplicates.
// Uses normalized word overlap (Jaccard similarity) to detect similar actions.
func deduplicateTodos(mongoDB *MongoDB, domain, tenantID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	filter := bson.D{
		{Key: "domain", Value: domain},
		{Key: "status", Value: "todo"},
	}
	if tenantID != "" {
		filter = append(filter, bson.E{Key: "tenantId", Value: tenantID})
	}

	cursor, err := mongoDB.Todos().Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}))
	if err != nil {
		log.Printf("deduplicateTodos: find error: %v", err)
		return
	}
	var todos []TodoItem
	if err := cursor.All(ctx, &todos); err != nil {
		log.Printf("deduplicateTodos: cursor error: %v", err)
		return
	}

	if len(todos) < 2 {
		return
	}

	// Normalize and tokenize action text for Jaccard similarity
	tokenize := func(s string) map[string]bool {
		words := map[string]bool{}
		for _, w := range strings.Fields(strings.ToLower(s)) {
			w = strings.Trim(w, ".,;:!?\"'()-")
			if len(w) > 2 { // skip small words
				words[w] = true
			}
		}
		return words
	}
	jaccard := func(a, b map[string]bool) float64 {
		if len(a) == 0 || len(b) == 0 {
			return 0
		}
		intersection := 0
		for w := range a {
			if b[w] {
				intersection++
			}
		}
		union := len(a) + len(b) - intersection
		if union == 0 {
			return 0
		}
		return float64(intersection) / float64(union)
	}

	type group struct {
		keepIdx int
		dupes   []int
	}

	tokens := make([]map[string]bool, len(todos))
	for i, t := range todos {
		tokens[i] = tokenize(t.Action)
	}

	merged := map[int]bool{}
	var groups []group

	for i := 0; i < len(todos); i++ {
		if merged[i] {
			continue
		}
		g := group{keepIdx: i}
		for j := i + 1; j < len(todos); j++ {
			if merged[j] {
				continue
			}
			// Same dimension and high textual similarity
			if todos[i].Dimension == todos[j].Dimension && jaccard(tokens[i], tokens[j]) > 0.6 {
				g.dupes = append(g.dupes, j)
				merged[j] = true
			}
		}
		if len(g.dupes) > 0 {
			groups = append(groups, g)
		}
	}

	if len(groups) == 0 {
		return
	}

	// Archive duplicates, keeping the highest-priority version
	priorityRank := map[string]int{"high": 3, "medium": 2, "low": 1}
	for _, g := range groups {
		bestIdx := g.keepIdx
		bestRank := priorityRank[todos[bestIdx].Priority]
		for _, di := range g.dupes {
			if priorityRank[todos[di].Priority] > bestRank {
				bestRank = priorityRank[todos[di].Priority]
				bestIdx = di
			}
		}

		// Archive all dupes except the best
		for _, idx := range append([]int{g.keepIdx}, g.dupes...) {
			if idx == bestIdx {
				continue
			}
			_, err := mongoDB.Todos().UpdateOne(ctx,
				bson.D{{Key: "_id", Value: todos[idx].ID}},
				bson.D{{Key: "$set", Value: bson.D{
					{Key: "status", Value: "archived"},
					{Key: "archivedReason", Value: fmt.Sprintf("Merged: similar to existing todo")},
				}}},
			)
			if err != nil {
				log.Printf("deduplicateTodos: archive error: %v", err)
			}
		}
	}

	totalArchived := 0
	for _, g := range groups {
		totalArchived += len(g.dupes)
	}
	if totalArchived > 0 {
		log.Printf("deduplicateTodos: archived %d duplicate todos for %s", totalArchived, domain)
	}
}

func handleListTodos(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{})
		if status := r.URL.Query().Get("status"); status != "" {
			filter = append(filter, bson.E{Key: "status", Value: status})
		}
		if optID := r.URL.Query().Get("optimization_id"); optID != "" {
			oid, err := primitive.ObjectIDFromHex(optID)
			if err == nil {
				filter = append(filter, bson.E{Key: "optimizationId", Value: oid})
			}
		}
		if sourceType := r.URL.Query().Get("source_type"); sourceType != "" {
			filter = append(filter, bson.E{Key: "sourceType", Value: sourceType})
		}

		opts := options.Find().
			SetSort(bson.D{
				{Key: "createdAt", Value: -1},
			}).
			SetLimit(200)

		cursor, err := mongoDB.Todos().Find(ctx, filter, opts)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)

		var todos []TodoItem
		if err := cursor.All(ctx, &todos); err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		if todos == nil {
			todos = []TodoItem{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(todos)
	}
}

func handleUpdateTodo(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		oid, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}

		var req struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Status != "todo" && req.Status != "completed" && req.Status != "backlogged" && req.Status != "archived" {
			http.Error(w, "Status must be 'todo', 'completed', 'backlogged', or 'archived'", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		now := time.Now()
		var update bson.D
		switch req.Status {
		case "completed":
			update = bson.D{
				{Key: "$set", Value: bson.D{
					{Key: "status", Value: req.Status},
					{Key: "completedAt", Value: now},
				}},
				{Key: "$unset", Value: bson.D{
					{Key: "backloggedAt", Value: ""},
					{Key: "archivedAt", Value: ""},
				}},
			}
		case "backlogged":
			update = bson.D{
				{Key: "$set", Value: bson.D{
					{Key: "status", Value: req.Status},
					{Key: "backloggedAt", Value: now},
				}},
				{Key: "$unset", Value: bson.D{
					{Key: "completedAt", Value: ""},
					{Key: "archivedAt", Value: ""},
				}},
			}
		case "archived":
			update = bson.D{
				{Key: "$set", Value: bson.D{
					{Key: "status", Value: req.Status},
					{Key: "archivedAt", Value: now},
				}},
				{Key: "$unset", Value: bson.D{
					{Key: "completedAt", Value: ""},
					{Key: "backloggedAt", Value: ""},
				}},
			}
		default:
			update = bson.D{
				{Key: "$set", Value: bson.D{
					{Key: "status", Value: req.Status},
				}},
				{Key: "$unset", Value: bson.D{
					{Key: "completedAt", Value: ""},
					{Key: "backloggedAt", Value: ""},
					{Key: "archivedAt", Value: ""},
				}},
			}
		}

		result, err := mongoDB.Todos().UpdateOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "_id", Value: oid}}), update)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		if result.MatchedCount == 0 {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func handleBulkArchiveTodos(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SourceType string `json:"source_type"` // "video" or "optimization"
			Domain     string `json:"domain"`
			Question   string `json:"question"` // for optimization: archive todos matching this question
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		now := time.Now()
		filter := tenantFilter(r.Context(), bson.D{
			{Key: "domain", Value: req.Domain},
			{Key: "status", Value: bson.D{{Key: "$in", Value: bson.A{"todo", "backlogged"}}}},
		})
		if req.SourceType == "video" {
			filter = append(filter, bson.E{Key: "sourceType", Value: "video"})
		} else if req.SourceType == "reddit" {
			filter = append(filter, bson.E{Key: "sourceType", Value: "reddit"})
		} else if req.SourceType == "optimization" && req.Question != "" {
			filter = append(filter, bson.E{Key: "question", Value: req.Question})
			// Optimization todos have empty sourceType for backwards compat
			filter = append(filter, bson.E{Key: "sourceType", Value: bson.D{{Key: "$ne", Value: "video"}}})
		}

		update := bson.D{
			{Key: "$set", Value: bson.D{
				{Key: "status", Value: "archived"},
				{Key: "archivedAt", Value: now},
			}},
			{Key: "$unset", Value: bson.D{
				{Key: "completedAt", Value: ""},
				{Key: "backloggedAt", Value: ""},
			}},
		}

		result, err := mongoDB.Todos().UpdateMany(ctx, filter, update)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"archived_count": result.ModifiedCount})
	}
}

// handleGetDomainShare returns the current share state for a domain.
func handleGetDomainShare(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		tenantID := saas.TenantIDFromContext(r.Context())

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		var ds DomainShare
		err := mongoDB.DomainShares().FindOne(ctx, bson.M{
			"tenantId": tenantID,
			"domain":   domain,
		}).Decode(&ds)
		if err == mongo.ErrNoDocuments {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"visibility": "private",
				"share_id":   "",
				"share_url":  "",
			})
			return
		}
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		shareURL := ""
		if ds.ShareID != "" {
			shareURL = "/share/" + ds.ShareID
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"visibility": ds.Visibility,
			"share_id":   ds.ShareID,
			"share_url":  shareURL,
		})
	}
}

// handleSetDomainShare sets the sharing visibility for a domain.
func handleSetDomainShare(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		tenantID := saas.TenantIDFromContext(r.Context())

		if !isShareAdmin(r.Context()) {
			http.Error(w, `{"error":"must be owner or admin to share"}`, http.StatusForbidden)
			return
		}

		var req struct {
			Visibility string `json:"visibility"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}

		if req.Visibility != "private" && req.Visibility != "public" && req.Visibility != "popular" {
			http.Error(w, `{"error":"visibility must be private, public, or popular"}`, http.StatusBadRequest)
			return
		}

		if req.Visibility == "popular" && !isRootShareAdmin(r.Context()) {
			http.Error(w, `{"error":"only root tenant admins can mark domains as popular"}`, http.StatusForbidden)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Verify the tenant actually has data for this domain
		count, err := mongoDB.Analyses().CountDocuments(ctx, bson.M{"tenantId": tenantID, "domain": domain})
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		if count == 0 {
			// Also check optimizations and brand profiles
			count, _ = mongoDB.Optimizations().CountDocuments(ctx, bson.M{"tenantId": tenantID, "domain": domain})
			if count == 0 {
				count, _ = mongoDB.BrandProfiles().CountDocuments(ctx, bson.M{"tenantId": tenantID, "domain": domain})
			}
		}
		if count == 0 {
			http.Error(w, `{"error":"no data found for this domain"}`, http.StatusNotFound)
			return
		}

		now := time.Now()
		shareID := ""
		if req.Visibility == "public" || req.Visibility == "popular" {
			shareID = generateShareID()
		}

		filter := bson.M{"tenantId": tenantID, "domain": domain}
		update := bson.M{
			"$set": bson.M{
				"visibility": req.Visibility,
				"shareId":    shareID,
				"updatedAt":  now,
			},
			"$setOnInsert": bson.M{
				"tenantId":  tenantID,
				"domain":    domain,
				"createdAt": now,
			},
		}
		opts := options.Update().SetUpsert(true)
		_, err = mongoDB.DomainShares().UpdateOne(ctx, filter, update, opts)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		// Capture screenshot when domain is marked as popular
		if req.Visibility == "popular" {
			go captureBrandScreenshot(mongoDB, domain)
		}

		shareURL := ""
		if shareID != "" {
			shareURL = "/share/" + shareID
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"visibility": req.Visibility,
			"share_id":   shareID,
			"share_url":  shareURL,
		})
	}
}

// handleGetSharedDomain returns all domain data for a public/popular share link.
func handleGetSharedDomain(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		shareID := r.PathValue("shareId")

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		var ds DomainShare
		err := mongoDB.DomainShares().FindOne(ctx, bson.M{
			"shareId":    shareID,
			"visibility": bson.M{"$in": []string{"public", "popular"}},
		}).Decode(&ds)
		if err == mongo.ErrNoDocuments {
			http.Error(w, `{"error":"share not found"}`, http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		tenantDomain := bson.M{"tenantId": ds.TenantID, "domain": ds.Domain}

		// Fetch analyses (limit 20, newest first)
		var analyses []Analysis
		analysisCur, err := mongoDB.Analyses().Find(ctx, tenantDomain,
			options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(20))
		if err == nil {
			analysisCur.All(ctx, &analyses)
			analysisCur.Close(ctx)
		}
		// Strip rawText
		for i := range analyses {
			analyses[i].RawText = ""
		}

		// Fetch optimizations (limit 50, newest first)
		var optimizations []Optimization
		optCur, err := mongoDB.Optimizations().Find(ctx, tenantDomain,
			options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(50))
		if err == nil {
			optCur.All(ctx, &optimizations)
			optCur.Close(ctx)
		}
		// Strip rawText
		for i := range optimizations {
			optimizations[i].RawText = ""
		}

		// Fetch brand profile
		var brandProfile *BrandProfile
		var bp BrandProfile
		if err := mongoDB.BrandProfiles().FindOne(ctx, tenantDomain).Decode(&bp); err == nil {
			brandProfile = &bp
		}

		// Fetch video analysis
		var videoAnalysis *VideoAnalysis
		var va VideoAnalysis
		if err := mongoDB.VideoAnalyses().FindOne(ctx, tenantDomain).Decode(&va); err == nil {
			va.RawText = ""
			videoAnalysis = &va
		}

		// Fetch todos (status=todo, limit 100)
		var todos []TodoItem
		todoFilter := bson.M{"tenantId": ds.TenantID, "domain": ds.Domain, "status": "todo"}
		todoCur, err := mongoDB.Todos().Find(ctx, todoFilter,
			options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(100))
		if err == nil {
			todoCur.All(ctx, &todos)
			todoCur.Close(ctx)
		}

		// Fetch domain summary
		var domainSummary *DomainSummary
		var dsm DomainSummary
		if err := mongoDB.DomainSummaries().FindOne(ctx, tenantDomain).Decode(&dsm); err == nil {
			dsm.RawText = ""
			domainSummary = &dsm
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"domain":         ds.Domain,
			"visibility":     ds.Visibility,
			"analyses":       analyses,
			"optimizations":  optimizations,
			"brand_profile":  brandProfile,
			"video_analysis": videoAnalysis,
			"todos":          todos,
			"domain_summary": domainSummary,
		})
	}
}

// handleGetPopularDomains returns all domains marked as "popular".
func handleGetPopularDomains(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		cursor, err := mongoDB.DomainShares().Find(ctx, bson.M{"visibility": "popular"})
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)

		var shares []DomainShare
		if err := cursor.All(ctx, &shares); err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		type PopularDomain struct {
			Domain        string `json:"domain"`
			BrandName     string `json:"brand_name"`
			ShareID       string `json:"share_id"`
			AvgScore      int    `json:"avg_score"`
			ReportCount   int    `json:"report_count"`
			AnalysisCount int    `json:"analysis_count"`
			HasVideo      bool   `json:"has_video"`
			HasScreenshot bool   `json:"has_screenshot"`
		}

		results := make([]PopularDomain, 0, len(shares))
		for _, s := range shares {
			pd := PopularDomain{
				Domain:  s.Domain,
				ShareID: s.ShareID,
			}

			// Get brand name
			var bp BrandProfile
			if err := mongoDB.BrandProfiles().FindOne(ctx, bson.M{"tenantId": s.TenantID, "domain": s.Domain}).Decode(&bp); err == nil {
				pd.BrandName = bp.BrandName
			}

			tdFilter := bson.M{"tenantId": s.TenantID, "domain": s.Domain}

			// Count analyses
			analysisCount, _ := mongoDB.Analyses().CountDocuments(ctx, tdFilter)
			pd.AnalysisCount = int(analysisCount)

			// Get optimization count and avg score
			optCur, err := mongoDB.Optimizations().Find(ctx, tdFilter,
				options.Find().SetProjection(bson.M{"result.overallScore": 1}))
			if err == nil {
				var opts []Optimization
				optCur.All(ctx, &opts)
				optCur.Close(ctx)
				if len(opts) > 0 {
					total := 0
					for _, o := range opts {
						total += o.Result.OverallScore
					}
					pd.AvgScore = total / len(opts)
				}
				pd.ReportCount = len(opts)
			}

			// Use analysis count as report count if no optimizations
			if pd.ReportCount == 0 {
				pd.ReportCount = pd.AnalysisCount
			}

			// Check for video analysis
			videoCount, _ := mongoDB.VideoAnalyses().CountDocuments(ctx, tdFilter)
			pd.HasVideo = videoCount > 0

			// Check for screenshot
			ssCount, _ := mongoDB.BrandScreenshots().CountDocuments(ctx, bson.M{
				"domain": s.Domain, "sizeBytes": bson.M{"$gt": 0},
			})
			pd.HasScreenshot = ssCount > 0

			results = append(results, pd)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

// handleServeBrandScreenshot serves a cached screenshot JPEG for a popular domain.
func handleServeBrandScreenshot(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		var ss BrandScreenshot
		filter := bson.M{"domain": domain, "sizeBytes": bson.M{"$gt": 0}}
		if err := mongoDB.BrandScreenshots().FindOne(ctx, filter).Decode(&ss); err != nil {
			if err == mongo.ErrNoDocuments {
				http.Error(w, "Screenshot not found", http.StatusNotFound)
			} else {
				http.Error(w, "Database error", http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", ss.ContentType)
		w.Header().Set("Content-Length", strconv.Itoa(len(ss.ImageData)))
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("ETag", fmt.Sprintf(`"%s-%d"`, domain, ss.CapturedAt.Unix()))
		w.Write(ss.ImageData)
	}
}

// Brand Intelligence handlers

func handleListBrands(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		opts := options.Find().
			SetSort(bson.D{{Key: "updatedAt", Value: -1}}).
			SetLimit(50)

		cursor, err := mongoDB.BrandProfiles().Find(ctx, tenantFilter(r.Context(), bson.D{}), opts)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)

		var profiles []BrandProfile
		if err := cursor.All(ctx, &profiles); err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		summaries := make([]BrandProfileSummary, len(profiles))
		for i, p := range profiles {
			summaries[i] = BrandProfileSummary{
				ID:              p.ID,
				Domain:          p.Domain,
				BrandName:       p.BrandName,
				CompetitorCount: len(p.Competitors),
				QueryCount:      len(p.TargetQueries),
				Completeness:    computeBrandCompleteness(p),
				Public:          p.Public,
				UpdatedAt:       p.UpdatedAt,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(summaries)
	}
}

func handleGetBrand(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		var profile BrandProfile
		err := mongoDB.BrandProfiles().FindOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})).Decode(&profile)
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(profile)
	}
}

func handleSaveBrand(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))

		var req BrandProfile
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		now := time.Now()
		tid := saas.TenantIDFromContext(r.Context())
		update := bson.D{
			{Key: "$set", Value: bson.D{
				{Key: "domain", Value: domain},
				{Key: "brandName", Value: req.BrandName},
				{Key: "description", Value: req.Description},
				{Key: "categories", Value: req.Categories},
				{Key: "products", Value: req.Products},
				{Key: "primaryAudience", Value: req.PrimaryAudience},
				{Key: "keyUseCases", Value: req.KeyUseCases},
				{Key: "competitors", Value: req.Competitors},
				{Key: "targetQueries", Value: req.TargetQueries},
				{Key: "keyMessages", Value: req.KeyMessages},
				{Key: "differentiators", Value: req.Differentiators},
				{Key: "presence", Value: req.Presence},
				{Key: "presenceComplete", Value: req.PresenceComplete},
				{Key: "public", Value: req.Public},
				{Key: "updatedAt", Value: now},
			}},
			{Key: "$setOnInsert", Value: bson.D{
				{Key: "createdAt", Value: now},
				{Key: "tenantId", Value: tid},
			}},
		}

		brandFilter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		opts := options.Update().SetUpsert(true)
		result, err := mongoDB.BrandProfiles().UpdateOne(ctx, brandFilter, update, opts)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		// Fetch the saved profile to return it
		var saved BrandProfile
		err = mongoDB.BrandProfiles().FindOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})).Decode(&saved)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if result.UpsertedCount > 0 {
			w.WriteHeader(http.StatusCreated)
		}
		json.NewEncoder(w).Encode(saved)
	}
}

func handleDeleteBrand(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		result, err := mongoDB.BrandProfiles().DeleteOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}}))
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		if result.DeletedCount == 0 {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func handleDeleteAnalysis(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		oid, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		// Find all optimizations for this analysis to cascade-delete their todos
		cursor, err := mongoDB.Optimizations().Find(ctx, bson.D{{Key: "analysisId", Value: oid}}, options.Find().SetProjection(bson.D{{Key: "_id", Value: 1}}))
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		var optIDs []primitive.ObjectID
		for cursor.Next(ctx) {
			var doc struct {
				ID primitive.ObjectID `bson:"_id"`
			}
			if cursor.Decode(&doc) == nil {
				optIDs = append(optIDs, doc.ID)
			}
		}

		// Delete todos for all those optimizations
		if len(optIDs) > 0 {
			mongoDB.Todos().DeleteMany(ctx, bson.D{{Key: "optimizationId", Value: bson.D{{Key: "$in", Value: optIDs}}}})
		}

		// Delete all optimizations for this analysis
		mongoDB.Optimizations().DeleteMany(ctx, bson.D{{Key: "analysisId", Value: oid}})

		// Delete the analysis itself
		result, err := mongoDB.Analyses().DeleteOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "_id", Value: oid}}))
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		if result.DeletedCount == 0 {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func handleDeleteOptimization(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		oid, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Delete all todos for this optimization
		mongoDB.Todos().DeleteMany(ctx, bson.D{{Key: "optimizationId", Value: oid}})

		// Delete the optimization itself
		result, err := mongoDB.Optimizations().DeleteOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "_id", Value: oid}}))
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		if result.DeletedCount == 0 {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// Domain Summary handlers

func buildDomainSummaryPrompt(domain string, optimizations []Optimization, analysis *Analysis, videoAnalysis *VideoAnalysis, redditAnalysis *RedditAnalysis, brandInfo BrandContextInfo) string {
	var sections strings.Builder

	// Site Analysis (summary only)
	if analysis != nil {
		sections.WriteString("\n=== SITE ANALYSIS ===\n")
		if analysis.Result.SiteSummary != "" {
			sections.WriteString("Site Summary: " + analysis.Result.SiteSummary + "\n")
		}
		sections.WriteString(fmt.Sprintf("Pages Crawled: %d\n", len(analysis.Result.CrawledPages)))
		sections.WriteString(fmt.Sprintf("Questions Discovered: %d\n", len(analysis.Result.Questions)))
	}

	// Optimization Reports (scores only)
	if len(optimizations) > 0 {
		sections.WriteString(fmt.Sprintf("\n=== OPTIMIZATION REPORTS (%d) ===\n", len(optimizations)))
		for i, opt := range optimizations {
			sections.WriteString(fmt.Sprintf("- Report %d: \"%s\" — Overall: %d, Content Authority: %d, Structural: %d, Source Authority: %d, Knowledge Persistence: %d\n",
				i+1, opt.Question, opt.Result.OverallScore,
				opt.Result.ContentAuthority.Score, opt.Result.StructuralOptimization.Score,
				opt.Result.SourceAuthority.Score, opt.Result.KnowledgePersistence.Score))
		}
	}

	// Video Authority (executive summary + score only)
	if videoAnalysis != nil && videoAnalysis.Result != nil {
		vr := videoAnalysis.Result
		sections.WriteString("\n=== YOUTUBE VIDEO AUTHORITY ===\n")
		sections.WriteString(fmt.Sprintf("Overall Video Authority Score: %d/100\n", vr.OverallScore))
		if vr.ExecutiveSummary != "" {
			sections.WriteString("Summary: " + vr.ExecutiveSummary + "\n")
		}
	}

	// Reddit Authority (executive summary + score only)
	if redditAnalysis != nil && redditAnalysis.Result != nil {
		rr := redditAnalysis.Result
		sections.WriteString("\n=== REDDIT AUTHORITY ===\n")
		sections.WriteString(fmt.Sprintf("Overall Reddit Authority Score: %d/100\n", rr.OverallScore))
		if rr.ExecutiveSummary != "" {
			sections.WriteString("Summary: " + rr.ExecutiveSummary + "\n")
		}
	}

	brandSection := ""
	if brandInfo.Used && brandInfo.ContextString != "" {
		brandSection = fmt.Sprintf("\n--- Brand Intelligence Context ---\n%s\n--- End Brand Context ---\n", brandInfo.ContextString)
	}

	// Build inventory of what's included
	var included []string
	if analysis != nil {
		included = append(included, "Site Analysis")
	}
	if len(optimizations) > 0 {
		included = append(included, fmt.Sprintf("%d Optimization Reports", len(optimizations)))
	}
	if videoAnalysis != nil && videoAnalysis.Result != nil {
		included = append(included, "YouTube Video Authority")
	}
	if redditAnalysis != nil && redditAnalysis.Result != nil {
		included = append(included, "Reddit Authority")
	}

	return fmt.Sprintf(`You are an LLM visibility strategist. You are given high-level summaries and scores from multiple report types for a single domain. Synthesize these into a unified strategic overview.

Domain: %s
Reports Included: %s
%s%s
INSTRUCTIONS:

1. **Executive Summary**: Write a 2-3 paragraph strategic overview of this domain's LLM visibility position across all available channels. Cover the biggest strengths, weaknesses, and overall trajectory. Weave together findings from all report types present.

2. **Themes**: Identify 3-5 recurring patterns across the reports. Reference sources by label (e.g., "Optimization Report 3", "Video Authority", "Reddit Authority", "Site Analysis"). Themes should span report types when possible.

3. **Priority Action Items**: Based on the score patterns and summaries, recommend 5-10 prioritized actions. Use priority levels: high, medium, low.

4. **Contradictions**: If different report summaries give conflicting signals, surface those explicitly with a recommendation on how to reconcile. If none, return an empty array.

5. **Dimension Trends**: Calculate the average score (0-100) for each optimization dimension across optimization reports. If no optimization reports exist, omit or use 0.

Return as JSON (no markdown code fences, just raw JSON):
{
  "executive_summary": "2-3 paragraph strategic overview covering all report types",
  "average_score": 65,
  "score_range": [40, 85],
  "themes": [
    {"title": "Theme name", "description": "What this means and why it matters", "report_refs": ["Optimization Report 1", "Video Authority"]}
  ],
  "action_items": [
    {"priority": "high", "action": "Specific action", "dimension": "content_authority", "expected_impact": "Expected improvement", "source_reports": ["Optimization Report 1", "Reddit Authority"]}
  ],
  "contradictions": [
    {"topic": "What is contradicted", "positions": ["Optimization reports say X", "Reddit analysis says Y"], "report_refs": ["Optimization Report 1", "Reddit Authority"], "recommendation": "How to reconcile"}
  ],
  "dimension_trends": {"content_authority": 60, "structural_optimization": 55, "source_authority": 70, "knowledge_persistence": 50}
}

If there are no contradictions, return an empty array for contradictions. Be specific and actionable.`, domain, strings.Join(included, ", "), sections.String(), brandSection)
}

// isSummaryStale checks if any data source has new data since the summary was generated.
func isSummaryStale(ctx context.Context, mongoDB *MongoDB, tenantCtx context.Context, domain string, summary DomainSummary) (bool, int64) {
	domainFilter := tenantFilter(tenantCtx, bson.D{{Key: "domain", Value: domain}})

	// Newer optimizations
	newerOpts, _ := mongoDB.Optimizations().CountDocuments(ctx, tenantFilter(tenantCtx, bson.D{
		{Key: "domain", Value: domain},
		{Key: "createdAt", Value: bson.D{{Key: "$gt", Value: summary.GeneratedAt}}},
	}))
	if newerOpts > 0 {
		return true, newerOpts
	}

	// Newer site analysis
	newerAnalysis, _ := mongoDB.Analyses().CountDocuments(ctx, append(domainFilter, bson.E{Key: "createdAt", Value: bson.D{{Key: "$gt", Value: summary.GeneratedAt}}}))
	if newerAnalysis > 0 {
		return true, newerAnalysis
	}

	// New video analysis that wasn't included
	if !summary.IncludesVideo {
		var va struct{ ID primitive.ObjectID `bson:"_id"` }
		if err := mongoDB.VideoAnalyses().FindOne(ctx, domainFilter, options.FindOne().SetProjection(bson.D{{Key: "_id", Value: 1}})).Decode(&va); err == nil {
			return true, 0
		}
	} else {
		newerVideo, _ := mongoDB.VideoAnalyses().CountDocuments(ctx, append(domainFilter, bson.E{Key: "generatedAt", Value: bson.D{{Key: "$gt", Value: summary.GeneratedAt}}}))
		if newerVideo > 0 {
			return true, 0
		}
	}

	// New Reddit analysis that wasn't included
	if !summary.IncludesReddit {
		var ra struct{ ID primitive.ObjectID `bson:"_id"` }
		if err := mongoDB.RedditAnalyses().FindOne(ctx, domainFilter, options.FindOne().SetProjection(bson.D{{Key: "_id", Value: 1}})).Decode(&ra); err == nil {
			return true, 0
		}
	} else {
		newerReddit, _ := mongoDB.RedditAnalyses().CountDocuments(ctx, append(domainFilter, bson.E{Key: "generatedAt", Value: bson.D{{Key: "$gt", Value: summary.GeneratedAt}}}))
		if newerReddit > 0 {
			return true, 0
		}
	}

	return false, 0
}

func handleDomainSummaryStatus(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		domainFilter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		var summary DomainSummary
		err := mongoDB.DomainSummaries().FindOne(ctx, domainFilter).Decode(&summary)

		if err == mongo.ErrNoDocuments {
			count, _ := mongoDB.Optimizations().CountDocuments(ctx, domainFilter)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"exists":             false,
				"total_report_count": count,
			})
			return
		}
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		totalCount, _ := mongoDB.Optimizations().CountDocuments(ctx, domainFilter)
		stale, newerCount := isSummaryStale(ctx, mongoDB, r.Context(), domain, summary)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"exists":             true,
			"generated_at":       summary.GeneratedAt,
			"included_count":     summary.ReportCount,
			"includes_analysis":  summary.IncludesAnalysis,
			"includes_video":     summary.IncludesVideo,
			"includes_reddit":    summary.IncludesReddit,
			"total_report_count": totalCount,
			"newer_report_count": newerCount,
			"stale":              stale,
		})
	}
}

func handleGetDomainSummary(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		domainFilter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		var summary DomainSummary
		err := mongoDB.DomainSummaries().FindOne(ctx, domainFilter).Decode(&summary)
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		stale, newerCount := isSummaryStale(ctx, mongoDB, r.Context(), domain, summary)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"summary":            summary,
			"stale":              stale,
			"newer_report_count": newerCount,
		})
	}
}

func handleGenerateDomainSummary(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		// Resolve primary LLM provider and API key for this tenant
		provider, apiKey, _, err := resolvePrimaryLLM(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Configure an API key in Settings", "code": "api_key_required"})
			return
		}
		_ = apiKey // used by provider.Stream below

		// Load all optimizations for this domain (max 30)
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		cursor, err := mongoDB.Optimizations().Find(ctx, tenantFilter(r.Context(), bson.D{
			{Key: "domain", Value: domain},
		}), options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(30))
		cancel()
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Failed to load optimizations"})
			return
		}
		var optimizations []Optimization
		ctx2, cancel2 := context.WithTimeout(r.Context(), 15*time.Second)
		if err := cursor.All(ctx2, &optimizations); err != nil {
			cancel2()
			sendSSE(w, flusher, "error", map[string]string{"message": "Failed to read optimizations"})
			return
		}
		cancel2()

		// Load site analysis
		var analysis *Analysis
		var an Analysis
		anCtx, anCancel := context.WithTimeout(r.Context(), 10*time.Second)
		anFilter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		anOpts := options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetProjection(bson.D{{Key: "rawText", Value: 0}})
		if err := mongoDB.Analyses().FindOne(anCtx, anFilter, anOpts).Decode(&an); err == nil {
			analysis = &an
		}
		anCancel()

		// Load video analysis
		var videoAnalysis *VideoAnalysis
		var va VideoAnalysis
		vaCtx, vaCancel := context.WithTimeout(r.Context(), 10*time.Second)
		vaFilter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		vaOpts := options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "rawText", Value: 0}})
		if err := mongoDB.VideoAnalyses().FindOne(vaCtx, vaFilter, vaOpts).Decode(&va); err == nil {
			videoAnalysis = &va
		}
		vaCancel()

		// Load Reddit analysis
		var redditAnalysis *RedditAnalysis
		var ra RedditAnalysis
		raCtx, raCancel := context.WithTimeout(r.Context(), 10*time.Second)
		raFilter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		raOpts := options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "rawText", Value: 0}})
		if err := mongoDB.RedditAnalyses().FindOne(raCtx, raFilter, raOpts).Decode(&ra); err == nil {
			redditAnalysis = &ra
		}
		raCancel()

		hasVideo := videoAnalysis != nil && videoAnalysis.Result != nil
		hasReddit := redditAnalysis != nil && redditAnalysis.Result != nil

		if len(optimizations) == 0 && analysis == nil && !hasVideo && !hasReddit {
			sendSSE(w, flusher, "error", map[string]string{"message": "No reports found for this domain"})
			return
		}

		brandInfo := lookupBrandContext(mongoDB, domain, saas.TenantIDFromContext(r.Context()))

		// Build status message
		var parts []string
		if len(optimizations) > 0 {
			parts = append(parts, fmt.Sprintf("%d optimization reports", len(optimizations)))
		}
		if analysis != nil {
			parts = append(parts, "site analysis")
		}
		if hasVideo {
			parts = append(parts, "video authority")
		}
		if hasReddit {
			parts = append(parts, "Reddit authority")
		}
		sendSSE(w, flusher, "status", map[string]string{
			"message": fmt.Sprintf("Synthesizing %s for %s...", strings.Join(parts, ", "), domain),
		})

		prompt := buildDomainSummaryPrompt(domain, optimizations, analysis, videoAnalysis, redditAnalysis, brandInfo)

		models := provider.Models()

		for mi, model := range models {
			if mi > 0 {
				sendSSE(w, flusher, "status", map[string]string{
					"message": fmt.Sprintf("%s unavailable, falling back to %s...", models[mi-1].Name, model.Name),
				})
			}

			// No tools needed — pure synthesis of existing data
			claudeBody, _ := provider.BuildStreamBody(model.ID, 8192, prompt, false)

			const maxRetries = 3
			backoff := 2 * time.Second
			var lastErr error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					sendSSE(w, flusher, "status", map[string]string{
						"message": fmt.Sprintf("%s overloaded, retrying in %ds (attempt %d/%d)...", model.Name, int(backoff.Seconds()), attempt, maxRetries),
					})
					select {
					case <-time.After(backoff):
					case <-r.Context().Done():
						return
					}
					backoff *= 2
				}

				result, err := provider.Stream(r.Context(), apiKey, claudeBody, w, flusher)
				if err == errOverloaded {
					lastErr = err
					if attempt < maxRetries {
						continue
					}
					break
				}
				if err != nil {
					sendSSE(w, flusher, "error", map[string]string{"message": err.Error()})
					return
				}

				// Parse and save the summary
				cleanJSON := stripJSONFencing(result.ResultJSON)
				var summaryResult DomainSummaryResult
				if err := json.Unmarshal([]byte(cleanJSON), &summaryResult); err != nil {
					sendSSE(w, flusher, "error", map[string]string{"message": "Failed to parse summary results"})
					return
				}

				optIDs := make([]primitive.ObjectID, len(optimizations))
				for i, o := range optimizations {
					optIDs[i] = o.ID
				}

				summary := DomainSummary{
					Domain:           domain,
					TenantID:         saas.TenantIDFromContext(r.Context()),
					Result:           summaryResult,
					RawText:          result.RawText,
					Model:            model.Name,
					OptimizationIDs:  optIDs,
					ReportCount:      len(optimizations),
					IncludesAnalysis: analysis != nil,
					IncludesVideo:    hasVideo,
					IncludesReddit:   hasReddit,
					GeneratedAt:      time.Now(),
				}

				saveCtx, saveCancel := context.WithTimeout(r.Context(), 10*time.Second)
				_, saveErr := mongoDB.DomainSummaries().ReplaceOne(saveCtx,
					tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}}),
					summary,
					options.Replace().SetUpsert(true),
				)
				saveCancel()
				if saveErr != nil {
					log.Printf("Failed to save domain summary: %v", saveErr)
				}

				sendSSE(w, flusher, "done", map[string]any{
					"result":            result.ResultJSON,
					"model":             model.Name,
					"generated_at":      summary.GeneratedAt,
					"report_count":      summary.ReportCount,
					"includes_analysis": summary.IncludesAnalysis,
					"includes_video":    summary.IncludesVideo,
					"includes_reddit":   summary.IncludesReddit,
					"domain":            domain,
				})
				return
			}

			log.Printf("%s API (%s) exhausted retries for domain summary: %v", provider.Name(), model.ID, lastErr)
		}

		sendSSE(w, flusher, "error", map[string]string{
			"message": "All models are currently overloaded. Please try again later.",
		})
	}
}

func handleDiscoverCompetitors(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := r.PathValue("domain")

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		provider, apiKey, _, err := resolvePrimaryLLM(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Configure an API key in Settings", "code": "api_key_required"})
			return
		}
		_ = apiKey

		// Load existing brand profile for context (optional)
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		var brand BrandProfile
		brandErr := mongoDB.BrandProfiles().FindOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})).Decode(&brand)
		cancel()

		brandContext := ""
		if brandErr == nil && brand.BrandName != "" {
			brandContext = fmt.Sprintf("\nKnown brand name: %s\nDescription: %s\nCategories: %s\n",
				brand.BrandName, brand.Description, strings.Join(brand.Categories, ", "))
		}

		sendSSE(w, flusher, "status", map[string]string{
			"message": "Discovering competitors for " + domain + "...",
		})

		prompt := fmt.Sprintf(`You are a competitive intelligence analyst. Your task is to discover competitors for a given website/brand.

Website domain: %s%s

Follow this discovery process:

1. **Site Analysis**: Search for and visit %s to understand what they do, their product category, and market positioning.

2. **Search-Based Discovery**: Run these types of searches:
   - "[product category] alternatives"
   - "[brand name] vs"
   - "best [category] tools/software/services"
   - "[brand name] competitors"
   - Look at results from G2, Capterra, TrustRadius, and similar review/comparison sites

3. **LLM Knowledge Probe**: Based on your own knowledge, who are the main competitors? This reveals what's already in LLM training data.

4. **Cross-Reference**: For each competitor found, note where you found them (search results, review sites, or your own knowledge).

Identify 5-15 competitors. For each, determine their relationship to the target brand.

Return your findings as JSON (no markdown code fences, just raw JSON):
{
  "competitors": [
    {
      "name": "Competitor Name",
      "url": "https://competitor.com",
      "relationship": "direct",
      "source": "search",
      "confidence": 0.9,
      "notes": "Brief note about why they're a competitor"
    }
  ]
}

- relationship: "direct" (same category), "indirect" (overlapping use cases), "aspirational" (larger/successful peer), or "adjacent" (complementary product)
- source: "search" (found in web search results), "review_site" (found on G2/Capterra/etc), "llm_knowledge" (from your training data), or "multiple" (found in multiple places)
- confidence: 0.0-1.0 how confident you are this is a real competitor`, domain, brandContext, domain)

		models := provider.Models()

		for mi, model := range models {
			if mi > 0 {
				sendSSE(w, flusher, "status", map[string]string{
					"message": fmt.Sprintf("%s unavailable, falling back to %s...", models[mi-1].Name, model.Name),
				})
			}

			claudeBody, _ := provider.BuildStreamBody(model.ID, 16384, prompt, true)

			const maxRetries = 3
			backoff := 2 * time.Second
			var lastErr error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					sendSSE(w, flusher, "status", map[string]string{
						"message": fmt.Sprintf("%s overloaded, retrying in %ds (attempt %d/%d)...", model.Name, int(backoff.Seconds()), attempt, maxRetries),
					})
					select {
					case <-time.After(backoff):
					case <-r.Context().Done():
						return
					}
					backoff *= 2
				}

				result, err := provider.Stream(r.Context(), apiKey, claudeBody, w, flusher)
				if err == errOverloaded {
					lastErr = err
					if attempt < maxRetries {
						continue
					}
					break
				}
				if err != nil {
					sendSSE(w, flusher, "error", map[string]string{"message": err.Error()})
					return
				}

				// Send results (not saved to DB — user reviews first)
				sendSSE(w, flusher, "done", map[string]any{
					"result": result.ResultJSON,
					"model":  model.Name,
				})
				return
			}

			log.Printf("%s API (%s) exhausted retries for competitor discovery: %v", provider.Name(), model.ID, lastErr)
		}

		sendSSE(w, flusher, "error", map[string]string{
			"message": "All models are currently overloaded. Please try again later.",
		})
	}
}

func handleSuggestQueries(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := r.PathValue("domain")

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		provider, apiKey, _, err := resolvePrimaryLLM(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Configure an API key in Settings", "code": "api_key_required"})
			return
		}
		_ = apiKey

		// Load brand profile for context
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		var brand BrandProfile
		brandErr := mongoDB.BrandProfiles().FindOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})).Decode(&brand)
		cancel()

		if brandErr != nil {
			sendSSE(w, flusher, "error", map[string]string{
				"message": "Brand profile not found. Please save basic brand info first.",
			})
			return
		}

		sendSSE(w, flusher, "status", map[string]string{
			"message": "Generating query suggestions for " + brand.BrandName + "...",
		})

		competitorNames := make([]string, len(brand.Competitors))
		for i, c := range brand.Competitors {
			competitorNames[i] = c.Name
		}

		prompt := fmt.Sprintf(`You are an LLM visibility strategist. Generate target search queries that a brand should optimize for in LLM responses.

Brand: %s
Website: %s
Description: %s
Categories: %s
Products/Features: %s
Target Audience: %s
Key Use Cases: %s
Known Competitors: %s

Generate 20-30 target queries organized by type. These are queries where the brand wants to appear in LLM-generated responses.

Query types:
- **brand**: Queries that include the brand name (e.g., "[brand] review", "is [brand] good?", "[brand] pricing")
- **category**: Generic queries about the product category (e.g., "best [category] tools", "top [category] software 2025")
- **comparison**: Head-to-head queries (e.g., "[brand] vs [competitor]" for each major competitor)
- **problem**: Problem/need-oriented queries where the brand's product is the answer (e.g., "how to [use case]", "best way to [solve problem]")

For each query, assign a priority:
- **high**: Core business queries, high commercial intent
- **medium**: Important for visibility but less direct
- **low**: Nice to have, exploratory queries

Return as JSON (no markdown code fences, just raw JSON):
{
  "queries": [
    {
      "query": "The actual search query text",
      "priority": "high",
      "type": "category"
    }
  ]
}`, brand.BrandName, domain, brand.Description,
			strings.Join(brand.Categories, ", "),
			strings.Join(brand.Products, ", "),
			brand.PrimaryAudience,
			strings.Join(brand.KeyUseCases, ", "),
			strings.Join(competitorNames, ", "))

		models := provider.Models()

		for mi, model := range models {
			if mi > 0 {
				sendSSE(w, flusher, "status", map[string]string{
					"message": fmt.Sprintf("%s unavailable, falling back to %s...", models[mi-1].Name, model.Name),
				})
			}

			claudeBody, _ := provider.BuildStreamBody(model.ID, 16384, prompt, false)

			const maxRetries = 3
			backoff := 2 * time.Second
			var lastErr error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					sendSSE(w, flusher, "status", map[string]string{
						"message": fmt.Sprintf("%s overloaded, retrying in %ds (attempt %d/%d)...", model.Name, int(backoff.Seconds()), attempt, maxRetries),
					})
					select {
					case <-time.After(backoff):
					case <-r.Context().Done():
						return
					}
					backoff *= 2
				}

				result, err := provider.Stream(r.Context(), apiKey, claudeBody, w, flusher)
				if err == errOverloaded {
					lastErr = err
					if attempt < maxRetries {
						continue
					}
					break
				}
				if err != nil {
					sendSSE(w, flusher, "error", map[string]string{"message": err.Error()})
					return
				}

				sendSSE(w, flusher, "done", map[string]any{
					"result": result.ResultJSON,
					"model":  model.Name,
				})
				return
			}

			log.Printf("%s API (%s) exhausted retries for query suggestion: %v", provider.Name(), model.ID, lastErr)
		}

		sendSSE(w, flusher, "error", map[string]string{
			"message": "All models are currently overloaded. Please try again later.",
		})
	}
}

func handleGenerateDescription(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := r.PathValue("domain")

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		provider, apiKey, _, err := resolvePrimaryLLM(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Configure an API key in Settings", "code": "api_key_required"})
			return
		}
		_ = apiKey

		sendSSE(w, flusher, "status", map[string]string{
			"message": "Analyzing " + domain + " to generate description...",
		})

		prompt := fmt.Sprintf(`You are a brand analyst. Visit and analyze the website at %s to produce a concise brand description.

1. Search for and visit the homepage and key pages of %s
2. Understand what the company does, who it serves, and what makes it distinctive
3. Write a clear, factual 2-3 sentence description

Return as JSON (no markdown code fences, just raw JSON):
{
  "description": "The 2-3 sentence brand description",
  "brand_name": "The company/brand name as it appears on the site",
  "categories": ["category1", "category2"],
  "products": ["product1", "feature1"]
}`, domain, domain)

		models := provider.Models()

		for mi, model := range models {
			if mi > 0 {
				sendSSE(w, flusher, "status", map[string]string{
					"message": fmt.Sprintf("%s unavailable, falling back to %s...", models[mi-1].Name, model.Name),
				})
			}

			claudeBody, _ := provider.BuildStreamBody(model.ID, 4096, prompt, true)

			const maxRetries = 3
			backoff := 2 * time.Second
			var lastErr error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					sendSSE(w, flusher, "status", map[string]string{
						"message": fmt.Sprintf("%s overloaded, retrying in %ds (attempt %d/%d)...", model.Name, int(backoff.Seconds()), attempt, maxRetries),
					})
					select {
					case <-time.After(backoff):
					case <-r.Context().Done():
						return
					}
					backoff *= 2
				}

				result, err := provider.Stream(r.Context(), apiKey, claudeBody, w, flusher)
				if err == errOverloaded {
					lastErr = err
					if attempt < maxRetries {
						continue
					}
					break
				}
				if err != nil {
					sendSSE(w, flusher, "error", map[string]string{"message": err.Error()})
					return
				}

				sendSSE(w, flusher, "done", map[string]any{
					"result": result.ResultJSON,
					"model":  model.Name,
				})
				return
			}

			log.Printf("%s API (%s) exhausted retries for description generation: %v", provider.Name(), model.ID, lastErr)
		}

		sendSSE(w, flusher, "error", map[string]string{
			"message": "All models are currently overloaded. Please try again later.",
		})
	}
}

func handlePredictAudience(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := r.PathValue("domain")

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		provider, apiKey, _, err := resolvePrimaryLLM(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Configure an API key in Settings", "code": "api_key_required"})
			return
		}
		_ = apiKey

		// Load existing brand profile for context
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		var brand BrandProfile
		brandErr := mongoDB.BrandProfiles().FindOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})).Decode(&brand)
		cancel()

		brandContext := ""
		if brandErr == nil {
			if brand.BrandName != "" {
				brandContext += fmt.Sprintf("\nBrand: %s\n", brand.BrandName)
			}
			if brand.Description != "" {
				brandContext += fmt.Sprintf("Description: %s\n", brand.Description)
			}
			if len(brand.Categories) > 0 {
				brandContext += fmt.Sprintf("Categories: %s\n", strings.Join(brand.Categories, ", "))
			}
			if len(brand.Products) > 0 {
				brandContext += fmt.Sprintf("Products: %s\n", strings.Join(brand.Products, ", "))
			}
		}

		sendSSE(w, flusher, "status", map[string]string{
			"message": "Analyzing " + domain + " to predict target audience...",
		})

		prompt := fmt.Sprintf(`You are a brand strategist. Visit and analyze the website at %s to determine who the target audience is and what key use cases the product/service addresses.
%s
Steps:
1. Search for and visit the homepage and key pages of %s
2. Identify the primary target demographic — roles, industries, company sizes, or consumer segments
3. Identify specific use cases, problems solved, and jobs-to-be-done
4. Be specific rather than generic — mention actual roles, industries, or scenarios

Return as JSON (no markdown code fences, just raw JSON):
{
  "primary_audience": "A specific 2-3 sentence description of the primary target audience",
  "key_use_cases": ["specific use case 1", "specific use case 2", "..."]
}`, domain, brandContext, domain)

		models := provider.Models()

		for mi, model := range models {
			if mi > 0 {
				sendSSE(w, flusher, "status", map[string]string{
					"message": fmt.Sprintf("%s unavailable, falling back to %s...", models[mi-1].Name, model.Name),
				})
			}

			claudeBody, _ := provider.BuildStreamBody(model.ID, 4096, prompt, true)

			const maxRetries = 3
			backoff := 2 * time.Second
			var lastErr error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					sendSSE(w, flusher, "status", map[string]string{
						"message": fmt.Sprintf("%s overloaded, retrying in %ds (attempt %d/%d)...", model.Name, int(backoff.Seconds()), attempt, maxRetries),
					})
					select {
					case <-time.After(backoff):
					case <-r.Context().Done():
						return
					}
					backoff *= 2
				}

				result, err := provider.Stream(r.Context(), apiKey, claudeBody, w, flusher)
				if err == errOverloaded {
					lastErr = err
					if attempt < maxRetries {
						continue
					}
					break
				}
				if err != nil {
					sendSSE(w, flusher, "error", map[string]string{"message": err.Error()})
					return
				}

				sendSSE(w, flusher, "done", map[string]any{
					"result": result.ResultJSON,
					"model":  model.Name,
				})
				return
			}

			log.Printf("%s API (%s) exhausted retries for audience prediction: %v", provider.Name(), model.ID, lastErr)
		}

		sendSSE(w, flusher, "error", map[string]string{
			"message": "All models are currently overloaded. Please try again later.",
		})
	}
}

func handleSuggestClaims(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := r.PathValue("domain")

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		provider, apiKey, _, err := resolvePrimaryLLM(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Configure an API key in Settings", "code": "api_key_required"})
			return
		}
		_ = apiKey

		// Load existing brand profile for context
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		var brand BrandProfile
		brandErr := mongoDB.BrandProfiles().FindOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})).Decode(&brand)
		cancel()

		brandContext := ""
		if brandErr == nil {
			if brand.BrandName != "" {
				brandContext += fmt.Sprintf("\nBrand: %s\n", brand.BrandName)
			}
			if brand.Description != "" {
				brandContext += fmt.Sprintf("Description: %s\n", brand.Description)
			}
			if len(brand.Categories) > 0 {
				brandContext += fmt.Sprintf("Categories: %s\n", strings.Join(brand.Categories, ", "))
			}
			if len(brand.Products) > 0 {
				brandContext += fmt.Sprintf("Products: %s\n", strings.Join(brand.Products, ", "))
			}
		}

		sendSSE(w, flusher, "status", map[string]string{
			"message": "Searching " + domain + " for brand claims and proof points...",
		})

		prompt := fmt.Sprintf(`You are a brand analyst. Visit and analyze the website at %s to discover the brand's key claims, value propositions, proof points, and statistics.
%s
Steps:
1. Search for and visit the homepage, about page, product pages, and any case study or testimonial pages on %s
2. Look for: factual claims (e.g., "Used by 10,000+ teams"), value propositions, statistics, awards, certifications, customer proof points
3. Identify which claims are backed by evidence on specific pages
4. Assign priority based on how prominently the claim is featured

Return as JSON (no markdown code fences, just raw JSON):
{
  "claims": [
    {
      "claim": "The specific claim text",
      "evidence_url": "URL where this claim appears",
      "priority": "high|medium|low"
    }
  ]
}

Include 5-15 claims, prioritizing the most prominent and verifiable ones.`, domain, brandContext, domain)

		models := provider.Models()

		for mi, model := range models {
			if mi > 0 {
				sendSSE(w, flusher, "status", map[string]string{
					"message": fmt.Sprintf("%s unavailable, falling back to %s...", models[mi-1].Name, model.Name),
				})
			}

			claudeBody, _ := provider.BuildStreamBody(model.ID, 8192, prompt, true)

			const maxRetries = 3
			backoff := 2 * time.Second
			var lastErr error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					sendSSE(w, flusher, "status", map[string]string{
						"message": fmt.Sprintf("%s overloaded, retrying in %ds (attempt %d/%d)...", model.Name, int(backoff.Seconds()), attempt, maxRetries),
					})
					select {
					case <-time.After(backoff):
					case <-r.Context().Done():
						return
					}
					backoff *= 2
				}

				result, err := provider.Stream(r.Context(), apiKey, claudeBody, w, flusher)
				if err == errOverloaded {
					lastErr = err
					if attempt < maxRetries {
						continue
					}
					break
				}
				if err != nil {
					sendSSE(w, flusher, "error", map[string]string{"message": err.Error()})
					return
				}

				sendSSE(w, flusher, "done", map[string]any{
					"result": result.ResultJSON,
					"model":  model.Name,
				})
				return
			}

			log.Printf("%s API (%s) exhausted retries for claim suggestion: %v", provider.Name(), model.ID, lastErr)
		}

		sendSSE(w, flusher, "error", map[string]string{
			"message": "All models are currently overloaded. Please try again later.",
		})
	}
}

func handlePredictDifferentiators(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := r.PathValue("domain")

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		provider, apiKey, _, err := resolvePrimaryLLM(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Configure an API key in Settings", "code": "api_key_required"})
			return
		}
		_ = apiKey

		// Load existing brand profile for context (including competitors)
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		var brand BrandProfile
		brandErr := mongoDB.BrandProfiles().FindOne(ctx, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})).Decode(&brand)
		cancel()

		brandContext := ""
		if brandErr == nil {
			if brand.BrandName != "" {
				brandContext += fmt.Sprintf("\nBrand: %s\n", brand.BrandName)
			}
			if brand.Description != "" {
				brandContext += fmt.Sprintf("Description: %s\n", brand.Description)
			}
			if len(brand.Categories) > 0 {
				brandContext += fmt.Sprintf("Categories: %s\n", strings.Join(brand.Categories, ", "))
			}
			if len(brand.Products) > 0 {
				brandContext += fmt.Sprintf("Products: %s\n", strings.Join(brand.Products, ", "))
			}
			if len(brand.Competitors) > 0 {
				names := make([]string, len(brand.Competitors))
				for i, c := range brand.Competitors {
					names[i] = c.Name
				}
				brandContext += fmt.Sprintf("Known competitors: %s\n", strings.Join(names, ", "))
			}
		}

		sendSSE(w, flusher, "status", map[string]string{
			"message": "Analyzing " + domain + " to identify differentiators...",
		})

		prompt := fmt.Sprintf(`You are a competitive analyst. Visit and analyze the website at %s to identify what makes this brand unique compared to competitors.
%s
Steps:
1. Search for and visit the homepage, product pages, and comparison/features pages on %s
2. Identify unique features, approaches, technologies, or positioning that set this brand apart
3. If competitors are known, compare against them to find true differentiators
4. Focus on tangible, specific differentiators rather than generic marketing language

STRICT FORMAT RULES:
- Each differentiator MUST be 2-5 words. Maximum 5 words. No exceptions.
- NEVER use commas within a differentiator phrase. Use hyphens or "and" instead.
- Think of these as tags or badges, not descriptions or sentences.

Good: "AI-powered automation" "No per-seat pricing" "Open-source core" "Real-time collaboration" "Enterprise-grade security"
Bad: "Uses advanced AI to automate workflows" (too long)
Bad: "Flexible, scalable architecture" (contains comma)

Return as JSON (no markdown code fences, just raw JSON):
{
  "differentiators": ["phrase 1", "phrase 2", "phrase 3"]
}

Include 5-12 differentiators, ordered by distinctiveness.`, domain, brandContext, domain)

		models := provider.Models()

		for mi, model := range models {
			if mi > 0 {
				sendSSE(w, flusher, "status", map[string]string{
					"message": fmt.Sprintf("%s unavailable, falling back to %s...", models[mi-1].Name, model.Name),
				})
			}

			claudeBody, _ := provider.BuildStreamBody(model.ID, 8192, prompt, true)

			const maxRetries = 3
			backoff := 2 * time.Second
			var lastErr error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					sendSSE(w, flusher, "status", map[string]string{
						"message": fmt.Sprintf("%s overloaded, retrying in %ds (attempt %d/%d)...", model.Name, int(backoff.Seconds()), attempt, maxRetries),
					})
					select {
					case <-time.After(backoff):
					case <-r.Context().Done():
						return
					}
					backoff *= 2
				}

				result, err := provider.Stream(r.Context(), apiKey, claudeBody, w, flusher)
				if err == errOverloaded {
					lastErr = err
					if attempt < maxRetries {
						continue
					}
					break
				}
				if err != nil {
					sendSSE(w, flusher, "error", map[string]string{"message": err.Error()})
					return
				}

				sendSSE(w, flusher, "done", map[string]any{
					"result": result.ResultJSON,
					"model":  model.Name,
				})
				return
			}

			log.Printf("%s API (%s) exhausted retries for differentiator prediction: %v", provider.Name(), model.ID, lastErr)
		}

		sendSSE(w, flusher, "error", map[string]string{
			"message": "All models are currently overloaded. Please try again later.",
		})
	}
}

// ── Video Authority Analyzer Handlers ────────────────────────────────────

func handleVideoDiscover(ytKey string, mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ytKey == "" {
			http.Error(w, `{"error":"YOUTUBE_API_KEY not configured"}`, http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		var req struct {
			Domain      string   `json:"domain"`
			BrandName   string   `json:"brand_name"`
			ChannelURL  string   `json:"channel_url"`
			SearchTerms []string `json:"search_terms"`
			Competitors []string `json:"competitors"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Invalid request body"})
			return
		}

		progress := func(msg string) {
			sendSSE(w, flusher, "status", map[string]string{"message": msg})
		}

		videos, quotaUsed, err := discoverVideos(mongoDB, ytKey, req.BrandName, req.ChannelURL, req.SearchTerms, req.Competitors, progress)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": err.Error()})
			return
		}

		// Strip videos to lightweight summaries for the SSE payload
		type videoSummary struct {
			VideoID      string    `json:"video_id"`
			Title        string    `json:"title"`
			ChannelTitle string    `json:"channel_title"`
			ChannelID    string    `json:"channel_id"`
			PublishedAt  time.Time `json:"published_at"`
			ViewCount    int64     `json:"view_count"`
			Duration     string    `json:"duration"`
			RelevanceTag string    `json:"relevance_tag"`
		}
		summaries := make([]videoSummary, len(videos))
		for i, v := range videos {
			summaries[i] = videoSummary{
				VideoID:      v.VideoID,
				Title:        v.Title,
				ChannelTitle: v.ChannelTitle,
				ChannelID:    v.ChannelID,
				PublishedAt:  v.PublishedAt,
				ViewCount:    v.ViewCount,
				Duration:     v.Duration,
				RelevanceTag: v.RelevanceTag,
			}
		}
		sendSSE(w, flusher, "done", map[string]any{
			"videos":         summaries,
			"quota_estimate": quotaUsed,
		})
	}
}

func buildVideoAuthorityPrompt(domain string, ownVideos, thirdPartyVideos []YouTubeVideo, competitors, searchTerms []string, brandInfo BrandContextInfo, assessments map[string]*VideoAssessment) string {
	var sb strings.Builder
	brandName := domain
	if brandInfo.Used {
		for _, line := range strings.Split(brandInfo.ContextString, "\n") {
			if strings.HasPrefix(line, "Company: ") {
				brandName = strings.TrimPrefix(line, "Company: ")
				break
			}
		}
	}

	sb.WriteString(fmt.Sprintf(`You are an expert in Video LLM Authority analysis. Your job is to assess how strongly a brand's video ecosystem signals expertise to LLMs.

LLMs don't watch videos — they consume transcripts, titles, descriptions, and metadata. A 7B model trained on quality YouTube transcripts surpassed 72B models (LiveCC, CVPR 2025). Transcript IS the video to an LLM.

RESEARCH CONTEXT:
- Quotation-ready content gets +41%% LLM visibility; statistics add +33%% (GEO, Princeton 2024)
- Citation accuracy in AI search is only 49-68%%. 23-32%% of claims are unsupported.
- Citation concentration is extreme: top 20 sources capture 28-67%% of all citations (Gini 0.69-0.83). Being #1 vs #2 has outsized impact.
- Views and subscriber counts do NOT predict AI citation. Structural factors matter most.
- LLMs have U-shaped attention: beginning and end of transcripts get disproportionate weight.
- YouTube is #1 social citation source for LLMs (16%% of answers). Its share doubled from 19%% to 39%% in 4 months.
- Perplexity generates one-sided answers 83.4%% of the time — negative patterns get amplified.
- First-mover advantage in content gaps captures disproportionate citation share.
- Different AI providers cite different sources (cross-family similarity only 0.11-0.58).

NOTE: Each video below includes a pre-computed transcript assessment with keyword alignment, quotability, info density scores, key quotes, topics, and sentiment. Use these assessments as your primary evidence — they were produced by analyzing the full transcript text.

Brand: %s
Domain: %s
Target Search Terms: %s
Known Competitors: %s
`, brandName, domain, strings.Join(searchTerms, ", "), strings.Join(competitors, ", ")))

	if brandInfo.Used {
		sb.WriteString(brandInfo.ContextString)
	}

	// Helper to write assessment or fallback for a video
	writeVideoAssessment := func(v YouTubeVideo) {
		a := assessments[v.VideoID]
		if a != nil && a.HasTranscript {
			sb.WriteString(fmt.Sprintf("Assessment: keyword_alignment=%d, quotability=%d, info_density=%d\n", a.KeywordAlignment, a.Quotability, a.InfoDensity))
			if len(a.KeyQuotes) > 0 {
				sb.WriteString(fmt.Sprintf("Key Quotes: \"%s\"\n", strings.Join(a.KeyQuotes, "\" | \"")))
			}
			if len(a.Topics) > 0 {
				sb.WriteString(fmt.Sprintf("Topics: %s\n", strings.Join(a.Topics, ", ")))
			}
			sb.WriteString(fmt.Sprintf("Brand Sentiment: %s\n", a.BrandSentiment))
			sb.WriteString(fmt.Sprintf("Summary: %s\n", a.Summary))
		} else if v.Transcript != "" {
			// Fallback: assessment failed but transcript exists
			transcript := v.Transcript
			if len(transcript) > 1000 {
				transcript = transcript[:1000] + "... [truncated]"
			}
			sb.WriteString(fmt.Sprintf("Transcript (raw): %s\n", transcript))
		} else {
			sb.WriteString("Transcript: [NOT AVAILABLE — invisible to LLMs]\n")
			if v.Description != "" {
				desc := v.Description
				if len(desc) > 500 {
					desc = desc[:500] + "..."
				}
				sb.WriteString(fmt.Sprintf("Description: %s\n", desc))
			}
		}
	}

	// Own channel videos — detailed
	if len(ownVideos) > 0 {
		sb.WriteString(fmt.Sprintf("\n\n=== OWN CHANNEL VIDEOS (%d) ===\n\n", len(ownVideos)))
		for i, v := range ownVideos {
			sb.WriteString(fmt.Sprintf("--- Own Video %d ---\n", i+1))
			sb.WriteString(fmt.Sprintf("Title: %s\nVideo ID: %s\n", v.Title, v.VideoID))
			sb.WriteString(fmt.Sprintf("Views: %d | Likes: %d | Comments: %d\n", v.ViewCount, v.LikeCount, v.CommentCount))
			sb.WriteString(fmt.Sprintf("Duration: %s | Published: %s\n", v.Duration, v.PublishedAt.Format("2006-01-02")))
			if v.Description != "" {
				desc := v.Description
				if len(desc) > 500 {
					desc = desc[:500] + "..."
				}
				sb.WriteString(fmt.Sprintf("Description: %s\n", desc))
			}
			if len(v.Tags) > 0 {
				sb.WriteString(fmt.Sprintf("Tags: %s\n", strings.Join(v.Tags, ", ")))
			}
			writeVideoAssessment(v)
			sb.WriteString("\n")
		}
	}

	// Third-party videos — all included (compact assessments fit comfortably)
	if len(thirdPartyVideos) > 0 {
		sb.WriteString(fmt.Sprintf("\n\n=== THIRD-PARTY / LANDSCAPE VIDEOS (%d) ===\n\n", len(thirdPartyVideos)))
		for i, v := range thirdPartyVideos {
			sb.WriteString(fmt.Sprintf("--- Third-Party Video %d [%s] ---\n", i+1, v.RelevanceTag))
			sb.WriteString(fmt.Sprintf("Title: %s\nVideo ID: %s\nChannel: %s\n", v.Title, v.VideoID, v.ChannelTitle))
			sb.WriteString(fmt.Sprintf("Views: %d | Likes: %d\n", v.ViewCount, v.LikeCount))
			sb.WriteString(fmt.Sprintf("Published: %s\n", v.PublishedAt.Format("2006-01-02")))
			writeVideoAssessment(v)
			sb.WriteString("\n")
		}
	}

	sb.WriteString(fmt.Sprintf(`
Produce a unified Video LLM Authority report with 4 pillar scores (each 0-100):

=== PILLAR 1: TRANSCRIPT AUTHORITY (weight 30%%) ===
How well does the brand's own video content establish expertise through spoken words that LLMs ingest?
- Assess ONLY the own channel videos above.
- CRITICAL: If a video has no transcript, cap its contribution at 10. No captions = invisible.
- Keyword alignment: Do target search terms appear naturally in spoken words?
- Quotability: Standalone, citable statements? ("X is the best Y for Z because...") → +41%% visibility
- Statistical evidence: Specific numbers/benchmarks spoken aloud? → +33%% visibility
- Information density: Focused explainer vs. rambling content
- Front-loading: Key claims in first 20%% of transcript? (U-shaped attention)
- Entity explicitness: Brand/product name spoken clearly, not just shown on screen?

Sub-metrics to include: transcript_coverage (%%  of own videos with transcripts), keyword_alignment (0-100), quotability_score (0-100), information_density (0-100).
Evidence: 2-4 specific observations.

Per own-channel video, produce a scorecard: video_id, title, overall_score, transcript_power, structural_extractability, discovery_surface, has_transcript, key_findings (2-4 items).
- transcript_power (45%%): spoken content quality as LLM training data
- structural_extractability (30%%): how easily LLMs can parse and represent it (topic segmentation, Q&A patterns, claim clarity, metadata alignment)
- discovery_surface (25%%): findability by AI retrieval (title optimization, description depth, tag coverage, freshness)
- overall_score = transcript_power * 0.45 + structural_extractability * 0.30 + discovery_surface * 0.25

=== PILLAR 2: TOPICAL DOMINANCE (weight 25%%) ===
How comprehensively does the brand own key topic areas vs. the competitive landscape?
- Analyze ALL videos (own + third-party) to map topic coverage.
- Topics covered vs. total topics in the space
- Coverage depth: surface mentions vs. in-depth treatment
- Share of voice: %% of videos mentioning each brand. Include per-brand breakdown.
- Content gaps: topics where competitors are present but brand is absent. Score each gap's opportunity (0-100).
- First-mover opportunities in unclaimed territory.

Sub-metrics: topics_covered, topics_total, coverage_depth (0-100), vs_competitors (narrative comparison).
Include share_of_voice array and content_gaps array.

=== PILLAR 3: CITATION NETWORK (weight 25%%) ===
How connected and referenced is the brand by other authoritative creators?
- Analyze third-party videos for brand mentions and creator authority.
- Score creator authority (0-100) based on transcript quality, topical consistency — NOT subscriber count.
- Assess each creator's role: advocate/critic/neutral.
- Flag concentration risk: is the narrative dominated by 1-2 creators?
- Identify high-authority creators who cover competitors but NOT the brand (outreach targets).

Sub-metrics: creator_mentions (count), authoritative_refs (count of high-authority mentions), concentration_risk (narrative).
Include top_creators array and creator_targets array.

=== PILLAR 4: BRAND NARRATIVE QUALITY (weight 20%%) ===
When the brand appears in third-party video content, how is it framed?
- For each brand mention: sentiment (positive/negative/neutral), mention_context (recommendation/tutorial/comparison/complaint/passing), mention_position (early/middle/late), extractability (high/medium/low), competitors_mentioned.
- Weight early + high-extractability mentions higher (U-shaped attention).
- Apply 30%% confidence discount: LLM-constructed narrative may diverge from actual content.
- Narrative coherence: are mentions consistent or contradictory?
- Vulnerability assessment: negative patterns that LLMs would amplify?

Sub-metrics: sentiment breakdown (positive/neutral/negative/total), narrative_coherence (0-100).
Write narrative_summary: what an LLM would conclude about "%s" from this video evidence.
Include brand_mentions array and key_themes array.

=== OVERALL SCORE ===
overall_score = transcript_authority * 0.30 + topical_dominance * 0.25 + citation_network * 0.25 + brand_narrative * 0.20

Write executive_summary: 2-3 paragraph strategic overview of the brand's video LLM authority position.
Write confidence_note: explicit statement about citation accuracy limitations and what they mean for this brand.

Provide 5-12 structured recommendations. Each: action, expected_impact, dimension (one of "transcript_authority", "topical_dominance", "citation_network", "brand_narrative"), priority ("high"/"medium"/"low"), optionally video_id.

Return ONLY valid JSON matching this structure exactly:
{
  "overall_score": 58,
  "transcript_authority": {
    "score": 65, "evidence": ["...", "..."],
    "transcript_coverage": 80, "keyword_alignment": 55, "quotability_score": 60, "information_density": 70
  },
  "topical_dominance": {
    "score": 50, "evidence": ["...", "..."],
    "topics_covered": 4, "topics_total": 8, "coverage_depth": 55, "vs_competitors": "...",
    "share_of_voice": [{"brand_name": "%s", "mention_count": 10, "percentage": 25.0}],
    "content_gaps": [{"query": "...", "competitors_mentioned": ["A"], "opportunity_score": 80, "video_count": 5, "recommendation": "..."}]
  },
  "citation_network": {
    "score": 45, "evidence": ["...", "..."],
    "creator_mentions": 8, "authoritative_refs": 3, "concentration_risk": "...",
    "top_creators": [{"channel_title": "...", "channel_id": "...", "subscriber_count": 100000, "sentiment": "positive", "video_count": 2, "total_views": 50000, "role": "advocate", "authority_score": 75}],
    "creator_targets": [{"channel_title": "...", "channel_id": "...", "subscriber_count": 500000, "category_relevance": "...", "competitors_mentioned": ["A"], "outreach_reason": "..."}]
  },
  "brand_narrative": {
    "score": 62, "evidence": ["...", "..."],
    "sentiment": {"positive": 6, "neutral": 3, "negative": 1, "total": 10},
    "narrative_summary": "Based on the video evidence...",
    "narrative_coherence": 70, "key_themes": ["Theme 1", "Theme 2"],
    "brand_mentions": [{"video_id": "...", "title": "...", "channel_title": "...", "view_count": 50000, "sentiment": "positive", "mention_context": "Recommended as top pick", "mention_position": "early", "extractability": "high", "competitors_mentioned": ["A"]}]
  },
  "video_scorecards": [
    {"video_id": "...", "title": "...", "overall_score": 70, "transcript_power": 60, "structural_extractability": 75, "discovery_surface": 80, "has_transcript": true, "key_findings": ["...", "..."]}
  ] // IMPORTANT: Include scorecards for ALL own-channel videos plus the top 20 and bottom 10 third-party videos by overall_score. Do NOT include all 300+ videos.,
  "executive_summary": "...",
  "confidence_note": "...",
  "recommendations": [
    {"action": "...", "expected_impact": "...", "dimension": "transcript_authority", "priority": "high", "video_id": "..."}
  ]
}`, brandName, brandName))

	return sb.String()
}

func handleVideoAnalyze(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool, ytKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ytKey == "" {
			http.Error(w, `{"error":"YOUTUBE_API_KEY not configured"}`, http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		provider, apiKey, _, err := resolvePrimaryLLM(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Configure an API key in Settings", "code": "api_key_required"})
			return
		}
		_ = apiKey

		var req struct {
			Domain           string              `json:"domain"`
			Config           VideoAnalysisConfig `json:"config"`
			SelectedVideoIDs []string            `json:"selected_video_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Invalid request body"})
			return
		}

		if len(req.SelectedVideoIDs) == 0 {
			sendSSE(w, flusher, "error", map[string]string{"message": "No videos selected for analysis"})
			return
		}
		req.Domain = normalizeDomain(req.Domain)

		brandInfo := lookupBrandContext(mongoDB, req.Domain, saas.TenantIDFromContext(r.Context()))

		// Phase 1: Gather video data
		sendSSE(w, flusher, "status", map[string]string{
			"message": fmt.Sprintf("Fetching metadata for %d videos...", len(req.SelectedVideoIDs)),
		})

		videos, err := cachedVideoDetails(mongoDB, ytKey, req.SelectedVideoIDs)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Failed to fetch video details: " + err.Error()})
			return
		}

		// Fetch transcripts with adaptive backoff on rate limiting
		total := len(videos)
		cachedCount, noCaptionsCount, blockedCount, errorCount := 0, 0, 0, 0
		methodCounts := map[string]int{} // tracks which fetch method succeeded
		consecutiveErrors := 0
		delay := 500 * time.Millisecond
		allMethodNames := []string{"android", "web", "scrape", "warp-android", "warp-web", "warp-scrape"}

		for i := range videos {
			freshTotal := 0
			for _, c := range methodCounts {
				freshTotal += c
			}
			var methodParts []string
			for _, m := range allMethodNames {
				if c, ok := methodCounts[m]; ok && c > 0 {
					methodParts = append(methodParts, fmt.Sprintf("%d %s", c, m))
				}
			}
			fetchedStr := fmt.Sprintf("%d fetched", freshTotal)
			if len(methodParts) > 0 {
				fetchedStr = fmt.Sprintf("%d fetched (%s)", freshTotal, strings.Join(methodParts, ", "))
			}

			sendSSE(w, flusher, "progress", map[string]string{
				"message": fmt.Sprintf("Extracting transcripts (%d/%d)... [%d cached, %s, %d no captions, %d blocked, %d errors]",
					i+1, total, cachedCount, fetchedStr, noCaptionsCount, blockedCount, errorCount),
			})
			transcript, fromCache, method, err := cachedTranscript(mongoDB, videos[i].VideoID)

			if err != nil {
				if errors.Is(err, errNoCaptions) {
					noCaptionsCount++
					consecutiveErrors = 0
				} else if errors.Is(err, errBlocked) {
					blockedCount++
					consecutiveErrors++
					if consecutiveErrors >= 3 {
						newDelay := delay * 2
						if newDelay > 5*time.Second {
							newDelay = 5 * time.Second
						}
						if newDelay > delay {
							delay = newDelay
							log.Printf("Transcript: %d consecutive blocked/errors, increasing delay to %v", consecutiveErrors, delay)
						}
					}
				} else {
					log.Printf("Transcript error for %s: %v", videos[i].VideoID, err)
					errorCount++
					consecutiveErrors++
					if consecutiveErrors >= 3 {
						newDelay := delay * 2
						if newDelay > 5*time.Second {
							newDelay = 5 * time.Second
						}
						if newDelay > delay {
							delay = newDelay
							log.Printf("Transcript: %d consecutive errors, increasing delay to %v", consecutiveErrors, delay)
						}
					}
				}
			} else {
				consecutiveErrors = 0
				if delay > 500*time.Millisecond {
					delay = 500 * time.Millisecond
				}
			}

			videos[i].Transcript = transcript
			if transcript != "" {
				if fromCache {
					cachedCount++
				} else {
					methodCounts[method]++
				}
			} else if err == nil {
				noCaptionsCount++
			}
			// Delay between uncached fetches
			if !fromCache && i < total-1 {
				time.Sleep(delay)
			}
		}

		freshTotal := 0
		for _, c := range methodCounts {
			freshTotal += c
		}
		var methodParts []string
		for _, m := range allMethodNames {
			if c, ok := methodCounts[m]; ok && c > 0 {
				methodParts = append(methodParts, fmt.Sprintf("%d %s", c, m))
			}
		}
		fetchedStr := fmt.Sprintf("%d fetched", freshTotal)
		if len(methodParts) > 0 {
			fetchedStr = fmt.Sprintf("%d fetched (%s)", freshTotal, strings.Join(methodParts, ", "))
		}
		transcriptCount := cachedCount + freshTotal
		sendSSE(w, flusher, "status", map[string]string{
			"message": fmt.Sprintf("Transcripts: %d/%d videos (%d cached, %s, %d no captions, %d blocked, %d errors)",
				transcriptCount, total, cachedCount, fetchedStr, noCaptionsCount, blockedCount, errorCount),
		})

		// Determine own channel ID for filtering
		var ownChannelID string
		if req.Config.ChannelURL != "" {
			chID, err := resolveChannelID(ytKey, req.Config.ChannelURL)
			if err == nil {
				ownChannelID = chID
			}
		}

		// Split videos into own vs third-party
		var ownVideos, thirdPartyVideos []YouTubeVideo
		for _, v := range videos {
			if ownChannelID != "" && v.ChannelID == ownChannelID {
				v.RelevanceTag = "own"
				ownVideos = append(ownVideos, v)
			} else {
				if v.RelevanceTag == "" {
					v.RelevanceTag = "category_content"
				}
				thirdPartyVideos = append(thirdPartyVideos, v)
			}
		}

		usedModel := ""

		models := provider.Models()

		// Helper to run an LLM analysis with retries and fallback
		runAnalysis := func(prompt, phaseName string) (string, string, error) {
			for mi, model := range models {
				if mi > 0 {
					sendSSE(w, flusher, "status", map[string]string{
						"message": fmt.Sprintf("%s unavailable, falling back to %s for %s...", models[mi-1].Name, model.Name, phaseName),
					})
				}

				claudeBody, _ := provider.BuildStreamBody(model.ID, 65536, prompt, false)

				const maxRetries = 3
				backoff := 2 * time.Second
				var lastErr error

				for attempt := 0; attempt <= maxRetries; attempt++ {
					if attempt > 0 {
						sendSSE(w, flusher, "status", map[string]string{
							"message": fmt.Sprintf("%s overloaded, retrying in %ds (attempt %d/%d)...", model.Name, int(backoff.Seconds()), attempt, maxRetries),
						})
						select {
						case <-time.After(backoff):
						case <-r.Context().Done():
							return "", "", fmt.Errorf("request cancelled")
						}
						backoff *= 2
					}

					result, err := provider.Stream(r.Context(), apiKey, claudeBody, w, flusher)
					if err == errOverloaded {
						lastErr = err
						if attempt < maxRetries {
							continue
						}
						break
					}
					if err != nil {
						return "", "", err
					}

					return result.ResultJSON, model.Name, nil
				}

				log.Printf("%s API (%s) exhausted retries for %s: %v", provider.Name(), model.ID, phaseName, lastErr)
			}
			return "", "", fmt.Errorf("all models overloaded")
		}

		// Extract competitor names from brand context
		var competitorNames []string
		if brandInfo.Used {
			cCtx, cCancel := context.WithTimeout(r.Context(), 5*time.Second)
			var brand BrandProfile
			if err := mongoDB.BrandProfiles().FindOne(cCtx, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: req.Domain}})).Decode(&brand); err == nil {
				for _, c := range brand.Competitors {
					competitorNames = append(competitorNames, c.Name)
				}
			}
			cCancel()
		}

		// Phase 1: Per-video transcript assessments with Haiku
		sendSSE(w, flusher, "status", map[string]string{
			"message": "Phase 1: Assessing individual transcripts...",
		})
		assessments := assessVideos(r.Context(), provider, apiKey, videos, req.Domain, req.Config.SearchTerms, mongoDB, w, flusher)

		assessedCount := 0
		for _, a := range assessments {
			if a != nil {
				assessedCount++
			}
		}
		sendSSE(w, flusher, "status", map[string]string{
			"message": fmt.Sprintf("Phase 1 complete: %d/%d videos assessed. Starting final analysis...", assessedCount, transcriptCount),
		})

		// Phase 2: Unified analysis with compact assessments
		sendSSE(w, flusher, "status", map[string]string{
			"message": fmt.Sprintf("Phase 2: Analyzing %d videos (%d own channel, %d third-party) for LLM authority...",
				len(videos), len(ownVideos), len(thirdPartyVideos)),
		})

		prompt := buildVideoAuthorityPrompt(req.Domain, ownVideos, thirdPartyVideos, competitorNames, req.Config.SearchTerms, brandInfo, assessments)
		resultJSON, modelName, err := runAnalysis(prompt, "Video Authority")
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Analysis failed: " + err.Error()})
			return
		}
		usedModel = modelName

		resultJSON = stripJSONFencing(resultJSON)
		var result VideoAuthorityResult
		if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
			log.Printf("Warning: failed to parse video authority result: %v", err)
			sendSSE(w, flusher, "error", map[string]string{"message": "Failed to parse analysis result"})
			return
		}

		sendSSE(w, flusher, "status", map[string]string{
			"message": fmt.Sprintf("Analysis complete — Overall Score: %d/100", result.OverallScore),
		})

		// Save results
		analysis := VideoAnalysis{
			Domain:           req.Domain,
			TenantID:         saas.TenantIDFromContext(r.Context()),
			Config:           req.Config,
			Videos:           videos,
			Result:           &result,
			RawText:          resultJSON,
			Model:            usedModel,
			BrandContextUsed: brandInfo.Used,
			GeneratedAt:      time.Now(),
		}

		saveCtx, saveCancel := context.WithTimeout(r.Context(), 10*time.Second)
		upsertResult, saveErr := mongoDB.VideoAnalyses().ReplaceOne(saveCtx,
			tenantFilter(r.Context(), bson.D{{Key: "domain", Value: req.Domain}}),
			analysis,
			options.Replace().SetUpsert(true),
		)
		saveCancel()
		if saveErr != nil {
			log.Printf("Failed to save video analysis: %v", saveErr)
		}

		// Create todos from recommendations
		if saveErr == nil {
			var analysisID primitive.ObjectID
			if upsertResult.UpsertedID != nil {
				analysisID = upsertResult.UpsertedID.(primitive.ObjectID)
			} else {
				fetchCtx, fetchCancel := context.WithTimeout(r.Context(), 5*time.Second)
				var existing VideoAnalysis
				if err := mongoDB.VideoAnalyses().FindOne(fetchCtx, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: req.Domain}})).Decode(&existing); err == nil {
					analysisID = existing.ID
				}
				fetchCancel()
			}
			if !analysisID.IsZero() {
				go createTodosFromVideoAnalysis(mongoDB, analysisID, req.Domain, saas.TenantIDFromContext(r.Context()), result.Recommendations)
			}
		}

		// Build result for frontend
		resultMap := map[string]any{
			"domain":             req.Domain,
			"config":             req.Config,
			"videos":             videos,
			"result":             &result,
			"model":              usedModel,
			"brand_context_used": brandInfo.Used,
			"generated_at":       analysis.GeneratedAt,
		}

		frontendJSON, _ := json.Marshal(resultMap)
		sendSSE(w, flusher, "done", map[string]any{
			"result": string(frontendJSON),
		})
	}
}

func handleGetVideoAnalysis(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		opts := options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}})

		var analysis VideoAnalysis
		err := mongoDB.VideoAnalyses().FindOne(ctx, filter, opts).Decode(&analysis)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			} else {
				http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(analysis)
	}
}

func handleGetVideoDetails(mongoDB *MongoDB) http.HandlerFunc {
	const transcriptSnippetLen = 500

	type videoDetail struct {
		VideoID          string           `json:"video_id"`
		Title            string           `json:"title"`
		Transcript       string           `json:"transcript"`
		TranscriptLength int              `json:"transcript_length"`
		Assessment       *VideoAssessment `json:"assessment"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		opts := options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}})

		var analysis VideoAnalysis
		err := mongoDB.VideoAnalyses().FindOne(ctx, filter, opts).Decode(&analysis)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			} else {
				http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			}
			return
		}

		details := make([]videoDetail, 0, len(analysis.Videos))
		for _, v := range analysis.Videos {
			snippet := v.Transcript
			if len(snippet) > transcriptSnippetLen {
				snippet = snippet[:transcriptSnippetLen]
			}
			d := videoDetail{
				VideoID:          v.VideoID,
				Title:            v.Title,
				Transcript:       snippet,
				TranscriptLength: len(v.Transcript),
			}
			if a, ok := cachedVideoAssessment(mongoDB, v.VideoID, domain, analysis.Config.SearchTerms); ok {
				a.VideoID = v.VideoID
				a.Title = v.Title
				a.HasTranscript = v.Transcript != ""
				d.Assessment = a
			}
			details = append(details, d)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(details)
	}
}

func handleListVideoAnalyses(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Exclude bulky fields
		opts := options.Find().
			SetSort(bson.D{{Key: "generatedAt", Value: -1}}).
			SetLimit(50).
			SetProjection(bson.D{
				{Key: "rawText", Value: 0},
				{Key: "videos.transcript", Value: 0},
				{Key: "videos.description", Value: 0},
				{Key: "videos.tags", Value: 0},
				{Key: "result.videoScorecards", Value: 0},
				{Key: "result.brandNarrative.brandMentions", Value: 0},
				{Key: "result.citationNetwork.topCreators", Value: 0},
				{Key: "result.citationNetwork.creatorTargets", Value: 0},
				{Key: "result.topicalDominance.contentGaps", Value: 0},
			})

		cursor, err := mongoDB.VideoAnalyses().Find(ctx, tenantFilter(r.Context(), bson.D{}), opts)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		var results []bson.M
		if err := cursor.All(ctx, &results); err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		var summaries []map[string]any
		for _, r := range results {
			summary := map[string]any{
				"id":           r["_id"],
				"domain":       r["domain"],
				"model":        r["model"],
				"generated_at": r["generatedAt"],
			}
			if res, ok := r["result"].(bson.M); ok {
				if score, ok := res["overallScore"]; ok {
					summary["overall_score"] = score
				}
			}
			if vids, ok := r["videos"].(bson.A); ok {
				summary["video_count"] = len(vids)
			} else {
				summary["video_count"] = 0
			}
			summaries = append(summaries, summary)
		}

		if summaries == nil {
			summaries = []map[string]any{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(summaries)
	}
}

func handleDeleteVideoAnalysis(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Find the analysis first to get its ID for cascade delete
		delFilter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		var analysis struct {
			ID primitive.ObjectID `bson:"_id"`
		}
		findErr := mongoDB.VideoAnalyses().FindOne(ctx, delFilter).Decode(&analysis)

		result, err := mongoDB.VideoAnalyses().DeleteOne(ctx, delFilter)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		// Cascade delete associated todos
		if findErr == nil && result.DeletedCount > 0 {
			mongoDB.Todos().DeleteMany(ctx, bson.D{{Key: "videoAnalysisId", Value: analysis.ID}})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"deleted": result.DeletedCount > 0,
		})
	}
}

// ── Reddit Authority Analyzer handlers ─────────────────────────────

func handleRedditDiscover(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		var req struct {
			Domain      string   `json:"domain"`
			BrandName   string   `json:"brand_name"`
			Subreddits  []string `json:"subreddits"`
			SearchTerms []string `json:"search_terms"`
			Competitors []string `json:"competitors"`
			TimeFilter  string   `json:"time_filter"` // "month", "year", "all"
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Invalid request body"})
			return
		}

		if len(req.SearchTerms) == 0 {
			sendSSE(w, flusher, "error", map[string]string{"message": "At least one search term is required"})
			return
		}

		// Normalize subreddits
		var subs []string
		for _, s := range req.Subreddits {
			if n := normalizeSubreddit(s); n != "" {
				subs = append(subs, n)
			}
		}
		// Always include a broad search
		if len(subs) == 0 {
			subs = []string{"all"}
		}

		timeFilter := req.TimeFilter
		if timeFilter == "" {
			timeFilter = "year"
		}

		// Build search terms: explicit terms + brand name + competitor names
		allTerms := make([]string, 0, len(req.SearchTerms)+1+len(req.Competitors))
		seen := map[string]bool{}
		for _, t := range req.SearchTerms {
			t = strings.TrimSpace(t)
			if t != "" && !seen[strings.ToLower(t)] {
				seen[strings.ToLower(t)] = true
				allTerms = append(allTerms, t)
			}
		}
		// Add brand name as a search term if not already present
		if req.BrandName != "" && !seen[strings.ToLower(req.BrandName)] {
			seen[strings.ToLower(req.BrandName)] = true
			allTerms = append(allTerms, req.BrandName)
		}

		progress := func(msg string) {
			sendSSE(w, flusher, "status", map[string]string{"message": msg})
		}

		progress(fmt.Sprintf("Discovering Reddit threads across %d subreddits with %d search terms...", len(subs), len(allTerms)))

		threads, err := redditDiscoverThreads(subs, allTerms, timeFilter, 15, 2*time.Second, progress)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Reddit discovery failed: " + err.Error()})
			return
		}

		if len(threads) == 0 {
			sendSSE(w, flusher, "error", map[string]string{"message": "No Reddit threads found matching your search criteria"})
			return
		}

		progress(fmt.Sprintf("Found %d unique threads. Fetching thread details...", len(threads)))

		// Sort by score descending and fetch top threads with comments
		sortThreadsByScore(threads)
		maxFetch := 40
		if len(threads) < maxFetch {
			maxFetch = len(threads)
		}
		detailed := redditFetchThreadDetails(threads[:maxFetch], maxFetch, 2*time.Second, progress)

		// Build summaries for frontend
		type threadSummary struct {
			ID          string    `json:"id"`
			Subreddit   string    `json:"subreddit"`
			Title       string    `json:"title"`
			Score       int       `json:"score"`
			UpvoteRatio float64   `json:"upvote_ratio"`
			NumComments int       `json:"num_comments"`
			Permalink   string    `json:"permalink"`
			CreatedUTC  time.Time `json:"created_utc"`
			IsSelfPost  bool      `json:"is_self_post"`
			HasComments bool      `json:"has_comments"`
		}
		summaries := make([]threadSummary, len(detailed))
		for i, t := range detailed {
			summaries[i] = threadSummary{
				ID:          t.ID,
				Subreddit:   t.Subreddit,
				Title:       t.Title,
				Score:       t.Score,
				UpvoteRatio: t.UpvoteRatio,
				NumComments: t.NumComments,
				Permalink:   t.Permalink,
				CreatedUTC:  t.CreatedUTC,
				IsSelfPost:  t.IsSelfPost,
				HasComments: len(t.TopComments) > 0,
			}
		}

		sendSSE(w, flusher, "done", map[string]any{
			"threads": summaries,
			"total":   len(threads),
		})
	}
}

func sortThreadsByScore(threads []RedditThread) {
	for i := 1; i < len(threads); i++ {
		for j := i; j > 0 && threads[j].Score > threads[j-1].Score; j-- {
			threads[j], threads[j-1] = threads[j-1], threads[j]
		}
	}
}

func handleRedditAnalyze(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		provider, apiKey, _, err := resolvePrimaryLLM(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Configure an API key in Settings", "code": "api_key_required"})
			return
		}
		_ = apiKey

		var req struct {
			Domain            string   `json:"domain"`
			Config            RedditAnalysisConfig `json:"config"`
			SelectedThreadIDs []string `json:"selected_thread_ids"`
			// Full thread data from discovery
			Threads []RedditThread `json:"threads"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Invalid request body"})
			return
		}

		if len(req.Threads) == 0 {
			sendSSE(w, flusher, "error", map[string]string{"message": "No threads selected for analysis"})
			return
		}
		req.Domain = normalizeDomain(req.Domain)

		brandInfo := lookupBrandContext(mongoDB, req.Domain, saas.TenantIDFromContext(r.Context()))

		// Fetch full thread details with comments for selected threads
		sendSSE(w, flusher, "status", map[string]string{
			"message": fmt.Sprintf("Fetching full content for %d threads...", len(req.Threads)),
		})

		selectedSet := map[string]bool{}
		for _, id := range req.SelectedThreadIDs {
			selectedSet[id] = true
		}

		// Filter to selected threads
		var threadsToAnalyze []RedditThread
		for _, t := range req.Threads {
			if len(selectedSet) == 0 || selectedSet[t.ID] {
				threadsToAnalyze = append(threadsToAnalyze, t)
			}
		}

		// Fetch full thread details with comments
		detailed := redditFetchThreadDetails(threadsToAnalyze, len(threadsToAnalyze), 2*time.Second, func(msg string) {
			sendSSE(w, flusher, "status", map[string]string{"message": msg})
		})

		// Extract competitor names
		var competitorNames []string
		if brandInfo.Used {
			cCtx, cCancel := context.WithTimeout(r.Context(), 5*time.Second)
			var brand BrandProfile
			if err := mongoDB.BrandProfiles().FindOne(cCtx, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: req.Domain}})).Decode(&brand); err == nil {
				for _, c := range brand.Competitors {
					competitorNames = append(competitorNames, c.Name)
				}
			}
			cCancel()
		}

		// Build the analysis prompt
		sendSSE(w, flusher, "status", map[string]string{
			"message": fmt.Sprintf("Analyzing %d Reddit threads for LLM authority signals...", len(detailed)),
		})

		prompt := buildRedditAuthorityPrompt(req.Domain, detailed, competitorNames, req.Config.SearchTerms, brandInfo)

		// Model fallback chain (same as video)
		usedModel := ""
		models := provider.Models()

		runAnalysis := func(prompt, phaseName string) (string, string, error) {
			for mi, model := range models {
				if mi > 0 {
					sendSSE(w, flusher, "status", map[string]string{
						"message": fmt.Sprintf("%s unavailable, falling back to %s for %s...", models[mi-1].Name, model.Name, phaseName),
					})
				}

				claudeBody, _ := provider.BuildStreamBody(model.ID, 65536, prompt, false)

				const maxRetries = 3
				backoff := 2 * time.Second
				var lastErr error

				for attempt := 0; attempt <= maxRetries; attempt++ {
					if attempt > 0 {
						sendSSE(w, flusher, "status", map[string]string{
							"message": fmt.Sprintf("%s overloaded, retrying in %ds (attempt %d/%d)...", model.Name, int(backoff.Seconds()), attempt, maxRetries),
						})
						select {
						case <-time.After(backoff):
						case <-r.Context().Done():
							return "", "", fmt.Errorf("request cancelled")
						}
						backoff *= 2
					}

					result, err := provider.Stream(r.Context(), apiKey, claudeBody, w, flusher)
					if err == errOverloaded {
						lastErr = err
						if attempt < maxRetries {
							continue
						}
						break
					}
					if err != nil {
						return "", "", err
					}

					return result.ResultJSON, model.Name, nil
				}

				log.Printf("%s API (%s) exhausted retries for %s: %v", provider.Name(), model.ID, phaseName, lastErr)
			}
			return "", "", fmt.Errorf("all models overloaded")
		}

		resultJSON, modelName, err := runAnalysis(prompt, "Reddit Authority")
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Analysis failed: " + err.Error()})
			return
		}
		usedModel = modelName

		resultJSON = stripJSONFencing(resultJSON)
		var result RedditAuthorityResult
		if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
			log.Printf("Warning: failed to parse reddit authority result: %v", err)
			sendSSE(w, flusher, "error", map[string]string{"message": "Failed to parse analysis result"})
			return
		}

		sendSSE(w, flusher, "status", map[string]string{
			"message": fmt.Sprintf("Analysis complete — Overall Score: %d/100", result.OverallScore),
		})

		// Convert threads to summaries for storage
		storedThreads := make([]RedditThreadSummary, len(detailed))
		for i, t := range detailed {
			storedThreads[i] = RedditThreadSummary{
				ID:           t.ID,
				Subreddit:    t.Subreddit,
				Title:        t.Title,
				SelfText:     truncate(t.SelfText, 500),
				Author:       t.Author,
				Score:        t.Score,
				UpvoteRatio:  t.UpvoteRatio,
				NumComments:  t.NumComments,
				URL:          t.URL,
				Permalink:    t.Permalink,
				CreatedUTC:   t.CreatedUTC,
				IsSelfPost:   t.IsSelfPost,
				CommentCount: len(t.TopComments),
			}
		}

		// Save results
		analysis := RedditAnalysis{
			Domain:           req.Domain,
			TenantID:         saas.TenantIDFromContext(r.Context()),
			Config:           req.Config,
			Threads:          storedThreads,
			Result:           &result,
			RawText:          resultJSON,
			Model:            usedModel,
			BrandContextUsed: brandInfo.Used,
			GeneratedAt:      time.Now(),
		}

		saveCtx, saveCancel := context.WithTimeout(r.Context(), 10*time.Second)
		_, saveErr := mongoDB.RedditAnalyses().ReplaceOne(saveCtx,
			tenantFilter(r.Context(), bson.D{{Key: "domain", Value: req.Domain}}),
			analysis,
			options.Replace().SetUpsert(true),
		)
		saveCancel()
		if saveErr != nil {
			log.Printf("Failed to save reddit analysis: %v", saveErr)
		}

		// Create todos from recommendations
		if saveErr == nil && len(result.Recommendations) > 0 {
			go createTodosFromRedditAnalysis(mongoDB, req.Domain, saas.TenantIDFromContext(r.Context()), result.Recommendations)
		}

		// Build result for frontend
		resultMap := map[string]any{
			"domain":             req.Domain,
			"config":             req.Config,
			"threads":            storedThreads,
			"result":             &result,
			"model":              usedModel,
			"brand_context_used": brandInfo.Used,
			"generated_at":       analysis.GeneratedAt,
		}

		frontendJSON, _ := json.Marshal(resultMap)
		sendSSE(w, flusher, "done", map[string]any{
			"result": string(frontendJSON),
		})
	}
}

func createTodosFromRedditAnalysis(mongoDB *MongoDB, domain, tenantID string, recommendations []RedditRecommendation) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, rec := range recommendations {
		if rec.Priority != "high" && rec.Priority != "medium" {
			continue
		}
		todo := TodoItem{
			TenantID:       tenantID,
			SourceType:     "reddit",
			Domain:         domain,
			Question:       "Reddit Authority",
			Action:         rec.Action,
			Summary:        rec.Action,
			ExpectedImpact: rec.ExpectedImpact,
			Dimension:      rec.Dimension,
			Priority:       rec.Priority,
			Status:         "todo",
			CreatedAt:      time.Now(),
		}
		if _, err := mongoDB.Todos().InsertOne(ctx, todo); err != nil {
			log.Printf("Failed to create reddit todo: %v", err)
		}
	}

	// Deduplicate todos for this domain
	go deduplicateTodos(mongoDB, domain, tenantID)
}

func buildRedditAuthorityPrompt(domain string, threads []RedditThread, competitors, searchTerms []string, brandInfo BrandContextInfo) string {
	var sb strings.Builder
	brandName := domain
	if brandInfo.Used {
		for _, line := range strings.Split(brandInfo.ContextString, "\n") {
			if strings.HasPrefix(line, "Company: ") {
				brandName = strings.TrimPrefix(line, "Company: ")
				break
			}
		}
	}

	sb.WriteString(fmt.Sprintf(`You are an expert at analyzing Reddit presence for LLM authority optimization.

## Context

**Target Brand**: %s (domain: %s)
`, brandName, domain))

	if brandInfo.Used {
		sb.WriteString(fmt.Sprintf("\n**Brand Intelligence**:\n%s\n", brandInfo.ContextString))
	}

	if len(competitors) > 0 {
		sb.WriteString(fmt.Sprintf("\n**Key Competitors**: %s\n", strings.Join(competitors, ", ")))
	}

	if len(searchTerms) > 0 {
		sb.WriteString(fmt.Sprintf("\n**Search Terms Used**: %s\n", strings.Join(searchTerms, ", ")))
	}

	sb.WriteString(`
## Research Context

Reddit is the #2 source for LLM-generated citations, behind only YouTube. Reddit threads—especially highly-upvoted recommendation threads—strongly influence how LLMs answer questions about products, services, and brands. Key dynamics:

1. **Training Data Weight**: Reddit was foundational in LLM training (WebText, Common Crawl). Google pays $60M/year and OpenAI $70M/year for Reddit data access.
2. **Multi-User Validation**: Upvotes and comment consensus create credibility signals that single-author content cannot match.
3. **Recommendation Threads**: "Best X for Y" threads are among the most influential for LLM responses to comparison queries.
4. **Community Authority**: Mentions in authoritative, topic-specific subreddits carry more weight than general subreddits.
5. **Recency Signal**: Recent discussions with active engagement signal ongoing relevance.

## Task

Analyze the following Reddit threads to assess this brand's Reddit authority—how strongly Reddit discussions would influence LLMs to cite, recommend, or reference this brand.

## Thread Data

`)

	for i, t := range threads {
		sb.WriteString(fmt.Sprintf("### Thread %d: r/%s — %s\n", i+1, t.Subreddit, t.Title))
		sb.WriteString(fmt.Sprintf("Score: %d | Upvote Ratio: %.0f%% | Comments: %d | Posted: %s\n",
			t.Score, t.UpvoteRatio*100, t.NumComments, t.CreatedUTC.Format("2006-01-02")))
		sb.WriteString(fmt.Sprintf("Permalink: https://reddit.com%s\n", t.Permalink))

		if t.SelfText != "" {
			text := t.SelfText
			if len(text) > 1000 {
				text = text[:1000] + "..."
			}
			sb.WriteString(fmt.Sprintf("\n**Post Body**:\n%s\n", text))
		}

		if len(t.TopComments) > 0 {
			sb.WriteString("\n**Top Comments**:\n")
			for j, c := range t.TopComments {
				if j >= 10 {
					break
				}
				body := c.Body
				if len(body) > 500 {
					body = body[:500] + "..."
				}
				sb.WriteString(fmt.Sprintf("- [%d pts] %s\n", c.Score, body))
			}
		}
		sb.WriteString("\n---\n\n")
	}

	sb.WriteString(fmt.Sprintf(`## Output Format

Return a JSON object with the following structure. ALL scores are 0-100. Be thorough and evidence-based.

{
  "overall_score": <0-100 weighted average>,
  "presence": {
    "score": <0-100>,
    "evidence": ["evidence point 1", "..."],
    "total_mentions": <count of threads where %s is explicitly mentioned>,
    "unique_subreddits": <count of unique subreddits with mentions>,
    "share_of_voice": [
      {"brand_name": "%s", "mention_count": <n>, "percentage": <0-100>},
      {"brand_name": "<competitor>", "mention_count": <n>, "percentage": <0-100>}
    ],
    "mention_trend": "growing|stable|declining"
  },
  "sentiment": {
    "score": <0-100>,
    "evidence": ["..."],
    "sentiment": {"positive": <n>, "neutral": <n>, "negative": <n>, "total": <n>},
    "recommendation_rate": <0-100 percent of mentions that recommend the brand>,
    "top_praise": ["praise theme 1", "..."],
    "top_criticism": ["criticism theme 1", "..."],
    "notable_mentions": [
      {
        "thread_id": "<id>",
        "subreddit": "<subreddit>",
        "title": "<thread title>",
        "score": <thread score>,
        "sentiment": "positive|neutral|negative",
        "context": "<excerpt showing the mention>",
        "is_recommendation": <true if user recommends the brand>
      }
    ]
  },
  "competitive": {
    "score": <0-100>,
    "evidence": ["..."],
    "win_rate": <0-100 percent of head-to-head comparisons where brand wins>,
    "comparison_threads": <count of threads comparing brand to competitors>,
    "differentiators": ["unique strength cited by Reddit users", "..."],
    "competitor_strengths": ["competitor advantage not countered", "..."],
    "head_to_head_examples": [<same format as notable_mentions>]
  },
  "training_signal": {
    "score": <0-100>,
    "evidence": ["..."],
    "high_score_threads": <count of threads with 50+ score>,
    "deep_threads": <count of threads with 10+ comments>,
    "authority_tier": "strong|moderate|weak",
    "key_threads": [<most influential threads in notable_mention format>],
    "recommendations": ["specific action to improve Reddit training signal", "..."]
  },
  "executive_summary": "<2-3 paragraph analysis of the brand's Reddit presence and its implications for LLM authority>",
  "confidence_note": "<brief note on data limitations or confidence level>",
  "recommendations": [
    {
      "action": "<specific actionable recommendation>",
      "expected_impact": "<expected outcome>",
      "dimension": "presence|sentiment|competitive|training_signal",
      "priority": "high|medium|low"
    }
  ]
}

**Scoring Guidelines**:
- **Presence (25%% weight)**: Volume and breadth of mentions. High = mentioned across many relevant subreddits, frequently. Low = rarely discussed.
- **Sentiment (25%% weight)**: Tone and recommendation strength. High = frequently recommended, positive consensus. Low = criticized or ignored.
- **Competitive (25%% weight)**: Position vs. competitors. High = wins head-to-head comparisons, cited as best-in-class. Low = loses comparisons, positioned as inferior.
- **Training Signal (25%% weight)**: Likelihood that Reddit content will influence LLM training. High = many high-upvote, deep-comment threads in authoritative subreddits. Low = low-engagement or shallow discussions.

Return ONLY the JSON object, no markdown fencing.
`, brandName, brandName))

	return sb.String()
}

func handleGetRedditAnalysis(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		opts := options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}})

		var analysis RedditAnalysis
		err := mongoDB.RedditAnalyses().FindOne(ctx, filter, opts).Decode(&analysis)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			} else {
				http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(analysis)
	}
}

func handleListRedditAnalyses(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		opts := options.Find().
			SetSort(bson.D{{Key: "generatedAt", Value: -1}}).
			SetLimit(50).
			SetProjection(bson.D{
				{Key: "rawText", Value: 0},
				{Key: "threads.selfText", Value: 0},
			})

		cursor, err := mongoDB.RedditAnalyses().Find(ctx, tenantFilter(r.Context(), bson.D{}), opts)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		var results []bson.M
		if err := cursor.All(ctx, &results); err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		var summaries []map[string]any
		for _, r := range results {
			summary := map[string]any{
				"id":           r["_id"],
				"domain":       r["domain"],
				"model":        r["model"],
				"generated_at": r["generatedAt"],
			}
			if res, ok := r["result"].(bson.M); ok {
				if score, ok := res["overallScore"]; ok {
					summary["overall_score"] = score
				}
			}
			if threads, ok := r["threads"].(bson.A); ok {
				summary["thread_count"] = len(threads)
			} else {
				summary["thread_count"] = 0
			}
			summaries = append(summaries, summary)
		}

		if summaries == nil {
			summaries = []map[string]any{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(summaries)
	}
}

func handleDeleteRedditAnalysis(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		delFilter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		result, err := mongoDB.RedditAnalyses().DeleteOne(ctx, delFilter)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"deleted": result.DeletedCount > 0,
		})
	}
}

// ============================================================
// Search Visibility Analysis
// ============================================================

func buildSearchVisibilityPrompt(domain string, brandInfo BrandContextInfo, competitors []string) string {
	var sb strings.Builder
	brandName := domain
	if brandInfo.Used {
		for _, line := range strings.Split(brandInfo.ContextString, "\n") {
			if strings.HasPrefix(line, "Company: ") {
				brandName = strings.TrimPrefix(line, "Company: ")
				break
			}
		}
	}

	sb.WriteString(fmt.Sprintf(`You are an expert at analyzing search visibility signals that affect whether AI systems — including Google AI Overviews, ChatGPT, Claude, Gemini, and Perplexity — will discover, index, and cite a website's content.

## Context

**Target Brand**: %s (domain: %s)
`, brandName, domain))

	if brandInfo.Used {
		sb.WriteString(fmt.Sprintf("\n**Brand Intelligence**:\n%s\n", brandInfo.ContextString))
	}

	if len(competitors) > 0 {
		sb.WriteString(fmt.Sprintf("\n**Key Competitors**: %s\n", strings.Join(competitors, ", ")))
	}

	sb.WriteString(fmt.Sprintf(`
## Research Context

Search visibility affects AI citation through two distinct pathways:

**Google AI Overviews (strong SEO correlation):**
- 76%% of AI Overview citations pull from top-10 organic pages (Ahrefs, 1.9M citations study)
- Being cited in an AIO increases CTR by 80%%+ for that site
- Reddit (40.1%%) and Wikipedia (26.3%%) dominate AIO citations
- 82%% of AIOs appear for keywords with <1,000 monthly searches (long-tail dominated)

**Standalone LLMs (weak SEO correlation):**
- Only 12%% of ChatGPT/Claude/Gemini cited URLs rank in Google's top 10
- Brand web mentions (0.664 correlation) and YouTube mentions (0.737) are stronger predictors than backlinks (0.37)
- 28%% of ChatGPT's top-cited pages have zero Google organic visibility

**Crawl Accessibility:**
- GPTBot grew 305%% YoY; OpenAI's crawl-to-referral ratio is 1,700:1
- OpenAI, Anthropic, and Perplexity each now run 3 separate bots: training, indexing, and user-fetch
- Blocking training bots while allowing search bots is a valid strategy; blocking everything hurts AI visibility
- ~21%% of top-1000 sites block GPTBot

**Content Freshness:**
- AI assistants cite content that is 25.7%% newer than traditional Google results (Ahrefs, 17M citations)
- 65%% of AI bot crawl hits target content published within the past year
- ChatGPT, Perplexity strongly favor recent content; Google AIOs actually prefer older authoritative content

**Brand Search Momentum:**
- Brand search volume: 0.334 correlation with AI citation frequency
- Winner-takes-all: brands in top 25%% for web mentions average 169 AI Overview mentions vs 14 for the 50th-75th percentile (12x gap)

## Your Task

Using the web_search tool, conduct a comprehensive search visibility analysis of %s. You must:

1. **Visit the site's robots.txt** — search for "%s robots.txt" and/or try to access it directly. Identify which AI crawlers are explicitly allowed or disallowed (GPTBot, ClaudeBot, PerplexityBot, Googlebot, and their SearchBot/User variants).

2. **Check for structured data** — Visit key pages of the site and check for Schema.org markup (JSON-LD), OpenGraph tags, structured content (tables, FAQ sections, comparison formats).

3. **Assess organic search presence** — Search for the brand name and key product/service terms. How prominently does the site appear in search results? How well does it rank for relevant informational queries?

4. **Evaluate content freshness** — Check publication dates, last-updated indicators, blog/content publishing frequency. Are key pages being regularly updated?

5. **Assess brand search momentum** — Search for the brand name to gauge how well-known it is. Look for web mentions, reviews, news coverage. Compare against any competitors.

6. **Check sitemap** — Search for the site's sitemap.xml and assess its completeness.

Return your analysis as a JSON object with this exact structure:

{
  "overall_score": <0-100 integer>,
  "aio_readiness": {
    "score": <0-100 integer, weighted 30%%>,
    "evidence": ["specific finding 1", "specific finding 2", ...],
    "organic_presence": <0-100>,
    "structured_data": <0-100>,
    "content_format": <0-100>,
    "answer_prominence": <0-100>
  },
  "crawl_accessibility": {
    "score": <0-100 integer, weighted 20%%>,
    "evidence": ["specific finding 1", "specific finding 2", ...],
    "robots_txt_policy": "<summary of robots.txt AI crawler policy>",
    "ai_bot_access": <0-100>,
    "sitemap_quality": <0-100>,
    "render_accessibility": <0-100>,
    "crawler_details": [
      {"name": "GPTBot", "allowed": true/false, "notes": "..."},
      {"name": "ClaudeBot", "allowed": true/false, "notes": "..."},
      {"name": "PerplexityBot", "allowed": true/false, "notes": "..."},
      {"name": "Google-Extended", "allowed": true/false, "notes": "..."},
      {"name": "Googlebot", "allowed": true/false, "notes": "..."}
    ]
  },
  "brand_momentum": {
    "score": <0-100 integer, weighted 25%%>,
    "evidence": ["specific finding 1", "specific finding 2", ...],
    "brand_search_trend": "growing" | "stable" | "declining",
    "competitor_compare": "<narrative comparison>",
    "web_mention_strength": <0-100>,
    "entity_recognition": <0-100>
  },
  "content_freshness": {
    "score": <0-100 integer, weighted 25%%>,
    "evidence": ["specific finding 1", "specific finding 2", ...],
    "average_content_age": "<narrative description>",
    "update_frequency": "frequent" | "moderate" | "infrequent" | "stale",
    "freshness_signals": <0-100>,
    "content_decay_risk": <0-100>
  },
  "executive_summary": "<2-3 paragraph summary of search visibility posture and its implications for AI citation>",
  "confidence_note": "<brief note on data limitations>",
  "recommendations": [
    {
      "action": "<specific action to take>",
      "priority": "high" | "medium" | "low",
      "expected_impact": "<what improvement to expect>",
      "dimension": "<which pillar this addresses>"
    }
  ]
}

IMPORTANT:
- The overall_score should be a weighted average: AIO Readiness 30%%, Crawl Accessibility 20%%, Brand Momentum 25%%, Content Freshness 25%%
- Provide 3-6 evidence items per pillar with specific, verifiable findings
- Include 4-8 prioritized recommendations
- Be specific and cite actual findings from your searches
- Return ONLY the JSON object, no markdown fencing or explanation
`, domain, domain))

	return sb.String()
}

func handleSearchAnalyze(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		provider, apiKey, _, err := resolvePrimaryLLM(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Configure an API key in Settings", "code": "api_key_required"})
			return
		}

		var req struct {
			Domain string `json:"domain"`
			Force  bool   `json:"force"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Invalid request body"})
			return
		}
		if req.Domain == "" {
			sendSSE(w, flusher, "error", map[string]string{"message": "Domain is required"})
			return
		}
		req.Domain = normalizeDomain(req.Domain)

		// Check for cached result (30-day TTL)
		if !req.Force {
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			var existing SearchAnalysis
			err := mongoDB.SearchAnalyses().FindOne(ctx,
				tenantFilter(r.Context(), bson.D{
					{Key: "domain", Value: req.Domain},
					{Key: "generatedAt", Value: bson.D{{Key: "$gt", Value: time.Now().AddDate(0, 0, -30)}}},
				}),
			).Decode(&existing)
			cancel()
			if err == nil && existing.Result != nil {
				resultMap := map[string]any{
					"domain":             existing.Domain,
					"result":             existing.Result,
					"model":              existing.Model,
					"brand_context_used": existing.BrandContextUsed,
					"generated_at":       existing.GeneratedAt,
				}
				frontendJSON, _ := json.Marshal(resultMap)
				sendSSE(w, flusher, "done", map[string]any{
					"result": string(frontendJSON),
					"cached": true,
				})
				return
			}
		}

		brandInfo := lookupBrandContext(mongoDB, req.Domain, saas.TenantIDFromContext(r.Context()))

		// Extract competitor names
		var competitorNames []string
		if brandInfo.Used {
			cCtx, cCancel := context.WithTimeout(r.Context(), 5*time.Second)
			var brand BrandProfile
			if err := mongoDB.BrandProfiles().FindOne(cCtx, tenantFilter(r.Context(), bson.D{{Key: "domain", Value: req.Domain}})).Decode(&brand); err == nil {
				for _, c := range brand.Competitors {
					competitorNames = append(competitorNames, c.Name)
				}
			}
			cCancel()
		}

		sendSSE(w, flusher, "status", map[string]string{
			"message": "Analyzing search visibility signals...",
		})

		prompt := buildSearchVisibilityPrompt(req.Domain, brandInfo, competitorNames)

		// Model fallback chain
		usedModel := ""
		models := provider.Models()

		var resultJSON string
		for mi, model := range models {
			if mi > 0 {
				sendSSE(w, flusher, "status", map[string]string{
					"message": fmt.Sprintf("%s unavailable, falling back to %s...", models[mi-1].Name, model.Name),
				})
			}

			claudeBody, _ := provider.BuildStreamBody(model.ID, 65536, prompt, true)

			const maxRetries = 3
			backoff := 2 * time.Second
			var lastErr error

			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					sendSSE(w, flusher, "status", map[string]string{
						"message": fmt.Sprintf("%s overloaded, retrying in %ds (attempt %d/%d)...", model.Name, int(backoff.Seconds()), attempt, maxRetries),
					})
					select {
					case <-time.After(backoff):
					case <-r.Context().Done():
						sendSSE(w, flusher, "error", map[string]string{"message": "Request cancelled"})
						return
					}
					backoff *= 2
				}

				result, err := provider.Stream(r.Context(), apiKey, claudeBody, w, flusher)
				if err == errOverloaded {
					lastErr = err
					if attempt < maxRetries {
						continue
					}
					break
				}
				if err != nil {
					sendSSE(w, flusher, "error", map[string]string{"message": "Analysis failed: " + err.Error()})
					return
				}

				resultJSON = result.ResultJSON
				usedModel = model.Name
				goto done
			}

			log.Printf("%s API (%s) exhausted retries: %v", provider.Name(), model.ID, lastErr)
		}

		if usedModel == "" {
			sendSSE(w, flusher, "error", map[string]string{"message": "All models overloaded"})
			return
		}

	done:
		resultJSON = stripJSONFencing(resultJSON)
		var result SearchVisibilityResult
		if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
			log.Printf("Warning: failed to parse search visibility result: %v", err)
			sendSSE(w, flusher, "error", map[string]string{"message": "Failed to parse analysis result"})
			return
		}

		sendSSE(w, flusher, "status", map[string]string{
			"message": fmt.Sprintf("Analysis complete — Overall Score: %d/100", result.OverallScore),
		})

		// Save results
		analysis := SearchAnalysis{
			Domain:           req.Domain,
			TenantID:         saas.TenantIDFromContext(r.Context()),
			Result:           &result,
			RawText:          resultJSON,
			Model:            usedModel,
			BrandContextUsed: brandInfo.Used,
			GeneratedAt:      time.Now(),
		}

		saveCtx, saveCancel := context.WithTimeout(r.Context(), 10*time.Second)
		_, saveErr := mongoDB.SearchAnalyses().ReplaceOne(saveCtx,
			tenantFilter(r.Context(), bson.D{{Key: "domain", Value: req.Domain}}),
			analysis,
			options.Replace().SetUpsert(true),
		)
		saveCancel()
		if saveErr != nil {
			log.Printf("Failed to save search analysis: %v", saveErr)
		}

		// Create todos from recommendations
		if saveErr == nil && len(result.Recommendations) > 0 {
			go createTodosFromSearchAnalysis(mongoDB, req.Domain, saas.TenantIDFromContext(r.Context()), result.Recommendations)
		}

		// Build result for frontend
		resultMap := map[string]any{
			"domain":             req.Domain,
			"result":             &result,
			"model":              usedModel,
			"brand_context_used": brandInfo.Used,
			"generated_at":       analysis.GeneratedAt,
		}

		frontendJSON, _ := json.Marshal(resultMap)
		sendSSE(w, flusher, "done", map[string]any{
			"result": string(frontendJSON),
		})
	}
}

func createTodosFromSearchAnalysis(mongoDB *MongoDB, domain, tenantID string, recommendations []SearchRecommendation) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, rec := range recommendations {
		if rec.Priority != "high" && rec.Priority != "medium" {
			continue
		}
		todo := TodoItem{
			TenantID:       tenantID,
			SourceType:     "search",
			Domain:         domain,
			Question:       "Search Visibility",
			Action:         rec.Action,
			Summary:        rec.Action,
			ExpectedImpact: rec.ExpectedImpact,
			Dimension:      rec.Dimension,
			Priority:       rec.Priority,
			Status:         "todo",
			CreatedAt:      time.Now(),
		}
		if _, err := mongoDB.Todos().InsertOne(ctx, todo); err != nil {
			log.Printf("Failed to create search todo: %v", err)
		}
	}

	// Deduplicate todos for this domain
	go deduplicateTodos(mongoDB, domain, tenantID)
}

func handleGetSearchAnalysis(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		var analysis SearchAnalysis
		err := mongoDB.SearchAnalyses().FindOne(ctx,
			tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}}),
		).Decode(&analysis)
		if err != nil {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(analysis)
	}
}

func handleListSearchAnalyses(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		filter := tenantFilter(r.Context(), bson.D{})
		opts := options.Find().
			SetSort(bson.D{{Key: "generatedAt", Value: -1}}).
			SetProjection(bson.D{
				{Key: "domain", Value: 1},
				{Key: "result.overall_score", Value: 1},
				{Key: "model", Value: 1},
				{Key: "generatedAt", Value: 1},
			})

		cursor, err := mongoDB.SearchAnalyses().Find(ctx, filter, opts)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)

		var summaries []SearchAnalysisSummary
		for cursor.Next(ctx) {
			var doc struct {
				ID          primitive.ObjectID `bson:"_id"`
				Domain      string             `bson:"domain"`
				Result      *struct {
					OverallScore int `bson:"overallScore"`
				} `bson:"result"`
				Model       string    `bson:"model"`
				GeneratedAt time.Time `bson:"generatedAt"`
			}
			if err := cursor.Decode(&doc); err != nil {
				continue
			}
			s := SearchAnalysisSummary{
				ID:          doc.ID,
				Domain:      doc.Domain,
				Model:       doc.Model,
				GeneratedAt: doc.GeneratedAt,
			}
			if doc.Result != nil {
				score := doc.Result.OverallScore
				s.OverallScore = &score
			}
			summaries = append(summaries, s)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(summaries)
	}
}

func handleDeleteSearchAnalysis(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		delFilter := tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}})
		result, err := mongoDB.SearchAnalyses().DeleteOne(ctx, delFilter)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"deleted": result.DeletedCount > 0,
		})
	}
}

// BrandContextInfo holds brand context lookup results.
type BrandContextInfo struct {
	Used             bool
	ProfileUpdatedAt *time.Time
	ContextString    string
}

// lookupBrandContext returns brand context for use in prompts. If no profile exists, returns empty info.
// tenantID is optional — when non-empty, filters brand profiles by tenant.
func lookupBrandContext(mongoDB *MongoDB, domain, tenantID string) BrandContextInfo {
	domain = normalizeDomain(domain)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.D{{Key: "domain", Value: domain}}
	if tenantID != "" {
		filter = append(filter, bson.E{Key: "tenantId", Value: tenantID})
	}
	var brand BrandProfile
	err := mongoDB.BrandProfiles().FindOne(ctx, filter).Decode(&brand)
	if err != nil {
		return BrandContextInfo{}
	}

	var parts []string
	if brand.BrandName != "" {
		parts = append(parts, fmt.Sprintf("Company: %s", brand.BrandName))
	}
	if brand.Description != "" {
		parts = append(parts, fmt.Sprintf("Description: %s", brand.Description))
	}
	if len(brand.Categories) > 0 {
		parts = append(parts, fmt.Sprintf("Categories: %s", strings.Join(brand.Categories, ", ")))
	}
	if len(brand.Products) > 0 {
		parts = append(parts, fmt.Sprintf("Products/Features: %s", strings.Join(brand.Products, ", ")))
	}
	if brand.PrimaryAudience != "" {
		parts = append(parts, fmt.Sprintf("Target Audience: %s", brand.PrimaryAudience))
	}
	if len(brand.KeyUseCases) > 0 {
		parts = append(parts, fmt.Sprintf("Key Use Cases: %s", strings.Join(brand.KeyUseCases, ", ")))
	}
	if len(brand.Competitors) > 0 {
		names := make([]string, len(brand.Competitors))
		for i, c := range brand.Competitors {
			names[i] = c.Name
		}
		parts = append(parts, fmt.Sprintf("Known Competitors: %s", strings.Join(names, ", ")))
	}
	if len(brand.Differentiators) > 0 {
		parts = append(parts, fmt.Sprintf("Key Differentiators: %s", strings.Join(brand.Differentiators, ", ")))
	}
	if len(brand.KeyMessages) > 0 {
		claims := make([]string, len(brand.KeyMessages))
		for i, m := range brand.KeyMessages {
			if m.EvidenceURL != "" {
				claims[i] = fmt.Sprintf("%s (evidence: %s) [%s]", m.Claim, m.EvidenceURL, m.Priority)
			} else {
				claims[i] = fmt.Sprintf("%s [%s]", m.Claim, m.Priority)
			}
		}
		parts = append(parts, fmt.Sprintf("Brand Claims (claims the brand aspires to make):\n- %s", strings.Join(claims, "\n- ")))
	}
	if len(brand.TargetQueries) > 0 {
		tqs := make([]string, len(brand.TargetQueries))
		for i, tq := range brand.TargetQueries {
			tqs[i] = fmt.Sprintf("%s [%s, %s]", tq.Query, tq.Priority, tq.Type)
		}
		parts = append(parts, fmt.Sprintf("Target Queries (questions the brand wants to be found for):\n- %s", strings.Join(tqs, "\n- ")))
	}

	updatedAt := brand.UpdatedAt
	if len(parts) == 0 {
		return BrandContextInfo{ProfileUpdatedAt: &updatedAt}
	}

	contextStr := fmt.Sprintf("\n\n--- Brand Intelligence Context for %s ---\n%s\n--- End Brand Context ---\n", domain, strings.Join(parts, "\n"))
	return BrandContextInfo{
		Used:             true,
		ProfileUpdatedAt: &updatedAt,
		ContextString:    contextStr,
	}
}

// computeBrandCompleteness calculates a 0-100 completeness score for a brand profile.
func computeBrandCompleteness(p BrandProfile) int {
	score := 0
	if p.BrandName != "" {
		score += 8
	}
	if p.Description != "" {
		score += 10
	}
	if len(p.Categories) > 0 {
		score += 6
	}
	if len(p.Products) > 0 {
		score += 6
	}
	if p.PrimaryAudience != "" {
		score += 8
	}
	if len(p.KeyUseCases) > 0 {
		score += 7
	}
	if len(p.Competitors) > 0 {
		score += 15
	}
	if len(p.Competitors) >= 3 {
		score += 10
	}
	if len(p.TargetQueries) > 0 {
		score += 8
	}
	if len(p.TargetQueries) >= 5 {
		score += 4
	}
	if len(p.KeyMessages) > 0 {
		score += 4
	}
	if len(p.Differentiators) > 0 {
		score += 4
	}
	if p.PresenceComplete || p.Presence.YouTubeURL != "" || len(p.Presence.Subreddits) > 0 || len(p.Presence.ReviewSiteURLs) > 0 {
		score += 10
	}
	if score > 100 {
		score = 100
	}
	return score
}

func extractJSON(text string) string {
	text = strings.TrimSpace(text)
	if json.Valid([]byte(text)) {
		return text
	}

	// Check for ```json code blocks
	if idx := strings.Index(text, "```json"); idx != -1 {
		start := idx + 7
		if end := strings.Index(text[start:], "```"); end != -1 {
			candidate := strings.TrimSpace(text[start : start+end])
			if json.Valid([]byte(candidate)) {
				return candidate
			}
		}
	}

	// Check for ``` code blocks
	if idx := strings.Index(text, "```"); idx != -1 {
		start := idx + 3
		if nl := strings.Index(text[start:], "\n"); nl != -1 {
			start += nl + 1
		}
		if end := strings.Index(text[start:], "```"); end != -1 {
			candidate := strings.TrimSpace(text[start : start+end])
			if json.Valid([]byte(candidate)) {
				return candidate
			}
		}
	}

	// Find outermost { }
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start != -1 && end > start {
		candidate := text[start : end+1]
		if json.Valid([]byte(candidate)) {
			return candidate
		}
	}

	return text
}

// ==================== LLM Test Handlers ====================

type testRawResponse struct {
	providerID   string
	providerName string
	model        string
	queryIdx     int
	response     string
	err          error
}

func handleGenerateTestQueries(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Domain string `json:"domain"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		req.Domain = normalizeDomain(req.Domain)
		if req.Domain == "" {
			http.Error(w, `{"error":"domain is required"}`, http.StatusBadRequest)
			return
		}

		tenantID := saas.TenantIDFromContext(r.Context())
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		filter := bson.D{{Key: "domain", Value: req.Domain}}
		if tenantID != "" {
			filter = append(filter, bson.E{Key: "tenantId", Value: tenantID})
		}
		var brand BrandProfile
		err := mongoDB.BrandProfiles().FindOne(ctx, filter).Decode(&brand)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"queries":    []LLMTestQuery{},
				"brand_name": req.Domain,
			})
			return
		}

		var queries []LLMTestQuery

		// Use existing target queries from brand profile
		for _, tq := range brand.TargetQueries {
			queries = append(queries, LLMTestQuery{
				Query:    tq.Query,
				Type:     tq.Type,
				Priority: tq.Priority,
			})
		}

		brandName := brand.BrandName
		if brandName == "" {
			brandName = req.Domain
		}

		// Auto-generate additional queries from brand profile fields
		seen := map[string]bool{}
		for _, q := range queries {
			seen[strings.ToLower(q.Query)] = true
		}
		addQuery := func(query, qType, priority string) {
			if len(queries) >= 12 {
				return
			}
			lower := strings.ToLower(query)
			if seen[lower] {
				return
			}
			seen[lower] = true
			queries = append(queries, LLMTestQuery{Query: query, Type: qType, Priority: priority})
		}

		// Brand awareness (always)
		if len(queries) == 0 {
			addQuery(fmt.Sprintf("What is %s?", brandName), "brand", "high")
		}

		// Category discovery
		for i, cat := range brand.Categories {
			if i >= 2 {
				break
			}
			addQuery(fmt.Sprintf("What are the best %s tools?", cat), "category", "high")
		}

		// Product-specific queries
		for i, prod := range brand.Products {
			if i >= 2 {
				break
			}
			addQuery(fmt.Sprintf("What is %s?", prod), "brand", "medium")
		}

		// Audience-targeted queries
		if brand.PrimaryAudience != "" && len(brand.Categories) > 0 {
			addQuery(fmt.Sprintf("Best %s for %s", brand.Categories[0], strings.ToLower(brand.PrimaryAudience)), "category", "medium")
		}

		// Competitor comparisons
		for i, comp := range brand.Competitors {
			if i >= 2 {
				break
			}
			addQuery(fmt.Sprintf("How does %s compare to %s?", brandName, comp.Name), "comparison", "medium")
		}

		// Use case / discovery queries
		for i, uc := range brand.KeyUseCases {
			if i >= 2 {
				break
			}
			addQuery(fmt.Sprintf("How do I %s?", strings.ToLower(uc)), "discovery", "medium")
		}

		// Key message verification
		for i, km := range brand.KeyMessages {
			if i >= 1 {
				break
			}
			addQuery(fmt.Sprintf("Is it true that %s?", strings.ToLower(km.Claim)), "brand", "low")
		}

		// Differentiator queries
		if len(brand.Differentiators) > 0 {
			addQuery(fmt.Sprintf("What makes %s different from competitors?", brandName), "comparison", "medium")
		}

		// Reddit presence queries
		if len(brand.Presence.Subreddits) > 0 {
			addQuery(fmt.Sprintf("What does Reddit say about %s?", brandName), "discovery", "low")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"queries":    queries,
			"brand_name": brandName,
		})
	}
}

func handleListProviderModels() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		type modelInfo struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		type providerModels struct {
			ID     string      `json:"id"`
			Name   string      `json:"name"`
			Models []modelInfo `json:"models"`
		}
		var result []providerModels
		for _, pid := range validProviderIDs() {
			p := getProvider(pid)
			if p == nil {
				continue
			}
			var models []modelInfo
			for _, m := range p.Models() {
				models = append(models, modelInfo{ID: m.ID, Name: m.Name})
			}
			result = append(result, providerModels{ID: pid, Name: p.Name(), Models: models})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func handleGetLLMTest(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		if domain == "" {
			http.Error(w, `{"error":"domain is required"}`, http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var test LLMTest
		err := mongoDB.LLMTests().FindOne(ctx,
			tenantFilter(r.Context(), bson.D{
				{Key: "domain", Value: domain},
				{Key: "competitorOf", Value: bson.D{{Key: "$in", Value: bson.A{"", nil}}}},
			}),
			options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}),
		).Decode(&test)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"error":"not found"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(test)
	}
}

func handleGetLLMTestHistory(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		if domain == "" {
			http.Error(w, `{"error":"domain is required"}`, http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Return all test runs (excluding competitor tests), sorted newest first, limit 20
		// Project out full response text for compactness
		cursor, err := mongoDB.LLMTests().Find(ctx,
			tenantFilter(r.Context(), bson.D{
				{Key: "domain", Value: domain},
				{Key: "competitorOf", Value: bson.D{{Key: "$in", Value: bson.A{"", nil}}}},
			}),
			options.Find().
				SetSort(bson.D{{Key: "generatedAt", Value: -1}}).
				SetLimit(20).
				SetProjection(bson.D{
					{Key: "results.provider_results.response", Value: 0},
				}),
		)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		var tests []LLMTest
		if err := cursor.All(ctx, &tests); err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tests)
	}
}

func handleGetCompetitorTests(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		if domain == "" {
			http.Error(w, `{"error":"domain is required"}`, http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Find all competitor tests where competitorOf matches the primary domain
		// Return only the most recent test per competitor domain
		pipeline := mongo.Pipeline{
			{{Key: "$match", Value: bson.D{
				{Key: "competitorOf", Value: domain},
				{Key: "tenantId", Value: saas.TenantIDFromContext(r.Context())},
			}}},
			{{Key: "$sort", Value: bson.D{{Key: "generatedAt", Value: -1}}}},
			{{Key: "$group", Value: bson.D{
				{Key: "_id", Value: "$domain"},
				{Key: "doc", Value: bson.D{{Key: "$first", Value: "$$ROOT"}}},
			}}},
			{{Key: "$replaceRoot", Value: bson.D{{Key: "newRoot", Value: "$doc"}}}},
			{{Key: "$project", Value: bson.D{
				{Key: "results.provider_results.response", Value: 0},
			}}},
		}

		cursor, err := mongoDB.LLMTests().Aggregate(ctx, pipeline)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		var tests []LLMTest
		if err := cursor.All(ctx, &tests); err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tests)
	}
}

func handleDeleteLLMTest(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		if domain == "" {
			http.Error(w, `{"error":"domain is required"}`, http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		_, err := mongoDB.LLMTests().DeleteMany(ctx,
			tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}}),
		)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}
}

func handleLLMTest(mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		var req struct {
			Domain       string            `json:"domain"`
			Providers    []string          `json:"providers"`
			Queries      []LLMTestQuery    `json:"queries"`
			Models       map[string]string `json:"models"`        // providerID → modelID override
			CompetitorOf string            `json:"competitor_of"` // if set, this is a competitor test for the given primary domain
			Force        bool              `json:"force"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Invalid request body"})
			return
		}
		req.Domain = normalizeDomain(req.Domain)
		if req.Domain == "" {
			sendSSE(w, flusher, "error", map[string]string{"message": "Domain is required"})
			return
		}
		if len(req.Providers) == 0 {
			sendSSE(w, flusher, "error", map[string]string{"message": "At least one provider is required"})
			return
		}
		if len(req.Queries) == 0 {
			sendSSE(w, flusher, "error", map[string]string{"message": "At least one query is required"})
			return
		}

		// Validate providers and resolve API keys
		type providerWithKey struct {
			provider LLMProvider
			apiKey   string
		}
		var providerKeys []providerWithKey
		for _, pid := range req.Providers {
			p := getProvider(pid)
			if p == nil {
				sendSSE(w, flusher, "error", map[string]string{"message": fmt.Sprintf("Unknown provider: %s", pid)})
				return
			}
			key, err := resolveProviderKey(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled, pid)
			if err != nil {
				sendSSE(w, flusher, "error", map[string]string{"message": fmt.Sprintf("No API key configured for %s", p.Name())})
				return
			}
			providerKeys = append(providerKeys, providerWithKey{provider: p, apiKey: key})
		}

		// Look up brand context
		brandInfo := lookupBrandContext(mongoDB, req.Domain, saas.TenantIDFromContext(r.Context()))
		brandName := req.Domain
		if brandInfo.Used {
			for _, line := range strings.Split(brandInfo.ContextString, "\n") {
				if strings.HasPrefix(line, "Company: ") {
					brandName = strings.TrimPrefix(line, "Company: ")
					break
				}
			}
		}

		totalCalls := len(req.Providers) * len(req.Queries)
		completedCalls := 0

		sendSSE(w, flusher, "status", map[string]string{
			"message": fmt.Sprintf("Testing %d queries across %d providers (%d total calls)...", len(req.Queries), len(req.Providers), totalCalls),
		})

		// Phase 1: Query each provider with each query
		responses := make([]testRawResponse, 0, totalCalls)

		for _, pk := range providerKeys {
			// Use model override if specified, otherwise use primary model
			model := pk.provider.Models()[0]
			if overrideID, ok := req.Models[pk.provider.ProviderID()]; ok && overrideID != "" {
				for _, m := range pk.provider.Models() {
					if m.ID == overrideID {
						model = m
						break
					}
				}
			}
			for qi, q := range req.Queries {
				completedCalls++
				sendSSE(w, flusher, "status", map[string]string{
					"message": fmt.Sprintf("[%d/%d] Querying %s (%s): \"%s\"...", completedCalls, totalCalls, pk.provider.Name(), model.Name, truncateStr(q.Query, 60)),
				})

				resp, err := pk.provider.Call(r.Context(), pk.apiKey, model.ID, q.Query, 4096)
				responses = append(responses, testRawResponse{
					providerID:   pk.provider.ProviderID(),
					providerName: pk.provider.Name(),
					model:        model.Name,
					queryIdx:     qi,
					response:     resp,
					err:          err,
				})
			}
		}

		// Phase 2: Evaluate all responses using the primary LLM
		sendSSE(w, flusher, "status", map[string]string{
			"message": "Evaluating responses against brand profile...",
		})

		primaryProvider, primaryKey, _, err := resolvePrimaryLLM(r.Context(), mongoDB, encKey, fallbackKey, saasEnabled)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Could not resolve primary LLM for evaluation"})
			return
		}

		// Build evaluation prompt
		evalPrompt := buildTestEvaluationPrompt(brandName, req.Domain, brandInfo, req.Queries, responses)

		evalModel := primaryProvider.Models()[0].ID
		evalResp, err := primaryProvider.Call(r.Context(), primaryKey, evalModel, evalPrompt, 16384)
		if err != nil {
			sendSSE(w, flusher, "error", map[string]string{"message": "Evaluation failed: " + err.Error()})
			return
		}

		// Parse evaluation results
		evalJSON := stripJSONFencing(evalResp)
		var evalResult struct {
			Evaluations []struct {
				QueryIndex  int    `json:"query_index"`
				ProviderID  string `json:"provider_id"`
				Mentioned   bool   `json:"mentioned"`
				Recommended bool   `json:"recommended"`
				Sentiment   string `json:"sentiment"`
				Accuracy    string `json:"accuracy"`
				Score       int    `json:"score"`
			} `json:"evaluations"`
		}
		if err := json.Unmarshal([]byte(evalJSON), &evalResult); err != nil {
			log.Printf("Failed to parse evaluation result: %v — raw: %s", err, evalJSON[:min(200, len(evalJSON))])
			sendSSE(w, flusher, "error", map[string]string{"message": "Failed to parse evaluation results"})
			return
		}

		// Build structured results
		queryResults := make([]LLMTestQueryResult, len(req.Queries))
		for qi, q := range req.Queries {
			queryResults[qi] = LLMTestQueryResult{
				Query:           q,
				ProviderResults: []LLMTestProviderResult{},
			}
		}

		// Map responses into query results
		for _, resp := range responses {
			pr := LLMTestProviderResult{
				ProviderID:   resp.providerID,
				ProviderName: resp.providerName,
				Model:        resp.model,
				Response:     resp.response,
			}
			if resp.err != nil {
				pr.Response = fmt.Sprintf("Error: %s", resp.err.Error())
				pr.Sentiment = "absent"
				pr.Accuracy = "not_applicable"
			}
			queryResults[resp.queryIdx].ProviderResults = append(queryResults[resp.queryIdx].ProviderResults, pr)
		}

		// Apply evaluation scores to results
		for _, eval := range evalResult.Evaluations {
			if eval.QueryIndex < 0 || eval.QueryIndex >= len(queryResults) {
				continue
			}
			qr := &queryResults[eval.QueryIndex]
			for i := range qr.ProviderResults {
				if qr.ProviderResults[i].ProviderID == eval.ProviderID {
					qr.ProviderResults[i].Mentioned = eval.Mentioned
					qr.ProviderResults[i].Recommended = eval.Recommended
					qr.ProviderResults[i].Sentiment = eval.Sentiment
					qr.ProviderResults[i].Accuracy = eval.Accuracy
					qr.ProviderResults[i].Score = eval.Score
					break
				}
			}
		}

		// Compute per-provider summaries
		summaryMap := map[string]*LLMTestSummary{}
		for _, pk := range providerKeys {
			modelName := pk.provider.Models()[0].Name
			if overrideID, ok := req.Models[pk.provider.ProviderID()]; ok && overrideID != "" {
				for _, m := range pk.provider.Models() {
					if m.ID == overrideID {
						modelName = m.Name
						break
					}
				}
			}
			summaryMap[pk.provider.ProviderID()] = &LLMTestSummary{
				ProviderID:   pk.provider.ProviderID(),
				ProviderName: pk.provider.Name(),
				Model:        modelName,
			}
		}

		for _, qr := range queryResults {
			for _, pr := range qr.ProviderResults {
				s := summaryMap[pr.ProviderID]
				if s == nil {
					continue
				}
				s.OverallScore += pr.Score
				if pr.Mentioned {
					s.MentionRate++
				}
				if pr.Recommended {
					s.RecommendRate++
				}
				if pr.Accuracy == "accurate" {
					s.AccuracyRate++
				}
				switch pr.Sentiment {
				case "positive":
					s.SentimentScore += 100
				case "neutral":
					s.SentimentScore += 50
				case "negative":
					s.SentimentScore += 10
				}
			}
		}

		numQueries := len(req.Queries)
		var summaries []LLMTestSummary
		overallTotal := 0
		for _, pid := range req.Providers {
			s := summaryMap[pid]
			if numQueries > 0 {
				s.OverallScore = s.OverallScore / numQueries
				s.MentionRate = s.MentionRate * 100 / numQueries
				s.RecommendRate = s.RecommendRate * 100 / numQueries
				s.AccuracyRate = s.AccuracyRate * 100 / numQueries
				s.SentimentScore = s.SentimentScore / numQueries
			}
			overallTotal += s.OverallScore
			summaries = append(summaries, *s)
		}

		overallScore := 0
		if len(summaries) > 0 {
			overallScore = overallTotal / len(summaries)
		}

		// Determine run number
		runNumber := 1
		runCtx, runCancel := context.WithTimeout(r.Context(), 5*time.Second)
		var prevTest LLMTest
		if err := mongoDB.LLMTests().FindOne(runCtx,
			tenantFilter(r.Context(), bson.D{{Key: "domain", Value: req.Domain}}),
			options.FindOne().SetSort(bson.D{{Key: "runNumber", Value: -1}}),
		).Decode(&prevTest); err == nil {
			runNumber = prevTest.RunNumber + 1
		}
		runCancel()

		test := LLMTest{
			TenantID:          saas.TenantIDFromContext(r.Context()),
			Domain:            req.Domain,
			BrandName:         brandName,
			RunNumber:         runNumber,
			CompetitorOf:      normalizeDomain(req.CompetitorOf),
			Queries:           req.Queries,
			Results:           queryResults,
			ProviderSummaries: summaries,
			OverallScore:      overallScore,
			BrandContextUsed:  brandInfo.Used,
			GeneratedAt:       time.Now(),
		}

		// Save to DB (insert new document for history tracking)
		saveCtx, saveCancel := context.WithTimeout(r.Context(), 10*time.Second)
		insertResult, saveErr := mongoDB.LLMTests().InsertOne(saveCtx, test)
		saveCancel()
		if saveErr != nil {
			log.Printf("Failed to save LLM test: %v", saveErr)
		} else if oid, ok := insertResult.InsertedID.(primitive.ObjectID); ok {
			test.ID = oid
		}

		sendSSE(w, flusher, "status", map[string]string{
			"message": fmt.Sprintf("Test complete — Overall Score: %d/100", overallScore),
		})

		testJSON, _ := json.Marshal(test)
		sendSSE(w, flusher, "done", map[string]any{
			"result": string(testJSON),
		})
	}
}

func handleVisibilityScore(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeDomain(r.PathValue("domain"))
		if domain == "" {
			http.Error(w, `{"error":"domain is required"}`, http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		tenantID := saas.TenantIDFromContext(r.Context())

		type component struct {
			Name      string  `json:"name"`
			Score     int     `json:"score"`
			Weight    float64 `json:"weight"`
			Available bool    `json:"available"`
		}

		components := []component{
			{Name: "Optimization", Weight: 0.30},
			{Name: "Video Authority", Weight: 0.20},
			{Name: "Reddit Authority", Weight: 0.20},
			{Name: "Search Visibility", Weight: 0.15},
			{Name: "LLM Test", Weight: 0.15},
		}

		// 1. Optimization average score
		cursor, err := mongoDB.Optimizations().Find(ctx,
			tenantFilter(r.Context(), bson.D{{Key: "domain", Value: domain}}),
			options.Find().SetProjection(bson.D{{Key: "result.overallScore", Value: 1}}),
		)
		if err == nil {
			var opts []struct {
				Result struct {
					OverallScore int `bson:"overallScore"`
				} `bson:"result"`
			}
			if cursor.All(ctx, &opts) == nil && len(opts) > 0 {
				total := 0
				for _, o := range opts {
					total += o.Result.OverallScore
				}
				components[0].Score = total / len(opts)
				components[0].Available = true
			}
		}

		// 2. Video authority (latest)
		var va struct {
			Result *struct {
				OverallScore int `bson:"overallScore"`
			} `bson:"result"`
		}
		if mongoDB.VideoAnalyses().FindOne(ctx,
			bson.D{{Key: "domain", Value: domain}, {Key: "tenantId", Value: tenantID}},
			options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "result.overallScore", Value: 1}}),
		).Decode(&va) == nil && va.Result != nil {
			components[1].Score = va.Result.OverallScore
			components[1].Available = true
		}

		// 3. Reddit authority (latest)
		var ra struct {
			Result *struct {
				OverallScore int `bson:"overallScore"`
			} `bson:"result"`
		}
		if mongoDB.RedditAnalyses().FindOne(ctx,
			bson.D{{Key: "domain", Value: domain}, {Key: "tenantId", Value: tenantID}},
			options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "result.overallScore", Value: 1}}),
		).Decode(&ra) == nil && ra.Result != nil {
			components[2].Score = ra.Result.OverallScore
			components[2].Available = true
		}

		// 4. Search visibility (latest)
		var sa struct {
			Result *struct {
				OverallScore int `bson:"overallScore"`
			} `bson:"result"`
		}
		if mongoDB.SearchAnalyses().FindOne(ctx,
			bson.D{{Key: "domain", Value: domain}, {Key: "tenantId", Value: tenantID}},
			options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "result.overallScore", Value: 1}}),
		).Decode(&sa) == nil && sa.Result != nil {
			components[3].Score = sa.Result.OverallScore
			components[3].Available = true
		}

		// 5. LLM Test (latest non-competitor)
		var lt struct {
			OverallScore int `bson:"overallScore"`
		}
		if mongoDB.LLMTests().FindOne(ctx,
			bson.D{
				{Key: "domain", Value: domain},
				{Key: "tenantId", Value: tenantID},
				{Key: "competitorOf", Value: bson.D{{Key: "$in", Value: bson.A{"", nil}}}},
			},
			options.FindOne().SetSort(bson.D{{Key: "generatedAt", Value: -1}}).SetProjection(bson.D{{Key: "overallScore", Value: 1}}),
		).Decode(&lt) == nil {
			components[4].Score = lt.OverallScore
			components[4].Available = true
		}

		// Compute weighted score (re-weight proportionally for available components only)
		totalWeight := 0.0
		weightedSum := 0.0
		for _, c := range components {
			if c.Available {
				totalWeight += c.Weight
				weightedSum += float64(c.Score) * c.Weight
			}
		}

		score := 0
		if totalWeight > 0 {
			score = int(weightedSum / totalWeight)
		}

		availableCount := 0
		for _, c := range components {
			if c.Available {
				availableCount++
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"score":      score,
			"components": components,
			"available":  availableCount,
			"total":      len(components),
		})
	}
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func buildTestEvaluationPrompt(brandName, domain string, brandInfo BrandContextInfo, queries []LLMTestQuery, responses []testRawResponse) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`You are evaluating how well different AI assistants represent a brand in their responses to user queries.

## Brand Information
**Brand**: %s
**Domain**: %s
`, brandName, domain))

	if brandInfo.Used {
		sb.WriteString(fmt.Sprintf("\n%s\n", brandInfo.ContextString))
	}

	sb.WriteString(`
## Instructions

For each query-response pair, evaluate:
1. **mentioned**: Is the brand explicitly mentioned by name? (boolean)
2. **recommended**: Is the brand recommended, suggested, or positioned as a good option? (boolean)
3. **sentiment**: Overall sentiment toward the brand in the response. Use "absent" if the brand isn't mentioned at all. (positive/neutral/negative/absent)
4. **accuracy**: Are factual claims about the brand accurate based on the brand information above? Use "not_applicable" if brand isn't discussed. (accurate/partially_accurate/inaccurate/not_applicable)
5. **score**: Overall brand representation quality 0-100. Consider: Is the brand mentioned? Positioned favorably? Information accurate? Recommended when relevant?

## Query-Response Pairs

`)

	for _, resp := range responses {
		if resp.err != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf(`### Query %d (asked to %s)
**Query**: %s
**Response**: %s

`, resp.queryIdx, resp.providerName, queries[resp.queryIdx].Query, resp.response))
	}

	sb.WriteString(`## Output Format

Return ONLY a JSON object with this exact structure:
{"evaluations": [{"query_index": 0, "provider_id": "anthropic", "mentioned": true, "recommended": false, "sentiment": "neutral", "accuracy": "accurate", "score": 65}, ...]}

Include one evaluation entry per query-response pair. Return ONLY the JSON object, no markdown fencing.`)

	return sb.String()
}
