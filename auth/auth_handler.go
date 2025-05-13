// The auth package contains the implementation of the Envoy External Authorization gRPC service.
// It is responsible for receiving requests from Envoy and authorizing them based on the PortalApp
// data stored in the portalappstore package. It receives a check request from GUARD and determines if
// the request should be authorized.
package auth

import (
	"context"
	"fmt"

	envoy_core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_auth "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	envoy_type "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/pokt-network/poktroll/pkg/polylog"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"

	store "github.com/buildwithgrove/path-external-auth-server/portal_app_store"
	"github.com/buildwithgrove/path-external-auth-server/ratelimit"
)

const (
	// TODO_TECHDEBT(@commoddity): This path segment should be configurable via a single source of truth.
	// Not sure the best way to do this as it is referred to in multiple disparate places (eg. GUARD Helm charts, PATH's router.go & here)
	pathPrefix = "/v1/"

	// The portal app and account id need to match PATH's expected HTTP headers.
	// See the following code section in PATH:
	// https://github.com/buildwithgrove/path/blob/1e7b2d83294e8c406479ae5e480f4dca97414cee/gateway/observation.go#L16-L18
	reqHeaderPortalAppID = "Portal-Application-ID" // Set on all service requests
	reqHeaderAccountID   = "Portal-Account-ID"     // Set on all service requests

	errBody = `{"code": %d, "message": "%s"}`
)

// The PortalAppStore interface contains an in-memory store of PortalApps.
//
// It is used to allow fast lookups of authorization data for PATH when processing requests.
type PortalAppStore interface {
	GetPortalApp(portalAppID store.PortalAppID) (*store.PortalApp, bool)
}

// The AuthHandler struct contains the methods for processing requests from Envoy,
// primarily the Check method that is called by Envoy for each request.
type AuthHandler struct {
	Logger polylog.Logger

	// The PortalAppStore contains an in-memory store of PortalApps
	PortalAppStore PortalAppStore

	// The authorizers to be used for the request
	APIKeyAuthorizer Authorizer
}

// Check satisfies the implementation of the Envoy External Authorization gRPC service.
// It performs the following steps:
// - Extracts the portal app ID from the path
// - Extracts the account user ID from the headers
// - Fetches the PortalApp from the database
// - Performs all configured authorization checks
// - Returns a response with the HTTP headers set
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

	// Get the request headers
	headers := req.GetHeaders()

	// Extract the portal app ID from the request
	// It may be extracted from the URL path or the headers
	portalAppID, err := extractPortalAppID(req)
	if err != nil {
		a.Logger.Info().Err(err).Msg("unable to extract portal app ID from request")
		return getDeniedCheckResponse(err.Error(), envoy_type.StatusCode_BadRequest), nil
	}

	logger := a.Logger.With("portal_app_id", portalAppID)
	logger.Debug().Msg("handling check request")

	// Fetch PortalApp from portal app store
	gatewayPortalApp, ok := a.getPortalApp(portalAppID)
	if !ok {
		logger.Info().Msg("specified portal app not found: rejecting the request.")
		return getDeniedCheckResponse("portal app not found", envoy_type.StatusCode_NotFound), nil
	}

	// Perform all configured authorization checks
	if err := a.authPortalApp(headers, gatewayPortalApp); err != nil {
		logger.Info().Err(err).Msg("request failed authorization: rejecting the request.")
		return getDeniedCheckResponse(err.Error(), envoy_type.StatusCode_Unauthorized), nil
	}

	// Add portal app ID and rate limiting values to the headers
	// to be passed upstream along the filter chain to the rate limiter.
	httpHeaders := a.getHTTPHeaders(gatewayPortalApp)

	// Return a valid response with the HTTP headers set
	return getOKCheckResponse(httpHeaders), nil
}

/* --------------------------------- Helpers -------------------------------- */

// getPortalApp fetches the PortalApp from the portal app store and a bool indicating if it was found
func (a *AuthHandler) getPortalApp(portalAppID store.PortalAppID) (*store.PortalApp, bool) {
	return a.PortalAppStore.GetPortalApp(portalAppID)
}

// authPortalApp performs all configured authorization checks on the request
func (a *AuthHandler) authPortalApp(headers map[string]string, gatewayPortalApp *store.PortalApp) error {
	// If the portal app has no authorization requirements, return no error
	if gatewayPortalApp.Auth == nil || gatewayPortalApp.Auth.APIKey == "" {
		return nil
	}

	// Otherwise, perform API Key authorization
	return a.APIKeyAuthorizer.authorizeRequest(headers, gatewayPortalApp)
}

// getHTTPHeaders sets all HTTP headers required by the PATH services on the request being forwarded
func (a *AuthHandler) getHTTPHeaders(gatewayPortalApp *store.PortalApp) []*envoy_core.HeaderValueOption {
	portalAppID := string(gatewayPortalApp.PortalAppID)
	accountID := string(gatewayPortalApp.AccountID)

	headers := []*envoy_core.HeaderValueOption{
		// Set portal app ID header on all requests
		// eg. "PortalApp-Id: a12b3c4d"
		{
			Header: &envoy_core.HeaderValue{
				Key:   reqHeaderPortalAppID,
				Value: portalAppID,
			},
		},
		// Set account ID header on all requests
		// eg. "Account-Id: 3f4g2js2"
		{
			Header: &envoy_core.HeaderValue{
				Key:   reqHeaderAccountID,
				Value: accountID,
			},
		},
	}

	// Check if portal app should be rate limited and add the rate limit header if so
	if gatewayPortalApp.RateLimit != nil {
		if rateLimitHeader := ratelimit.GetRateLimitHeader(portalAppID, gatewayPortalApp.RateLimit); rateLimitHeader != nil {
			headers = append(headers, rateLimitHeader)
		}
	}

	return headers
}

// getDeniedCheckResponse returns a CheckResponse with a denied status and error message
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

// getOKCheckResponse returns a CheckResponse with an OK status and the provided headers
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
