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
	// [OPTIONAL]: Data source type - "postgres" or "postgrest"
	//   - Default: "postgres" if not set
	dataSourceTypeEnv     = "DATA_SOURCE_TYPE"
	defaultDataSourceType = "postgres"

	// [REQUIRED]: PostgreSQL connection string for the PortalApp database used by the auth server.
	//   - Example: "postgresql://username:password@localhost:5432/dbname"
	postgresConnectionStringEnv = "POSTGRES_CONNECTION_STRING"

	// [REQUIRED when DATA_SOURCE_TYPE=postgrest]: PostgREST base URL
	//   - Example: "http://localhost:3000"
	postgrestBaseURLEnv = "POSTGREST_BASE_URL"

	// [REQUIRED when DATA_SOURCE_TYPE=postgrest]: JWT secret for PostgREST authentication
	//   - Example: "supersecretjwtsecretforlocaldevelopment123456789"
	postgrestJWTSecretEnv = "POSTGREST_JWT_SECRET"

	// [OPTIONAL]: JWT role for PostgREST authentication
	//   - Examples: "authenticated", "anon"
	postgrestJWTRoleEnv = "POSTGREST_JWT_ROLE"

	// [REQUIRED when DATA_SOURCE_TYPE=postgrest]: JWT email for PostgREST authentication
	//   - Example: "service@grove.city"
	postgrestJWTEmailEnv = "POSTGREST_JWT_EMAIL"

	// [REQUIRED]: GCP project ID for the data warehouse used by the rate limit store.
	//   - Example: "your-project-id"
	gcpProjectIDEnv = "GCP_PROJECT_ID"

	// [OPTIONAL]: PostgREST request timeout
	//   - Default: 30s if not set
	//   - Examples: "30s", "1m", "2m30s"
	postgrestTimeoutEnv     = "POSTGREST_TIMEOUT"
	defaultPostgrestTimeout = 30 * time.Second

	// [OPTIONAL]: Port to run the external auth server on.
	//   - Default: 10001 if not set
	portEnv     = "PORT"
	defaultPort = 10001

	// [OPTIONAL]: Port to run the Prometheus metrics server on.
	//   - Default: 9090 if not set
	metricsPortEnv     = "METRICS_PORT"
	defaultMetricsPort = 9090

	// [OPTIONAL]: Port to run the pprof server on.
	//   - Default: 6060 if not set
	pprofPortEnv     = "PPROF_PORT"
	defaultPprofPort = 6060

	// [OPTIONAL]: Log level for the external auth server.
	//   - Default: "info" if not set
	loggerLevelEnv     = "LOGGER_LEVEL"
	defaultLoggerLevel = "info"

	// [OPTIONAL]: Image tag/version for the application.
	//   - Default: "development" if not set
	imageTagEnv     = "IMAGE_TAG"
	defaultImageTag = "development"

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

// DataSourceType represents the type of data source to use
type DataSourceType string

const (
	DataSourceTypePostgres  DataSourceType = "postgres"
	DataSourceTypePostgREST DataSourceType = "postgrest"
)

// envVars holds configuration values.
//   - All fields are private.
//   - Use gatherEnvVars to load, validate, and hydrate defaults from environment variables.
type envVars struct {
	// Database configuration
	dataSourceType DataSourceType

	postgresConnectionString string

	postgrestBaseURL   string
	postgrestJWTSecret string
	postgrestJWTRole   string
	postgrestJWTEmail  string
	postgrestTimeout   time.Duration

	// Data warehouse configuration
	gcpProjectID string
	// Server port configuration
	port        int
	metricsPort int
	pprofPort   int

	// Application configuration
	loggerLevel string
	imageTag    string

	// Store refresh intervals
	portalAppStoreRefreshInterval time.Duration
	rateLimitStoreRefreshInterval time.Duration
}

// gatherEnvVars:
//   - Loads configuration from environment variables
//   - Validates and hydrates defaults for missing/invalid values
func gatherEnvVars() (envVars, error) {
	// Initialize with environment variables
	e := envVars{
		dataSourceType: DataSourceType(os.Getenv(dataSourceTypeEnv)),

		postgresConnectionString: os.Getenv(postgresConnectionStringEnv),

		postgrestBaseURL:   os.Getenv(postgrestBaseURLEnv),
		postgrestJWTSecret: os.Getenv(postgrestJWTSecretEnv),
		postgrestJWTRole:   os.Getenv(postgrestJWTRoleEnv),
		postgrestJWTEmail:  os.Getenv(postgrestJWTEmailEnv),

		gcpProjectID: os.Getenv(gcpProjectIDEnv),
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

	// Parse metrics port environment variable (if provided)
	metricsPortStr := os.Getenv(metricsPortEnv)
	if metricsPortStr != "" {
		p, err := strconv.Atoi(metricsPortStr)
		if err != nil {
			return envVars{}, fmt.Errorf("invalid metrics port format: %v", err)
		}
		e.metricsPort = p
	}

	// Parse pprof port environment variable (if provided)
	pprofPortStr := os.Getenv(pprofPortEnv)
	if pprofPortStr != "" {
		p, err := strconv.Atoi(pprofPortStr)
		if err != nil {
			return envVars{}, fmt.Errorf("invalid pprof port format: %v", err)
		}
		e.pprofPort = p
	}

	// Parse log level from environment (if provided)
	loggerLevel := os.Getenv(loggerLevelEnv)
	if loggerLevel != "" {
		e.loggerLevel = loggerLevel
	}

	// Parse image tag from environment (if provided)
	imageTag := os.Getenv(imageTagEnv)
	if imageTag != "" {
		e.imageTag = imageTag
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

	// Parse PostgREST timeout from environment (if provided)
	postgrestTimeoutStr := os.Getenv(postgrestTimeoutEnv)
	if postgrestTimeoutStr != "" {
		duration, err := time.ParseDuration(postgrestTimeoutStr)
		if err != nil {
			return envVars{}, fmt.Errorf("invalid PostgREST timeout format: %v", err)
		}
		e.postgrestTimeout = duration
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
	// GCP project ID must be set
	if e.gcpProjectID == "" {
		return fmt.Errorf("%s is not set", gcpProjectIDEnv)
	}

	// Validate based on data source type
	switch e.dataSourceType {
	case DataSourceTypePostgres:
		if e.postgresConnectionString == "" {
			return fmt.Errorf("%s is required when DATA_SOURCE_TYPE=postgres", postgresConnectionStringEnv)
		}
		// Connection string must match expected format
		matched, err := regexp.MatchString(postgresConnectionStringRegex.String(), e.postgresConnectionString)
		if err != nil {
			return fmt.Errorf("failed to validate postgresConnectionString: %v", err)
		}
		if !matched {
			return fmt.Errorf("postgresConnectionString does not match the required pattern")
		}
	case DataSourceTypePostgREST:
		if e.postgrestBaseURL == "" {
			return fmt.Errorf("%s is required when DATA_SOURCE_TYPE=postgrest", postgrestBaseURLEnv)
		}
		if e.postgrestJWTSecret == "" {
			return fmt.Errorf("%s is required when DATA_SOURCE_TYPE=postgrest", postgrestJWTSecretEnv)
		}
		if e.postgrestJWTEmail == "" {
			return fmt.Errorf("%s is required when DATA_SOURCE_TYPE=postgrest", postgrestJWTEmailEnv)
		}
	default:
		return fmt.Errorf("unsupported data source type: %s", e.dataSourceType)
	}

	return nil
}

// hydrateDefaults sets defaults for missing/invalid values
func (e *envVars) hydrateDefaults() {
	if e.dataSourceType == "" {
		e.dataSourceType = DataSourceType(defaultDataSourceType)
	}
	if e.postgrestTimeout == 0 {
		e.postgrestTimeout = defaultPostgrestTimeout
	}
	if e.port == 0 {
		e.port = defaultPort
	}
	if e.metricsPort == 0 {
		e.metricsPort = defaultMetricsPort
	}
	if e.pprofPort == 0 {
		e.pprofPort = defaultPprofPort
	}
	if e.loggerLevel == "" {
		e.loggerLevel = defaultLoggerLevel
	}
	if e.imageTag == "" {
		e.imageTag = defaultImageTag
	}
	if e.portalAppStoreRefreshInterval == 0 {
		e.portalAppStoreRefreshInterval = defaultPortalAppStoreRefreshInterval
	}
	if e.rateLimitStoreRefreshInterval == 0 {
		e.rateLimitStoreRefreshInterval = defaultRateLimitStoreRefreshInterval
	}
}
