# 🫛 PEAS <!-- omit in toc -->

<div align="center">
<h1>🫛 PEAS<br/>PATH External Auth Server</h1>
<img src="https://storage.googleapis.com/grove-brand-assets/Presskit/Logo%20Joined-2.png" alt="Grove logo" width="500"/>
</div>
<br/>

- [Introduction](#introduction)
- [PEAS Responsibilities](#peas-responsibilities)
  - [Authenticating Requests](#authenticating-requests)
  - [Assigning Rate Limiting Headers](#assigning-rate-limiting-headers)
  - [Docker Image](#docker-image)
  - [Architecture Diagram](#architecture-diagram)
  - [`PortalApp` Structure](#portalapp-structure)
- [Request Headers](#request-headers)
- [Rate Limiting Implementation](#rate-limiting-implementation)
  - [How does Rate Limiting Work?](#how-does-rate-limiting-work)
  - [Rate Limit Store Refresh](#rate-limit-store-refresh)
- [Portal App Store Refresh](#portal-app-store-refresh)
  - [How does Portal App Store Refresh Work?](#how-does-portal-app-store-refresh-work)
  - [Configuration](#configuration)
- [PostgREST Data Source](#postgrest-data-source)
  - [PostgREST Configuration](#postgrest-configuration)
  - [JWT Authentication](#jwt-authentication)
- [Envoy Gateway Integration](#envoy-gateway-integration)
- [Prometheus Metrics](#prometheus-metrics)
  - [Key Metrics](#key-metrics)
  - [Endpoints](#endpoints)
- [Getting Portal App Auth \& Rate Limit Status](#getting-portal-app-auth--rate-limit-status)
  - [Prerequisites](#prerequisites)
  - [Usage](#usage)
  - [Example Output](#example-output)
- [PEAS Environment Variables](#peas-environment-variables)
- [Developing Metrics Dashboard Locally](#developing-metrics-dashboard-locally)
  - [Stack Components](#stack-components)
  - [Prerequisites](#prerequisites-1)
  - [Quick Start](#quick-start)
  - [Local Testing](#local-testing)
    - [Test the health endpoint:](#test-the-health-endpoint)
    - [Test the metrics endpoint:](#test-the-metrics-endpoint)
    - [Generate some test traffic:](#generate-some-test-traffic)
    - [Load Testing](#load-testing)
  - [Cleanup](#cleanup)
  - [Dashboard](#dashboard)
    - [Dashboard Screenshot](#dashboard-screenshot)
    - [Importing Dashboard to Production Grafana](#importing-dashboard-to-production-grafana)

## Introduction

**PEAS** (PATH External Auth Server) is an external authorization server that can
be used to authorize requests to the [PATH Gateway](https://github.com/buildwithgrove/path).

It is part of the GUARD authorization system for PATH and runs in the PATH Kubernetes cluster.

## PEAS Responsibilities

PEAS has the following two responsibilities:

### Authenticating Requests

**Is request to GUARD authorized?**

- If authorized, forward the request upstream
- If not authorized, return an error

### Assigning Rate Limiting Headers

**Is request to GUARD rate limited?**

- If rate limited, forward the request upstream with rate limiting headers
- If not rate limited, forward the request upstream without rate limiting headers

Data for authentication and rate limiting is sourced from the Grove Portal Database. For more information about the Grove Portal Database, see the [Grove Portal Database README](./postgres/grove/README.md).

### Docker Image

[PEAS GHCR Package](https://github.com/orgs/buildwithgrove/packages/container/package/path-external-auth-server)

```bash
docker pull ghcr.io/buildwithgrove/path-external-auth-server:latest
```

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

| Header                  | Contents                                       | Included For All Requests | Example Value |
| ----------------------- | ---------------------------------------------- | ------------------------- | ------------- |
| `Portal-Application-ID` | The portal app ID of the authorized portal app | ✅                         | "a12b3c4d"    |
| `Portal-Account-ID`     | The account ID associated with the portal app  | ✅                         | "3f4g2js2"    |

## Rate Limiting Implementation

PEAS provides rate limiting capabilities through an in-memory rate limit store that tracks account usage and enforces monthly limits:

### How does Rate Limiting Work?

1. **Rate Limit Store**: Maintains an in-memory map of rate limited accounts, refreshed periodically from BigQuery data warehouse
2. **Monthly Usage Tracking**: Monitors account usage against their monthly relay limits based on plan type
3. **Plan-Based Limits**:
   - **Free Plan (`PLAN_FREE`)**: 1,000,000 relays per month
   - **Unlimited Plan (`PLAN_UNLIMITED`)**: Custom limits set per account, or unlimited if no limit specified
4. **Real-time Enforcement**: Blocks requests from accounts that exceed their monthly limits

### Rate Limit Store Refresh

The rate limit store automatically refreshes from the data warehouse to update account usage:

- **Default Refresh Interval**: 5 minutes
- **Data Source**: BigQuery data warehouse for monthly usage statistics
- **Configuration**: `RATE_LIMIT_STORE_REFRESH_INTERVAL` environment variable
- **Monitoring**: Refresh operations are logged and metrics are available via Prometheus

## Portal App Store Refresh

PEAS maintains an in-memory store of portal app data for fast authorization lookups. This store is automatically refreshed from the Grove Portal Database on a configurable interval.

### How does Portal App Store Refresh Work?

1. **Initial Load**: On startup, PEAS fetches all portal app data from the database to populate the in-memory store
2. **Background Refresh**: A background goroutine periodically refreshes the store by fetching the latest data from the database
3. **Thread-Safe Updates**: The store uses read-write locks to ensure thread-safe access during refresh operations
4. **Performance Monitoring**: Each refresh operation is timed and logged with metrics for monitoring

### Configuration

The refresh interval is configurable via the `REFRESH_INTERVAL` environment variable:

- **Default**: 30 seconds
- **Format**: Duration string (e.g., `30s`, `1m`, `2m30s`)
- **Purpose**: Balance between data freshness and database load

## PostgREST Data Source

PEAS supports [PostgREST](https://docs.postgrest.org/en/v13/) as an alternative data source to direct PostgreSQL connections. PostgREST provides a RESTful API layer over PostgreSQL databases, enabling PEAS to fetch portal application and account data via HTTP rather than direct database connections.

For more information about PostgREST configuration and usage, see the [official PostgREST documentation](https://docs.postgrest.org/en/v13/).

<div align="center">
  <a href="https://docs.postgrest.org/en/v13/">
    <img src="https://github.com/PostgREST/postgrest/blob/main/static/logo.png?raw=true" alt="PostgREST logo" width="200"/>
  </a>
</div>

### PostgREST Configuration

To use PostgREST as the data source, configure the following environment variables:

```bash
# Set data source type to PostgREST
DATA_SOURCE_TYPE="postgrest"

# PostgREST API endpoint
POSTGREST_BASE_URL="https://db.rpc.com/api"

# JWT authentication (required for PostgREST access)
POSTGREST_JWT_SECRET="supersecretjwtsecretforlocaldevelopment123456789"
POSTGREST_JWT_EMAIL="service@rpc.com"
POSTGREST_JWT_ROLE="admin"

# Request timeout (optional)
POSTGREST_TIMEOUT="30s"  # Optional, defaults to 30s
```

### JWT Authentication

PEAS generates fresh JWT tokens for each PostgREST API request using the configured secret, email, and role. 

This ensures secure access to the PostgREST API with proper authentication and authorization based on your [PostgREST JWT configuration](https://docs.postgrest.org/en/v13/references/auth.html).

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

## Prometheus Metrics

PEAS exposes Prometheus metrics on the `/metrics` endpoint for monitoring authorization performance, rate limiting, and system health.

### Key Metrics

- **Authorization Metrics**: Request counts, success rates, and response times
- **Rate Limiting Metrics**: Account usage, rate limit decisions, and store sizes
- **System Health**: Data source refresh errors and store performance

### Endpoints

- `/metrics` - Prometheus metrics endpoint (port `9090` by default)
- `/healthz` - Health check endpoint
- `/debug/pprof/` - Runtime profiling (port `6060` by default)

A comprehensive Grafana dashboard is available at `grafana/dashboard.json` for visualizing all metrics.

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
    "message": "This account is rate limited. To upgrade your plan or modify your account settings, log in to your account at https://portal.grove.city/"
  },
  "deniedResponse": {
    "status": {
      "code": "TooManyRequests"
    },
    "body": "{\"code\": 429, \"message\": \"This account is rate limited. To upgrade your plan or modify your account settings, log in to your account at https://portal.grove.city/\"}"
  }
}
```

This tool uses gRPC reflection to communicate with PEAS, testing the same authorization flow that Envoy Gateway uses in production.

## PEAS Environment Variables

PEAS is configured via environment variables.

| Variable                          | Required | Type     | Description                                                                        | Example                                              | Default Value |
| --------------------------------- | -------- | -------- | ---------------------------------------------------------------------------------- | ---------------------------------------------------- | ------------- |
| **Data Source Configuration**     |          |          |                                                                                    |                                                      |               |
| DATA_SOURCE_TYPE                  | ❌        | string   | Data source type: "postgres" or "postgrest"                                        | postgres, postgrest                                  | postgres      |
| POSTGRES_CONNECTION_STRING        | ✅*       | string   | PostgreSQL connection string (required when DATA_SOURCE_TYPE=postgres)             | postgresql://username:password@localhost:5432/dbname | -             |
| POSTGREST_BASE_URL                | ✅*       | string   | PostgREST API base URL (required when DATA_SOURCE_TYPE=postgrest)                  | https://db.rpc.com/api                               | -             |
| POSTGREST_JWT_SECRET              | ✅*       | string   | JWT secret for PostgREST authentication (required when DATA_SOURCE_TYPE=postgrest) | supersecretjwtsecretforlocaldevelopment123456789     | -             |
| POSTGREST_JWT_EMAIL               | ✅*       | string   | JWT email for PostgREST authentication (required when DATA_SOURCE_TYPE=postgrest)  | service@rpc.com                                      | -             |
| POSTGREST_JWT_ROLE                | ❌        | string   | JWT role for PostgREST authentication                                              | admin                                                | -             |
| POSTGREST_TIMEOUT                 | ❌        | duration | PostgREST request timeout                                                          | 30s, 1m, 2m30s                                       | 30s           |
| **System Configuration**          |          |          |                                                                                    |                                                      |               |
| GCP_PROJECT_ID                    | ✅        | string   | GCP project ID for the data warehouse used by rate limiting                        | your-project-id                                      | -             |
| PORT                              | ❌        | int      | Port to run the external auth server on                                            | 10001                                                | 10001         |
| METRICS_PORT                      | ❌        | int      | Port to run the Prometheus metrics server on                                       | 9090                                                 | 9090          |
| PPROF_PORT                        | ❌        | int      | Port to run the pprof server on                                                    | 6060                                                 | 6060          |
| LOGGER_LEVEL                      | ❌        | string   | Log level for the external auth server                                             | info, debug, warn, error                             | info          |
| IMAGE_TAG                         | ❌        | string   | Image tag/version for the application                                              | v1.0.0                                               | development   |
| PORTAL_APP_STORE_REFRESH_INTERVAL | ❌        | duration | Refresh interval for portal app data from the database                             | 30s, 1m, 2m30s                                       | 30s           |
| RATE_LIMIT_STORE_REFRESH_INTERVAL | ❌        | duration | Refresh interval for rate limit data from the data warehouse                       | 30s, 1m, 2m30s                                       | 5m            |

**\* Required when the corresponding DATA_SOURCE_TYPE is selected**

## Developing Metrics Dashboard Locally

This section describes how to run and test the PEAS metrics dashboard locally using Docker Compose, Prometheus, and Grafana.

### Stack Components

- **Prometheus**: Metrics collection from locally running PEAS
- **Grafana**: Dashboard visualization using the PEAS dashboard

### Prerequisites

- **PEAS running locally**: Run PEAS directly on your machine (not in Docker)
- Create a `.env` file in the parent directory (`../`) with your database credentials and configuration
- Ensure you have access to your remote PostgreSQL and GCP BigQuery instances

### Quick Start

1. **Start the monitoring stack**:
   ```bash
   cd grafana/local
   docker compose up -d
   ```
2. **Start PEAS locally** (in another terminal, from repo root):
   ```bash
   go run .
   ```
3. **Access the services**:
   - **PEAS gRPC Server**: `localhost:10001`
   - **PEAS Metrics**: `http://localhost:9090/metrics`
   - **PEAS Health**: `http://localhost:9090/healthz`
   - **PEAS pprof**: `http://localhost:6060/debug/pprof/`
   - **Prometheus**: `http://localhost:9091`
   - **Grafana**: `https://db.rpc.com/api` (admin/admin)
4. **View the dashboard**:
   - Go to Grafana at `https://db.rpc.com/api`
   - Login with admin/admin
   - The PEAS dashboard should be automatically loaded

### Local Testing

#### Test the health endpoint:

```bash
curl http://localhost:9090/healthz | jq
```

#### Test the metrics endpoint:

```bash
curl http://localhost:9090/metrics | grep peas_
```

#### Generate some test traffic:

Since PEAS is a gRPC server, you can use grpcurl to send test requests:

```bash
# Install grpcurl if needed
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
# Test authorization request (will likely fail but generate metrics)
grpcurl -plaintext localhost:10001 envoy.service.auth.v3.Authorization/Check
```

#### Load Testing

You can run a load test using the provided script:

```bash
make load-test
```

Or with custom parameters:

```bash
make load-test-custom TOTAL_REQUESTS=5000 SUCCESS_RATE=80
```

### Cleanup

```bash
docker compose down -v  # Removes containers and volumes
```

### Dashboard

The PEAS dashboard is automatically provisioned in Grafana when running the observability stack locally for development purposes.

For production deployments, you can import the dashboard manually.

#### Dashboard Screenshot

![Dashboard Screenshot](./.github/dashboard.png)

#### Importing Dashboard to Production Grafana

To import the PEAS dashboard into your production Grafana instance follow the documentation on the [Grafana documentation](https://grafana.com/docs/grafana/latest/dashboards/build-dashboards/import-dashboards/).
