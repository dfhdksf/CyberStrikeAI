# AI Agent 系统

## 概述

CyberStrikeAI 的 Agent 系统分为两层：
- **单 Agent 循环** (`internal/agent/`): 基于 OpenAI function-calling 的工具调用循环
- **多智能体编排** (`internal/multiagent/`): 基于 CloudWeGo Eino ADK 的多 Agent 协作

---

## 单 Agent 循环

### 核心流程

```
用户消息 → 构建消息列表 → 调用 LLM（含工具定义）
                                    ↓
                          LLM 返回 tool_calls?
                           ├─ 否 → 返回文本回复
                           └─ 是 → 执行工具调用
                                    ↓
                          工具结果追加到消息列表
                                    ↓
                          继续调用 LLM（循环）
                                    ↓
                    达到 max_iterations 或 LLM 无 tool_calls → 结束
```

### 关键设计

**迭代限制**: 通过 `agent.max_iterations` 配置最大循环次数，防止无限循环。

**工具超时**: 每个工具调用有独立超时（`agent.tool_timeout`），超时自动终止并返回错误信息给 LLM。

**对话恢复**: 支持 orphan tool message repair——当历史对话中存在未闭合的 tool_call 时，自动修补消息结构。

**流式输出**: 通过 SSE (Server-Sent Events) 实时流式返回 LLM 的文本输出。

---

## 多智能体编排

### 架构基础：CloudWeGo Eino ADK

Eino 是字节跳动开源的 AI 应用开发框架，CyberStrikeAI 使用其 ADK（Agent Development Kit）实现多智能体：

```
Eino ADK
├── Graph（有向图执行引擎）
├── Agent（工具调用循环）
├── Middleware（中间件链）
├── Callbacks（可观测性回调）
└── Orchestration（编排策略）
    ├── Deep（任务委派）
    ├── PlanExecute（计划执行）
    └── Supervisor（监督转发）
```

### 三种编排模式

#### Deep 模式（默认）

```
Orchestrator
    ├── 分析任务 → 决定委派给哪个子 Agent
    ├── transfer_to_recon_agent
    │       └── Recon Agent 执行侦察任务 → 返回结果
    ├── transfer_to_penetration_agent
    │       └── Penetration Agent 执行渗透 → 返回结果
    └── 综合所有子 Agent 结果 → 最终报告
```

- Orchestrator 拥有 `transfer_to_*` 工具，用于委派任务
- 子 Agent 完成后将结果返回 Orchestrator
- 适合任务明确、可分解的场景

#### Plan-Execute 模式

```
Planner → 生成执行计划（步骤列表）
    ↓
Executor → 逐步执行计划
    ↓
Replanner → 根据执行结果调整计划
    ↓
循环直到计划完成
```

- Planner 不执行工具，只生成结构化计划
- Executor 按计划调用工具
- Replanner 在每步后评估是否需要调整
- 适合复杂、多步骤、需要动态调整的任务

#### Supervisor 模式

```
Supervisor
    ├── 分析当前上下文
    ├── 决定转发给哪个子 Agent
    └── 子 Agent 处理后返回 Supervisor
        └── 继续分析或结束
```

- Supervisor 只做路由决策，不直接执行
- 适合任务类型多样、需要灵活切换的场景

### 子 Agent 定义

子 Agent 通过 Markdown 文件定义（`agents/` 目录），格式：

```markdown
---
name: recon
description: 信息收集与侦察专家
model: ""  # 留空则继承主模型
tools:
  - nmap
  - subfinder
  - httpx
---

## 角色

你是一个专业的信息收集 Agent...

## 指令

1. 首先确定目标范围...
2. 使用被动侦察工具...
```

### 中间件体系

Eino ADK 中间件在 Agent 处理链中插入额外逻辑：

| 中间件 | 作用 |
|--------|------|
| Summarization | 对话过长时自动摘要，管理上下文窗口 |
| Tool Search | 渐进式工具披露，按需搜索加载工具 |
| Reduction | 截断过长的工具输出 |
| HITL | Human-in-the-Loop 拦截 |
| Monitor | 工具执行记录 |

### Checkpoint 与恢复

多智能体支持 checkpoint 持久化：
- 每次 Agent 状态变更时保存 checkpoint
- 支持从 checkpoint 恢复执行
- 用于长时间运行任务的断点续传

---

## LLM 客户端 (`internal/openai/`)

### 多模型支持

```go
type Provider string

const (
    ProviderOpenAI    Provider = "openai"
    ProviderClaude    Provider = "claude"
    ProviderDeepSeek  Provider = "deepseek"
    // ... 其他 OpenAI 兼容提供商
)
```

### Claude 桥接

对于 Anthropic Claude 模型，客户端自动将 OpenAI 格式转换为 Claude Messages API：
- `messages` 格式转换
- `tool_use` / `tool_result` 块映射
- System prompt 处理差异
- Thinking/reasoning 块透传

### 推理模式 (`internal/reasoning/`)

支持配置 LLM 的推理能力：
- 标准模式：直接回复
- 思考模式：启用 extended thinking（Claude）或 reasoning tokens（OpenAI o1/o3）
- 可配置 budget tokens

---

## 视觉分析 (`internal/vision/`)

支持通过 VL（Vision-Language）模型分析图像：
- 截图分析（Web 页面、终端输出）
- 验证码识别
- 网络拓扑图理解
- 独立的模型配置（可使用不同于主模型的 VL 模型）

---

## Human-in-the-Loop (`internal/hitl/`)

### 审核机制

```
Agent 请求工具调用
        ↓
HITL 中间件拦截
        ↓
┌─ 在白名单中? → 放行
└─ 不在白名单 → 触发审核
        ↓
  ┌─ AI 审核 Agent 评估风险
  └─ 或等待人工审批
        ↓
  ┌─ 批准 → 执行
  └─ 拒绝 → 返回拒绝消息给 Agent
```

### 配置选项

- **工具白名单**: 无需审核直接放行的工具列表
- **审核模式**: `approval`（人工批准）或 `review-edit`（可修改参数后批准）
- **AI 审核 Agent**: 使用独立 LLM 调用评估工具调用的风险
- **留存策略**: 审核记录的保存时长
