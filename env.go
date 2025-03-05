package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"

	"github.com/buildwithgrove/path-external-auth-server/auth"
)

const (
	grpcHostPortEnv               = "GRPC_HOST_PORT"
	grpcUseInsecureCredentialsEnv = "GRPC_USE_INSECURE_CREDENTIALS"
	epIDExtractorTypeEnv          = "ENDPOINT_ID_EXTRACTOR_TYPE"
	portEnv                       = "PORT"

	defaultPort = 10001
)

var defaultEndpointIDExtractorType = auth.EndpointIDExtractorTypeURLPath
var grpcHostPortPattern = "^[^:]+:[0-9]+$"

// envVars holds configuration values, all fields are private
// Use gatherEnvVars to load values from environment variables
// and perform validation and default hydration.
type envVars struct {
	grpcHostPort               string
	grpcUseInsecureCredentials bool
	epIDExtractorType          auth.EndpointIDExtractorType
	port                       int
}

// gatherEnvVars loads configuration from environment variables
// and validates/hydrates defaults for missing/invalid values.
func gatherEnvVars() (envVars, error) {
	e := envVars{
		grpcHostPort:      os.Getenv(grpcHostPortEnv),
		epIDExtractorType: auth.EndpointIDExtractorType(os.Getenv(epIDExtractorTypeEnv)),
	}

	portStr := os.Getenv(portEnv)
	if portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil {
			return envVars{}, fmt.Errorf("invalid port format: %v", err)
		}
		e.port = p
	}

	insecureStr := os.Getenv(grpcUseInsecureCredentialsEnv)
	if insecureStr != "" {
		insecure, err := strconv.ParseBool(insecureStr)
		if err != nil {
			return envVars{}, fmt.Errorf("invalid value for %s: %v", grpcUseInsecureCredentialsEnv, err)
		}
		e.grpcUseInsecureCredentials = insecure
	}

	if err := e.validateAndHydrate(); err != nil {
		return envVars{}, err
	}

	return e, nil
}

func (e *envVars) validateAndHydrate() error {
	if e.grpcHostPort == "" {
		return fmt.Errorf("%s is not set", grpcHostPortEnv)
	}
	matched, err := regexp.MatchString(grpcHostPortPattern, e.grpcHostPort)
	if err != nil {
		return fmt.Errorf("failed to validate grpcHostPort: %v", err)
	}
	if !matched {
		return fmt.Errorf("grpcHostPort does not match the required pattern")
	}

	if !e.epIDExtractorType.IsValid() {
		fmt.Printf("invalid endpoint ID extractor type: %s, using default: %s\n", e.epIDExtractorType, defaultEndpointIDExtractorType)
		e.epIDExtractorType = defaultEndpointIDExtractorType
	}
	if e.port == 0 {
		e.port = defaultPort
	}
	return nil
}
