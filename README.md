# 🫛 PEAS <!-- omit in toc -->

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
- [Portal App Store Refresh](#portal-app-store-refresh)
  - [How It Works](#how-it-works)
  - [Configuration](#configuration)
- [Envoy Gateway Integration](#envoy-gateway-integration)
- [Getting Portal App Auth \& Rate Limit Status](#getting-portal-app-auth--rate-limit-status)
  - [Prerequisites](#prerequisites)
  - [Usage](#usage)
  - [Example Output](#example-output)
- [PEAS Environment Variables](#peas-environment-variables)

## Introduction

**PEAS** (PATH External Auth Server) is an external authorization server that can be used to authorize requests to the [PATH Gateway](https://github.com/buildwithgrove/path). 

It is part of the GUARD authorization system for PATH and runs in the PATH Kubernetes cluster.

It has the following two responsibilities:

1. Authenticating Requests
   - Determines if requests to GUARD are authorized
   - If the request is authorized, the request is forwarded upstream
2. Assigning Rate Limiting Headers
   - Assigns rate limiting headers to requests to GUARD
   - These headers are forwarded to the upstream Envoy rate limit service
  
Data for authentication and rate limiting is sourced from the Grove Portal Database. For more information about the Grove Portal Database, see the [Grove Portal Database README](./postgres/grove/README.md).

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

The `PortalApp` structure is defined in the `store` package and contains all data required from the Grove Portal Database for authorization and rate limiting.

See `PortalApp` structure [here](https://github.com/buildwithgrove/path-external-auth-server/blob/main/store/portal_app.go).

## Request Headers

PEAS adds the following headers to authorized requests before forwarding them to the upstream service:

| Header                  | Contents                                                                               | Included For All Requests                                            | Example Value |
| ----------------------- | -------------------------------------------------------------------------------------- | -------------------------------------------------------------------- | ------------- |
| `Portal-Application-ID` | The portal app ID of the authorized portal app                                         | ✅                                                                    | "a12b3c4d"    |
| `Portal-Account-ID`     | The account ID associated with the portal app                                          | ✅                                                                    | "3f4g2js2"    |
| `Rl-Plan-Free`          | The account ID for rate limiting purposes (PLAN_FREE)                                  | ❌ (Only for `PLAN_FREE` portal apps)                                 | "3f4g2js2"    |
| `Rl-User-Limit-<X>`     | The account ID for rate limiting purposes with a user limit _(X = relays in millions)_ | ❌ (Only for `PLAN_UNLIMITED` portal apps with user-specified limits) | "3f4g2js2"    |

## Rate Limiting Implementation

PEAS provides rate limiting capabilities through the following mechanisms:

1. **Plan-Based Rate Limiting**: For `PLAN_FREE` portal apps, PEAS will add headers like `Rl-Plan-Free: <account-id>`.

2. **User-Based Rate Limiting**: For `PLAN_UNLIMITED` portal apps with user-specified monthly limits, PEAS adds headers based on the limit in millions:
   - 10 million monthly user limit: `Rl-User-Limit-10: <account-id>`
   - 40 million monthly user limit: `Rl-User-Limit-40: <account-id>`
   - etc..

These headers are processed by the Envoy rate limiter configured in the GUARD system, allowing for granular control over request rates.

## Portal App Store Refresh

PEAS maintains an in-memory store of portal app data for fast authorization lookups. This store is automatically refreshed from the Grove Portal Database on a configurable interval.

### How It Works

1. **Initial Load**: On startup, PEAS fetches all portal app data from the database to populate the in-memory store
2. **Background Refresh**: A background goroutine periodically refreshes the store by fetching the latest data from the database
3. **Thread-Safe Updates**: The store uses read-write locks to ensure thread-safe access during refresh operations
4. **Performance Monitoring**: Each refresh operation is timed and logged with metrics for monitoring

### Configuration

The refresh interval is configurable via the `REFRESH_INTERVAL` environment variable:

- **Default**: 30 seconds
- **Format**: Duration string (e.g., `30s`, `1m`, `2m30s`)
- **Purpose**: Balance between data freshness and database load

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

## Getting Portal App Auth & Rate Limit Status

PEAS includes a convenient Makefile target for testing authorization and rate limit status for Portal Apps during development.

### Prerequisites

- PEAS server running on `localhost:10001`
- `grpcurl` installed ([installation guide](https://github.com/fullstorydev/grpcurl#installation))
- `jq` installed for JSON formatting ([installation guide](https://jqlang.github.io/jq/download/))

### Usage

```bash
# Test without API key (for apps that don't require authentication)
make get_portal_app_auth_status PORTAL_APP_ID=1a2b3c4d

# Test with API key (for apps that require authentication)  
make get_portal_app_auth_status PORTAL_APP_ID=1a2b3c4d API_KEY=4c352139ec5ca9288126300271d08867
```

### Example Output

**Successful Authorization:**
```json
{
  "status": {
    "message": "ok"
  },
  "okResponse": {
    "headers": [
      {
        "header": {
          "key": "Portal-Application-ID",
          "value": "1a2b3c4d"
        }
      },
      {
        "header": {
          "key": "Portal-Account-ID", 
          "value": "d4c3b2a1"
        }
      }
    ]
  }
}
```

**Failed Authorization:**
```json
{
  "status": {
    "code": 7,
    "message": "portal app not found"
  },
  "deniedResponse": {
    "status": {
      "code": "NotFound"
    },
    "body": "{\"code\": 404, \"message\": \"portal app not found\"}"
  }
}
```

**Failed Rate Limit Check:**
```json
{
  "status": {
    "code": 7,
    "message": "account is rate limited"
  },
  "deniedResponse": {
    "status": {
      "code": "TooManyRequests"
    },
    "body": "{\"code\": 429, \"message\": \"account is rate limited\"}"
  }
}
```

This tool uses gRPC reflection to communicate with PEAS, testing the same authorization flow that Envoy Gateway uses in production.

## PEAS Environment Variables

PEAS is configured via environment variables.

| Variable                   | Required | Type     | Description                                                           | Example                                              | Default Value |
| -------------------------- | -------- | -------- | --------------------------------------------------------------------- | ---------------------------------------------------- | ------------- |
| POSTGRES_CONNECTION_STRING | ✅        | string   | The PostgreSQL connection string for the database with PortalApp data | postgresql://username:password@localhost:5432/dbname | -             |
| PORT                       | ❌        | int      | The port to run the external auth server on                           | 10001                                                | 10001         |
| LOGGER_LEVEL               | ❌        | string   | The log level to use for the external auth server                     | info                                                 | info          |
| REFRESH_INTERVAL           | ❌        | duration | The interval for refreshing portal app data from the database         | 30s, 1m, 2m30s                                       | 30s           |
