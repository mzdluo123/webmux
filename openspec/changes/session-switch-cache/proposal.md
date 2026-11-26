# Proposal

## Why

当前在浏览器中切换不同会话（终端或聊天），或从会话页切回首页再返回原会话时，路由层的 `{#key sessionId}` 会彻底卸载整个 `Terminal.svelte` 组件树。
卸载会销毁 xterm.js 实例、WebGL 上下文、聊天时间线模型、滚动位置与 WebSocket 连接；切回原会话时只能重新建连并触发全量回放或重拉历史，导致明显闪烁与等待。

## What Changes

- 在应用外壳内引入最近使用（LRU）会话页缓存池，保留近期访问的会话页实例与布局状态，切回已缓存会话时即时展示。
- 严格隔离前后台会话页的全局副作用（浏览器标题、快捷键、弹窗、焦点、文档滚动与分屏去重）。
- 引入 Active / Warm / Cool 三态后台生命周期：
  - Warm 态在后台通过现有增量协议持续吸收终端输出与聊天事件，切回即最新；
  - Cool 态挂起 WebSocket 连接并保留 `generation + offset` / `generation + seq`，切回时先展示已有画面再做增量补齐；
  - 后台会话在宽限期后释放 WebGL 上下文，切回前台按需恢复，避免触及浏览器 WebGL 上限。
- 结合全局 `/ws/sessions` 列表流，对 Cool 态但有新活动的缓存会话执行有界节流后台预刷新。
- 同步缓存已加载的 `TerminalFeatures` 模块快照，消除首次打开新会话时的异步首帧空白。

## Capabilities

### New Capabilities

- `session-switch-cache`：浏览器端会话切换的 LRU 缓存池、三态后台刷新与资源回收机制。

### Modified Capabilities

- `app-shell`：外壳主区域支持按 LRU 策略保活近期会话页，并在导航离开再返回时复用实例。

## Impact

- 前端视图与页面：`App.svelte`、`Terminal.svelte`、`SessionTerminal.svelte`、`features/terminal.ts`。
- 前端连接与缓存：`domains/sessions/session-cache.ts`、`hooks/shared/session-cache.svelte.ts`、`terminal-renderer.ts`、`terminal.svelte.ts`、`terminal-socket.svelte.ts`、`chat-socket.svelte.ts`、`hybrid.svelte.ts`、`ws.ts`、`ws-contracts.ts`、`perf.ts`。
- 后端：复用现有 `/ws/s/{id}`（`generation + offset`）与 `/ws/s/{id}/chat`（`generation + seq`）增量续接协议，无需改动服务端协议或持久化格式。
