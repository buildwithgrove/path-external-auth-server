package ratelimit

import (
	"fmt"

	envoy_core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"

	"github.com/buildwithgrove/path-external-auth-server/proto"
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
//
// Example:
//   - "PLAN_FREE" -> "Rl-Plan-Free"
var rateLimitedPlanTypeHeaders = map[string]string{
	DBPlanFree: PlanFreeHeader,
}

// Prefix for user-specific rate limit headers
// where the integer value is the monthly relay limit in millions
//
// Example:
//   - monthlyRelayLimit of 40,000,000 returns "Rl-User-Limit-40"
//   - monthlyRelayLimit of 10,000,000 returns "Rl-User-Limit-10"
const userLimitHeaderPrefix = "Rl-User-Limit-%d"

// getUserLimitHeader generates a rate limit header based on the monthly relay limit.
// `monthlyRelayLimit` must be set in intervals of 1,000,000.
//
// Example:
//   - monthlyRelayLimit of 40,000,000 returns "Rl-User-Limit-40"
//   - monthlyRelayLimit of 10,000,000 returns "Rl-User-Limit-10"
func getUserLimitHeader(monthlyRelayLimit int32) string {
	// Convert to millions and format as header
	millionValue := monthlyRelayLimit / 1_000_000
	return fmt.Sprintf(userLimitHeaderPrefix, millionValue)
}

// getRateLimitHeader returns the rate limit header for the given endpoint ID
// and metadata.
//
// Examples:
//   - PLAN_FREE: "Rl-Plan-Free: <endpoint-id>"
//   - 40 million monthly relay limit: "Rl-User-Limit-40: <endpoint-id>"
//   - 10 million monthly relay limit: "Rl-User-Limit-10: <endpoint-id>"
func GetRateLimitHeader(gatewayEndpoint *proto.GatewayEndpoint) *envoy_core.HeaderValueOption {
	metadata := gatewayEndpoint.GetMetadata()

	planType := metadata.GetPlanType()
	if planType == "" {
		return nil
	}

	endpointID := gatewayEndpoint.GetEndpointId()

	// First check if the request is for a rate limited plan, eg. PLAN_FREE
	// e.g. "Rl-Plan-Free: <endpoint-id>"
	if rateLimitHeader, ok := rateLimitedPlanTypeHeaders[planType]; ok {
		return &envoy_core.HeaderValueOption{
			Header: &envoy_core.HeaderValue{
				Key:   rateLimitHeader,
				Value: endpointID,
			},
		}
	}

	// Then check if the request is for an gateway endpoint with a user-specific monthly user limit
	// e.g. "Rl-User-Limit-40: <endpoint-id>" = 40 million monthly user limit
	if monthlyRelayLimit := metadata.GetMonthlyRelayLimit(); monthlyRelayLimit > 0 {
		header := getUserLimitHeader(monthlyRelayLimit)
		return &envoy_core.HeaderValueOption{
			Header: &envoy_core.HeaderValue{
				Key:   header,
				Value: endpointID,
			},
		}
	}

	return nil
}
