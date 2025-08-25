package grove

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pokt-network/poktroll/pkg/polylog"

	"github.com/buildwithgrove/path-external-auth-server/postgres/grove/sqlc"
	"github.com/buildwithgrove/path-external-auth-server/store"
)

// GrovePostgresDriver implements the store.DataSource interface
// to provide data from Grove's Postgres database for the portal app store.
var _ store.DataSource = &GrovePostgresDriver{}

type (
	// GrovePostgresDriver implements the store.DataSource interface
	// to provide data from a Postgres database for the portal app store.
	GrovePostgresDriver struct {
		logger polylog.Logger
		driver *postgresDriver
	}

	// The postgresDriver struct wraps the SQLC generated queries and the pgxpool.Pool.
	// See: https://docs.sqlc.dev/en/latest/tutorials/getting-started-postgresql.html
	postgresDriver struct {
		*sqlc.Queries
		DB *pgxpool.Pool
	}
)

/* ---------- Postgres Connection Funcs ---------- */

// Regular expression to match a valid PostgreSQL connection string
var postgresConnectionStringRegex = regexp.MustCompile(`^postgres(?:ql)?:\/\/[^:]+:[^@]+@[^:]+:\d+\/[^?]+(?:\?.+)?$`)

/*
NewGrovePostgresDriver returns a PostgreSQL data source that implements the store.DataSource interface.

The data source connects to a PostgreSQL database and:
1. Provides methods to fetch initial portal app data
2. Provides a channel for receiving portal app updates
3. Listens for changes from the database
*/
func NewGrovePostgresDriver(
	logger polylog.Logger,
	connectionString string,
) (*GrovePostgresDriver, error) {
	if !isValidPostgresConnectionString(connectionString) {
		return nil, fmt.Errorf("invalid postgres connection string")
	}

	config, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.NewWithConfig: %v", err)
	}

	// Verify connection immediately
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	driver := &postgresDriver{
		Queries: sqlc.New(pool),
		DB:      pool,
	}

	dataSource := &GrovePostgresDriver{
		logger: logger,
		driver: driver,
	}

	return dataSource, nil
}

// isValidPostgresConnectionString checks if a string is a valid PostgreSQL connection string.
func isValidPostgresConnectionString(s string) bool {
	return postgresConnectionStringRegex.MatchString(s)
}

/* ---------- DataSource Interface Implementation ---------- */

// GetPortalApps loads the full set of PortalApps from the Postgres database.
func (d *GrovePostgresDriver) GetPortalApps() (map[store.PortalAppID]*store.PortalApp, error) {
	d.logger.Info().Msg("💾 Executing SelectPortalApps query...")
	rows, err := d.driver.Queries.SelectPortalApps(context.Background())
	if err != nil {
		d.logger.Error().Err(err).Msg("failed to fetch portal applications from database")
		return nil, fmt.Errorf("failed to fetch portal applications: %w", err)
	}

	d.logger.Info().Int("num_rows", len(rows)).Msg("✅ Successfully fetched Portal Applications from Postgres")

	return sqlcPortalAppsToPortalApps(rows), nil
}

// Close cleans up resources used by the data source.
func (d *GrovePostgresDriver) Close() {
	// The listener doesn't have a Close method, but when
	// we close the DB pool it will close the connections
	d.driver.DB.Close()
}
