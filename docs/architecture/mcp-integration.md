# MCP 协议集成

## 概述

MCP（Model Context Protocol）是 Anthropic 提出的开放协议，用于标准化 AI 模型与外部工具/数据源的交互。CyberStrikeAI 既实现了 MCP Server（暴露内置能力），也作为 MCP Client 连接外部 MCP 服务器。

---

## 内置 MCP Server (`internal/mcp/server.go`)

### 协议实现

- **传输层**: HTTP（JSON-RPC 2.0 over HTTP POST）
- **端点**: `/mcp`（与主 HTTP 服务器共享端口）
- **认证**: 自定义 Header（`X-MCP-Auth` + 64 位 hex token，首次运行自动生成）

### 请求处理流程

```
HTTP POST /mcp
    ↓
认证检查（Header token）
    ↓
JSON-RPC 解析
    ↓
路由到对应 handler:
├── initialize        → 返回服务器能力
├── tools/list        → 返回所有已注册工具
├── tools/call        → 执行工具并返回结果
├── prompts/list      → 返回提示词模板
├── prompts/get       → 获取具体提示词
├── resources/list    → 返回资源列表
└── resources/read    → 读取资源内容
```

### 工具注册

工具来源：
1. **安全工具** (YAML): 从 `tools/` 目录加载，转换为 MCP Tool 定义
2. **内置功能工具**: 漏洞管理、项目黑板、知识库、WebShell、批量任务、C2
3. **Eino ADK 工具**: 通过 `einomcp` 桥接层注册

### 内置工具分类

```
漏洞管理:
  - record_vulnerability    记录漏洞
  - list_vulnerabilities    列出漏洞
  - get_vulnerability       获取漏洞详情

项目黑板:
  - upsert_project_fact     更新项目事实
  - get_project_fact        获取事实
  - list_project_facts      列出所有事实
  - search_project_facts    搜索事实
  - deprecate_project_fact  废弃事实
  - restore_project_fact    恢复事实

知识库:
  - list_risk_types         列出风险类型
  - search_knowledge_base   搜索知识库

视觉分析:
  - analyze_image           分析图像

WebShell:
  - webshell_exec           执行命令
  - webshell_file_list      列出文件
  - webshell_file_read      读取文件
  - webshell_file_write     写入文件
  - webshell_manage         管理连接

批量任务:
  - batch_task_list/get/create/start/rerun/pause/delete/...

C2 (命令与控制):
  - c2_listener/session/task/task_manage/payload/event/profile/file
```

### 执行监控

每次工具调用都会生成执行记录：
- 唯一执行 ID
- 工具名、参数、调用时间
- 执行状态（running / completed / failed / cancelled）
- 输出内容、耗时
- 关联的对话 ID
- 通过 SSE 实时推送状态变更

---

## 外部 MCP Manager (`internal/mcp/external_manager.go`)

### 支持的传输方式

| 传输方式 | 说明 | 配置 |
|----------|------|------|
| stdio | 启动子进程，通过 stdin/stdout 通信 | command, args, env |
| sse | Server-Sent Events 长连接 | url, headers |
| streamable-http | Streamable HTTP 传输 | url, headers |

### 连接管理

```go
type ExternalMCPConfig struct {
    Name      string            // 服务器标识名
    Transport string            // stdio / sse / streamable-http
    Command   string            // stdio: 可执行文件
    Args      []string          // stdio: 命令参数
    Env       map[string]string // stdio: 环境变量
    URL       string            // sse/http: 服务器地址
    Headers   map[string]string // sse/http: 自定义 Header
    Enabled   bool              // 是否启用
    Tools     []ToolConfig      // 工具级别的启用/禁用
}
```

### 工具名映射

外部 MCP 工具名格式为 `server_name::tool_name`，但 OpenAI function-calling 不支持 `::`，因此自动映射为 `server_name__tool_name`。

### 缓存与重连

- 工具列表缓存 60 秒 TTL
- 断线后指数退避重连
- 连接状态可通过 API 查询

---

## Eino ↔ MCP 桥接 (`internal/einomcp/`)

Eino ADK 有自己的工具接口定义，`einomcp` 包负责在两者之间转换：

```
Eino Tool Definition ←→ MCP Tool Schema
Eino Tool Call       ←→ MCP tools/call
Eino Tool Result     ←→ MCP Call Result
```

这使得多智能体编排可以无缝使用所有 MCP 注册的工具。

---

## MCP Stdio 模式 (`cmd/mcp-stdio/`)

独立的 MCP stdio 传输服务器，允许外部 IDE（如 VSCode、Cursor）直接连接 CyberStrikeAI 的工具集：

```bash
# 在 IDE 的 MCP 配置中
{
  "command": "./cyberstrike-ai-mcp",
  "args": ["-config", "config.yaml"]
}
```

通过 stdin/stdout 提供完整的 MCP 协议服务。
