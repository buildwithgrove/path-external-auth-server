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
	"time"

	envoy_core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_auth "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	envoy_type "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/pokt-network/poktroll/pkg/polylog"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"

	"github.com/buildwithgrove/path-external-auth-server/store"
)

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

// PortalAppStore interface provides an in-memory store of PortalApps.
//
// Used for:
// - Fast lookups of authorization data for PATH when processing requests.
type PortalAppStore interface {
	GetPortalApp(portalAppID store.PortalAppID) (*store.PortalApp, bool)
}

type RateLimitStore interface {
	IsAccountRateLimited(accountID store.AccountID) bool
}

// AuthHandler processes requests from Envoy.
//
// Primary responsibilities:
// - Handles requests via the Check method (called for each request)
type AuthHandler struct {
	Logger polylog.Logger

	// PortalAppStore: in-memory store of PortalApps
	PortalAppStore PortalAppStore

	// RateLimitStore: used for checking if an account is rate limited
	RateLimitStore RateLimitStore

	// APIKeyAuthorizer: used for request authorization
	APIKeyAuthorizer Authorizer
}

// Check implements the Envoy External Authorization gRPC service.
// Steps performed:
// - Extract portal app ID from the path
// - Extract account user ID from headers
// - Fetch PortalApp from the database
// - Perform all configured authorization checks
// - Return a response with HTTP headers set
func (a *AuthHandler) Check(
	ctx context.Context,
	checkReq *envoy_auth.CheckRequest,
) (*envoy_auth.CheckResponse, error) {
	start := time.Now()
	var portalAppID store.PortalAppID
	defer func() {
		if portalAppID != "" {
			logAuthDuration(a.Logger, string(portalAppID), start)
		}
	}()

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

	// Extract the portal app ID from the request
	// It may be extracted from the URL path or the headers
	portalAppID, err := extractPortalAppID(headers, path)
	if err != nil {
		a.Logger.Info().Err(err).Msg("unable to extract portal app ID from request")
		return getDeniedCheckResponse(err.Error(), envoy_type.StatusCode_BadRequest), nil
	}

	logger := a.Logger.With("portal_app_id", portalAppID)
	logger.Debug().Msg("handling check request")

	// Fetch PortalApp from portal app store
	portalApp, ok := a.getPortalApp(portalAppID)
	if !ok {
		logger.Info().Msg("specified portal app not found: rejecting the request.")
		return getDeniedCheckResponse("portal app not found", envoy_type.StatusCode_NotFound), nil
	}

	// Perform all configured authorization checks
	if err := a.authPortalApp(headers, portalApp); err != nil {
		logger.Info().Err(err).Msg("request failed authorization: rejecting the request.")
		return getDeniedCheckResponse(err.Error(), envoy_type.StatusCode_Unauthorized), nil
	}

	// Check if the account is rate limited
	if err := a.checkAccountRateLimit(portalApp); err != nil {
		logger.Info().Msg("account is rate limited: rejecting the request.")
		return getDeniedCheckResponse("account is rate limited", envoy_type.StatusCode_TooManyRequests), nil
	}

	// Add portal app ID, account ID, and rate limiting values to the headers
	// to be passed upstream along the filter chain to the rate limiter.
	httpHeaders := a.getHTTPHeaders(portalApp)

	// Return a valid response with the HTTP headers set
	return getOKCheckResponse(httpHeaders), nil
}

// logAuthDuration logs the duration of the auth request in ms at debug level, and warns if over 100ms.
func logAuthDuration(logger polylog.Logger, portalAppID string, start time.Time) {
	elapsed := time.Since(start)
	elapsedMs := float64(elapsed.Microseconds()) / 1000.0
	logger = logger.With("portal_app_id", portalAppID)
	logger.Debug().Float64("auth_request_duration_ms", elapsedMs).Msg("auth request processed")
	if elapsed > 100*time.Millisecond {
		logger.Warn().Float64("auth_request_duration_ms", elapsedMs).Msg("auth request took over 100ms")
	}
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
// - Returns the PortalApp and a bool indicating if it was found.
func (a *AuthHandler) getPortalApp(portalAppID store.PortalAppID) (*store.PortalApp, bool) {
	return a.PortalAppStore.GetPortalApp(portalAppID)
}

// authPortalApp performs all configured authorization checks on the request.
// - Returns nil if no authorization is required (Auth is nil or APIKey is empty)
// - Otherwise, performs API Key authorization
func (a *AuthHandler) authPortalApp(headers http.Header, portalApp *store.PortalApp) error {
	if portalApp.Auth == nil || portalApp.Auth.APIKey == "" {
		return nil
	}
	return a.APIKeyAuthorizer.authorizeRequest(headers, portalApp)
}

// checkRateLimit checks if the account is rate limited.
// - Returns nil if the account is not eligible for rate limiting.
// - Returns an error if the account is rate limited.
func (a *AuthHandler) checkAccountRateLimit(portalApp *store.PortalApp) error {
	if portalApp.RateLimit == nil {
		return nil
	}
	if a.RateLimitStore.IsAccountRateLimited(portalApp.AccountID) {
		return fmt.Errorf("account is rate limited")
	}
	return nil
}

// getHTTPHeaders sets all HTTP headers required by the PATH services on the request being forwarded.
// - Adds portal app ID header on all requests ("Portal-Application-ID: <id>")
// - Adds account ID header on all requests ("Portal-Account-ID: <id>")
// - Adds rate limit header if applicable
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
// - Sets PermissionDenied code and error message in response.
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
// - Sets OK code and attaches provided headers to response.
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
