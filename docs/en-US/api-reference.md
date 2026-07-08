# API Reference

[中文](../zh-CN/api-reference.md)

CyberStrikeAI exposes built-in OpenAPI docs:

```text
/api-docs
GET /api/openapi/spec
```

The OpenAPI spec is protected to avoid exposing the API surface to unauthenticated users.

## Authentication

Login:

```http
POST /api/auth/login
Content-Type: application/json

{"password":"your-password"}
```

The auth middleware accepts token from:

1. `Authorization: Bearer <token>`
2. `Authorization: <token>`
3. `?token=<token>`
4. `auth_token` cookie

Prefer `Authorization: Bearer` for scripts. Query tokens can leak through logs.

## Agent APIs

Single-agent:

- `POST /api/eino-agent`
- `POST /api/eino-agent/stream`

Multi-agent:

- `POST /api/multi-agent`
- `POST /api/multi-agent/stream`

`orchestration` may be `deep`, `plan_execute`, or `supervisor`.

## SSE Notes

Streaming endpoints are long-lived. Clients should:

- handle `error` events;
- wait for `done`;
- avoid blindly replaying destructive requests;
- disable proxy buffering;
- pass `conversationId` when continuing a conversation.

## Stability Tiers

| API type | Stability | Recommendation |
| --- | --- | --- |
| `/api/auth/*` | high | safe to integrate |
| `/api/eino-agent*` | high | preferred chat entry |
| `/api/openapi/spec` | high | client generation |
| `/api/config*` | medium | admin automation only |
| `/api/c2/*`, `/api/webshell/*` | medium | high-risk, restrict access |
| frontend private calls | low | avoid plugin dependency |

## Common Areas

- Conversations: `/api/conversations`
- Projects/facts: `/api/projects`
- Vulnerabilities: `/api/vulnerabilities`
- Knowledge: `/api/knowledge/*`
- Roles: `/api/roles`
- Skills: `/api/skills`
- External MCP: `/api/external-mcp`
- Monitoring: `/api/monitor`
- Audit: `/api/audit`
- C2: `/api/c2`
- WebShell: `/api/webshell`

## Curl Example

```bash
curl -k https://127.0.0.1:8080/api/conversations \
  -H "Authorization: Bearer <token>"
```

```bash
curl -k https://127.0.0.1:8080/api/eino-agent \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"message":"Run authorized basic recon against 127.0.0.1; avoid high-risk actions."}'
```

## Source Anchors

- Routes: `internal/app/app.go`
- Auth middleware: `internal/security/auth_middleware.go`
- OpenAPI: `internal/handler/openapi.go`
- Single-agent: `internal/handler/eino_single_agent.go`
- Multi-agent: `internal/handler/multi_agent.go`
