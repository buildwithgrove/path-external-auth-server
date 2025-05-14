package auth

import (
	"fmt"
	"net/http"

	store "github.com/buildwithgrove/path-external-auth-server/portal_app_store"
)

// errUnauthorized is returned when a request is not authorized.
// It is left intentionally vague to avoid leaking information to the client.
var errUnauthorized = fmt.Errorf("unauthorized")

// Authorizer is an interface for authorizing requests against a PortalApp.
type Authorizer interface {
	// authorizeRequest authorizes a request using the provided headers and a PortalApp.
	authorizeRequest(headers http.Header, portalApp *store.PortalApp) error
}
