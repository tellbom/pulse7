# pulse7 Web GUI

基于 `artifacts/api-contract.md`（冻结契约）与定稿 Figma 原型（`web/figma/Agent Tool Web Page`）复刻的图形界面。

## 技术栈（对齐目标环境 Win7 SP1 + Chrome 109）

- Vue 3.4 + Vite 4.5（`build.target: chrome109`）
- Element Plus 2.4（Chrome 109 时代版本；消息提示等轻量使用，视觉以原型样式为准）
- 字体只用 Win7 自带：微软雅黑（正文）/ Consolas（机器原文）
- 无 Tailwind / 无 CSS-in-JS：scoped + 语义令牌（`src/styles/tokens.css`），类名按组件前缀隔离，避免样式覆盖

## 运行

```bash
npm install
npm run dev        # 开发（无后端时自动进入内置演示后端 mock 模式）
npm run build      # 产物 → web/dist/
npm run preview    # 本机静态预览（--host，可供 VM 访问回测）
npm run deploy:agent  # 复制 dist/ → agent/web/，供 pulse7 serve 嵌入
```

## 与 pulse7 serve 的集成

- 页面同源由 `pulse7 serve` 提供；`index.html` 内 `<meta name="pulse7-token" content="__PULSE7_TOKEN__">`
  由服务端替换为本次进程 token（见 `agent/api_server.go`）。
- token 缺失（占位符未替换）时自动启用内置演示后端（`src/api/mock.js`），用于本地开发与 VM 静态回测。
- 所有请求走 `Authorization: Bearer`；SSE 用 fetch 流读取 `/api/events`（`event:` + `data:` 包络）。

## 回测

宿主机 `npm run preview`（监听 0.0.0.0:4173）→ VM（192.168.140.128，Win7 SP1 + Chrome 109）Chrome 访问
`http://192.168.140.1:4173/` 验证渲染与交互；或 `npm run build` 后将 dist/ 放到任意静态服务。
