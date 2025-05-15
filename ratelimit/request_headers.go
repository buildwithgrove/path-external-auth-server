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
// - Key: plan type from DB
// - Value: header key matched in GUARD config
const (
	// KV Pair for the free plan
	PlanFree_DatabaseType  = "PLAN_FREE"    // Free plan key (i.e. database value)
	PlanFree_RequestHeader = "Rl-Plan-Free" // Free plan value (i.e. request header key)
)

// Map: DB plan type -> GUARD header key
// Example:
// - "PLAN_FREE" -> "Rl-Plan-Free"
var rateLimitedPlanTypeHeaders = map[store.PlanType]string{
	PlanFree_DatabaseType: PlanFree_RequestHeader,
}

// Prefix for user-specific rate limit headers
// - Integer = monthly relay limit (in millions)
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
// - Returns rate limit header for endpoint ID + metadata
// Examples:
//   - PLAN_FREE: "Rl-Plan-Free: <endpoint-id>"
//   - 40M relay limit: "Rl-User-Limit-40: <endpoint-id>"
//   - 10M relay limit: "Rl-User-Limit-10: <endpoint-id>"
func GetRateLimitRequestHeader(portalApp *store.PortalApp) *envoy_core.HeaderValueOption {
	// Return nil if the portal app is not rate limited.
	if portalApp.RateLimit == nil {
		return nil
	}

	rateLimit := portalApp.RateLimit

	// First check if the portal app is rate limited by plan type.
	// e.g. "Rl-Plan-Free: <portal-app-id>"
	if rateLimitHeader, ok := rateLimitedPlanTypeHeaders[rateLimit.PlanType]; ok {
		return &envoy_core.HeaderValueOption{
			Header: &envoy_core.HeaderValue{
				Key:   rateLimitHeader,
				Value: string(portalApp.PortalAppID),
			},
		}
	}

	// Then check if the portal app is rate limited by user-specified monthly limit.
	// e.g. "Rl-User-Limit-40: <portal-app-id>" = 40 million monthly user limit
	if rateLimit.MonthlyUserLimit > 0 {
		rateLimitHeader := getUserLimitRequestHeader(rateLimit.MonthlyUserLimit)
		return &envoy_core.HeaderValueOption{
			Header: &envoy_core.HeaderValue{
				Key:   rateLimitHeader,
				Value: string(portalApp.PortalAppID),
			},
		}
	}

	return nil
}
