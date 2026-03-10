# Trello Webhook Local Gateway

A local forwarding gateway for Trello webhooks, implemented with Go + Gin.

## Background

In this setup, two systems need to be connected:

- Trello webhook: sends board events to your `callbackURL`
- OpenClaw webhook (`/hooks/agent`): requires `Authorization: Bearer <token>` on every request

The problem is that Trello webhooks do not support custom HTTP headers, so Trello cannot directly send a Bearer token to OpenClaw.

That is why this gateway exists. It handles three responsibilities:

1. Receive Trello webhook requests
2. Verify that requests are genuinely from Trello (signature verification)
3. Transform and forward the payload to OpenClaw with an injected Bearer token

This project was built specifically to solve that integration gap.

## OpenClaw Webhook Behavior (Why This Gateway Exists)

Based on OpenClaw's public webhook documentation and gateway config reference (`docs/automation/webhook.md` and `docs/gateway/configuration-reference.md` in `openclaw/openclaw`):

- OpenClaw exposes hook endpoints under `/hooks` (default path), including:
  - `POST /hooks/wake`
  - `POST /hooks/agent`
  - `POST /hooks/<name>` (mapping-based)
- Hook requests require token authentication, typically:
  - `Authorization: Bearer <token>` (recommended)
  - `x-openclaw-token: <token>`
- For `/hooks/agent`, the JSON payload requires a `message` field; optional fields include `name`, `agentId`, `sessionKey`, `deliver`, `channel`, `to`, and others.

Why this matters for Trello integration:

- Trello webhooks cannot set custom authorization headers.
- OpenClaw hooks require authenticated requests and a `message` payload.
- Therefore this gateway verifies Trello signatures, injects Bearer auth, and transforms Trello events into an OpenClaw-compatible `/hooks/agent` request.

## Core Design

- Framework: `github.com/gin-gonic/gin`
- Architecture: `main` is separated from business logic
  - Entry point: `cmd/trello-openclaw-webhook-gateway/main.go`
  - Business logic: `internal/app/*`
- Signature verification: `HMAC-SHA1(secret, raw_body + callbackURL)`, then Base64-compare with `X-Trello-Webhook`
- Forward request timeout: 30 seconds
- Graceful shutdown: supports `SIGINT/SIGTERM`
- Logging: stdout with timestamps

## Request Flow

1. Trello sends a `HEAD` request when creating a webhook:
- The gateway returns `200` so Trello's callback URL validation succeeds

2. Trello sends `POST` requests for events:
- Read the raw body
- Read `X-Trello-Webhook`
- Compute signature using `TRELLO_API_SECRET` + `CALLBACK_URL` and verify it
- Return `403` if verification fails
- If verification passes, forward a slimmed JSON payload

3. Forward to OpenClaw:
- URL: `FORWARD_URL`
- Header: `Authorization: Bearer <FORWARD_TOKEN>`
- Body: a slimmed Trello JSON that keeps only required fields:
  - `action.type`
  - `action.data.card.id`
  - `action.data.listBefore.name`
  - `action.data.listBefore.id`
  - `action.data.listAfter.name`
  - `action.data.listAfter.id`

Security note:

- Field slimming is intentional to reduce prompt injection risk.
- The gateway drops free-form or unnecessary fields (for example comments, descriptions, and other arbitrary text) and only forwards minimal routing fields.
- This limits untrusted user-generated content from entering the downstream mapping engine.

4. Propagate OpenClaw's response status code back to Trello.

## Forwarded Payload Format

After signature verification, the gateway forwards a compact payload.

Example:

```json
{
  "action": {
    "type": "updateCard",
    "data": {
      "card": { "id": "69ae188a" },
      "listBefore": { "name": "Backlog", "id": "x" },
      "listAfter": { "name": "Analyze", "id": "y" }
    }
  }
}
```

## Message Generation Rules

- If both `listBefore` and `listAfter` exist:
  - `Trello: card "{card.name}" moved from "{listBefore.name}" to "{listAfter.name}" (by {memberCreator.fullName})`
- If `action.type == commentCard`:
  - `Trello: {memberCreator.fullName} commented on card "{card.name}": {action.data.text}`
- For other action types:
  - `Trello: {action.type} on card "{card.name}" in board "{board.name}" by {memberCreator.fullName}`
  - Append compact raw JSON

## Configuration

Both CLI flags and environment variables are supported (CLI flags take precedence):

- `--listen` / `LISTEN_ADDR`
  - Listen address, default `:18790`
- `--trello-api-secret` / `TRELLO_API_SECRET` (required)
  - Trello API Secret used for signature verification
- `--callback-url` / `CALLBACK_URL` (required)
  - The callback URL used when creating the Trello webhook (must match Trello config exactly)
- `--forward-url` / `FORWARD_URL` (required)
  - OpenClaw webhook endpoint (for example: `http://127.0.0.1:18789/hooks/agent`)
- `--forward-token` / `FORWARD_TOKEN` (required)
  - OpenClaw Bearer token

## Quick Start

### 1. Build

```bash
go build -o trello-gateway ./cmd/trello-openclaw-webhook-gateway
```

### 2. Run with Environment Variables

```bash
export LISTEN_ADDR=":18790"
export TRELLO_API_SECRET="your_trello_api_secret"
export CALLBACK_URL="https://your-public-domain/"
export FORWARD_URL="http://127.0.0.1:18789/hooks/agent"
export FORWARD_TOKEN="your_openclaw_token"

./trello-gateway
```

### 3. Run with CLI Flags

```bash
./trello-gateway \
  --listen ":18790" \
  --trello-api-secret "your_trello_api_secret" \
  --callback-url "https://your-public-domain/" \
  --forward-url "http://127.0.0.1:18789/hooks/agent" \
  --forward-token "your_openclaw_token"
```

## Development and Testing

```bash
go test ./...
go build ./...
```

The current implementation includes test coverage for configuration, signature verification, message building, forwarding, and request routing.
