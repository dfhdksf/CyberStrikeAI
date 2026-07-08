# API 参考

CyberStrikeAI 内置 OpenAPI 规格和 API 文档页面。启动服务后访问：

```text
/api-docs
```

OpenAPI JSON：

```text
GET /api/openapi/spec
```

`/api/openapi/spec` 需要登录认证，避免未授权用户直接枚举接口结构。

## 认证

登录：

```http
POST /api/auth/login
Content-Type: application/json

{"password":"your-password"}
```

认证成功后，前端通常使用 Cookie 会话。外部客户端也可参考 OpenAPI 中的 Bearer Token 描述，按实际返回字段接入。

常用认证接口：

- `POST /api/auth/login`
- `POST /api/auth/logout`
- `POST /api/auth/change-password`
- `GET /api/auth/validate`

## 对话与 Agent

单代理：

- `POST /api/eino-agent`
- `POST /api/eino-agent/stream`

多代理：

- `POST /api/multi-agent`
- `POST /api/multi-agent/stream`

多代理请求体通过 `orchestration` 指定：

- `deep`
- `plan_execute`
- `supervisor`

对话管理：

- `POST /api/conversations`
- `GET /api/conversations`
- `GET /api/conversations/:id`
- `PUT /api/conversations/:id`
- `DELETE /api/conversations/:id`
- `POST /api/conversations/:id/delete-turn`
- `GET /api/messages/:id/process-details`

## 项目、漏洞、攻击链

项目：

- `GET /api/projects`
- `POST /api/projects`
- `GET /api/projects/:id`
- `PUT /api/projects/:id`
- `DELETE /api/projects/:id`
- `GET /api/projects/:id/facts`
- `POST /api/projects/:id/facts`
- `GET /api/projects/:id/fact-graph`

漏洞：

- `GET /api/vulnerabilities`
- `POST /api/vulnerabilities`
- `GET /api/vulnerabilities/:id`
- `PUT /api/vulnerabilities/:id`
- `DELETE /api/vulnerabilities/:id`
- `GET /api/vulnerabilities/export`

攻击链：

- `GET /api/attack-chain/:conversationId`
- `POST /api/attack-chain/:conversationId/regenerate`

## 工具、MCP、配置

配置：

- `GET /api/config`
- `PUT /api/config`
- `POST /api/config/apply`
- `GET /api/config/tools`
- `GET /api/config/tools/:name/schema`
- `POST /api/config/test-openai`
- `POST /api/config/test-vision`
- `POST /api/config/list-models`

MCP：

- `POST /api/mcp`
- `GET /api/external-mcp`
- `PUT /api/external-mcp/:name`
- `POST /api/external-mcp/:name/start`
- `POST /api/external-mcp/:name/stop`
- `DELETE /api/external-mcp/:name`

## 知识库、Skills、角色、Agent

知识库：

- `GET /api/knowledge/categories`
- `GET /api/knowledge/items`
- `POST /api/knowledge/scan`
- `POST /api/knowledge/index`
- `POST /api/knowledge/search`

角色：

- `GET /api/roles`
- `POST /api/roles`
- `GET /api/roles/:name`
- `PUT /api/roles/:name`
- `DELETE /api/roles/:name`

Skills：

- `GET /api/skills`
- `POST /api/skills`
- `GET /api/skills/:name`
- `PUT /api/skills/:name`
- `DELETE /api/skills/:name`
- `GET /api/skills/:name/files`
- `GET /api/skills/:name/file`
- `PUT /api/skills/:name/file`

Markdown 子代理：

- `GET /api/multi-agent/markdown-agents`
- `POST /api/multi-agent/markdown-agents`
- `GET /api/multi-agent/markdown-agents/:filename`
- `PUT /api/multi-agent/markdown-agents/:filename`
- `DELETE /api/multi-agent/markdown-agents/:filename`

## 高风险能力

WebShell：

- `GET /api/webshell/connections`
- `POST /api/webshell/connections`
- `POST /api/webshell/exec`
- `POST /api/webshell/file`

C2：

- `GET /api/c2/listeners`
- `POST /api/c2/listeners`
- `GET /api/c2/sessions`
- `POST /api/c2/tasks`
- `POST /api/c2/payloads/build`

终端：

- `POST /api/terminal/run`
- `POST /api/terminal/run/stream`
- `GET /api/terminal/ws`

这些接口应只开放给可信管理员，并配合 HTTPS、强密码、网络隔离和审计。

## 调用建议

- 优先使用 `/api-docs` 查看完整参数和响应结构。
- 流式接口使用 SSE，反向代理需关闭缓冲。
- 所有修改类接口都应处理 401、403、404、409、500。
- 外部集成建议创建最小权限网络路径，不要把 Web 管理面直接暴露到公网。

## 认证细节

认证中间件会按顺序提取 token：

1. `Authorization: Bearer <token>`
2. `Authorization: <token>`
3. 查询参数 `?token=<token>`
4. Cookie `auth_token`

这意味着外部脚本最稳妥的方式是使用 `Authorization: Bearer`。查询参数虽然支持，但容易进入代理日志，不建议生产使用。

## SSE 客户端注意事项

`/api/eino-agent/stream` 和 `/api/multi-agent/stream` 是长连接。客户端应处理：

- 网络中断后不要盲目重放破坏性请求。
- 收到 `error` 事件后读取错误正文。
- 收到 `done` 才视为本轮结束。
- 代理层不能缓冲。
- 请求体中的 `conversationId` 决定是否接续已有对话。

## API 稳定性分层

| API 类型 | 稳定性 | 集成建议 |
| --- | --- | --- |
| `/api/auth/*` | 高 | 可直接集成 |
| `/api/eino-agent*` | 高 | 推荐外部对话入口 |
| `/api/openapi/spec` | 高 | 用于生成客户端 |
| `/api/config*` | 中 | 管理工具使用，谨慎自动化 |
| `/api/c2/*`、`/api/webshell/*` | 中 | 高风险，必须加权限边界 |
| 前端私有调用细节 | 低 | 不建议插件依赖 |

## Curl 示例

登录并提取 token 的返回字段可能随实现调整，建议先看 `/api-docs`。如果已有 token：

```bash
curl -k https://127.0.0.1:8080/api/conversations \
  -H "Authorization: Bearer <token>"
```

发送非流式单代理请求：

```bash
curl -k https://127.0.0.1:8080/api/eino-agent \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"message":"对 127.0.0.1 做授权的基础信息收集，先不要执行高风险操作"}'
```

## 源码锚点

- 路由：`internal/app/app.go`
- 认证：`internal/security/auth_middleware.go`
- OpenAPI：`internal/handler/openapi.go`
- 单代理：`internal/handler/eino_single_agent.go`
- 多代理：`internal/handler/multi_agent.go`
