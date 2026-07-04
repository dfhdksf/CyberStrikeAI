# 数据持久化

## 概述

CyberStrikeAI 使用 SQLite 作为持久化层，配合文件系统存储知识库文档。

---

## SQLite 配置

### 数据库文件

| 数据库 | 路径 | 用途 |
|--------|------|------|
| 主数据库 | `data/conversations.db` | 对话、C2、审计、监控等 |
| 知识库 | `data/knowledge.db` | RAG 向量索引 |

### WAL 模式

启用 Write-Ahead Logging 模式，提供：
- 读写并发（读不阻塞写）
- 崩溃恢复能力
- 自动 checkpoint 循环（后台定时触发）

### 初始化

```go
func InitDB(dbPath string) (*sql.DB, error) {
    db.Exec("PRAGMA journal_mode=WAL")
    db.Exec("PRAGMA synchronous=NORMAL")
    db.Exec("PRAGMA busy_timeout=5000")
    // 自动建表（IF NOT EXISTS）
}
```

---

## 数据模型

### 对话系统

```
conversations
├── id (UUID)
├── title
├── group_id
├── role
├── project_id
├── created_at / updated_at
└── messages (JSON array in SQLite)

conversation_groups
├── id
├── name
├── color
└── sort_order
```

### 漏洞管理

```
vulnerabilities
├── id (UUID)
├── conversation_id
├── title
├── severity (critical/high/medium/low/info)
├── category
├── target
├── description
├── proof_of_concept
├── remediation
├── status (open/confirmed/fixed/accepted)
├── cvss_score
└── created_at / updated_at
```

### 项目黑板

```
project_facts
├── id
├── project_id
├── key (唯一标识)
├── value (事实内容)
├── category
├── confidence (0-1)
├── source
├── deprecated (软删除)
└── created_at / updated_at
```

### C2 数据

```
c2_listeners       → 监听器配置和状态
c2_sessions        → 活跃会话
c2_tasks           → 下发任务
c2_events          → 事件日志
c2_payloads        → 生成的载荷
c2_profiles        → 通信配置
c2_files           → 文件传输记录
```

### 批量任务

```
batch_tasks
├── id
├── name
├── description
├── targets (JSON array)
├── tool / parameters
├── schedule (cron expression)
├── status
├── results (JSON)
└── created_at / started_at / completed_at
```

### 审计日志

```
audit_logs
├── id
├── action
├── actor
├── target
├── details (JSON)
├── ip_address
└── timestamp
```

### 执行监控

```
execution_records
├── id (UUID)
├── tool_name
├── parameters (JSON)
├── conversation_id
├── status (running/completed/failed/cancelled)
├── output
├── started_at / completed_at
├── duration_ms
└── error
```

---

## 知识库 (`internal/knowledge/`)

### RAG 流程

```
文档导入 → 文本分块 → 嵌入向量化 → 存储到知识库
    ↓
查询 → MultiQuery 扩展 → 向量检索 → 重排序 → 去重 → 返回结果
```

### 组件

| 组件 | 实现 | 说明 |
|------|------|------|
| 嵌入模型 | OpenAI Embeddings API | 可配置 model、维度 |
| 向量存储 | SQLite + 自定义索引 | 余弦相似度 |
| 分块策略 | 固定大小 + 重叠 | 可配置 chunk_size、overlap |
| MultiQuery | LLM 生成多个变体查询 | 提高召回率 |
| 重排序 | Cohere Rerank / 交叉编码器 | 可选 |
| 去重 | 内容指纹 | 后检索去重 |

### 知识库目录结构

```
knowledge_base/
├── SQL注入/
│   ├── 基础原理.md
│   ├── 绕过技巧.md
│   └── 防御方案.md
├── Prompt注入/
│   ├── 攻击向量.md
│   └── 防御策略.md
└── ...
```

### 索引管理

- 支持增量索引（只处理新增/修改的文档）
- 支持全量重建索引
- 通过 API 触发索引更新
- 索引状态可查询

---

## 文件存储

### 对话附件

```
chat_uploads/
├── {conversation_id}/
│   ├── screenshot.png
│   ├── report.pdf
│   └── ...
```

### 运行时数据

```
data/
├── conversations.db     # 主数据库
├── knowledge.db         # 知识库
├── tls/                 # 自签名证书（自动生成）
│   ├── cert.pem
│   └── key.pem
└── ...
```
