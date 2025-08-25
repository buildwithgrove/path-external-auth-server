package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	// autoload env vars

	_ "github.com/joho/godotenv/autoload"
)

const (
	// [REQUIRED]: PostgreSQL connection string for the PortalApp database used by the auth server.
	//   - Example: "postgresql://username:password@localhost:5432/dbname"
	postgresConnectionStringEnv = "POSTGRES_CONNECTION_STRING"

	// [REQUIRED]: GCP project ID for the data warehouse used by the rate limit store.
	//   - Example: "your-project-id"
	gcpProjectIDEnv = "GCP_PROJECT_ID"

	// [OPTIONAL]: Port to run the external auth server on.
	//   - Default: 10001 if not set
	portEnv     = "PORT"
	defaultPort = 10001

	// [OPTIONAL]: Log level for the external auth server.
	//   - Default: "info" if not set
	loggerLevelEnv     = "LOGGER_LEVEL"
	defaultLoggerLevel = "info"

	// [OPTIONAL]: Refresh interval for the portal app store.
	//   - Default: 30s if not set
	//   - Examples: "30s", "1m", "2m30s"
	portalAppStoreRefreshIntervalEnv     = "PORTAL_APP_STORE_REFRESH_INTERVAL"
	defaultPortalAppStoreRefreshInterval = 30 * time.Second

	// [OPTIONAL]: Refresh interval for the rate limit store.
	//   - Default: 5m if not set
	//   - Examples: "30s", "1m", "2m30s"
	rateLimitStoreRefreshIntervalEnv     = "RATE_LIMIT_STORE_REFRESH_INTERVAL"
	defaultRateLimitStoreRefreshInterval = 5 * time.Minute
)

var postgresConnectionStringRegex = regexp.MustCompile(`^postgres(?:ql)?:\/\/[^:]+:[^@]+@[^:]+:\d+\/[^?]+(?:\?.+)?$`)

// envVars holds configuration values.
//   - All fields are private.
//   - Use gatherEnvVars to load, validate, and hydrate defaults from environment variables.
type envVars struct {
	postgresConnectionString      string
	gcpProjectID                  string
	port                          int
	loggerLevel                   string
	portalAppStoreRefreshInterval time.Duration
	rateLimitStoreRefreshInterval time.Duration
}

// gatherEnvVars:
//   - Loads configuration from environment variables
//   - Validates and hydrates defaults for missing/invalid values
func gatherEnvVars() (envVars, error) {
	// Initialize with Postgres connection string from environment
	e := envVars{
		postgresConnectionString: os.Getenv(postgresConnectionStringEnv),
		gcpProjectID:             os.Getenv(gcpProjectIDEnv),
	}

	// Parse port environment variable (if provided)
	portStr := os.Getenv(portEnv)
	if portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil {
			return envVars{}, fmt.Errorf("invalid port format: %v", err)
		}
		e.port = p
	}

	// Parse log level from environment (if provided)
	loggerLevel := os.Getenv(loggerLevelEnv)
	if loggerLevel != "" {
		e.loggerLevel = loggerLevel
	}

	// Parse portal app store refresh interval from environment (if provided)
	portalAppStoreRefreshIntervalStr := os.Getenv(portalAppStoreRefreshIntervalEnv)
	if portalAppStoreRefreshIntervalStr != "" {
		duration, err := time.ParseDuration(portalAppStoreRefreshIntervalStr)
		if err != nil {
			return envVars{}, fmt.Errorf("invalid refresh interval format: %v", err)
		}
		e.portalAppStoreRefreshInterval = duration
	}

	// Parse rate limit store refresh interval from environment (if provided)
	rateLimitStoreRefreshIntervalStr := os.Getenv(rateLimitStoreRefreshIntervalEnv)
	if rateLimitStoreRefreshIntervalStr != "" {
		duration, err := time.ParseDuration(rateLimitStoreRefreshIntervalStr)
		if err != nil {
			return envVars{}, fmt.Errorf("invalid refresh interval format: %v", err)
		}
		e.rateLimitStoreRefreshInterval = duration
	}

	// Apply defaults for any unset configuration
	e.hydrateDefaults()

	// Validate all required configuration
	if err := e.validate(); err != nil {
		return envVars{}, err
	}
	return e, nil
}

// validate checks that all required environment variables are set and valid
func (e *envVars) validate() error {
	// Postgres connection string must be set
	if e.postgresConnectionString == "" {
		return fmt.Errorf("%s is not set", postgresConnectionStringEnv)
	}

	// GCP project ID must be set
	if e.gcpProjectID == "" {
		return fmt.Errorf("%s is not set", gcpProjectIDEnv)
	}

	// Connection string must match expected format
	matched, err := regexp.MatchString(postgresConnectionStringRegex.String(), e.postgresConnectionString)
	if err != nil {
		return fmt.Errorf("failed to validate postgresConnectionString: %v", err)
	}
	if !matched {
		return fmt.Errorf("postgresConnectionString does not match the required pattern")
	}

	return nil
}

// hydrateDefaults sets defaults for missing/invalid values
func (e *envVars) hydrateDefaults() {
	if e.port == 0 {
		e.port = defaultPort
	}
	if e.loggerLevel == "" {
		e.loggerLevel = defaultLoggerLevel
	}
	if e.portalAppStoreRefreshInterval == 0 {
		e.portalAppStoreRefreshInterval = defaultPortalAppStoreRefreshInterval
	}
	if e.rateLimitStoreRefreshInterval == 0 {
		e.rateLimitStoreRefreshInterval = defaultRateLimitStoreRefreshInterval
	}
}
