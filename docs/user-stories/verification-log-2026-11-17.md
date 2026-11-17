# 核对记录：2026-11-17 起

本文记录 2026-11-17 起各次局部核对的原始记录，包括故事范围、执行过的验证、证据与限制。
此前的增量验证见 [2026-11-15 起记录](verification-log-2026-11-15.md)。

## 2026-11-17：固定壳层聊天输入的 IME 收起闩锁（US-Q05）

### 范围与根因

- 平板等固定壳层（窗格宽度不小于 1024px、主屏幕独立模式、分屏、代码栏打开）下，页面永不解除根文档滚动锁定，`keyboardDismissed` 闩锁在 IME 被观察收起后不再复位。
- 会话 chrome 随后把每一次新的 `chatEditing` 置位都视为待释放状态：点按消息框立即被 blur，输入框在收起一次键盘后失效；切换会话重挂页面（`{#key sessionId}` 重置 `useViewport`）后恢复，与报告一致。
- 修复为只在 `keyboardDismissed` 由 false 变 true 的当次观察沿上释放编辑状态；已观察过的闩锁不再拒绝之后的有意聚焦。US-Q05 补充验收要点，US-H02 的终端键盘开合行为不变。

### 回归证据

- 新增 `SessionChrome` 浏览器套件，以 stub 的会话 chrome 直接驱动 `keyboardDismissed` 与 `chatEditing`：闩锁标志下新聚焦不被释放、IME 重新开合后仍能再次释放、挂载前已置位的标志不影响首次聚焦；未修复的代码上两个用例均失败。
- 已运行 `SessionChrome`、`Viewport`、`TerminalTouch`、`TerminalHybrid` 浏览器用例全部通过；`pnpm check`、`pnpm lint`、`pnpm check:arch`、Oxfmt 与 `make docs-check` 通过。
- PR CI 首轮 `splits.spec.ts` 分屏用例因首字符输入竞态失败一次，与本改动无关，重跑通过。

### 限制

- 未做真实平板验证：无头浏览器无 `any-pointer: coarse` 与真实 IME 收起，收起路径以 `keyboardDismissed` 标志直接驱动；Android 返回键保留焦点与键盘按钮重新唤起路径未在真机复测。
