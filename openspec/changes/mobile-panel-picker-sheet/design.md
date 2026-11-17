# Design

## Context

`SessionSidePanel` 是终端页右侧栏的宿主，内部三个视图：`picker`（选择页）、`code`、`split`。`SessionTerminalStage.openPanels` 将 `codeShown` 置真并调用 `openPicker()`；`codeShown` 经 `codePanelState` 跨会话持久化，`panelView` 随页面存活。面板宽度小于 1024px 时（`desktop=false`），`mobile-code` 样式让面板整屏覆盖终端；视口窄于 640px 是应用的手机档位（`ResponsiveDialog`、`CodeFileActions` 均以此为 Dialog/Drawer 分界）。

## Decisions

### 选择页在手机上是独立浮层状态，而不是面板视图

手机端选择页渲染为底部 `Drawer`（vaul portal 到 body），由 `SessionTerminalStage` 的 `pickerOpen` 状态驱动，不写入 `panelView`：

- `openPanels` 分流：视口 <640px → `pickerOpen = true`；否则维持 `codeShown = true` + `openPicker()`。
- 抽屉取消（点遮罩、下滑、取消键）只关闭浮层，`panelView`/`codeShown` 未变，自然回到先前画面，也不需要记录返回视图。
- 选择项 → `panelView = "code" | "split"`、`codeShown = true`、关闭抽屉；侧栏持久化语义不变（只有真正展开侧栏才写 `shown`）。
- `panelView` 在手机上永远不取 `picker`，桌面的视图内选择页逻辑与焦点管理保持不变。

备选方案「`view="picker"` 仍在手机上取值、仅在渲染时换成抽屉」要求记录进入选择页前的视图以支持取消返回，且 `openPanels` 不能置 `codeShown`，状态组合更多；否决。

### 阈值按视口而非面板宽度

抽屉分界用 `MediaQuery("(max-width: 639px)")`（视口），不用 `desktop`（面板宽 ≥1024）。窄分屏窗格中面板 <1024 但页面仍是桌面宽度，全局底部抽屉不应出现；640–1023 视口（平板、窄窗口）维持整屏面板内的选择页。

### 返回与关闭的映射

- 分屏页「← 返回面板选择」改为 `onpicker` 回调：`SessionTerminalStage` 按视口决定重开抽屉或回到页内选择页。该回调与 `openPanels` 共用同一分流。
- 抽屉在 `codeShown` 时提供「关闭侧栏面板」行，走现有 `closePanel`（关闭 + 焦点回收入口按钮），补齐代码页在手机上缺失的关闭路径；`codeShown` 为假时不显示该行。
- 抽屉内当前面板视图行带 `aria-current`，使「我在哪一层」可见。

### 复用既有形态

抽屉结构沿用 `CodeFileActions` 的手机分支：`Drawer.Root` + `Drawer.Header` + `ListButton variant="action-sheet"` 行 + 底部取消按钮。选择项沿用同一批 `m.s4_*` 文案与处理函数，桌面选择页代码路径不变。

## Risks

- `panelView` 仍为 `"picker" | "code" | "split"`，手机路径不写入 `picker`；若未来有人直接赋值 `picker`，手机会回退到旧的整屏选择页——在注释中说明手机不取该值。
- 抽屉开着时 `+` 按钮被模态遮罩挡住，重复点按无副作用。
- `aria-expanded` 仍反映 `codeShown`；抽屉本身是模态对话框，语义独立。
