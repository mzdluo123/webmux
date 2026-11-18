# Proposal

## Why

会话页「打开面板」把代码与分屏收进统一的右侧栏选择页（#630），该选择页按桌面侧栏设计：居中标题、两行按钮和 C/S 键盘提示。手机上它被整屏放大后变成一个几乎空白的中间页——进入代码页必须经它中转，代码页内没有可见的关闭路径，kbd 提示在触屏上没有可对应的按键，取消选择也没有落点。

应用已有手机端惯例：`CodeFileActions`、`AppShell` 等在同一选择语义下，视口窄于 640px 时以底部抽屉呈现操作面板。选择页需要沿用这一惯例，而不是为手机单独发明第二种入口。

## What Changes

- 视口宽度小于 640 CSS px 时，「打开面板」弹出底部抽屉选择页，终端或当前面板保留在遮罩后方；取消抽屉不动作，回到先前画面。
- 抽屉在选择项基础上追加「关闭侧栏面板」项（侧栏已打开时），并标记当前所在面板。
- 手机端分屏页「返回面板选择」重新打开抽屉；桌面端行为与布局完全不变。
- 选择页键盘 C/S 与 kbd 提示只保留在桌面侧栏选择页中。
- 同步受影响的用户故事与使用文档，迁移依赖「整屏选择页」的手机测试路径。

## Capabilities

### New Capabilities

- `terminal-side-panel`：右侧栏面板的入口、选择页形态（桌面页内 / 手机抽屉）、面板视图切换与关闭路径。此前无对应规范（#630 未走 OpenSpec），本规格完整记录改后行为。

### Modified Capabilities

- 无。`terminal-header` 只规定顶栏常驻项，选择页形态属于本规格。

## Impact

- `web/src/components/terminal/SessionSidePanel.svelte`：分屏页「返回面板选择」改为回调；选择页在手机上不再是一个面板视图。
- `web/src/components/terminal/SessionTerminalStage.svelte`：`openPanels` 按视口分流；新增抽屉状态与选择回调。
- 新增 `web/src/components/terminal/SessionPanelPickerSheet.svelte`：底部抽屉选择页，复用 `ui/drawer` 与 `ListButton action-sheet`。
- i18n：新增少量抽屉文案键；复用 `s4_open_panel`、`s4_panel_picker`、`s4_close_side_panel` 等现有键。
- 测试：`web/test/e2e/fixtures.ts` 面板辅助方法、`app-shell`、`core-only`、`splits`、`hybrid`、`compact-chrome`、`TerminalXrOwnership`、`FeatureHostTransitions` 的手机路径。
- 文档：`docs/user-stories/terminal.md`、`terminal-splits.md`、`code-and-worktrees.md`、`docs/usage/terminal.md`、`docs/usage/code.md`。
