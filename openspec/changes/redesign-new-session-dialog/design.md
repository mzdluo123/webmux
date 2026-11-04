# Design

## Context

`NewSessionSheet.svelte`（686 行）同时承担“新建会话”“仅新建工作树”“分屏新建”三种意图，用 `intent`、`split`、`treeOnly`、`mode`、`commandSource` 多组状态交织控制显示。调用点有两处：`SessionList.svelte` 通过 `start(mode, repositoryId, worktreeId)` 传入 `initialMode / initialRepositoryId / initialWorktreeId`，其中全局新建已在单仓库视图用 `selectedRepository` 预选主工作区；`Terminal.svelte` 传入 `initialCwd / initialWorkspace / split`。

浏览器本地已有 `recentDirs`、`lastLaunch`、`customCommand`、`hybridStartSide`。请求契约（`CreateSessionRequestSchema`、`CreateWorktreeRequestSchema`）与后端无需改变。仓库 UI 规范要求优先使用 `$lib/components/ui`（已有 `popover`、`command`、`select`、`collapsible`、`drawer`），不得新增 eslint-disable 或扩张 `check:arch` 基线。动机见 proposal.md 的 Why。

## Goals / Non-Goals

**Goals:**

- 默认值即结果：主页选中仓库与最近一次启动方案共同决定初始状态，常见路径零点击提交。
- 一个对话框、两个选择行、一个汇总；去掉两层 Tab。
- 把单个超大组件拆成职责清晰、可单测的部分，状态模型不再依赖 `intent` 分支堆叠。
- 手机与桌面能力一致。

**Non-Goals:**

- 不改后端、契约、wire 快照、启动配置存储与设置页。
- 不按仓库分别记忆启动方案，不记住会话名称。
- 不做命令面板式入口或多步向导。
- 不改变工作树创建、上游刷新与冲突处理的行为（沿用 `worktree-lifecycle`）。

## Decisions

### 1. 目标位置建模为判别联合

用单一 `target` 状态取代 `mode + cwd + repositoryId + worktreeId`：

- `{ kind: "directory", cwd }`
- `{ kind: "worktree", repositoryId, worktreeId }`
- `{ kind: "new-worktree", repositoryId }`（分支字段另存）
- `{ kind: "unset", repositoryId? }`（主页选中仓库无可用目标，要求用户选择）

纯函数 `resolveInitialTarget({ context, selectedRepositoryId, repositories, recent, defaultCwd })` 实现 spec 的优先级（上下文入口 > 主页选中仓库主工作区/其他可用工作树 > 最近目录 > 默认目录），放在独立 `.ts` 中单测。`SessionList` 与 `Terminal` 只传“上下文入口”与 `selectedRepositoryId`，不再各自推导预选。

备选：保留 `mode` 枚举与 Tab 只换外观。否决：状态耦合不变，达不到“大改”的目的。

### 2. 命令建模为 `launch` 判别联合并保持存储兼容

`{ kind: "config", id, startSide? } | { kind: "custom", text } | { kind: "unset" }`。`resolveInitialLaunch({ lastLaunch, customCommand, hybridStartSide, configs, status })` 为纯函数：记忆项有效则用之；无记忆取第一个配置，无配置取 custom；记忆的配置已删除得到 `unset`（沿用 spec，不静默替换），这与现有代码把失效项改为 custom 的行为有意不同。存储键与格式不变，仅在创建成功后写入，故无需迁移。

加载中（`status: "loading"`）先渲染命令行骨架；一旦完成且用户尚未操作命令行，才应用 `resolveInitialLaunch`，避免覆盖用户在加载期间已选的“自定义命令…”。

备选：新增 `lastSession` 对象键，存位置+命令。否决：位置默认值应跟随主页选中仓库，持久化位置会与之冲突。

### 3. 位置与命令选择器的呈现

- 桌面：位置行与命令行是触发按钮（图标 + 主文案 + 次文案），点击打开 `Popover` + `Command`（带搜索、分组）。位置选择器分组顺序：当前上下文仓库的工作树、最近目录、其他仓库、“输入或浏览目录…”、“新建工作树…”。
- 窄屏（抽屉内）：同一内容以就地展开的 `Collapsible` 渲染，不开启浮层，避免抽屉内嵌套 Popover 的裁剪与焦点问题。用同一 `SelectorList` 组件、两种容器，由媒体查询选择。
- 直接目录输入在选择器底部展开：`Input` + 既有 `DirectoryPicker`，保留 mkdir 确认流程（`AlertDialog` 不变）。

备选：沿用 `Select` 组件。否决：无法分组承载输入框与搜索，工作树条目多时难用。

### 4. 组件拆分

- `NewSessionDialog.svelte`：持有 `target / launch / name / error / busy`，负责提交与记忆写入，替代 `NewSessionSheet`（调用点同步改名，无别名）。
- `LocationPicker.svelte`、`LaunchPicker.svelte`：受控选择器，只发出变更事件。
- `NewWorktreeFields.svelte`：包装现有 `WorktreeCreateFields`，负责精简 3 字段与“高级”折叠，保持其 `isReady()` 契约。
- `newSessionDefaults.ts`：上述两个 `resolve*` 纯函数及摘要文案生成。

“仅新建工作树”与分屏仍通过 props 区分，但差异限定为：命令行是否显示、方向控件、主按钮文案，不再改变其他状态逻辑。

### 5. 主页选中仓库的传递

`SessionList` 向对话框传 `selectedRepositoryId`（`"all" | "direct"` 归一为空）；对话框只读它，不回写。`Terminal` 不传，继续用 `initialWorkspace / initialCwd` 作为上下文入口。`creating` 目标对象保留 `mode` 仅用于区分“仅新建工作树”入口，其余预选改由 `resolveInitialTarget` 完成。

### 6. 汇总行

底部汇总由 `summarize(target, launch)` 生成，如“将在 webmux · feat/x 运行 Claude 恢复”，使用 `role="status"` 便于读屏；任一为 `unset` 时改为提示文案并禁用主按钮。

## Risks / Trade-offs

- **隐藏的细节**：低频项折叠后可发现性下降。缓解：“高级”标题上附摘要（如“已命名”“自定义存放目录”）。
- **Popover 可访问性**：焦点陷阱与 Esc 分层易出错。缓解：使用设计系统的 Popover/Command，浏览器测试覆盖 Esc 只关闭选择器、键盘选择与焦点回到触发按钮。
- **失效记忆不再静默改 custom**：与旧行为不同，多一次点击。取舍：与现行 spec“不得静默替换”一致，避免误用其他配置。
- **范围较大的重写**：测试与 E2E 用例依赖 Tab 文案。缓解：先落纯函数与单测，再改组件；E2E 通过 `fixtures.ts` 的统一助手选择位置和命令，减少散落的文案依赖。
- **加载竞态**：启动配置与仓库异步到达。缓解：默认值只在“用户尚未操作该行”时应用，操作后以用户选择为准。

## Migration Plan

无数据迁移：本地存储键格式不变，服务端无变化。回滚即还原前端组件与测试，不留兼容层。

## Open Questions

无。已作为假设记录：最近一次启动方案对所有位置通用；对话框内位置选择不回写主页选中仓库。
