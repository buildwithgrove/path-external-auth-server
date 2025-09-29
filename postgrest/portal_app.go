package postgrest

import (
	"github.com/buildwithgrove/path-external-auth-server/store"
	portal_db_sdk "github.com/grove/path/portal-db/sdk/go"
)

const (
	// Plan types from the database - matching Grove's implementation
	PlanFree_DatabaseType      store.PlanType = "PLAN_FREE"
	PlanUnlimited_DatabaseType store.PlanType = "PLAN_UNLIMITED"
)

// convertSDKToPortalApp converts SDK types directly to store.PortalApp
// combining portal_db_sdk.PortalApplications and portal_db_sdk.PortalAccounts data.
func convertSDKToPortalApp(
	app portal_db_sdk.PortalApplications,
	account portal_db_sdk.PortalAccounts,
) *store.PortalApp {
	planType := store.PlanType(account.PortalPlanType)
	return &store.PortalApp{
		ID:        store.PortalAppID(app.PortalApplicationId),
		AccountID: store.AccountID(app.PortalAccountId),
		PlanType:  planType,
		Auth:      getAuthDetailsFromSDK(app),
		RateLimit: getRateLimitDetailsFromSDK(account),
	}
}

// getAuthDetailsFromSDK determines the authentication configuration from SDK data
func getAuthDetailsFromSDK(app portal_db_sdk.PortalApplications) *store.Auth {
	if app.SecretKeyRequired != nil && *app.SecretKeyRequired && app.SecretKeyHash != nil {
		return &store.Auth{
			APIKey: *app.SecretKeyHash,
		}
	}

	return nil
}

// getRateLimitDetailsFromSDK determines the rate limiting configuration from SDK data
func getRateLimitDetailsFromSDK(account portal_db_sdk.PortalAccounts) *store.RateLimit {
	// The following scenarios are rate limited:
	// 		- PLAN_FREE
	// 		- PLAN_UNLIMITED with a user-specified monthly user limits
	planType := store.PlanType(account.PortalPlanType)

	// Free plans are always rate limited so we set the limit a non-nil RateLimit field.
	if planType == PlanFree_DatabaseType {
		return &store.RateLimit{}
	}

	// Unlimited plans with optional user-specified limits are rate limited.
	if planType == PlanUnlimited_DatabaseType && account.PortalAccountUserLimit != nil && *account.PortalAccountUserLimit > 0 {
		return &store.RateLimit{
			MonthlyUserLimit: int32(*account.PortalAccountUserLimit),
		}
	}

	// All other scenarios are not rate limited
	return nil
}
