package ratelimit

import (
	"fmt"

	store "github.com/buildwithgrove/path-external-auth-server/portal_app_store"
	envoy_core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
)

// Rate limit config in this file matches GUARD Helm Chart `values.yaml` (production)
//
// Docs:
// - https://gateway.envoyproxy.io/docs/tasks/traffic/global-rate-limit/#rate-limit-distinct-users-except-admin

// Rate limiting plan constants:
//   - Key: plan type from DB
//   - Value: header key matched in GUARD config
//
// KV Pair for the free plan
const PlanFree_DatabaseType store.PlanType = "PLAN_FREE" // The plan type as specified in the database
const PlanFree_RequestHeader = "Rl-Plan-Free"            // The header key to be matched in the GUARD configuration

// Map: DB plan type -> GUARD header key
// Example:
//   - "PLAN_FREE" -> "Rl-Plan-Free"
var rateLimitedPlanTypeHeaders = map[store.PlanType]string{
	PlanFree_DatabaseType: PlanFree_RequestHeader,
}

// Prefix for user-specified rate limit headers
//   - Integer = monthly relay limit (in millions)
//
// Examples:
//   - 40,000,000 → "Rl-User-Limit-40"
//   - 10,000,000 → "Rl-User-Limit-10"
const userLimitHeaderPrefix = "Rl-User-Limit-%d"

// getUserLimitRequestHeader:
// - Generates rate limit header from monthly relay limit
// - `monthlyRelayLimit` must be a multiple of 1,000,000
// Examples:
//   - 40,000,000 → "Rl-User-Limit-40"
//   - 10,000,000 → "Rl-User-Limit-10"
func getUserLimitRequestHeader(monthlyRelayLimit int32) string {
	// Convert to millions and format as header
	millionValue := monthlyRelayLimit / 1_000_000
	return fmt.Sprintf(userLimitHeaderPrefix, millionValue)
}

// GetRateLimitRequestHeader:
//   - Returns rate limit header for account ID
//   - If the account is not rate limited, returns nil
//
// Examples:
//   - PLAN_FREE: "Rl-Plan-Free: <account-id>"
//   - 40M relay limit: "Rl-User-Limit-40: <account-id>"
//   - 10M relay limit: "Rl-User-Limit-10: <account-id>"
func GetRateLimitRequestHeader(portalApp *store.PortalApp) *envoy_core.HeaderValueOption {
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
		rateLimitHeader := getUserLimitRequestHeader(rateLimit.MonthlyUserLimit)

		return &envoy_core.HeaderValueOption{
			Header: &envoy_core.HeaderValue{
				Key:   rateLimitHeader,
				Value: string(portalApp.AccountID),
			},
		}
	}

	return nil
}
