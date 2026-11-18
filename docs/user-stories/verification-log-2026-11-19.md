# 核对记录：2026-11-19 起

承接 [verification-log-2026-11-18](verification-log-2026-11-18.md)，收录 2026-11-19 起的局部核对记录；索引见 [WebMux User Stories](../user-stories.md#核对记录)。

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
