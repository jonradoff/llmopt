package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ── handleAPIv1ListDomains ─────────────────────────────────────────────────────

func TestHandleAPIv1ListDomains_Empty(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1ListDomains(db)

	req := testRequest(t, "GET", "/api/v1/domains", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	domains, ok := resp["domains"].([]any)
	if !ok {
		t.Fatal("expected domains array")
	}
	if len(domains) != 0 {
		t.Errorf("expected empty domains, got %d", len(domains))
	}
}

func TestHandleAPIv1ListDomains_WithData(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	// Insert analyses for two different domains
	db.Analyses().InsertMany(ctx, []any{
		bson.M{"tenantId": "test-tenant", "domain": "example.com", "createdAt": time.Now()},
		bson.M{"tenantId": "test-tenant", "domain": "other.com", "createdAt": time.Now()},
	})

	handler := handleAPIv1ListDomains(db)
	req := testRequest(t, "GET", "/api/v1/domains", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	domains, ok := resp["domains"].([]any)
	if !ok {
		t.Fatal("expected domains array")
	}
	if len(domains) < 2 {
		t.Errorf("expected at least 2 domains, got %d", len(domains))
	}
}

// ── handleAPIv1GetAnalysis ────────────────────────────────────────────────────

func TestHandleAPIv1GetAnalysis_NotFound(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1GetAnalysis(db)

	req := testRequest(t, "GET", "/api/v1/domains/example.com/analysis", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusNotFound)
}

func TestHandleAPIv1GetAnalysis_EmptyDomain(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1GetAnalysis(db)

	req := testRequest(t, "GET", "/api/v1/domains//analysis", nil)
	req.SetPathValue("domain", "")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleAPIv1GetAnalysis_Found(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.Analyses().InsertOne(ctx, Analysis{
		TenantID:  "test-tenant",
		Domain:    "example.com",
		CreatedAt: time.Now(),
	})

	handler := handleAPIv1GetAnalysis(db)
	req := testRequest(t, "GET", "/api/v1/domains/example.com/analysis", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var result bson.M
	json.NewDecoder(w.Body).Decode(&result)
	if result["domain"] != "example.com" {
		t.Errorf("domain: got %v", result["domain"])
	}
}

// ── handleAPIv1GetOptimizations ───────────────────────────────────────────────

func TestHandleAPIv1GetOptimizations_EmptyDomain(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1GetOptimizations(db)

	req := testRequest(t, "GET", "/api/v1/domains//optimizations", nil)
	req.SetPathValue("domain", "")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleAPIv1GetOptimizations_Empty(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1GetOptimizations(db)

	req := testRequest(t, "GET", "/api/v1/domains/example.com/optimizations", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var results []any
	json.NewDecoder(w.Body).Decode(&results)
	if len(results) != 0 {
		t.Errorf("expected empty results, got %d", len(results))
	}
}

func TestHandleAPIv1GetOptimizations_WithData(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.Optimizations().InsertOne(ctx, Optimization{
		TenantID:  "test-tenant",
		Domain:    "example.com",
		CreatedAt: time.Now(),
	})

	handler := handleAPIv1GetOptimizations(db)
	req := testRequest(t, "GET", "/api/v1/domains/example.com/optimizations", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var results []any
	json.NewDecoder(w.Body).Decode(&results)
	if len(results) != 1 {
		t.Errorf("expected 1 optimization, got %d", len(results))
	}
}

// ── handleAPIv1GetVideo ───────────────────────────────────────────────────────

func TestHandleAPIv1GetVideo_EmptyDomain(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1GetVideo(db)

	req := testRequest(t, "GET", "/api/v1/domains//video", nil)
	req.SetPathValue("domain", "")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleAPIv1GetVideo_NotFound(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1GetVideo(db)

	req := testRequest(t, "GET", "/api/v1/domains/example.com/video", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusNotFound)
}

func TestHandleAPIv1GetVideo_Found(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.VideoAnalyses().InsertOne(ctx, VideoAnalysis{
		TenantID:    "test-tenant",
		Domain:      "example.com",
		GeneratedAt: time.Now(),
	})

	handler := handleAPIv1GetVideo(db)
	req := testRequest(t, "GET", "/api/v1/domains/example.com/video", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

// ── handleAPIv1GetReddit ──────────────────────────────────────────────────────

func TestHandleAPIv1GetReddit_EmptyDomain(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1GetReddit(db)

	req := testRequest(t, "GET", "/api/v1/domains//reddit", nil)
	req.SetPathValue("domain", "")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleAPIv1GetReddit_NotFound(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1GetReddit(db)

	req := testRequest(t, "GET", "/api/v1/domains/example.com/reddit", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusNotFound)
}

func TestHandleAPIv1GetReddit_Found(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.RedditAnalyses().InsertOne(ctx, RedditAnalysis{
		TenantID:    "test-tenant",
		Domain:      "example.com",
		GeneratedAt: time.Now(),
	})

	handler := handleAPIv1GetReddit(db)
	req := testRequest(t, "GET", "/api/v1/domains/example.com/reddit", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

// ── handleAPIv1GetSearch ──────────────────────────────────────────────────────

func TestHandleAPIv1GetSearch_EmptyDomain(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1GetSearch(db)

	req := testRequest(t, "GET", "/api/v1/domains//search", nil)
	req.SetPathValue("domain", "")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleAPIv1GetSearch_NotFound(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1GetSearch(db)

	req := testRequest(t, "GET", "/api/v1/domains/example.com/search", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusNotFound)
}

func TestHandleAPIv1GetSearch_Found(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.SearchAnalyses().InsertOne(ctx, SearchAnalysis{
		TenantID:    "test-tenant",
		Domain:      "example.com",
		GeneratedAt: time.Now(),
	})

	handler := handleAPIv1GetSearch(db)
	req := testRequest(t, "GET", "/api/v1/domains/example.com/search", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

// ── handleAPIv1GetSummary ─────────────────────────────────────────────────────

func TestHandleAPIv1GetSummary_EmptyDomain(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1GetSummary(db)

	req := testRequest(t, "GET", "/api/v1/domains//summary", nil)
	req.SetPathValue("domain", "")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleAPIv1GetSummary_NotFound(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1GetSummary(db)

	req := testRequest(t, "GET", "/api/v1/domains/example.com/summary", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusNotFound)
}

func TestHandleAPIv1GetSummary_Found(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.DomainSummaries().InsertOne(ctx, DomainSummary{
		TenantID:    "test-tenant",
		Domain:      "example.com",
		GeneratedAt: time.Now(),
	})

	handler := handleAPIv1GetSummary(db)
	req := testRequest(t, "GET", "/api/v1/domains/example.com/summary", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

// ── handleAPIv1GetTests ───────────────────────────────────────────────────────

func TestHandleAPIv1GetTests_EmptyDomain(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1GetTests(db)

	req := testRequest(t, "GET", "/api/v1/domains//tests", nil)
	req.SetPathValue("domain", "")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleAPIv1GetTests_Empty(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1GetTests(db)

	req := testRequest(t, "GET", "/api/v1/domains/example.com/tests", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	tests, ok := resp["tests"].([]any)
	if !ok {
		t.Fatal("expected tests array")
	}
	if len(tests) != 0 {
		t.Errorf("expected empty tests, got %d", len(tests))
	}
	if resp["count"] != float64(0) {
		t.Errorf("expected count=0, got %v", resp["count"])
	}
}

// ── handleAPIv1GetScore ───────────────────────────────────────────────────────

func TestHandleAPIv1GetScore_EmptyDomain(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1GetScore(db)

	req := testRequest(t, "GET", "/api/v1/domains//score", nil)
	req.SetPathValue("domain", "")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleAPIv1GetScore_NoData(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1GetScore(db)

	req := testRequest(t, "GET", "/api/v1/domains/example.com/score", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	// Score should be 0 when no data available
	if resp["score"] != float64(0) {
		t.Errorf("expected score=0 with no data, got %v", resp["score"])
	}
	if resp["available"] != float64(0) {
		t.Errorf("expected available=0, got %v", resp["available"])
	}
	if resp["total"] != float64(5) {
		t.Errorf("expected total=5 components, got %v", resp["total"])
	}
}

// ── handleAPIv1GetBrand ───────────────────────────────────────────────────────

func TestHandleAPIv1GetBrand_EmptyDomain(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1GetBrand(db)

	req := testRequest(t, "GET", "/api/v1/domains//brand", nil)
	req.SetPathValue("domain", "")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleAPIv1GetBrand_NotFound(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1GetBrand(db)

	req := testRequest(t, "GET", "/api/v1/domains/example.com/brand", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusNotFound)
}

func TestHandleAPIv1GetBrand_Found(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.BrandProfiles().InsertOne(ctx, BrandProfile{
		TenantID: "test-tenant",
		Domain:   "example.com",
	})

	handler := handleAPIv1GetBrand(db)
	req := testRequest(t, "GET", "/api/v1/domains/example.com/brand", nil)
	req.SetPathValue("domain", "example.com")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

// ── handleAPIv1ListTodos ──────────────────────────────────────────────────────

func TestHandleAPIv1ListTodos_Empty(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1ListTodos(db)

	req := testRequest(t, "GET", "/api/v1/todos", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var todos []any
	json.NewDecoder(w.Body).Decode(&todos)
	if len(todos) != 0 {
		t.Errorf("expected empty todos, got %d", len(todos))
	}
}

func TestHandleAPIv1ListTodos_WithData(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.Todos().InsertMany(ctx, []any{
		bson.M{"tenantId": "test-tenant", "domain": "example.com", "status": "todo", "action": "Test action 1", "createdAt": time.Now()},
		bson.M{"tenantId": "test-tenant", "domain": "example.com", "status": "completed", "action": "Test action 2", "createdAt": time.Now()},
	})

	handler := handleAPIv1ListTodos(db)
	req := testRequest(t, "GET", "/api/v1/todos", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var todos []any
	json.NewDecoder(w.Body).Decode(&todos)
	if len(todos) != 2 {
		t.Errorf("expected 2 todos, got %d", len(todos))
	}
}

func TestHandleAPIv1ListTodos_FilterByStatus(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.Todos().InsertMany(ctx, []any{
		bson.M{"tenantId": "test-tenant", "domain": "example.com", "status": "todo", "action": "Pending todo", "createdAt": time.Now()},
		bson.M{"tenantId": "test-tenant", "domain": "example.com", "status": "completed", "action": "Completed todo", "createdAt": time.Now()},
	})

	handler := handleAPIv1ListTodos(db)
	req := testRequest(t, "GET", "/api/v1/todos?status=todo", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var todos []any
	json.NewDecoder(w.Body).Decode(&todos)
	if len(todos) != 1 {
		t.Errorf("expected 1 todo with status=todo, got %d", len(todos))
	}
}

func TestHandleAPIv1ListTodos_FilterByDomain(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	db.Todos().InsertMany(ctx, []any{
		bson.M{"tenantId": "test-tenant", "domain": "example.com", "status": "todo", "action": "Example todo", "createdAt": time.Now()},
		bson.M{"tenantId": "test-tenant", "domain": "other.com", "status": "todo", "action": "Other todo", "createdAt": time.Now()},
	})

	handler := handleAPIv1ListTodos(db)
	req := testRequest(t, "GET", "/api/v1/todos?domain=example.com", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var todos []any
	json.NewDecoder(w.Body).Decode(&todos)
	if len(todos) != 1 {
		t.Errorf("expected 1 todo for example.com, got %d", len(todos))
	}
}

// ── handleAPIv1GetTodo ────────────────────────────────────────────────────────

func TestHandleAPIv1GetTodo_InvalidID(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1GetTodo(db)

	req := testRequest(t, "GET", "/api/v1/todos/invalid-id", nil)
	req.SetPathValue("id", "invalid-id")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleAPIv1GetTodo_NotFound(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1GetTodo(db)

	newID := primitive.NewObjectID().Hex()
	req := testRequest(t, "GET", "/api/v1/todos/"+newID, nil)
	req.SetPathValue("id", newID)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusNotFound)
}

func TestHandleAPIv1GetTodo_Found(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	result, _ := db.Todos().InsertOne(ctx, bson.M{
		"tenantId": "test-tenant",
		"domain":   "example.com",
		"status":   "todo",
		"action":   "Test todo action",
	})
	insertedID := result.InsertedID.(primitive.ObjectID)

	handler := handleAPIv1GetTodo(db)
	req := testRequest(t, "GET", "/api/v1/todos/"+insertedID.Hex(), nil)
	req.SetPathValue("id", insertedID.Hex())
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var todo TodoItem
	json.NewDecoder(w.Body).Decode(&todo)
	if todo.Domain != "example.com" {
		t.Errorf("domain: got %q, want example.com", todo.Domain)
	}
}

// ── handleAPIv1UpdateTodo ─────────────────────────────────────────────────────

func TestHandleAPIv1UpdateTodo_InvalidID(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1UpdateTodo(db)

	req := testRequest(t, "PUT", "/api/v1/todos/invalid-id", map[string]string{"status": "completed"})
	req.SetPathValue("id", "invalid-id")
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleAPIv1UpdateTodo_InvalidStatus(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1UpdateTodo(db)

	newID := primitive.NewObjectID().Hex()
	req := testRequest(t, "PUT", "/api/v1/todos/"+newID, map[string]string{"status": "invalid-status"})
	req.SetPathValue("id", newID)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleAPIv1UpdateTodo_NotFound(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1UpdateTodo(db)

	newID := primitive.NewObjectID().Hex()
	req := testRequest(t, "PUT", "/api/v1/todos/"+newID, map[string]string{"status": "completed"})
	req.SetPathValue("id", newID)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusNotFound)
}

func TestHandleAPIv1UpdateTodo_Completed(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	result, _ := db.Todos().InsertOne(ctx, bson.M{
		"tenantId": "test-tenant",
		"domain":   "example.com",
		"status":   "todo",
		"action":   "Test todo",
	})
	insertedID := result.InsertedID.(primitive.ObjectID)

	handler := handleAPIv1UpdateTodo(db)
	req := testRequest(t, "PUT", "/api/v1/todos/"+insertedID.Hex(), map[string]string{"status": "completed"})
	req.SetPathValue("id", insertedID.Hex())
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["updated"] != true {
		t.Errorf("expected updated=true, got %v", resp["updated"])
	}
	if resp["status"] != "completed" {
		t.Errorf("expected status=completed, got %v", resp["status"])
	}
}

func TestHandleAPIv1UpdateTodo_Backlogged(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	result, _ := db.Todos().InsertOne(ctx, bson.M{
		"tenantId": "test-tenant",
		"status":   "todo",
		"action":   "Test todo",
	})
	insertedID := result.InsertedID.(primitive.ObjectID)

	handler := handleAPIv1UpdateTodo(db)
	req := testRequest(t, "PUT", "/api/v1/todos/"+insertedID.Hex(), map[string]string{"status": "backlogged"})
	req.SetPathValue("id", insertedID.Hex())
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestHandleAPIv1UpdateTodo_Archived(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	result, _ := db.Todos().InsertOne(ctx, bson.M{
		"tenantId": "test-tenant",
		"status":   "todo",
		"action":   "Test todo",
	})
	insertedID := result.InsertedID.(primitive.ObjectID)

	handler := handleAPIv1UpdateTodo(db)
	req := testRequest(t, "PUT", "/api/v1/todos/"+insertedID.Hex(), map[string]string{"status": "archived"})
	req.SetPathValue("id", insertedID.Hex())
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestHandleAPIv1UpdateTodo_BackToTodo(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	result, _ := db.Todos().InsertOne(ctx, bson.M{
		"tenantId": "test-tenant",
		"status":   "completed",
		"action":   "Test todo",
	})
	insertedID := result.InsertedID.(primitive.ObjectID)

	handler := handleAPIv1UpdateTodo(db)
	req := testRequest(t, "PUT", "/api/v1/todos/"+insertedID.Hex(), map[string]string{"status": "todo"})
	req.SetPathValue("id", insertedID.Hex())
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

// ── handleAPIv1BulkUpdateTodos ────────────────────────────────────────────────

func TestHandleAPIv1BulkUpdateTodos_EmptyIDs(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1BulkUpdateTodos(db)

	body := map[string]any{"ids": []string{}, "status": "completed"}
	req := testRequest(t, "PUT", "/api/v1/todos/bulk", body)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleAPIv1BulkUpdateTodos_TooManyIDs(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1BulkUpdateTodos(db)

	ids := make([]string, 101)
	for i := range ids {
		ids[i] = primitive.NewObjectID().Hex()
	}
	body := map[string]any{"ids": ids, "status": "completed"}
	req := testRequest(t, "PUT", "/api/v1/todos/bulk", body)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleAPIv1BulkUpdateTodos_InvalidStatus(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1BulkUpdateTodos(db)

	body := map[string]any{"ids": []string{primitive.NewObjectID().Hex()}, "status": "invalid"}
	req := testRequest(t, "PUT", "/api/v1/todos/bulk", body)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleAPIv1BulkUpdateTodos_InvalidIDInArray(t *testing.T) {
	db := testMongoDB(t)
	handler := handleAPIv1BulkUpdateTodos(db)

	body := map[string]any{"ids": []string{"not-a-valid-id"}, "status": "completed"}
	req := testRequest(t, "PUT", "/api/v1/todos/bulk", body)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestHandleAPIv1BulkUpdateTodos_Success(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	result1, _ := db.Todos().InsertOne(ctx, bson.M{"tenantId": "test-tenant", "status": "todo", "action": "Todo 1"})
	result2, _ := db.Todos().InsertOne(ctx, bson.M{"tenantId": "test-tenant", "status": "todo", "action": "Todo 2"})

	id1 := result1.InsertedID.(primitive.ObjectID).Hex()
	id2 := result2.InsertedID.(primitive.ObjectID).Hex()

	handler := handleAPIv1BulkUpdateTodos(db)
	body := map[string]any{"ids": []string{id1, id2}, "status": "completed"}
	req := testRequest(t, "PUT", "/api/v1/todos/bulk", body)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["updated"] != float64(2) {
		t.Errorf("expected updated=2, got %v", resp["updated"])
	}
}

func TestHandleAPIv1BulkUpdateTodos_Backlogged(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	result, _ := db.Todos().InsertOne(ctx, bson.M{"tenantId": "test-tenant", "status": "todo", "action": "Todo"})
	id := result.InsertedID.(primitive.ObjectID).Hex()

	handler := handleAPIv1BulkUpdateTodos(db)
	body := map[string]any{"ids": []string{id}, "status": "backlogged"}
	req := testRequest(t, "PUT", "/api/v1/todos/bulk", body)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}

func TestHandleAPIv1BulkUpdateTodos_Archived(t *testing.T) {
	db := testMongoDB(t)
	ctx := testAuthContext("test-tenant", "test-user")

	result, _ := db.Todos().InsertOne(ctx, bson.M{"tenantId": "test-tenant", "status": "todo", "action": "Todo"})
	id := result.InsertedID.(primitive.ObjectID).Hex()

	handler := handleAPIv1BulkUpdateTodos(db)
	body := map[string]any{"ids": []string{id}, "status": "archived"}
	req := testRequest(t, "PUT", "/api/v1/todos/bulk", body)
	w := httptest.NewRecorder()
	handler(w, req)

	assertStatus(t, w, http.StatusOK)
}
