# Tesla Remote MCP

A single-account Tesla MCP service. Connect the Tesla account once through the
browser, then use MCP tools with that account. Tokens stay in memory for the
lifetime of the process. Restarting or deploying the service requires connecting
the account again.

## Architecture

- `internal/auth`: browser authorization, callback validation, token exchange,
  connection status, and access token renewal.
- `internal/store`: storage interface and the single-account `InMemoryStore`.
- `internal/tesla`: Fleet API operations using tokens supplied by the auth service.
- `internal/mcp`: MCP tools delegating to Tesla operations.
- `internal/server`: constructs and connects these components and HTTP routes.
- `cmd`: starts the HTTP server.

OAuth and MCP share one auth service and token store. Before each Fleet API call,
the auth service returns a valid access token or renews it through the existing
OAuth library. It saves the rotated refresh token before returning. Concurrent
requests share one renewal. A revoked or expired refresh token requires reconnecting
through the browser; temporary failures can be retried on a later tool call.

Tesla API requests and token exchanges use the caller's context and a bounded HTTP
timeout. Once Tesla issues new credentials, saving them gets a separate bounded
context so a caller disconnecting does not discard a rotated refresh token.

## Environment

| Variable | Purpose |
| --- | --- |
| `TESLA_CLIENT_ID` | Tesla application client ID. |
| `TESLA_CLIENT_SECRET` | Tesla application client secret. |
| `TESLA_AUDIENCE` | Fleet API base URL for the account's region. For Europe: `https://fleet-api.prd.eu.vn.cloud.tesla.com`. |
| `DOMAIN_SERVICE` | Public origin without a trailing slash, e.g. `https://your-service.onrender.com`. Defaults to `http://localhost:8080`. |
| `PORT` | HTTP listen port. Defaults to `8080`; Render supplies this variable. |
| `APP_ENV` | Set to `production` when deploying behind a trusted reverse proxy such as Render. Unset by default. |

Register `DOMAIN_SERVICE` plus `/auth/callback` as the redirect URI in the Tesla
application. For local use with a different port, set `DOMAIN_SERVICE` accordingly.
The OAuth state cookie uses `Secure` when this configured callback uses HTTPS,
including when TLS terminates at Render's proxy.

Set `APP_ENV=production` in Render's environment settings. In production, the MCP
handler disables the SDK's localhost Host check because a trusted reverse proxy
can forward public requests over a loopback connection. Other environments retain
the SDK's default protection.

## Use

1. Set the environment variables and run `go run ./cmd`.
2. Open the service's `/` page and choose **Poveži Teslu**. Complete Tesla consent.
3. Connect an MCP client using Streamable HTTP at `/mcp`.
4. Call `list_vehicles` with `{}`. `ping` works even before connecting Tesla.

The `/` page shows connection status and offers an explicit reconnection link.
It does not start a new login on every visit. Tokens are never returned by MCP
tools or displayed on the page.

This is one shared Tesla account. The MCP endpoint has no client authentication,
so any client with network access to it can use that account's tools. Tesla OAuth
authorizes the service to access Tesla; it does not authenticate MCP clients.

## Verify

```sh
go test ./...
go vet ./...
```

Tests simulate Tesla responses without contacting a real account. Protocol and
token behavior follow the [Go MCP SDK](https://github.com/modelcontextprotocol/go-sdk)
and [Tesla token documentation](https://developer.tesla.com/docs/fleet-api/authentication/third-party-tokens).
