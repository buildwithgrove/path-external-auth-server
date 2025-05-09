package endpointstore

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/pokt-network/poktroll/pkg/polylog/polyzero"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	protoPkg "google.golang.org/protobuf/proto"

	"github.com/buildwithgrove/path-external-auth-server/proto"
)

// MockStream is a mock implementation of the grpc.ClientStream interface.
type MockStream struct {
	grpc.ClientStream
	updates chan *proto.AuthDataUpdate
	closed  bool
}

func (m *MockStream) Recv() (*proto.AuthDataUpdate, error) {
	// Don't block forever if the channel is empty - this prevents test timeouts
	select {
	case update, ok := <-m.updates:
		if !ok || update == nil {
			m.closed = true
			return nil, io.EOF
		}
		return update, nil
	case <-time.After(100 * time.Millisecond):
		// To prevent the test from hanging, return EOF if no update is available in a reasonable time
		if m.closed {
			return nil, io.EOF
		}
		return nil, io.EOF
	}
}

func newTestStore(t *testing.T, ctx context.Context, updates chan *proto.AuthDataUpdate, ctrl *gomock.Controller) *endpointStore {
	mockClient := NewMockGatewayEndpointsClient(ctrl)

	// Set up the expected call for FetchAuthDataSync to be called multiple times
	// This is needed because our improved reconnection logic will call it on startup and when reconnecting
	mockClient.EXPECT().FetchAuthDataSync(gomock.Any(), gomock.Any()).Return(getTestGatewayEndpoints(), nil).AnyTimes()

	// Set up the expected call for StreamUpdates
	mockStream := &MockStream{updates: updates}
	mockClient.EXPECT().StreamAuthDataUpdates(gomock.Any(), gomock.Any()).Return(mockStream, nil).AnyTimes()

	store, err := NewEndpointStore(ctx, polyzero.NewLogger(), mockClient)
	require.NoError(t, err)

	return store
}

func Test_GetGatewayEndpoint(t *testing.T) {
	tests := []struct {
		name                    string
		endpointID              string
		expectedGatewayEndpoint *proto.GatewayEndpoint
		expectedEndpointFound   bool
		update                  *proto.AuthDataUpdate
	}{
		{
			name:                    "should return gateway endpoint when found",
			endpointID:              "endpoint_1_static_key",
			expectedGatewayEndpoint: getTestGatewayEndpoints().Endpoints["endpoint_1_static_key"],
			expectedEndpointFound:   true,
		},
		{
			name:                    "should return different gateway endpoint when found",
			endpointID:              "endpoint_2_no_auth",
			expectedGatewayEndpoint: getTestGatewayEndpoints().Endpoints["endpoint_2_no_auth"],
			expectedEndpointFound:   true,
		},
		{
			name:                    "should return brand new gateway endpoint when update is received for new endpoint",
			endpointID:              "endpoint_3_static_key",
			update:                  getTestUpdate("endpoint_3_static_key"),
			expectedGatewayEndpoint: getTestUpdate("endpoint_3_static_key").GatewayEndpoint,
			expectedEndpointFound:   true,
		},
		{
			name:                    "should return updated existing gateway endpoint when update is received for existing endpoint",
			endpointID:              "endpoint_2_no_auth",
			update:                  getTestUpdate("endpoint_2_no_auth"),
			expectedGatewayEndpoint: getTestUpdate("endpoint_2_no_auth").GatewayEndpoint,
			expectedEndpointFound:   true,
		},
		{
			name:                    "should not return gateway endpoint when update is received to delete endpoint",
			endpointID:              "endpoint_1_static_key",
			update:                  getTestUpdate("endpoint_1_static_key"),
			expectedGatewayEndpoint: nil,
			expectedEndpointFound:   false,
		},
		{
			name:                    "should return false when gateway endpoint not found",
			endpointID:              "endpoint_3_static_key",
			expectedGatewayEndpoint: nil,
			expectedEndpointFound:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := require.New(t)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Create buffered channel to prevent blocking
			updates := make(chan *proto.AuthDataUpdate, 10)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			store := newTestStore(t, ctx, updates, ctrl)

			// Send updates for this test case
			if test.update != nil {
				updates <- test.update
				// Allow time for update to be processed
				time.Sleep(50 * time.Millisecond)
			}

			gatewayEndpoint, found := store.GetGatewayEndpoint(test.endpointID)
			c.Equal(test.expectedEndpointFound, found)
			c.True(protoPkg.Equal(test.expectedGatewayEndpoint, gatewayEndpoint), "expected and actual GatewayEndpoint do not match")

			// Close the channel to end the test cleanly
			close(updates)
		})
	}
}

// getTestGatewayEndpoints returns a mock response for the initial endpoint store data,
// received when the endpoint store is first created.
func getTestGatewayEndpoints() *proto.AuthDataResponse {
	return &proto.AuthDataResponse{
		Endpoints: map[string]*proto.GatewayEndpoint{
			"endpoint_1_static_key": {
				EndpointId: "endpoint_1_static_key",
				Auth: &proto.Auth{
					AuthType: &proto.Auth_StaticApiKey{
						StaticApiKey: &proto.StaticAPIKey{
							ApiKey: "api_key_1",
						},
					},
				},
				Metadata: &proto.Metadata{
					AccountId: "account_1",
					PlanType:  "PLAN_UNLIMITED",
					Email:     "amos.burton@opa.belt",
				},
			},
			"endpoint_2_no_auth": {
				EndpointId: "endpoint_2_no_auth",
				Auth: &proto.Auth{
					AuthType: &proto.Auth_NoAuth{},
				},
				Metadata: &proto.Metadata{
					AccountId: "account_2",
					PlanType:  "PLAN_UNLIMITED",
					Email:     "paul.atreides@arrakis.com",
				},
			},
		},
	}
}

// getTestUpdate returns a mock update for a given endpoint ID, used to test the endpoint store's behavior when updates are received.
// Will be one of three cases:
// 1. An existing GatewayEndpoint was updated (endpoint_2_no_auth)
// 2. A new GatewayEndpoint was created (endpoint_3_static_key)
// 3. An existing GatewayEndpoint was deleted (endpoint_1_static_key)
func getTestUpdate(endpointID string) *proto.AuthDataUpdate {
	updatesMap := map[string]*proto.AuthDataUpdate{
		"endpoint_2_no_auth": {
			EndpointId: "endpoint_2_no_auth",
			GatewayEndpoint: &proto.GatewayEndpoint{
				EndpointId: "endpoint_2_no_auth",
				Auth: &proto.Auth{
					AuthType: &proto.Auth_NoAuth{},
				},
				Metadata: &proto.Metadata{
					AccountId: "account_2",
					PlanType:  "PLAN_UNLIMITED",
					Email:     "paul.atreides@arrakis.com",
				},
			},
			Delete: false,
		},
		"endpoint_3_static_key": {
			EndpointId: "endpoint_3_static_key",
			GatewayEndpoint: &proto.GatewayEndpoint{
				EndpointId: "endpoint_3_static_key",
				Auth: &proto.Auth{
					AuthType: &proto.Auth_StaticApiKey{
						StaticApiKey: &proto.StaticAPIKey{
							ApiKey: "new_api_key",
						},
					},
				},
				Metadata: &proto.Metadata{
					AccountId: "account_3",
					PlanType:  "PLAN_PRO",
					Email:     "new.email@example.com",
				},
			},
			Delete: false,
		},
		"endpoint_1_static_key": {
			EndpointId: "endpoint_1_static_key",
			Delete:     true,
		},
	}

	return updatesMap[endpointID]
}
