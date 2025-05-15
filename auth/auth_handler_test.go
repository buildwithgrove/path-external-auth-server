package auth

import (
	"context"
	"fmt"
	"testing"

	envoy_core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_auth "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	envoy_type "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/pokt-network/poktroll/pkg/polylog/polyzero"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"

	"github.com/buildwithgrove/path-external-auth-server/proto"
	"github.com/buildwithgrove/path-external-auth-server/ratelimit"
)

func Test_Check(t *testing.T) {
	tests := []struct {
		name               string
		checkReq           *envoy_auth.CheckRequest
		expectedResp       *envoy_auth.CheckResponse
		endpointID         string
		mockEndpointReturn *proto.GatewayEndpoint
	}{
		{
			name: "should return OK check response if check request is valid and user is authorized to access endpoint with rate limit headers set",
			checkReq: &envoy_auth.CheckRequest{
				Attributes: &envoy_auth.AttributeContext{
					Request: &envoy_auth.AttributeContext_Request{
						Http: &envoy_auth.AttributeContext_HttpRequest{
							Path: "/v1/endpoint_free",
						},
					},
				},
			},
			expectedResp: &envoy_auth.CheckResponse{
				Status: &status.Status{
					Code:    int32(codes.OK),
					Message: "ok",
				},
				HttpResponse: &envoy_auth.CheckResponse_OkResponse{
					OkResponse: &envoy_auth.OkHttpResponse{
						Headers: []*envoy_core.HeaderValueOption{
							{Header: &envoy_core.HeaderValue{Key: reqHeaderEndpointID, Value: "endpoint_free"}},
							{Header: &envoy_core.HeaderValue{Key: reqHeaderAccountID, Value: "account_1"}},
							{Header: &envoy_core.HeaderValue{Key: ratelimit.PlanFree_RequestHeader, Value: "endpoint_free"}},
						},
					},
				},
			},
			endpointID: "endpoint_free",
			mockEndpointReturn: &proto.GatewayEndpoint{
				EndpointId: "endpoint_free",
				Auth: &proto.Auth{
					AuthType: &proto.Auth_NoAuth{},
				},
				Metadata: &proto.Metadata{
					AccountId: "account_1",
					PlanType:  ratelimit.PlanFree_DatabaseType,
				},
			},
		},
		{
			name: "should return OK check response if check request is valid and user is authorized to access endpoint with no rate limit headers set",
			checkReq: &envoy_auth.CheckRequest{
				Attributes: &envoy_auth.AttributeContext{
					Request: &envoy_auth.AttributeContext_Request{
						Http: &envoy_auth.AttributeContext_HttpRequest{
							Path: "/v1/endpoint_unlimited",
							Headers: map[string]string{
								reqHeaderAPIKey: "api_key_good",
							},
						},
					},
				},
			},
			expectedResp: &envoy_auth.CheckResponse{
				Status: &status.Status{
					Code:    int32(codes.OK),
					Message: "ok",
				},
				HttpResponse: &envoy_auth.CheckResponse_OkResponse{
					OkResponse: &envoy_auth.OkHttpResponse{
						Headers: []*envoy_core.HeaderValueOption{
							{Header: &envoy_core.HeaderValue{Key: reqHeaderEndpointID, Value: "endpoint_unlimited"}},
							{Header: &envoy_core.HeaderValue{Key: reqHeaderAccountID, Value: "account_2"}},
						},
					},
				},
			},
			endpointID: "endpoint_unlimited",
			mockEndpointReturn: &proto.GatewayEndpoint{
				EndpointId: "endpoint_unlimited",
				Auth: &proto.Auth{
					AuthType: &proto.Auth_StaticApiKey{
						StaticApiKey: &proto.StaticAPIKey{
							ApiKey: "api_key_good",
						},
					},
				},
				Metadata: &proto.Metadata{
					AccountId: "account_2",
					PlanType:  "PLAN_UNLIMITED",
				},
			},
		},
		{
			name: "should return ok check response if endpoint requires API key auth",
			checkReq: &envoy_auth.CheckRequest{
				Attributes: &envoy_auth.AttributeContext{
					Request: &envoy_auth.AttributeContext_Request{
						Http: &envoy_auth.AttributeContext_HttpRequest{
							Path: "/v1/api_key_endpoint",
							Headers: map[string]string{
								reqHeaderAPIKey: "api_key_good",
							},
						},
					},
				},
			},
			expectedResp: &envoy_auth.CheckResponse{
				Status: &status.Status{
					Code:    int32(codes.OK),
					Message: "ok",
				},
				HttpResponse: &envoy_auth.CheckResponse_OkResponse{
					OkResponse: &envoy_auth.OkHttpResponse{
						Headers: []*envoy_core.HeaderValueOption{
							{Header: &envoy_core.HeaderValue{Key: reqHeaderEndpointID, Value: "api_key_endpoint"}},
							{Header: &envoy_core.HeaderValue{Key: reqHeaderAccountID, Value: "account_3"}},
						},
					},
				},
			},
			endpointID: "api_key_endpoint",
			mockEndpointReturn: &proto.GatewayEndpoint{
				EndpointId: "api_key_endpoint",
				Auth: &proto.Auth{
					AuthType: &proto.Auth_StaticApiKey{
						StaticApiKey: &proto.StaticAPIKey{
							ApiKey: "api_key_good",
						},
					},
				},
				Metadata: &proto.Metadata{
					AccountId: "account_3",
				},
			},
		},
		{
			name: "should return ok check response if endpoint does not require auth",
			checkReq: &envoy_auth.CheckRequest{
				Attributes: &envoy_auth.AttributeContext{
					Request: &envoy_auth.AttributeContext_Request{
						Http: &envoy_auth.AttributeContext_HttpRequest{
							Path: "/v1/public_endpoint",
						},
					},
				},
			},
			expectedResp: &envoy_auth.CheckResponse{
				Status: &status.Status{
					Code:    int32(codes.OK),
					Message: "ok",
				},
				HttpResponse: &envoy_auth.CheckResponse_OkResponse{
					OkResponse: &envoy_auth.OkHttpResponse{
						Headers: []*envoy_core.HeaderValueOption{
							{Header: &envoy_core.HeaderValue{Key: reqHeaderEndpointID, Value: "public_endpoint"}},
							{Header: &envoy_core.HeaderValue{Key: reqHeaderAccountID, Value: "account_4"}},
						},
					},
				},
			},
			endpointID: "public_endpoint",
			mockEndpointReturn: &proto.GatewayEndpoint{
				EndpointId: "public_endpoint",
				Auth: &proto.Auth{
					AuthType: &proto.Auth_NoAuth{
						NoAuth: &proto.NoAuth{},
					},
				},
				Metadata: &proto.Metadata{
					AccountId: "account_4",
				},
			},
		},
		{
			name: "should return ok check response if endpoint ID is passed via header",
			checkReq: &envoy_auth.CheckRequest{
				Attributes: &envoy_auth.AttributeContext{
					Request: &envoy_auth.AttributeContext_Request{
						Http: &envoy_auth.AttributeContext_HttpRequest{
							Path: "/v1",
							Headers: map[string]string{
								reqHeaderEndpointID: "endpoint_id_from_header",
							},
						},
					},
				},
			},
			expectedResp: &envoy_auth.CheckResponse{
				Status: &status.Status{
					Code:    int32(codes.OK),
					Message: "ok",
				},
				HttpResponse: &envoy_auth.CheckResponse_OkResponse{
					OkResponse: &envoy_auth.OkHttpResponse{
						Headers: []*envoy_core.HeaderValueOption{
							{Header: &envoy_core.HeaderValue{Key: reqHeaderEndpointID, Value: "endpoint_id_from_header"}},
							{Header: &envoy_core.HeaderValue{Key: reqHeaderAccountID, Value: "account_5"}},
						},
					},
				},
			},
			endpointID: "endpoint_id_from_header",
			mockEndpointReturn: &proto.GatewayEndpoint{
				EndpointId: "endpoint_id_from_header",
				Auth: &proto.Auth{
					AuthType: &proto.Auth_NoAuth{
						NoAuth: &proto.NoAuth{},
					},
				},
				Metadata: &proto.Metadata{
					AccountId: "account_5",
				},
			},
		},
		{
			name: "should return denied check response if gateway endpoint not found",
			checkReq: &envoy_auth.CheckRequest{
				Attributes: &envoy_auth.AttributeContext{
					Request: &envoy_auth.AttributeContext_Request{
						Http: &envoy_auth.AttributeContext_HttpRequest{
							Path: "/v1/endpoint_not_found",
						},
					},
				},
			},
			expectedResp: &envoy_auth.CheckResponse{
				Status: &status.Status{
					Code:    int32(codes.PermissionDenied),
					Message: "endpoint not found",
				},
				HttpResponse: &envoy_auth.CheckResponse_DeniedResponse{
					DeniedResponse: &envoy_auth.DeniedHttpResponse{
						Status: &envoy_type.HttpStatus{
							Code: envoy_type.StatusCode_NotFound,
						},
						Body: `{"code": 404, "message": "endpoint not found"}`,
					},
				},
			},
			endpointID:         "endpoint_not_found",
			mockEndpointReturn: &proto.GatewayEndpoint{},
		},
		{
			name: "should return denied check response if user is not authorized to access endpoint using API key auth",
			checkReq: &envoy_auth.CheckRequest{
				Attributes: &envoy_auth.AttributeContext{
					Request: &envoy_auth.AttributeContext_Request{
						Http: &envoy_auth.AttributeContext_HttpRequest{
							Path: "/v1/endpoint_api_key",
							Headers: map[string]string{
								reqHeaderAPIKey: "api_key_123",
							},
						},
					},
				},
			},
			expectedResp: &envoy_auth.CheckResponse{
				Status: &status.Status{
					Code:    int32(codes.PermissionDenied),
					Message: errUnauthorized.Error(),
				},
				HttpResponse: &envoy_auth.CheckResponse_DeniedResponse{
					DeniedResponse: &envoy_auth.DeniedHttpResponse{
						Status: &envoy_type.HttpStatus{
							Code: envoy_type.StatusCode_Unauthorized,
						},
						Body: fmt.Sprintf(`{"code": 401, "message": "%s"}`, errUnauthorized.Error()),
					},
				},
			},
			endpointID: "endpoint_api_key",
			mockEndpointReturn: &proto.GatewayEndpoint{
				EndpointId: "endpoint_api_key",
				Auth: &proto.Auth{
					AuthType: &proto.Auth_StaticApiKey{
						StaticApiKey: &proto.StaticAPIKey{
							ApiKey: "api_key_not_this_one",
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := require.New(t)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := NewMockEndpointStore(ctrl)
			if test.endpointID != "" {
				mockStore.EXPECT().GetGatewayEndpoint(test.endpointID).Return(test.mockEndpointReturn, test.mockEndpointReturn.EndpointId != "")
			}

			authHandler := &AuthHandler{
				Logger: polyzero.NewLogger(),

				EndpointStore:    mockStore,
				APIKeyAuthorizer: &APIKeyAuthorizer{},
			}

			resp, err := authHandler.Check(context.Background(), test.checkReq)
			c.NoError(err)
			c.Equal(test.expectedResp, resp)
		})
	}
}
