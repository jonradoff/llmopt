package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"llmopt/internal/saas"
)

const userAccessKeyPrefix = "lok_"

// generateAccessKey generates a new lok_ key and returns the raw key and its SHA-256 hex hash.
func generateAccessKey() (rawKey, keyHash, keyPreview string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return
	}
	rawKey = userAccessKeyPrefix + base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(rawKey))
	keyHash = hex.EncodeToString(sum[:])
	// preview: prefix + first 4 chars + "..." + last 6 chars
	if len(rawKey) > 14 {
		keyPreview = fmt.Sprintf("%s...%s", rawKey[:len(userAccessKeyPrefix)+4], rawKey[len(rawKey)-6:])
	} else {
		keyPreview = rawKey[:4] + "..."
	}
	return
}

// hashAccessKey returns the SHA-256 hex hash of a raw lok_ key.
func hashAccessKey(rawKey string) string {
	sum := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(sum[:])
}

// handleListUserAccessKeys handles GET /api/user/access-keys
func handleListUserAccessKeys(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := saas.UserIDFromContext(r.Context())
		tenantID := saas.TenantIDFromContext(r.Context())

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
		cursor, err := mongoDB.UserAccessKeys().Find(ctx, bson.M{
			"userId":   userID,
			"tenantId": tenantID,
			"isActive": true,
		}, opts)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		defer cursor.Close(ctx)

		var keys []UserAccessKey
		if err := cursor.All(ctx, &keys); err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		if keys == nil {
			keys = []UserAccessKey{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"keys": keys})
	}
}

// handleCreateUserAccessKey handles POST /api/user/access-keys
// Returns the raw key exactly once in the response.
func handleCreateUserAccessKey(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := saas.UserIDFromContext(r.Context())
		tenantID := saas.TenantIDFromContext(r.Context())

		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		if req.Name == "" {
			http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
			return
		}
		if len(req.Name) > 64 {
			http.Error(w, `{"error":"name must be 64 characters or fewer"}`, http.StatusBadRequest)
			return
		}

		rawKey, keyHash, keyPreview, err := generateAccessKey()
		if err != nil {
			http.Error(w, `{"error":"failed to generate key"}`, http.StatusInternalServerError)
			return
		}

		now := time.Now()
		key := UserAccessKey{
			ID:         primitive.NewObjectID(),
			UserID:     userID,
			TenantID:   tenantID,
			Name:       req.Name,
			KeyHash:    keyHash,
			KeyPreview: keyPreview,
			CreatedAt:  now,
			IsActive:   true,
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		if _, err := mongoDB.UserAccessKeys().InsertOne(ctx, key); err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"key":     key,
			"raw_key": rawKey, // shown only once
		})
	}
}

// handleDeleteUserAccessKey handles DELETE /api/user/access-keys/{id}
func handleDeleteUserAccessKey(mongoDB *MongoDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := saas.UserIDFromContext(r.Context())
		tenantID := saas.TenantIDFromContext(r.Context())

		idStr := r.PathValue("id")
		oid, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		result, err := mongoDB.UserAccessKeys().UpdateOne(ctx,
			bson.M{"_id": oid, "userId": userID, "tenantId": tenantID},
			bson.M{"$set": bson.M{"isActive": false}},
		)
		if err != nil {
			http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
			return
		}
		if result.MatchedCount == 0 {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// ValidateUserAccessKey looks up a lok_ key in the user_access_keys collection.
// Returns the UserAccessKey document on success, or an error if not found/inactive.
// Also updates lastUsedAt asynchronously.
func ValidateUserAccessKey(db *mongo.Database, rawKey string) (*UserAccessKey, error) {
	hash := hashAccessKey(rawKey)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	coll := db.Collection("user_access_keys")
	var key UserAccessKey
	err := coll.FindOne(ctx, bson.M{"keyHash": hash, "isActive": true}).Decode(&key)
	if err != nil {
		return nil, err
	}

	// Update lastUsedAt in the background (non-blocking)
	go func() {
		bg, bgCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer bgCancel()
		now := time.Now()
		_, _ = coll.UpdateByID(bg, key.ID, bson.M{"$set": bson.M{"lastUsedAt": now}})
	}()

	return &key, nil
}
