# 🫛 PEAS

<div align="center">
<h1>🫛 PEAS<br/>PATH External Auth Server</h1>
<img src="https://storage.googleapis.com/grove-brand-assets/Presskit/Logo%20Joined-2.png" alt="Grove logo" width="500"/>
</div>
<br/>

## Introduction

**PEAS** (PATH External Auth Server) is an external authorization server that can be used to authorize requests to the [PATH Gateway](https://github.com/buildwithgrove/path). It is part of the GUARD authorization system for PATH and runs in the PATH Kubernetes cluster.

### Docker Image

```bash
docker pull ghcr.io/buildwithgrove/path-external-auth-server:latest
```

- [PEAS GHCR Package](https://github.com/orgs/buildwithgrove/packages/container/package/path-external-auth-server)

### `GatewayEndpoint` Structure

PEAS receives a list of `GatewayEndpoints` that define which endpoints are authorized to use the PATH Gateway.

- [`GatewayEndpoint` protobuf definition.](https://github.com/buildwithgrove/path-external-auth-server/blob/main/proto/gateway_endpoint.proto)

`GatewayEndpoint` data is received from a `remote gRPC server` that may be implemented by a PATH gateway operator in any way they see fit. The only requirement is that it adhere to the to spec defined in the protobuf definition.

## Request Headers

PEAS adds the following headers to authorized requests before forwarding them to the upstream service:

| Header                  | Contents                                               | Included For All Requests           | Example Value |
| ----------------------- | ------------------------------------------------------ | ----------------------------------- | ------------- |
| `Portal-Application-ID` | The portal application ID of the authorized portal app | ✅                                   | "a12b3c4d"    |
| `Portal-Account-ID`     | The account ID associated with the portal application  | ✅                                   | "3f4g2js2"    |
| `Rl-Plan-Free`          | The endpoint ID for rate limiting purposes (PLAN_FREE) | ❌ (Only for rate-limited endpoints) | "a12b3c4d"    |

## 🐾 PADS (PATH Auth Data Server)

PADS provides data from either a static YAML file or a Grove-specific Postgres database implementation.

- [PADS Repository](https://github.com/buildwithgrove/path-auth-data-server)

```bash
docker pull ghcr.io/buildwithgrove/path-auth-data-server:latest
```

- [PADS GHCR Package](https://github.com/orgs/buildwithgrove/packages/container/package/path-auth-data-server)

## Envoy Gateway Docs

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

| Variable                      | Required | Type   | Description                                                                                                                          | Example          | Default Value |
| ----------------------------- | -------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------ | ---------------- | ------------- |
| GRPC_HOST_PORT                | ✅        | string | The host and port for the remote gRPC server connection that provides the GatewayEndpoint data. Must adhere to a `host:port` format. | guard-pads:10002 | -             |
| GRPC_USE_INSECURE_CREDENTIALS | ❌        | bool   | Whether to use insecure credentials for the gRPC connection. Must be `true` if the remote gRPC server is not TLS-enabled.            | `true`           | `false`       |
| PORT                          | ❌        | int    | The port to run the external auth server on.                                                                                         | 10001            | 10001         |
