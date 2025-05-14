package auth

import (
	"fmt"
	"net/http"
	"strings"

	store "github.com/buildwithgrove/path-external-auth-server/portal_app_store"
)

// extractPortalAppID extracts the portal app ID from the HTTP request.
// It first attempts to extract the ID from the header.
// If the above fails, it falls back to the URL path.
// If both extraction methods fail, it returns an error.
func extractPortalAppID(headers http.Header, path string) (store.PortalAppID, error) {
	if id := extractFromHeader(headers); id != "" {
		return id, nil
	}
	if id := extractFromPath(path); id != "" {
		return id, nil
	}
	return "", fmt.Errorf("portal app ID not provided in header or path")
}

// extractFromHeader extracts the portal app ID from the headers.
// It returns the portal app ID if found and non-empty, otherwise an empty string.
// Example: Header = "Portal-Application-ID: 1a2b3c4d" -> portalAppID = "1a2b3c4d"
func extractFromHeader(headers http.Header) store.PortalAppID {
	// Use http.Header's Get method which is case-insensitive
	portalAppID := headers.Get(reqHeaderPortalAppID)
	if portalAppID == "" {
		return ""
	}

	return store.PortalAppID(portalAppID)
}

// extractFromPath extracts the portal app ID from the URL path.
// It expects the path to have the prefix "/v1/" and the portal app ID as the first segment after it.
// It returns the portal app ID if found, otherwise an empty string.
// Example: http://eth.path.grove.city/v1/1a2b3c4d -> portalAppID = "1a2b3c4d"
func extractFromPath(path string) store.PortalAppID {
	if strings.HasPrefix(path, pathPrefix) {
		segments := strings.Split(strings.TrimPrefix(path, pathPrefix), "/")
		if len(segments) > 0 && segments[0] != "" {
			return store.PortalAppID(segments[0])
		}
	}
	return ""
}
