<div align="center">
<h1>🫛 PEAS<br/>PATH External Auth Server</h1>
<img src="https://storage.googleapis.com/grove-brand-assets/Presskit/Logo%20Joined-2.png" alt="Grove logo" width="500"/>

</div>
<br/>

## Introduction

**PEAS** (PATH External Auth Server) is an external authorization server that can be used
to authorize requests to the PATH Gateway. It is part of the GUARD authorization system for PATH and runs in the GUARD cluster.

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
