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
