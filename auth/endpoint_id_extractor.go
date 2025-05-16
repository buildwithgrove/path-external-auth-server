package auth

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/buildwithgrove/path-external-auth-server/store"
)

// extractPortalAppID extracts the portal app ID from an HTTP request.
//
// Extraction order:
// - Try to extract from the header first
// - If not found, try to extract from the URL path
// - If neither method succeeds, return an error
func extractPortalAppID(headers http.Header, path string) (store.PortalAppID, error) {
	if id := extractPortalAppIDFromHeader(headers); id != "" {
		return id, nil
	}
	if id := extractPortalAppIDFromPath(path); id != "" {
		return id, nil
	}
	return "", fmt.Errorf("portal app ID not provided in header or path")
}

// extractPortalAppIDFromHeader gets the portal app ID from HTTP headers.
//
// - Returns the portal app ID if present and non-empty
// - Returns an empty string if not found
//
// Example:
//
//	Header: "Portal-Application-ID: 1a2b3c4d"
//	Returns: "1a2b3c4d"
func extractPortalAppIDFromHeader(headers http.Header) store.PortalAppID {
	// Use http.Header's Get method which is case-insensitive
	portalAppID := headers.Get(reqHeaderPortalAppID)
	if portalAppID == "" {
		return ""
	}

	return store.PortalAppID(portalAppID)
}

// extractPortalAppIDFromPath gets the portal app ID from the URL path.
//
// - Expects path to start with "/v1/"
// - Returns the first segment after the prefix as the portal app ID
// - Returns an empty string if not found
//
// Example:
//
//	Path: "/v1/1a2b3c4d"
//	Returns: "1a2b3c4d"
func extractPortalAppIDFromPath(path string) store.PortalAppID {
	if strings.HasPrefix(path, pathPrefix) {
		segments := strings.Split(strings.TrimPrefix(path, pathPrefix), "/")
		if len(segments) > 0 && segments[0] != "" {
			return store.PortalAppID(segments[0])
		}
	}
	return ""
}
