# Web 前端架构

## 概述

CyberStrikeAI 前端是一个 **无框架的单页应用 (SPA)**，使用原生 JavaScript 构建，通过客户端路由实现页面切换。

---

## 技术栈

| 组件 | 技术 | 说明 |
|------|------|------|
| UI 框架 | Vanilla JS | 无 React/Vue/Angular |
| 路由 | 自定义 hash router | `web/static/js/router.js` |
| 样式 | 原生 CSS | 无预处理器 |
| 国际化 | i18next | 中英文切换 |
| Markdown | marked.js | 渲染 Agent 回复 |
| XSS 防护 | DOMPurify | 净化 HTML |
| 终端 | XTerm.js | Web Terminal |
| 图谱 | Cytoscape.js | 攻击链可视化 |
| 表格导出 | SheetJS (xlsx) | 漏洞导出 |

---

## 模块划分

```
web/static/js/
├── app.js                  # 应用入口、初始化
├── router.js               # 客户端路由
├── api.js                  # API 请求封装
├── auth.js                 # 登录/登出逻辑
├── chat.js                 # 聊天界面（核心交互）
├── dashboard.js            # 仪表盘
├── agents.js               # 多智能体管理
├── settings.js             # 系统配置
├── builtin-tools.js        # 内置工具管理
├── roles.js                # 角色管理
├── skills.js               # 技能包管理
├── knowledge.js            # 知识库管理
├── vulnerabilities.js      # 漏洞管理
├── projects.js             # 项目管理
├── audit.js                # 审计日志
├── monitor.js              # 执行监控
├── hitl.js                 # HITL 审核界面
├── tasks.js                # 批量任务
├── terminal.js             # Web Terminal
├── c2.js                   # C2 控制面板
├── webshell.js             # WebShell 管理
├── fact-graph.js           # 项目事实图谱
├── attack-chain.js         # 攻击链可视化
├── notifications.js        # 通知系统
├── i18n.js                 # 国际化
├── wechat-robot.js         # 微信机器人配置
└── fofa.js                 # FOFA 搜索
```

---

## 路由系统

基于 URL hash 的客户端路由：

```javascript
// router.js
const routes = {
  '':            () => loadChat(),
  'chat':        () => loadChat(),
  'dashboard':   () => loadDashboard(),
  'agents':      () => loadAgents(),
  'settings':    () => loadSettings(),
  'tools':       () => loadTools(),
  'roles':       () => loadRoles(),
  'skills':      () => loadSkills(),
  'knowledge':   () => loadKnowledge(),
  'vulns':       () => loadVulnerabilities(),
  'projects':    () => loadProjects(),
  'audit':       () => loadAudit(),
  'monitor':     () => loadMonitor(),
  'hitl':        () => loadHITL(),
  'tasks':       () => loadTasks(),
  'terminal':    () => loadTerminal(),
  'c2':          () => loadC2(),
  'webshell':    () => loadWebShell(),
};
```

---

## 实时通信

### SSE (Server-Sent Events)

用于 Agent 流式回复：

```javascript
// chat.js
const eventSource = new EventSource(`/api/multi-agent/stream?token=${token}`);
eventSource.onmessage = (event) => {
  const data = JSON.parse(event.data);
  appendToChat(data.content);
};
```

### WebSocket

用于需要双向通信的场景：
- Web Terminal（XTerm.js ↔ PTY session）
- C2 实时事件推送

---

## 国际化

```
web/static/i18n/
├── en-US.json    # English
└── zh-CN.json    # 简体中文
```

使用 i18next 实现运行时语言切换，无需刷新页面。

---

## 页面布局

```
┌─────────────────────────────────────────────────┐
│  顶部导航栏（logo、语言切换、退出登录）             │
├──────────┬──────────────────────────────────────┤
│          │                                      │
│  侧边栏   │           主内容区                    │
│  (导航)   │                                      │
│          │  ┌──────────────────────────────┐    │
│  聊天     │  │                              │    │
│  仪表盘   │  │     动态加载的页面内容          │    │
│  Agent   │  │                              │    │
│  工具     │  │                              │    │
│  角色     │  │                              │    │
│  技能     │  └──────────────────────────────┘    │
│  知识库   │                                      │
│  漏洞     │                                      │
│  项目     │                                      │
│  监控     │                                      │
│  终端     │                                      │
│  C2      │                                      │
│  审计     │                                      │
│  设置     │                                      │
│          │                                      │
├──────────┴──────────────────────────────────────┤
│  状态栏（连接状态、运行任务数）                      │
└─────────────────────────────────────────────────┘
```

---

## 安全措施

- **XSS 防护**: 所有 Agent 输出经 DOMPurify 净化后再渲染
- **Token 存储**: 使用 httpOnly cookie（首选）或 localStorage
- **CSRF**: 所有写操作使用 Bearer token 认证（非 Cookie 认证）
- **CSP**: Content Security Policy header（服务端设置）
