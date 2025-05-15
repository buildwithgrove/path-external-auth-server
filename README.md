<div align="center">
<h1>🫛 PEAS<br/>PATH External Auth Server</h1>
<img src="https://storage.googleapis.com/grove-brand-assets/Presskit/Logo%20Joined-2.png" alt="Grove logo" width="500"/>

</div>
<br/>

- [Introduction](#introduction)
  - [Docker Image](#docker-image)
  - [Architecture Diagram](#architecture-diagram)
  - [`PortalApp` Structure](#portalapp-structure)
- [Request Headers](#request-headers)
- [Rate Limiting Implementation](#rate-limiting-implementation)
- [Envoy Gateway Integration](#envoy-gateway-integration)
- [PEAS Environment Variables](#peas-environment-variables)


## Introduction

**PEAS** (PATH External Auth Server) is an external authorization server that can be used to authorize requests to the [PATH Gateway](https://github.com/buildwithgrove/path). It is part of the GUARD authorization system for PATH and runs in the PATH Kubernetes cluster.

### Docker Image

```bash
docker pull ghcr.io/buildwithgrove/path-external-auth-server:latest
```

- [PEAS GHCR Package](https://github.com/orgs/buildwithgrove/packages/container/package/path-external-auth-server)
### Architecture Diagram

```mermaid
graph TD
    User[/"<big>PATH<br>User</big>"\]
    Envoy[<big>Envoy Proxy</big>]

    AUTH["PEAS (PATH External Auth Server)"]
    AUTH_DECISION{Did<br>Authorize<br>Request?}
    PATH[<big>PATH</big>]

    Error[[Error Returned to User]]
    Result[[Result Returned to User]]

    GroveDB[("Grove Portal Database<br>(Postgres)")]

    subgraph AUTH["PEAS<br/>PATH External Auth Server"]
    end

    User -->|1.Send Request| Envoy
    Envoy -.->|2.Authorization Check<br>gRPC| AUTH
    AUTH -.->|3.Authorization Result<br>gRPC| Envoy
    Envoy --> AUTH_DECISION
    AUTH_DECISION -->|4.No <br> Forward Request| Error
    AUTH_DECISION -->|4.Yes <br> Forward Request| PATH
    PATH -->|5.Response| Result

    GroveDB <-->|Postgres Connection| AUTH
```

### `PortalApp` Structure

`PEAS` manages authentication and assigning rate limiting headers for portal apps that are defined with the following structure:

```go
// PortalApp represents a single portal app for a user's account.
type PortalApp struct {
	// Unique identifier for the PortalApp.
	ID PortalAppID
	// Unique identifier for the PortalApp's account.
	AccountID AccountID
	// The authorization settings for the PortalApp.
	// Auth can be one of:
	//   - NoAuth: The portal app does not require authorization (Auth will be nil)
	//   - APIKey: The portal app uses an API key for authorization
	Auth *Auth
	// Rate Limiting settings for the PortalApp.
	// If the portal app is not rate limited, RateLimit will be nil.
	RateLimit *RateLimit
}

// Auth represents the authorization settings for a PortalApp.
// Only API key auth is supported by the Grove Portal.
type Auth struct {
	APIKey string
}

// RateLimit contains rate limiting settings for a PortalApp.
type RateLimit struct {
	PlanType         PlanType
	MonthlyUserLimit int32
}
```

`PortalApp` data is sourced from a Postgres database compatible with the Grove Portal Database. For more information about the Grove Postgres integration, see the [Grove Postgres README](./postgres/grove/README.md).

## Request Headers

PEAS adds the following headers to authorized requests before forwarding them to the upstream service:

| Header                  | Contents                                                                               | Included For All Requests                                            | Example Value |
| ----------------------- | -------------------------------------------------------------------------------------- | -------------------------------------------------------------------- | ------------- |
| `Portal-Application-ID` | The portal app ID of the authorized portal app                                         | ✅                                                                    | "a12b3c4d"    |
| `Portal-Account-ID`     | The account ID associated with the portal app                                          | ✅                                                                    | "3f4g2js2"    |
| `Rl-Plan-Free`          | The account ID for rate limiting purposes (PLAN_FREE)                                  | ❌ (Only for `PLAN_FREE` portal apps)                                 | "3f4g2js2"    |
| `Rl-User-Limit-<X>`     | The account ID for rate limiting purposes with a user limit *(X = relays in millions)* | ❌ (Only for `PLAN_UNLIMITED` portal apps with user-specified limits) | "3f4g2js2"    |

## Rate Limiting Implementation

PEAS provides rate limiting capabilities through the following mechanisms:

1. **Plan-Based Rate Limiting**: For `PLAN_FREE` portal apps, PEAS will add headers like `Rl-Plan-Free: <account-id>`.

2. **User-Based Rate Limiting**: For `PLAN_UNLIMITED` portal apps with user-specified monthly limits, PEAS adds headers based on the limit in millions:
   - 10 million monthly user limit: `Rl-User-Limit-10: <account-id>`
   - 40 million monthly user limit: `Rl-User-Limit-40: <account-id>`
   - etc..

These headers are processed by the Envoy rate limiter configured in the GUARD system, allowing for granular control over request rates.

## Envoy Gateway Integration

PEAS exposes a gRPC service that adheres to the spec expected by Envoy Proxy's `ext_authz` HTTP Filter.

<div align="center">
  <a href="https://www.envoyproxy.io/docs/envoy/latest/">
    <img src="https://raw.githubusercontent.com/cncf/artwork/refs/heads/main/projects/envoy/envoy-gateway/horizontal/color/envoy-gateway-horizontal-color.svg" alt="Envoy logo" width="200"/>
  </a>
</div>

For more information see:
- [Envoy Gateway External Authorization Docs](https://gateway.envoyproxy.io/docs/tasks/security/ext-auth/)
- [Envoy Proxy `ext_authz` HTTP Filter Docs](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/ext_authz_filter)

## PEAS Environment Variables

PEAS is configured via environment variables.

| Variable                   | Required | Type   | Description                                                           | Example                                              | Default Value |
| -------------------------- | -------- | ------ | --------------------------------------------------------------------- | ---------------------------------------------------- | ------------- |
| POSTGRES_CONNECTION_STRING | ✅        | string | The PostgreSQL connection string for the database with PortalApp data | postgresql://username:password@localhost:5432/dbname | -             |
| PORT                       | ❌        | int    | The port to run the external auth server on                           | 10001                                                | 10001         |
| LOGGER_LEVEL               | ❌        | string | The log level to use for the external auth server                     | info                                                 | info          |
