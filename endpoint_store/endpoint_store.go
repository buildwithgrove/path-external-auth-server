// The endpointstore package contains the implementation of an in-memory store that stores
// GatewayEndpoints and their associated data from PADS (PATH Auth Data Server).
// See: https://github.com/buildwithgrove/path-auth-data-server
//
// It fetches this data from the remote gRPC server through an initial store update
// on startup, then listens for updates from the remote gRPC server to update the store.
package endpointstore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/pokt-network/poktroll/pkg/polylog"

	"github.com/buildwithgrove/path-external-auth-server/auth"
	"github.com/buildwithgrove/path-external-auth-server/proto"
)

const reconnectDelay = time.Second * 2

// endpointStore is an in-memory store that stores gateway endpoints and their associated data.
type endpointStore struct {
	logger polylog.Logger

	grpcClient proto.GatewayEndpointsClient

	gatewayEndpoints   map[string]*proto.GatewayEndpoint
	gatewayEndpointsMu sync.RWMutex
}

// Enforce that the EndpointStore implements the endpointStore interface.
var _ auth.EndpointStore = &endpointStore{}

// NewEndpointStore creates a new endpoint store, which stores GatewayEndpoints in memory for fast access.
// It initializes the store by requesting data from a remote gRPC server and listens for updates from the remote server to update the store.
func NewEndpointStore(ctx context.Context, logger polylog.Logger, grpcClient proto.GatewayEndpointsClient) (*endpointStore, error) {
	store := &endpointStore{
		logger: logger.With("component", "endpoint_data_store"),

		grpcClient: grpcClient,

		gatewayEndpoints:   make(map[string]*proto.GatewayEndpoint),
		gatewayEndpointsMu: sync.RWMutex{},
	}

	// Use exponential backoff for initialization
	backoff := 500 * time.Millisecond
	maxBackoff := 10 * time.Second
	maxAttempts := 10
	attempt := 0

	for attempt < maxAttempts {
		attempt++

		// Check if context is done before attempting initialization
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		err := store.initializeStoreFromRemote(ctx)
		if err == nil {
			store.logger.Info().Int("attempt", attempt).Msg("successfully initialized endpoint store")
			break
		}

		if attempt == maxAttempts {
			store.logger.Error().Int("max_attempts", maxAttempts).Msg("exceeded maximum initialization attempts")
			return nil, fmt.Errorf("failed to initialize endpoint store after %d attempts: %w", maxAttempts, err)
		}

		store.logger.Warn().
			Err(err).
			Int("attempt", attempt).
			Int("max_attempts", maxAttempts).
			Dur("backoff", backoff).
			Msg("failed to initialize endpoint store, retrying...")

		// Wait with exponential backoff
		select {
		case <-time.After(backoff):
			// Increase backoff for next attempt
			backoff = time.Duration(float64(backoff) * 1.5)
			backoff = min(backoff, maxBackoff)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Start listening for updates from the remote gRPC server.
	go store.listenForRemoteUpdates(ctx)

	return store, nil
}

// GetGatewayEndpoint returns a GatewayEndpoint from the store and a bool indicating if it exists in the store.
func (c *endpointStore) GetGatewayEndpoint(endpointID string) (*proto.GatewayEndpoint, bool) {
	c.gatewayEndpointsMu.RLock()
	defer c.gatewayEndpointsMu.RUnlock()

	gatewayEndpoint, ok := c.gatewayEndpoints[endpointID]
	return gatewayEndpoint, ok
}

// initializeStoreFromRemote requests the initial data from the remote gRPC server to set the store.
func (c *endpointStore) initializeStoreFromRemote(ctx context.Context) error {
	gatewayEndpointsResponse, err := c.grpcClient.FetchAuthDataSync(ctx, &proto.AuthDataRequest{})
	if err != nil {
		return fmt.Errorf("failed to get initial data from remote server: %w", err)
	}

	c.gatewayEndpointsMu.Lock()
	defer c.gatewayEndpointsMu.Unlock()
	c.gatewayEndpoints = gatewayEndpointsResponse.GetEndpoints()

	return nil
}

// listenForRemoteUpdates listens for updates from the remote gRPC server and updates the store accordingly.
// Updates will be one of three cases:
// 1. A new GatewayEndpoint was created
// 2. An existing GatewayEndpoint was updated
// 3. An existing GatewayEndpoint was deleted
func (c *endpointStore) listenForRemoteUpdates(ctx context.Context) {
	backoff := reconnectDelay
	maxBackoff := 30 * time.Second

	for {
		// If the context is done, exit the goroutine
		select {
		case <-ctx.Done():
			c.logger.Info().Msg("context cancelled, stopping update stream")
			return
		default:
		}

		err := c.connectAndProcessUpdates(ctx)
		if err != nil {
			c.logger.Error().Err(err).Dur("backoff", backoff).Msg("error in update stream, retrying")

			// Wait for backoff period before reconnecting
			select {
			case <-time.After(backoff):
				// Increase backoff for next attempt with exponential backoff, capped at maxBackoff
				backoff = time.Duration(float64(backoff) * 1.5)
				backoff = min(backoff, maxBackoff)
			case <-ctx.Done():
				return
			}
		} else {
			// Reset backoff on successful connection
			backoff = reconnectDelay
			c.logger.Info().Msg("update stream ended normally, reconnecting")

			// Small delay to avoid hammering the server
			select {
			case <-time.After(100 * time.Millisecond):
			case <-ctx.Done():
				return
			}
		}
	}
}

// connectAndProcessUpdates connects to the remote gRPC server and processes updates from the server.
func (c *endpointStore) connectAndProcessUpdates(ctx context.Context) error {
	// First try to refresh the entire store when reconnecting
	// This ensures we have the latest state in case we missed updates while disconnected
	// If this fails, continue with stream anyway as it might work
	c.logger.Info().Msg("refreshing endpoint store before starting stream")
	err := c.initializeStoreFromRemote(ctx)
	if err != nil {
		c.logger.Warn().Err(err).Msg("failed to refresh endpoint store, continuing with existing data")
	} else {
		c.logger.Info().Msg("successfully refreshed endpoint store")
	}

	// Create a child context that we can cancel if needed
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	c.logger.Info().Msg("connecting to stream auth data updates")
	stream, err := c.grpcClient.StreamAuthDataUpdates(streamCtx, &proto.AuthDataUpdatesRequest{})
	if err != nil {
		return fmt.Errorf("failed to stream updates from remote server: %w", err)
	}

	c.logger.Info().Msg("connected to stream auth data updates")

	for {
		select {
		case <-ctx.Done():
			c.logger.Info().Msg("parent context cancelled, stopping update stream")
			return nil
		default:
			update, err := stream.Recv()
			if err == io.EOF {
				c.logger.Info().Msg("update stream ended, attempting to reconnect")
				return nil // Return to trigger a reconnection
			}
			if err != nil {
				c.logger.Error().Err(err).Msg("error receiving update")
				return fmt.Errorf("error receiving update: %w", err)
			}
			if update == nil {
				c.logger.Error().Msg("received nil update")
				continue
			}

			PrettyLog("DEBUG PEAS- received update", update)

			c.gatewayEndpointsMu.Lock()
			if update.Delete {
				delete(c.gatewayEndpoints, update.EndpointId)
				c.logger.Info().Str("endpoint_id", update.EndpointId).Msg("deleted gateway endpoint")
			} else {
				c.gatewayEndpoints[update.EndpointId] = update.GatewayEndpoint
				c.logger.Info().Str("endpoint_id", update.EndpointId).Msg("updated gateway endpoint")
			}
			c.gatewayEndpointsMu.Unlock()
		}
	}
}

func PrettyLog(args ...interface{}) {
	for _, arg := range args {
		var prettyJSON bytes.Buffer
		jsonArg, _ := json.Marshal(arg)
		str := string(jsonArg)
		_ = json.Indent(&prettyJSON, []byte(str), "", "    ")
		output := prettyJSON.String()

		fmt.Println(output)
	}
}
