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

## 2026-11-12：Git 工作台 04（V1a/V1b）真实数据接入的 Log 视图——拓扑图、分页筛选、提交详情、文件 Diff 与两版本比较（US-J20、US-J21）

- 范围：
  - 服务端引用与范围增强：在 `BranchesResult` / `/api/repositories/{id}/branches` 与 CLI `repo git branches` 中新增 `tags`（附注标签自动解包至目标提交 OID），扩展 `resolveNamedRefTip` 支持 `refs/tags/<name>` 历史筛选，并重录 wire 快照；
  - 左侧引用筛选树与检出解耦：分组展示收藏分支（按主机与仓库持久化）、当前分支、本地分支、远端跟踪分支与标签，单击仅筛选历史而不切换工作树或磁盘 `HEAD`，行尾菜单“检出分支…”桥接预选分支的 `SwitchBranch` 预览确认；
  - 快捷范围与组合筛选：支持快捷范围（含受控 `#547`“当前任务”说明弹层）、关键字、作者、起止日期、字面路径组合筛选、选中提交消失提示及一键“定位当前 HEAD”，同步扩展 URL 查询参数 `author`、`since`、`until`、`path`；
  - `%P` 拓扑分页与提交详情：对接 `/git/history` 游标流式追加、`indirect` 虚线边与 `refsChanged` 提示；右上变更文件列表（键盘导航、路径二次过滤、重命名与二进制标记）与右下提交详情（合并提交多父节点切换、独立提交者、OID 复制、父提交跳转）联动；
  - Pierre `CodeDiffReader` 与两版本比较：复用 `CodeDiffReader` 提供底部可折叠/拖拽高度差异栏、放大态、手机端单列三步 Unified 阅读及快照全文/下载；支持任意两提交只读比较、交换方向与 `LatestRequest` + `AbortController` 竞态隔离。
  - 2026-11-19（T063）：隔离 Chromium 中，桌面详情在普通、比较和交换方向三态保持 206.52 px 阅读视口。比较从顶部开始，退出恢复 310 px 阅读位置，交换方向后焦点留在方向按钮。
- 依据：
  - Go 单元与竞态测试：`go test -race ./internal/workspace ./internal/codeview ./internal/wire ./internal/api ./internal/clicmd ./internal/archtest ./internal/docguard` 通过（含 `TestBranchesTagsAndHistoryTagFiltering` 及 wire 快照校验）；
  - 前端架构与单测门禁：`make check`、`test/git-log-controller.test.ts`、`test/route.test.ts` 与 `test/git-workbench.test.ts` 通过；
  - 浏览器与 Storybook 测试：`test/browser/GitLogController.test.ts`、`test/browser/GitWorkbench.test.ts`、`test/browser/GitWorkbenchRoute.test.ts` 与 `pnpm --dir web test:storybook`（含多视口溢出巡检）通过；
  - Playwright 端到端与视觉验证：
    - `test/e2e/git-workbench-log.spec.ts` 与 `test/e2e/git-workbench-route.spec.ts`（桌面与窄屏项目）验证分支/标签筛选不改写磁盘 `HEAD`、显式“检出分支…”预选弹窗、路径过滤 `indirect` 虚线边、游标分页追加与合并父节点 `#1/#2` 切换；
    - 同一 E2E 进一步验证任意两提交比较与方向交换、放大 Diff 返回保留列表滚动位置、手机窄屏三步导航零横向溢出，以及全程 `.git/index` 内容摘要与暂存区零改动；
    - 在真实运行实例上完成桌面（1280×900）、平板（768×800）、手机（390×844）浅色与深色前后对比截图及常速交互 GIF 录制与逐张视觉核查。
- 2026-11-19（T061）：隔离 Chromium 用 122 个引用验证桌面和抽屉内的引用分组。未触及滚动边界时按钮纵坐标保持，页尾收起时焦点随按钮平滑到达新的滚动边界，开启减少动态效果时直接到位。
- 限制：无头 Chromium 的 `hover` 与 `any-pointer: coarse` 媒体特性与物理设备存在差异，拖拽分隔条的真实触控阻尼与真机 iOS/Android 浏览器表现未在物理设备上验证。

## 2026-11-12：Git 工作台 05（V1a）Commit 暂存闭环、三处共享分支动作与宽幅 Push 对话框（US-J22）

- 范围：
  - 服务端推送预览增强：`PushPreview` 上限提升至 `maxPushListedCommits = 500` 以完整列出待推提交，支持通过工作树或仓库级提交详情/单文件差异接口检视任意待推提交内容；
  - `Commit` 真实 `index` 暂存与提交闭环：左侧 `Changes` 四组树实现阅读高亮、`identityKey` 绑定的“已审阅”与真实 Git `index`“已暂存”三态严格分离，支持分组批量、整文件及常驻不依赖悬停的逐块 `Stage`/`Unstage`、降级文件整文件兜底、冲突跳转终端及外部改动过期快照门禁；
  - 全量 `HEAD -> index` 提交预览与三态复核：拆分标题/正文编写器（`Ctrl/Cmd+Enter` 仅打开预览），预览框支持逐文件展开阅读真实 `HEAD -> index` Pierre Diff、`409 stale` 原位刷新预览，以及 `matched`、`needs_review`（Hook 改写树自动中止串联推送）与 `uncertain` 三态复核；
  - 三处共享分支动作与宽幅 `GitPushDialog`：
    - 顶栏分支弹层、`Log` 引用树与 `Branches` 视图统一复用共享分支模型、工作树占用保护及 `⋯`/右键/`Shift+F10` 菜单；
    - 桌面 ~900px 双栏、手机全屏推送确认框支持浏览全部待推提交及其 Pierre 文件 Diff，并在“提交并推送”第二步取消或失败时独立保留本地提交且提供 Fetch/Update 恢复入口；
  - 文档树重构：因 `worktrees.md` 超过 48 KiB 体积上限，保留 `worktrees.md` 为树状索引，原文拆分为 `worktrees/lifecycle.md`（US-J01–US-J18）与 `worktrees/git-workbench.md`（US-J19–US-J22）。
- 依据：
  - Go 单元与竞态测试：`go test -race ./internal/workspace ./internal/codeview ./internal/wire ./internal/api ./internal/archtest ./internal/docguard` 通过；
  - 前端架构与单测门禁：`make check`、`test/git-branches-and-staging.test.ts` 与 `test/git-workbench.test.ts` 通过；
  - 浏览器与 Storybook 多视口测试：`test/browser/GitWorkbenchCommitPush.test.ts`、`test/browser/GitWorkbench.test.ts` 与 `pnpm --dir web test:storybook`（含 `git-workbench.spec.ts` 12 视口档位布局与对话框可达性巡检）通过；
  - Playwright 端到端与视觉验证：
    - `test/e2e/git-workbench-commit-push.spec.ts`、`test/e2e/git-workbench-log.spec.ts` 与 `test/e2e/git-workbench-route.spec.ts`（桌面与窄屏项目）验证真实单块 `index` 暂存、外部修改自动失效“已审阅”、`409 stale` 一键重生成预览与 `pre-commit` Hook 改写树返回 `needs_review` 中止自动推送；
    - 同一 E2E 进一步验证远端超前两步独立报错、首次发布分支、工作树占用保护与 `Shift+F10` 检出脏文件/活跃会话警告，并在真实运行实例上完成桌面、平板、手机浅色与深色前后对比截图及常速交互 GIF 核查。
- 限制：无头 Chromium 的键盘焦点样式与物理移动设备触控反馈无法完全替代真机 iOS/Android 浏览器验收；外部 GPG 硬件密钥交互签名未在无头 CI 环境实测。

## 2026-11-12：Git 工作台 06（V1a）持久 Git 操作账本、幂等恢复与底部状态栏（US-J23）

- 范围：
  - `internal/statefile`、`internal/archtest/persistence_inventory.json`、`cmd/webmux/configview.go` 与 `docs/persistence.md` 登记 `git-operations.json`（`0600` 原子替换 + 独立锁，单库 100 条、全局 1000 条、7 天保留、2 MiB `Fits` 上界、注销仓库同步清理、输出经凭据脱敏截断 ≤ 4 KiB）；
  - `internal/workspace` 解耦两阶段预览凭证 `confirmationId` 与持久实例 `operationId` + `clientRequestId`：
    - 对 `commit`、`push`、`fetch`、`fast_forward`、`switch` 实现在途请求合并、跨重连/重启幂等重放与参数冲突 `409 idempotency_conflict`；
    - 启动时按 `GIT_REFLOG_ACTION=webmux-commit:<operationId>` 与本地引用只读对账崩溃遗留条目（零自动重试、零 `git reset`）；
  - `internal/api`、`internal/contract`、`internal/wire` 新增 `GET /api/repositories/{id}/git/operations` 与 `GET /api/repositories/{id}/git/operations/{operationId}` 及线上快照；
  - `web/src` 新增 `git-operations-model.ts`、`git-operations-controller.svelte.ts` 与 `GitWorkbenchStatusBar.svelte`，替换工作台纯静态底栏为 24–28px 实时状态栏与操作历史 `Popover`，接通网络中断自动查询恢复及 `uncertain` 防盲目重提门禁。
- 依据：
  - `make persistence-check`、`make docs-check`、`make check` 通过；
  - `go test -race ./internal/workspace ./internal/api ./internal/wire ./internal/statefile ./internal/archtest ./cmd/webmux` 通过；
  - 前端 `git-operations.test.ts`、`GitWorkbenchOperations.test.ts`、Storybook 套件与隔离实例 Playwright E2E（`git-workbench-operations.spec.ts`、`git-workbench-commit-push.spec.ts`，覆盖桌面与 320px 手机视口及响应中断无重复提交）通过。
- 限制：仅在 Linux 本地隔离实例与无头 Chromium 中核验；无头浏览器未验证 `hover` 与 `any-pointer: coarse` 媒体查询，未在 macOS 或 iOS/Android 物理真机上核对触屏手感；未重新核对本组以外的其余故事。

## 2026-11-13：Git 工作台 07（V1b）固定任务审阅基线、单份净差异审阅与交付收尾摘要（US-J19、US-J21、US-J24）

- 范围：
  - `internal/statefile`、`internal/archtest/persistence_inventory.json`、`cmd/webmux/configview.go` 与 `docs/persistence.md` 登记 `git-task-baselines.json`；
  - `internal/workspace` 实现固定任务审阅基线与任务历史范围：
    - 新建工作树自动记录 `worktree_create` 基线，`unborn` 空仓首个会话启动记录 `unborn: true`（不伪造全零 OID），支持手动指定/清除（`manual_select` + 诚实记录设定前已存在修改）；
    - 非 `unborn` 基线写入 `refs/webmux/baselines/<worktreeId>` 防 `git gc` 回收，删除/批量删除/释放工作树与注销仓库同步清理，并接通 `GET .../git/history?scope=task`（`baseOid..HEAD` 与 `no_task_baseline`）；
  - `internal/codeview`、`internal/api`、`internal/contract`、`internal/wire` 新增 `GET/PUT/DELETE .../git/task-baseline`、`GET .../git/task-changes` 与 `GET .../git/task-diff`（单份净差异合并不重复计数，写入 `/git/snapshots/{snapshotId}`）及线上快照；
  - `web/src` 新增 `git-task-api.ts`、`git-snapshot-diff-reader.svelte.ts`、`git-task-review.svelte.ts`、`GitTaskReviewPane.svelte` 与 `GitTaskDeliverySummary.svelte`：
    - 接通 `Commit` 视图双审阅范围（工作区 100% 干净时仍可阅读任务净差异与直接发起宽幅推送预览）、`Log` 视图 `当前任务` 范围与提交行菜单设基线，以及不自动删树的收尾出口。
- 依据：
  - `make persistence-check`、`make docs-check`、`make check` 通过；
  - `go test -race ./internal/statefile ./internal/workspace ./internal/codeview ./internal/api ./internal/wire ./internal/archtest ./internal/docguard ./cmd/...` 通过；
  - 前端单测、浏览器与多视口测试通过：
    - Vitest 全量单测、`GitWorkbench.test.ts` 与 `GitWorkbenchTaskBaseline.test.ts` 浏览器测试及 Storybook 全量多视口与 `320px` 大字号溢出守卫（87 项）通过；
    - 隔离实例 Playwright E2E（`git-workbench-task-baseline.spec.ts`、`git-workbench-commit-push.spec.ts`、`git-workbench-log.spec.ts`、`git-workbench-operations.spec.ts`、`git-workbench-route.spec.ts`，桌面与手机视口）全部通过。
- 限制：仅在 Linux 本地隔离实例与无头 Chromium 中核验；无头浏览器未验证 `hover` 与 `any-pointer: coarse` 媒体查询，未在 macOS 或 iOS/Android 物理真机上核对触屏手感；未重新核对本组以外的其余故事。

## 2026-11-13：Git 工作台 08（V1b）跨设备乐观锁提交草稿、审阅进度同步与指定会话返工意见交回（US-I06、US-J07、US-J19–US-J25）

- 范围：
  - `internal/statefile`、`internal/archtest/persistence_inventory.json`、`cmd/webmux/configview.go` 与 `docs/persistence.md` 登记 `git-workbench-state.json`（`0600` 原子替换 + 独立锁，单条草稿 ≤ 32 KiB、2 MiB `Fits` 上界、绝不记录提交草稿正文日志，删树/释放/注销仓库同步清理）；
  - `internal/workspace`、`internal/api`、`internal/contract`、`internal/wire` 新增 `GET/PUT .../git/commit-draft` 与 `GET/PUT .../git/reviews`（`scope=uncommitted|task`）及线上快照：
    - 提交草稿按 `baseRevision` 乐观并发校验，修订号不匹配返回 `409 draft_conflict` 与远端最新草稿；仅在真实创建提交（`matched` / `needs_review`）后清空，预览过期、钩子/锁失败、`uncertain` 与推送取消/失败均保留草稿；
    - 审阅标记绑定 Git blob OID 与 SHA-256 内容身份（不依赖 `mtime`），外部改动后自动失效并拦截过期 `PUT`（`applied: false`、`staleRejected: true`）；
  - `web/src` 新增 `git-collab-api.ts`、`git-commit-draft*.ts`、`git-review-sync.svelte.ts`、`git-rework*.ts`、`GitCommitDraftConflictCard.svelte`、`GitDiffSelectionBar.svelte`、`GitReworkDialog.svelte` 等组件：
    - 接通 `Commit` 双范围审阅进度徽章与“下一处未审阅”一键跳转、`409` 双栏草稿冲突裁决卡片、“让会话协助起草说明”入口；
    - 接通 `Commit`、`相对任务起点的变化` 与 `Log` Diff 选行/逐块摘录至返工意见草稿箱、历史提交/索引/起点行号防串行标注、实时工作区改动新鲜度校验、多会话强制显式单选目标会话，以及仅填入目标会话输入草稿而不向 PTY 自动发送 `\r`；
  - 新增 `docs/usage/git-workbench.md` 并与 `docs/README.md`、`docs/usage.md`、`docs/usage/code.md`、`docs/usage/worktrees.md`、`docs/protocol/http-workspace.md` 建立双向相对链接。
- 依据：
  - `make persistence-check`、`make docs-check`、`make check` 通过；
  - `go test -race ./...` 全量通过（含 `internal/workspace`、`internal/api` wire 快照与 `internal/docguard`）；
  - 前端 `git-collab.test.ts` 与全量 Vitest 单测（72 文件）、Vitest 浏览器套件（88 文件 / 740 项）、Storybook 全量多视口与 `320px` 2x 大字号溢出守卫（90 项）通过；
  - 隔离实例全量 Playwright E2E（490 项，含 `git-workbench-v1-collab.spec.ts` 桌面与 320px 手机视口）及性能套件（`test:performance`）通过。
- 限制：仅在 Linux 本地隔离实例与无头 Chromium 中核验；无头浏览器未验证 `hover` 与 `any-pointer: coarse` 媒体查询，未在 macOS 或 iOS/Android 物理真机上核对触屏手感；未重新核对本组以外的其余故事。

## 2026-11-13：Git 工作台 09（V2）历史分叉与多步操作预览、无状态协调恢复及三方冲突解决器（US-J22、US-J23、US-J26）

- 范围：
  - `internal/worktreefs` 新增原子工作树文本写入与受限路径删除（`WriteAtomicFile` / `DeleteFile`），强制拒绝父分量符号链接逃逸与 `.git` 目录写入；
  - `internal/workspace`、`internal/api`、`internal/contract`、`internal/wire` 新增 V2 分叉预览执行、协调恢复与三方冲突裁决端点及线上快照：
    - 两阶段分叉与遴选回滚端点：`merge`、`rebase`、`cherry-pick`、`revert` 预览与执行；
    - 协调恢复与三方冲突端点：`GET .../git/operation-state`、`POST .../git/operation-continue`、`POST .../git/operation-abort-preview`、`POST .../git/operation-abort`、`GET .../git/conflict`、`POST .../git/conflict/save`、`POST .../git/conflict/resolve`；
    - 拒绝暴露任何 `push --force` 开关；推送 `non_fast_forward` 被拒对话框内联提供 `Fetch`、`Fast-Forward`、合并远端分支与变基到远端分支；
    - 从 `.git` 真实 sequencer 元数据无状态读取进行中操作进度、不反转 Rebase 来源列语义（左侧为当前分支重放提交，右侧为变基目标基底），诚实披露 `Abort` 缺失回退锚点；
    - 严格分离“写入工作树草稿（不暂存）”与“标记已解决（清除 `1..3` 暂存阶段）”，覆盖文本三方分块与二进制/删改/子模块/符号链接/超大文件专卡裁决；
  - `web/src` 新增 V2 分叉预览、进行中操作横幅与三方冲突解决器前端实现：
    - 契约与控制器：`git-conflict-schemas.ts`、`git-conflict-api.ts`、`git-divergence-controller.svelte.ts`、`git-conflict-controller.svelte.ts`；
    - 视图组件：`GitDivergencePreviewDialog.svelte`、`GitInProgressBanner.svelte`、`GitAbortOperationDialog.svelte`、`GitConflictView.svelte`（含桌面三栏与窄屏单列折叠面板）。
- 依据：
  - `make persistence-check`、`make docs-check`、`make check` 通过；
  - `go test -race ./...` 全量通过（含 `internal/worktreefs`、`internal/workspace`、`internal/api` wire 快照与 `internal/docguard`）；
  - 前端 `git-conflict.test.ts` 与全量 Vitest 单测（73 文件 / 950 项）、`GitConflictView.test.ts` 与全量浏览器套件（89 文件 / 743 项）、Storybook 全量多视口与 `320px` 2x 大字号溢出守卫（94 项）通过；
  - 隔离实例全量 Playwright E2E（494 项，含 `git-workbench-conflicts.spec.ts` 桌面与 320px 手机视口）及性能套件（`test:performance`）通过。
- 限制：仅在 Linux 本地隔离实例与无头 Chromium 中核验；无头浏览器未验证 `hover` 与 `any-pointer: coarse` 媒体查询，未在 macOS 或 iOS/Android 物理真机上核对触屏手感；未重新核对本组以外的其余故事。

## 2026-11-13：Git 工作台 10（V2）独立 Stash 视图、受保护提交改写、引用远程维护与单文件历史/Blame（US-I01、US-J15、US-J20、US-J22、US-J23、US-J27）

- 范围：
  - `internal/workspace`、`internal/codeview`、`internal/api`、`internal/contract`、`internal/wire` 新增 V2 贮藏、高级提交、引用远程维护与单文件溯源端点及线上快照：
    - 独立 `Stash` 栈读取、两阶段创建（`-u`、`--keep-index`）、安全两步 `Pop`（先 `apply --quiet <oid>`，冲突时 100% 保留原贮藏并进入无 `Continue` 的 `unstash` 冲突态）、按精确 OID 校验的 `Drop` 及合并 `S^3` 未跟踪父节点的 Diff 快照；
    - 提交 `--amend`（含已有草稿保护、基于本地跟踪引用只读判定 `amendPushed` 与 `409 pushed_amend_ack_required` 强制确认）、`--author` 单次覆盖与 `-s / --signoff` 尾注预览；
    - 本地分支两阶段重命名（`git branch -m`）、上游绑定/解绑、轻量与附注标签创建/删除、远程仓库增删改（内嵌凭据脱敏 `***` 且写入拒绝字面量 `***` 占位符），以及上述写操作的 `git-operations.json` 持久账本与启动只读对账；
    - 单文件 `--follow` 跨重命名历史（`GET .../git/file-history`）、`--no-optional-locks` 只读逐行溯源（`GET .../git/blame`，支持沿父提交与前身路径链式回溯）及任意提交版本对 `HEAD`/`worktree` 的单文件比较快照；
  - `web/src` 新增 V2 贮藏、高级提交、引用远程维护与单文件溯源前端实现：
    - 契约与控制器：`git-stash-*`、`git-v2-*`、`git-commit-options.svelte.ts`、`git-refs-manage-controller.svelte.ts`、`git-remotes-controller.svelte.ts`、`git-file-inspector-controller.svelte.ts`；
    - 视图与入口：`GitStashPane`、`GitCommitAdvancedOptions`、`GitRemotesSection`、`GitFileInspectorDialog`、`GitBlameView` 等组件，并在会话代码工作区头部与工作台变更/历史文件行接入单文件历史与 Blame 入口；
  - 文档树重构：将 `git-workbench.md` 按文档预算拆分为树形索引及 `git-workbench/core.md`（US-J19–US-J23）与 `git-workbench/collab-and-advanced.md`（US-J24–US-J27）。
- 依据：
  - `make persistence-check`、`make docs-check`、`make check` 通过；
  - `go test -race ./...` 全量通过（含 `internal/workspace`、`internal/codeview`、`internal/api` wire 快照与 `internal/docguard`）；
  - 前端 `git-stash-and-v2.test.ts` 与全量 Vitest 单测（74 文件 / 994 项）、`GitWorkbenchStashBlame.test.ts` 与全量浏览器套件（90 文件 / 746 项）、Storybook 全量多视口与 `320px` 2x 大字号溢出守卫（99 项）通过；
  - 隔离实例全量 Playwright E2E（454 项，含 `git-workbench-stash-blame.spec.ts` 桌面与手机视口）及性能套件（18 项）通过。
- 限制：仅在 Linux 本地隔离实例与无头 Chromium 中核验；无头浏览器未验证 `hover` 与 `any-pointer: coarse` 媒体查询，未在 macOS 或 iOS/Android 物理真机上核对触屏手感；未重新核对本组以外的其余故事。

## 2026-11-13：Git 工作台 11（V3）逐行（Sub-hunk）暂存、受控丢弃更改与三阶段 Smart Checkout（US-J16、US-J19、US-J23、US-J28）

- 范围：
  - `internal/worktreefs` 新增 `WriteRegularFile` 支持保留可执行位覆盖或新建工作区普通文件；
  - `internal/workspace`、`internal/api`、`internal/contract`、`internal/wire` 新增 V3 逐行补丁合成、受控丢弃更改与三阶段 `Smart Checkout` 端点及线上快照：
    - `POST .../git/stage` 与 `POST .../git/unstage` 支持 `lineSelections: [{hunkId, lineIndices}]`，保留 `100644`/`100755` 权限、`LF`/`CRLF` 行尾与无末尾换行字节，且工作区原文件零改动；
    - 两阶段 `discard-preview` / `discard` 支持单文件、多选文件（含未跟踪）、单 Hunk 与选中行，写盘前复核磁盘 SHA-256（`409 stale`）并写入有界恢复提交 `refs/webmux/discard-recovery/<opId>`（绝不触碰 `.git/index`）；
    - 两阶段 `smart-checkout-preview` / `smart-checkout` 持久化三阶段状态机 `stashing -> switching -> unstashing`（切换失败自动回滚恢复，目标冲突时 100% 保留受控 `stashOid` 并接入 `GitConflictView` 三方裁决）及启动只读对账；
  - `web/src` 新增逐行暂存面板 `GitStagingLineSelector`、`CodeDiffReader` 选区直连暂存、多选与局部 `GitDiscardPreviewDialog`，以及活跃会话折叠保护下的 `SwitchBranchSmartCheckout`；
  - 协议文档重构：将 Git 工作台 HTTP 协议路由独立拆分为 `docs/protocol/http-git-workbench.md`。
- 依据：
  - `make persistence-check`、`make docs-check`、`make check` 通过；
  - `go test -race ./...` 全量通过（含 `internal/worktreefs`、`internal/workspace`、`internal/api` wire 快照与 `internal/docguard`）；
  - 前端 `git-v3-line-smart.test.ts` 与全量 Vitest 单测（75 文件 1,009 项）、`SwitchBranchDialog.test.ts` / `GitWorkbenchLineSmart.test.ts` 与全量浏览器套件（91 文件 749 项）、Storybook 全量套件（460 项，含多视口与 `320px` 2x 大字号溢出守卫）通过；
  - 隔离实例 Playwright E2E（103 项，含 `git-workbench-line-smart.spec.ts` 桌面与 320px 手机视口 6 项）及性能套件（18 项）通过。
- 限制：仅在 Linux 本地隔离实例与无头 Chromium 中核验；无头浏览器未验证 `hover` 与 `any-pointer: coarse` 媒体查询，未在 macOS 或 iOS/Android 物理真机上核对触屏手感；未重新核对本组以外的其余故事。

### 2026-11-19：切换 Git 选行模式时保留代码行锚点（US-J28）

- 隔离 Chromium 对全文差异测量切换前后，另外验证手机紧凑差异的行号映射和减少动态效果。
- `git-workbench-line-smart.spec.ts` 的真实索引逐行暂存用例在桌面及 iPhone SE Chromium 均通过。
- 本轮使用无头 Linux Chromium，未在物理触屏或真机浏览器核对。

## 2026-11-13：Git 工作台 12 原生设计系统重构（US-J20–US-J28）

- 对应 US-J20–US-J28：复用原生色板、通用紧凑列表、等高控件、顶部水平标签与操作账本浮层，统一文件状态与审阅图标；故事 ID 和数量不变。
- 已通过 Git 单元测试 52 项、浏览器组件测试 31 项、Storybook 矩阵与几何检查 34 项；Git E2E 最新全套通过 42 项，操作账本的两项修订断言另行重跑通过。
- 对照基线 cdb1c247，在隔离 Storybook 数据下采集桌面、平板、手机的浅色与深色历史/暂存对比截图，并录制标签切换及操作账本展开/关闭 GIF。
- 限制：无头 Chromium 不能替代真机触摸与物理 hover；没有进行真机验证。未重新核验其他故事组。

### 2026-11-19：固定 Git 逐行选择操作栏尺寸（US-J28）

- 隔离 Chromium 覆盖首选、Shift 扩选、全选和清空，核对工具栏与代码行盒尺寸。手机另测减少动态效果与模拟触摸单击。
- `GitWorkbenchLineSmart.test.ts` 的浏览器用例及真实隔离服务的逐行暂存 E2E 通过。
- 本轮使用无头 Linux Chromium，未在物理触屏或真机浏览器核对。

### Git 工作台收尾润色

- 沿用 US-J20–US-J28：历史 Diff 复用中文布局开关与文件状态图标，分组文案移除英文尾注，行内审阅采用仅切换图标和主题色的通用变体。
- 已通过 `make check`、52 项 Git 单元、31 项浏览器组件、34 项 Storybook 与完整 44 项 Git E2E；浏览器断言覆盖审阅态切换前后同尺寸、零边框与透明底色。
- 重录全部 12 张 After 与标签/账本 GIF；12 张 Before 哈希不变。公开资产与指定本地目录逐一哈希一致，目录仅保留 24 张 PNG 与 1 张 GIF。
- 限制：验证使用隔离数据和无头 Chromium；未进行真机触摸与物理 hover 验证。

## 2026-11-14：Git 工作台提交拓扑轨道连续性修复（US-J21）

- 范围：
  - `web/src/domains/workspace/git-graph-layout.ts` 按跨行 `openLanes` 状态确定性推导提交行上下垂直轨道：
    - 消除合并提交分叉车道上的悬空短竖线与关闭车道复用时的误入轨，并补齐根提交入轨；
    - 分页追加加载目标父提交后，将 `boundary` 虚线边升级为连续实线 `direct` 边；
  - `web/src/components/workspace/GitLogCommitRow.svelte` 将拓扑 `<svg>` 改为贴合 `ListButton` 全高的绝对定位层（`inset-y-0 h-full`）：
    - 竖直轨道采用百分比纵坐标，曲线段采用 `preserveAspectRatio="none"` 与 `vector-effect="non-scaling-stroke"`，提交圆点保持未缩放外层正圆；
    - 消除紧凑行内边距、触控最小行高与多行折行带来的行间断缝。
- 依据：
  - `make check`（含 `make docs-check`、`pnpm --dir web check:arch`、Oxlint、`go vet`、`golangci-lint`）通过；
  - 前端单测 `test/git-workbench.test.ts` 与 `test/git-log-controller.test.ts`（8 项）通过；
  - 浏览器套件 `test/browser/GitWorkbench.test.ts` 与 `test/browser/GitLogController.test.ts`（8 项，含 `1280×900` 与 `390×844` 相邻提交行 SVG 零间隙与正圆节点几何断言）及 Storybook `test/storybook/git-workbench.spec.ts`（27 项）通过；
  - 隔离实例 Playwright E2E `test/e2e/git-workbench-log.spec.ts`（桌面与 `320×568` 手机视口共 2 项）通过，并在隔离演示仓库下采集桌面（`1280×900`）、平板（`768×800`）、手机（`390×844`）浅色与深色共 12 张修复前后对比截图。
- 限制：仅在 Linux 本地隔离实例与无头 Chromium 中核验；无头浏览器未验证 `hover` 与 `any-pointer: coarse` 媒体查询，未在 macOS 或 iOS/Android 物理真机上核验；未重新核对 `US-J21` 以外的其余故事。

## 2026-11-14：Git 工作台顶栏按钮与目标切换器统一重构（US-J20、US-J23）

- 范围：
  - `web/src/components/workspace/GitWorkbenchTargetSwitchers.svelte` 将自定义 `Popover` + `ListButton` 仓库与工作树切换器统一替换为 shadcn `DropdownMenu`（`RadioGroup` / `RadioItem`）及标准 `Button variant="outline" size="sm"`；
  - `web/src/components/workspace/GitOperationsPopover.svelte` 将操作账本触发器由无边框 `ghost` 嵌套实心 `Badge` 统一为 `Button variant="outline" size="sm" | "icon-sm"` 内联截断文本，消除空态空标签与跨排高度差异；
  - `web/src/components/workspace/GitWorkbenchTopBar.svelte` 移除左侧目标组在 `flex-wrap` 容器内的 `flex-1`（`flex-basis: 0%`）挤压缺陷，并将右侧返工意见、返回会话、远程同步与更多菜单统一收敛为 `size="sm"` / `size="icon-sm"`（窄屏触控下由 `.git-workbench .wm-control` 统一提升至 `36px`）。
- 依据：
  - `make check`（含 `make web`、`pnpm --dir web check:arch`、`make docs-check`、`make persistence-check`、Oxlint、`go vet`、`golangci-lint`）通过；
  - 浏览器与 Storybook 测试：
    - `GitWorkbenchRoute.test.ts`、`GitWorkbenchOperations.test.ts`、`GitWorkbench.test.ts`、`GitWorkbenchLineSmart.test.ts` 通过（覆盖 `1280×900`、`768×800`、`390×844`、`320×568` 等高与防挤压断言）；
    - Storybook `test/storybook/git-workbench.spec.ts`（27 项）通过；
  - 端到端与视觉证据：
    - 隔离实例 Playwright E2E `git-workbench-route.spec.ts`、`git-workbench-operations.spec.ts`、`git-workbench-commit-push.spec.ts`（14 项）通过；
    - 隔离演示仓库下采集桌面（`1280×900`）、平板（`768×800`）、手机（`390×844`）浅色与深色共 12 张前后对比截图及顶栏菜单交互 GIF。
- 限制：仅在 Linux 本地隔离实例与无头 Chromium 中核验；无头浏览器未验证物理 `hover` 与 `any-pointer: coarse` 媒体查询，未在 macOS 或 iOS/Android 物理真机上核验；未重新核对 `US-J20`、`US-J23` 以外的其余故事。

## 2026-11-16：Git 工作台差异工具栏收敛为双栏显示并移除快照全文与放大模式（US-H14、US-J20、US-J21、US-J22、US-J24）

- 范围：
  - 移除 `GitDiffStyleSwitch.svelte` 以及各面板的 `diffMode` 切换入口与 `GitDeviceLayoutSchema` 中的持久化字段：
    - 桌面与平板视口固定以双栏（`Split`）展示差异，`CodeDiffReader` 在 `< 480px` 窄容器及 `< 640px` 手机步进视图下自动回退为单列 `Unified`；
    - 保留旧版 `localStorage` 布局记录兼容读取（自动忽略遗留 `diffMode` 键而不丢弃分栏比例）；
  - 移除内联快照全文预览并保留二进制/超预算快照原始下载：
    - 清理 `GitLogDiffPanel.svelte`、`GitLogDiffFallbackView.svelte` 及 `git-snapshot-diff-reader.svelte.ts` 的 `openSnapshotSide` / `closeSnapshotSide` 状态；
    - `gitSnapshotFile` 作为 US-J19 仓库与工作树 REST 完整类型封装保留在 `git-api.ts`；
  - 移除两处“放大查看文件差异”按钮与 `maximized` 全工作区放大层：
    - 调整 `GitLogFilesPane.svelte`、`GitLogPane.svelte` 与 `GitLogDiffPanel.svelte`，历史差异固定在底部可折叠/拖拽高度停靠栏（或手机单列步进页）阅读；
    - 同步更新 `US-H14` 与 `coverage.md` 的分层返回说明；
  - 修复 `GitStagingDiffPanel.svelte` 外层 `ScrollArea` 包裹 `contain-strict` `CodeDiffReader` 导致暂存差异区高度坍缩的问题，改为定高弹性列容器。
- 依据：
  - 静态与治理门禁：`make format-check`、`make docs-check`、`make persistence-check`、`openspec validate --specs`、`pnpm --dir web check:test`、`pnpm --dir web lint`、`pnpm --dir web check:arch`、`go vet` 与 `golangci-lint` 通过；
  - 前端单测与浏览器测试：
    - `i18n.test.ts`、`git-workbench-memory.test.ts`、`git-workbench.test.ts`、`git-log-controller.test.ts`、`git-api.test.ts`、`architecture.test.ts`、`dependency-policy.test.ts`（67 项）通过；
    - `CodeDiffReader.test.ts`、`GitWorkbench.test.ts`、`GitLogController.test.ts`、`GitWorkbenchCommitPush.test.ts`、`GitWorkbenchTaskBaseline.test.ts`、`GitWorkbenchStashBlame.test.ts`、`GitWorkbenchLineSmart.test.ts`（22 项）通过；
    - Storybook 多视口套件 `test/storybook/git-workbench.spec.ts`（27 项）通过；
  - 端到端与多视口视觉证据：
    - 隔离实例 Playwright E2E `git-workbench-log.spec.ts`、`git-workbench-commit-push.spec.ts`、`git-workbench-task-baseline.spec.ts`、`git-workbench-stash-blame.spec.ts`（24 项）通过；
    - 2026-11-19（T064）：隔离 Chromium 验证比较加载、失败及路径过滤的文件结果区保持几何稳定，反向重命名仍选中同一文件。浏览器用例 `GitLogController.test.ts` 验证快速反向与过期响应处理。
    - 在隔离实例采集 `Log` 与 `Commit` 在桌面（`1280×900`）、平板（`768×800`）、手机（`390×844`）浅色与深色共 24 张修改前后对比截图，并通过 `1280×900` 至 `320×568` 及 `667×375` 共 7 组视口的 `scrollWidth <= innerWidth` 防溢出校验。
- 限制：仅在 Linux 本地隔离实例与无头 Chromium 中核验；无头浏览器未验证物理 `hover` 与 `any-pointer: coarse` 媒体查询，未在 macOS 或 iOS/Android 物理真机上核验；未重新核对本组以外的其余故事。

## 2026-11-18：工作台工作区刷新与 Git worktree 实时同步（US-C08、US-C16、US-J03）

- 范围（故事编号与总数 264 保持原状）：
  - 服务端按需监听（`internal/workspace/watch.go`、`internal/featurehub/workspace.go`、`GET /ws/repositories`）：
    - 仅在存在活跃订阅者时以 `2s` 周期比对 `<CommonDir>/HEAD` 与 `<CommonDir>/worktrees/*` 元数据哈希；
    - `250ms` 防抖稳定后重关联活动会话并推送 `{"type":"changed"}`，仓库与工作树写操作完成后同步触发通知；
  - 服务端已观测工作树基线（`internal/workspace/known_worktrees.go`、`internal/settings/service.go`）：
    - 在 `repositories.json` 按仓库维护有界基线（单仓上限 1000 条），首次成功读取建立基线以保持历史空闲工作树隐藏；
    - 基线建立后外部新增的可用 `linked` worktree 自动经 `settings.PinHomeWorktrees` 加入 `homeWorktrees`；
    - 受控新建（含 `--no-pin`）与手动移出主页的工作树保持原选择，设置暂不可写时留待下次重读重试；
  - 前端与工作台侧栏（`AppSidebar.svelte`、`AppShell.svelte`、`shell-resources.ts`、`repositories.svelte.ts`）：
    - 展开侧栏在「新建会话」右侧并排提供「刷新工作区」图标按钮（`variant="outline" size="icon-touch"`，刷新中标记 `aria-busy`）；
    - 订阅 `/ws/repositories`（`80ms` 合并防抖重读），并将前台恢复及工作台相关弹窗打开时的重读统一接到外壳生命周期。
- 依据：
  - 静态与治理门禁：`make check`、`pnpm --dir web check:arch`、`make docs-check`、`make persistence-check`、`go vet` 与 `make go-lint`（`golangci-lint` v2.15.0，0 issues）通过；
  - Go `-race` 回归：
    - `go test -race ./cmd/webmux ./internal/api ./internal/featureapi ./internal/featurehub ./internal/settings ./internal/workspace ./internal/featurewire ./internal/archtest ./internal/docguard` 通过；
    - 覆盖 `watch_test.go` 与 `TestRepositoriesWebSocketEnforcesAuthAndStreamsChanges`；
  - 前端回归：
    - `WorkbenchTree.test.ts`、`AppShell.test.ts` 与 `TvNavigation.test.ts` 浏览器组件测试通过；
    - Playwright E2E `worktrees.spec.ts`、`app-shell.spec.ts`、`workspace-discovery.spec.ts`、`repository-scope.spec.ts`、`i18n.spec.ts`、`git-workbench-route.spec.ts` 与 `tv-browser.spec.ts`（1280 与 1920）通过；
  - 视觉核对：隔离演示服务在 `1280×900`、`1280×720`、`768×800`、`390×844`、`375×667`、`320×568`、`667×375` 明暗主题与中英文下 `scrollWidth <= innerWidth`，同排按钮高度一致。
- 限制：仅在 Linux 无头 Chromium 下验证，`hover` 与 `any-pointer: coarse` 均为 `false`，物理鼠标悬停与触屏硬件行为未在手机、平板、macOS 或 Windows 真机验证。
