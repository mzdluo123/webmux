## Context

动机和实测数据见 proposal.md 的 Why 部分；行为契约见各 delta spec。与实现相关的现状如下：

- 终端顶栏在 `web/src/pages/Terminal.svelte`。页面已有两套宽度判断：JS 中的 `desktop`（`min-width:1024px`，决定左右分栏还是“终端 / 代码”切换），以及 `styles.css` 中的 `(any-pointer: coarse), (max-width: 639px)`（控件触控尺寸）。快捷指令入口的显隐完全由 CSS 决定（`.quick-commands-header`）。
- 代码区由 `CodePanel.svelte`（工作区栏、标签栏）、`CodeNavigator.svelte`、`CodeDocumentView.svelte`（文档栏、操作栏）、`CodeDiffReader.svelte`（Diff 工具栏、选择提示）和 `CodeImageView.svelte`（图片工具栏）组成。代码区可以弹到独立窗口（Document Picture-in-Picture 或普通弹窗），这时适用该窗口自己的视口宽度。
- `ui/` 下已有 `dropdown-menu`、`tabs`、`drawer`、`popover` 等 shadcn-svelte 组件，没有 toggle-group。
- `terminal.svelte.ts` 只暴露 `scrollToBottom()`，不暴露当前是否位于底部。
- 已有的 iOS 与主屏幕 Web App 适配：`.terminal-page` 是固定定位容器，高度取 `visualViewport` 高度（`viewport.svelte.ts`），四边用 `env(safe-area-inset-*)` 留出安全区；`display-mode: standalone` 下给状态栏区域铺上顶栏背景色，以避开 iOS 26 起顶部约 40pt 的模糊层。`toggleFullscreen` 在 Fullscreen API 不可用时已经显示“添加到主屏幕”的提示。`popoutAvailable()` 的条件是“有 Document PiP，或有精细指针”。软键栏用 `onmousedown` 调 `preventDefault` 保住焦点，避免收起键盘。
- 兼容基线为 iOS/iPadOS 27 上的 Safari（WebKit 27）及其主屏幕 Web App。按 firt.dev 的 iOS PWA 兼容表（2026-10-13 更新）：iPhone 不支持 Fullscreen API（只有 iPad 支持），manifest 的 `display: fullscreen` 会回退为 `standalone`，没有 Virtual Keyboard API。现有代码已按 WebKit 不支持 Document Picture-in-Picture 处理（`codePopout.ts` 走 `window.open` 回退）。

## Goals / Non-Goals

**Goals:**

- 窄屏的显隐只用 CSS 断点（Tailwind 的 `max-sm:` / `sm:`，即 640 px）实现，不新增 JS 布局状态，避免 SSR 或首帧闪烁，也与现有 `.quick-commands-header` 规则一致。
- 菜单项与原按钮共用同一个处理函数和禁用条件，不复制逻辑。
- 宽屏终端顶栏的按钮集合与顺序不变。

**Non-Goals:**

- 不调整 1024 px 分栏断点，不改变 640–1023 px 平板宽度的“终端 / 代码”切换行为。
- 不改动代码导航面板内部的搜索、过滤控件。
- 不引入新的图标库或 UI 依赖。

## Decisions

### 1. 窄屏断点用 640 px，不按指针类型判断

顶栏收纳与否取决于可用宽度，而不是输入方式：横屏手机、带触屏的笔记本宽度都够，不应该被收纳。640 px 与 `docs/design-system.md` 中“小于 640px 的窗口”控件规则一致。图片的放大/缩小按钮是例外：只有在没有双指缩放替代时才必须保留，所以按“窄屏且粗指针”判断（`max-sm:` 叠加 `[@media(any-pointer:coarse)]:`）。

备选：沿用 `(any-pointer: coarse)`。会让宽屏触屏设备也丢失顶栏按钮，不采用。

### 2. 同一份标记，按断点切换按钮与菜单项

顶栏中的全屏、上传、滚到底按钮加 `max-sm:hidden`；终端菜单中新增一组对应菜单项，外层加 `sm:hidden`，并在其后放一条同样只在窄屏显示的分隔线。两处调用同一函数（`toggleFullscreen`、`fileInput.click()`、滚到底逻辑），禁用条件抽成 `$derived` 后共享。

全屏入口另有一个能力条件 `fullscreenOffered = fullscreenSupported || !standalone`：其中 `fullscreenSupported` 沿用 `toggleFullscreen` 现有的检测（`document.fullscreenEnabled` 且 `requestFullscreen` 为函数），`standalone` 为 `matchMedia("(display-mode: standalone)")`。这两项在页面运行期间不变，挂载时计算一次即可。条件为假时，按钮和菜单项都不渲染。Safari 标签页中不支持全屏时保留入口，复用现有提示文字。

备选：用 JS 媒体查询只渲染其中一种。需要新增状态并处理变更事件，与快捷指令入口的现有做法不一致，不采用。

### 3. 连接状态并入状态点

“已连接”文字只在 `t.connection !== "connected"` 时渲染，并加 `whitespace-nowrap`。状态点的 `aria-label` / `title` 改为 `${sessionStatusText(t.info)} · ${连接状态文字}`。会话信息尚未加载、没有状态点时，仍单独显示连接文字，保证状态可见。

### 4. “回到最新”按钮的状态来源

在 `terminal.svelte.ts` 中新增 `$state` 字段 `atBottom`，在 xterm 的 `onScroll` 和 `onWriteParsed` 回调中按 `buffer.active.viewportY >= buffer.active.baseY` 更新，并通过返回对象的 getter 暴露。触屏滚动经 `touch.ts` 调用 xterm 的滚动接口，同样会触发 `onScroll`。`Terminal.svelte` 在 `.terminal-area` 内绝对定位一个带图标和“最新”文字的 `Button`（`sm:hidden`），显示条件为 `!t.atBottom && 终端视图可见`。点击调用既有 `t.scrollToBottom()`；按钮沿用软键栏的 `onmousedown` → `preventDefault` 做法，不抢 xterm 焦点，也不收起已弹出的键盘。`.terminal-area` 位于已扣除安全区、高度跟随 `visualViewport` 的 `.terminal-page` 内，所以按钮自然在键盘上方，也不会落到主屏幕指示条下面，不需要额外的 inset 计算。观察端平移浏览时同样适用，不影响控制权。

备选：监听 `.xterm-viewport` 的 DOM scroll 事件。依赖 xterm 内部 DOM 结构，不采用。

### 5. 分段视图控件复用 `ui/tabs` 的样式与语义

“文件 / Diff”“Diff / 旧侧 / 新侧”“预览 / 源码”是互斥的视图选择，使用 `Tabs.List` + `Tabs.Trigger`（只用列表部分，内容区仍由 `CodeDocumentView` 现有分支渲染），获得 `role="tablist"`、`aria-selected` 和方向键导航。当前 Diff 处于完整文件状态时，由现有 `shownDiff` / `mode` 状态推导选中项，切换仍调用现有 `compare()`、`full(side)`、`showSvg()`，选区保留规则不变。

备选：通过 shadcn CLI 新增 toggle-group。语义上 tabs 也匹配“切换视图”，而且组件已经存在，不为此新增组件。

### 6. 窄屏代码区菜单

`CodePanel` 标签行最右侧新增一个 ⋯ `DropdownMenu`（`sm:hidden`），菜单内容由当前激活的 `CodeDocumentView` 通过 snippet 或回调提供，避免 `CodePanel` 了解文档内部状态。JS 无法直接读取 `env()`，所以在 `:root` 上定义 `--safe-top: env(safe-area-inset-top)`、`--safe-bottom: env(safe-area-inset-bottom)`，打开菜单时用 `getComputedStyle` 读出数值，再加到 `collisionPadding` 的上下两边。`max-height` 取 bits-ui 提供的可用高度变量，超出时在菜单内滚动，保证在主屏幕 Web App 和软键盘弹出时都完整可见。菜单项包括：

- `DropdownMenu.Label`：文档完整路径（`break-all`）。
- `DropdownMenu.RadioGroup`：视图（文件 / Diff 或 Diff / 旧侧 / 新侧；SVG 为预览 / 源码）。
- `DropdownMenu.RadioGroup`：引用来源或预览来源（仅 Diff）。
- Diff 显示开关（行号、换行等），以 `CheckboxItem` 呈现，由 `CodeDiffReader` 暴露现有状态。
- 刷新文档、刷新全部。

宽屏仍显示单行文档栏与完整 Diff 工具栏，二者同源于同一份状态。

备选：使用底部 `drawer`。选项多时更像设置面板，会额外占用一层交互；下拉菜单与终端菜单一致，不采用。

### 7. 工作区栏与标签栏合并

`CodePanel` 的 `<header>` 与标签列表合并为一个 flex 行：导航开关、`min-w-0 flex-1 overflow-x-auto` 的标签条、刷新全部、独立窗口（`max-sm:hidden`），窄屏再加代码区菜单。根路径移到 `CodeNavigator` 顶部作为标题，同时拼入刷新全部按钮的 `title`。导航状态异常（`navStatus`）时的提示与“刷新导航”按钮移到导航面板顶部，不再占用标签行。

### 8. 操作栏

`CodeDocumentView` 的操作栏改为 `flex` 单行：宽屏左侧放从 `CodeDiffReader` / `CodeReader` 提出的选择提示（`truncate`，`max-sm:hidden`），右侧放按钮。“引用整个文件”以 `start !== undefined` 为渲染条件。原 `CodeDiffReader` / `CodeReader` 底部的提示行移除，避免两处重复。

### 9. iOS 27 与主屏幕 Web App 的限制

| 限制 | 处理 |
|---|---|
| iPhone 没有 Fullscreen API | 见决策 2：Safari 标签页保留入口并提示添加到主屏幕；主屏幕 Web App 隐藏入口 |
| 主屏幕 Web App 没有地址栏和后退按钮 | 返回会话列表按钮在任何宽度都常驻；终端菜单保留“重新同步”，代替浏览器的刷新 |
| 没有 Document PiP；主屏幕 Web App 中 `window.open` 会打开 App 内浏览器页面（推断，需真机确认） | `popoutAvailable()` 改为 `documentPiP \|\| (pointer: fine && !standalone)` |
| 触屏没有悬停提示 | 浮动按钮带可见文字；纯图标控件保留 `aria-label`，tooltip 只作为宽屏补充 |
| 字号小于 16px 的表单控件在聚焦时会让页面放大 | 窄屏来源选择改为菜单单选项；宽屏 `<select>` 不变 |
| 没有 Virtual Keyboard API，键盘高度只能从 `visualViewport` 得到 | 浮动按钮和菜单都位于跟随 `visualViewport` 的容器内，不自行估算键盘高度 |
| 状态栏使用 black-translucent，iOS 26 起顶部有模糊层 | 顶栏仍是 `.terminal-page` 的第一个子元素，沿用现有安全区留白与背景色，不在顶栏上方插入任何内容 |
| 带触控板的 iPad 同时匹配 `any-pointer: coarse` 和 `pointer: fine` | 图片 ＋/− 只在“窄屏且粗指针”时隐藏，触控板捏合仍按 Ctrl+滚轮缩放 |

兼容基线是 iOS 27，可以直接使用 `:has()`、`dvh`、`field-sizing` 等现有代码已在用的特性，不为更早的 iOS 写降级代码。

## Risks / Trade-offs

- [窄屏菜单让低频操作多一次点击] → 只收纳实测低频的操作；滚到底这类高频操作有浮动按钮补偿。
- [现有 E2E / 浏览器测试依赖按钮名称与位置] → 同一变更中更新选择器，新增 320/393/1440 px 布局断言；菜单项保留原可访问名称，减少改动面。
- [`Tabs` 用作视图切换时，读屏软件会把它读作“标签页”] → 语义可接受（切换的是同一文档的不同视图）；`aria-label` 分别为“文件视图”“Diff 视图”。
- [独立窗口宽度小于 640 px 时也会套用窄屏收纳] → 这与窗口实际可用空间一致，可以接受；独立窗口入口本身在窄屏主页面不显示。
- [`code-image-preview` 尚未归档，本变更修改其要求] → 归档顺序必须是先 `add-code-image-preview`，再本变更；tasks 中写明。
- [移动端 Chromium 模拟不能替代真机，Playwright 的 `emulateMedia` 也不能模拟 `display-mode: standalone`] → 能力条件（`fullscreenOffered`、`popoutAvailable`）抽成可注入的纯函数，用单元测试覆盖各种组合；真机在 iOS 27 iPhone 的 Safari 标签页与主屏幕 Web App、iPadOS 27（含触控板）和 Android Chrome 上，检查浮动按钮与键盘、菜单安全区、全屏入口、独立窗口入口和图片双指缩放。
- [iOS 26 起 Safari 标签页底部的浮动工具栏可能盖住页面底部] → 浮动按钮位于软键栏上方，不比现有软键栏更靠下；真机确认，如有遮挡再按 `visualViewport` 调整（推断，尚未验证）。

## Migration Plan

纯前端变更，随下一次构建发布，无数据或配置迁移。回滚即还原前端提交。
