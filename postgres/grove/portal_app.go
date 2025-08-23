package grove

import (
	"github.com/buildwithgrove/path-external-auth-server/postgres/grove/sqlc"
	"github.com/buildwithgrove/path-external-auth-server/store"
)

const (
	PlanFree_DatabaseType  store.PlanType = "PLAN_FREE"
	PlanUnlimited_Database store.PlanType = "PLAN_UNLIMITED"
)

// portalApplicationRow is a struct that represents a row from the portal_applications table
// in the existing Grove Portal Database. It is necessary to convert the existing `portal_applications`
// table schema to the new `PortalApp` struct expected by the PATH Go External Authorization Server.
type portalApplicationRow struct {
	ID                string         `json:"id"`                  // The PortalApp ID maps to PortalApp.PortalAppII
	AccountID         string         `json:"account_id"`          // The PortalApp AccountID maps to the PortalApp.Metadata.AccountId
	SecretKey         string         `json:"secret_key"`          // The PortalApp SecretKey maps to the PortalApp.Auth.AuthType.StaticApiKey.ApiKey
	SecretKeyRequired bool           `json:"secret_key_required"` // The PortalApp SecretKeyRequired determines whether the auth type is StaticApiKey or NoAuth
	MonthlyUserLimit  int32          `json:"monthly_relay_limit"` // The PortalApp MonthlyUserLimit maps to the PortalApp.Metadata.MonthlyUserLimit
	Plan              store.PlanType `json:"plan"`                // The PortalApp Plan maps to the PortalApp.Metadata.PlanType
}

// sqlcPortalAppsToPortalAppRow (not the plurality of Apps) converts a row from the
// `SelectPortalAppsRow` query to the intermediate portalApplicationRow struct.
// This is necessary because SQLC generates a specific struct for each query, which needs
// to be converted to a common struct before converting to the store.PortalApp struct.
func sqlcPortalAppsToPortalAppRow(r sqlc.SelectPortalAppsRow) *portalApplicationRow {
	return &portalApplicationRow{
		ID:                r.ID,
		AccountID:         r.AccountID.String,
		SecretKey:         r.SecretKey.String,
		SecretKeyRequired: r.SecretKeyRequired.Bool,
		Plan:              store.PlanType(r.Plan.String),
		MonthlyUserLimit:  r.MonthlyUserLimit.Int32,
	}
}

func (r *portalApplicationRow) convertToPortalApp() *store.PortalApp {
	return &store.PortalApp{
		ID:        store.PortalAppID(r.ID),
		AccountID: store.AccountID(r.AccountID),
		Auth:      r.getAuthDetails(),
		RateLimit: r.getRateLimitDetails(),
	}
}

func (r *portalApplicationRow) getAuthDetails() *store.Auth {
	if r.SecretKeyRequired {
		return &store.Auth{
			APIKey: r.SecretKey,
		}
	}

	return nil
}

func (r *portalApplicationRow) getRateLimitDetails() *store.RateLimit {
	// The following scenarios are rate limited:
	// 		- PLAN_FREE
	// 		- PLAN_UNLIMITED with a user-specified monthly user limit
	if r.Plan == PlanFree_DatabaseType || r.MonthlyUserLimit > 0 {
		return &store.RateLimit{
			PlanType:         store.PlanType(r.Plan),
			MonthlyUserLimit: r.MonthlyUserLimit,
		}
	}

	return nil
}

func sqlcPortalAppsToPortalApps(rows []sqlc.SelectPortalAppsRow) map[store.PortalAppID]*store.PortalApp {
	portalApps := make(map[store.PortalAppID]*store.PortalApp, len(rows))
	for _, row := range rows {
		portalAppRow := sqlcPortalAppsToPortalAppRow(row)
		portalApps[store.PortalAppID(portalAppRow.ID)] = portalAppRow.convertToPortalApp()
	}

	return portalApps
}
