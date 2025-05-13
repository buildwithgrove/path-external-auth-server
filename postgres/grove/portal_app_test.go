package grove

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	store "github.com/buildwithgrove/path-external-auth-server/portal_app_store"
	"github.com/buildwithgrove/path-external-auth-server/postgres/grove/sqlc"
)

func Test_sqlcPortalAppsToProto(t *testing.T) {
	tests := []struct {
		name     string
		rows     []sqlc.SelectPortalAppsRow
		expected map[store.PortalAppID]*store.PortalApp
		wantErr  bool
	}{
		{
			name: "should convert rows to auth data response successfully",
			rows: []sqlc.SelectPortalAppsRow{
				{
					ID:                "portal_app_1_static_key",
					AccountID:         pgtype.Text{String: "account_1", Valid: true},
					Plan:              pgtype.Text{String: "PLAN_UNLIMITED", Valid: true},
					SecretKeyRequired: pgtype.Bool{Bool: true, Valid: true},
					SecretKey:         pgtype.Text{String: "secret_key_1", Valid: true},
				},
				{
					ID:                "portal_app_2_no_auth",
					AccountID:         pgtype.Text{String: "account_2", Valid: true},
					Plan:              pgtype.Text{String: "PLAN_FREE", Valid: true},
					SecretKeyRequired: pgtype.Bool{Bool: false, Valid: true},
					SecretKey:         pgtype.Text{String: "secret_key_2", Valid: true},
				},
			},
			expected: map[store.PortalAppID]*store.PortalApp{
				"portal_app_1_static_key": {
					PortalAppID: "portal_app_1_static_key",
					AccountID:   "account_1",
					Auth: &store.Auth{
						APIKey: "secret_key_1",
					},
				},
				"portal_app_2_no_auth": {
					PortalAppID: "portal_app_2_no_auth",
					AccountID:   "account_2",
					Auth:        nil, // No auth required
					RateLimit: &store.RateLimit{
						PlanType: "PLAN_FREE",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := sqlcPortalAppsToPortalApps(test.rows)
			require.Equal(t, test.expected, result)
		})
	}
}
