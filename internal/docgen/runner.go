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
