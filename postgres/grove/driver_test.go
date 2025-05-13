package grove

import (
	"flag"
	"os"
	"testing"

	"github.com/pokt-network/poktroll/pkg/polylog/polyzero"
	"github.com/stretchr/testify/require"

	store "github.com/buildwithgrove/path-external-auth-server/portal_app_store"
)

var connectionString string

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		return
	}

	// Initialize the ephemeral postgres docker container
	pool, resource, databaseURL := setupPostgresDocker()
	connectionString = databaseURL

	// Run DB integration test
	exitCode := m.Run()

	// Cleanup the ephemeral postgres docker container
	cleanupPostgresDocker(m, pool, resource)
	os.Exit(exitCode)
}

func Test_Integration_FetchAuthDataSync(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping driver integration test")
	}

	tests := []struct {
		name     string
		expected map[store.PortalAppID]*store.PortalApp
	}{
		{
			name: "should retrieve all portal appsdata correctly",
			expected: map[store.PortalAppID]*store.PortalApp{
				"portal_app_1_no_auth": {
					PortalAppID: "portal_app_1_no_auth",
					AccountID:   "account_1",
					Auth:        nil, // No auth required
					RateLimit: &store.RateLimit{
						PlanType: "PLAN_FREE",
					},
				},
				"portal_app_2_static_key": {
					PortalAppID: "portal_app_2_static_key",
					AccountID:   "account_2",
					Auth: &store.Auth{
						APIKey: "secret_key_2",
					},
				},
				"portal_app_3_static_key": {
					PortalAppID: "portal_app_3_static_key",
					AccountID:   "account_3",
					Auth: &store.Auth{
						APIKey: "secret_key_3",
					},
					RateLimit: &store.RateLimit{
						PlanType: "PLAN_FREE",
					},
				},
				"portal_app_4_no_auth": {
					PortalAppID: "portal_app_4_no_auth",
					AccountID:   "account_1",
					Auth:        nil, // No auth required
					RateLimit: &store.RateLimit{
						PlanType: "PLAN_FREE",
					},
				},
				"portal_app_5_static_key": {
					PortalAppID: "portal_app_5_static_key",
					AccountID:   "account_2",
					Auth: &store.Auth{
						APIKey: "secret_key_5",
					},
				},
				"portal_app_6_user_limit": {
					PortalAppID: "portal_app_6_user_limit",
					AccountID:   "account_4",
					Auth:        nil, // No auth required
					RateLimit: &store.RateLimit{
						PlanType:         "PLAN_UNLIMITED",
						MonthlyUserLimit: 10_000_000,
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := require.New(t)

			dataSource, err := NewGrovePostgresDriver(polyzero.NewLogger(), connectionString)
			c.NoError(err)

			authData, err := dataSource.FetchInitialData()
			c.NoError(err)
			c.Equal(test.expected, authData)
		})
	}
}
