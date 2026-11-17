# WebMux Design System

WebMux 的 UI 从使用者当前的任务出发：让人看懂发生了什么、找到下一步，并顺手完成工作。
界面形态、信息密度和视觉表现由任务决定。组件与令牌支撑实现，Storybook 提供可交互的验证入口。

## Design principles

1. **先确定用户目的。** 写清使用者带着什么上下文进入、最常做的动作、完成后的可见结果。根据真实任务决定页面结构、默认状态和入口。
2. **为任务选择交互形态。** 终端围绕操作程序，Git 围绕理解和提交变更，消息围绕阅读与交流，设置围绕修改偏好。API 资源和数据库字段用于实现，页面按用户工作过程组织。
3. **打开即可继续。** 优先呈现正在工作的对象与内容，在确有数据支持时恢复选择和阅读位置。首次使用的空状态提供明确下一步，加载状态保留最终布局。
4. **让下一步容易发现。** 高频动作有可见名称、符合习惯的位置与明确反馈。创建、配置和高级选项在用户需要时展开；常驻信息和操作的数量取决于任务频率、发现成本与空间。
5. **在对象附近行动。** 频道承担导航，消息承担交流上下文，Diff 承担变更审阅。回复、复制、重试等操作贴近对象，作用范围与当前位置清楚对应。
6. **视觉帮助理解。** 通过排版、头像、图标、颜色、表面和适当的动效建立身份、关系、层级与状态。视觉表现可以丰富；每项设计说明它帮助用户理解的内容或完成的动作。
7. **反馈与恢复连续。** 成功结果优先体现在内容变化中；发送、失败与重试跟随对应对象。保留草稿和当前上下文，错误同时说明发生了什么与下一步，危险操作说明后果。
8. **设备各有自然路径。** 桌面可同时展示导航和内容，手机按列表、详情、返回组织。触控、键盘、输入法、软键盘及减少动态效果属于完整交互，验证真实任务能否完成。
9. **展示已有事实。** 未读、在线、参与者、送达和成功状态都有明确数据依据；接口尚未提供的能力在设计中标明范围，界面通过真实反馈建立信任。

## Design workflow

开始实现前，用几句话确定：用户目的、主要对象、进入后的默认画面、最短任务路径、成功与失败反馈，以及手机上的操作顺序。
比较布局方案时，以理解成本、发现成本、完成任务的步骤和上下文连续性为依据。需求已明确时直接选择并实现。
复用顺序为 `ui/` 基础组件、现有业务复合组件、shadcn-svelte registry；复用保证熟悉的交互语义，必要时扩展设计系统以适应真实任务。

以频道消息为例：桌面使用频道导航、消息时间线和持续可达的输入区；手机先选频道再进入对话。创建频道从命名入口发起，完成后直接进入新频道。
频道名称、搜索、发送、加载历史与错误恢复服务于阅读和交流。字节限制接近上限时再提示，精确时间与技术细节按需查看。

## Interaction foundations

- 字号按阅读角色组织：12px 元数据、14px 正文与常规操作、16px 分区标题、24–30px 页面标题。手机输入使用至少 16px，保持输入法与系统缩放体验。
- 状态色同时提供文字或可访问名称：绿色为成功/运行，琥珀色为等待/注意，红色为阻塞/失败/危险，蓝色为说明，中性色为空闲。业务状态由真实数据决定。
- 按钮沿用已有语义：`default` 强调主要动作，`outline`、`ghost`、`quiet` 根据可发现性和注意力分配承载其他动作；危险动作使用 `destructive` 或 `destructive-outline`。
- `aria-pressed` 表达开关，`aria-expanded` 表达展开，`aria-current` 表达当前位置；可访问状态与可见状态同步。Tabs 用于关联面板，SegmentedControl 用于单值选择。
- 主导航与常规触控目标使用 44px；终端紧凑顶栏保留 32px，设置、代码等高密度区域可使用现有 36px 档位。同排图标按钮使用一致的尺寸。
- 选中项要能快速辨认，样式在对应设计系统组件中定义。信息、错误和长文本允许换行；工具栏根据任务优先级与视口安排。
- 动效解释状态或空间变化，并尊重减少动态效果；焦点清楚可见。组件复用、令牌、前端架构与验证细则见[前端架构](frontend-architecture.md)与[UI skill](../.agents/skills/webmux-ui/SKILL.md)。

## Tokens

唯一令牌入口是 `web/src/design-tokens.css`，由应用的 `styles.css` 引入，Storybook 直接使用同一份样式。

| 层        | 内容                                                                                       | 修改规则                               |
| --------- | ------------------------------------------------------------------------------------------ | -------------------------------------- |
| Primitive | neutral、green、amber、blue 色阶，control 尺寸                                             | 定义原始值，避免散落到业务组件         |
| Semantic  | background、card、popover、foreground、primary、muted、success、warning、destructive、info | 根据用途引用；明暗主题覆盖相同角色     |
| Component | control-xs/sm/default/lg/touch，Tailwind radius-sm/md/lg/xl                                | 控件直接消费尺寸和圆角，页面只控制布局 |

- Git 工作台复用 background、card、popover、border 与 primary；列表选中使用浅主题底色与左侧细线。

- 间距使用 Tailwind 的 4px 基准：4 / 8 / 12 / 16 / 24 / 32。
- 圆角按角色取值，只使用 `--radius` 派生的 Tailwind 刻度，由 `arch:test` 检查：
  - `rounded-sm`（4px）：标签、徽标、键帽、附件条、复选框，以及菜单、列表、文件树中的行。
  - `rounded-md`（6px）：按钮、输入框、选择器、导航项等控件。
  - `rounded-lg`（8px）：卡片、会话行、提示、聊天气泡与面板、菜单和弹出框、空状态、骨架屏等内容容器。
  - `rounded-xl`（12px）：只用于 Dialog、AlertDialog 与 Drawer 的弹层表面。
  - `rounded-full` 用于圆点、开关、滑块与滚动条；`rounded-none` 用于贴边的行与分段；`rounded-[inherit]` 只用于跟随父级圆角的内层。
  - 不使用裸 `rounded`、`rounded-xs` 或 `rounded-2xl` 及以上；CSS 中的 `border-radius` 只写 `var(--radius-*)`、`0`、`50%`、`999px` 或 `inherit`。
- 默认控件高 36px，紧凑 32px，工具 28px，大控件 44px。信息密集页面（设置、工作区、代码）在触屏或小于 640px 时，常规按钮与图标按钮用紧凑触控高度 36px（`--control-touch-compact`），由 `.settings-layout`、`.workspace-page`、`.code-panel` 统一设置；页面沿用该统一尺寸规则。
- 拉丁界面字体使用内置 Inter，界面中日韩文字使用系统无衬线字体；终端和代码使用内置 Maple Mono NF CN（含拉丁字形、制表符、Nerd Font 图标及按 `unicode-range` 切片的中文）。终端标题里的 Nerd Font 私有区图标在界面中回退到同一套内置图标分片，其余界面文字仍使用 Inter 与系统字体。
- ANSI 终端和语法高亮令牌保持独立；业务状态不再借用 ANSI 色板。
- 焦点使用 primary 对应的 ring；不依赖 hover 提供唯一反馈。

## Component system

继续使用 `web/src/lib/components/ui/` 中的 shadcn-svelte 与 bits-ui 组件，保留键盘导航、焦点管理和 ARIA 行为。修改生成组件后，重新运行生成器时需要审查本地差异。这些约定由 [前端架构守卫](frontend-architecture.md) 检查，现有的 variant、size 与例外契约也列在其中。

| 层级       | 组件                                                             | 约定                                                 |
| ---------- | ---------------------------------------------------------------- | ---------------------------------------------------- |
| 基础操作   | Button、Badge、Input、Textarea、Label、Switch、Select            | 使用 variant 和 size；不以页面选择器覆盖全体按钮外观 |
| 容器和导航 | Card、Item、Tabs、DropdownMenu、Popover                          | 内容层使用边界和留白；浮层使用独立表面和阴影         |
| 弹层       | Dialog、AlertDialog、Drawer                                      | 共享文字与形状规范；危险菜单项保留红色               |
| 公共反馈   | InlineNotice、StripNotice、InfoHint                              | 位于 `ui/`，统一提示语义与按需说明                   |
| 公共组合   | ResponsiveDialog、PickerField、AdvancedSection、KeyValueSummary  | 位于 `ui/`，通过 props、snippet 和回调组合基础控件   |
| 业务模式   | SessionStatus、WorkspaceGroup、SessionRow、TerminalInputComposer | 位于 `components/`，负责领域状态和业务操作           |

`SessionStatus` 区分需立即处理的提醒（需要回答、等待授权、出错停止）与普通等待，以及正常退出与非零退出码。`InlineNotice` 的 danger 使用 `role="alert"`，其他提示使用 `role="status"`；常规静态说明可继续使用普通段落，避免过多实时播报。

表单标签应使用 `for` 关联控件；帮助和错误使用 `aria-describedby`，无效字段设置 `aria-invalid`。字段在手机上使用 16px 文字，避免阅读困难和输入缩放。图标按钮需要可访问名称。

长路径可在列表中截断，但应保留完整可访问名称或 `title`。错误描述、警告、表单标签必须换行。终端预览、代码视图和单行输入的内部滚动/截断属于明确的产品行为。

- 多信息选择器使用 `Select`：
  - 启动配置显示图标、名称与等宽命令摘要，仓库和工作树显示名称与路径摘要。
  - 触发器和展开选项使用相同主题、焦点环与选中标记，长内容保留完整提示。
  - 使用键盘方向键、Enter 和 Escape 选择或关闭。
- 多选一按语义选组件，两者都用下划线标出当前项：
  - 切换带关联面板的内容区域（如 Tailscale 模式、会话视图、代码根）用 `Tabs.List variant="line"`；隐藏面板不能影响当前方式的校验或提交。
  - 只选一个值（如分屏方向、主题、状态筛选、显示方式）用 `SegmentedControl`：基于 RadioGroup（`role="radiogroup"` 与 `radio`），方向键切换，已选项不能被再次点击清空。
  - 不用一组普通按钮模拟分段控件。每个选项都需要说明文字时用 `RadioGroup`（如新建会话的起始端）：选项卡片带标题与说明，边框标出当前项。

## Motion

聊天与快速开始共用输入卡片高度动画，发送按钮同步移动；文字不缩放，
重复操作从当前高度继续，窗口尺寸变化或减少动态效果时取消并恢复自然布局。

会话选项使用 `Drawer.Content motion="origin"`：轮廓从齿轮位置展开，
文字和表单控件不缩放；系统开启“减少动态效果”时直接显示。
它用于表达入口与面板的关联，其他抽屉仍沿用默认滑入。

侧栏按钮切换明暗主题时，从按钮中心展开新配色，反馈触发位置及新主题覆盖范围。
过渡时长为 360 ms。开启「减少动态效果」或浏览器不支持过渡时直接切换。

## Icons

- **统一使用 Iconify。** 新增或调整 UI 图标时使用 Iconify，优先选用 `tabler` 图标集，保持线条、尺寸和视觉风格一致；已有直接依赖图标库的实现随相关 UI 调整逐步迁移。
- **CLI 品牌标志。** 启动配置的 Antigravity CLI、Claude Code、Codex、Gemini CLI、Cursor、GitHub Copilot、OpenCode、Qwen、Kimi、CodeBuddy、Pi / omp 与 Qoder / QoderCN 使用 LobeHub 单色 SVG，
  经同一个 `unplugin-icons` 管线编译为 Svelte 组件；来源版本与 MIT 许可保存在 [图标目录](../web/src/icons/lobehub/README.md)。通用终端仍使用 Tabler。
- **图标随应用打包。** 仅引入所需图标数据，在本地构建时打包，运行时不依赖公共 Iconify API 或 CDN，保证离线和受限网络下仍可显示。
- **用图标简化常用操作。** 清空、删除、排序、关闭等含义明确的操作可使用纯图标按钮；发送、添加等主要操作或含义不明确的动作保留必要文字。同一动作使用同一图标，不混用 emoji 或另画一套通用操作图标。
- **名称与交互不可省略。** 纯图标按钮必须提供 `aria-label` 或屏幕阅读器文字，并提供可见的悬停提示（`title` 或 Tooltip）；装饰性图标使用 `aria-hidden="true"`。按钮继续遵循现有焦点、禁用态和触摸区域规范。
- **沿用语义样式。** 图标大小随基础控件尺寸，颜色继承文字或语义令牌，不通过图标颜色单独表达状态。本约定针对应用 UI，不替换 PTY 输出中的字符或终端字体字形。

## TV、头显与沉浸式呈现

设备模式由用户明确选择，不通过 UA、分辨率或 coarse pointer 猜测。设置与终端菜单共享设备本地开关；TV 使用根 `data-ui-mode="tv"`，头显浏览器模式使用 `data-headset-mode="true"`，默认均关闭。

- TV 在 `styles.css` 根作用域将字号设为 125%，`--control-sm/default/touch/touch-compact` 设为 `2.75rem`，`--control-lg` 设为 `3rem`，顶栏高度设为 `3.5rem`。复用现有语义色、variant 和 size，不新增页面专属色板。
- TV 焦点使用 `--tv-focus-width: 3px`、`--tv-focus-offset: 3px` 和 primary 描边；根属性也覆盖 portal 菜单与对话框。xterm 隐藏输入框不绘制外框，捕获输入时改在终端区域显示内侧描边。
- 方向导航只移动到可见、可用目标，并将目标滚入视口；文本输入、代码编辑器、树、菜单和其他 roving 控件保留原有按键语义。弹层保留焦点约束、可访问名称与关闭后的焦点恢复。
- TV 终端区明确显示导航/输入状态；返回先退出输入，Esc 控件发送真正的终端 Escape。网页不能接收的系统返回键不承诺可拦截。
- 头显模式提供明确的「键盘」入口并抑制意外自动聚焦；TV/头显模式隐藏自动触屏软键，但保留用户强制显示选择。coarse pointer 仍可用于触控目标，不代表手机布局或头显身份。

沉浸式入口由能力、用户手势与活动状态共同决定：检查安全上下文、WebXR 和 `immersive-vr`；支持时宽屏显示「进入 VR」，窄屏/TV 放入菜单，不支持时菜单仍解释原因。
活动时显示「退出 VR」，忙碌时阻止重复请求；普通全屏与 XR 请求互斥，但全屏不是进入 XR 的前提。

XR 使用独立 Canvas/Three.js 面板，不把 DOM 控件或 xterm 私有画布截图搬入 VR。
终端呈现公开 Buffer 的文本、颜色、基本属性、光标、宽字符与组合字符；纹理每边不超过 4096 像素且总量不超过 4 Mi 像素，面板物理缩放不修改 PTY 网格。
Kitty 等未支持图形显示说明与「返回 2D」入口，文本和输入继续可用；聊天、代码与 Diff 为专用只读文本视图，不承诺富文本或编辑器的视觉等价。

上述设计契约仍需真实头显与电视验证控制器、系统键盘、十英尺可读性和双眼显示；浏览器 fixture 与视口截图不等于硬件兼容证明。

## Storybook

运行 `pnpm --dir web storybook`，打开 `Design system`：

| Story          | 检查内容                                         |
| -------------- | ------------------------------------------------ |
| Foundations    | 设计核心、语义色、文字层级、间距与形状           |
| Actions        | 按钮层级、密度、禁用和处理状态                   |
| Forms          | 常规、无效、只读、禁用字段与开关                 |
| Feedback       | 会话状态、成功、警告、错误和说明                 |
| Surfaces       | 卡片、空状态、加载状态、标签页和菜单             |
| Overlays       | 响应式编辑弹层与危险操作确认                     |
| Session States | 运行、等待、空闲、正常退出和失败退出的真实会话行 |

现有 `Workspace` 展示真实业务布局，`UI/Overflow` 保留极端长文本场景。示例交互只修改内存中的演示数据。

## Preview evidence

以下截图来自运行中的真实 Storybook 组件，使用仓库内的固定示例数据。

| 视图              | 浅色                                                                                                                         | 深色                                                                                                                       |
| ----------------- | ---------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| 设计基础          | [查看](images/design-system/foundations-light.jpg)                                                                           | [查看](images/design-system/foundations-dark.jpg)                                                                          |
| 工作区 · 1280×720 | [修改前](images/design-system/workspace-before-light.jpg) / [修改后](images/design-system/workspace-light.jpg)               | [修改前](images/design-system/workspace-before-dark.jpg) / [修改后](images/design-system/workspace-dark.jpg)               |
| 工作区 · 390×844  | [修改前](images/design-system/workspace-before-mobile-light.jpg) / [修改后](images/design-system/workspace-mobile-light.jpg) | [修改前](images/design-system/workspace-before-mobile-dark.jpg) / [修改后](images/design-system/workspace-mobile-dark.jpg) |

## Verification and scope

新增设计系统场景沿用六种视口/字号与明暗主题矩阵。检查布局、正文颜色对比度、错误文字、表单编辑、菜单语义、弹层关闭与焦点恢复。既有防溢出和组件/E2E 套件继续承担业务行为验证。

本轮统一覆盖核心控件、内容容器、弹层和反馈模式，并应用到工作区、设置、草稿输入和代码变更状态。终端字符网格、语法高亮、虚拟列表行高、会话生命周期和后端协议保持原有实现。

Storybook 的几何与交互检查不是像素快照比较。手机视口和根字号测试不能替代真实 iOS/Android 键盘、触摸及系统缩放验证。
