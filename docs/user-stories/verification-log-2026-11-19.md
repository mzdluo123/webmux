# 核对记录：2026-11-19 起

承接 [verification-log-2026-11-18](verification-log-2026-11-18.md)，收录 2026-11-19 起的局部核对记录；索引见 [WebMux User Stories](../user-stories.md#核对记录)。

## 2026-11-21 两步交接与接收方选择（US-Q20）

- `pnpm --dir web exec playwright test test/e2e/handoff.spec.ts` 在桌面及手机 Chromium 上共通过 12 项。覆盖创建新聊天、搜索已有聊天、返回修改、页数校验、预览图片与按需补查。
- 隔离 Chromium 在桌面、平板、手机和手机横屏的浅色与深色主题核对预览，页面无横向溢出，底部操作栏位于视口内。真机软键盘未实测。

## 2026-11-21 历史只随图片交接（US-Q20、US-K04）

- `go test -race ./internal/handoff ./internal/historyprojection` 通过。短历史生成一页，长历史首尾及中段正文均留在图片中。目标缺少图片能力或预算无法容纳图片时停止投递。
- `pnpm --dir web exec playwright test test/e2e/handoff.spec.ts` 在桌面与移动 Chromium 视口验证了短历史新建目标、过期、预算编辑、长历史预览和能力拒绝。短历史用例核对测试 Agent 收到的三个 ACP 块，PNG 字节与预览一致。
- `pnpm --dir web check:arch`、`make docs-check` 通过。图片 token 数为预算估计，本次验证使用隔离测试 Agent。

## 2026-11-19：单窗格隐藏焦点边框（US-D13、US-D14）

- Playwright 运行 `splits.spec.ts`、`session-pane-files.spec.ts` 和 `git-workbench-route.spec.ts`，桌面与移动项目共 36 项通过，8 项按项目条件跳过。核对分屏焦点、放大与还原、收起、标签合并和窄屏布局。
- 隔离 Shell 会话在 1280×900、1280×720、768×800、390×844、375×667、320×568 的明暗主题下均通过焦点边框和页面横向溢出检查。截图与放大、还原录屏使用无头 Chromium，实体触屏与真机浏览器未核对。
- `pnpm --dir web check`、`check:test`、`lint`、`check:arch`（49 项）及 `make format-check docs-check` 通过。

## 2026-11-19：`InlineNotice` 行尾操作对齐与浮层实心表面统一（US-C01、US-L03、US-L06）

- 故事编号与总数（265）保持不变，同步更新 `US-C01`、`docs/design-system.md` 与 `docs/frontend-architecture.md`：
  - `InlineNotice` 新增可选 `actions` 插槽与 `surface="inline" | "floating"` 变体：状态图标高度锁定首行行高（`1.625em`），单行时图标、正文与按钮中心对齐，多行时图标固定在首行，空 `actions` 容器自动隐藏；
  - 带操作插槽时正文列采用 `basis-[max(11rem,60%)]`，在 320px 单按钮场景保持同行排列，多按钮中等宽度下整组换行至下一行；
  - `surface="floating"` 采用 `bg-popover` 与阴影的不透明卡片表面；`LazyDialog.svelte`、`FeatureLoadNotices.svelte` 与 `App.svelte` 底部固定提示切换至该变体；
  - 全仓 15 个带行尾按钮的 `InlineNotice` 调用文件统一迁移至 `actions` 插槽，并补充 `DesignSystem.svelte` 反馈示例与 `InlineNotice.test.ts` 几何测试。
- 已执行验证：
  - 静态与文档门禁：`make format-check docs-check persistence-check`、`pnpm --dir web check`、`check:test`、`check:storybook`、`lint`、`check:arch`（49 项）、`go vet ./...`、`GOOS=darwin go vet ./...` 与 `make go-lint`（0 issues）通过；
  - 单元与浏览器组件测试：`pnpm --dir web test:unit`（91 个文件共 1306 项），Vitest 浏览器套件 `InlineNotice.test.ts`、`SessionListNotices.test.ts`、`LaunchConfigsSettings.test.ts`、`GitWorkbenchCommitPush.test.ts`、`GitWorkbench.test.ts`（5 个文件共 26 项）通过；
  - Storybook 与端到端测试：Storybook `contracts.spec.ts`、`design-system.spec.ts`、`overflow.spec.ts`（63 项）及 Playwright E2E `page-stability.spec.ts`、`workspace-discovery.spec.ts`（26 项）通过；
  - 隔离实例视觉核对：覆盖 `1280×900`、`1280×720`、`768×800`、`667×375`、`390×844`、`375×667`、`320×568` 七种视口的明暗主题，均满足 `scrollWidth <= innerWidth` 且浮层完整位于视口内，并生成桌面、平板、手机前后对比截图。
- 局限：无头 Linux Chromium 的 `hover` 与 `any-pointer: coarse` 默认未激活，真实鼠标悬停、实体触屏及 macOS / iOS / Android 真机浏览器未验证。

## 2026-11-19：工作树置顶多仓库隔离与删除/释放自动收敛修复（US-J29）

- 故事编号与总数（265）保持不变，修复 `US-J29` 多仓库与删除收敛边界并同步更新 `docs/protocol/settings.md`、`docs/user-stories/code-and-worktrees/worktrees/lifecycle.md`：
  - `worktree-pins.ts` 将单仓库置顶切换与调序的失效 ID 收敛限定在目标仓库内，保留其余仓库已保存的置顶条目；补齐最多 200 个仓库、合计最多 1000 项置顶的前端上限门禁及 `settings-intent.ts` 409 冲突变基裁剪；
  - `internal/settings` 新增 `RetainWorktreePins` 与 `DropRepositoryWorktreePins` 并接入 `internal/workspace` 健康快照收敛与仓库取消登记，清理删除、释放或外部移除的工作树置顶 ID；`CoreShellDialogs.svelte` 与 `ReleaseWorktreeDialog.svelte` 补齐前端即时移除回调；
  - `GitWorktreeGuardState.svelte` 可写工作树选择守卫页同步置顶排序与图钉标记，`ManageWorktreesSelectList.svelte` 与 `RepositoryGroup.svelte` 按当前可见候选列表计算上下移边界。
- 已执行验证：
  - Go 与静态门禁：`make format-check docs-check persistence-check`、`go vet ./...`、`GOOS=darwin go vet ./...`、`make go-lint`（0 issues）及 `go test -race -count=1 ./internal/settings ./internal/workspace ./internal/api ./internal/wire ./internal/clicmd ./internal/archtest ./cmd/webmux` 通过；
  - 前端静态与单元测试：`pnpm --dir web build`、`pnpm --dir web check:test`、`pnpm --dir web lint`、`pnpm --dir web check:arch`（49 项）与 `make test-unit`（91 个文件共 1289 项）通过；
  - 浏览器组件测试：`WorkbenchTree.test.ts`、`RepositoryPanel.test.ts`、`RepositoryGroup.test.ts`、`GitWorkbenchRoute.test.ts`、`AppShell.test.ts`、`WorkspaceGroup.test.ts`（6 个文件共 86 项）通过；
  - Playwright 端到端测试：`worktrees.spec.ts`、`settings-sync.spec.ts`、`git-workbench-route.spec.ts`、`session-creation.spec.ts`、`quick-start.spec.ts`（69 项通过、1 项按项目条件跳过）通过。
- 局限：无头 Linux Chromium 的 `hover` 与 `any-pointer: coarse` 默认未激活，真实鼠标悬停显现、实体触屏常驻及真机浏览器未验证。

## 2026-11-19：工作树置顶与跨列表优先排序（US-J29、US-J14、US-C17）

- 新增 US-J29 并同步更新 US-J14、US-C17，故事总数由 264 增至 265：
  - `settings.json` 与 `/api/settings` 新增 `worktreePins`（每仓库最多 50 项、最多 200 个仓库、总计最多 1000 项），支持按仓库粒度冲突变基与失效工作树自动收敛；
  - 所属仓库内已置顶工作树按保存顺序排在最前（高于未置顶的主工作区与普通工作树），显示图钉标记，覆盖侧栏「按分支」树、工作台首页工作树分组、仓库面板工作树分区与「主页显示…」弹窗、新建会话位置选择器、快速开始已有工作树选择器及 Git 工作台目标工作树切换器；
  - 置顶尚未加入主页的空闲工作树时，同一次 `PATCH /api/settings` 将其加入 `homeWorktrees`；仓库面板「管理工作树 → 批量清理」中的「选中全部可安全清理（N）」跳过已置顶项并保留手动勾选框。
- 已执行验证：
  - Go 与静态门禁：`go test -race -count=1 ./internal/settings ./internal/wire ./internal/api ./internal/clicmd ./internal/workspace ./internal/archtest ./internal/docguard`、`make persistence-check`、`make go-lint`（0 issues）、`go vet ./...` 与 `GOOS=darwin go vet ./...` 均通过；
  - 前端静态与单元测试：`pnpm --dir web build`、`pnpm --dir web check:test`、`pnpm --dir web lint`、`pnpm --dir web check:arch`（49 项架构守卫）及 `make test-unit`（90 个文件共 1260 项）通过；
  - 浏览器组件测试：`WorkbenchTree.test.ts`（17 项）、`RepositoryPanel.test.ts`（28 项）、`RepositoryGroup.test.ts`（6 项）、`GitWorkbenchRoute.test.ts`（8 项）、`AppShell.test.ts`（19 项）与 `WorkspaceGroup.test.ts`（6 项）通过；
  - Playwright 端到端测试：`worktrees.spec.ts`（桌面与 iPhone SE 共 26 项）及 `settings-sync.spec.ts`、`git-workbench-route.spec.ts`、`session-creation.spec.ts`、`quick-start.spec.ts`（43 项通过、1 项按项目条件跳过）通过；
  - 隔离 Chromium 视觉核对：核对 1280×900、1280×720、768×800、667×375、390×844、375×667、320×568 七个视口的浅色与深色主题，无页面级横向溢出，展开菜单落在视口内，同排图标按钮尺寸一致；提供前后对比截图及[桌面端置顶、调序与取消置顶录屏](../images/pr/worktree-pins/interaction-desktop.gif)。
- 局限：无头 Linux Chromium 的 `hover` 与 `any-pointer: coarse` 默认未激活，真实鼠标悬停显现、实体触屏常驻及 macOS / iOS / Android 真机浏览器未验证。

## 2026-11-19：设置页合并启动配置与 Agent 工具及自动隐藏未安装 Agent（US-B01、US-B03、US-B08、US-B13、US-Q18、US-Q20）

### 范围

- 故事编号与总数保持不变，将 `docs/user-stories/access-and-sessions/connection-and-launch.md` 拆分为 A 组连接认证（`connection-and-launch.md`）与 B 组创建会话及启动位置（`launch.md`），并同步更新索引与验收标准：
  - 设置导航合并为单一「启动配置」（`#launch-configs`，兼容 `#agent-tools` 跳转）；卡片顶部汇总本机已安装计数、可更新计数、扫描时间及重扫/检查更新操作；
  - 已安装的受检 CLI Agent 启动配置在行内直接融合当前版本、解析路径、按需检查的更新状态与外部 ACP 连接器子卡片；未安装的受检 Agent 启动配置与未安装工具统一折叠于底部「未安装的工具（N）」区域，保留编辑、删除与一键复制安装命令，新建或补回未安装预设时自动展开；
  - 新建会话「运行」选择器、快速开始聊天与交接新建目标在本机扫描就绪后自动隐藏对应主 CLI 未安装的受检 Agent 启动配置：
    - 保留 `Shell` 等非 Agent 配置、显式路径、携带自定义 `PATH` 环境变量及主 CLI 已安装仅缺外部 ACP 连接器的配置，扫描未完成或失败时保留全部配置；
    - 默认选中自动跳过未安装项，记住的配置变为未安装时要求显式重新选择。

### 已执行验证与局限

- 静态门禁、格式、架构与文档守卫：
  - `make format` 与 `make check`（含 `pnpm --dir web build`、`format-check`、`docs-check`、`persistence-check`、`pnpm --dir web check:test`、`pnpm --dir web lint`、`pnpm --dir web check:arch` 49 项依赖与分层测试、`go vet` 及 `golangci-lint` v2.15.0）全部通过，0 warning / 0 error。
- 单元测试、浏览器组件、Storybook 响应式防溢出与端到端回归：
  - Vitest 单元测试：`test/launch-configs.test.ts`（61 项）、`test/new-session-defaults.test.ts`（13 项）、`test/i18n.test.ts`（5 项）通过；
  - Vitest 浏览器测试：`LaunchConfigsSettings.test.ts`、`AgentToolsSettings.test.ts`、`SettingsNavigation.test.ts`、`NewSessionDialog.test.ts`、`QuickStart.test.ts`、`FeatureCreationAvailability.test.ts` 通过；
  - Storybook 布局与防溢出测试：`pnpm --dir web exec playwright test --config playwright.storybook.config.ts -g "launch configs"` 覆盖 4 类场景 × 12 种视口/主题/大字号（含 320px `large-text`）全部通过；
  - Playwright E2E 测试：`agent-tools.spec.ts`、`launch-configs.spec.ts`、`session-creation.spec.ts`、`quick-start.spec.ts`、`i18n.spec.ts`（桌面与 iPhone SE 移动视口）通过；
  - 隔离 Chromium 视觉核验覆盖 `1280×900`、`768×800`、`390×844` 浅色与深色主题的修改前后对比及真实操作 GIF，核验工具栏与页脚同排按钮盒高度一致且无横向溢出。
- 局限：
  - 本轮验证在无头 Linux Chromium 中执行（`hover` 与 `any-pointer: coarse` 为 false），未连接外网包仓库生产源（更新检查使用受控模拟响应），未在物理触屏或真机 iOS Safari / Android 浏览器上复测。

## 2026-11-19：启动配置完成与介入提醒范围切换（T090，US-B08）

### 范围

- 故事编号、总数与覆盖索引保持不变，更新 US-B08 的提醒范围交互验收标准：
  - `web/src/components/settings/LaunchAttentionFields.svelte`：方向键可在「全部提醒」「仅需介入」「关闭」三档之间连续切换；
  - 选「仅需介入」时四类信号源保持可用，时间阈值原位禁用并维持与「全部提醒」一致的高度；
  - 选「关闭」时信号源、优先级说明与时间阈值通过 `useDockMotion` 收起并标记 `hidden` 与 `inert`，开启减少动态效果时直接切换；
  - 时间阈值超出 `0–300` 秒或 `0–60` 秒范围时按需渲染 `role="alert"` 实时错误文本，无错误时保持紧凑布局。

### 已执行验证与局限

- 前端类型、静态与文档门禁：
  - `pnpm --dir web check`、`pnpm --dir web check:test`、`pnpm --dir web lint` 与 `pnpm --dir web check:arch` 通过；
  - `go test -count=1 ./internal/docguard` 通过。
- 浏览器组件测试：
  - `pnpm --dir web exec vp test run --config vitest.browser.config.ts test/browser/LaunchConfigDialog.test.ts` 通过，覆盖提醒范围键盘切换、「关闭」收起高度、阈值原位禁用与范围错误按需播报。
- 局限：
  - 本轮验证在无头 Linux Chromium 中执行，未连接生产后端，未在物理触屏或真机浏览器上复测。
  - 视觉证据见 [PR #786](https://github.com/mzdluo123/webmux/pull/786)。

## 2026-11-19：终端底部软键工具栏精简与按设备显示开关（US-H01、US-H07）

### 范围

- 故事编号与总数（264）保持不变，更新 `US-H01`、`US-H07` 与覆盖索引：
  - 将终端 `⋯` 菜单「显示」分组与「设置 → 外观与阅读」中的开关文案统一为「显示底部工具栏」（`Show bottom toolbar`），并提供「恢复默认工具栏」回退到按设备指针自动判断；
  - 将底部工具栏显隐从服务端 `/api/settings`（`forceSoftKeys`）迁移为当前浏览器本地偏好 `webmux.bottomToolbar`（`any-pointer: coarse` 默认显示，非触屏桌面默认隐藏，任意设备均可独立开启或关闭）；
  - 旧版 `settings.json` 中的 `forceSoftKeys` 兼容读取并在下次写入时落盘清理，`PATCH /api/settings` 携带 `forceSoftKeys` 返回 `400`；
  - 重排 `SoftKeys.svelte` 为 Windows RDP 风格紧凑单行工具栏：左侧固定「唤起或收起键盘」与 `Fn` 切换键，右侧固定「粘贴」、窄屏「草稿模式」与「快捷指令」；`>= 640px` 居中紧凑胶囊，`< 640px` 贴边紧凑单行条；
  - 主层提供 `Esc`、`Tab`、`↵`、`Ctrl`、`Alt`、`Shift` 与四个方向键，`Fn` 层原位切换为 `/`、`@`、`#`、`!`、`Home`、`End`、`PgUp`、`PgDn` 与 `Ctrl+C/D/Z/L/R`；
  - 移除双行 `TouchGamepad.svelte` 与 `touch-gamepad-preference.svelte.ts`，在底部工具栏方向键上直接支持原地长按连发与横向滚动拖动取消。

### 已执行验证与局限

- 静态与治理门禁：
  - `make check`（含 `make web`、`make format-check`、`make docs-check`、`make persistence-check`、`pnpm --dir web check:test`、`pnpm --dir web lint`、`pnpm --dir web check:arch`、`go vet`、`make go-lint` `golangci-lint` v2.15.0 0 issues）通过。
- 后端、前端、Storybook 与端到端回归：
  - `make test-go`（全量 Go `-race` 测试，含 `internal/settings`、`internal/wire`、`internal/api`、`internal/archtest` 与 `protocol/relay`）通过；
  - `pnpm --dir web test:unit`（89 个文件）通过；
  - Vitest 浏览器组件测试（`SoftKeys.test.ts`、`TerminalViewport.test.ts`、`SettingsNavigation.test.ts`、`SettingsStore.test.ts`、`DevicePreferences.test.ts`）通过；
  - `make test-storybook`（142 项）通过；
  - 隔离实例 Playwright 端到端套件（`softkeys.spec.ts`、`settings-sync.spec.ts`、`quick-commands.spec.ts`、`compact-chrome.spec.ts`、`i18n.spec.ts`、`composer.spec.ts`，覆盖桌面与 `320×568` 移动 Chromium）通过；
  - 在隔离实例采集桌面（`1280×900`）、平板（`768×800`）、手机（`390×844`）浅色与深色共 12 张前后对比截图及平板交互录屏，并在 `1280×900`、`1280×720`、`768×800`、`390×844`、`375×667`、`320×568`、`667×375` 视口校验无横向溢出且终端菜单完整落在视口内。
- 局限：
  - 验证在 Linux 本地隔离实例与无头 Chromium 中完成，未在物理触屏设备（真实 `any-pointer: coarse` 硬件、外接实体键盘切换）及 iOS / Android 真机软键盘环境下复测。

## 2026-11-19：移除 Git 工作台返工意见与会话协助起草说明（US-J07、US-J25、US-J28）

### 范围

- 故事编号与总数（264）保持原状，同步更新 `US-J07`、`US-J25`、`US-J28` 与覆盖索引：
  - 删除 `GitReworkDialog.svelte`、`git-rework-model.ts`、`git-rework-controller.svelte.ts`、`git-session-handoff.ts` 以及 `GitReworkItemSchema` 与 `ReworkSessionHandoff` Storybook 场景；
  - 移除 `GitWorkbenchTopBar.svelte` 与 `GitWorkbenchShellHeader.svelte` 的「返工意见」入口、`GitStagingHunkStrip.svelte` 的「加入意见」按钮，以及 `GitCommitComposer.svelte` 的「让会话协助起草说明」按钮；
  - `GitDiffSelectionBar.svelte` 收敛为 `Commit` 视图的逐行暂存、取消暂存与丢弃操作条（`US-J28`），在紧凑补丁回退模式下将阅读器行号还原为真实文件行号并拦截跨侧选区，`GitLogDiffPanel.svelte` 移除意见选区条；
  - 保留 `US-J25` 的跨设备审阅进度同步（`GET/PUT .../git/reviews`）与 `baseRevision` 乐观锁提交草稿及 `409` 并排恢复（`GET/PUT .../git/commit-draft`）。

### 已执行验证与局限

- 静态与治理门禁：
  - `make format-check`、`make docs-check`、`make persistence-check`、`make web`、`pnpm --dir web check:test`、`pnpm --dir web check:storybook`、`pnpm --dir web lint`、`pnpm --dir web check:arch`、`go vet` 与 `make go-lint`（`golangci-lint` v2.15.0，0 issues）通过。
- 前端与端到端回归：
  - Vitest 单元回归（`git-workbench.test.ts`、`git-collab.test.ts`、`git-v3-line-smart.test.ts`、`i18n.test.ts` 共 24 项）通过；
  - Vitest 浏览器组件回归（`GitWorkbenchRoute.test.ts`、`GitWorkbenchCommitPush.test.ts`、`GitWorkbench.test.ts`、`GitWorkbenchLineSmart.test.ts` 共 28 项，含紧凑补丁回退行号映射与选区条暂存/取消暂存/丢弃用例）通过；
  - Storybook 套件 `test/storybook/git-workbench.spec.ts`（26 项）通过；
  - 隔离实例 Playwright 端到端套件 `git-workbench-*.spec.ts`（9 个文件，桌面与 iPhone SE 共 52 项）通过；
  - 在隔离演示仓库采集桌面（`1280×900`）、平板（`768×800`）、手机（`390×844`）浅色与深色共 12 张前后对比截图，并在 `1280×900`、`1280×720`、`768×800`、`390×844`、`375×667`、`320×568`、`667×375` 视口下校验 `scrollWidth <= innerWidth`。
- 局限：
  - 验证在 Linux 本地隔离实例与无头 Chromium 中完成，未验证物理 `hover`、`any-pointer: coarse` 媒体查询及 macOS / iOS / Android 真机浏览器。

## 2026-11-19：Windows 安装程序卸载清理与清除个人数据选项（#704，US-O04、US-O12）

### 范围

- 故事编号、总数与覆盖索引保持不变；`docs/user-stories/clients-network-tray.md` 按主题拆分为树状索引与 `clients-network-tray/{native-cli,public-access,windows-tray}.md`，同步更新 US-O04 与 US-O12：
  - `internal/tray/traysys/quit.go`、`instance_windows.go`、`cmd/webmux-tray/main_windows.go`：`webmux-tray.exe quit` 新增 `--stop-keepalive`，在确认无托盘运行（退出码 0 或 3）后校验创建时间戳结束 `keepalive.json` 记录的 WSL 保活进程并删除该文件；其他位置托盘仍在运行（退出码 1）时保持保活现状。
  - `deploy/windows/webmux.nsi`：卸载前先 `SetOutPath "$TEMP"` 释放安装目录工作路径占用，默认调用 `quit --stop-keepalive`、删除 `%TEMP%\webmux-journal-*.log`、递归清理 `downloads` 与 `updates` 并对当前及旧路径执行 `firewall remove`；
  - `deploy/windows/webmux.nsi`、`internal/tray/traysys/autostart_windows.go`：清理指向本次安装路径的 `StartupApproved\Run\WebMuxTray`、`NotifyIconSettings`、`MuiCache` 与 `AppCompatFlags` 注册表记录及空父目录；
  - `deploy/windows/webmux.nsi`：新增“卸载选项”页与静默 `/PURGE` 参数递归删除 `%APPDATA%\WebMux` 与 `%LOCALAPPDATA%\WebMux`，并修正卸载完成页 `MUI_FINISHPAGE_TEXT` 宏绑定以按选项展示保留或 WSL 清理说明。
  - `.github/scripts/windows-installer-smoke.ps1`：新增 `quit --stop-keepalive`、多文件缓存递归清理、未登记防火墙规则清理、安装目录作为初始工作目录的卸载、`StartupApproved`/`NotifyIconSettings`/`MuiCache`/`%TEMP%` 痕迹清理、空父目录清理与 `/S /PURGE` 递归全量清理冒烟断言。

### 已执行验证与局限

- 本地跨平台单元测试、交叉编译与架构/文档门禁：
  - `go test -race ./internal/tray/...`、`GOOS=windows GOARCH=amd64 go vet ./cmd/webmux-tray ./internal/tray/...`、`GOOS=windows GOARCH=arm64 go build ./cmd/webmux-tray` 通过；
  - `make docs-check`、`make persistence-check` 与 `go test -count=1 ./internal/archtest/...` 通过。
- 局限：
  - 本地环境为 Linux，NSIS 安装包构建与真实 Windows 会话下的安装、防火墙与卸载冒烟由 GitHub Actions `Windows installer smoke` 作业执行验证。

## 2026-11-19：Windows 托盘局域网访问防火墙只读检测、按需放行与卸载清理（#705，US-O08、US-O12）

### 范围

- 更新 US-O08、US-O12 的防火墙检测、开启局域网访问放行交互、运行期状态复查及安装卸载衔接验收标准：
  - `internal/tray/traysys`：直接解析 Windows 防火墙注册表规则（本机、组策略与 MDM）与配置文件策略，并通过 Network List Manager 读取网卡类别（专用/域/公用）；
  - `firewall allow [--clear-blocks]` 在单次管理员提权内修复失效同名规则并可选清理指向当前托盘的本机入站阻止规则，安装版同步维护 `HKCU\...\Uninstall\WebMux\FirewallRule`；
  - `internal/tray` 与 `internal/tray/winui`：开启“允许局域网访问”需要转发时在绑定非回环监听前检查防火墙并按需单次申请放行，运行期菜单实时提示未放行、阻止规则覆盖或公用网络/策略受限并提供一键修复或说明；
  - `deploy/windows/webmux.nsi` 与 `.github/scripts/windows-installer-smoke.ps1`：卸载阶段对 `$INSTDIR\webmux-tray.exe` 及记录路径执行 `firewall remove`（无规则时零弹窗退出 0），补充规则禁用/篡改修复、阻止规则清理与运行期补加规则卸载回归。

### 已执行验证与局限

- 本地架构、文档与单元测试门禁：
  - `make format-check`、`make docs-check`、`make persistence-check` 与 `make go-lint` 通过；
  - `GOOS=windows golangci-lint run ./cmd/webmux-tray ./internal/tray/...` 通过；
  - `go test -race ./internal/tray/... ./internal/archtest/...` 通过，覆盖 `[MS-GPFAS]` 规则串解析、同名规则禁用/篡改/重复项修复、本机与策略阻止规则区分、多网卡专用/公用网络菜单评估及单批次提权序列；
  - Windows amd64 / arm64 交叉编译（`./cmd/webmux-tray`、`./cmd/webmux-windows`）与 `GOOS=windows go vet` 通过。
- 局限：
  - 本地环境为 Linux，未在本地物理 Windows 桌面会话上交互点击托盘菜单与 UAC 弹窗；Windows 注册表/NLM 实机探测与 NSIS 安装卸载防火墙冒烟由 GitHub Actions `Windows installer smoke`（`windows-latest`）执行验证。

## 2026-11-19：终端指针悬停分流与输入确认复位（US-D03、US-D06）

### 范围

- 故事编号、总数与覆盖索引保持不变，更新 US-D03、US-D06 的验收标准与 [WebSocket 接口](../protocol/websocket.md)：
  - 终端悬停分流（`web/src/domains/terminal/terminal-keys.ts`、`terminal-protocol.ts`、`web/src/hooks/terminal/terminal-socket.svelte.ts`、`terminal.svelte.ts`）：
    - 识别 DEC 1003 SGR 无按键指针悬停（`CSI < Cb ; Cx ; Cy M` 且 `(Cb & ~28) == 35`），仅在控制端按动画帧合并后经 `0x06` 自动回复通道发送，并在后续按键、点击或提交前按序刷出；
    - 观察端悬停保持观察身份，保留软修饰键与关注提醒；
  - `internal/hub/hub.go`：单条 `/ws/s/{id}` 连接的 FIFO 输入队列扩展至 256 帧与 2 MiB 待写字节上限，吸收程序启动期设备查询应答与鼠标追踪帧突发；
  - 输入确认跟踪（`web/src/platform/ws-handoff.ts`、`web/src/platform/ws.ts`）：
    - 按输入序号跟踪同连接的 `0x04` 查询回执与 `0x03` 终态回执（`written`、`failed`），覆盖此前输入后清除未确认标记；
    - 探测期间有后续按键时自动追加跟进探测，服务端以 `4410` 正常结束退出流时同步重置未确认标记。

### 已执行验证与局限

- 静态、架构、文档与 Lint 门禁：
  - `make check` 通过（覆盖 `pnpm --dir web build`、`format-check`、`docs-check`、`persistence-check`、`check:test`、`lint`、`check:arch`、`go vet` 与 `golangci-lint`）。
- Go 竞态与前端单元、浏览器、E2E 测试：
  - `make test-fast` 通过，并行运行 Go `-race` 全包、`protocol/relay`、Vitest Node 单元套件与无头 Chromium 浏览器组件套件；
  - 新增回归用例覆盖 `TestInputQueueAbsorbsStartupBurstAndBoundsBytes`、`test/input.test.ts`、`test/terminal-protocol.test.ts`、`test/ws.test.ts` 与 `test/browser/TerminalTransport.test.ts`；
  - `WEBMUX_E2E_BINARY=$(realpath .cache/webmux-e2e/webmux) pnpm --dir web exec playwright test test/e2e/terminal.spec.ts` 通过。
- 局限：
  - 浏览器与 E2E 验证在 Linux 无头 Chromium 中执行，未在物理触控设备或 Safari 真机上复测。

## 2026-11-19：新会话首次连接时同步终端主题（US-D03、US-D06）

- 首次附着前由服务端应答实时 DA1 查询，让 OMP 等程序结束启动探测；控制端连接后，新发出的 OSC 11 查询取得实际背景色。浏览器和原生附着的历史回放不补发启动应答。
- `go test -race ./internal/session ./internal/attach -run 'Test(Probe|OSC|RealSessionAnswersProbe|ReplayDoesNotAnswerProbe|ColorScheme|AttachReportsNativeColorScheme)'` 通过。
- `pnpm --dir web exec playwright test test/e2e/terminal-replies.spec.ts test/e2e/attention.spec.ts -g 'a new TUI learns|reopening suppresses|opening a session does not clear attention' --workers=3`：桌面及手机模拟视口共 8 项通过。
- 在本地隔离 WebMux 与无头 Chromium 中运行 `omp --no-session`，等待启动探测写入 PTY 后才打开终端；确认 OMP 状态栏采用浅色，切换黑色再切回白色均重新查询并重绘。未使用真机。

## 2026-11-19：全量代码审查 P0/P1 修复核对（US-A08、US-A10、US-D03、US-F06、US-H12、US-J20–US-J22、US-J26–US-J28、US-Q03、US-Q12、US-R01）

### 范围

- 故事编号、总数（264）与覆盖索引保持不变，同步更新受影响故事与协议说明：
  - 端口转发与 Relay 鉴权（`US-R01`、`US-A08`、`docs/protocol/http-forward.md`）：`/forward/` 校验允许来源与 `Sec-Fetch-Site`，外站顶层文档导航 303 至 `/forward-auth?confirm=1`，Relay 所有者浏览器会话同源转发写请求校验精确 `Origin`；
  - 语音、会话伴随进程与托盘桥接（底层保障）：
    - `voice` 单协程串行发送事件并在锁外关闭满载管道，`Session.Output` 有界排空；
    - `forceSession`/`CleanupOrphan` 纳入标记与子孙树，8 MiB 回放放行 0 字节控制帧；
    - `tailscaleController.ownership` 与 `WSLService`/`localclient`/`bridgeConn` 消除竞态并回收连接；
  - 混合会话、ACP、Relay 与 P2P（`US-Q03`、`US-Q12`、`US-A10`、`US-A08`、`US-H12`）：
    - `config_reapply` 保护缺失项索引并克隆切片，`runToACP` 停止超时转入 `switchAbandoned`；
    - `acp.Conn` 非阻塞请求取消与有界写超时，`hybrid.Service.Close` 等待分离子任务并将关闭中会话落盘为 `StateClosed`，重启清理标记与脱离进程组的残留进程；
    - Relay 连接稳定达租期后重置退避，P2P 回收每会话 ICE 配额并支持动态 TURN 续期及前端优雅降级排空；
  - 终端草稿与输入确认（`US-F06`、`US-D03`、`US-H12`）：
    - `TerminalDrafts` 动态委托当前激活主机存储命名空间，P2P 端口映射轮询遇瞬时失败继续探测；
    - 在 `#791` 输入确认跟踪基础上补齐 `4404` 会话移除重置、前台探测跟进，以及手动重同步、视图切回或关闭提示条清除 `inputUncertain`；
  - Git 工作区与前端工作台（`US-J20`–`US-J22`、`US-J26`–`US-J28`）：
    - `ExecuteSmartCheckout` 按 `clientRequestId` 幂等重放，大文件（`> 32 MiB`）流式哈希与未跟踪嵌套 Git 仓库识别，`GitStashApplyResultSchema` 补齐 `needs_review`；
    - 冲突解决器锁定保存期切换并按身份缓存草稿与上下文锚点切片，各确认弹窗参数变更与异常立即失效旧 `confirmationId`；
    - 跨目标切换重置 `push`/`staging` 等子控制器，快照下载预捕获文件名并绑定取消，`CodeDiffReader` 与四个工作台差异面板对已删除/清空文件回退检索删除行。

### 已执行验证与局限

- 静态与治理门禁：
  - `make check`（含 `make web`、`make format-check`、`make docs-check`、`make persistence-check`、`pnpm --dir web check:test`、`pnpm --dir web lint`、`pnpm --dir web check:arch` 49/49、`go vet` 与 `golangci-lint` v2.15.0 0 issues）通过；
  - `make test-install`、`make tray`、`GOOS=windows go vet`、`GOOS=darwin go vet` 与 `go test -tags nop2p ./internal/p2p` 通过。
- 全量测试套件：
  - Go `-race`（`make test-go`，95 个包）、Vitest 单元测试（89 个文件，1,265 项）、Vitest 浏览器测试（120 个文件，1,008 项）、Storybook（142 项）、Playwright 端到端测试（546 项通过，56 项按设备跳过）与 4× CPU 性能回归（18 项通过，2 项跳过）全部通过；
  - 在隔离演示服务采集桌面（`1280×900`）、平板（`768×800`）、手机（`390×844`）浅色与深色前后对比截图及桌面交互 GIF，并在 `1280×900`、`1280×720`、`768×800`、`390×844`、`375×667`、`320×568`、`667×375` 视口校验 `scrollWidth <= innerWidth`。
- 局限：
  - 验证在 Linux 本地隔离实例与无头 Chromium 中完成，未验证物理 `hover`、`any-pointer: coarse` 媒体查询、原生 Windows 托盘交互、真实公网 Relay/TURN 节点及 macOS / iOS / Android 真机浏览器。

## 2026-11-19：手机工作区顶栏统一面板按钮（US-H08）

- 「打开面板」与文件浏览、显示菜单共用无边框图标按钮，窄屏下三者均为 44 × 44 CSS 像素。「打开面板」保留无障碍名称、提示和展开状态。
- `pnpm --dir web run check`、`check:test`、`lint`、`check:arch`（49 项）、Oxfmt 与 `make docs-check` 通过。
- `pnpm --dir web exec playwright test test/e2e/compact-chrome.spec.ts test/e2e/session-pane-files.spec.ts` 在预构建的隔离服务上通过 7 项，1 项按项目条件跳过。
- 隔离服务中使用 Chromium 核对七种视口的浅色与深色主题。页面无横向溢出，面板选择器在视口内可打开和关闭，桌面键盘焦点与鼠标悬停反馈可见。采集桌面、平板与手机顶栏的前后对比截图。触控通过模拟设备核对，未使用实体设备。

## 2026-11-19：窗格活动标签下划线（US-D14、US-I04）

- 活动标签的 2 px 细线与标题栏底边对齐，横跨标题和关闭按钮，并在两侧等距内缩。标签栏出现横向滚动条时，细线位于滚动条上方。
- `pnpm --dir web check`、`pnpm --dir web lint`、`pnpm --dir web check:arch`、`pnpm --dir web exec vp fmt --check src` 与 `make docs-check` 通过。
- Playwright 运行 `session-pane-files.spec.ts` 和 `splits.spec.ts`，22 项通过，8 项按项目条件跳过。
- 在有界面的 Chromium 隔离实例中核对七种视口的浅色与深色主题，验证细线位置、页面横向溢出、鼠标悬停、键盘切换、长文件名截断和标签栏横向滚动。桌面、平板和手机各采集浅色与深色前后截图。触屏使用 Chromium 模拟。

## 2026-11-20：分屏快速新建 Shell 采用服务端默认登录 Shell（US-D13、US-B01）

- 故事编号与总数（265）保持不变，同步更新 `US-D13`、[创建会话 API](../protocol/http-session-create.md) 与[终端使用](../usage/terminal.md)：
  - `POST /api/sessions` 省略 `cmd` 与 `configId` 时，由服务端 `sessioncreate` 启动注入的默认登录 Shell（`assemble.DefaultShellCmd(os.Getenv("SHELL"))`，即 `[$SHELL, "-l", "-i"]`，未设置 `$SHELL` 时回退 `["/bin/sh", "-l", "-i"]`）；显式传入 `cmd: []` 或 `cmd: [""]` 仍在预检返回 400。
  - 前端 `createSplitShellSession` 移除硬编码 `cmd: ["/bin/sh", "-i"]`，仅传递 `cwd` 与可选 `workspace`。
- 已执行验证：
  - Go `-race` 测试：`go test -race -count=1 ./cmd/internal/assemble ./internal/sessioncreate ./internal/api ./internal/mcpcontrol ./internal/archtest ./internal/docguard` 通过。
  - 静态与格式门禁：`make format-check`、`make docs-check`、`make persistence-check`、`pnpm --dir web build`、`pnpm --dir web check:test`、`pnpm --dir web lint`、`pnpm --dir web check:arch`（49 项）、`go vet ./...` 与 `GOOS=darwin go vet ./...` 通过。
  - 前端单元与端到端测试：`test/terminal-layout.test.ts`（6 项）、Playwright `test/e2e/session-creation.spec.ts` 与 `test/e2e/split-background.spec.ts`（22 项通过，含快速分屏验证 `cmd` 为 `["/bin/bash", "-l", "-i"]`）以及 `test/e2e/splits.spec.ts` 中 `stacked panes` 用例通过。
- 局限：
  - 本机 `golangci-lint`（以 Go 1.26 编译）低于仓库 `go.mod` 目标版本（Go 1.27.4），`make go-lint` 由 CI 环境复核；本机 `/tmp` 受磁盘配额限制，`cmd/webmux` 依赖 `/tmp` 的子测试与 `splits.spec.ts` 写入 `/tmp` 截图路径的用例由 CI 全量运行覆盖。

## 2026-11-21：精简工作台页头（US-C02、US-C16、US-C17、US-J20）

- Git 与新建会话同排，搜索框同行显示输出预览开关；会话总数保留在「全部」筛选项。
- `pnpm --dir web build`、`pnpm --dir web lint`、`pnpm --dir web check:arch`（49 项）通过。
- `pnpm --dir web exec playwright test test/e2e/repository-scope.spec.ts test/e2e/git-workbench-route.spec.ts -g 'repository scope limits the home list|the Git and session views switch in both directions'` 通过 4 项，包含 320px 下 200% 文字放大。
- 隔离实例用 Chromium 核对桌面、平板、390/375/320px 手机与 667×375 横屏的浅色和深色。搜索与开关同行、Git 打开当前工作树，手机按钮保留至少 44px 高度，页面无横向溢出。触屏为模拟，未使用真机。
- 390/375/320px 手机模拟分别检查浅色和深色：标题保持单行，主工作区标记在侧栏工作树行可见，侧栏「刷新工作区」发出请求，Git 直达当前工作树。
- Playwright 对 `repository-scope.spec.ts`、`settings-sync.spec.ts`、`token-link.spec.ts` 和 `worktrees.spec.ts` 按范围、刷新、旧响应和鉴权用例执行 `-g` 筛选，桌面和 iPhone SE 共 12 项通过。`pnpm --dir web check:test` 通过。
  - 设置同步和鉴权用例曾把关闭动画中的抽屉当作可交互入口。测试夹具改为在手机显式展开侧栏，等待导航稳定后定位刷新按钮。
  - 手机 390×844 深色演示录制从侧栏刷新并返回的过程。

## 2026-11-21：单端 PR 跳过另一端 CI 与路径分类收窄（US-L02）

- 故事编号与总数保持不变，同步更新 `US-L02`、[测试规则](../agent-rules/testing.md)、[CI 耗时优化](../ci-performance.md) 与 [CONTRIBUTING.md](../../CONTRIBUTING.md)：
  - 纯后端（`go=true, web=false`）PR 跳过 `E2E runtime` 与 4 个 Playwright E2E 分片；`Go race` 不再依赖 `E2E runtime`，改为直接构建内嵌前端后独立运行。
  - 纯前端（`go=false, web=true`）PR 跳过 `make test-install`，保留前端套件与 E2E；`.github/pr-assets/**` 截图证据不再命中 `.github/` 全量兜底。
  - `go.mod`、`go.sum`、`.golangci.yml`、`install.sh`、`protocol/**` 归入纯后端，`web/` 依赖与配置归入纯前端；跨端契约/夹具（wire 快照、`fakeagent`、`embed-assets.ts`、`sw.js`、`fake-agent-names.txt`）仍同时触发双端。推送到 `main` 继续强制完整套件。
- 已执行验证：
  - `bash -n .github/scripts/classify-changes.sh`、`bash -n .github/scripts/test-classify-changes.sh` 与 `bash .github/scripts/test-classify-changes.sh`（覆盖 23 组单路径/组合路径场景）通过。
  - `.github/workflows/test.yml` YAML 解析、`.github/pr-assets/` 排除过滤回归、`make docs-check`、`make persistence-check`、`go test -count=1 ./internal/archtest -run '^(TestAgents|TestGenericCodeNamesNoAgent)'` 与 `make format-check` 通过。
- 局限：
  - 本次修改涉及 `.github/`，PR 自身按安全规则在 GitHub Actions 触发全量运行；单端跳过效果在合入 `main` 后对后续纯前端/纯后端 PR 生效。

## 2026-11-21：可搜索选择器的通栏搜索

- 范围：启动方式、位置、目录、项目、会话切换、分屏、模型、工作树和分支。故事编号与总数保持不变。
  - 关联 US-B01、US-B04、US-B06、US-B10、US-B12、US-C18、US-D13、US-Q07、US-Q18、US-J03–US-J05、US-J14–US-J16、US-J21、US-J22。
- 已执行验证：
  - `pnpm --dir web check`、`pnpm --dir web lint`、`pnpm --dir web check:arch`（49 项）、`make format-check` 与 `make docs-check` 通过。
  - 使用 `vp test run --config vitest.browser.config.ts` 运行目录、快速开始、项目、会话切换、分屏、切换分支、工作树创建及分支操作测试，87 项通过。
  - 使用同一浏览器配置运行 `GitWorkbenchCommitPush.test.ts`，7 项通过，包含顶栏分支浮层。
  - 使用 `vp test run --config vitest.config.ts` 运行 `home-worktrees.test.ts`、`git-branches-and-staging.test.ts` 与 `commit-choices.test.ts`，16 项通过。
  - 在 Chromium 中运行快速开始位置选择器，检查桌面、平板、窄屏和横屏下的明暗主题、长分支名、浮层边界、焦点底线，以及按关键字筛选后用 Enter 选择。
- 局限：视觉验证使用 Chromium 和演示仓库数据，未在真机上执行。

## 2026-11-22：Windows 托盘开机 WSL 异步检测与退避重试（US-O04）

- 范围：更新 `US-O04` 验收要点（故事编号与总数不变），将托盘启动阶段的 WSL 检测改为异步执行。
  对 `wsl.exe` 超时及 `--list --verbose` 瞬态失败按退避自动重试，WSL 就绪后自动接管启动 WebMux；
  失败后打开菜单也可触发重新检测并恢复。
- 已执行验证：
  - `go test -race -count=1 ./internal/tray/...` 通过（含新增 `TestCheckWSLClassification` 与 5 项 `TestWSLBoot*` 用例）。
  - `GOOS=windows GOARCH=amd64 go build -o /dev/null ./cmd/webmux-tray` 与 `GOARCH=arm64` 交叉构建通过。
  - `GOOS=windows go vet -printf.funcs=problemf,invalidf,usagef,checkf ./cmd/webmux-tray ./cmd/webmux-windows ./internal/tray/... ./internal/wslclient/... ./internal/attach/... ./internal/clicmd/...` 通过。
  - `GOOS=windows GOARCH=amd64 golangci-lint run ./cmd/webmux-tray/... ./internal/tray/...` 与 `make go-lint` 均 0 告警通过。
  - `make docs-check`、`make persistence-check`、`go test -count=1 ./internal/archtest ./internal/docguard` 通过。
- 局限：未在本机直接重启 Windows 验证开机真实唤醒链路，Windows 安装与 `quit` 冒烟交由 CI 的 `windows-installer` 作业验证。

## 2026-11-22：主分支 CI 构建与中继撤销用例（US-L01）

- 前端依赖安装生成 Paraglide 声明，CI 两套 `svelte-check --tsrs` 检查复用声明，生产 `vp build` 单独运行。
- 独立工作树中 `pnpm --dir web install --frozen-lockfile`、两套 `svelte-check --tsrs`、`pnpm --dir web exec vp build` 与 `pnpm --dir web check:arch`（49 项）通过。
- `go test -race` 定向覆盖 `TestConnectorRevocationClosesEveryPooledPhysical` 的三条结束路径；其中 `peer-fin` 子用例单独重复 100 次通过。

## 2026-11-23：手机会话操作移到工作区顶栏

- 范围：US-D08、US-D14、US-G02、US-H07、US-H08、US-Q01。手机标题行保留身份与状态；顶栏菜单操作当前会话，「打开面板」收起及恢复窗格。
- `playwright test` 定向验证 `compact-chrome`、`session-header`、`hybrid`、`splits` 与 `softkeys`，12 项通过；调整快捷指令子菜单定位后，手机软键栏用例再次通过，`session-pane-files` 两项通过。
- `pnpm --dir web check`、测试和 Storybook 的 `svelte-check`、`pnpm --dir web lint`、`pnpm --dir web check:arch` 的 49 项及 `make docs-check` 通过。
- 隔离生产构建用 Chromium 核对 1280×900、768×800、667×375、390×844、375×667 与 320×568 的明暗主题。页面无横向溢出；菜单留在视口内，Esc 后焦点回到按钮；手机六枚顶栏按钮均为 44×44px。
- 录制 390×844 深色视口的混合会话视图切换、快捷指令、收起和恢复；未在 iOS 或 Android 真机验证。

## 2026-11-26：浏览器端会话切换 LRU 缓存与后台分级刷新（US-D03）

- 范围：更新 `US-D03` 验收要点（故事编号与总数不变），实现浏览器端最近使用会话页缓存池（桌面 5 / 移动 3）、Active / Warm / Cool 三态后台刷新、WebGL 后台宽限期释放与按需恢复、WebSocket `suspend`/`resume` 增量续接及前后台副作用隔离。
- 已执行验证：
  - `make format`、`make check`（含 `svelte-check`、`vp build`、`make docs-check`、`make persistence-check`、`pnpm --dir web check:arch` 49 项、`go vet` 与 `golangci-lint`）全部通过。
  - `pnpm --dir web exec vp test run --config vitest.config.ts` 全量 97 个单元测试文件（1335 项测试，含新增 `session-cache.test.ts` 与 `ws-suspend.test.ts`）全部通过。
  - `pnpm --dir web exec playwright test test/e2e/session-switch-cache.spec.ts test/e2e/browser-chrome.spec.ts test/e2e/continuity.spec.ts test/e2e/app-shell.spec.ts` 32 项通过（0 失败）。
- 局限：未在 iOS / Android 真机上实测低内存 WebGL 上下文回收行为。
