package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"

	// autoload env vars
	_ "github.com/joho/godotenv/autoload"
)

const (
	// [REQUIRED]: The PostgreSQL connection string for the database
	// that provides the PortalApp data for the auth server.
	//
	// Example: "postgresql://username:password@localhost:5432/dbname"
	postgresConnectionStringEnv = "POSTGRES_CONNECTION_STRING"

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

var postgresConnectionStringRegex = regexp.MustCompile(`^postgres(?:ql)?:\/\/[^:]+:[^@]+@[^:]+:\d+\/[^?]+(?:\?.+)?$`)

// envVars holds configuration values, all fields are private
// Use gatherEnvVars to load values from environment variables
// and perform validation and default hydration.
type envVars struct {
	postgresConnectionString string
	port                     int
	loggerLevel              string
}

// gatherEnvVars loads configuration from environment variables
// and validates/hydrates defaults for missing/invalid values.
func gatherEnvVars() (envVars, error) {
	// Initialize with postgres connection string from environment
	e := envVars{
		postgresConnectionString: os.Getenv(postgresConnectionStringEnv),
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
	// Verify the Postgres connection string is specified
	if e.postgresConnectionString == "" {
		return fmt.Errorf("%s is not set", postgresConnectionStringEnv)
	}

	// Ensure the Postgres connection string matches the expected format
	matched, err := regexp.MatchString(postgresConnectionStringRegex.String(), e.postgresConnectionString)
	if err != nil {
		return fmt.Errorf("failed to validate postgresConnectionString: %v", err)
	}
	if !matched {
		return fmt.Errorf("postgresConnectionString does not match the required pattern")
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
