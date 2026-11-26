# Tasks

## 1. 领域模型与缓存池基础设施

- [x] 1.1 新增 `web/src/domains/sessions/session-cache.ts`：实现纯 LRU 队列、桌面/移动容量计算、Active/Warm/Cool 三态分配、TTL 淘汰与后台预刷新挑选逻辑
- [x] 1.2 为 `session-cache.ts` 补充单元测试（LRU 命中与淘汰、容量上限、分屏驱逐、三态分配、TTL 清理、后台刷新节流）
- [x] 1.3 新增 `web/src/hooks/shared/session-cache.svelte.ts`：实现响应式缓存控制器，联动路由、全局会话列表、归档信号、`visibilitychange` 收缩与后台预刷新队列

## 2. 会话页缓存挂载与前后台副作用隔离

- [x] 2.1 在 `web/src/features/terminal.ts` 中为 `loadTerminalFeatures()` 增加模块级同步缓存与重置入口，消除冷启动首帧空白
- [x] 2.2 改造 `web/src/App.svelte`：在 `AppShell` 主区域使用缓存池 `{#each}` 渲染 `Terminal.svelte`，替代 `{#key sessionId}`，处理隐藏容器样式（`inert` / `aria-hidden`）、分屏去重与认证/重建清空
- [x] 2.3 改造 `web/src/pages/Terminal.svelte` 与 `web/src/components/terminal/SessionTerminal.svelte`：支持 `pageActive` 与 `cacheTier`，严格门控浏览器标题、`activePane`、全局快捷键、弹窗、`documentScroll`、焦点恢复与 `fit()`

## 3. WebGL 延迟回收与 WebSocket 三态挂起/续接

- [x] 3.1 改造 `web/src/hooks/terminal/terminal-renderer.ts`：增加 `setActive(active)`，后台宽限期后释放 `WebglAddon` 并暂停光标闪烁，切回前台自动恢复
- [x] 3.2 改造 `web/src/platform/ws-contracts.ts` 与 `web/src/platform/ws.ts`：为 `SocketConnection` 增加 `suspend()` 与 `resume()`，挂起不进入重连退避，恢复时按最新路径参数建连
- [x] 3.3 改造 `web/src/hooks/terminal/terminal-socket.svelte.ts` 与 `web/src/hooks/terminal/terminal.svelte.ts`：区分整页隐藏与 SPA 后台保活，支持 Warm 态持续吸收（抑制 resize/chrome）与 Cool 态挂起/增量续接
- [x] 3.4 改造 `web/src/hooks/chat/chat-socket.svelte.ts` 与 `web/src/hooks/sessions/hybrid.svelte.ts`：支持聊天流 `suspend()` / `resume()`，Cool 态保留 `ChatModel` 与 `seq` 并在切回或后台预刷新时增量补齐
- [x] 3.5 在 `web/src/platform/perf.ts` 中增加会话切换缓存命中与耗时埋点

## 4. 测试、用户故事与验证

- [x] 4.1 补充前端单元/组件测试：覆盖 WS `suspend`/`resume`、终端/聊天三态切换、缓存控制器驱逐与后台刷新
- [x] 4.2 补充 Playwright E2E 测试：验证两个会话间切换保留状态与滚动位、后台输出吸收、返回首页再切回不重连全量回放
- [x] 4.3 更新 `docs/user-stories/` 受影响的用户故事与最新核对记录
- [x] 4.4 运行 `pnpm --dir web check:arch`、`make check`、`make test` 并修复所有问题，提交 PR 并跟踪 CI 全部通过
