package main

import (
	"fmt"
	"net"

	envoy_auth "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	"github.com/pokt-network/poktroll/pkg/polylog"
	"github.com/pokt-network/poktroll/pkg/polylog/polyzero"
	"google.golang.org/grpc"

	"github.com/buildwithgrove/path-external-auth-server/auth"
	"github.com/buildwithgrove/path-external-auth-server/postgres/grove"
	"github.com/buildwithgrove/path-external-auth-server/store"
)

func main() {
	fmt.Println("🫛  Starting PEAS (Path External Auth Server) ...")

	env, err := gatherEnvVars()
	if err != nil {
		panic(fmt.Errorf("failed to gather environment variables: %v", err))
	}
	fmt.Println("💻 Log Level: ", env.loggerLevel)

	loggerOpts := []polylog.LoggerOption{
		polyzero.WithLevel(polyzero.ParseLevel(env.loggerLevel)),
	}

	// Initialize new polylog logger
	logger := polyzero.NewLogger(loggerOpts...)

	// Create a new postgres data source
	postgresDataSource, err := grove.NewGrovePostgresDriver(
		logger,
		env.postgresConnectionString,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to postgres: %v", err))
	}
	defer postgresDataSource.Close()

	logger.Info().Msg("🐘 Successfully connected to postgres as a data source")

	// Create a new portal app store
	portalAppStore, err := store.NewPortalAppStore(logger, postgresDataSource, env.refreshInterval)
	if err != nil {
		panic(err)
	}

	logger.Info().Msg("Successfully initialized portal app store")

	// Create a new listener to listen for requests from GUARD
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", env.port))
	if err != nil {
		panic(err)
	}

	// Create a new AuthHandler to handle the request auth
	authHandler := &auth.AuthHandler{
		Logger:           logger,
		PortalAppStore:   portalAppStore,
		APIKeyAuthorizer: &auth.AuthorizerAPIKey{},
	}

	// Create a new gRPC server for handling auth requests from GUARD
	// using Envoy Proxy's `ext_authz` HTTP Filter.
	//
	// See:
	//    - https://gateway.envoyproxy.io/docs/tasks/security/ext-auth/
	//    - https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/ext_authz_filter
	grpcServer := grpc.NewServer()

	// Register proto server
	envoy_auth.RegisterAuthorizationServer(grpcServer, authHandler)

	fmt.Printf("Auth server starting on port %d...\n", env.port)
	if err = grpcServer.Serve(listen); err != nil {
		panic(err)
	}
}
