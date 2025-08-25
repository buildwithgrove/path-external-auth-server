// The auth package implements the Envoy External Authorization gRPC service.
// Responsibilities:
// - Receives requests from Envoy
// - Authorizes requests based on PortalApp data stored in the store package
// - Receives a check request from GUARD and determines if the request should be authorized
package auth

import (
	"context"
	"fmt"
	"net/http"

	envoy_core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_auth "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	envoy_type "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/pokt-network/poktroll/pkg/polylog"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"

	"github.com/buildwithgrove/path-external-auth-server/store"
)

const accountRateLimitMessage = "This account is rate limited. To upgrade your plan or modify your account settings, log in to your account at https://portal.grove.city/"

const (
	// TODO_TECHDEBT(@commoddity): This path segment should be configurable via a single source of truth.
	// - Referred to in multiple places (e.g. GUARD Helm charts, PATH's router.go, and here)
	pathPrefix = "/v1/"

	// The portal app and account id must match PATH's expected HTTP headers.
	// Reference:
	// https://github.com/buildwithgrove/path/blob/1e7b2d83294e8c406479ae5e480f4dca97414cee/gateway/observation.go#L16-L18
	reqHeaderPortalAppID = "Portal-Application-ID" // Set on all service requests
	reqHeaderAccountID   = "Portal-Account-ID"     // Set on all service requests

	errBody = `{"code": %d, "message": "%s"}`
)

// portalAppStore interface provides an in-memory store of PortalApps.
//
// Used for:
//   - Fast lookups of authorization data for PATH when processing requests.
type portalAppStore interface {
	GetPortalApp(portalAppID store.PortalAppID) (*store.PortalApp, bool)
}

// rateLimitStore interface provides an in-memory store of rate limited accounts.
//
// Used for:
//   - Fast lookups of rate limited accounts for PATH when processing requests.
type rateLimitStore interface {
	IsAccountRateLimited(accountID store.AccountID) bool
}

// AuthHandler processes requests from Envoy.
//
// Primary responsibilities:
//   - Handles requests via the Check method (called for each request)
type AuthHandler struct {
	Logger polylog.Logger

	// PortalAppStore: in-memory store of PortalApps
	PortalAppStore portalAppStore

	// RateLimitStore: in-memory store of rate limited accounts
	RateLimitStore rateLimitStore

	// APIKeyAuthorizer: used for request authorization
	APIKeyAuthorizer Authorizer
}

// Check implements the Envoy External Authorization gRPC service.
// Steps performed:
//   - Extract Portal Application ID from the path
//   - Extract Account ID from headers
//   - Fetch Portal Application from the database
//   - Check if the Portal Application is authorized
//   - Check if the Account is rate limited
//   - Return an OK or Denied response with HTTP headers set
func (a *AuthHandler) Check(
	ctx context.Context,
	checkReq *envoy_auth.CheckRequest,
) (*envoy_auth.CheckResponse, error) {
	// Get the HTTP request
	req := checkReq.GetAttributes().GetRequest().GetHttp()
	if req == nil {
		return getDeniedCheckResponse("HTTP request not found", envoy_type.StatusCode_BadRequest), nil
	}

	// Get the request path
	path := req.GetPath()
	if path == "" {
		return getDeniedCheckResponse("path not provided", envoy_type.StatusCode_BadRequest), nil
	}

	// Get the request headers as a http.Header
	headers := convertMapToHeader(req.GetHeaders())

	// Extract the Portal Application ID from the request
	// It may be extracted from the URL path or the headers
	portalAppID, err := extractPortalAppID(headers, path)
	if err != nil {
		a.Logger.Debug().Err(err).Msg("🚫 unable to extract portal app ID from request")
		return getDeniedCheckResponse(err.Error(), envoy_type.StatusCode_BadRequest), nil
	}
	logger := a.Logger.With("portal_app_id", portalAppID)

	// If we get here, we have a valid Portal Application ID.
	logger.Debug().Msg("🔍 handling check request")

	// Fetch Portal Application from Portal Application store
	portalApp, ok := a.getPortalApp(portalAppID)
	if !ok {
		logger.Debug().Msg("🚫 specified portal app not found: rejecting the request.")
		return getDeniedCheckResponse("portal app not found", envoy_type.StatusCode_NotFound), nil
	}
	logger = logger.With("account_id", portalApp.AccountID)

	// Check if the Portal Application is authorized
	if err := a.checkPortalAppAuthorized(headers, portalApp); err != nil {
		logger.Debug().Err(err).Msg("🚫 request failed authorization: rejecting the request.")
		return getDeniedCheckResponse(err.Error(), envoy_type.StatusCode_Unauthorized), nil
	}

	// Check if the Account is rate limited
	if err := a.checkAccountRateLimited(portalApp); err != nil {
		logger.Debug().Msg("🚫 account is rate limited: rejecting the request.")
		return getDeniedCheckResponse(accountRateLimitMessage, envoy_type.StatusCode_TooManyRequests), nil
	}

	// Add Portal Application ID and Account ID to the headers
	// to be passed upstream along the filter chain to the rate limiter.
	httpHeaders := a.getHTTPHeaders(portalApp)

	// Return a valid response with the HTTP headers set
	return getOKCheckResponse(httpHeaders), nil
}

// --------------------------------- Helpers ---------------------------------

// convertMapToHeader converts a map[string]string to a http.Header.
// - Ensures case-insensitive header access.
func convertMapToHeader(headersMap map[string]string) http.Header {
	httpHeaders := make(http.Header, len(headersMap))
	for key, value := range headersMap {
		httpHeaders.Add(key, value)
	}
	return httpHeaders
}

// getPortalApp fetches the PortalApp from the portal app store.
//   - Returns the PortalApp and a bool indicating if it was found.
func (a *AuthHandler) getPortalApp(portalAppID store.PortalAppID) (*store.PortalApp, bool) {
	return a.PortalAppStore.GetPortalApp(portalAppID)
}

// checkPortalAppAuthorized performs all configured authorization checks on the request.
//   - Returns nil if no authorization is required (Auth is nil or APIKey is empty)
//   - Otherwise, performs API Key authorization
func (a *AuthHandler) checkPortalAppAuthorized(headers http.Header, portalApp *store.PortalApp) error {
	if portalApp.Auth == nil || portalApp.Auth.APIKey == "" {
		return nil
	}
	return a.APIKeyAuthorizer.authorizeRequest(headers, portalApp)
}

// checkAccountRateLimited checks if the account is rate limited.
//   - Returns nil if the account is not eligible for rate limiting.
//   - Returns an error if the account is rate limited.
func (a *AuthHandler) checkAccountRateLimited(portalApp *store.PortalApp) error {
	if portalApp.RateLimit == nil {
		return nil
	}
	if a.RateLimitStore.IsAccountRateLimited(portalApp.AccountID) {
		return fmt.Errorf("account is rate limited")
	}
	return nil
}

// getHTTPHeaders sets all HTTP headers required by the PATH service on the request being forwarded.
//   - Adds portal app ID header on all requests ("Portal-Application-ID: <id>")
//   - Adds account ID header on all requests ("Portal-Account-ID: <id>")
func (a *AuthHandler) getHTTPHeaders(portalApp *store.PortalApp) []*envoy_core.HeaderValueOption {
	headers := []*envoy_core.HeaderValueOption{
		{
			Header: &envoy_core.HeaderValue{
				Key:   reqHeaderPortalAppID,
				Value: string(portalApp.ID),
			},
		},
		{
			Header: &envoy_core.HeaderValue{
				Key:   reqHeaderAccountID,
				Value: string(portalApp.AccountID),
			},
		},
	}

	return headers
}

// getDeniedCheckResponse returns a CheckResponse with denied status and error message.
//   - Sets PermissionDenied code and error message in response.
func getDeniedCheckResponse(err string, httpCode envoy_type.StatusCode) *envoy_auth.CheckResponse {
	return &envoy_auth.CheckResponse{
		Status: &status.Status{
			Code:    int32(codes.PermissionDenied),
			Message: err,
		},
		HttpResponse: &envoy_auth.CheckResponse_DeniedResponse{
			DeniedResponse: &envoy_auth.DeniedHttpResponse{
				Status: &envoy_type.HttpStatus{
					Code: httpCode,
				},
				Body: fmt.Sprintf(errBody, httpCode, err),
			},
		},
	}
}

// getOKCheckResponse returns a CheckResponse with OK status and provided headers.
//   - Sets OK code and attaches provided headers to response.
func getOKCheckResponse(headers []*envoy_core.HeaderValueOption) *envoy_auth.CheckResponse {
	return &envoy_auth.CheckResponse{
		Status: &status.Status{
			Code:    int32(codes.OK),
			Message: "ok",
		},
		HttpResponse: &envoy_auth.CheckResponse_OkResponse{
			OkResponse: &envoy_auth.OkHttpResponse{
				Headers: headers,
			},
		},
	}
}
