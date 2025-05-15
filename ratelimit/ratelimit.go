package ratelimit

import (
	"fmt"

	envoy_core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"

	store "github.com/buildwithgrove/path-external-auth-server/portal_app_store"
)

// The rate limit configuration in this file corresponds to rate limit configuration
// in the GUARD Helm Chart's `values.yaml` file as deployed in production.
// Rate limits are applied by account ID.
//
// Documentation reference:
// https://gateway.envoyproxy.io/docs/tasks/traffic/global-rate-limit/#rate-limit-distinct-users-except-admin

// Constants for rate limiting plans
//   - The key is the plan type as specified in the database.
//   - The value is the header key to be matched in the GUARD configuration.
const DBPlanFree store.PlanType = "PLAN_FREE" // The plan type as specified in the database
const PlanFreeHeader = "Rl-Plan-Free"         // The header key to be matched in the GUARD configuration

// Map used to convert the plan type as specified in the database
// to the header key to be matched in the GUARD configuration.
var rateLimitedPlanTypeHeaders = map[store.PlanType]string{
	DBPlanFree: PlanFreeHeader, // PLAN_FREE: Rl-Plan-Free
}

// Prefix for user-specific rate limit headers where the
// integer value is the monthly user limit in millions.
//
// Examples:
//   - monthlyUserLimit of 40,000,000 returns "Rl-User-Limit-40"
//   - monthlyUserLimit of 10,000,000 returns "Rl-User-Limit-10"
const userLimitHeaderPrefix = "Rl-User-Limit-%d"

// getUserLimitHeader generates a rate limit header based on the monthly
// user limit. `monthlyUserLimit` must be set in intervals of 1,000,000.
//
// Examples:
//   - monthlyUserLimit of 40,000,000 returns "Rl-User-Limit-40"
//   - monthlyUserLimit of 10,000,000 returns "Rl-User-Limit-10"
func getUserLimitHeader(monthlyUserLimit int32) string {
	// Convert to millions and format as header
	millionValue := monthlyUserLimit / 1_000_000
	return fmt.Sprintf(userLimitHeaderPrefix, millionValue)
}

// GetRateLimitHeader returns the rate limit header for
// the given account and rate limit configuration.
//
// Examples:
//   - PLAN_FREE: "Rl-Plan-Free: <account-id>"
//   - 40 million monthly user limit: "Rl-User-Limit-40: <account-id>"
//   - 10 million monthly user limit: "Rl-User-Limit-10: <account-id>"
func GetRateLimitHeader(portalApp *store.PortalApp) *envoy_core.HeaderValueOption {
	// Return nil if the account is not rate limited.
	if portalApp.RateLimit == nil {
		return nil
	}

	rateLimit := portalApp.RateLimit

	// First check if the account is rate limited by plan type.
	// e.g. "Rl-Plan-Free: <account-id>"
	if rateLimitHeader, ok := rateLimitedPlanTypeHeaders[rateLimit.PlanType]; ok {
		return &envoy_core.HeaderValueOption{
			Header: &envoy_core.HeaderValue{
				Key:   rateLimitHeader,
				Value: string(portalApp.AccountID),
			},
		}
	}

	// Then check if the account is rate limited by user-specified monthly limit.
	// e.g. "Rl-User-Limit-40: <account-id>" = 40 million monthly user limit
	if rateLimit.MonthlyUserLimit > 0 {
		rateLimitHeader := getUserLimitHeader(rateLimit.MonthlyUserLimit)
		return &envoy_core.HeaderValueOption{
			Header: &envoy_core.HeaderValue{
				Key:   rateLimitHeader,
				Value: string(portalApp.AccountID),
			},
		}
	}

	return nil
}
