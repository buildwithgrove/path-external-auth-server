package grove

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	"github.com/buildwithgrove/path-external-auth-server/postgres/grove/sqlc"
	"github.com/buildwithgrove/path-external-auth-server/store"
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
					ID:        "portal_app_1_static_key",
					AccountID: pgtype.Text{String: "account_1", Valid: true},
					Plan: pgtype.Text{
						String: string(PlanUnlimited_DatabaseType),
						Valid:  true,
					},
					SecretKeyRequired: pgtype.Bool{Bool: true, Valid: true},
					SecretKey:         pgtype.Text{String: "secret_key_1", Valid: true},
				},
				{
					ID:        "portal_app_2_no_auth",
					AccountID: pgtype.Text{String: "account_2", Valid: true},
					Plan: pgtype.Text{
						String: string(PlanFree_DatabaseType),
						Valid:  true,
					},
					SecretKeyRequired: pgtype.Bool{Bool: false, Valid: true},
					SecretKey:         pgtype.Text{String: "secret_key_2", Valid: true},
				},
			},
			expected: map[store.PortalAppID]*store.PortalApp{
				"portal_app_1_static_key": {
					ID:        "portal_app_1_static_key",
					AccountID: "account_1",
					PlanType:  PlanUnlimited_DatabaseType,
					Auth: &store.Auth{
						APIKey: "secret_key_1",
					},
				},
				"portal_app_2_no_auth": {
					ID:        "portal_app_2_no_auth",
					AccountID: "account_2",
					PlanType:  PlanFree_DatabaseType,
					Auth:      nil, // No auth required
					RateLimit: &store.RateLimit{},
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
