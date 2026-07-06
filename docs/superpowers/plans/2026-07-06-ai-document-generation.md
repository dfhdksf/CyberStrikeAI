# AI 文档生成系统 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 CyberStrikeAI 的 Agent 读取任意目标项目源码 + 查知识库,把内容原地填充进企业 docx 模板,产出保留封面/表格/页眉页脚的 Word 文档。

**Architecture:** 三层分工——Skill 定义每种文档的章节规范(纯 Markdown,零代码扩展);现有 `read_file/glob/grep` + `search_knowledge_base` 采集素材;新增 3 个 docx MCP 工具用 python-docx 原地填充模板。Deep 编排器用 `task` 委派给新增的文档 sub-agent。

**Tech Stack:** Go 1.21+(MCP 工具、注册)、Python 3 + python-docx 1.2.0(docx 原地填充,venv 已装)、Eino ADK(编排,不改内核)。

## Global Constraints

- Go 版本:1.21+;Python:venv 已装 `python-docx 1.2.0`,零新增系统依赖(不引入 pandoc)。
- 模板根目录:`reference/表单模板/`(仓库内已有 27 份 `.docx`)。
- 首期文档类型:技术预研计划 + 产品需求文档(PRD)两种。
- MCP 工具注册遵循现有模式:`mcp.Tool{Name, Description, ShortDescription, InputSchema}` + `mcp.ToolHandler func(ctx, args) (*mcp.ToolResult, error)` + `mcpServer.RegisterTool(tool, handler)`。
- 软错误模式:handler 出错时返回 `&mcp.ToolResult{IsError: true, Content: [...]}, nil`(Go error 恒为 nil),与项目其它工具一致。
- 路径安全:所有 `template_path` 限定在 `reference/表单模板/` 下;`output_name` 用 `filepath.Base` 去除目录成分,防路径穿越。
- Python 脚本:成功时仅向 stdout 输出一行 JSON(`ensure_ascii=False`);错误写 stderr 并 `exit(1)`。
- Python 测试用内置 `unittest`(不依赖 pytest),用 `venv/bin/python3` 运行。

---

## File Structure

**新增:**
- `internal/docgen/scripts/docx_tool.py` — Python CLI:`list` / `outline` / `fill` 三个子命令,操作 docx。
- `internal/docgen/scripts/test_docx_tool.py` — Python unittest,针对真实模板。
- `internal/docgen/runner.go` — Go 侧:解析 venv python + 脚本路径,exec 子命令,JSON 编解码。
- `internal/docgen/runner_test.go` — Go 集成测试(真实调用 python)。
- `internal/docgen/tool.go` — 3 个 MCP 工具定义 + handler + 路径校验函数。
- `internal/docgen/tool_test.go` — 路径校验纯函数单测。
- `agents/document-generation.md` — 文档生成 sub-agent(front matter + system prompt)。
- `skills/doc-tech-preresearch/SKILL.md`、`skills/doc-prd/SKILL.md` — 两种文档的章节规范。

**修改:**
- `internal/mcp/builtin/constants.go` — 加 3 个工具名常量 + `IsBuiltinTool` + `GetAllBuiltinTools`。
- `internal/app/app.go` — 在 `registerProjectFactTools(...)` 调用后加 `docgen.RegisterDocgenTools(...)`(2 处注册点)。
- `requirements.txt` — 声明 `python-docx>=1.1.0`。

---

## Task 1: Python docx 引擎 — `list` 与 `outline` 子命令

**Files:**
- Create: `internal/docgen/scripts/docx_tool.py`
- Test: `internal/docgen/scripts/test_docx_tool.py`

**Interfaces:**
- Consumes: 真实模板 `reference/表单模板/1.项目立项/GYAQ-QM-IPM-MT-02.技术预研计划.docx`
- Produces: CLI `python3 docx_tool.py outline --template <abs_path>` → stdout 一行 JSON:
  `{"headings":[{"anchor":str,"style":str,"text":str,"hint":str}],"tables":[{"index":int,"name":str,"columns":[str]}],"cover_fields":[str]}`
- Produces: CLI `python3 docx_tool.py list --root <abs_dir>` → stdout 一行 JSON:
  `{"templates":[{"path":str,"name":str}]}`(path 为相对 root 的路径)

- [ ] **Step 1: 写失败测试(outline 解析真实模板)**

创建 `internal/docgen/scripts/test_docx_tool.py`:

```python
import json
import os
import subprocess
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
SCRIPT = os.path.join(HERE, "docx_tool.py")
REPO_ROOT = os.path.abspath(os.path.join(HERE, "..", "..", ".."))
TEMPLATE_ROOT = os.path.join(REPO_ROOT, "reference", "表单模板")
TECH_TEMPLATE = os.path.join(
    TEMPLATE_ROOT, "1.项目立项", "GYAQ-QM-IPM-MT-02.技术预研计划.docx"
)


def run(*args):
    proc = subprocess.run(
        ["python3", SCRIPT, *args], capture_output=True, text=True
    )
    return proc


class TestOutline(unittest.TestCase):
    def test_outline_returns_five_headings(self):
        proc = run("outline", "--template", TECH_TEMPLATE)
        self.assertEqual(proc.returncode, 0, proc.stderr)
        data = json.loads(proc.stdout)
        texts = [h["text"] for h in data["headings"]]
        # 5 个正文一级章节(样式 1GYKJ)
        self.assertIn("技术预研目标", "".join(texts))
        self.assertIn("可能存在的困难与风险", "".join(texts))
        self.assertGreaterEqual(len([h for h in data["headings"] if h["style"] == "1GYKJ"]), 5)

    def test_outline_captures_hint_text(self):
        proc = run("outline", "--template", TECH_TEMPLATE)
        data = json.loads(proc.stdout)
        joined = json.dumps(data, ensure_ascii=False)
        # 章节下的"编写说明/提示"文本被采集为 hint
        self.assertIn("可以验证", joined)

    def test_outline_lists_tables(self):
        proc = run("outline", "--template", TECH_TEMPLATE)
        data = json.loads(proc.stdout)
        self.assertGreaterEqual(len(data["tables"]), 5)


class TestList(unittest.TestCase):
    def test_list_finds_templates(self):
        proc = run("list", "--root", TEMPLATE_ROOT)
        self.assertEqual(proc.returncode, 0, proc.stderr)
        data = json.loads(proc.stdout)
        paths = [t["path"] for t in data["templates"]]
        self.assertTrue(any("技术预研计划" in p for p in paths))
        self.assertTrue(all(p.endswith(".docx") for p in paths))


if __name__ == "__main__":
    unittest.main()
```

- [ ] **Step 2: 运行测试,确认失败**

Run: `venv/bin/python3 internal/docgen/scripts/test_docx_tool.py -v`
Expected: FAIL —— `docx_tool.py` 不存在,报错 `can't open file .../docx_tool.py`。

- [ ] **Step 3: 实现 `docx_tool.py` 的 list 与 outline**

创建 `internal/docgen/scripts/docx_tool.py`:

```python
#!/usr/bin/env python3
"""CyberStrikeAI docx 工具:读取模板结构 / 列模板 / 原地填充。
成功时向 stdout 输出一行 JSON;失败时向 stderr 写错误并 exit(1)。
"""
import argparse
import json
import os
import sys

from docx import Document

# 一级正文章节样式名(见 reference 模板解剖)
HEADING_STYLE = "1GYKJ"
# 章节提示语常用前缀
HINT_PREFIXES = ("编写说明", "提示")


def _para_text(p):
    return (p.text or "").strip()


def cmd_outline(template):
    doc = Document(template)
    headings = []
    tables_meta = []
    cover_fields = []

    paras = doc.paragraphs
    for idx, p in enumerate(paras):
        style = p.style.name if p.style is not None else ""
        text = _para_text(p)
        if not text:
            continue
        if style == HEADING_STYLE:
            # 收集该标题之后、下一个标题之前的首个提示语作为 hint
            hint = ""
            for nxt in paras[idx + 1:]:
                nstyle = nxt.style.name if nxt.style is not None else ""
                if nstyle == HEADING_STYLE:
                    break
                ntext = _para_text(nxt)
                if ntext.startswith(HINT_PREFIXES):
                    hint = ntext
                    break
            headings.append(
                {"anchor": text, "style": style, "text": text, "hint": hint}
            )
        elif style == "GYKJb" and text.endswith("："):
            cover_fields.append(text.rstrip("："))

    for i, tbl in enumerate(doc.tables):
        columns = []
        if tbl.rows:
            columns = [c.text.strip() for c in tbl.rows[0].cells]
        name = columns[0] if columns else f"table_{i}"
        tables_meta.append({"index": i, "name": name, "columns": columns})

    return {"headings": headings, "tables": tables_meta, "cover_fields": cover_fields}


def cmd_list(root):
    templates = []
    for dirpath, _dirs, files in os.walk(root):
        for f in files:
            if f.endswith(".docx") and not f.startswith("~$"):
                full = os.path.join(dirpath, f)
                rel = os.path.relpath(full, root)
                templates.append({"path": rel, "name": os.path.splitext(f)[0]})
    templates.sort(key=lambda t: t["path"])
    return {"templates": templates}


def main():
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="cmd", required=True)

    p_out = sub.add_parser("outline")
    p_out.add_argument("--template", required=True)

    p_list = sub.add_parser("list")
    p_list.add_argument("--root", required=True)

    p_fill = sub.add_parser("fill")
    p_fill.add_argument("--payload", required=True)

    args = parser.parse_args()
    try:
        if args.cmd == "outline":
            result = cmd_outline(args.template)
        elif args.cmd == "list":
            result = cmd_list(args.root)
        elif args.cmd == "fill":
            from docx_fill import cmd_fill  # Task 2 提供
            result = cmd_fill(args.payload)
        else:
            raise SystemExit(2)
    except Exception as exc:  # noqa: BLE001
        sys.stderr.write(str(exc))
        sys.exit(1)
    sys.stdout.write(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    main()
```

- [ ] **Step 4: 运行测试,确认通过**

Run: `venv/bin/python3 internal/docgen/scripts/test_docx_tool.py -v`
Expected: PASS(4 个测试全过)。若 `hint` 断言失败,检查真实模板中提示语前缀,调整 `HINT_PREFIXES`。

- [ ] **Step 5: 提交**

```bash
git add internal/docgen/scripts/docx_tool.py internal/docgen/scripts/test_docx_tool.py
git commit -m "feat(docgen): docx 模板结构解析(list/outline 子命令)"
```

---

## Task 2: Python docx 引擎 — `fill` 子命令(原地填充)

**Files:**
- Create: `internal/docgen/scripts/docx_fill.py`
- Modify: `internal/docgen/scripts/test_docx_tool.py`(追加 fill 测试)

**Interfaces:**
- Consumes: Task 1 的 `docx_tool.py`(其 `fill` 子命令 `from docx_fill import cmd_fill`)
- Produces: `cmd_fill(payload_path: str) -> dict`,读取 JSON 文件,内容形如:
  `{"template":abs,"output":abs,"sections":[{"anchor":str,"paragraphs":[str],"bullets":[str]}],"tables":[{"index":int,"rows":[[str]]}],"cover":{field:value}}`
  返回 `{"output_path":str,"filled_sections":int,"filled_tables":int}`。
- 原地填充语义:打开模板 → 保留封面/版本表/页眉页脚/目录/样式不变 → 在每个 `anchor` 标题段落后插入 `paragraphs`(正文样式 `GYKJ5`)与 `bullets`(列表);按 `index` 向空表格追加行;填封面字段 → 另存到 output。

- [ ] **Step 1: 追加失败测试(fill 保留结构并填入内容)**

在 `test_docx_tool.py` 末尾(`if __name__` 之前)追加:

```python
class TestFill(unittest.TestCase):
    def _fill_payload(self, out_path):
        return {
            "template": TECH_TEMPLATE,
            "output": out_path,
            "sections": [
                {
                    "anchor": "技术预研目标",
                    "paragraphs": ["验证 docx 原地填充可行性。"],
                    "bullets": ["目标一", "目标二"],
                }
            ],
            "tables": [],
            "cover": {"当前版本": "V1.0"},
        }

    def test_fill_creates_output_and_inserts_content(self):
        import tempfile
        from docx import Document

        with tempfile.TemporaryDirectory() as td:
            out = os.path.join(td, "out.docx")
            payload = os.path.join(td, "p.json")
            with open(payload, "w", encoding="utf-8") as f:
                json.dump(self._fill_payload(out), f, ensure_ascii=False)
            proc = run("fill", "--payload", payload)
            self.assertEqual(proc.returncode, 0, proc.stderr)
            res = json.loads(proc.stdout)
            self.assertTrue(os.path.exists(res["output_path"]))
            # 内容被插入
            doc = Document(out)
            body = "\n".join(p.text for p in doc.paragraphs)
            self.assertIn("验证 docx 原地填充可行性", body)
            self.assertIn("目标一", body)

    def test_fill_preserves_headers_and_tables(self):
        import tempfile
        from docx import Document

        original = Document(TECH_TEMPLATE)
        orig_table_count = len(original.tables)
        orig_section_count = len(original.sections)

        with tempfile.TemporaryDirectory() as td:
            out = os.path.join(td, "out.docx")
            payload = os.path.join(td, "p.json")
            with open(payload, "w", encoding="utf-8") as f:
                json.dump(self._fill_payload(out), f, ensure_ascii=False)
            run("fill", "--payload", payload)
            doc = Document(out)
            # 表格数量不变(封面/版本表/进度表等全部保留)
            self.assertEqual(len(doc.tables), orig_table_count)
            # 分节数量不变(封面/目录/正文分节保留 → 页眉页脚随之保留)
            self.assertEqual(len(doc.sections), orig_section_count)
```

- [ ] **Step 2: 运行测试,确认失败**

Run: `venv/bin/python3 internal/docgen/scripts/test_docx_tool.py -v`
Expected: FAIL —— `fill` 子命令 import `docx_fill` 失败(模块不存在),返回码非 0。

- [ ] **Step 3: 实现 `docx_fill.py`**

创建 `internal/docgen/scripts/docx_fill.py`:

```python
"""docx 原地填充:在保留模板全部结构的前提下,把内容插入指定章节与表格。"""
import copy
import json

from docx import Document

HEADING_STYLE = "1GYKJ"
BODY_STYLE = "GYKJ5"


def _insert_paragraph_after(paragraph, text, style_name):
    """在给定段落之后插入一个新段落(复用底层 XML,保留文档流)。"""
    new_p = copy.deepcopy(paragraph._p)
    # 清空 clone 段落里的所有 run,只保留段落属性(pPr)
    for r in new_p.findall(
        "{http://schemas.openxmlformats.org/wordprocessingml/2006/main}r"
    ):
        new_p.remove(r)
    paragraph._p.addnext(new_p)

    from docx.text.paragraph import Paragraph

    para = Paragraph(new_p, paragraph._parent)
    try:
        para.style = style_name
    except KeyError:
        pass  # 样式不存在则用默认
    para.add_run(text)
    return para


def _find_heading(doc, anchor):
    for p in doc.paragraphs:
        style = p.style.name if p.style is not None else ""
        if style == HEADING_STYLE and (p.text or "").strip().find(anchor) >= 0:
            return p
    return None


def cmd_fill(payload_path):
    with open(payload_path, "r", encoding="utf-8") as f:
        payload = json.load(f)

    doc = Document(payload["template"])
    filled_sections = 0
    filled_tables = 0

    # 章节填充:在标题段落后按顺序插入 paragraphs + bullets
    for sec in payload.get("sections", []):
        heading = _find_heading(doc, sec["anchor"])
        if heading is None:
            continue
        anchor_p = heading
        for text in sec.get("paragraphs", []):
            anchor_p = _insert_paragraph_after(anchor_p, text, BODY_STYLE)
        for bullet in sec.get("bullets", []):
            anchor_p = _insert_paragraph_after(anchor_p, bullet, "List Bullet")
        filled_sections += 1

    # 表格填充:向指定索引表格追加行
    for t in payload.get("tables", []):
        idx = t["index"]
        if idx < 0 or idx >= len(doc.tables):
            continue
        table = doc.tables[idx]
        ncols = len(table.columns)
        for row_vals in t.get("rows", []):
            cells = table.add_row().cells
            for ci, val in enumerate(row_vals[:ncols]):
                cells[ci].text = str(val)
        filled_tables += 1

    # 封面字段:找到形如 "字段：" 的段落,在其 run 后补值
    cover = payload.get("cover", {})
    if cover:
        for p in doc.paragraphs:
            label = (p.text or "").strip().rstrip("：")
            if label in cover:
                p.add_run(" " + str(cover[label]))

    doc.save(payload["output"])
    return {
        "output_path": payload["output"],
        "filled_sections": filled_sections,
        "filled_tables": filled_tables,
    }
```

- [ ] **Step 4: 运行测试,确认通过**

Run: `venv/bin/python3 internal/docgen/scripts/test_docx_tool.py -v`
Expected: PASS(6 个测试全过)。若 `List Bullet` 样式缺失导致 KeyError,已被 try/except 兜底为默认样式,不影响通过。

- [ ] **Step 5: 提交**

```bash
git add internal/docgen/scripts/docx_fill.py internal/docgen/scripts/test_docx_tool.py
git commit -m "feat(docgen): docx 原地填充(fill 子命令,保留模板全部结构)"
```

---

## Task 3: Go runner — 调用 Python 引擎

**Files:**
- Create: `internal/docgen/runner.go`
- Test: `internal/docgen/runner_test.go`

**Interfaces:**
- Produces: `type Runner struct { repoRoot string }`
- Produces: `func NewRunner(repoRoot string) *Runner`
- Produces: `func (r *Runner) pythonBin() string` — 返回 `<repoRoot>/venv/bin/python3`,若不存在回退 `"python3"`。
- Produces: `func (r *Runner) scriptPath() string` — `<repoRoot>/internal/docgen/scripts/docx_tool.py`。
- Produces: `func (r *Runner) run(ctx context.Context, args []string, stdin []byte) ([]byte, error)` — exec python,返回 stdout;非零退出把 stderr 包进 error。
- Produces: `func (r *Runner) List(ctx context.Context, root string) ([]byte, error)`
- Produces: `func (r *Runner) Outline(ctx context.Context, templatePath string) ([]byte, error)`
- Produces: `func (r *Runner) Fill(ctx context.Context, payloadJSON []byte) ([]byte, error)` — 把 payload 写临时文件,传 `--payload <tmp>`,返回 stdout。

- [ ] **Step 1: 写失败测试**

创建 `internal/docgen/runner_test.go`:

```go
package docgen

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoRootForTest 从当前测试文件位置回溯到仓库根(internal/docgen → 上两级)。
func repoRootForTest(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

func TestOutlineReturnsHeadings(t *testing.T) {
	root := repoRootForTest(t)
	tmpl := filepath.Join(root, "reference", "表单模板", "1.项目立项", "GYAQ-QM-IPM-MT-02.技术预研计划.docx")
	if _, err := os.Stat(tmpl); err != nil {
		t.Skipf("模板缺失,跳过: %v", err)
	}
	r := NewRunner(root)
	out, err := r.Outline(context.Background(), tmpl)
	if err != nil {
		t.Fatalf("Outline error: %v", err)
	}
	var data struct {
		Headings []struct {
			Text string `json:"text"`
		} `json:"headings"`
	}
	if err := json.Unmarshal(out, &data); err != nil {
		t.Fatalf("bad json: %v; raw=%s", err, out)
	}
	joined := ""
	for _, h := range data.Headings {
		joined += h.Text
	}
	if !strings.Contains(joined, "技术预研目标") {
		t.Fatalf("期望包含章节'技术预研目标',实际: %s", joined)
	}
}

func TestListReturnsTemplates(t *testing.T) {
	root := repoRootForTest(t)
	templateRoot := filepath.Join(root, "reference", "表单模板")
	if _, err := os.Stat(templateRoot); err != nil {
		t.Skipf("模板目录缺失,跳过: %v", err)
	}
	r := NewRunner(root)
	out, err := r.List(context.Background(), templateRoot)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if !strings.Contains(string(out), "技术预研") {
		t.Fatalf("期望模板列表含技术预研,实际: %s", out)
	}
}
```

- [ ] **Step 2: 运行测试,确认失败**

Run: `go test ./internal/docgen/ -run 'TestOutline|TestList' -v`
Expected: FAIL —— 编译错误,`NewRunner`/`Outline`/`List` 未定义。

- [ ] **Step 3: 实现 runner.go**

创建 `internal/docgen/runner.go`:

```go
package docgen

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Runner 封装对 Python docx 引擎的调用。
type Runner struct {
	repoRoot string
}

func NewRunner(repoRoot string) *Runner {
	return &Runner{repoRoot: repoRoot}
}

func (r *Runner) pythonBin() string {
	venv := filepath.Join(r.repoRoot, "venv", "bin", "python3")
	if _, err := os.Stat(venv); err == nil {
		return venv
	}
	return "python3"
}

func (r *Runner) scriptPath() string {
	return filepath.Join(r.repoRoot, "internal", "docgen", "scripts", "docx_tool.py")
}

func (r *Runner) run(ctx context.Context, args []string, _ []byte) ([]byte, error) {
	full := append([]string{r.scriptPath()}, args...)
	cmd := exec.CommandContext(ctx, r.pythonBin(), full...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("docx 引擎执行失败: %w; stderr=%s", err, stderr.String())
	}
	return stdout.Bytes(), nil
}

func (r *Runner) List(ctx context.Context, root string) ([]byte, error) {
	return r.run(ctx, []string{"list", "--root", root}, nil)
}

func (r *Runner) Outline(ctx context.Context, templatePath string) ([]byte, error) {
	return r.run(ctx, []string{"outline", "--template", templatePath}, nil)
}

func (r *Runner) Fill(ctx context.Context, payloadJSON []byte) ([]byte, error) {
	tmp, err := os.CreateTemp("", "docx-fill-*.json")
	if err != nil {
		return nil, fmt.Errorf("创建临时 payload 失败: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(payloadJSON); err != nil {
		tmp.Close()
		return nil, fmt.Errorf("写入 payload 失败: %w", err)
	}
	tmp.Close()
	return r.run(ctx, []string{"fill", "--payload", tmp.Name()}, nil)
}
```

- [ ] **Step 4: 运行测试,确认通过**

Run: `go test ./internal/docgen/ -run 'TestOutline|TestList' -v`
Expected: PASS(两个测试通过;若 venv 无 python-docx 则报 import 错,先 `venv/bin/pip install python-docx`)。

- [ ] **Step 5: 提交**

```bash
git add internal/docgen/runner.go internal/docgen/runner_test.go
git commit -m "feat(docgen): Go runner 调用 python docx 引擎"
```

---

## Task 4: 路径安全校验(纯函数)

**Files:**
- Create: `internal/docgen/paths.go`
- Test: `internal/docgen/paths_test.go`

**Interfaces:**
- Produces: `func TemplateRoot(repoRoot string) string` — `<repoRoot>/reference/表单模板`。
- Produces: `func ResolveTemplatePath(repoRoot, rel string) (string, error)` — 把用户给的相对路径拼到 TemplateRoot 下,清理后校验仍在 TemplateRoot 内,且以 `.docx` 结尾;越界或非 docx 返回 error。
- Produces: `func SafeOutputName(name string) string` — `filepath.Base` 去目录成分,空则默认 `document.docx`,不以 `.docx` 结尾则补上。

- [ ] **Step 1: 写失败测试**

创建 `internal/docgen/paths_test.go`:

```go
package docgen

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveTemplatePathValid(t *testing.T) {
	root := "/repo"
	got, err := ResolveTemplatePath(root, "1.项目立项/技术预研计划.docx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(root, "reference", "表单模板", "1.项目立项", "技术预研计划.docx")
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestResolveTemplatePathTraversalRejected(t *testing.T) {
	if _, err := ResolveTemplatePath("/repo", "../../../etc/passwd"); err == nil {
		t.Fatal("期望拒绝路径穿越,但通过了")
	}
}

func TestResolveTemplatePathNonDocxRejected(t *testing.T) {
	if _, err := ResolveTemplatePath("/repo", "1.项目立项/notes.txt"); err == nil {
		t.Fatal("期望拒绝非 docx,但通过了")
	}
}

func TestSafeOutputName(t *testing.T) {
	cases := map[string]string{
		"报告.docx":        "报告.docx",
		"报告":             "报告.docx",
		"../../etc/x.docx": "x.docx",
		"":               "document.docx",
	}
	for in, want := range cases {
		if got := SafeOutputName(in); got != want {
			t.Fatalf("SafeOutputName(%q)=%q want %q", in, got, want)
		}
	}
}

func TestResolveTemplatePathAbsoluteRejected(t *testing.T) {
	// 绝对路径应被当作相对 TemplateRoot 处理或拒绝,不得逃逸
	got, err := ResolveTemplatePath("/repo", "/etc/passwd")
	if err == nil && !strings.HasPrefix(got, filepath.Join("/repo", "reference")) {
		t.Fatalf("绝对路径逃逸: %s", got)
	}
}
```

- [ ] **Step 2: 运行测试,确认失败**

Run: `go test ./internal/docgen/ -run 'TestResolveTemplatePath|TestSafeOutputName' -v`
Expected: FAIL —— 函数未定义,编译错误。

- [ ] **Step 3: 实现 paths.go**

创建 `internal/docgen/paths.go`:

```go
package docgen

import (
	"fmt"
	"path/filepath"
	"strings"
)

// TemplateRoot 返回企业模板根目录。
func TemplateRoot(repoRoot string) string {
	return filepath.Join(repoRoot, "reference", "表单模板")
}

// ResolveTemplatePath 把用户提供的相对路径安全解析到模板根下。
func ResolveTemplatePath(repoRoot, rel string) (string, error) {
	if !strings.HasSuffix(strings.ToLower(rel), ".docx") {
		return "", fmt.Errorf("模板必须是 .docx 文件: %s", rel)
	}
	base := TemplateRoot(repoRoot)
	// 去掉前导分隔符,避免绝对路径逃逸
	cleanedRel := filepath.Clean(strings.TrimPrefix(rel, string(filepath.Separator)))
	full := filepath.Clean(filepath.Join(base, cleanedRel))
	// 校验仍在 base 之内
	baseAbs := filepath.Clean(base)
	if full != baseAbs && !strings.HasPrefix(full, baseAbs+string(filepath.Separator)) {
		return "", fmt.Errorf("模板路径越界: %s", rel)
	}
	return full, nil
}

// SafeOutputName 去除目录成分并确保 .docx 扩展名。
func SafeOutputName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "document.docx"
	}
	if !strings.HasSuffix(strings.ToLower(name), ".docx") {
		name += ".docx"
	}
	return name
}
```

- [ ] **Step 4: 运行测试,确认通过**

Run: `go test ./internal/docgen/ -run 'TestResolveTemplatePath|TestSafeOutputName' -v`
Expected: PASS(5 个测试全过)。

- [ ] **Step 5: 提交**

```bash
git add internal/docgen/paths.go internal/docgen/paths_test.go
git commit -m "feat(docgen): 模板路径安全校验(防穿越)"
```

---

## Task 5: 工具名常量

**Files:**
- Modify: `internal/mcp/builtin/constants.go`

**Interfaces:**
- Produces: 常量 `ToolDocxListTemplates = "docx_list_templates"`、`ToolDocxReadOutline = "docx_read_outline"`、`ToolDocxFill = "docx_fill"`,并纳入 `IsBuiltinTool` / `GetAllBuiltinTools`。

- [ ] **Step 1: 加常量**

在 `internal/mcp/builtin/constants.go` 的 C2 工具常量块之后(第 62 行 `ToolC2File` 定义之后、`)` 之前)加入:

```go
	// 文档生成工具(docgen)
	ToolDocxListTemplates = "docx_list_templates" // 列出可用企业文档模板
	ToolDocxReadOutline   = "docx_read_outline"   // 读取模板章节/表格结构
	ToolDocxFill          = "docx_fill"           // 原地填充模板生成 docx
```

- [ ] **Step 2: 纳入 IsBuiltinTool**

在 `IsBuiltinTool` 的 `case` 列表里,`ToolC2File:` 之前(即最后一个 C2 常量 `ToolC2File` 那一行改为带逗号并追加三个):

```go
		ToolC2File,
		// 文档生成工具
		ToolDocxListTemplates,
		ToolDocxReadOutline,
		ToolDocxFill:
		return true
```

- [ ] **Step 3: 纳入 GetAllBuiltinTools**

在 `GetAllBuiltinTools` 返回的切片里,`ToolC2File,` 之后加入:

```go
		ToolC2File,
		// 文档生成工具
		ToolDocxListTemplates,
		ToolDocxReadOutline,
		ToolDocxFill,
	}
```

- [ ] **Step 4: 编译验证**

Run: `go build ./internal/mcp/...`
Expected: 无错误输出(退出码 0)。

- [ ] **Step 5: 提交**

```bash
git add internal/mcp/builtin/constants.go
git commit -m "feat(docgen): 注册 docx 工具名常量"
```

---

## Task 6: MCP 工具定义与 handler

**Files:**
- Create: `internal/docgen/tool.go`
- Test: `internal/docgen/tool_test.go`

**Interfaces:**
- Consumes: `NewRunner`/`List`/`Outline`/`Fill`(Task 3)、`TemplateRoot`/`ResolveTemplatePath`/`SafeOutputName`(Task 4)、工具名常量(Task 5)、`mcp.Server.RegisterTool`、`mcp.Tool`/`mcp.ToolResult`/`mcp.Content`(`internal/mcp/types.go`)。
- Consumes: 会话上下文解析 —— `agent.ConversationIDFromContext(ctx)` / `mcp.MCPConversationIDFromContext(ctx)`、`db.GetConversationProjectID(convID)`、`project.WorkspaceRootDir(cfg.Agent.WorkspaceRootDir, projectID, convID)`、`project.EnsureWorkspace(root)`。
- Produces: `func RegisterDocgenTools(mcpServer *mcp.Server, db *database.DB, cfg *config.Config, repoRoot string, logger *zap.Logger)`。
- Produces: 私有 helper `func textResult(s string) *mcp.ToolResult`、`func errResult(format string, a ...any) *mcp.ToolResult`(IsError=true)。

- [ ] **Step 1: 写 helper 的失败测试**

创建 `internal/docgen/tool_test.go`:

```go
package docgen

import "testing"

func TestErrResultIsError(t *testing.T) {
	r := errResult("boom %d", 1)
	if !r.IsError {
		t.Fatal("errResult 应 IsError=true")
	}
	if len(r.Content) != 1 || r.Content[0].Text != "boom 1" {
		t.Fatalf("unexpected content: %+v", r.Content)
	}
}

func TestTextResultNotError(t *testing.T) {
	r := textResult("hello")
	if r.IsError {
		t.Fatal("textResult 不应是错误")
	}
	if r.Content[0].Text != "hello" {
		t.Fatalf("unexpected: %+v", r.Content)
	}
}
```

- [ ] **Step 2: 运行测试,确认失败**

Run: `go test ./internal/docgen/ -run 'TestErrResult|TestTextResult' -v`
Expected: FAIL —— `errResult`/`textResult` 未定义。

- [ ] **Step 3: 实现 tool.go**

创建 `internal/docgen/tool.go`:

```go
package docgen

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/mcp/builtin"
	"cyberstrike-ai/internal/project"

	"go.uber.org/zap"
)

func textResult(s string) *mcp.ToolResult {
	return &mcp.ToolResult{Content: []mcp.Content{{Type: "text", Text: s}}}
}

func errResult(format string, a ...any) *mcp.ToolResult {
	return &mcp.ToolResult{
		Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf(format, a...)}},
		IsError: true,
	}
}

func convIDFromCtx(ctx context.Context) string {
	if id := agent.ConversationIDFromContext(ctx); id != "" {
		return id
	}
	return mcp.MCPConversationIDFromContext(ctx)
}

// resolveWorkspace 解析当前会话的绝对工作目录(生成的 docx 落此处)。
func resolveWorkspace(ctx context.Context, db *database.DB, cfg *config.Config) (string, error) {
	convID := convIDFromCtx(ctx)
	projectID := ""
	if convID != "" {
		if pid, err := db.GetConversationProjectID(convID); err == nil {
			projectID = pid
		}
	}
	rel := project.WorkspaceRootDir(cfg.Agent.WorkspaceRootDir, projectID, convID)
	return project.EnsureWorkspace(rel)
}

// RegisterDocgenTools 注册 3 个文档生成工具到 MCP。
func RegisterDocgenTools(mcpServer *mcp.Server, db *database.DB, cfg *config.Config, repoRoot string, logger *zap.Logger) {
	runner := NewRunner(repoRoot)

	// 工具 1: docx_list_templates
	listTool := mcp.Tool{
		Name:             builtin.ToolDocxListTemplates,
		Description:      "列出可用的企业文档模板(reference/表单模板 下的 .docx)。生成文档前先调用此工具,拿到模板相对路径,再用 docx_read_outline 读取结构。",
		ShortDescription: "列出可用文档模板",
		InputSchema: map[string]interface{}{
			"type": "object", "properties": map[string]interface{}{}, "required": []string{},
		},
	}
	mcpServer.RegisterTool(listTool, func(ctx context.Context, _ map[string]interface{}) (*mcp.ToolResult, error) {
		out, err := runner.List(ctx, TemplateRoot(repoRoot))
		if err != nil {
			logger.Error("docx list 失败", zap.Error(err))
			return errResult("列出模板失败: %v", err), nil
		}
		return textResult(string(out)), nil
	})

	// 工具 2: docx_read_outline
	outlineTool := mcp.Tool{
		Name:             builtin.ToolDocxReadOutline,
		Description:      "读取指定模板的章节标题、每节编写说明(hint)、表格列结构与封面字段。据此逐节撰写内容,再调用 docx_fill。参数 template_path 用 docx_list_templates 返回的相对路径。",
		ShortDescription: "读取模板章节/表格结构",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"template_path": map[string]interface{}{
					"type": "string", "description": "模板相对路径(相对 reference/表单模板/)",
				},
			},
			"required": []string{"template_path"},
		},
	}
	mcpServer.RegisterTool(outlineTool, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		rel, _ := args["template_path"].(string)
		full, err := ResolveTemplatePath(repoRoot, rel)
		if err != nil {
			return errResult("模板路径无效: %v", err), nil
		}
		out, err := runner.Outline(ctx, full)
		if err != nil {
			logger.Error("docx outline 失败", zap.Error(err))
			return errResult("读取模板结构失败: %v", err), nil
		}
		return textResult(string(out)), nil
	})

	// 工具 3: docx_fill
	fillTool := mcp.Tool{
		Name:             builtin.ToolDocxFill,
		Description:      "把内容原地填充进模板并生成 docx(保留封面/表格/页眉页脚/目录)。sections 每项 {anchor(章节标题), paragraphs[], bullets[]};tables 每项 {index, rows[][]};cover 为封面字段键值。生成文件写入当前会话工作目录。",
		ShortDescription: "填充模板生成 docx",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"template_path": map[string]interface{}{"type": "string", "description": "模板相对路径"},
				"output_name":   map[string]interface{}{"type": "string", "description": "输出文件名(如 技术预研计划-XX.docx)"},
				"sections": map[string]interface{}{
					"type": "array", "description": "章节内容",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"anchor":     map[string]interface{}{"type": "string"},
							"paragraphs": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
							"bullets":    map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
						},
						"required": []string{"anchor"},
					},
				},
				"tables": map[string]interface{}{"type": "array", "description": "表格行数据"},
				"cover":  map[string]interface{}{"type": "object", "description": "封面字段键值"},
			},
			"required": []string{"template_path", "output_name", "sections"},
		},
	}
	mcpServer.RegisterTool(fillTool, func(ctx context.Context, args map[string]interface{}) (*mcp.ToolResult, error) {
		rel, _ := args["template_path"].(string)
		full, err := ResolveTemplatePath(repoRoot, rel)
		if err != nil {
			return errResult("模板路径无效: %v", err), nil
		}
		ws, err := resolveWorkspace(ctx, db, cfg)
		if err != nil {
			return errResult("解析工作目录失败: %v", err), nil
		}
		outName := SafeOutputName(asString(args["output_name"]))
		outPath := filepath.Join(ws, outName)

		payload := map[string]interface{}{
			"template": full,
			"output":   outPath,
			"sections": args["sections"],
			"tables":   args["tables"],
			"cover":    args["cover"],
		}
		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			return errResult("序列化 payload 失败: %v", err), nil
		}
		out, err := runner.Fill(ctx, payloadJSON)
		if err != nil {
			logger.Error("docx fill 失败", zap.Error(err))
			return errResult("生成文档失败: %v", err), nil
		}
		return textResult(fmt.Sprintf("文档已生成: %s\n%s", outPath, string(out))), nil
	})

	logger.Info("文档生成工具已注册",
		zap.String("t1", listTool.Name), zap.String("t2", outlineTool.Name), zap.String("t3", fillTool.Name))
}

func asString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
```

- [ ] **Step 4: 运行测试 + 编译验证**

Run: `go test ./internal/docgen/ -run 'TestErrResult|TestTextResult' -v && go build ./internal/docgen/`
Expected: 测试 PASS,build 无错误。

- [ ] **Step 5: 提交**

```bash
git add internal/docgen/tool.go internal/docgen/tool_test.go
git commit -m "feat(docgen): 3 个 docx MCP 工具(list/outline/fill)"
```

---

## Task 7: 接入 app.go 注册流程

**Files:**
- Modify: `internal/app/app.go`(import 段 + 2 处注册点:`internal/app/app.go:134` 附近与 `internal/app/app.go:437` 附近)

**Interfaces:**
- Consumes: `docgen.RegisterDocgenTools(mcpServer, db, cfg, repoRoot, log.Logger)`(Task 6)。
- repoRoot 用 `os.Getwd()`(服务从仓库根启动,见 `docs/architecture/deployment.md`);解析失败则回退 `"."`。

- [ ] **Step 1: 加 import**

在 `internal/app/app.go` 顶部 import 块中,`cyberstrike-ai/internal/...` 分组内加入(保持字母序附近即可):

```go
	"cyberstrike-ai/internal/docgen"
```

同时确认 `"os"` 已在 import 中(若无则加 `"os"`)。

- [ ] **Step 2: 在主注册块接入(约 app.go:135)**

在 `registerVisionTools(mcpServer, cfg, log.Logger)` 之后新增:

```go
	registerVisionTools(mcpServer, cfg, log.Logger)

	// 注册文档生成工具(docgen)
	repoRoot, err := os.Getwd()
	if err != nil {
		repoRoot = "."
	}
	docgen.RegisterDocgenTools(mcpServer, db, cfg, repoRoot, log.Logger)
```

> 注意:若该函数作用域内已有名为 `err` 的变量导致 `:=` 冲突,改用 `repoRoot, wdErr := os.Getwd()` 并对应判断 `wdErr`。

- [ ] **Step 3: 在热重载注册器接入(约 app.go:437)**

在 `vulnerabilityRegistrar` 闭包内,`registerVisionTools(...)` 之后新增(此处用 `os.Getwd()` 同样处理):

```go
		registerVisionTools(mcpServer, cfg, log.Logger)
		if repoRoot, wdErr := os.Getwd(); wdErr == nil {
			docgen.RegisterDocgenTools(mcpServer, db, cfg, repoRoot, log.Logger)
		} else {
			docgen.RegisterDocgenTools(mcpServer, db, cfg, ".", log.Logger)
		}
		return nil
```

- [ ] **Step 4: 编译验证**

Run: `go build ./...`
Expected: 无错误输出(退出码 0)。

- [ ] **Step 5: 提交**

```bash
git add internal/app/app.go
git commit -m "feat(docgen): 接入 app 注册流程(含热重载)"
```

---

## Task 8: 文档 sub-agent + skills + 依赖声明

**Files:**
- Create: `agents/document-generation.md`
- Create: `skills/doc-tech-preresearch/SKILL.md`
- Create: `skills/doc-prd/SKILL.md`
- Modify: `requirements.txt`

**Interfaces:**
- Consumes: 工具名 `docx_list_templates`/`docx_read_outline`/`docx_fill`(Task 5/6)、现有工具 `read_file`/`glob`/`grep`/`search_knowledge_base`/`skill`。
- Produces: 一个可被 Deep 编排器 `task` 委派的文档生成 sub-agent + 两份文档规范 skill。

- [ ] **Step 1: 创建 sub-agent 定义**

创建 `agents/document-generation.md`(front matter 的 `tools` 用逗号分隔字符串,与项目现有约定一致):

```markdown
---
id: document-generation
name: 文档生成专员
description: 分析目标软件项目源码并结合知识库,按企业 docx 模板生成结构化文档(如技术预研计划、产品需求文档)。主 Agent 需在 task.description 中提供目标项目本地路径与要生成的文档类型。
tools: docx_list_templates, docx_read_outline, docx_fill, read_file, glob, grep, search_knowledge_base, list_knowledge_risk_types, skill
max_iterations: 0
---

你是授权研发流程中的**文档生成子代理**,负责把目标软件项目的真实情况整理成符合企业模板规范的 Word 文档。

## 输入前置条件

- 你默认不拥有父代理完整上下文,仅以本次 `task.description` 为准。
- 必须先确认两项信息:目标项目**本地路径**、要生成的**文档类型**。缺失则先向主 Agent 返回缺失清单,不要臆造。

## 工作流(严格按序)

1. 调用 `skill` 加载对应文档类型的规范(如 `doc-prd`、`doc-tech-preresearch`),获取该文档的章节要求与调研步骤。
2. 调用 `docx_list_templates` 找到对应模板的相对路径。
3. 调用 `docx_read_outline` 读取模板章节标题、每节 hint(编写说明)、表格列结构。
4. 用 `glob`/`grep`/`read_file` 在**目标项目路径下**分析代码:先读 README/go.mod/package.json 等定位技术栈,再读入口/路由/核心模块梳理功能点。**只读,不写、不执行目标项目代码。**
5. 需要行业规范或参考资料时,调用 `search_knowledge_base`。
6. 按模板每节的 hint 逐节撰写内容;每节内容组织为 paragraphs(段落)与 bullets(要点列表)。
7. 调用 `docx_fill` 生成成品:template_path 用第 2 步的路径,output_name 形如 `<文档名>-<项目名>.docx`,sections 的 anchor 用第 3 步返回的章节标题。

## 硬约束

- **基于代码事实撰写**,不臆造功能;代码中无法确认的需求标注 `[待确认]`。
- 遵循模板每节的 hint 要求。
- 禁止再次调用 `task`。
- 生成完成后,向主 Agent 报告成品文件路径。

输出后直接结束。
```

- [ ] **Step 2: 创建技术预研计划 skill**

创建 `skills/doc-tech-preresearch/SKILL.md`:

```markdown
---
name: doc-tech-preresearch
description: 生成《技术预研计划》文档的工作流:分析目标项目技术选型与可行性,填充企业技术预研计划模板
version: 1.0.0
---

# 技术预研计划文档生成

## 对应模板
`1.项目立项/GYAQ-QM-IPM-MT-02.技术预研计划.docx`(用 docx_list_templates 确认实际路径)

## 章节规范(对应模板 hint)
- **技术预研目标**:说明本次预研要验证的技术点,必须可验证。从目标项目的技术栈、关键依赖、架构难点提炼。
- **工作条件**:人员、软硬件设施、经费等要求。从项目规模与技术复杂度推断。
- **应递交的工作成果**:列出预研产出(原型/报告/选型结论),填入成果表。
- **进度表**:拆解预研任务,填入进度表(任务名/开始/结束/参加人员)。
- **可能存在的困难与风险**:识别技术风险,填风险表(编号/描述/策略/风险值/减缓策略)。

## 调研步骤
1. glob 找 go.mod/package.json/requirements.txt/README 确定技术栈
2. grep 关键框架、外部服务、协议,识别技术难点
3. read_file 读架构相关目录,判断可行性风险
4. 需要技术对比资料时 search_knowledge_base

## 填充要求
- 目标必须可验证,不写空泛描述
- 风险要具体到技术点,给出减缓策略
```

- [ ] **Step 3: 创建 PRD skill**

创建 `skills/doc-prd/SKILL.md`:

```markdown
---
name: doc-prd
description: 生成《产品需求文档(PRD)》的工作流:分析目标项目功能,填充企业产品需求文档模板
version: 1.0.0
---

# 产品需求文档(PRD)生成

## 对应模板
`4.需求/GYAQ-SD-RD-MT-002.产品需求文档.docx`(用 docx_list_templates 确认实际路径,并用 docx_read_outline 读取真实章节)

## 撰写原则
- 基于代码事实梳理功能,不臆造
- 代码里无法确认的需求标注 `[待确认]`
- 每个功能:名称 + 描述 + 优先级(P0/P1/P2)

## 调研步骤
1. glob 找 README/入口文件确定产品定位与技术栈
2. glob + read_file 读路由/handler/核心模块,枚举功能点
3. grep 配置项、权限、外部集成,补全非功能需求
4. 行业规范或竞品参考用 search_knowledge_base

## 章节填充
- 产品概述:一句话定位 + 目标用户 + 核心价值
- 功能列表/需求:逐条功能,含优先级
- 非功能需求:性能/安全/可用性(从代码中的鉴权、限流、TLS 等推断)
```

- [ ] **Step 4: 声明 python-docx 依赖**

在 `requirements.txt` 末尾追加:

```
# 文档生成(docgen)docx 原地填充
python-docx>=1.1.0
```

- [ ] **Step 5: 提交**

```bash
git add agents/document-generation.md skills/doc-tech-preresearch/SKILL.md skills/doc-prd/SKILL.md requirements.txt
git commit -m "feat(docgen): 文档生成 sub-agent + 两份文档规范 skill + 依赖声明"
```

---

## Task 9: 端到端验证与全量测试

**Files:**
- 无新增(纯验证任务)

**Interfaces:**
- Consumes: 全部前序任务的产物。

- [ ] **Step 1: 全量编译**

Run: `go build ./...`
Expected: 无错误(退出码 0)。

- [ ] **Step 2: 运行 docgen 全部 Go 测试**

Run: `go test ./internal/docgen/... -v`
Expected: 所有测试 PASS(paths / tool helper / runner Outline+List)。

- [ ] **Step 3: 运行 Python 全部测试**

Run: `venv/bin/python3 internal/docgen/scripts/test_docx_tool.py -v`
Expected: 6 个测试 PASS。

- [ ] **Step 4: 端到端冒烟(手工构造 fill,产出真实 docx 并校验保留结构)**

Run:

```bash
cd /Users/lilinfei/Desktop/AI-code/CyberStrikeAI
TMPL="reference/表单模板/1.项目立项/GYAQ-QM-IPM-MT-02.技术预研计划.docx"
cat > /tmp/docgen_smoke.json <<JSON
{"template":"$TMPL","output":"/tmp/docgen_out.docx","sections":[{"anchor":"技术预研目标","paragraphs":["冒烟测试:验证端到端生成。"],"bullets":["要点A","要点B"]}],"tables":[],"cover":{"当前版本":"V1.0"}}
JSON
venv/bin/python3 internal/docgen/scripts/docx_tool.py fill --payload /tmp/docgen_smoke.json
venv/bin/python3 -c "
from docx import Document
d=Document('/tmp/docgen_out.docx')
body='\n'.join(p.text for p in d.paragraphs)
assert '冒烟测试' in body, '内容未写入'
assert len(d.tables)>=5, '表格丢失'
assert len(d.sections)>=3, '分节/页眉页脚丢失'
print('SMOKE OK: 内容已填 + 表格', len(d.tables), '+ 分节', len(d.sections))
"
rm -f /tmp/docgen_smoke.json /tmp/docgen_out.docx
```

Expected: 打印 `SMOKE OK: ...`,无 AssertionError。

- [ ] **Step 5: 提交(如有验证性微调)**

若前面步骤发现问题并修复,提交修复;否则本步无改动可跳过。

```bash
git add -A
git commit -m "test(docgen): 端到端验证通过" || echo "无改动可提交"
```

- [ ] **Step 6: 下载路径的运行时人工核验(spec 验收 #4)**

本方案不新建下载端点——生成的 docx 落在会话 workspace,复用现有 `c.FileAttachment` 机制(`internal/handler/chat_uploads.go:198`,`GET /api/chat-uploads/download?path=...`)。启动服务后人工核验:让文档 sub-agent 生成一份文档,确认 Agent 回报的绝对路径能通过该端点下载。

**若发现** `validateChatAttachmentServerPath`(`internal/handler/agent.go:368`)把可下载范围限制在 `chat_uploads/` 而不含 workspace,则需追加一个后续任务:新增受保护的 workspace 文件下载端点,并按 `canAccessVulnerability` 的属主校验模式做会话/项目级授权(见 spec §8)。此为运行时依赖,不阻塞前 5 步的自动化验证。