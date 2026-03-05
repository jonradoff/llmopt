package main

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// testDBName is the ONLY database name that tests may use.
// This must NEVER be "llmopt" (the production database).
const testDBName = "llmopt_test"

// blockedDBNames are production database names that tests must never connect to.
var blockedDBNames = []string{"llmopt", "llmopt-dev", "llmopt-staging"}

var (
	testClient     *mongo.Client
	testClientOnce sync.Once
	testClientErr  error
)

// cleanTestDB deletes all documents from all known collections in the test database.
// This is used instead of Database.Drop() because Atlas users may not have dropDatabase permission.
func cleanTestDB(t *testing.T, db *MongoDB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Safety: only clean the test database.
	if db.Database.Name() != testDBName {
		t.Fatalf("SAFETY: refusing to clean database %q", db.Database.Name())
	}

	// List all collections and delete all documents from each.
	collections, err := db.Database.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		// If the database doesn't exist yet, that's fine.
		return
	}
	for _, coll := range collections {
		db.Database.Collection(coll).DeleteMany(ctx, bson.M{})
	}
}

// testMongoDB returns a *MongoDB connected to the "llmopt_test" database.
// All documents are cleaned before and after each test for complete isolation.
//
// SAFETY:
//   - Requires MONGODB_TEST_URI env var (CI should set this to a restricted Atlas user)
//   - Falls back to MONGODB_URI for local development only
//   - The database name is always "llmopt_test", never production
//   - Refuses to run if the database name doesn't match
//
// Skips the test if neither env var is set.
func testMongoDB(t *testing.T) *MongoDB {
	t.Helper()

	// Prefer MONGODB_TEST_URI (restricted credentials for CI).
	// Fall back to MONGODB_URI for local development.
	uri := os.Getenv("MONGODB_TEST_URI")
	if uri == "" {
		uri = os.Getenv("MONGODB_URI")
	}
	if uri == "" {
		t.Skip("MONGODB_TEST_URI not set, skipping integration test")
	}

	// Connect once, reuse for all tests (connection pooling).
	testClientOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		testClient, testClientErr = mongo.Connect(ctx, options.Client().ApplyURI(uri))
		if testClientErr == nil {
			testClientErr = testClient.Ping(ctx, nil)
		}
	})
	if testClientErr != nil {
		t.Fatalf("failed to connect to test MongoDB: %v", testClientErr)
	}

	db := &MongoDB{
		Client:   testClient,
		Database: testClient.Database(testDBName),
	}

	// SAFETY: verify we're using the test database, not production.
	if db.Database.Name() != testDBName {
		t.Fatalf("SAFETY: expected database %q, got %q", testDBName, db.Database.Name())
	}
	for _, blocked := range blockedDBNames {
		if db.Database.Name() == blocked {
			t.Fatalf("SAFETY: refusing to use production database %q", blocked)
		}
	}

	// Clean all collections before test for isolation.
	cleanTestDB(t, db)

	// Clean all collections after test.
	t.Cleanup(func() {
		cleanTestDB(t, db)
	})

	return db
}

// TestTestMongoDB verifies the test database helper works correctly.
func TestTestMongoDB(t *testing.T) {
	db := testMongoDB(t)

	// Verify the database name is exactly our test database.
	if db.Database.Name() != testDBName {
		t.Fatalf("database name: got %q, want %q", db.Database.Name(), testDBName)
	}

	// Verify it's not a production database.
	for _, blocked := range blockedDBNames {
		if db.Database.Name() == blocked {
			t.Fatalf("SAFETY: test database matches blocked name %q", blocked)
		}
	}

	// Verify we can write and read.
	ctx := context.Background()
	_, err := db.Analyses().InsertOne(ctx, map[string]string{"test": "value"})
	if err != nil {
		t.Fatalf("failed to insert test document: %v", err)
	}

	count, err := db.Analyses().CountDocuments(ctx, map[string]string{})
	if err != nil {
		t.Fatalf("failed to count documents: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 document, got %d", count)
	}
}

// TestTestDBNameSafety verifies that the test database name is not a production name.
func TestTestDBNameSafety(t *testing.T) {
	if testDBName == "llmopt" {
		t.Fatal("testDBName must not be 'llmopt' (production)")
	}
	if !strings.HasPrefix(testDBName, "llmopt_test") {
		t.Fatalf("testDBName %q should start with 'llmopt_test'", testDBName)
	}
	if len(testDBName) > 38 {
		t.Fatalf("testDBName %q exceeds Atlas 38-byte limit (%d bytes)", testDBName, len(testDBName))
	}
}
