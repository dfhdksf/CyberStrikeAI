# CyberStrikeAI 架构设计文档

## 项目概述

CyberStrikeAI 是一个 AI 原生的自动化安全测试平台，基于 Go 语言构建。平台将大语言模型（LLM）与专业安全工具深度集成，通过 MCP（Model Context Protocol）协议实现 AI Agent 对安全工具的统一调度，支持自动化渗透测试、漏洞扫描、权限提升、横向移动等完整攻击链。

**版本**: v1.6.49  
**语言**: Go 1.24 + Python 3.10+（工具依赖）  
**核心框架**: Gin (HTTP) + CloudWeGo Eino (多智能体) + MCP Go SDK

---

## 目录

- [系统架构总览](./system-overview.md)
- [目录结构](./directory-structure.md)
- [核心模块详解](./core-modules.md)
- [AI Agent 系统](./agent-system.md)
- [MCP 协议集成](./mcp-integration.md)
- [安全工具体系](./tool-system.md)
- [数据持久化](./database.md)
- [认证与安全](./auth-security.md)
- [Web 前端架构](./web-frontend.md)
- [部署与运维](./deployment.md)
