package ratelimit

import (
	"fmt"

	envoy_core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"

	store "github.com/buildwithgrove/path-external-auth-server/portal_app_store"
)

// The rate limit configuration in this file corresponds to rate limit configuration
// in the GUARD Helm Chart's `values.yaml` file as deployed in production.
//
// Documentation reference:
// https://gateway.envoyproxy.io/docs/tasks/traffic/global-rate-limit/#rate-limit-distinct-users-except-admin

// Constants for rate limiting plans
//   - The key is the plan type as specified in the database.
//   - The value is the header key to be matched in the GUARD configuration.
const (
	DBPlanFree     = "PLAN_FREE"    // The plan type as specified in the database
	PlanFreeHeader = "Rl-Plan-Free" // The header key to be matched in the GUARD configuration
)

// Map used to convert the plan type as specified in the database to the
// header key to be matched in the GUARD configuration.
var rateLimitedPlanTypeHeaders = map[string]string{
	DBPlanFree: PlanFreeHeader, // PLAN_FREE: Rl-Plan-Free
}

// Prefix for user-specific rate limit headers
// where the integer value is the monthly user limit in millions
//
// Example:
//   - monthlyUserLimit of 40,000,000 returns "Rl-User-Limit-40"
//   - monthlyUserLimit of 10,000,000 returns "Rl-User-Limit-10"
const userLimitHeaderPrefix = "Rl-User-Limit-%d"

// getUserLimitHeader generates a rate limit header based on the monthly user limit.
// `monthlyUserLimit` must be set in intervals of 1,000,000.
//
// Example:
//   - monthlyUserLimit of 40,000,000 returns "Rl-User-Limit-40"
//   - monthlyUserLimit of 10,000,000 returns "Rl-User-Limit-10"
func getUserLimitHeader(monthlyUserLimit int32) string {
	// Convert to millions and format as header
	millionValue := monthlyUserLimit / 1_000_000
	return fmt.Sprintf(userLimitHeaderPrefix, millionValue)
}

// GetRateLimitHeader returns the rate limit header for the given portal app ID
// and rate limit configuration.
//
// Examples:
//   - PLAN_FREE: "Rl-Plan-Free: <portal-app-id>"
//   - 40 million monthly user limit: "Rl-User-Limit-40: <portal-app-id>"
//   - 10 million monthly user limit: "Rl-User-Limit-10: <portal-app-id>"
func GetRateLimitHeader(
	portalAppID string,
	rateLimitData *store.RateLimit,
) *envoy_core.HeaderValueOption {
	if rateLimitData == nil {
		return nil
	}

	planType := string(rateLimitData.PlanType)

	// First check if the request is for a rate limited plan, eg. PLAN_FREE
	// If so, the header will be set to the planFreeHeader
	// e.g. "Rl-Plan-Free: <portal-app-id>"
	if planType != "" {
		if rateLimitHeader, ok := rateLimitedPlanTypeHeaders[planType]; ok {
			return &envoy_core.HeaderValueOption{
				Header: &envoy_core.HeaderValue{
					Key:   rateLimitHeader,
					Value: portalAppID,
				},
			}
		}
	}

	// Then check if the request is for an portal app with a user-specific monthly user limit
	// If so, the header will be set to the user's monthly user limit in millions
	// e.g. "Rl-User-Limit-40: <portal-app-id>" = 40 million monthly user limit
	if rateLimitData.MonthlyUserLimit > 0 {
		header := getUserLimitHeader(rateLimitData.MonthlyUserLimit)
		return &envoy_core.HeaderValueOption{
			Header: &envoy_core.HeaderValue{
				Key:   header,
				Value: portalAppID,
			},
		}
	}

	return nil
}
