# 部署与运维

## 部署方式

### 一键部署（推荐）

```bash
./run.sh          # HTTPS 模式（自签名证书）
./run.sh --http   # HTTP 模式
```

自动完成：Python 环境检查 → Go 环境检查 → venv 创建 → 依赖安装 → 编译 → 启动

### Makefile 管理

```bash
make build       # 编译 Go 二进制
make setup       # 创建 Python venv + 安装依赖
make start       # nohup 后台启动
make stop        # 停止服务
make restart     # 重启
make status      # 查看运行状态
make logs        # 实时查看日志
make air         # 热加载开发模式
make clean       # 清理编译产物
```

### 系统要求

| 依赖 | 最低版本 | 用途 |
|------|----------|------|
| Go | 1.21+ | 编译运行 |
| Python | 3.10+ | 安全工具依赖 |
| SQLite | 3.x | 数据库（Go 静态链接） |
| 安全工具 | - | nmap, sqlmap 等（按需安装） |

---

## 进程管理

### PID 文件机制

```
启动: nohup ./cyberstrike-ai > cyberstrike.log 2>&1 & echo $! > .cyberstrike.pid
停止: kill $(cat .cyberstrike.pid); rm .cyberstrike.pid
```

### 优雅关闭

```go
// main.go
ctx, cancel := context.WithCancel(context.Background())
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

go func() {
    <-sigCh
    cancel()  // 触发所有组件的优雅关闭
}()
```

关闭顺序：
1. 停止接受新请求
2. 等待进行中的 Agent 循环完成（有超时）
3. 关闭外部 MCP 连接
4. 关闭 C2 监听器
5. 关闭数据库连接
6. 退出

---

## 热加载开发（air）

### 配置 (`.air.toml`)

```toml
[build]
  cmd = "go build -o ./tmp/cyberstrike-ai cmd/server/main.go"
  bin = "./tmp/cyberstrike-ai"
  full_bin = "./tmp/cyberstrike-ai -config config.yaml --https"
  include_ext = ["go", "yaml", "html", "tmpl"]
  include_dir = ["cmd", "internal", "web"]
  exclude_dir = ["tmp", "venv", "data", "knowledge_base", ".git"]
  delay = 2000  # 批量部署时避免多次重编译
```

### 与 GoLand 部署配合

GoLand auto-deploy 会批量推送文件，`delay = 2000ms` 确保所有文件同步完成后才触发重编译。

---

## 远程部署（GoLand）

### 排除路径 (`.deployignore`)

以下路径不应从本地同步到服务器：

```
data/               # 运行时数据库
chat_uploads/       # 对话附件
knowledge_base/     # 知识库文档
config.yaml         # 服务器独立配置
tmp/                # 编译临时文件
venv/               # Python 环境
.cyberstrike.pid    # PID 文件
cyberstrike.log     # 日志
```

### 部署流程

```
本地 Mac (开发)
    ↓ GoLand SFTP Auto-Deploy
Ubuntu 服务器
    ↓ air 检测文件变更
自动重编译 + 重启服务
```

---

## Git 工作流

### 分支策略

```
upstream/main (Ed1s0nZ/CyberStrikeAI)
    ↓ git fetch upstream
origin/main (fork: dfhdksf/CyberStrikeAI)
    ↓ git checkout -b main-llf
main-llf (开发分支)
    ↓ 日常开发
    ↓ make sync (合并上游更新)
    ↓ make push (推送到 origin/main-llf)
```

### Makefile Git 命令

```bash
make commit m="消息"   # git add -A && git commit
make push              # 推送 main-llf 到 origin
make pull              # 拉取 origin/main-llf
make sync              # 合并 upstream/main 到 main-llf
make sync-main         # 同步 fork 的 main 与上游一致
make diff              # 查看当前改动
make log               # 查看领先上游的提交
make branch            # 分支状态总览
```

---

## 配置热更新

无需重启即可更新运行时配置：

```
Web 界面修改配置 → POST /api/config → 更新内存配置 + 写入 YAML
                                         ↓
                    POST /api/config/apply → 热重载组件
                                         ↓
                    ├── 重新注册安全工具到 MCP
                    ├── 重连外部 MCP 服务器
                    ├── 更新 Agent 参数
                    ├── 重载知识库配置
                    └── 更新聊天机器人配置
```

---

## 监控与告警

### 内置监控

- 工具执行统计（成功率、耗时）
- Agent 循环计数
- 活跃会话数
- C2 连接状态
- 数据库大小

### OpenTelemetry

通过 `internal/einoobserve/` 集成 OpenTelemetry：
- 分布式追踪（每次 Agent 调用生成 trace）
- 自定义 span（LLM 调用、工具执行）
- 可对接 Jaeger、Zipkin 等追踪后端

### 日志

```bash
make logs    # 实时查看 cyberstrike.log
tail -f cyberstrike.log | grep ERROR
```

---

## 升级

```bash
./upgrade.sh    # 拉取最新代码 → 重新编译 → 重启
```

或手动：
```bash
make stop
git pull
make build
make start
```
