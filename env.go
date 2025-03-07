package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
)

const (
	// [REQUIRED]: The host and port for the remote gRPC server connection
	// that provides the GatewayEndpoint data for the auth server.
	//
	// GRPC_HOST_PORT=guard-pads:10002 is the value to point to the default
	// PADS server in the cluster created by the GUARD Helm Chart.
	//
	// Example: "localhost:10002" or "auth-server.buildwithgrove.com:443"
	grpcHostPortEnv = "GRPC_HOST_PORT"

	// [OPTIONAL]: Whether to use insecure credentials for the gRPC connection.
	//
	// GRPC_USE_INSECURE_CREDENTIALS=true is required to run PEAS in the
	// cluster created by the GUARD Helm Chart, as PADS does not have TLS
	// enabled by default.
	//
	// Default is "false" if not set.
	grpcUseInsecureCredentialsEnv = "GRPC_USE_INSECURE_CREDENTIALS"

	// [OPTIONAL]: The port to run the external auth server on.
	//
	// Default is 10001 if not set.
	portEnv     = "PORT"
	defaultPort = 10001

	// [OPTIONAL]: The log level to use for the external auth server.
	//
	// Default is "info" if not set.
	loggerLevelEnv     = "LOGGER_LEVEL"
	defaultLoggerLevel = "info"
)

var grpcHostPortPattern = "^[^:]+:[0-9]+$"

// envVars holds configuration values, all fields are private
// Use gatherEnvVars to load values from environment variables
// and perform validation and default hydration.
type envVars struct {
	grpcHostPort               string
	grpcUseInsecureCredentials bool
	port                       int
	loggerLevel                string
}

// gatherEnvVars loads configuration from environment variables
// and validates/hydrates defaults for missing/invalid values.
func gatherEnvVars() (envVars, error) {
	// Initialize with GRPC host:port from environment
	e := envVars{
		grpcHostPort: os.Getenv(grpcHostPortEnv),
	}

	// Parse port environment variable if provided
	portStr := os.Getenv(portEnv)
	if portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil {
			return envVars{}, fmt.Errorf("invalid port format: %v", err)
		}
		e.port = p
	}

	// Parse insecure credentials flag from environment
	insecureStr := os.Getenv(grpcUseInsecureCredentialsEnv)
	if insecureStr != "" {
		insecure, err := strconv.ParseBool(insecureStr)
		if err != nil {
			return envVars{}, fmt.Errorf("invalid value for %s: %v", grpcUseInsecureCredentialsEnv, err)
		}
		e.grpcUseInsecureCredentials = insecure
	}

	// Parse log level from environment
	loggerLevel := os.Getenv(loggerLevelEnv)
	if loggerLevel != "" {
		e.loggerLevel = loggerLevel
	}

	// Apply default values for any unset configuration
	e.hydrateDefaults()

	// Ensure all required configuration is valid
	if err := e.validate(); err != nil {
		return envVars{}, err
	}
	return e, nil
}

// validate checks that all required environment variables are set with valid values
func (e *envVars) validate() error {
	// Verify the GRPC host:port is specified
	if e.grpcHostPort == "" {
		return fmt.Errorf("%s is not set", grpcHostPortEnv)
	}

	// Ensure the GRPC host:port matches the expected format
	matched, err := regexp.MatchString(grpcHostPortPattern, e.grpcHostPort)
	if err != nil {
		return fmt.Errorf("failed to validate grpcHostPort: %v", err)
	}
	if !matched {
		return fmt.Errorf("grpcHostPort does not match the required pattern")
	}

	return nil
}

// hydrateDefaults hydrates defaults for missing/invalid values.
func (e *envVars) hydrateDefaults() {
	if e.port == 0 {
		e.port = defaultPort
	}
	if e.loggerLevel == "" {
		e.loggerLevel = defaultLoggerLevel
	}
}
