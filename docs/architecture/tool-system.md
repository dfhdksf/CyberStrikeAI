# 安全工具体系

## 概述

CyberStrikeAI 的工具体系包含三个层次：
1. **安全工具** (YAML 定义): 约 100 个命令行安全工具的声明式封装
2. **技能包** (Markdown 定义): 24 个面向任务的操作指南
3. **角色** (YAML 定义): 预设的工具组合 + 系统提示词

---

## 安全工具 (`tools/`)

### YAML 定义格式

```yaml
name: nmap
description: 网络端口扫描工具
category: reconnaissance
command: nmap
parameters:
  - name: target
    type: string
    required: true
    description: 扫描目标 IP 或域名
  - name: ports
    type: string
    required: false
    flag: "-p"
    description: 端口范围
  - name: scan_type
    type: enum
    values: ["-sT", "-sS", "-sU", "-sV"]
    description: 扫描类型
  - name: output_format
    type: enum
    values: ["-oN", "-oX", "-oG"]
    description: 输出格式
examples:
  - description: 快速扫描常用端口
    command: nmap -sV -top-ports 1000 target.com
```

### 工具分类

| 分类 | 工具示例 | 用途 |
|------|----------|------|
| 侦察 | nmap, subfinder, httpx, whois, dig | 信息收集 |
| Web 扫描 | nikto, nuclei, wappalyzer | Web 漏洞检测 |
| 目录爆破 | ffuf, gobuster, dirsearch | 路径发现 |
| SQL 注入 | sqlmap | 数据库注入 |
| 密码破解 | hydra, hashcat, john | 凭据攻击 |
| 漏洞利用 | metasploit, searchsploit | 漏洞利用 |
| 后渗透 | linpeas, winpeas | 权限提升 |
| 容器安全 | trivy, docker-bench | 容器审计 |
| 二进制分析 | gdb, radare2, volatility3 | 逆向分析 |
| 网络 | tcpdump, wireshark, netcat | 流量分析 |
| 加密 | openssl, sslscan | 加密检测 |

### 工具到 MCP 的映射

```
YAML 定义 → config.SecurityToolConfig
    ↓
app 初始化时注册到 MCP Server
    ↓
MCP Tool Schema (name, description, inputSchema)
    ↓
Agent 通过 function-calling 调用
    ↓
CommandExecutor 执行实际命令
```

### 命令执行器 (`internal/security/executor.go`)

```go
type CommandExecutor struct {
    toolIndex  map[string]*Tool  // O(1) 工具查找
    workspace  string            // 工作目录
    timeout    time.Duration     // 默认超时
}
```

执行模式：
- **非交互式**: 执行命令等待完成，返回 stdout+stderr
- **流式**: 实时流式返回输出（用于长时间运行的扫描）
- **PTY Shell**: 创建 PTY 会话，支持交互式工具（如 Metasploit）

安全措施：
- 参数模板化，防止命令注入
- 可配置的无输出超时（自动终止挂起进程）
- 工作目录隔离
- 进程组管理（确保子进程随父进程终止）

---

## 技能包 (`skills/`)

### 定义格式

每个技能是一个目录，包含 `SKILL.md` 和相关资源文件：

```
skills/
├── sql-injection/
│   ├── SKILL.md           # 技能定义（前置条件 + 操作步骤）
│   └── payloads.txt       # 相关资源
├── xss-testing/
│   └── SKILL.md
└── container-security/
    └── SKILL.md
```

`SKILL.md` 格式：

```markdown
---
name: SQL 注入测试
description: 系统化的 SQL 注入检测与利用流程
category: web-security
tools:
  - sqlmap
  - burp
prerequisites:
  - 已确认目标 URL 和参数
---

## 测试流程

1. 手动探测注入点
2. 确认注入类型（联合、盲注、报错、堆叠）
3. 使用 sqlmap 自动化利用
4. 提取数据库结构和数据
5. 尝试 OS 命令执行和文件读取
```

### 渐进式工具披露

技能包通过 Eino 的 Tool Search Middleware 实现渐进式披露：
- Agent 不会看到全部 100+ 工具定义
- 当任务需要时，Agent 调用内置的搜索工具发现相关技能
- 按需加载技能中定义的工具子集
- 显著减少 Token 消耗

---

## 角色 (`roles/`)

### 定义格式

```yaml
name: penetration
display_name: 渗透测试专家
description: 专注于漏洞利用和渗透测试
system_prompt: |
  你是一个专业的渗透测试专家...
tools:
  - nmap
  - sqlmap
  - metasploit
  - hydra
  - burp
  - linpeas
max_iterations: 30
```

### 角色切换

用户可通过 Web 界面或 API 切换角色，切换时：
1. 加载对应角色的系统提示词
2. 只暴露角色定义的工具子集
3. 调整 Agent 参数（如最大迭代次数）

---

## C2 框架 (`internal/c2/`)

### 架构

内置的轻量级 Command & Control 框架：

```
C2 Server
├── Listeners（监听器）
│   ├── TCP Reverse Shell
│   ├── HTTP/HTTPS Beacon
│   └── WebSocket
├── Sessions（会话管理）
│   ├── 活跃连接
│   ├── 心跳检测
│   └── 会话加密（AES-256-GCM）
├── Tasks（任务下发）
│   ├── 命令执行
│   ├── 文件上传/下载
│   └── 持久化部署
├── Payloads（载荷生成）
│   ├── 多平台支持
│   └── 混淆选项
└── Profiles（通信配置）
    ├── 通信间隔
    ├── Jitter
    └── 伪装 Header
```

### HITL 集成

C2 的危险操作（如部署持久化、数据外传）自动触发 HITL 审核，人工批准后才执行。

### MCP 工具暴露

C2 能力以 MCP 工具形式暴露，Agent 可以通过标准 tool_call 操作 C2：
- 创建/管理监听器
- 查看活跃会话
- 下发任务
- 生成载荷
- 管理通信配置
