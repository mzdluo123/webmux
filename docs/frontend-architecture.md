# Frontend Architecture Guard

前端架构守卫把 [设计系统](design-system.md) 中“复用组件、使用令牌、页面只控制布局”的约定变成可执行的检查，使人和 coding agent 在提交前得到具体的修复提示。所有命令在仓库根目录运行。

```sh
pnpm --dir web check:arch    # 全部架构检查；make check 与 CI 也会运行
```

## 写 UI 之前：先找，再写

按顺序查找，找到就复用，不要重新实现：

1. `web/src/lib/components/ui/`：shadcn-svelte 基础组件（Button、Input、Dialog、DropdownMenu 等）。
2. `web/src/components/<主题>/` 与 `web/src/components/shared/`：业务复合组件与跨主题组件（ResponsiveDialog、InlineNotice、SessionStatus 等）。
3. shadcn-svelte registry：`pnpm --dir web exec shadcn-svelte add <name>`，生成后审查本地差异。

三处都没有，才新建组件。需要新外观时，在 `ui/` 对应组件的 `tv()` 中增加 variant 或 size，而不是在使用处用 class 覆盖。

## 分层

源码按「层 + 业务主题」组织，根目录只保留 `App.svelte`、`main.ts`、声明与全局样式。dependency-cruiser 同时检查运行时与 type-only 导入：

| 层             | 路径                     | 可以依赖                                            |
| -------------- | ------------------------ | --------------------------------------------------- |
| 契约           | `src/contracts/`         | 契约、第三方包                                      |
| 基础设施       | `src/platform/`          | 基础设施、契约、`lib` 工具                          |
| 设计系统叶子   | `src/lib/components/ui/` | `src/lib/`、第三方包                                |
| 通用工具       | `src/lib/`               | `lib`、第三方包，不依赖业务                         |
| 领域逻辑与状态 | `src/domains/<主题>/`    | 领域逻辑、基础设施、契约、工具；不依赖渲染层        |
| Hooks          | `src/hooks/<主题>/`      | 领域逻辑、基础设施、契约、工具；不依赖渲染层        |
| 业务组件       | `src/components/<主题>/` | 以上各层；不依赖页面与应用编排                      |
| 页面           | `src/pages/`             | 以上各层；由 `App.svelte` 组合                      |
| 应用贡献       | `src/plugins/`           | 显式动态入口、页面/组件及以上各层；底层不反向依赖它 |

主题包括 `sessions`、`terminal`、`chat`、`code`、`workspace`、`settings` 与 `notifications`。领域目录把相同业务概念的纯算法与响应式状态放在一起；资源生命周期与跨主题流程放在 hooks，而不是让底层领域反向依赖控制器。

跨主题复用组件放在 `components/shared/`，应用导航与侧栏放在 `components/shell/`，通用资源控制器放在 `hooks/shared/`。不要仅因有两个调用者就把业务代码提升到 shared；优先让它留在拥有该概念的主题下，由其他主题直接导入。

领域主题只能引用配置中登记的其他主题，不允许循环。`settings`、`chat` 不依赖其他主题；`sessions` 依赖 `settings`；`workspace`、`terminal` 依赖 `settings` 与 `sessions`；`notifications` 依赖 `sessions`；`code` 依赖 `terminal`。

新增领域边必须在 `.dependency-cruiser.cjs` 的 `DOMAIN_DEPENDENCIES` 中明确登记，并说明为何不应由 hooks 协调。不要引入层层转发的 `index.ts` 或旧路径兼容导出。未来若增加 `src/app/` 编排模块，领域、hooks 与可复用组件均不得依赖它。

`bits-ui`、`vaul-svelte`、`paneforge` 等 headless 原语与 `tailwind-variants` 只在 `ui/` 中使用。基础设施、领域逻辑与 hooks 不得导入 `.svelte` 渲染组件，`.svelte.ts` 状态模块不属于渲染组件。模块间不允许循环依赖（shadcn-svelte 生成的 barrel 自引用除外），未被引用的模块应删除。

### 放置新代码

1. 线上数据形状与 Zod 契约放在 `contracts/`；不要从这里导入业务状态或浏览器 I/O。
2. HTTP、WebSocket、浏览器存储等通用 I/O 放在 `platform/`，沿用统一的 API 与传输实现。
3. 能脱离页面调用的业务算法与领域状态放在 `domains/<主题>/`。
4. 需要清理资源、管理异步生命周期或协调多个领域的控制器放在 `hooks/<主题>/`。
5. 可渲染内容放在对应主题组件中；页面负责布局与组合，设计系统负责外观。

迁移文件时必须同时更新测试 mock、Storybook、worker URL、工具配置与文档链接；旧路径不保留垫片。

### 内置插件启动与贡献

`App.svelte` 依次完成 health → 经 Zod 校验的 `/api/capabilities` → 本地动态贡献 → 首页；发现失败或版本不符时 fail-closed，不猜测旧版全量功能。
`platform/plugin-host.svelte.ts` 只保存能力状态，提供 `pluginEnabled` / `pluginAvailable`，不导入业务组件或执行网络 I/O。
`plugins/registry.svelte.ts` 与创建、设置、终端贡献入口拥有静态可审计的动态 import；只加载已选择的插件。
禁用插件不仅隐藏入口：不得加载实现 chunk、调用其 REST/WS、启动计时器、后台探测或 provider。单个可选 chunk 失败留在该功能内，不使 PTY 终端整体失败。
核心新建对话框可直接填写 argv 与 cwd，无需 launch/workspace/files；缺少 launch 时，旧浏览器记住的 `configId` 不能猜成命令，必须重新显式填写。
选择与组合规则见 [配置](deployment/configuration.md#内置插件选择)，后端事实与插件状态不是同一个布尔值，见 [能力发现](protocol.md#能力发现)。

### XR 与设备模式边界

- `hooks/shared/` 拥有 TV/头显显式开关、设备偏好和 TV 导航生命周期；`platform/spatial-navigation.ts` 只计算几何候选。根属性由应用外壳设置，呈现使用共享设计系统令牌，不由设备 UA 推断业务模式。
- 复合控件用 `data-tv-native-keys` 保留自己的键盘协议；聊天内联菜单在 `webmux:tv-back` 的 capture 阶段，仅消费自己拥有焦点时的返回，再由根导航释放编辑。
- `hooks/terminal/terminal.svelte.ts` 的 `TerminalController` 保有唯一 xterm emulator、输入路由与 socket。DOM renderer 和 XR Canvas 共用这个解析模型；`setImmersive` 暂停 DOM 适配，退出后恢复，不建立第二个 PTY 或输入控制器。
- `domains/terminal/` 放共享按键编码、XR 文本/面板契约与布局校验；`domains/code/` 放代码/Diff 的只读 XR 适配。`hooks/terminal/` 协调会话来源、异步 XR 所有权、Canvas、射线、场景与资源清理，不让领域依赖渲染层。
- Three.js 运行时只允许在 XR 专属 `hooks/terminal/xr-*` 场景模块中导入。`xr-session.svelte.ts` 在用户手势请求会话后动态导入 `xr-workspace.ts`；普通页面与领域契约不得静态导入该场景入口或初始化 3D renderer。
- XR 布局经版本化 Zod 校验并通过设备偏好存储，只含面板 ID、类型、会话引用、位置、尺寸与可见性。拖动/缩放结束才写入；不保存标题、输出、草稿、代码或 token，不进入设置 API 与同步帧。

XR 面板姿态、纹理尺寸与每眼帧缓冲只影响呈现；PTY 网格仍由共享控制权流程管理，只有用户明确确认逻辑网格变化才可提交 resize。返回 2D、关闭面板和 XR 资源清理均不拥有服务端会话销毁权。

同深度 XR 面板按统一表面顺序绘制和命中；终端子平面与所属面板共面，使用组内顺序覆盖背景。真实前后关系仍交给深度测试，控制器射线单独置顶，不用改变持久化姿态解决遮挡。

## 检查项

| 命令          | 工具                     | 检查内容                                                                                                                                          |
| ------------- | ------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| `arch:lint`   | ESLint + `@shadcn/lint`  | 用 class 覆盖组件外观、原始调色板颜色、任意值、内联样式、未知类名、动态类名；手写原生控件；在 `ui/` 外导入原语、变体工具或 `@tabler/icons-svelte` |
| `arch:deps`   | dependency-cruiser       | 分层、领域依赖白名单、type-only 越界、循环依赖、孤立模块与无法解析的导入                                                                          |
| `arch:dupes`  | jscpd                    | 新增的复制粘贴代码块                                                                                                                              |
| `arch:unused` | knip                     | 从入口不可达的文件、未使用的依赖、导出与类型，重复导出                                                                                            |
| `arch:test`   | Vite+ `vp test` (Vitest) | 目录归属、同名组件、重复外观类、圆角、命名与文件大小；真实导入样例验证合法依赖、反向依赖、type-only、循环与旧路径拒绝                             |

Vite+ 的 Oxlint（`pnpm --dir web lint`，即 `vp lint`）负责通用 lint。它只读取 `.svelte` 的 `<script>`，模板相关规则仍由 ESLint 的架构与 Svelte 模板规则检查。

`test/dependency-policy.test.ts` 在临时目录生成独立的 TypeScript/Svelte 模块，调用实际 dependency-cruiser 配置并核对允许与拒绝结果；不以匹配配置文本替代依赖分析。`test/architecture.test.ts` 直接扫描生产树，阻止业务文件重新堆入根目录或无主题的组件、hooks 目录。

导入解析只豁免 Vite 的 `~icons/` 虚拟模块和 `@pierre/diffs/worker/worker.js?worker` 资源查询；这些由 Vite 插件解析，生产构建仍验证其存在。普通本地模块与本地 worker 的失效路径不豁免。

`no-restyle` 只允许使用处调整布局（外边距、宽度、高度、栅格与 flex 位置等）。例外由 `web/eslint.config.mjs` 中的组件契约声明，新增契约需要在 PR 中说明设计理由：

| 组件                                                                                                                                                                                | 使用处还可以调整            | 理由                                                                           |
| ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------- | ------------------------------------------------------------------------------ |
| `CardTitle`                                                                                                                                                                         | 排版（不含字重）            | 标题字号随所在区域变化                                                         |
| `CardContent`、`TabsContent`、`CommandList`、`ItemGroup`、`ItemActions`、`Collapsible`、`CollapsibleContent`、`DialogContent`、`DialogHeader`、`PopoverContent`、`ResponsiveDialog` | 内边距与间距                | 内容区域的留白取决于放入的内容，组件本身不规定                                 |
| `CollapsibleTrigger`、`PopoverTrigger`                                                                                                                                              | 全部外观                    | bits-ui 原语，设计系统没有给它们任何样式；需要按钮外观时用 `child` 渲染 Button |
| `Pane`、`ResizableHandle`                                                                                                                                                           | `styles.css` 中的布局钩子类 | 终端、草稿、代码与 Git 分栏的显示和尺寸由响应式 CSS 控制                       |

Git 工作台分栏使用 `git-pane` 与 `git-divider` 布局钩子类，只在样式表中表达分栏尺寸与 1px 分隔线。

组件需要由样式表控制布局时，优先用 `data-*` 属性作为选择器（如 `DirectoryPicker` 的 `data-directory-picker`），只在样式表中写布局；外观仍通过 variant 表达。

常用的设计系统扩展：

| 组件                | variant / size                                              | 用途                                                                                                                                                                                            |
| ------------------- | ----------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Button`            | `variant="quiet"`                                           | 平时弱化、悬停时恢复前景色的 ghost 按钮                                                                                                                                                         |
| `Button`            | `variant="hud"`                                             | 软键栏内摇杆与虚拟按键的高对比度表面（低不透明度边框与表面）                                                                                                                                    |
| `Button`            | `variant="success"`                                         | 对话框中更安全的替代操作（如“仅从主页隐藏”），绿色填充，放在右下角                                                                                                                              |
| `Button`            | `variant="destructive-outline"`                             | 文字形式的危险次要操作（如“删除全部快捷指令”）：红色描边与文字，无填充                                                                                                                          |
| `Card`              | `flat`                                                      | 设置页各分区：手机上去掉边框与底色，只留标题与内容                                                                                                                                              |
| `Item`              | `variant="divided"`                                         | 设置页的扁平列表行，行与行之间用细线分隔                                                                                                                                                        |
| `ItemGroup`         | `variant="divided"`                                         | 与 `Item variant="divided"` 配套：列表项之间不留间隙，细线紧贴行边                                                                                                                              |
| `Button`            | `aria-pressed` / `aria-expanded` / `aria-current="true"`    | outline、ghost、quiet 在开关按下、展开或当前项时显示前景色描边；不再提供填充的 `secondary`                                                                                                      |
| `Button`            | `raised`                                                    | 浮在终端或代码上的按钮（边框加阴影）                                                                                                                                                            |
| `Button`            | `wrap`                                                      | 较长的文字按钮（如“改为在新工作树打开”）在窄屏大字号下换行而不溢出；平时高度不变                                                                                                                |
| `Button`            | `size="icon-touch"` / `size="sm-touch"`                     | 窄屏（小于 `sm`）44px 触控尺寸，`sm` 以上恢复常规尺寸                                                                                                                                           |
| `Button`            | `size="icon-touch-sm"`                                      | 手机 44px、`sm` 以上 32px：紧凑工具栏中需保留触控尺寸的按钮（如终端顶栏 ⋯）                                                                                                                     |
| `Input`、`Textarea` | `mono`                                                      | 命令、路径、环境变量与终端文本                                                                                                                                                                  |
| `Textarea`          | `variant="ghost"`                                           | 在列表项或卡片中原地编辑                                                                                                                                                                        |
| `Textarea`          | `variant="bare"`                                            | 填满自带边框与焦点环的容器（如聊天输入框），自身不再显示焦点环                                                                                                                                  |
| `Empty`             | `variant="outline"` / `variant="card"`                      | 区块内的虚线空状态 / 独立卡片上的空页面                                                                                                                                                         |
| `Command`           | `variant="outline"`                                         | 嵌在表单或对话框中的列表                                                                                                                                                                        |
| `DropdownMenuLabel` | `variant="title"`                                           | 整个菜单的标题                                                                                                                                                                                  |
| `Item`              | `variant="card"`、`size="flush"`                            | 卡片表面、悬停高亮的列表项；内容自带内边距（如整行链接）                                                                                                                                        |
| `Input`、`Label`    | `size="sm"`                                                 | 代码导航等紧凑面板                                                                                                                                                                              |
| `NativeSelect`      | `size="sm"`                                                 | 原生下拉选择；手机保留系统选择器                                                                                                                                                                |
| `Select`            | —                                                           | 需要跨平台一致样式、同时展示名称与命令或路径的下拉框；保留完整内容提示与键盘选择                                                                                                                |
| `Tabs.List`         | `variant="line"`、`size="form"`、`animated`                 | 带关联面板的标签页统一使用 `line`，不使用填充胶囊样式的 `default`；`form` 用于表单中的分组切换；横向标签可用 `animated` 让下划线平滑移动                                                        |
| `SegmentedControl`  | `size="form"`、`wrap`                                       | 基于 RadioGroup（`radiogroup`/`radio`）的下划线分段控件；`form` 用于表单，`wrap` 在窄屏换行（如状态筛选）                                                                                       |
| `RadioGroup`        | —                                                           | 每个选项都带说明文字的单选（如新建会话的起始端）；传 `aria-label` 作为名称，边框标出当前项                                                                                                      |
| `Checkbox`          | —                                                           | 复选框，配合 `Label` 使用                                                                                                                                                                       |
| `ListButton`        | `row` / `text` / `content` / `option` / `tab` / `tab-close` | 列表、标签栏与卡片中的可点击区域。它不带控件尺寸，虚拟列表的固定行高不受影响；独立的操作按钮仍用 `Button`                                                                                       |
| `ListButton`        | `navigation` / `navigation-action` / `file-tree-overlay`    | 侧栏导航行：`aria-pressed` 标记选中的筛选，`aria-current="page"` 标记当前页面；后两者用于带行尾操作按钮的侧栏行，见表下说明                                                                     |
| `ListButton`        | `variant="review-toggle"`                                   | 常驻行内审阅图标；32px、紧凑触控 36px，aria-pressed 仅改变主题文字色，不绘制边框或填充选中块。                                                                                                  |
| `ListButton`        | `variant="compact-row"` / `variant="file-tree-action"`      | Git 工作台紧凑选中行（`rounded-sm`，浅主题底色与左侧细线）及其行尾动作按钮（窄屏与触屏常驻、桌面悬停/聚焦显示）                                                                                 |
| `ChatSelect`        | `labeled`、`searchable`                                     | 输入框下方的选择：`labeled` 同时显示当前值文字（快速开始的仓库与 agent），`searchable` 用可搜索面板筛选模型选项；带 `reason` 的选项保留在菜单中并禁用；基准分支改用可搜索的 `BranchCommandList` |

`ListButton` 的 `navigation-action`（聊天历史）与 `file-tree-overlay`（工作台树）在鼠标设备上不为行尾操作按钮留空位：按钮在悬停、聚焦或菜单打开时覆盖行尾的状态圆点或计数（这些标记加上 `sidebar-row.ts` 的 `underRowAction` 让位）；触屏上留出按钮位置，按钮常驻。

`ItemGroup variant="panel"` 统一提供紧凑列表的边框与分隔线；`Item variant="row"` 提供无独立卡片边框的行；`ListButton variant="navigation"` 提供侧栏导航行的选中与键盘焦点样式。

`ListButton variant="turn"` 用于宽聊天窗格的对话轮次刻度：`data-turn-role="user"` 使用前景色，`agent` 使用信息蓝，`aria-current="location"` 加长并提高当前刻度的不透明度；角色与消息摘要同时写入可访问名称和悬停提示。

粗指针设备的最小触控高度用 Tailwind 的 `any-pointer-coarse:` 变体（如 `any-pointer-coarse:min-h-11`）。

### 任意值与运行时尺寸

`Drawer.Content motion="origin" origin={bounds}` 使用 Motion 的原生动画接口，
`bounds` 在打开前从入口测量，避免焦点或键盘改变布局；轮廓展开而文字不缩放。默认 `motion="slide"` 保持原有抽屉动画。
动画由基础组件管理并在关闭、销毁时清理；减少动态效果时跳过展开动画。

`no-arbitrary-values` 允许布局类的任意值：依赖视口的尺寸计算（如 `max-h-[50dvh]`、`w-[min(90vw,520px)]`）是布局，不是设计令牌。间距、排版、颜色、圆角与阴影仍必须使用主题刻度或令牌，例如：

- 字号用 `text-xs` 等刻度，元数据最小 12px。
- 带安全区的底部留白用 `styles.css` 中的 `pb-safe-4`。

运行时才能确定的几何值（虚拟列表行高、图片缩放、视口高度、预览行数）用 `style:--name` 设置 CSS 自定义属性，再由类读取，如 `h-(--row-height)`、`transform-(--image-transform)`，或由 `styles.css` 中的工具类读取（如 `h-preview`）。不要用 `style:height` 等内联样式。

## 基线与新规则

`frontend-architecture-guard` 的存量治理已经完成，下面三个基线文件都为空，所有检查都按严格模式执行。保留这些文件，是为了以后引入有存量违规的新规则时，可以先记录基线，再逐步收紧：

| 文件                                            | 记录内容                     |
| ----------------------------------------------- | ---------------------------- |
| `web/eslint-suppressions.json`                  | 每个文件、每条规则的违规数量 |
| `web/.dependency-cruiser-known-violations.json` | 已知的分层违规               |
| `web/.jscpd-baseline.json`                      | 已知的重复代码块指纹         |

- 修复存量问题后运行 `pnpm --dir web arch:ratchet`，它只删除已修复的记录并格式化基线文件，然后提交基线变化。重复代码先通过 `arch:dupes` 检查：出现未登记的克隆时立即失败，不改写 jscpd 基线；只有没有新增克隆时才更新基线，删除已解决的指纹。
- ESLint 按“文件 + 规则”计数：同一文件同一规则的违规数超过记录时，该文件该规则的所有违规都会报告。
- 不要为了通过检查而扩大基线、添加 `eslint-disable` 或 `jscpd:ignore`。确有必要的例外，在 PR 中说明原因，并优先改为配置中的契约或白名单。

治理过程与每批的决策记录见 OpenSpec 变更 `frontend-architecture-guard`。

紧凑列表使用 `compact-row`，折叠头使用 `section-header`，来源槽位使用 `provenance-gutter`，可选择差异行使用 `diff-line`；焦点统一消费 ring。
