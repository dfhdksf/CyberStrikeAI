# 认证与安全

## 认证系统

### AuthManager (`internal/security/auth_manager.go`)

```
登录流程:
用户提交密码 → AuthManager.Login(password)
    ├── 密码比对
    ├── 生成 UUID session token
    ├── 存入内存 session store（带 TTL）
    └── 返回 token + 过期时间

请求认证:
请求到达 → AuthMiddleware
    ├── 提取 token（优先级: Header > Query > Cookie）
    │   ├── Authorization: Bearer <token>
    │   ├── ?token=<token>
    │   └── Cookie: auth_token=<token>
    ├── AuthManager.ValidateSession(token)
    │   ├── 查找 session
    │   ├── 检查是否过期
    │   └── 返回有效/无效
    ├── 有效 → 放行到下一个 handler
    └── 无效 → 401 Unauthorized
```

### 安全特性

- **密码管理**: 首次运行自动生成强密码，写入 config.yaml
- **Session TTL**: 可配置（默认 12 小时），过期自动失效
- **并发安全**: sync.RWMutex 保护 session store
- **无状态前端**: Token 可存于 Cookie 或 localStorage

---

## MCP 认证

### Token 机制

```yaml
# config.yaml
mcp:
  auth_header: "X-MCP-Auth"
  auth_value: "a1b2c3d4..."  # 64 位 hex，首次运行自动生成
```

- 所有 MCP 请求必须携带认证 Header
- Token 首次启动时自动生成（crypto/rand）
- 支持通过 Web 界面查看/重置

---

## 命令执行安全

### 参数处理

工具参数通过模板化方式组装命令，而非直接拼接：

```go
// 不安全（不使用）:
cmd := fmt.Sprintf("nmap %s", userInput)

// 安全（实际实现）:
args := buildArgsFromSchema(tool.Parameters, userParams)
cmd := exec.Command(tool.Command, args...)
```

### Shell Session (`internal/security/shell_session.go`)

PTY-based shell session 用于交互式工具：
- 每个 session 有独立的 PTY
- 可配置空闲超时（自动关闭）
- 终端大小动态调整
- Unix only（creack/pty）

### 执行隔离

- 工作目录隔离（每个执行在配置的 workspace 中）
- 进程组管理（`setpgid`），确保清理子进程
- 无输出超时检测（防止进程挂起）
- 信号级别的终止（先 SIGINT → 等待 → SIGKILL）

---

## 限流

### Rate Limiter (`internal/security/ratelimit.go`)

针对机器人 webhook 端点的 Per-IP 限流：

```go
type RateLimiter struct {
    visitors map[string]*visitor  // IP -> 限流状态
    mu       sync.RWMutex
    rate     rate.Limit           // 每秒请求数
    burst    int                  // 突发容量
}
```

配置：60 请求/分钟（per IP），burst = 10

---

## TLS/HTTPS

### 自签名证书

首次以 `--https` 启动时自动生成：
- RSA 4096 位私钥
- 有效期 1 年
- 存储于 `data/tls/cert.pem` 和 `data/tls/key.pem`
- 支持 HTTP/2

### 启动模式

```bash
./cyberstrike-ai -config config.yaml --https  # HTTPS (默认)
./cyberstrike-ai -config config.yaml          # HTTP
```

---

## HITL 安全层

Human-in-the-Loop 作为最后一道安全防线：

### 风险评估

AI 审核 Agent 评估每个工具调用的风险等级：
- 信息收集类（低风险）: 通常自动放行
- 主动扫描类（中风险）: 需要确认
- 漏洞利用类（高风险）: 必须人工审批
- 数据操作类（极高风险）: 严格审批 + 参数审查

### 白名单机制

```yaml
hitl:
  tool_whitelist:
    - nmap          # 端口扫描：自动放行
    - whois         # Whois 查询：自动放行
    - dig           # DNS 查询：自动放行
  # 不在白名单中的工具需要审核
```

### 审核流程

```
工具调用请求
    ↓
├── 白名单命中 → 直接执行
└── 未命中 → 进入审核队列
    ↓
    ├── AI 审核（自动）
    │   └── LLM 评估风险 → 建议批准/拒绝
    └── 人工审核（手动）
        └── Web 界面显示待审核 → 用户操作
    ↓
    ├── 批准 → 执行并记录
    ├── 修改后批准 → 修改参数后执行
    └── 拒绝 → 返回拒绝原因给 Agent
```
