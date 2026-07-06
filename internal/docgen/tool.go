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

// resolveWorkspace resolves the absolute workspace directory for the current conversation.
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

// RegisterDocgenTools registers the 3 docx generation tools with the MCP server.
func RegisterDocgenTools(mcpServer *mcp.Server, db *database.DB, cfg *config.Config, repoRoot string, logger *zap.Logger) {
	runner := NewRunner(repoRoot)

	// Tool 1: docx_list_templates
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

	// Tool 2: docx_read_outline
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

	// Tool 3: docx_fill
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
