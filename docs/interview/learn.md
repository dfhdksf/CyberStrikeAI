# MCP 工具 vs 普通工具：设计决策与实现解析

## Q1: 为什么用 MCP 工具而不是普通工具？二者有什么区别？

### 普通工具调用方式

```go
// Agent 直接持有工具映射，调用时直接执行
type Agent struct {
    tools map[string]func(args map[string]any) (string, error)
}

func (a *Agent) handleToolCall(name string, args map[string]any) string {
    handler := a.tools[name]
    result, _ := handler(args)
    return result
}
```

没有中间层，Agent 直接调用执行器。简单直接，但缺少可观测性和扩展性。

### MCP 工具调用方式

Agent 不直接调用工具，所有调用经过 MCP Server 作为中间层。这里的 MCP Server 不是远程网络服务，而是一个**进程内的 Go struct**（本质是 `map[string]ToolHandler` + 监控/审计逻辑）。

```
Agent ──tool_call──→ MCP Server.CallTool()
                          │
                          ├── 记录执行（监控）
                          ├── 检查 HITL（审核）
                          ├── 派发到 handler
                          │      └── Executor.Run("nmap", args)
                          └── 更新执行状态（完成/失败/取消）
```

### 核心区别

| 能力 | 普通工具 | MCP 工具 |
|------|----------|----------|
| 工具注册 | Agent 内部维护 | 统一注册表，多个消费方共享 |
| 执行监控 | 无 | 每次调用自动生成带 UUID 的执行记录 |
| 取消执行 | 手动实现 | 通过 execution ID 从 UI 取消 |
| 审计追踪 | 无 | 时间、参数、状态、耗时全记录到 SQLite |
| 外部工具接入 | 每种协议写一套 | stdio/SSE/HTTP 统一协议 |
| IDE 集成 | 不可能 | MCP stdio 模式直接暴露给 Cursor/VSCode |
| 多 Agent 共享 | 每个 Agent 各自注册 | 单 Agent 和 Eino 多 Agent 共用同一注册表 |
| HITL 拦截 | 在每个调用点手写 | 在 CallTool 统一拦截 |

### 为什么选 MCP

1. **统一入口**: 100+ 工具、两套 Agent 引擎、外部 MCP 服务器，都走同一个注册-调用路径
2. **可观测性**: 每次工具调用自动有监控记录，UI 实时展示，支持取消
3. **可扩展性**: 接入新工具只需写 YAML 或连外部 MCP Server，不动 Agent 代码
4. **生态兼容**: 遵循 Anthropic 开放协议标准，能和任何支持 MCP 的工具/IDE 互通

### 协议标准化的价值

"普通工具"是**私有接口**——Agent 和工具之间用自定义数据结构通信，换框架就要重写。

MCP 是**开放协议**——工具定义遵循 JSON Schema，调用遵循 JSON-RPC 2.0：
- 内部工具注册一次，单 Agent 和多 Agent 都能用
- 外部工具通过 stdio/SSE 连上就能用，不改 Agent 代码
- 反向暴露——自己的工具也能通过 `cmd/mcp-stdio/` 给外部 IDE 使用

---

## Q2: 项目如何封装工具为 MCP 并加入各种能力？

### 完整调用链

```
YAML 文件 (tools/nmap.yaml)
    ↓ config.LoadToolsFromDir()
Go 结构体 (config.ToolConfig)
    ↓ executor.RegisterTools(mcpServer)
MCP Tool 定义 + Handler 闭包
    ↓ mcpServer.RegisterTool(tool, handler)
存入 MCP Server 的 tools map
    ↓ Agent 请求工具列表
转为 OpenAI function-calling 格式发给 LLM
    ↓ LLM 返回 tool_calls
agent.executeToolViaMCP()
    ↓
mcpServer.CallTool() ← 在此注入所有能力
    ↓
handler 闭包 → executor.ExecuteTool()
    ↓
exec.CommandContext("nmap", args...) → OS 进程
```

### 第一步：YAML 加载

**`internal/config/config.go:1263`**

```go
func LoadToolsFromDir(dir string) ([]ToolConfig, error) {
    entries, _ := os.ReadDir(dir)
    for _, entry := range entries {
        tool, _ := LoadToolFromFile(filePath)  // yaml.Unmarshal → ToolConfig
        tools = append(tools, *tool)
    }
    return tools, nil
}
```

YAML 定义（`tools/nmap.yaml`）反序列化为：

```go
type ToolConfig struct {
    Name             string            // "nmap"
    Command          string            // "nmap"（实际可执行文件）
    Args             []string          // ["-sT", "-sV", "-sC"]（默认参数）
    Description      string            // 给 LLM 看的详细描述
    ShortDescription string            // 短描述（省 token）
    Parameters       []ParameterConfig // 参数定义（映射为 JSON Schema）
    Enabled          bool
    AllowedExitCodes []int             // 某些工具成功时返回非零码
}
```

### 第二步：应用启动时组装

**`internal/app/app.go:118-126`**

```go
mcpServer := mcp.NewServerWithStorage(log.Logger, db)          // 创建 MCP Server + SQLite
executor := security.NewExecutor(&cfg.Security, mcpServer, log.Logger)  // 创建执行器
executor.RegisterTools(mcpServer)                               // 封装并注册
```

### 第三步：RegisterTools — 封装核心

**`internal/security/executor.go:235-301`**

做三件事：

#### 3a. YAML Parameters → JSON Schema

```go
tool := mcp.Tool{
    Name:        toolConfigCopy.Name,
    Description: toolConfigCopy.Description,
    InputSchema: e.buildInputSchema(&toolConfigCopy),  // 关键转换
}
```

`buildInputSchema`（`executor.go:1274`）将 YAML 参数定义转为 OpenAI function-calling 要求的 JSON Schema：

```go
func (e *Executor) buildInputSchema(toolConfig *config.ToolConfig) map[string]interface{} {
    for _, param := range toolConfig.Parameters {
        prop := map[string]interface{}{
            "type":        e.convertToOpenAIType(param.Type),
            "description": param.Description,
        }
        if len(param.Options) > 0 { prop["enum"] = param.Options }
        if param.Required { required = append(required, param.Name) }
        properties[param.Name] = prop
    }
    return map[string]interface{}{"type": "object", "properties": properties, "required": required}
}
```

#### 3b. 创建 Handler 闭包

```go
handler := func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
    return e.ExecuteTool(ctx, toolName, args)
}
```

闭包捕获 `toolName`，调用时转发到 Executor 的实际执行逻辑。

#### 3c. 注册到 MCP Server

```go
mcpServer.RegisterTool(tool, handler)
```

**`internal/mcp/server.go:132-146`**：

```go
func (s *Server) RegisterTool(tool Tool, handler ToolHandler) {
    s.tools[tool.Name] = handler     // 存处理函数
    s.toolDefs[tool.Name] = tool     // 存定义（给 LLM）
    // 自动创建 MCP Resource 文档
    s.resources["tool://"+tool.Name] = &Resource{...}
}
```

### 第四步：CallTool — 能力注入层

**`internal/mcp/server.go:811-922`**

这是所有 MCP 层能力集中注入的位置：

```go
func (s *Server) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (*ToolResult, string, error) {
    // ══ 能力 1: 统一注册表查找 ══
    handler, exists := s.tools[toolName]

    // ══ 能力 2: 创建执行记录 ══
    executionID := uuid.New().String()
    execution := &ToolExecution{
        ID: executionID, ToolName: toolName,
        Arguments: args, Status: "running", StartTime: time.Now(),
    }
    s.executions[executionID] = execution

    // ══ 能力 3: 持久化到 SQLite ══
    s.storage.SaveToolExecution(execution)

    // ══ 能力 4: 注册 cancel func（UI 可远程取消） ══
    execCtx, runCancel := context.WithCancel(ctx)
    s.registerRunningCancel(executionID, runCancel)

    // ══ 能力 5: 执行工具 ══
    result, err := handler(execCtx, args)

    // ══ 能力 6: 处理用户终止说明 ══
    s.applyAbortUserNoteToCancelledToolResult(executionID, &result, &err)

    // ══ 能力 7: 更新状态 + 统计 ══
    execution.Status = "completed" / "failed" / "cancelled"
    execution.Duration = now.Sub(execution.StartTime)
    s.storage.SaveToolExecution(execution)
    s.updateStats(toolName, failed)

    return finalResult, executionID, nil
}
```

### 第五步：实际命令执行

**`internal/security/executor.go:75-231`**

```go
func (e *Executor) ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (*mcp.ToolResult, error) {
    toolConfig := e.toolIndex[toolName]                     // O(1) 查找
    cmdArgs := e.buildCommandArgs(toolName, toolConfig, args)  // YAML 参数 → CLI flags
    cmd := exec.CommandContext(ctx, toolConfig.Command, cmdArgs...)

    // 流式回调（SSE 实时推给前端）
    if cb, ok := ctx.Value(ToolOutputCallbackCtxKey).(ToolOutputCallback); ok {
        output, err = streamCommandOutput(ctx, cmd, cb, timeout)
    } else {
        output, err = combinedOutputCancellable(ctx, cmd)
    }

    // 需要 TTY 的工具自动 PTY 重试
    if shouldRetryWithPTY(output) {
        output, err = runCommandWithPTY(ctx, cmd2, cb)
    }

    return &mcp.ToolResult{Content: [{Type: "text", Text: output}]}, nil
}
```

### 第六步：多 Agent 桥接（Eino ADK）

多 Agent 模式通过 `einomcp` 桥接层复用同一条 MCP 路径：

**`internal/einomcp/mcp_tools.go:29`**

```go
func ToolsFromDefinitions(ag *agent.Agent, ...) ([]tool.BaseTool, error) {
    for _, d := range defs {
        out = append(out, &mcpBridgeTool{name: d.Function.Name, agent: ag, ...})
    }
}
```

**`mcp_tools.go:102`** — Eino 调用工具时：

```go
func (m *mcpBridgeTool) InvokableRun(ctx, argumentsInJSON, opts) (string, error) {
    return runMCPToolInvocation(ctx, m.agent, m.holder, m.name, argumentsInJSON, ...)
}
```

**`mcp_tools.go:174`** — 最终汇入同一路径：

```go
res, err := ag.ExecuteMCPToolForConversation(ctx, holder.Get(), toolName, args)
// → executeToolViaMCP() → mcpServer.CallTool() → handler → executor
```

### 取消能力：从 UI 到进程级 Kill

```
前端"终止"按钮 → POST /api/monitor/execution/:id/cancel
    ↓
mcpServer.CancelToolExecutionWithNote(id, note)   // server.go:1150
    ↓
runningCancels[id]() → context cancel 触发
    ↓
exec.CommandContext 收到 ctx.Done() → kill 进程组
    ↓
CallTool 检测 context.Canceled → 状态标记 "cancelled"
    ↓
Agent 收到终止消息，继续推理（不中断整条任务链）
```

---

## 总结

### MCP 层注入的 7 种能力

| # | 能力 | 代码位置 | 没有 MCP 会怎样 |
|---|------|----------|----------------|
| 1 | 统一注册表 | `server.go:132` RegisterTool | 每个 Agent 各自维护 map |
| 2 | 执行记录 | `server.go:820` 创建 ToolExecution | 无审计追踪 |
| 3 | 持久化 | `server.go:836` SaveToolExecution | 重启丢失历史 |
| 4 | 取消支持 | `server.go:842` registerRunningCancel | 无法从 UI 终止 |
| 5 | 终止说明 | `server.go:852` applyAbortUserNote | Agent 不知为何被终止 |
| 6 | 统计分析 | `server.go:909` updateStats | 无成功率/耗时数据 |
| 7 | 多消费方共享 | 单 Agent + Eino 多 Agent 共用 | 两套注册逻辑 |

### 设计本质

MCP Server 是一个**进程内中间件层**，在 "handler 闭包" 和 "Agent 调用" 之间插入监控、审计、取消等横切关注点。类比 HTTP 中间件在请求处理链中加入日志和认证——不改业务逻辑，但获得全面的运维能力。
