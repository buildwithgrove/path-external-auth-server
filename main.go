package main

import (
	"context"
	"fmt"
	"net"

	envoy_auth "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	"github.com/pokt-network/poktroll/pkg/polylog"
	"github.com/pokt-network/poktroll/pkg/polylog/polyzero"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/buildwithgrove/path-external-auth-server/auth"
	"github.com/buildwithgrove/path-external-auth-server/dwh"
	"github.com/buildwithgrove/path-external-auth-server/postgres/grove"
	"github.com/buildwithgrove/path-external-auth-server/ratelimit"
	"github.com/buildwithgrove/path-external-auth-server/store"
)

func main() {
	// Gather environment variables
	env, err := gatherEnvVars()
	if err != nil {
		panic(fmt.Errorf("failed to gather environment variables: %v", err))
	}

	// Initialize new polylog logger
	loggerOpts := []polylog.LoggerOption{
		polyzero.WithLevel(polyzero.ParseLevel(env.loggerLevel)),
	}
	logger := polyzero.NewLogger(loggerOpts...)

	logger.Info().Str("logger_level", env.loggerLevel).
		Msg("🫛 Starting PEAS (Path External Auth Server) ...")

	// Create a new postgres data source
	postgresDataSource, err := grove.NewGrovePostgresDriver(
		logger, env.postgresConnectionString,
	)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to postgres: %v", err))
	}
	defer postgresDataSource.Close()
	logger.Info().Msg("🐘 Successfully connected to postgres as a data source")

	// Create a new data warehouse driver
	dataWarehouseDriver, err := dwh.NewDriver(context.Background(), env.gcpProjectID)
	if err != nil {
		panic(err)
	}
	defer dataWarehouseDriver.Close()
	logger.Info().Msg("💽 Successfully connected to data warehouse as a data source")

	// Create a new portal app store
	portalAppStore, err := store.NewPortalAppStore(
		logger,
		postgresDataSource,
		env.portalAppStoreRefreshInterval,
	)
	if err != nil {
		panic(err)
	}
	logger.Info().Msg("✅ Successfully initialized portal app store")

	// Create a new rate limit store
	rateLimitStore, err := ratelimit.NewRateLimitStore(
		logger,
		dataWarehouseDriver,
		portalAppStore,
		env.rateLimitStoreRefreshInterval,
	)
	if err != nil {
		panic(err)
	}
	logger.Info().Msg("✅ Successfully initialized rate limit store")

	// Create a new listener to listen for requests from GUARD
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", env.port))
	if err != nil {
		panic(err)
	}

	// Create a new AuthHandler to handle the request auth
	authHandler := auth.NewAuthHandler(
		logger,
		portalAppStore,
		rateLimitStore,
		&auth.AuthorizerAPIKey{},
	)

	// Create a new gRPC server for handling auth requests from GUARD
	// using Envoy Proxy's `ext_authz` HTTP Filter.
	//
	// See:
	//    - https://gateway.envoyproxy.io/docs/tasks/security/ext-auth/
	//    - https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/ext_authz_filter
	grpcServer := grpc.NewServer()

	// Register proto server
	envoy_auth.RegisterAuthorizationServer(grpcServer, authHandler)

	// Enable gRPC reflection for easy lookup of Portal Application/Account auth status.
	//
	// See README.md section: "Getting Portal App Auth & Rate Limit Status" for more details.
	//
	// Security Note: This is safe for Envoy External Auth servers because:
	// 1. The Envoy External Auth API is a public, standardized interface defined at:
	//    https://github.com/envoyproxy/envoy/blob/main/api/envoy/service/auth/v3/external_auth.proto
	// 2. The API specification is fully documented in Envoy's official documentation:
	//    https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/ext_authz_filter
	// 3. Reflection only exposes the standard CheckRequest/CheckResponse schema, not our
	//    business logic, database structure, or implementation details.
	// 4. This enables easy testing of auth/rate limit status for Portal accounts using
	//    grpcurl without requiring proto files or protosets.
	reflection.Register(grpcServer)

	logger.Info().Int("port", env.port).
		Msg("✅ PEAS started successfully!")

	if err = grpcServer.Serve(listen); err != nil {
		panic(err)
	}
}
