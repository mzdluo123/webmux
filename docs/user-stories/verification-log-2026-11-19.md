# 核对记录：2026-11-19 起

承接 [verification-log-2026-11-18](verification-log-2026-11-18.md)，收录 2026-11-19 起的局部核对记录；索引见 [WebMux User Stories](../user-stories.md#核对记录)。

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
