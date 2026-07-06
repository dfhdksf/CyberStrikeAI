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
