package auth

import (
	"net/http"

	store "github.com/buildwithgrove/path-external-auth-server/portal_app_store"
)

const (
	authHeaderKey = "Authorization"
	apiKeyPrefix  = "Bearer "
)

// APIKeyAuthorizer authorizes a request using an API key.
// It compares the API key in the request headers with the API key in the PortalApp.
type APIKeyAuthorizer struct{}

// authorizeRequest authorizes a request using an API key.
func (a *APIKeyAuthorizer) authorizeRequest(headers http.Header, portalApp *store.PortalApp) error {
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
