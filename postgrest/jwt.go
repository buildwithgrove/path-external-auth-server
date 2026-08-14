package postgrest

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// Use const header since it never changes for PostgREST
const (
	header     = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9" // {"alg":"HS256","typ":"JWT"}
	expiration = 1 * time.Hour
)

// generatePostgRESTJWT creates a JWT token for PostgREST authentication using Go best practices.
// Uses stdlib encoding/base64.RawURLEncoding which handles URL-safe base64 without padding.
func generatePostgRESTJWT(secret, role, email string) (string, error) {
	now := time.Now()

	// Create payload with minimal required claims for PostgREST
	payload := map[string]interface{}{
		"role":  role,
		"email": email,
		"exp":   now.Add(expiration).Unix(),
	}

	// Marshal payload to JSON
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JWT payload: %w", err)
	}

	// Encode payload using stdlib RawURLEncoding (no padding, URL-safe)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadJSON)

	// Create unsigned token
	unsignedToken := header + "." + encodedPayload

	// Sign with HMAC-SHA256
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(unsignedToken))
	signature := h.Sum(nil)

	// Encode signature using stdlib RawURLEncoding
	encodedSignature := base64.RawURLEncoding.EncodeToString(signature)

	// Return complete JWT
	return unsignedToken + "." + encodedSignature, nil
}
