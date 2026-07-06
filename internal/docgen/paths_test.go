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
