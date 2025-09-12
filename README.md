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
- [Prometheus Metrics](#prometheus-metrics)
  - [Key Metrics](#key-metrics)
  - [Usage](#usage)
- [Getting Portal App Auth \& Rate Limit Status](#getting-portal-app-auth--rate-limit-status)
  - [Prerequisites](#prerequisites)
  - [Usage](#usage-1)
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

## Prometheus Metrics

PEAS exposes Prometheus metrics on the `/metrics` endpoint for monitoring authorization performance and system health.

### Key Metrics

| Metric                                  | Type      | Description                                                                     |
| --------------------------------------- | --------- | ------------------------------------------------------------------------------- |
| `peas_auth_requests_total`              | Counter   | Authorization requests by portal app, account, status, and error type           |
| `peas_auth_request_duration_seconds`    | Histogram | Authorization request processing time                                           |
| `peas_rate_limit_checks_total`          | Counter   | Rate limiting decisions by account, plan type, and outcome                      |
| `peas_store_size_total`                 | Gauge     | Current size of in-memory stores (portal apps, accounts, rate limited accounts) |
| `peas_account_usage_total`              | Gauge     | Monthly usage for accounts exceeding their limits                               |
| `peas_data_source_refresh_errors_total` | Counter   | Errors during Postgres and BigQuery data refresh operations                     |

### Usage

Metrics are automatically collected when PEAS is running. The metrics server exposes multiple endpoints:

- `/metrics` - Prometheus metrics endpoint  
- `/healthz` - Health check endpoint (returns JSON with status, service, and version)

Configure your Prometheus server to scrape the `/metrics` endpoint on port `9090` (or the port specified by `METRICS_PORT`).

PEAS also provides a pprof server on port `6060` (or `PPROF_PORT`) at `/debug/pprof/` for runtime profiling.

**Example Grafana Queries:**
```promql
# Authorization success rate
rate(peas_auth_requests_total{status="authorized"}[5m])

# Accounts over monthly limit (current month)
peas_account_usage_total

# Rate limiting effectiveness
peas_rate_limit_checks_total{decision="rate_limited"}
```

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
| METRICS_PORT               | ❌        | int      | The port to run the Prometheus metrics server on                      | 9090                                                 | 9090          |
| PPROF_PORT                 | ❌        | int      | The port to run the pprof server on                                   | 6060                                                 | 6060          |
| IMAGE_TAG                  | ❌        | string   | The image tag/version for the application                             | v1.0.0                                               | unknown       |
| LOGGER_LEVEL               | ❌        | string   | The log level to use for the external auth server                     | info                                                 | info          |
| REFRESH_INTERVAL           | ❌        | duration | The interval for refreshing portal app data from the database         | 30s, 1m, 2m30s                                       | 30s           |

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
   - **Grafana**: `http://localhost:3000` (admin/admin)
4. **View the dashboard**:
   - Go to Grafana at `http://localhost:3000`
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
The PEAS dashboard is automatically provisioned in Grafana. You can import or update it by replacing the dashboard JSON file in `grafana/dashboard.json`.

**Note:** The dashboard layout and panels may change over time. Refer to the dashboard in Grafana for the latest view.

#### Dashboard Screenshot
![Dashboard Screenshot](./.github/dashboard.png)
