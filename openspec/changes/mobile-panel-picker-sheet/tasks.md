# Tasks

## 1. 抽屉选择页组件

- [x] 1.1 新增 `SessionPanelPickerSheet.svelte`：`Drawer` 底部抽屉，渲染与桌面选择页相同的「代码」「分屏」`action-sheet` 行；`codeShown` 时追加「关闭侧栏面板」行；当前面板视图行 `aria-current`；无 kbd 提示。
- [x] 1.2 新增 `s4_panel_picker_hint` 键（中英双语），复用 `s4_open_panel`、`s4_panel_picker`、`s4_close_side_panel`、`s4_code`、`s4_split_pane`、`s4_cancel` 等现有键。

## 2. 分流与返回接线

- [x] 2.1 新增 `SessionPanelDock.svelte` 宿主：持有 `pickerOpen` 与视口查询，`openPanels()` 按视口分流（手机开抽屉、桌面展开侧栏选择页）；抽屉选择回调写入 `panelView` 与 `codeShown`；手机上 `view === "picker"` 归一化回上一真实视图，避免页面与抽屉同时挂载。`SessionTerminalStage.svelte` 委托给宿主。
- [x] 2.2 `SessionSidePanel.svelte`：分屏页「返回面板选择」改为 `onpicker` 回调，由宿主按视口决定重开抽屉或 `openPicker()`；`openPicker()` 与页内选择页保持桌面路径不变。

## 3. 测试迁移

- [x] 3.1 `web/test/e2e/fixtures.ts` 面板辅助方法区分两种选择页形态（`closePanelPicker` 先认抽屉「取消」、再认面板区「关闭侧栏面板」；`toggleCode` 用页面范围定位）；迁移 `core-only`、`compact-chrome` 的断言。
- [x] 3.2 `TerminalXrOwnership`、`FeatureHostTransitions` 浏览器测试不受影响（600px 视口路径不经过选择页），已通过回归。
- [x] 3.3 手机路径回归由定向 E2E 覆盖：抽屉取消不动作、「关闭侧栏面板」从代码页直达终端、分屏页返回重开抽屉、当前项勾选标记（截图验证）。

## 4. 文档

- [x] 4.1 更新 `docs/user-stories/terminal.md`（US-H08）、`terminal-splits.md`（US-D13）、`code-and-worktrees.md`（US-I01）中手机选择页叙述。
- [x] 4.2 更新 `docs/usage/terminal.md`、`docs/usage/code.md` 手机操作描述。

## 5. 验证

- [x] 5.1 并行执行 `pnpm --dir web check`、`lint`、`check:arch`、`vp fmt --check` 与受影响的 Vitest/Playwright 用例；140 项定向 E2E 初跑暴露的同名按钮严格模式冲突已修复并复测通过。
- [x] 5.2 已实拍 390×844、375×667、320×568 手机、820×1180 平板、1280×720 桌面与明暗主题截图及交互 GIF，存入 `.github/pr-assets/mobile-panel-picker-sheet/` 供 PR 证据。
