# Design

## Context

WebMux 后端已经为终端流 `/ws/s/{id}` 提供 `generation + offset` 字节增量续接，为聊天流 `/ws/s/{id}/chat` 提供 `generation + seq` 事件增量续接。
前端在 [App.svelte](../../../web/src/App.svelte) 中通过 `{#if sessionId}` 与 `{#key sessionId}` 渲染 [Terminal.svelte](../../../web/src/pages/Terminal.svelte)，导致任何路由切换都会销毁整个会话页树，前端无法复用已有 xterm 缓冲、聊天时间线与增量续接位点。

## Goals / Non-Goals

### Goals

- 切回近期访问的终端或聊天会话时直接展示原实例，保留滚动位置、查找、分屏布局、代码面板与输入草稿。
- 后台会话在受控资源预算内持续保持最新或切回时瞬间增量补齐，不触发全量清屏回放。
- 严格控制 WebGL 上下文数量、后台 WebSocket 数量与浏览器内存占用，兼顾桌面端与移动端。
- 隔离后台缓存会话的全局副作用（标题、快捷键、弹窗、焦点、PTY 尺寸、控制权）。

### Non-Goals

- 不跨浏览器刷新（F5）或跨标签页持久化 xterm 内存缓冲区。
- 不修改服务端 WebSocket 帧格式、认证或控制权协议。
- 不改变单个会话页内既有的分屏（`MAX_PANES`）与 DockWorkspace 交互语义。

## Decisions

### 1. 分层与模块归属

遵循前端架构分层（[docs/frontend-architecture.md](../../../docs/frontend-architecture.md)）：

- `web/src/domains/sessions/session-cache.ts`：纯状态机与算法，包含 LRU 队列操作、设备容量计算、Active / Warm / Cool 三态分级规则、空闲 TTL 淘汰与后台预刷新队列挑选。零 Svelte/DOM 依赖。
- `web/src/hooks/shared/session-cache.svelte.ts`：响应式控制器，跟踪路由与会话列表变化、驱动 Warm→Cool 降级定时器与 TTL 清理、响应 `document.hidden` 收紧策略、调度 Cool 态后台增量预刷新。
- `web/src/App.svelte`：在 `AppShell` 主区域中以 `{#each}` 渲染缓存池中的 `Terminal.svelte`，透传 `pageActive` 与 `cacheTier`（`"active" | "warm" | "cool"`），并处理分屏去重与认证/重建清空。

### 2. 缓存容量与显隐方式

- 容量上限：桌面端默认保活 5 个会话页（其中最多 2 个 Warm 后台热保活，其余为 Cool 挂起）；触控/窄屏默认保活 3 个会话页（其中最多 1 个 Warm）。
- 显隐方式：非活跃缓存页包裹在独立容器中，使用 CSS 类设置 `position: absolute; inset: 0; visibility: hidden; pointer-events: none; z-index: -1; content-visibility: hidden;` 并添加 `inert` 与 `aria-hidden="true"`。这样既跳过浏览器光栅化绘制，又保留 DOM 盒模型与滚动位置。
- 生命周期清理：
  - 会话从活跃列表消失且被归档/关闭时，立即从缓存池驱逐（除非正在前台展示恢复态）。
  - 会话 B 若被加入当前活跃页的分屏中，先从独立缓存池中驱逐 B，保证同一 `sessionId` 全局至多存在一个活跃 `SessionTerminal`。
  - 后台空闲超过 15 分钟自动淘汰；浏览器整页隐藏超过 5 分钟收缩至仅保留当前活跃会话。

### 3. 三态连接与 WebGL 回收

- **Socket 挂起/恢复**：在 `SocketConnection`（`ws-contracts.ts` / `ws.ts`）新增 `suspend()` 与 `resume()`。`suspend()` 主动关闭底层传输并停止自动重连退避；`resume()` 按最新路径函数（携带当前 `generation + offset` 或 `generation + seq`）重新打开流。
- **终端三态行为**：
  - `active`：实时写入、允许 resize 与控制、启用 WebGL 与光标闪烁。
  - `warm`：保持 WS 连接并继续写入 xterm 推进 `offset`（不因 `!pageActive` 丢帧导致 4408），但抑制 `sendSize()`、`reportScheme()`、`setBrowserChrome()` 与自动抢焦点；后台宽限期（8 秒）后释放 `WebglAddon` 并关闭 `cursorBlink`。
  - `cool`：调用 `ws.suspend()`，释放 `WebglAddon`，保留 xterm 缓冲区与 `synced / generation / offset`；切回前台时调用 `ws.resume()` 原地增量续接。
- **聊天三态行为**：
  - `active` / `warm`：保持 `/ws/s/{id}/chat` 连接，继续执行 `applyFrame` 更新 `ChatModel`；后台抑制自动滚动测量与焦点操作。
  - `cool`：调用 `chat.suspend()`，保留 `ChatModel` 与 `state`；切回前台时调用 `chat.resume()` 携带 `generation + seq` 增量追平。

### 4. Cool 态后台预刷新

- 监听全局 `/ws/sessions` 列表快照中的会话活跃信号（`status` / `attention` / `title` / `updatedAt` 变化）。
- 当某 Cool 态缓存会话出现新活动且页面可见时，将其加入有界串行预刷新队列（并发 1、防抖 3 秒、单会话最小间隔 20 秒）：短暂 `resume()` 追平增量后再回到 `cool`（或当 Warm 槽位空闲时提升为 `warm`）。

### 5. 首帧优化与性能观测

- 在 `web/src/features/terminal.ts` 中对 `loadTerminalFeatures()` 的成功结果做模块级同步缓存（能力重建时重置），使未命中缓存的新会话也能首帧同步渲染。
- 在 `web/src/platform/perf.ts` 中记录会话切换命中情况（`hit-warm` / `hit-cool` / `miss`）与切回耗时，便于观测优化效果。

## Risks / Trade-offs

- **风险**：隐藏容器若未彻底门控全局事件，可能导致后台会话响应 `Ctrl+B`、抢占焦点或覆盖浏览器标题。
  - **缓解**：通过 `pageActive` 统一门控 `Terminal.svelte` 和 `SessionTerminal.svelte` 的全局监听器、弹窗、标题、`activePane` 和 `documentScroll`，并补充单元测试与 E2E 验证。
- **风险**：移动端内存较紧，缓存过多带大 scrollback 的终端可能增加内存压力。
  - **缓解**：移动端默认容量收紧为 3（仅 1 个 Warm），切走即进入宽限期释放 WebGL，后台 5 分钟自动收缩缓存。
