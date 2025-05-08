// The auth package contains the implementation of the Envoy External Authorization gRPC service.
// It is responsible for receiving requests from Envoy and authorizing them based on the GatewayEndpoint
// data stored in the endpointstore package. It receives a check request from GUARD and determines if
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

	"github.com/buildwithgrove/path-external-auth-server/proto"
)

const (
	// TODO_TECHDEBT(@commoddity): This path segment should be configurable via a single source of truth.
	// Not sure the best way to do this as it is referred to in multiple disparate places (eg. GUARD Helm charts, PATH's router.go & here)
	pathPrefix = "/v1/"

	// The endpoint and account id need to match PATH's expected HTTP headers.
	// See the following code section in PATH:
	// https://github.com/buildwithgrove/path/blob/1e7b2d83294e8c406479ae5e480f4dca97414cee/gateway/observation.go#L16-L18
	reqHeaderEndpointID = "Portal-Application-ID" // Set on all service requests
	reqHeaderAccountID  = "Portal-Account-ID"     // Set on all service requests

	errBody = `{"code": %d, "message": "%s"}`
)

// Set only on endpoints with plan types that should be rate limited
// The key is the plan type as specified in the database.
// The value is the header key to be matched in the GUARD configuration.
//
// DEV_NOTE: New plans from the database should be added to this map.
//
// Documentation reference:
// https://gateway.envoyproxy.io/docs/tasks/traffic/global-rate-limit/#rate-limit-distinct-users-except-admin
const (
	dbPlanFree     = "PLAN_FREE"    // The plan type as specified in the database
	planFreeHeader = "Rl-Plan-Free" // The header key to be matched in the GUARD configuration
)

// Map used to convert the plan type as specified in the database to the
// header key to be matched in the GUARD configuration.
var rateLimitedPlanTypeHeaders = map[string]string{
	dbPlanFree: planFreeHeader,
}

// The EndpointStore interface contains an in-memory store of GatewayEndpoints
// and their associated data from the PADS (PATH Auth Data Server).
// See: https://github.com/buildwithgrove/path-auth-data-server
//
// It used to allow fast lookups of authorization data for PATH when processing requests.
type EndpointStore interface {
	GetGatewayEndpoint(endpointID string) (*proto.GatewayEndpoint, bool)
}

// The AuthHandler struct contains the methods for processing requests from Envoy,
// primarily the Check method that is called by Envoy for each request.
type AuthHandler struct {
	Logger polylog.Logger

	// The EndpointStore contains an in-memory store of GatewayEndpoints
	// and their associated data from the PADS (PATH Auth Data Server).
	EndpointStore EndpointStore

	// The authorizers to be used for the request
	APIKeyAuthorizer Authorizer
}

// Check satisfies the implementation of the Envoy External Authorization gRPC service.
// It performs the following steps:
// - Extracts the endpoint ID from the path
// - Extracts the account user ID from the headers
// - Fetches the GatewayEndpoint from the database
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

	// Extract the endpoint ID from the request
	// It may be extracted from the URL path or the headers
	endpointID, err := extractEndpointID(req)
	if err != nil {
		a.Logger.Info().Err(err).Msg("unable to extract endpoint ID from request")
		return getDeniedCheckResponse(err.Error(), envoy_type.StatusCode_BadRequest), nil
	}

	logger := a.Logger.With("endpoint_id", endpointID)
	logger.Debug().Msg("handling check request")

	// Fetch GatewayEndpoint from endpoint store
	gatewayEndpoint, ok := a.getGatewayEndpoint(endpointID)
	if !ok {
		logger.Info().Msg("specified endpoint not found: rejecting the request.")
		return getDeniedCheckResponse("endpoint not found", envoy_type.StatusCode_NotFound), nil
	}

	// Perform all configured authorization checks
	if err := a.authGatewayEndpoint(headers, gatewayEndpoint); err != nil {
		logger.Info().Err(err).Msg("request failed authorization: rejecting the request.")
		return getDeniedCheckResponse(err.Error(), envoy_type.StatusCode_Unauthorized), nil
	}

	// Add endpoint ID and rate limiting values to the headers
	// to be passed upstream along the filter chain to the rate limiter.
	httpHeaders := a.getHTTPHeaders(gatewayEndpoint)

	// Return a valid response with the HTTP headers set
	return getOKCheckResponse(httpHeaders), nil
}

/* --------------------------------- Helpers -------------------------------- */

// getGatewayEndpoint fetches the GatewayEndpoint from the endpoint store and a bool indicating if it was found
func (a *AuthHandler) getGatewayEndpoint(endpointID string) (*proto.GatewayEndpoint, bool) {
	return a.EndpointStore.GetGatewayEndpoint(endpointID)
}

// authGatewayEndpoint performs all configured authorization checks on the request
func (a *AuthHandler) authGatewayEndpoint(headers map[string]string, gatewayEndpoint *proto.GatewayEndpoint) error {
	// Get the authorization type for the gateway endpoint
	authType := gatewayEndpoint.GetAuth().GetAuthType()

	switch authType.(type) {
	case *proto.Auth_NoAuth:
		return nil // If the endpoint has no authorization requirements, return no error

	case *proto.Auth_StaticApiKey:
		return a.APIKeyAuthorizer.authorizeRequest(headers, gatewayEndpoint)

	default:
		return fmt.Errorf("invalid authorization type")
	}
}

// getHTTPHeaders sets all HTTP headers required by the PATH services on the request being forwarded
func (a *AuthHandler) getHTTPHeaders(gatewayEndpoint *proto.GatewayEndpoint) []*envoy_core.HeaderValueOption {
	endpointID := gatewayEndpoint.GetEndpointId()
	metadata := gatewayEndpoint.GetMetadata()

	headers := []*envoy_core.HeaderValueOption{
		// Set endpoint ID header on all requests
		// eg. "Endpoint-Id: a12b3c4d"
		{
			Header: &envoy_core.HeaderValue{
				Key:   reqHeaderEndpointID,
				Value: endpointID,
			},
		},
		// Set account ID header on all requests
		// eg. "Account-Id: 3f4g2js2"
		{
			Header: &envoy_core.HeaderValue{
				Key:   reqHeaderAccountID,
				Value: metadata.GetAccountId(),
			},
		},
	}

	// Set rate limit headers if the endpoint should be rate limited
	if planType := metadata.GetPlanType(); planType != "" {
		// Only plans that are configured to be rate limited should have a rate limit header
		if rateLimitHeader, ok := rateLimitedPlanTypeHeaders[planType]; ok {
			// Set the rate header with the endpoint ID
			// eg. "Rate-Limit-Plan-Free: a12b3c4d"
			headers = append(headers, &envoy_core.HeaderValueOption{
				Header: &envoy_core.HeaderValue{
					Key:   rateLimitHeader,
					Value: endpointID,
				},
			})
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
