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

## Relay 页内配对与宿主流程（US-A10）

本次覆盖宿主权威授权流程、刷新与导航恢复、原位验证码操作、连接诊断及独立删除回执。
保留 v2 全新绑定与 v3 状态；配对成功保持公网关闭，当前浏览器路径信息移入次级「连接诊断」。

### 自动验证

- `go test -race ./internal/relay ./internal/relaysetting` 通过。
- `umask 0022; go test -race ./cmd/webmux ./internal/api ./internal/archtest` 通过。
- 默认 `umask 0077` 的首次整包运行触发既有配置迁移测试权限断言；标准权限环境通过，配置迁移实现未修改。
- `golangci-lint run ./cmd/webmux ./internal/api ./internal/relay ./internal/contract ./internal/archtest`：0 issues。
- 前端 `check`、`build`、`check:test`、`check:storybook`、`lint` 与源文件格式检查通过。
- `check:arch` 通过，架构测试 46 项；未放宽重复代码、依赖或样式基线。
- `relay-auth.test.ts` 与 `wire-contract.test.ts` 共 376 项通过，包含新增的 6 份 Relay wire 快照。
- `RelaySettings.test.ts` 浏览器组件 20 项通过，覆盖重复创建、导航恢复、断网恢复、明确终止、取消竞争与未知结果核对。
- `relay-settings.spec.ts` 桌面与 iPhone SE 共 6 项通过，覆盖刷新、导航、暂时查询失败、响应丢失、显式启用及删除关闭证据。

### 界面证据与边界

- Storybook 浅深色状态矩阵检查 112 组视口与状态组合，页面无横向溢出。
- 视口覆盖 1280×900、1280×720、768×800、390×844、375×667、320×568、667×375。
- 另检查展开诊断、危险操作与长错误文案，并刷新验证码有效期及旧访问警告的最终截图。
- 真实隔离嵌入应用的健康、初始配对阶段与操作回执均正确，未配对主区没有公网开关。
- 截图与交互录像位于本次会话交付物目录，使用隔离演示数据并保留桌面、平板、手机的浅深色前后对照。
- 本轮未代用户执行真实 GitHub 授权，未修改远端 Relay 或生产绑定。
- 无头浏览器未验证真实悬停、粗指针、iOS/Android 真机键盘及触控行为。

实现依据：[宿主控制器](../../cmd/webmux/relay_enrollment.go)、[页内设置](../../web/src/components/settings/RelaySettings.svelte)、[配对规格](../../openspec/specs/official-relay-pairing/spec.md)。
