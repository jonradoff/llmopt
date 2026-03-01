package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"llmopt/internal/saas"
)

// verifyAnthropicKey makes a minimal API call to check if a key is valid.
// Returns status: "active", "invalid", "no_credits", or "error".
func verifyAnthropicKey(ctx context.Context, apiKey string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	body, _ := json.Marshal(map[string]any{
		"model":      "claude-haiku-4-5-20251001",
		"max_tokens": 10,
		"messages": []map[string]any{
			{"role": "user", "content": "Reply with just the word 'ok'."},
		},
	})

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "error", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "error", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == 200:
		return "active", nil
	case resp.StatusCode == 401:
		return "invalid", nil
	case resp.StatusCode == 429:
		// Could be rate limit or out of credits — check error body
		errBody, _ := io.ReadAll(resp.Body)
		errStr := strings.ToLower(string(errBody))
		if strings.Contains(errStr, "credit") || strings.Contains(errStr, "billing") {
			return "no_credits", nil
		}
		// Rate limited but key is valid
		return "active", nil
	case resp.StatusCode == 529:
		// API overloaded — key is probably fine, API is just busy
		return "active", nil
	default:
		errBody, _ := io.ReadAll(resp.Body)
		return "error", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(errBody))
	}
}

// resolveAnthropicKey looks up the tenant's Anthropic API key from the database,
// decrypts it, and returns the plaintext key. In non-SaaS mode (dev), falls back
// to the system key. Returns an error if no key is configured.
func resolveAnthropicKey(ctx context.Context, mongoDB *MongoDB, encKey []byte, fallbackKey string, saasEnabled bool) (string, error) {
	if !saasEnabled {
		return fallbackKey, nil
	}

	tenantID := saas.TenantIDFromContext(ctx)
	if tenantID == "" {
		return "", fmt.Errorf("no tenant context")
	}

	var doc TenantAPIKey
	err := mongoDB.TenantAPIKeys().FindOne(ctx, bson.M{
		"tenantId": tenantID,
		"provider": "anthropic",
	}).Decode(&doc)
	if err != nil {
		return "", fmt.Errorf("api_key_required")
	}

	if doc.EncryptedKey == "" {
		return "", fmt.Errorf("api_key_required")
	}

	plaintext, err := decryptSecret(doc.EncryptedKey, encKey)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt API key: %w", err)
	}

	return plaintext, nil
}
