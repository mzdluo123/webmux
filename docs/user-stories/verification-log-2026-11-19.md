# 核对记录：2026-11-19 起

承接 [verification-log-2026-11-18](verification-log-2026-11-18.md)，收录 2026-11-19 起的局部核对记录；索引见 [WebMux User Stories](../user-stories.md#核对记录)。

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
