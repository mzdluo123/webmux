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
- 依据：
  - Go 单元与竞态测试：`go test -race ./internal/workspace ./internal/codeview ./internal/wire ./internal/api ./internal/clicmd ./internal/archtest ./internal/docguard` 通过（含 `TestBranchesTagsAndHistoryTagFiltering` 及 wire 快照校验）；
  - 前端架构与单测门禁：`make check`、`test/git-log-controller.test.ts`、`test/route.test.ts` 与 `test/git-workbench.test.ts` 通过；
  - 浏览器与 Storybook 测试：`test/browser/GitLogController.test.ts`、`test/browser/GitWorkbench.test.ts`、`test/browser/GitWorkbenchRoute.test.ts` 与 `pnpm --dir web test:storybook`（含多视口溢出巡检）通过；
  - Playwright 端到端与视觉验证：
    - `test/e2e/git-workbench-log.spec.ts` 与 `test/e2e/git-workbench-route.spec.ts`（桌面与窄屏项目）验证分支/标签筛选不改写磁盘 `HEAD`、显式“检出分支…”预选弹窗、路径过滤 `indirect` 虚线边、游标分页追加与合并父节点 `#1/#2` 切换；
    - 同一 E2E 进一步验证任意两提交比较与方向交换、放大 Diff 返回保留列表滚动位置、手机窄屏三步导航零横向溢出，以及全程 `.git/index` 内容摘要与暂存区零改动；
    - 在真实运行实例上完成桌面（1280×900）、平板（768×800）、手机（390×844）浅色与深色前后对比截图及常速交互 GIF 录制与逐张视觉核查。
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
