package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
)

const (
	grpcHostPortEnv               = "GRPC_HOST_PORT"
	grpcUseInsecureCredentialsEnv = "GRPC_USE_INSECURE_CREDENTIALS"
	portEnv                       = "PORT"
	defaultPort                   = 10001
)

var grpcHostPortPattern = "^[^:]+:[0-9]+$"

// envVars holds configuration values, all fields are private
// Use gatherEnvVars to load values from environment variables
// and perform validation and default hydration.
type envVars struct {
	grpcHostPort               string
	grpcUseInsecureCredentials bool
	port                       int
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
}
