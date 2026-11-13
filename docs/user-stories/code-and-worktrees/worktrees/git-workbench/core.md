# Git 工作台基础闭环与账本（US-J19–US-J23）

本页收录独立 Git 工作台的基础接口、独立 `/git` 路由与状态隔离、拓扑历史图、真实 `index` 暂存与提交推送确认框，以及持久 Git 操作账本故事（US-J19–US-J23）。
进阶故事见[任务基线、协同、冲突解决与日常高级操作](collab-and-advanced.md)，分索引见[Git 工作台与仓库级接口](../git-workbench.md)。

## J. Git 工作台基础闭环与账本

- **US-J19 仓库与工作树 Git 只读、暂存与提交接口**：作为在不启动终端或混合会话时审阅与整理仓库提交的使用者及前端工作台调用方，我希望通过统一的仓库与工作树 Git REST 接口读取历史拓扑、比较任意提交、按文件或 Hunk 操作真实 `index` 并预览与执行提交，以便在零活动会话下安全完成完整 Git 审阅与提交流程。
  验收要点：
  - 无需 `sessionId` 或活动会话即可访问 `/api/repositories/{id}/git/*` 与 `/api/repositories/{id}/worktrees/{worktreeId}/git/*`，全部路由沿用 Bearer 认证（401）、Origin/Host 校验（403）与 `Cache-Control: no-store`；
  - 仓库级与工作树级读取能力按是否绑定工作树分层开放：
    - 省略 `worktreeId` 时 `GET .../git/context` 绑定仓库级只读上下文（`repository` 或 `bare`），支持 `GET .../git/file?source=commit:{oid}` 与 `GET .../git/compare`（含根提交空树与双向交换比较），对工作区 `tree`、`files`、`changes`、`diff` 返回 409 `reason: "readonly"`；
    - 传入可用 `worktreeId` 时开放工作树目录、文件名搜索、`worktree|index|head|commit:{oid}` 四种来源快照及 `side=before|after` 的 `file`/`raw`（含 `HEAD`）；
  - `contextId` 与 `snapshotId` 严格绑定 `(repositoryId, worktreeId)`：跨仓库混用返回 404，同仓库跨工作树混用返回 409 `reason: "context_mismatch"`，过期返回 409 `reason: "stale"`；非法 OID、非真实父提交 `parentOid`、修订注入或越界路径返回 400；
  - 历史图与提交详情独立分页查询：
    - `GET .../git/history` 返回 `status`（`ready` / `empty` / `unborn` / `no_worktree` / `no_task_baseline` / `no_upstream` / `upstream_gone` / `detached_no_branch`）、拓扑分页 `rows`（真实 `parents`、`refs`、`lane`、`activeLanes` 与 `direct|indirect|boundary` 边）、`nextCursor` 与 `refsChanged`；
    - `GET .../git/commits/{oid}` 返回完整提交消息、作者与提交者元组、`comparedParentOid`（合并提交支持切换任一真实父 OID）与变更文件列表；
  - 暂存操作直接读写目标工作树真实 Git `index`：
    - `GET .../git/staging`、`GET .../git/staging/diff`、`POST .../git/stage` 与 `POST .../git/unstage` 支持整文件与单 Hunk 暂存/取消暂存（含 `unborn HEAD`）；
    - 路径身份或 Hunk 变化返回 409 `reason: "stale"`，检测到外部 `.git/index.lock` 返回 409 `reason: "busy"` 且绝不删除锁文件；
  - 提交采用预览确认与执行后复核两阶段保障：
    - `POST .../git/commit-preview` 展示完整 `HEAD -> index` 待提交集合（含终端 CLI 预暂存文件）、身份、可执行 Hook、阻塞项与限时确认码；
    - `POST .../git/commit` 在独立 2 分钟受控窗口内执行提交（客户端断开不强杀已启动的 `git commit`），经 reflog 与 `{oid, parents, tree}` 复核返回 `matched`、`needs_review`（如 `pre-commit` Hook 改写索引）或 `uncertain`；
  - 访问与错误日志不输出文件内容、Diff 正文、提交消息全文或凭据；前端 `web/src/contracts/git-api-schemas.ts` 与 `web/src/platform/git-api.ts` 复用统一 `api()`/`apiBlob()`/`APIError` 与 wire 快照校验，不修改既有 `/api/sessions/{id}/code/*` 行为。
- **US-J20 独立 `/git` 工作台路由、多入口联动与跨视图状态隔离**：作为同时管理多份工作树与会话的使用者，我希望在零活跃会话或任一会话旁通过独立 URL 打开一等公民 Git 工作台，在历史、提交与分支视图间自由往返而不丢阅读或草稿状态，并能无损返回原会话。
  验收要点：
  - 路由支持 `/git`、`/git/{repositoryId}`（只读仓库模式）与 `/git/{repositoryId}/worktrees/{worktreeId}`（工作树读写模式），并解析深链查询参数 `view=log|commit|branches|stash`、`scope=all|branch|task|to-push|incoming`、`branch`、`commit`、`q`、`author`、`since`、`until`、`path` 与 `returnSession`；
  - 显式 URL 查询参数优先级始终高于本地记忆，浏览器前进/后退仅改变查询串时响应式同步当前视图与筛选，不销毁单实例 `AppShell` 或重复建立 `/ws/sessions` 连接；在 `/git/...` 页面切换侧栏“工作台 / 聊天”模式保持当前 `/git/...` 路由；
  - 零活跃会话下可通过侧栏仓库“⋯”菜单、首页仓库菜单、仓库面板头部“Git 工作台”按钮或直接访问 URL 打开工作台，全程不创建任何伪终端会话；创建并删除最后一个会话不会卸载或中断当前 `/git` 页面；
  - 三类入口精确落位并支持无损返回会话：
    - 仓库级入口打开只读仓库模式 `/git/{repositoryId}`；
    - 工作树“⋯”菜单提供“在 Git 工作台打开”（`/git/{repositoryId}/worktrees/{worktreeId}`），且当工作树可用、未锁定、非 `bare` 并存在待处理改动时额外提供“提交改动…”直达 `?view=commit`；
    - 会话代码工作区头部“在 Git 中查看”携带 `returnSession={sessionId}` 跳转并在顶栏提供“返回会话”；若原会话已退出、归档或删除则诚实提示状态，工作台继续独立可用；
  - 顶栏语义分离与工作树写门禁：
    - 顶栏左侧固定展示操作目标（仓库切换器、工作树切换器与当前检出分支/`HEAD` 徽标），与历史视图的 `branchFilter`（“历史筛选：…”徽标）严格分离；
    - 只读仓库模式或 `bare` 工作树进入“提交 (Commit)”视图时显式要求选择可写工作树，绝不静默落到任意工作树；
    - 工作树缺失、待清理或已锁定时禁用写操作并提供管理/修复或只读切换入口；
  - 跨视图、跨工作树与跨设备尺寸隔离记忆：
    - 按 `(hostId, repositoryId, worktreeId)` 隔离保留选中提交、父提交对比、筛选条件、列表与 Diff 滚动位置、已看标记与本地提交草稿，`Log -> Commit -> Branches -> Log` 往返不丢状态，切换工作树或仓库立即清空确认码与跨目录草稿；
    - 分栏比例、引用栏显隐与统一/并排 Diff 偏好按设备尺寸档位（`desktop`、`compact`、`tablet`、`phone`）独立持久化，手机端操作不覆盖桌面端布局。
- **US-J21 Git 工作台历史视图：引用筛选、`%P` 拓扑分页、提交详情、两版本比较与 Pierre Diff 阅读**：作为检视仓库演进或对照分支差异的使用者，我希望在同一历史工作面筛选多分支拓扑提交图、切换合并父节点或比较任意两提交，并复用只读差异阅读器检查变更，绝不静默切换检出状态或改动 `.git/index`。
  验收要点：
  - 左侧引用筛选树与检出解耦：
    - 桌面端左侧栏默认 `240px`（支持 `180–360px` 拖拽与折叠，`< 1280px` 收纳为抽屉），分组展示收藏分支（按主机与仓库持久化，默认收录 `defaultBranch`）、当前分支、本地分支、远端跟踪分支与标签（`refs/tags/`，附注标签解包至提交 OID），标注 `HEAD`、`已占用` 及上游领先/落后或 `上游已删`；
    - 单击任一分支或标签仅筛选历史提交列表并更新顶栏浏览筛选徽标，顶栏活动工作树、磁盘检出分支与 `HEAD` 保持不变；切换分支必须经行尾菜单“检出分支…”进入预选目标分支的 `SwitchBranch` 预览确认流程，已由其他工作树占用的分支引导打开所在工作树；
  - 快捷范围与服务端组合筛选：
    - 支持 `所有历史`（默认）、`当前分支`、`当前任务`（`scope=task`，按固定任务基线精确解析 `baseOid..HEAD` 或 `unborn` 全历史）、`待推送`、`远端新增` 五种快捷范围；
    - 支持组合关键字（`q`）、作者（`author`）、起止日期（`since`/`until`）与字面路径（`path`）向服务端请求，且可写工作树下提交行菜单提供一键“设为当前工作树任务基线”；
    - 遇到 `no_worktree`、`no_task_baseline`、`detached_no_branch`、`no_upstream`、`upstream_gone`、`unborn` 或 `empty` 时直接展示明确中文原因提示与操作入口，绝不误导显示为“0 个提交”；
    - 筛选或刷新后若原选中提交仍在首屏则保留选中，否则说明原因并在存在 `HEAD` 时提供一键“定位当前 HEAD”；
  - `%P` 拓扑车道图与游标流式分页：
    - 基于真实父提交 `%P` 渲染节点、分支/标签/`HEAD` 徽标、`direct` 实线边、`indirect` 虚线边（当路径或关键字过滤省略中间节点时额外显示“筛选省略中间节点”）及页尾 `boundary` 延续短边；
    - 滚动触底或点击“加载更多历史”通过不透明 `cursor` 追加下一页且不重排已渲染行；当分页或刷新返回 `refsChanged: true` 时给出非阻塞“分支引用已更新，可刷新”提示；
  - 右上变更文件、右下提交详情与合并父节点切换：
    - 右上“变更文件”展示状态徽标（`A`/`M`/`D`/`R`/`C`/`T`）、重命名路径（`oldPath → newPath`）、二进制标记与按路径二次过滤，支持 `ArrowUp`/`ArrowDown`/`Home`/`End` 键盘导航、单击在底部预览、`Enter` 或双击放大至主工作区；
    - 右下“提交详情”展示完整消息（标题与保留换行的正文）、作者与提交者（`distinctCommitter` 为真时分开列出）、完整 OID 复制、可点击跳转的父提交 OID 与全部引用徽标；合并提交（`parents.length >= 2`）提供显式父节点切换器并联动刷新变更文件与差异；
  - Pierre `CodeDiffReader` 复用、任意两版本比较与异步竞态隔离：
    - 底部可折叠与高度可拖拽（`140–640px`）差异栏及主工作区放大态统一复用 Pierre `CodeDiffReader`，保留语法高亮 worker、文件内查找、行号跳转、自动换行与字号缩放、Unified/Split 切换、变更前后完整快照预览及原始文件下载；
    - 差异面板对加载中、二进制、超预算/纯元数据变更与错误重试态给出中文内嵌反馈；放大态按 `Escape`（无打开面板时）或返回按钮恢复原提交、文件选中与列表滚动位置；
    - 提交行与引用行菜单支持“设为比较起点 / 与起点比较 / 与当前选中提交比较”，只读拉取 `baseOid → targetOid` 变更文件列表与逐文件差异，支持一键“交换方向”与“退出比较”，全程不执行 `git checkout`、不触碰 `.git/index`、不自动触发 `git fetch`；
    - 提交详情、父节点切换、单文件差异与两版本比较均绑定 `LatestRequest` 与 `AbortController`，快速连续切换时丢弃过期响应，绝不覆盖当前选中状态；
    - 窄屏手机端（`< 640px`）采用“提交列表（含引用筛选抽屉）→ 提交详情与变更文件 → Unified 差异阅读”单列步进导航且无横向溢出。
- **US-J22 Git 工作台提交暂存闭环、三处共享分支动作与宽幅推送确认对话框**：作为在同一工作树混用终端/Agent 与浏览器整理代码的使用者，我希望直接对真实 Git `index` 执行整文件或逐块暂存、在完整预览与三态核对后提交，并在顶栏、历史与分支三处复用同一套安全分支动作及双栏推送确认框核对待推提交与文件差异。
  验收要点：
  - `Changes` 四组树与阅读、审阅、暂存三态严格分离：
    - 左侧按 `Conflicts`、`Staged`、`Unstaged`、`Untracked` 四组展示精确计数并支持折叠（含空分组）；部分块已暂存的同一路径同时出现在 `Unstaged`（`index → working tree`）与 `Staged`（`HEAD → index`）；
    - 当前阅读高亮（`aria-selected`）、绑定 `identityKey` 内容版本的本地“已审阅”标记与真实 Git `index` 驱动的“已暂存”三态严格分离；无勾选框暂存列表，文件或暂存身份在外部变化时自动失效旧“已审阅”标记；
    - 支持分组头批量 `Stage`/`Unstage`、文件行整文件 `Stage`/`Unstage`，以及不依赖鼠标悬停（桌面键盘与触屏均常驻可达）、绑定 `StageFileIdentity` 与 `hunkIds` 的常驻逐块操作条；
  - Pierre `CodeDiffReader` 暂存阅读、降级兜底、冲突引导与过期快照门禁：
    - 右侧差异区复用 Pierre `CodeDiffReader`，顶栏标明文件路径、比较端点（`index → working tree` 或 `HEAD → index`，缺失侧显式注明“文件不存在”）、上/下一块跳转、Unified/Split 切换及整文件暂存/取消暂存；
    - 二进制、重命名/复制、符号链接/子模块、`.gitattributes` 含 `filter`/`working-tree-encoding`、纯删除、纯权限变更及超预算（`> 2 MiB` 或 `> 20,000` 行）文件展示明确 `degradeReason`，仅开放整文件 `Stage`/`Unstage`；
    - 冲突条目（`Conflicts`）不伪装为可提交文件，展示进行中 Git 操作并提供一键打开内置三方冲突解决器（US-J26）以及跳转或新建该工作树终端会话入口；
    - 外部终端或 Agent 修改当前文件或 `.git/index` 导致 `409 stale` 或前台检测到 `identityKey` 变化时，保留当前阅读视图、置灰旧快照暂存按钮并提示点击“刷新快照”；
  - 独立标题/正文编写、全量 `HEAD -> index` 提交预览与三态结果复核：
    - 提交编写区拆分标题（`Input`）与正文（`Textarea`）：
      - `Ctrl/Cmd+Enter` 仅打开提交预览确认框（绝不直接执行 `git commit`），实时显示生效作者身份、签名与可执行 Hook 摘要及前置阻塞提示；
      - 展开高级提交选项支持 `--amend`（含已有草稿防静默覆盖三选一提示、已推送 `HEAD` 风险警告与禁用一键“提交并推送…”）、单次作者覆盖（`Name <email>`）及 `Signed-off-by`（`-s`）尾注预览（US-J27）；
      - 非 `amend` 且 `index` 为空时禁用提交并说明原因，若当前分支存在待推提交则直接提供“推送待推提交…”入口；
    - 提交预览确认框完整列出全部 `HEAD -> index` 待提交文件（含终端 CLI 预暂存文件并显式标记）并支持逐文件展开阅读真实 `HEAD -> index` Pierre Diff，同时展示未纳入本次提交的未暂存/未跟踪数量、身份/签名/Hook 与阻塞项；分离 `HEAD` 时提供从当前 `HEAD` 新建并切换分支入口；
    - `POST .../git/commit` 遇到 `409 stale` 时保持预览弹窗打开、保留提交草稿并提供一键“重新生成提交预览”；遇到 `409 busy`（`.git/index.lock`）或 `409 commit_failed`（Hook/签名失败）时原位报错且绝不删除锁文件或丢失草稿；
    - 提交执行后按三态结果分别处理：
      1. `matched`：仅当用户发起“提交并推送…”时自动打开推送确认框；
      2. `needs_review`（如 `pre-commit` Hook 格式化重暂存改写树）：立即中止自动串联推送，突出展示实际 `commitOid`、`reviewReasons` 与前往 `Log` 查看入口；
      3. `uncertain`：诚实提示不确定状态、保留草稿并提供刷新历史与暂存区入口，禁止盲目重试；
  - 顶栏分支弹层、`Log` 引用树与 `Branches` 视图三处共享分支模型与动作：
    - 三处统一将当前检出分支与收藏分支置顶，一致显示本地/远程类型、准确上游领先/落后计数（无上游、上游已删或未统计时绝不伪造 `0`）、占用工作树路径与活跃会话数，且每行均支持行尾 `⋯` 按钮、鼠标右键菜单与键盘 `Shift+F10` / `ContextMenu` 键；
    - 复用真实仓库级分支与工作树流程：在历史中筛选、`检出分支…`（复用 `SwitchBranchDialog` 展示脏文件与活跃会话风险警告）、与当前基准比较、从此分支新建分支、在新工作树打开、删除本地分支、获取远程（Fetch）、快进更新（Update）及推送/发布分支；
    - 已由另一工作树检出占用的本地分支绝不提供检出按钮，主操作固定为“打开所在工作树”与“从该提交新建分支工作树…”；
  - ~900px 宽幅双栏推送确认框与两步独立结果报告：
    - 桌面端展示约 `900px` 双栏确认框，手机窄屏（`< 640px`）展示全屏确认页：
      - 左栏展示仓库、`sourceBranch → remote/targetBranch`（首次发布支持选择远程与编辑目标分支名）、脱敏 `pushUrl` 及全部待推提交列表（上限 500 条）；
      - 右栏展示所选待推提交的变更文件列表、真实 Pierre `CodeDiffReader` 文件差异及完整提交说明与作者信息；
    - “提交并推送…”两步独立恢复：
      - 第一步本地提交成功后若用户取消推送或推送失败（如远端非快进超前、鉴权失败），明确报告“本地已提交（`<shortOid>`），推送未完成/已取消”及原因并完整保留本地提交；
      - 直接提供“获取远程 (Fetch)”、“快进更新… (Update)”、“合并远端分支… (Merge)”与“变基到远端分支… (Rebase)”恢复入口（US-J26），全程绝不提供 `--force`、`--no-verify` 或 `git add -A`。
- **US-J23 持久 Git 操作账本、重连幂等对账与底部状态栏闭环**：作为在网络波动、刷新页面或升级重启服务期间执行 Git 写操作的使用者，我希望全部受控 Git 写操作（提交、推送、获取远程、快进、切分支及合并/变基/遴选/还原/继续/中止）均记录持久 `operationId` 并幂等去重、启动对账，且在底部状态栏可查。
  验收要点：
  - 预检凭证与持久操作实例职责解耦：
    - `confirmationId` 仅负责两阶段预览快照绑定（防过期/防错库）；
    - 基础与序列操作（`commit`、`push`、`fetch`、`fast_forward`、`switch`、`merge`、`rebase`、`cherry_pick`、`revert`、`continue`、`abort`）统一生成持久 `operationId` 并记录 `clientRequestId`；
    - 贮藏与引用远程操作（`stash_create`、`stash_apply`、`stash_pop`、`stash_drop`、`branch_rename`、`branch_upstream`、`tag_create`、`tag_delete`、`remote_add`、`remote_set_url`、`remote_remove`）同样纳入持久账本；
    - 同一 `(repositoryId, clientRequestId)` 且参数摘要一致时，并发在途请求合并等待同一执行实例，已完成操作在断线重连或服务重启后直接重放持久结果，绝不二次执行底层 Git 写命令；
    - 复用相同 `clientRequestId` 但参数变化时返回 HTTP `409`（`reason: "idempotency_conflict"`）；
  - 有界 `statefile` 持久账本与隐私边界：
    - 账本以 `git-operations.json`（`0600` 原子替换 + 生命周期锁）持久保存并纳入 `statefile.Catalog`、`persistence_inventory.json` 与 `webmux config show` 诊断；
    - 单仓库最多保留 100 条、全局最多 1000 条、超过 7 天淘汰已终结记录且整体满足 2 MiB `Fits` 上界；移除仓库登记（`DELETE /api/repositories/{id}`）时同步清理该仓库全部操作记录；
    - 输出摘要经凭据脱敏并截断至 `<= 4 KiB`，绝不持久化环境变量、原始凭据、Diff 正文或文件内容；开放 `GET /api/repositories/{id}/git/operations?worktreeId&limit` 与 `GET /api/repositories/{id}/git/operations/{operationId}`（跨仓库访问返回 404）；
  - 服务启动真实 Git 引用与 reflog 只读对账（零自动重跑、零 `git reset`）：
    - 服务启动加载遗留 `pending`/`running` 条目时，绝不重新执行原 Git 写命令或执行任何 `git reset`；
    - `commit` 结合 `GIT_REFLOG_ACTION=webmux-commit:<operationId>` 与 `{oid, parents, tree}` 核对收敛为 `succeeded`、`needs_review`、`failed` 或 `uncertain`；
    - `push`、`fast_forward` 与 `switch` 依据本地跟踪引用、上游配置及工作树 `HEAD`/洁净度只读收敛；`merge`、`rebase`、`cherry_pick`、`revert`、`continue`、`abort` 结合 Git 序列目录状态、`ORIG_HEAD` 与 `refs/webmux/recovery/<worktreeId>/<kind>` 恢复锚点只读收敛；`fetch` 中断统一标记为 `uncertain`；
  - 底部 `24–28px` 状态栏、断线自动查询恢复与 `uncertain` 安全门禁：
    - 工作台底部常驻 `24–28px` 状态栏（桌面、平板与手机视口均不膨胀或横向溢出），展示工作树路径、最近 `FETCH` 时间、当前阶段（`等待校验` / `执行中` / `重连核对中`）或最近操作徽标与摘要；
    - 点击状态栏触发器展开碰撞内边距受控的 `Popover`，查看最近操作列表、`op:` 实例 ID、`reviewReasons` 与脱敏输出；
    - 请求超时或网络中断时前端优先通过 `GET .../git/operations` 按 `clientRequestId`/`operationId` 轮询恢复终态，刷新页面或跨设备打开时自动恢复未确认的 `needs_review` / `uncertain` 横幅；
    - 处于未核实的 `uncertain` 状态时禁用盲目一键重新提交，引导先点击“刷新历史与暂存区核实”。

依据：

- Git 只读与暂存提交接口：[只读路由](../../../../../internal/api/git_read.go)、[暂存提交路由](../../../../../internal/api/git_write.go)、[线上契约](../../../../../internal/contract/git_read.go)、[转换层](../../../../../internal/wire/git.go)；
  [代码上下文与比较](../../../../../internal/codeview/git_lookup.go)、[历史与暂存提交服务](../../../../../internal/workspace/staging.go)、[前端契约](../../../../../web/src/contracts/git-api-schemas.ts)、[前端请求封装](../../../../../web/src/platform/git-api.ts)；
  [只读接口测试](../../../../../internal/api/git_read_test.go)、[暂存提交接口测试](../../../../../internal/api/git_write_test.go)、[快照录制](../../../../../internal/api/wire_git_snapshot_test.go)、[前端契约单测](../../../../../web/test/git-api.test.ts)。
- 独立 `/git` 路由与壳层：
  - 页面与状态：[Git 工作台页](../../../../../web/src/pages/GitWorkbench.svelte)、[控制器](../../../../../web/src/hooks/workspace/git-workbench-controller.svelte.ts)；
  - 路由与记忆：[路由契约](../../../../../web/src/hooks/shared/route.ts)、[记忆模块](../../../../../web/src/domains/workspace/git-workbench-memory.ts)；
  - 入口与门禁：[顶栏组件](../../../../../web/src/components/workspace/GitWorkbenchTopBar.svelte)、[写门禁](../../../../../web/src/components/workspace/GitWorktreeGuardState.svelte)、[代码头部入口](../../../../../web/src/components/code/CodeWorkspaceHeader.svelte)；
  - 测试验证：[路由单测](../../../../../web/test/route.test.ts)、[记忆单测](../../../../../web/test/git-workbench-memory.test.ts)、[浏览器测试](../../../../../web/test/browser/GitWorkbenchRoute.test.ts)、[路由 E2E](../../../../../web/test/e2e/git-workbench-route.spec.ts)。
- Git 工作台历史视图：
  - 服务端引用与范围：[分支与标签枚举](../../../../../internal/workspace/git_refs.go)、[历史范围解析](../../../../../internal/workspace/history_scope.go)、[历史分页与拓扑](../../../../../internal/workspace/history.go)、[历史测试](../../../../../internal/workspace/history_test.go)；
  - 控制器：[历史与分页](../../../../../web/src/hooks/workspace/git-log-history.svelte.ts)、[两版本比较](../../../../../web/src/hooks/workspace/git-log-compare.svelte.ts)、[历史差异](../../../../../web/src/hooks/workspace/git-log-diff.svelte.ts)；
  - 收藏领域模型：[收藏分支](../../../../../web/src/domains/workspace/git-favorites.ts)；
  - 左侧与中间视图：[Log 主面板](../../../../../web/src/components/workspace/GitLogPane.svelte)、[引用树](../../../../../web/src/components/workspace/GitLogBranchTree.svelte)、[筛选栏](../../../../../web/src/components/workspace/GitLogFilterBar.svelte)；
  - 右侧变更与详情：[变更文件列表](../../../../../web/src/components/workspace/GitLogFilesPane.svelte)、[提交详情](../../../../../web/src/components/workspace/GitLogDetailPane.svelte)；
  - 底部与放大差异：[差异面板](../../../../../web/src/components/workspace/GitLogDiffPanel.svelte)、[Pierre 阅读器](../../../../../web/src/components/code/CodeDiffReader.svelte)；
  - 单测与组件测试：[收藏与深链单测](../../../../../web/test/git-log-controller.test.ts)、[竞态与分页浏览器单测](../../../../../web/test/browser/GitLogController.test.ts)、[工作台组件测试](../../../../../web/test/browser/GitWorkbench.test.ts)；
  - 端到端验证：[历史视图 E2E](../../../../../web/test/e2e/git-workbench-log.spec.ts)。
- Git 工作台提交暂存、三处共享分支动作与宽幅推送确认：
  - 服务端暂存与推送：[推送预览提交列表](../../../../../internal/workspace/push.go)、[Hunk 补丁解析](../../../../../internal/workspace/hunk.go)、[索引暂存写入](../../../../../internal/workspace/staging_write.go)、[推送测试](../../../../../internal/workspace/push_test.go)；
  - 服务端提交复核：[提交预览](../../../../../internal/workspace/commit_preview.go)、[提交执行复核](../../../../../internal/workspace/commit_execute.go)；
  - 暂存状态与主面板：[暂存控制器](../../../../../web/src/hooks/workspace/git-workbench-staging.svelte.ts)、[暂存差异阅读](../../../../../web/src/hooks/workspace/git-staging-diff.svelte.ts)、[提交主面板](../../../../../web/src/components/workspace/GitCommitPane.svelte)；
  - 提交编写与预览弹窗：[提交编写器](../../../../../web/src/components/workspace/GitCommitComposer.svelte)、[提交预览弹窗](../../../../../web/src/components/workspace/GitCommitPreviewDialog.svelte)；
  - 共享分支模型与视图：[分支模型](../../../../../web/src/domains/workspace/git-branches-model.ts)、[分支行与菜单](../../../../../web/src/components/workspace/GitBranchListItem.svelte)、[分支视图](../../../../../web/src/components/workspace/GitBranchesPane.svelte)；
  - 宽幅推送确认：[推送控制器](../../../../../web/src/hooks/workspace/git-push-controller.svelte.ts)、[宽幅推送框](../../../../../web/src/components/workspace/GitPushDialog.svelte)；
  - 单测与浏览器测试：[分支与审阅单测](../../../../../web/test/git-branches-and-staging.test.ts)、[提交推送浏览器测试](../../../../../web/test/browser/GitWorkbenchCommitPush.test.ts)；
  - 多视口与端到端测试：[Storybook 多视口测试](../../../../../web/test/storybook/git-workbench.spec.ts)、[提交/分支/推送 E2E](../../../../../web/test/e2e/git-workbench-commit-push.spec.ts)。
- 持久 Git 操作账本、重连幂等对账与底部状态栏：
  - 服务端账本与对账：[持久化目录登记](../../../../../internal/statefile/catalog.go)、[操作账本存储](../../../../../internal/workspace/operations_store.go)、[启动对账](../../../../../internal/workspace/operations_reconcile.go)、[幂等重放](../../../../../internal/workspace/operations_replay.go)；
  - 服务端测试：[工作区幂等与崩溃对账测试](../../../../../internal/workspace/operations_test.go)、[操作查询与 409 API 测试](../../../../../internal/api/git_operations_test.go)；
  - 前端控制器与组件：[操作领域模型](../../../../../web/src/domains/workspace/git-operations-model.ts)、[操作与重连控制器](../../../../../web/src/hooks/workspace/git-operations-controller.svelte.ts)、[底部状态栏与历史弹层](../../../../../web/src/components/workspace/GitWorkbenchStatusBar.svelte)；
  - 测试验证：[操作模型单测](../../../../../web/test/git-operations.test.ts)、[状态栏与重连浏览器测试](../../../../../web/test/browser/GitWorkbenchOperations.test.ts)、[持久账本与断线恢复 E2E](../../../../../web/test/e2e/git-workbench-operations.spec.ts)。
