#!/usr/bin/env python3
"""CyberStrikeAI docx 工具:读取模板结构 / 列模板 / 原地填充。
成功时向 stdout 输出一行 JSON;失败时向 stderr 写错误并 exit(1)。
"""
import argparse
import json
import os
import sys

from docx import Document

# 一级正文章节样式(style_id;真实模板中 name 为 "标题 1（GYKJ）")
HEADING_STYLE_ID = "1GYKJ"
# 封面字段样式(style_id;真实模板中 name 为 "封面副标题（GYKJ）")
COVER_STYLE_ID = "GYKJf1"
# 章节提示语常用前缀
HINT_PREFIXES = ("编写说明", "提示")


def _para_text(p):
    return (p.text or "").strip()


def _style_matches(style, target_id):
    if style is None:
        return False
    return style.style_id == target_id or style.name == target_id


def cmd_outline(template):
    doc = Document(template)
    headings = []
    tables_meta = []
    cover_fields = []

    paras = doc.paragraphs
    for idx, p in enumerate(paras):
        text = _para_text(p)
        if not text:
            continue
        if _style_matches(p.style, HEADING_STYLE_ID):
            # 收集该标题之后、下一个标题之前的首个提示语作为 hint
            hint = ""
            for nxt in paras[idx + 1:]:
                if _style_matches(nxt.style, HEADING_STYLE_ID):
                    break
                ntext = _para_text(nxt)
                if ntext.startswith(HINT_PREFIXES):
                    hint = ntext
                    break
            headings.append(
                {"anchor": text, "style": HEADING_STYLE_ID, "text": text, "hint": hint}
            )
        elif _style_matches(p.style, COVER_STYLE_ID) and text.endswith("："):
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
