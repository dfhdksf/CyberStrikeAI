"""docx 原地填充:在保留模板全部结构的前提下,把内容插入指定章节与表格。

设计要点:
- 章节:定位 style_id=1GYKJ 的一级标题段落,在其后插入正文段落(style_id=GYKJ5)
  与项目符号段落,原有标题、封面、页眉页脚、目录、表格全部保留。
- 表格:按 index 向指定表格追加行(常见如"进度表")。
- 封面字段:真实模板中封面字段(文档编号 / 文档密级 / 当前版本 / 发布时间 等)
  以形如 "字段名：" 的形式存放在 table[0] 的单元格里,而不是独立段落。
  因此填充时必须同时扫描 doc.paragraphs 与所有 table.cells。
"""
import copy
import json

from docx import Document
from docx.text.paragraph import Paragraph

W_NS = "{http://schemas.openxmlformats.org/wordprocessingml/2006/main}"
HEADING_STYLE_ID = "1GYKJ"
BODY_STYLE_ID = "GYKJ5"
BULLET_STYLE = "List Bullet"


def _resolve_style(doc, style_key):
    """按 style_id 或 name 解析样式;找不到返回 None(调用方回退默认)。"""
    if not style_key:
        return None
    for style in doc.styles:
        if getattr(style, "style_id", None) == style_key:
            return style
    try:
        return doc.styles[style_key]
    except KeyError:
        return None


def _apply_style(para, doc, style_key):
    style = _resolve_style(doc, style_key)
    if style is not None:
        try:
            para.style = style
        except (KeyError, ValueError):
            pass


def _insert_paragraph_after(paragraph, text, doc, style_key):
    """在给定段落之后插入一个新段落(克隆 pPr,保持文档流与段落属性)。"""
    new_p = copy.deepcopy(paragraph._p)
    # 清空 clone 段落里的所有 run,仅保留 pPr(段落属性)
    for r in new_p.findall(W_NS + "r"):
        new_p.remove(r)
    paragraph._p.addnext(new_p)

    para = Paragraph(new_p, paragraph._parent)
    _apply_style(para, doc, style_key)
    para.add_run(text)
    return para


def _iter_all_paragraphs(doc):
    """遍历文档 body 段落与全部表格单元格内段落。"""
    for p in doc.paragraphs:
        yield p
    for table in doc.tables:
        for row in table.rows:
            for cell in row.cells:
                for p in cell.paragraphs:
                    yield p


def _find_heading(doc, anchor):
    for p in doc.paragraphs:
        style = p.style
        style_id = getattr(style, "style_id", None) if style is not None else None
        if style_id == HEADING_STYLE_ID and anchor in (p.text or ""):
            return p
    return None


def _label_of(text):
    """把段落/单元格文本规范化为封面字段标签(去空白 + 去掉尾部中英文冒号)。"""
    return (text or "").strip().rstrip("：:").strip()


def _fill_cover(doc, cover):
    """把封面字段值追加到匹配的段落 / 表格单元格。"""
    if not cover:
        return
    # 同一 payload 中同名字段只填一次,避免误伤重复出现的标签
    remaining = dict(cover)

    for p in _iter_all_paragraphs(doc):
        if not remaining:
            return
        label = _label_of(p.text)
        if label and label in remaining:
            p.add_run(" " + str(remaining.pop(label)))


def cmd_fill(payload_path):
    with open(payload_path, "r", encoding="utf-8") as f:
        payload = json.load(f)

    doc = Document(payload["template"])
    filled_sections = 0
    filled_tables = 0

    # 章节填充:定位标题后按序插入正文段落与项目符号
    for sec in payload.get("sections", []):
        heading = _find_heading(doc, sec["anchor"])
        if heading is None:
            continue
        anchor_p = heading
        for text in sec.get("paragraphs", []):
            anchor_p = _insert_paragraph_after(anchor_p, text, doc, BODY_STYLE_ID)
        for bullet in sec.get("bullets", []):
            anchor_p = _insert_paragraph_after(anchor_p, bullet, doc, BULLET_STYLE)
        filled_sections += 1

    # 表格填充:按 index 向表格追加行
    for t in payload.get("tables", []):
        idx = t.get("index", -1)
        if idx < 0 or idx >= len(doc.tables):
            continue
        table = doc.tables[idx]
        ncols = len(table.columns)
        for row_vals in t.get("rows", []):
            cells = table.add_row().cells
            for ci, val in enumerate(row_vals[:ncols]):
                cells[ci].text = str(val)
        filled_tables += 1

    # 封面填充(段落 + 表格单元格两种存放形式都要处理)
    _fill_cover(doc, payload.get("cover", {}))

    doc.save(payload["output"])
    return {
        "output_path": payload["output"],
        "filled_sections": filled_sections,
        "filled_tables": filled_tables,
    }
