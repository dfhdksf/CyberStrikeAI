# 目录结构

```
CyberStrikeAI/
├── cmd/                          # 可执行入口
│   ├── server/main.go            # 主服务器（Web + API + MCP）
│   ├── mcp-stdio/main.go         # MCP stdio 传输模式（供外部 IDE 调用）
│   ├── test-sse-mcp-server/      # SSE MCP 测试服务器
│   ├── test-external-mcp/        # 外部 MCP 连接测试
│   └── test-config/              # 配置加载测试
│
├── internal/                     # 内部包（不可外部导入）
│   ├── agent/                    # 单 Agent 循环（OpenAI function-calling）
│   ├── agents/                   # Markdown 多智能体定义加载器
│   ├── app/                      # 应用引导、路由注册、生命周期
│   ├── attackchain/              # 攻击链图谱构建
│   ├── audit/                    # 平台操作审计日志
│   ├── c2/                       # 内置 C2（命令与控制）框架
│   ├── config/                   # 配置类型定义与加载
│   ├── database/                 # SQLite 持久化层
│   ├── einomcp/                  # Eino ADK ↔ MCP 工具桥接
│   ├── einoobserve/              # OpenTelemetry 回调
│   ├── handler/                  # HTTP/API 处理器（50+ 文件）
│   ├── hitl/                     # Human-in-the-Loop 审核
│   ├── knowledge/                # RAG 知识库（嵌入、索引、检索）
│   ├── logger/                   # 结构化日志（zap）
│   ├── mcp/                      # MCP Server + 外部 MCP Client 管理
│   │   └── builtin/              # 内置工具名称常量
│   ├── monitor/                  # 工具执行监控与留存
│   ├── multiagent/               # 多智能体编排（Eino ADK）
│   ├── openai/                   # OpenAI/Claude API 客户端
│   ├── project/                  # 项目黑板（跨对话事实）
│   ├── projectprompt/            # 项目提示词组装
│   ├── reasoning/                # LLM 推理配置
│   ├── robot/                    # 聊天机器人集成（钉钉/飞书/企微/微信）
│   ├── security/                 # 认证、命令执行器、限流
│   ├── skillpackage/             # 技能包文件系统布局
│   └── vision/                   # 视觉分析（VL 模型）
│
├── agents/                       # 多智能体 Markdown 定义（16 个专业 Agent）
│   ├── orchestrator.md           # Deep 模式协调者
│   ├── recon.md                  # 侦察 Agent
│   ├── penetration.md            # 渗透 Agent
│   ├── vulnerability-triage.md   # 漏洞评估 Agent
│   ├── privilege-escalation.md   # 提权 Agent
│   ├── lateral-movement.md       # 横向移动 Agent
│   └── ...                       # 更多专业 Agent
│
├── tools/                        # 安全工具 YAML 配置（~100 个）
│   ├── nmap.yaml                 # 端口扫描
│   ├── sqlmap.yaml               # SQL 注入
│   ├── nuclei.yaml               # 漏洞扫描
│   ├── ffuf.yaml                 # 目录爆破
│   └── ...
│
├── roles/                        # 角色 YAML 配置
│   ├── penetration.yaml          # 渗透测试角色
│   ├── recon.yaml                # 信息收集角色
│   ├── ctf.yaml                  # CTF 角色
│   └── ...
│
├── skills/                       # ADK 技能包（24 个目录）
│   ├── sql-injection/SKILL.md
│   ├── xss-testing/SKILL.md
│   ├── container-security/SKILL.md
│   └── ...
│
├── knowledge_base/               # RAG 知识库文档
│   ├── SQL注入/
│   ├── Prompt注入/
│   └── ...
│
├── plugins/                      # 外部插件
│   └── burp-extension/           # Burp Suite 扩展（Java JAR）
│
├── mcp-servers/                  # 外部 MCP 服务器文档
│
├── web/                          # Web 前端
│   ├── templates/                # HTML 模板
│   │   ├── index.html            # 主 SPA 页面
│   │   └── api-docs.html         # API 文档页面
│   └── static/                   # 静态资源
│       ├── css/                  # 样式
│       ├── js/                   # 35 个 JS 模块
│       ├── i18n/                 # 国际化（中/英）
│       └── vendor/               # 第三方库
│
├── data/                         # 运行时数据（SQLite 数据库）
├── docs/                         # 文档
├── images/                       # 截图资源
│
├── config.yaml                   # 主配置文件
├── go.mod / go.sum               # Go 模块
├── requirements.txt              # Python 依赖
├── Makefile                      # 构建/部署命令
├── run.sh                        # 一键部署脚本
└── upgrade.sh                    # 升级脚本
```

## 包依赖关系

```
cmd/server/main.go
    → config.Load()
    → app.New()
        → handler.*          (所有 HTTP 处理器)
        → security.*         (认证、执行器)
        → mcp.NewServer()    (内置 MCP 服务器)
        → mcp.NewExternalManager()  (外部 MCP 管理)
        → database.*         (SQLite 初始化)
        → knowledge.*        (RAG 知识库)
        → multiagent.*       (Eino 编排)
        → c2.*               (C2 框架)
        → robot.*            (聊天机器人)
        → audit.*            (审计日志)
        → monitor.*          (执行监控)
```
