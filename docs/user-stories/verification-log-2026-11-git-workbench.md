# Git 工作台功能核对记录（2026-11-12 起）

本页记录独立 Git 工作台（US-J19–US-J28）各阶段契约、路由、界面、持久操作账本与冲突解决能力的实现与验证证据，按提交顺序升序排列；故事索引与范围约定见 [WebMux User Stories](../user-stories.md) 与 [功能覆盖核对](coverage.md)。

## 2026-11-12：Git 工作台 02（V1a）仓库与工作树级 Git REST API 及契约快照（US-J19）

- 范围：交付零活跃会话下可用的仓库与工作树级 Git REST API、契约快照及前端请求封装：
  - 路由：`/api/repositories/{repoId}/git/*` 10 个只读端点与 `/api/repositories/{repoId}/worktrees/{worktreeId}/git/*` 16 个只读及暂存/提交端点；
  - 契约与错误映射：`internal/contract`、`internal/wire`、`internal/api` 错误细分（`readonly`、`stale`、`busy`、`context_mismatch`、`commit_blocked`、`commit_failed`）、26 份 wire 快照，以及前端 `git-api-schemas.ts` 与 `git-api.ts`。
- 依据：
  - `go test -race ./internal/api ./internal/wire ./internal/codeview ./internal/workspace ./internal/archtest ./internal/docguard` 通过：
    - 覆盖鉴权/Origin 拦截、零活跃会话读写、历史车道图分页、合并提交父切换、跨仓 404，以及跨工作树/过期上下文/只读/锁冲突 409；
    - 覆盖单 hunk 暂存与取消暂存、外部 CLI 暂存预览、客户端断连下已进入 `git commit` 的原子完成及日志脱敏；
  - `go test ./internal/api -run TestWireSnapshots` 与 `pnpm --dir web exec vitest run --config vitest.config.ts` 通过，校验全部后端响应快照、数组契约与前端 API 封装；
  - `make check`（含 `make web`、`pnpm --dir web check:arch`、`make docs-check`、`make persistence-check`、`go vet` 与 `golangci-lint`）通过。
- 限制：V1a 仅交付 REST 契约、后端路由与前端 typed client/schema，尚未挂载 Git 工作台浏览器页面，未执行工作台 UI 截图或真实移动端浏览器交互验收。

## 2026-11-12：Git 工作台 03（V1a）独立 `/git` 路由、`AppShell` 集成与多入口联动（US-I01、US-I02、US-J20）

- 范围：
  - 新增壳内独立路由 `/git`、`/git/{repositoryId}` 与 `/git/{repositoryId}/worktrees/{worktreeId}`，支持 `view`、`scope`、`branch`、`commit`、`q` 与 `returnSession` 深链查询参数及 `location.search` 响应式 `popstate` 同步；
  - 打通仓库菜单与仓库面板头部（“Git 工作台”）、工作树菜单（“在 Git 工作台打开”与有改动时的“提交改动…”）及会话代码工作区头部（“在 Git 中查看”与顶栏“返回会话”）三类入口，实现代码阅读状态同步捕获与无损返回原会话；
  - 实现顶栏固定操作目标（仓库切换、工作树选择、检出分支/`HEAD` 徽标）与历史筛选引用的语义分离、只读仓库与异常工作树写门禁、按 `(hostId, repositoryId, worktreeId)` 隔离视图状态与草稿记忆，以及按设备尺寸档位（`desktop`/`compact`/`tablet`/`phone`）隔离分栏比例与 Diff 显示偏好。
- 依据：
  - 单元与架构守卫：`pnpm --dir web check:arch`、`test/route.test.ts`、`test/git-workbench-memory.test.ts` 与 `go test -count=1 ./internal/docguard ./internal/archtest` 通过；
  - 浏览器组件测试：`test/browser/GitWorkbenchRoute.test.ts`、`test/browser/GitWorkbench.test.ts`、`test/browser/WorkbenchTree.test.ts`、`test/browser/RepositoryPanel.test.ts` 与 `test/browser/RepositoryGroup.test.ts` 通过；
  - Playwright 端到端与视觉验证：
    - `test/e2e/git-workbench-route.spec.ts`（`chromium-desktop` 与 `chromium-mobile`）验证零会话不启动 PTY、最后会话删除存活、只读仓库 `Commit` 选工作树门禁、会话代码区往返恢复聊天草稿与阅读滚动位置、跨视图及 `Back`/`Forward` 状态恢复、跨工作树草稿隔离与单一 `/ws/sessions` 连接；
    - 在真实运行应用上完成桌面（1280×900）、平板（768×800）、手机（390×844）浅色与深色截图（含修改界面的前后对比）、交互 GIF 录制及 `scrollWidth <= innerWidth` 横向溢出检查。
- 限制：无头 Chromium 的 `hover` 与 `any-pointer: coarse` 媒体特性与物理设备不同，行尾悬停显隐与物理触屏手感未验证，也未在真机 iOS/Android 浏览器上验证。
