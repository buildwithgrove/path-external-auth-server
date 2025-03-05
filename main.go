package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	envoy_auth "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	_ "github.com/joho/godotenv/autoload" // autoload env vars
	"github.com/pokt-network/poktroll/pkg/polylog/polyzero"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/buildwithgrove/path-external-auth-server/auth"
	store "github.com/buildwithgrove/path-external-auth-server/endpoint_store"
	"github.com/buildwithgrove/path-external-auth-server/proto"
)

func main() {
	// Initialize new polylog logger
	logger := polyzero.NewLogger()

	env, err := gatherEnvVars()
	if err != nil {
		panic(fmt.Errorf("failed to gather environment variables: %v", err))
	}

	// Connect to the gRPC server for the GatewayEndpoints service
	conn, err := connectGRPC(env.grpcHostPort, env.grpcUseInsecureCredentials)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to gRPC server: %v", err))
	}
	defer conn.Close()

	// Create a new gRPC client for the GatewayEndpoints service
	grpcClient := proto.NewGatewayEndpointsClient(conn)

	// Create a new endpoint store
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	endpointStore, err := store.NewEndpointStore(ctx, logger, grpcClient)
	if err != nil {
		panic(err)
	}

	// Create a new listener to listen for requests from Envoy
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", env.port))
	if err != nil {
		panic(err)
	}

	// Create a new AuthHandler to handle the request auth
	authHandler := &auth.AuthHandler{
		Logger: logger,

		EndpointStore:    endpointStore,
		APIKeyAuthorizer: &auth.APIKeyAuthorizer{},
		JWTAuthorizer:    &auth.JWTAuthorizer{},
	}

	// Create a new gRPC server for handling auth requests from Envoy
	grpcServer := grpc.NewServer()

	// Register envoy proto server
	envoy_auth.RegisterAuthorizationServer(grpcServer, authHandler)

	fmt.Printf("Auth server starting on port %d...\n", env.port)
	if err = grpcServer.Serve(listen); err != nil {
		panic(err)
	}
}

/* -------------------- Gateway Init Helpers -------------------- */

// connectGRPC connects to the gRPC server for the GatewayEndpoints service
// and returns a gRPC client connection.
func connectGRPC(hostPort string, useInsecureCredentials bool) (*grpc.ClientConn, error) {
	var creds credentials.TransportCredentials
	if useInsecureCredentials {
		creds = insecure.NewCredentials()
	} else {
		creds = credentials.NewTLS(&tls.Config{})
	}
	return grpc.NewClient(hostPort, grpc.WithTransportCredentials(creds))
}
