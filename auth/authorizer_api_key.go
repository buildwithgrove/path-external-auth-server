package auth

import (
	"net/http"

	"github.com/buildwithgrove/path-external-auth-server/store"
)

const (
	// authHeaderKey MUST be present
	authHeaderKey = "Authorization"

	// apiKeyPrefix MAY be present
	apiKeyPrefix = "Bearer "
)

var _ Authorizer = (*AuthorizerAPIKey)(nil)

// AuthorizerAPIKey
//
// - Authorizes a request using an API key
// - Compares the API key in the request headers with the API key in the PortalApp
type AuthorizerAPIKey struct{}

// authorizeRequest
//
// - Authorizes a request using an API key
// - Returns errUnauthorized if the API key is missing or does not match
func (a *AuthorizerAPIKey) authorizeRequest(
	headers http.Header,
	portalApp *store.PortalApp) error {
	// Extract the API key from the Authorization header (case-insensitive lookup)
	headerValue := headers.Get(authHeaderKey)
	if headerValue == "" {
		return errUnauthorized
	}

	// Remove the "Bearer " prefix from the API key if present
	apiKey := headerValue
	if len(headerValue) > len(apiKeyPrefix) && headerValue[:len(apiKeyPrefix)] == apiKeyPrefix {
		apiKey = headerValue[len(apiKeyPrefix):]
	}

	// Compare the API key with the expected value
	if apiKey != portalApp.Auth.APIKey {
		return errUnauthorized
	}

	return nil
}
