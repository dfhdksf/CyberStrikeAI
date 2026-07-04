# 系统架构总览

## 架构分层

```
┌─────────────────────────────────────────────────────────────────┐
│                        Web 前端 (SPA)                            │
│            Vanilla JS + Client-side Router + XTerm.js            │
├─────────────────────────────────────────────────────────────────┤
│                       HTTP API 层 (Gin)                          │
│           RESTful API + WebSocket + SSE 实时流                    │
├──────────────┬──────────────────────────────────┬───────────────┤
│  认证/安全    │         业务逻辑层                 │   审计/监控    │
│  AuthManager │  Agent │ MultiAgent │ Knowledge   │  Audit+Monitor│
│  RateLimit   │  C2    │ Project    │ BatchTask   │  AttackChain  │
├──────────────┴──────────────────────────────────┴───────────────┤
│                     MCP 协议层 (JSON-RPC 2.0)                    │
│         内置 MCP Server  ←→  外部 MCP Client Manager             │
├─────────────────────────────────────────────────────────────────┤
│                       工具执行层                                  │
│    Security Tools (YAML) │ Shell Sessions │ WebShell │ Plugins   │
├─────────────────────────────────────────────────────────────────┤
│                       持久化层                                    │
│           SQLite (WAL) │ 文件系统 │ 知识库向量索引                  │
└─────────────────────────────────────────────────────────────────┘
```

## 核心设计原则

### 1. Tool-First（工具优先）

所有能力均以 MCP Tool 形式暴露。AI Agent 不直接操作系统，而是通过标准化的 MCP 工具调用完成一切任务——安全扫描、漏洞管理、知识检索、C2 操作等。这保证了：
- 统一的工具发现和描述机制
- 完整的执行审计链路
- Human-in-the-Loop 拦截点

### 2. LLM Provider Agnostic（模型无关）

通过 OpenAI 兼容协议 + Claude Messages API 桥接，支持任意 LLM 提供商：
- OpenAI / Azure OpenAI
- Anthropic Claude（专用桥接层转换 Messages API）
- DeepSeek、Qwen 等国产模型
- 任意 OpenAI 兼容 API

### 3. 多智能体协作

基于 CloudWeGo Eino ADK 实现三种编排模式：
- **Deep（任务委派）**: 主 Agent 分解任务，委派给专业子 Agent
- **Plan-Execute（计划执行）**: 规划器生成步骤，执行器逐步完成，规划器动态调整
- **Supervisor（监督者）**: 监督者根据上下文转发给合适的子 Agent

### 4. 渐进式工具披露

Agent 不会一次性加载全部 100+ 工具。通过 Eino 的 Tool Search Middleware，Agent 根据当前任务动态搜索和加载相关工具，减少 Token 消耗。

## 数据流

### 单次对话流程

```
用户输入 → API Handler → Agent Loop
                              ↓
                    LLM 决策 (function calling)
                              ↓
                    MCP Tool 调用 ← HITL 审核
                              ↓
                    工具执行 (Command Executor)
                              ↓
                    结果返回 → LLM 继续推理
                              ↓
                    最终回复 → SSE Stream → 前端
```

### 多智能体协作流程

```
用户输入 → Orchestrator Agent
               ↓
    ┌──────────┼──────────┐
    ↓          ↓          ↓
  Recon    Penetration  Reporting
  Agent      Agent       Agent
    ↓          ↓          ↓
  工具调用   工具调用    汇总输出
    └──────────┼──────────┘
               ↓
         最终报告 → 用户
```

## 技术栈

| 层级 | 技术选型 | 说明 |
|------|----------|------|
| HTTP 框架 | Gin v1.9 | 路由、中间件、静态文件 |
| 多智能体 | CloudWeGo Eino v0.8 | ADK 编排、中间件、回调 |
| MCP 协议 | go-sdk v1.2 | 外部 MCP 服务器连接 |
| 数据库 | SQLite + go-sqlite3 | WAL 模式，自动 checkpoint |
| LLM 客户端 | Eino OpenAI ext | OpenAI 兼容 + Claude 桥接 |
| 终端 | creack/pty | PTY shell 会话 |
| WebSocket | gorilla/websocket | C2、终端实时通信 |
| Token 计数 | tiktoken-go | 上下文窗口管理 |
| 定时任务 | robfig/cron | 批量任务调度 |
| 可观测性 | OpenTelemetry | 分布式追踪 |
| 前端 | Vanilla JS SPA | 无框架，客户端路由 |
| 图可视化 | Cytoscape.js | 攻击链图谱 |
| 终端模拟 | XTerm.js | Web Terminal |
