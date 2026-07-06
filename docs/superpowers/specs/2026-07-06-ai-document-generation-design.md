# AI 文档生成系统 · 设计文档

- 日期: 2026-07-06
- 状态: 待评审
- 分支: main-llf

## 1. 背景与目标

CyberStrikeAI 现有一套成熟的 Agent 架构(Eino ADK 的 Deep / Plan-Execute / Supervisor 编排、Markdown sub-agent、Skill 按需加载、MCP 工具)。本方案在**不改动编排内核**的前提下,新增一个「文档生成」能力:

> 给定**任意目标软件项目**的源码,让 Agent 读代码 + 查知识库,把内容填充进**企业质量体系 docx 模板**,产出符合模板样式规范的 Word 文档。

目标文档来自 `reference/表单模板/`,是一套覆盖软件研发全生命周期的企业模板(立项 → 需求 → 开发 → 测试 → 发布 → 验收),共 27 份 `.docx`,例如:

- `1.项目立项/GYAQ-QM-IPM-MT-02.技术预研计划.docx`
- `4.需求/GYAQ-SD-RD-MT-002.产品需求文档.docx`
- `6.测试/GYAQ-SD-VER-MT-007.XXXX测试报告模版.docx`

### 首期范围(YAGNI)

先打通 **1-2 种文档**(建议:技术预研计划 + 产品需求文档),验证「读模板 → Agent 填充 → 产出 docx」全链路。其余 25 份模板按同一套机制扩展,**无需改 Go 代码**。

## 2. 关键决策(已与用户确认)

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 内容来源 | 读目标项目源码 + 查知识库 | 通用文档工厂,不同文档按需取材 |
| 编排骨架 | **Deep 编排器** | 主 Agent `write_todos` 规划文档清单,`task` 委派给文档 sub-agent;支持文档间依赖与并行 |
| 文档规范载体 | **Skill**(`skills/doc-*/SKILL.md`) | 章节规范/写作要求是流程知识,按需加载,加新文档类型零代码 |
| docx 生成技术 | **python-docx 原地填充** | 模板是企业表单(封面/版本表/页眉页脚/6表格/自定义样式),必须完整保留;pandoc 会丢失固定结构 |
| 目标项目来源 | **本地路径** | 复用现有 `read_file/glob/grep`,无新管道 |

### 为什么否决 pandoc

真实模板 `技术预研计划.docx` 解剖结果:

- 4 个分节(`w:sectPr` × 4):封面 / 版本记录 / 目录 / 正文
- 6 个表格(`w:tbl` × 6):版本变更记录、成果清单、进度表、风险表等
- 独立页眉页脚(header × 6、footer × 7),footer 含「国御安全 版权所有」「文档密级:内部控制」等**必须原样保留**的企业固定内容
- 2 张图片(封面 logo)
- 大量自定义命名样式(`1GYKJ` 一级标题、`GYKJ5` 正文、`0GYKJ` 封面标题、`TOC1` 目录、`GYKJf1/GYKJf8` 版本记录/目录标题)

pandoc `--reference-doc` **只提取样式、从 Markdown 重新生成整篇**,会丢弃封面、版本表、页眉页脚固定内容、目录结构——模板等于白做。因此改用「保留整个模板文档,只往指定位置原地填充」的 python-docx 方案。

### 可行性验证(已实测)

- 项目 venv 已装 `python-docx 1.2.0`,**零新增系统依赖**
- 5 个正文章节统一用 `1GYKJ` 样式 → 可靠 **按段落样式名锚定章节**,内容不会插错位

## 3. 三层架构

```
用户: "给 /path/to/project 生成技术预研计划和产品需求文档"
                         │
                         ▼
        ┌────────────────────────────────────┐
        │  Deep 主编排器 (agents/orchestrator.md) │  ← 已存在,不改
        │  write_todos 规划文档清单 → task 委派    │
        └────────────────┬───────────────────┘
                         │ task("生成技术预研计划", ...)
                         ▼
        ┌────────────────────────────────────┐
        │  文档生成 sub-agent                   │  ← 新增 agents/document-generation.md
        └────────────────┬───────────────────┘
          ┌──────────┬───┴──────┬─────────────┐
          ▼          ▼          ▼             ▼
     [1] skill   [2] read_file [3] search_  [4] docx MCP 工具
      文档规范     glob/grep     knowledge     ← 新增 internal/docgen/
     "写哪些章节"  读目标代码    _base 查资料   "读模板→原地填充→出docx"
```

**三层分工**:

1. **Skill(内容规范)** — `skills/doc-*/SKILL.md` 定义每种文档「有哪些章节、每章写什么、按什么顺序调研」。纯 Markdown,加新文档类型只需加 skill。
2. **现有工具(采集素材)** — `read_file/glob/grep` 读目标代码,`search_knowledge_base`/`list_knowledge_risk_types` 查知识库。全部复用。
3. **docx MCP 工具(渲染落地)** — 新增,把 Agent 填好的内容用 python-docx 原地填进模板,保留其余一切。

## 4. docx 工具设计(唯一需写代码的部分)

新建 `internal/docgen/` 包 + Python 脚本,注册 **3 个 MCP 工具**(遵循 `internal/knowledge/tool.go` 的 `mcp.Tool{}` + handler 闭包 + `RegisterTool` 模式)。

Go 端通过项目现有的「Go 调 Python」机制执行脚本(与安全工具一致),脚本用 python-docx 操作 docx,JSON 通信。

### 工具 1: `docx_list_templates` — 列可用模板

```
输入: {}
输出: 模板清单(相对 reference/表单模板/ 的路径 + 文档名),供 Agent 选择
```

### 工具 2: `docx_read_outline` — 读模板结构

```
输入:  { template_path: "1.项目立项/GYAQ-QM-IPM-MT-02.技术预研计划.docx" }
输出:  一份"待填清单",含每个章节的标题、样式锚点、编写说明提示、表格结构:
  [章节] 1. 技术预研目标   (anchor: 1GYKJ #1)
     提示: 说明本次技术预研的主要内容与目标(必须是可以验证的)
  [章节] 2. 工作条件       (anchor: 1GYKJ #2)
     提示: 说明人员、软硬件设施、经费等要求
  [表格] 版本变更记录: 列[修订人|内容摘要|版本|日期|审核人|审批人]
  [表格] 进度表: 列[任务名称|开始时间|结束时间|参加人员]
  ...
```

脚本用 python-docx 遍历段落,按样式名识别章节标题(`1GYKJ`)、封面填空项(`GYKJb`)、表格(`doc.tables`),把「编写说明:...」提示语一并返回,让 Agent 知道每节该写什么。

### 工具 3: `docx_fill` — 原地填充产出 docx

```
输入: {
  template_path: "...",
  output_name:   "技术预研计划-某项目.docx",
  sections: [
    { anchor: "1. 技术预研目标", content: "本次预研目标是验证..." },
    { anchor: "2. 工作条件",     content: "人员: 2名后端..." }
  ],
  tables: [
    { table_name: "进度表", rows: [["需求分析","2026-07","2026-08","张三"]] }
  ],
  cover: { "文档编号": "GYAQ-...", "当前版本": "V1.0", "发布时间": "2026-07-06" }
}
输出: { output_path: "<workspace>/技术预研计划-某项目.docx" }
```

脚本用 python-docx:
1. 打开模板(**完整保留**封面/版本表/页眉页脚/目录/样式)
2. 按 anchor 定位每个章节标题段落,在其**后方插入**内容段落(沿用模板样式)
3. 按 table_name 定位空表格,填入行数据
4. 填封面填空项
5. 另存为成品到 workspace,返回路径

内容富文本处理:Agent 产出的内容用轻量标记(段落、无序列表),脚本映射为对应 python-docx 段落样式。首期支持段落 + 列表 + 表格;复杂图表列为后续。

### 输出与交付

- 成品写入 per-session **workspace**(`project.WorkspaceRootDir`,复用现有机制)
- 下载复用现有 `c.FileAttachment` 模式(参照 `internal/handler/chat_uploads.go` 的 Download)

## 5. Skill 设计(文档规范,纯 Markdown,零代码)

每种文档一个 skill,首期做 2 个:

```
skills/
├── doc-tech-preresearch/SKILL.md   # 技术预研计划
└── doc-prd/SKILL.md                # 产品需求文档
```

`SKILL.md` 示例(`doc-prd`):

```markdown
---
name: doc-prd
description: 生成产品需求文档(PRD)的工作流:分析目标项目代码,结合知识库,填充 PRD 模板
version: 1.0.0
---
# PRD 文档生成工作流

## 步骤
1. docx_list_templates 找到"产品需求文档"模板
2. docx_read_outline 读模板章节与编写说明
3. 用 glob 找 README/go.mod/package.json 确定技术栈
4. 用 glob+read_file 读入口/路由/核心模块,梳理功能点
5. 需要行业规范时 search_knowledge_base 查参考
6. 按模板章节逐节撰写内容
7. docx_fill 产出成品 docx

## 填充要求
- 基于代码事实,不臆造功能
- 代码里没有的需求,标注[待确认]
- 遵循模板每节的"编写说明"提示
```

## 6. 文档 sub-agent 设计

新增 `agents/document-generation.md`(参照 `agents/reporting-remediation.md` 的 front matter + 正文即 system prompt 模式):

```markdown
---
id: document-generation
name: 文档生成专员
description: 分析目标软件项目源码,结合知识库,按企业模板生成结构化 Word 文档。主 Agent 需提供目标项目路径与文档类型。
tools: [docx_list_templates, docx_read_outline, docx_fill, read_file, glob, grep, search_knowledge_base, list_knowledge_risk_types]
max_iterations: 0
---
(正文 = system prompt:说明职责、调研纪律、先读模板结构再填充、基于事实不臆造、
 调用对应 skill 获取该文档类型的章节规范)
```

被 Deep 编排器的 `task` 工具委派调用。**不设 `kind: orchestrator`**(每模式仅一个编排器)。

> 实现注意:front matter 的 `tools` 字段(`FrontMatter.Tools interface{}`,`internal/agents/markdown.go:27`)同时支持逗号分隔字符串 `"a, b"` 与 YAML 数组 `[a, b]` 两种写法;沿用哪种以现有 `agents/*.md` 为准。`read_file/glob/grep` 依赖 `multi_agent.eino_skills.filesystem_tools: true`(默认开),`skill` 工具用于按需加载上述文档规范 skill。

## 7. 工程改动清单

| 改动 | 文件 | 类型 |
|------|------|------|
| docx Python 脚本 | `internal/docgen/scripts/docx_tool.py` | 新增 |
| docx 工具注册 | `internal/docgen/tool.go` | 新增 |
| docx 工具执行封装 | `internal/docgen/runner.go`(Go 调 Python) | 新增 |
| 工具名常量 | `internal/mcp/builtin/constants.go` | 改(加 3 个常量 + IsBuiltinTool/GetAllBuiltinTools) |
| 注册 wiring | `internal/app/app.go` | 改(加 RegisterDocgenTool 调用) |
| 文档 sub-agent | `agents/document-generation.md` | 新增 |
| 文档 skills | `skills/doc-tech-preresearch/SKILL.md`、`skills/doc-prd/SKILL.md` | 新增 |
| Python 依赖声明 | `requirements.txt`(python-docx,venv 已装) | 改 |

**不改动**:Eino 编排内核、`agents/orchestrator.md`、现有 filesystem/knowledge 工具。

## 8. 安全考量

- **路径校验**:`docx_read_outline`/`docx_fill` 的 `template_path` 限定在 `reference/表单模板/` 下;目标项目路径与 output 限定在允许范围,参照现有 `validateChatAttachmentServerPath` 的防路径穿越模式。
- **只读目标代码**:文档 sub-agent 对目标项目仅用 `read_file/glob/grep`,不授予写/执行,避免读到 workspace 外敏感文件或误改目标项目。
- **HITL**:docx 工具属低风险(本地文件读写,不触网、不执行目标代码),可评估加入白名单自动放行;但读取任意本地路径仍建议保留审计记录。
- **Python 脚本注入**:Go 调 Python 通过参数数组/临时 JSON 文件传参,不做字符串拼接命令,防命令注入。

## 9. 扩展路径(首期之后)

加一种新文档类型,仅两步、**不改 Go 代码**:

1. 确认 `reference/表单模板/` 有对应 `.docx`(已有 27 份)
2. 加一个 `skills/doc-xxx/SKILL.md` 写清该文档的章节规范与调研步骤

docx 工具是**模板无关**的(读任意模板结构 + 填任意内容),无需为每种文档改工具。

后续可选增强(预留,不首期实现):

- 复杂富文本(图片、嵌套表格、图表)填充
- 目录域(TOC)自动更新
- 多文档批量生成 + 打包下载
- 从项目已有漏洞/事实数据(而非仅代码)取材,生成安全评估类文档

## 10. 验收标准(首期)

1. 用户给出本地项目路径,请求生成「技术预研计划」→ 产出的 docx **保留**封面、版本记录表、页眉页脚、目录
2. 5 个正文章节被正确填入基于代码分析的内容,位置无错位
3. 至少一个表格(如进度表)被正确填充
4. 成品可通过下载端点获取
5. 加第二种文档(PRD)仅通过新增 skill 实现,未改 docx 工具代码
