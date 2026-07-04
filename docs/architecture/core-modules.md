# 核心模块详解

## 应用引导 (`internal/app/`)

应用引导层负责组装所有组件并启动服务器。

### 初始化流程

```go
app.New(cfg, logger, configPath)
    ↓
1. 初始化 SQLite 数据库（WAL 模式）
2. 启动数据库 checkpoint 循环
3. 初始化安全模块（AuthManager, CommandExecutor）
4. 初始化内置 MCP Server
5. 注册安全工具到 MCP（从 YAML 加载）
6. 初始化外部 MCP Manager
7. 初始化知识库（RAG）
8. 初始化 C2 框架（可选）
9. 初始化审计、监控模块
10. 初始化聊天机器人（可选）
11. 注册所有 HTTP 路由
    ↓
application.RunWithContext(ctx)
    ↓
启动 Gin HTTP 服务器（HTTP 或 HTTPS/HTTP2）
等待 context 取消信号 → 优雅关闭
```

### 路由组织

路由在 `app` 包中集中注册，分为：
- **公开路由**: `/api/auth/login`, 机器人 webhook
- **受保护路由**: 需要 AuthMiddleware，包含所有业务 API
- **静态资源**: `web/static/` 和 `web/templates/`

---

## 配置系统 (`internal/config/`)

### 配置结构

```go
type Config struct {
    Version     string          // 显示版本号
    Server      ServerConfig    // host, port, TLS
    Auth        AuthConfig      // 密码, session TTL
    Log         LogConfig       // 级别, 输出
    Audit       AuditConfig     // 审计日志
    Monitor     MonitorConfig   // 执行监控留存
    OpenAI      OpenAIConfig    // LLM 提供商配置
    Vision      VisionConfig    // 视觉模型
    FOFA        FOFAConfig      // FOFA 搜索引擎
    Agent       AgentConfig     // 迭代次数, 超时, 工作区
    HITL        HITLConfig      // 人工审核配置
    Security    SecurityConfig  // 工具配置, 描述模式
    Database    DatabaseConfig  // SQLite 路径
    ExternalMCP []ExternalMCPConfig  // 外部 MCP 服务器
    Knowledge   KnowledgeConfig // RAG 配置
    C2          C2Config        // C2 模块开关
    Robots      RobotsConfig    // 聊天机器人
    MultiAgent  MultiAgentConfig // 多智能体编排
    Project     ProjectConfig   // 项目黑板
    RolesDir    string          // 角色目录
    SkillsDir   string          // 技能目录
    AgentsDir   string          // Agent 定义目录
}
```

### 热更新机制

配置支持运行时热更新，通过 `POST /api/config/apply` 触发：
1. 使用 `gopkg.in/yaml.v3` 的 Node AST 操作，保留 YAML 注释
2. 更新前创建 `.backup` 文件
3. 逐字段定位并替换 Node 值
4. 写回文件后，热重载相关组件（工具注册、MCP 连接、Agent 配置等）

---

## 日志系统 (`internal/logger/`)

基于 `uber-go/zap` 实现结构化日志：
- 支持 JSON 和 Console 输出格式
- 可配置日志级别（debug/info/warn/error）
- 文件和标准输出双写
- 自动轮转（基于文件大小）

---

## 审计系统 (`internal/audit/`)

记录平台关键操作的审计日志：
- 用户登录/登出
- 配置变更
- 工具执行
- Agent 对话
- C2 操作
- 持久化到 SQLite，支持查询和导出

---

## 监控系统 (`internal/monitor/`)

工具执行的全生命周期监控：
- 执行记录（开始时间、结束时间、状态、输出）
- SSE 实时推送执行状态
- 执行取消支持
- 可配置留存策略
- 统计分析（成功率、耗时分布）
