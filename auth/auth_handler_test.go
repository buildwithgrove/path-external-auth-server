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

	"github.com/buildwithgrove/path-external-auth-server/ratelimit"
	"github.com/buildwithgrove/path-external-auth-server/store"
)

func Test_Check(t *testing.T) {
	tests := []struct {
		name                string
		checkReq            *envoy_auth.CheckRequest
		expectedResp        *envoy_auth.CheckResponse
		portalAppID         store.PortalAppID
		mockPortalAppReturn *store.PortalApp
	}{
		{
			name: "should return OK check response if check request is valid and user is authorized to access portal app with rate limit headers set",
			checkReq: &envoy_auth.CheckRequest{
				Attributes: &envoy_auth.AttributeContext{
					Request: &envoy_auth.AttributeContext_Request{
						Http: &envoy_auth.AttributeContext_HttpRequest{
							Path: "/v1/portal_app_free",
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
							{Header: &envoy_core.HeaderValue{Key: reqHeaderPortalAppID, Value: "portal_app_free"}},
							{Header: &envoy_core.HeaderValue{Key: reqHeaderAccountID, Value: "account_1"}},
							{Header: &envoy_core.HeaderValue{Key: string(ratelimit.PlanFree_RequestHeader), Value: "account_1"}},
						},
					},
				},
			},
			portalAppID: "portal_app_free",
			mockPortalAppReturn: &store.PortalApp{
				ID:        "portal_app_free",
				AccountID: "account_1",
				Auth:      nil, // No auth required
				RateLimit: &store.RateLimit{
					PlanType: ratelimit.PlanFree_DatabaseType,
				},
			},
		},
		{
			name: "should return OK check response if check request is valid and user is authorized to access portal app with no rate limit headers set",
			checkReq: &envoy_auth.CheckRequest{
				Attributes: &envoy_auth.AttributeContext{
					Request: &envoy_auth.AttributeContext_Request{
						Http: &envoy_auth.AttributeContext_HttpRequest{
							Path: "/v1/portal_app_unlimited",
							Headers: map[string]string{
								authHeaderKey: "api_key_good",
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
							{Header: &envoy_core.HeaderValue{Key: reqHeaderPortalAppID, Value: "portal_app_unlimited"}},
							{Header: &envoy_core.HeaderValue{Key: reqHeaderAccountID, Value: "account_2"}},
						},
					},
				},
			},
			portalAppID: "portal_app_unlimited",
			mockPortalAppReturn: &store.PortalApp{
				ID:        "portal_app_unlimited",
				AccountID: "account_2",
				Auth: &store.Auth{
					APIKey: "api_key_good",
				},
				RateLimit: nil, // No rate limiting
			},
		},
		{
			name: "should return ok check response if portal app requires API key auth",
			checkReq: &envoy_auth.CheckRequest{
				Attributes: &envoy_auth.AttributeContext{
					Request: &envoy_auth.AttributeContext_Request{
						Http: &envoy_auth.AttributeContext_HttpRequest{
							Path: "/v1/portal_app_api_key",
							Headers: map[string]string{
								authHeaderKey: "api_key_good",
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
							{Header: &envoy_core.HeaderValue{Key: reqHeaderPortalAppID, Value: "portal_app_api_key"}},
							{Header: &envoy_core.HeaderValue{Key: reqHeaderAccountID, Value: "account_3"}},
						},
					},
				},
			},
			portalAppID: "portal_app_api_key",
			mockPortalAppReturn: &store.PortalApp{
				ID:        "portal_app_api_key",
				AccountID: "account_3",
				Auth: &store.Auth{
					APIKey: "api_key_good",
				},
			},
		},
		{
			name: "should return ok check response if portal app does not require auth",
			checkReq: &envoy_auth.CheckRequest{
				Attributes: &envoy_auth.AttributeContext{
					Request: &envoy_auth.AttributeContext_Request{
						Http: &envoy_auth.AttributeContext_HttpRequest{
							Path: "/v1/portal_app_public",
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
							{Header: &envoy_core.HeaderValue{Key: reqHeaderPortalAppID, Value: "portal_app_public"}},
							{Header: &envoy_core.HeaderValue{Key: reqHeaderAccountID, Value: "account_4"}},
						},
					},
				},
			},
			portalAppID: "portal_app_public",
			mockPortalAppReturn: &store.PortalApp{
				ID:        "portal_app_public",
				AccountID: "account_4",
				Auth:      nil, // No auth required
			},
		},
		{
			name: "should return ok check response if portal app ID is passed via header",
			checkReq: &envoy_auth.CheckRequest{
				Attributes: &envoy_auth.AttributeContext{
					Request: &envoy_auth.AttributeContext_Request{
						Http: &envoy_auth.AttributeContext_HttpRequest{
							Path: "/v1",
							Headers: map[string]string{
								reqHeaderPortalAppID: "portal_app_id_from_header",
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
							{Header: &envoy_core.HeaderValue{Key: reqHeaderPortalAppID, Value: "portal_app_id_from_header"}},
							{Header: &envoy_core.HeaderValue{Key: reqHeaderAccountID, Value: "account_5"}},
						},
					},
				},
			},
			portalAppID: "portal_app_id_from_header",
			mockPortalAppReturn: &store.PortalApp{
				ID:        "portal_app_id_from_header",
				AccountID: "account_5",
				Auth:      nil, // No auth required
			},
		},
		{
			name: "should return denied check response if portal app not found",
			checkReq: &envoy_auth.CheckRequest{
				Attributes: &envoy_auth.AttributeContext{
					Request: &envoy_auth.AttributeContext_Request{
						Http: &envoy_auth.AttributeContext_HttpRequest{
							Path: "/v1/portal_app_not_found",
						},
					},
				},
			},
			expectedResp: &envoy_auth.CheckResponse{
				Status: &status.Status{
					Code:    int32(codes.PermissionDenied),
					Message: "portal app not found",
				},
				HttpResponse: &envoy_auth.CheckResponse_DeniedResponse{
					DeniedResponse: &envoy_auth.DeniedHttpResponse{
						Status: &envoy_type.HttpStatus{
							Code: envoy_type.StatusCode_NotFound,
						},
						Body: `{"code": 404, "message": "portal app not found"}`,
					},
				},
			},
			portalAppID:         "portal_app_not_found",
			mockPortalAppReturn: nil,
		},
		{
			name: "should return denied check response if user is not authorized to access portal app using API key auth",
			checkReq: &envoy_auth.CheckRequest{
				Attributes: &envoy_auth.AttributeContext{
					Request: &envoy_auth.AttributeContext_Request{
						Http: &envoy_auth.AttributeContext_HttpRequest{
							Path: "/v1/portal_app_api_key",
							Headers: map[string]string{
								authHeaderKey: "api_key_123",
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
			portalAppID: "portal_app_api_key",
			mockPortalAppReturn: &store.PortalApp{
				ID: "portal_app_api_key",
				Auth: &store.Auth{
					APIKey: "api_key_not_this_one",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := require.New(t)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := NewMockPortalAppStore(ctrl)
			if test.portalAppID != "" {
				mockStore.EXPECT().GetPortalApp(test.portalAppID).Return(test.mockPortalAppReturn, test.mockPortalAppReturn != nil)
			}

			authHandler := &AuthHandler{
				Logger: polyzero.NewLogger(),

				PortalAppStore:   mockStore,
				APIKeyAuthorizer: &AuthorizerAPIKey{},
			}

			resp, err := authHandler.Check(context.Background(), test.checkReq)
			c.NoError(err)
			c.Equal(test.expectedResp, resp)
		})
	}
}
