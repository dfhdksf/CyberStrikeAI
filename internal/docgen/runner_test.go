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
