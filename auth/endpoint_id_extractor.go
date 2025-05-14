package auth

import (
	"fmt"
	"net/http"
	"strings"

	envoy_auth "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
)

// extractEndpointID extracts the endpoint ID from the HTTP request.
// It first attempts to extract the ID from the header.
// If the above fails, it falls back to the URL path.
// If both extraction methods fail, it returns an error.
func extractEndpointID(req *envoy_auth.AttributeContext_HttpRequest) (string, error) {
	if id := extractFromHeader(req); id != "" {
		return id, nil
	}
	if id := extractFromPath(req); id != "" {
		return id, nil
	}
	return "", fmt.Errorf("endpoint ID not provided in header or path")
}

// extractFromHeader extracts the endpoint ID from the headers.
// It returns the endpoint ID if found and non-empty, otherwise an empty string.
// Example: Header = "Portal-Application-ID: 1a2b3c4d" -> endpointID = "1a2b3c4d"
func extractFromHeader(req *envoy_auth.AttributeContext_HttpRequest) string {
	headers := req.GetHeaders()

	// Convert map[string]string to http.Header as `GetHeaders` returns
	// map[string]string which could lead to case-sensitivity issues.
	httpHeaders := make(http.Header)
	for key, value := range headers {
		httpHeaders.Add(key, value)
	}

	// Use http.Header's Get method which is case-insensitive
	endpointID := httpHeaders.Get(reqHeaderEndpointID)
	if endpointID == "" {
		return ""
	}

	return endpointID
}

// extractFromPath extracts the endpoint ID from the URL path.
// It expects the path to have the prefix "/v1/" and the endpoint ID as the first segment after it.
// It returns the endpoint ID if found, otherwise an empty string.
// Example: http://eth.path.grove.city/v1/1a2b3c4d -> endpointID = "1a2b3c4d"
func extractFromPath(req *envoy_auth.AttributeContext_HttpRequest) string {
	path := req.GetPath()
	if strings.HasPrefix(path, pathPrefix) {
		segments := strings.Split(strings.TrimPrefix(path, pathPrefix), "/")
		if len(segments) > 0 && segments[0] != "" {
			return segments[0]
		}
	}
	return ""
}
