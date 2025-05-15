package ratelimit

import (
	"fmt"

	envoy_core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"

	"github.com/buildwithgrove/path-external-auth-server/proto"
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
var rateLimitedPlanTypeHeaders = map[string]string{
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
func GetRateLimitRequestHeader(gatewayEndpoint *proto.GatewayEndpoint) *envoy_core.HeaderValueOption {
	metadata := gatewayEndpoint.GetMetadata()

	// Get the plan type from the metadata
	planType := metadata.GetPlanType()
	if planType == "" {
		return nil
	}

	// Get the endpoint ID from the gateway endpoint
	endpointID := gatewayEndpoint.GetEndpointId()

	// Check if request is for a rate-limited plan (e.g., PLAN_FREE)
	// Example: "Rl-Plan-Free: <endpoint-id>"
	if rateLimitHeader, ok := rateLimitedPlanTypeHeaders[planType]; ok {
		return &envoy_core.HeaderValueOption{
			Header: &envoy_core.HeaderValue{
				Key:   rateLimitHeader,
				Value: endpointID,
			},
		}
	}

	// Otherwise, check if endpoint has user-specific monthly limit
	// Example: "Rl-User-Limit-40: <endpoint-id>" (40M monthly limit)
	if monthlyRelayLimit := metadata.GetMonthlyRelayLimit(); monthlyRelayLimit > 0 {
		header := getUserLimitRequestHeader(monthlyRelayLimit)
		return &envoy_core.HeaderValueOption{
			Header: &envoy_core.HeaderValue{
				Key:   header,
				Value: endpointID,
			},
		}
	}

	return nil
}
