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

    def test_fill_cover_field_in_table_gets_written(self):
        """封面字段位于 table[0] 单元格中,填充后单元格文本应包含值。"""
        import tempfile
        from docx import Document

        with tempfile.TemporaryDirectory() as td:
            out = os.path.join(td, "out.docx")
            payload_data = self._fill_payload(out)
            # 只保留 cover 字段测试,清空 sections 干扰
            payload_data["cover"] = {"当前版本": "V1.0"}
            payload = os.path.join(td, "p.json")
            with open(payload, "w", encoding="utf-8") as f:
                json.dump(payload_data, f, ensure_ascii=False)
            proc = run("fill", "--payload", payload)
            self.assertEqual(proc.returncode, 0, proc.stderr)
            doc = Document(out)
            # 值必须真的落到表格 0 里,证明我们处理了 cell 中的封面字段
            table_text = "\n".join(
                cell.text
                for row in doc.tables[0].rows
                for cell in row.cells
            )
            self.assertIn("V1.0", table_text)


if __name__ == "__main__":
    unittest.main()
