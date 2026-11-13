# HTTP API：Git 工作台与仓库级接口

本页记录无需活动终端或混合会话即可直接使用的两套 Git 接口：

- 仓库级只读接口（`/api/repositories/{id}/git`）；
- 工作树级真实 `index` 暂存、提交、丢弃更改、受控贮藏、分叉冲突裁决及 `Smart Checkout` 接口（`/api/repositories/{id}/worktrees/{worktreeId}/git`）。

仓库登记、工作树生命周期、普通原地切换（`switch`）与会话内代码工作区见 [HTTP API：仓库与代码工作区](http-workspace.md)。

所有响应均带 `Cache-Control: no-store`，沿用 Bearer 认证与 Origin/Host 检查。

## 独立代码、历史对象比较与提交历史（`/api/repositories/{id}/git`）

| 路由                                                                | 说明                                                                                                           |
| ------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| `GET /context?worktreeId`                                           | 绑定仓库只读根（`repository` / `bare`）或工作树根（`worktree`），返回 `contextId` 与能力                       |
| `GET /tree?contextId&nodeId&includeIgnored&cursor`                  | 工作树目录分页列表                                                                                             |
| `GET /files?contextId&q&includeIgnored`                             | 工作树有界文件名搜索                                                                                           |
| `GET /file?contextId&nodeId&source`                                 | 文件快照，`source` 支持 `worktree`、`index`、`head`、`commit:{oid}` 与 `stash:{oid}`                           |
| `GET /changes?contextId&group&q&nodeId&cursor`                      | 工作树未暂存、已暂存与未跟踪修改分页列表                                                                       |
| `GET /diff?contextId&changeId`                                      | 工作树冻结双侧原文快照                                                                                         |
| `GET /compare?contextId&baseOid&targetOid&target&path&changeId...`  | 任意两提交（或根提交空树、`target=worktree` 工作区实时对比）间变更分页与单文件冻结双侧 Diff                    |
| `GET /stashes`                                                      | 列出仓库 `refs/stash` 栈条目（含 `oid`、`stashRef`、`branch`、`baseOid`、`hasUntracked`）                      |
| `GET /stashes/{oid}`                                                | 单条贮藏详情及合并后的已跟踪 + 第 3 父节点未跟踪（`untracked: true`）文件清单                                  |
| `GET /stashes/{oid}/diff?contextId&path&oldPath&untracked`          | 单条贮藏文件冻结双侧 Diff 快照（未跟踪文件自动以空基准对比 `S^3` blob）                                        |
| `POST /stashes/drop-preview {stashRef, expectedOid}`                | 预览丢弃单条贮藏并生成 `git stash apply <oid>` 恢复命令与 `confirmationId`                                     |
| `POST /stashes/drop {confirmationId, clientRequestId?}`             | 复核 `stashRef` 仍指向 `expectedOid` 后删除该贮藏条目（不提供 `stash clear`）                                  |
| `GET /file-history?path&ref&worktreeId&cursor&limit`                | 单文件只读 `--follow` 跨重命名提交历史分页（返回 `status`、`oldPath` 重命名轨迹与 `nextCursor`）               |
| `GET /blame?path&rev&worktreeId`                                    | 单文件只读 `git blame --porcelain`（`--no-optional-locks`）逐行溯源与父提交/前身路径元数据                     |
| `GET /snapshots/{snapshotId}/file?side=before\|after`               | 读取快照修改前或修改后的完整文档                                                                               |
| `GET /snapshots/{snapshotId}/raw?side=before\|after&download=1`     | 流式返回快照原始字节（≤100 MiB），复用 `/code` 传输槽与安全响应头；同时支持 `HEAD`                             |
| `GET /history?worktreeId&scope&branch&q&author&since&until&path...` | 拓扑分页历史图（`all` / `branch` / `task` / `to-push` / `incoming`），返回 `status`、`rows`、`taskBaseline` 等 |
| `GET /commits/{oid}?parentOid`                                      | 提交详情、作者/提交者元组、真实父节点、`comparedParentOid`（合并提交可指定合法父 OID）与变更文件               |
| `GET /operations?worktreeId&limit`                                  | 最近持久 Git 写操作账本（`limit` 默认 20，最大 100；支持按 `worktreeId` 过滤）                                 |
| `GET /operations/{operationId}`                                     | 单条 Git 写操作实例状态、预览摘要、复核原因、恢复对象/受控贮藏 OID 与脱敏输出摘要（跨仓库访问返回 404）        |

- 省略 `worktreeId` 时为仓库级只读上下文：允许 `compare` 与 `source=commit:{oid}`；调用工作区 `tree`、`files`、`changes`、`diff` 或 `source=worktree` 返回 409（`reason: "readonly"`）。
- 跨仓库复用 `contextId` / `snapshotId` 返回 404；同仓库跨工作树混用返回 409（`reason: "context_mismatch"`）；上下文或快照过期返回 409（`reason: "stale"`）。只读 JSON 路由 10 秒超时。

## 工作树真实 `index` 暂存、丢弃更改与高级操作（`/api/repositories/{id}/worktrees/{worktreeId}/git`）

| 路由                                                                     | 说明                                                                                                                                  |
| ------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------- |
| `GET /staging`                                                           | 读取真实 Git `index` 与工作区的已暂存、未暂存（含未跟踪）及冲突列表，签发 `StageFileIdentity`                                         |
| `GET /staging/diff?area&path&oldPath`                                    | 单文件 `unstaged` / `staged` 统一 Hunk 列表、行尾标记与可逐块/逐行暂存判定（`hunkStageAvailable`）                                    |
| `POST /stage {identity, hunkIds?, lineSelections?}`                      | 整文件、指定 `hunkIds` 或任意逐行 `lineSelections: [{hunkId, lineIndices}]` 暂存到真实 `index`（工作区原文件零改动），204             |
| `POST /unstage {identity, hunkIds?, lineSelections?}`                    | 整文件、指定 `hunkIds` 或任意逐行 `lineSelections` 从真实 `index` 取消暂存（含 `unborn HEAD`，工作区原文件零改动），204               |
| `POST /discard-preview {scope, paths?, identities?, identity?, ...}`     | 两阶段预览丢弃未暂存修改或未跟踪文件（支持 `files` / `hunks` / `lines`），列出覆盖/删除清单、活跃会话与限时 `confirmationId`          |
| `POST /discard {confirmationId, acknowledgeSessions?, clientRequestId?}` | 写盘前复核全部目标 SHA-256（防 409 误覆盖）、将原内容保存至 `refs/webmux/discard-recovery/<opId>` 后改写工作区，绝不触碰已暂存索引    |
| `POST /smart-checkout-preview {target, includeUntracked?}`               | 当未提交改动阻断普通切换时，预览三阶段受控贮藏原地切换（统计暂存/未暂存/未跟踪数量、活跃会话与 `confirmationId`）                     |
| `POST /smart-checkout {confirmationId, acknowledgeSessions?, ...}`       | 执行三阶段 `stashing -> switching -> unstashing` 原地切换：切换失败自动回放恢复原状，目标冲突时保留受控贮藏并返回三方冲突状态         |
| `POST /commit-preview {amend?, authorOverride?, signoff?}`               | 基于完整 `HEAD -> index`（`amend` 时为 `HEAD^ -> index`）生成预览，返回 `amendPushed`、`headCommitMessage`、`signoffTrailer` 与确认码 |
| `POST /commit {confirmationId, message, amend?, ..., clientRequestId?}`  | 复核快照、高级提交参数与已推送 `acknowledgePushedAmend` 后执行提交，返回 `{operationId?, outcome}`                                    |
| `POST /stashes/create-preview {message?, includeUntracked?, keepIndex?}` | 预览将当前工作树改动打包为新贮藏（统计已暂存/未暂存/未跟踪数量与活跃会话确认）                                                        |
| `POST /stashes/create {confirmationId, acknowledgeSessions?, ...}`       | 执行 `git stash push` 创建贮藏条目并返回新 `entry` 与 `operationId`                                                                   |
| `POST /stashes/apply-preview {stashRef, expectedOid, mode, ...}`         | 预览 `apply` 或安全两步 `pop`（含 `--index` 选项、重叠文件预判与活跃会话确认）                                                        |
| `POST /stashes/apply {confirmationId, acknowledgeSessions?, ...}`        | 执行应用或安全两步弹出：若遇合并冲突绝不丢弃原贮藏，返回 `status: "conflicted"` 与 `operation-state`（`kind: "unstash"`）             |
| `GET /task-baseline`                                                     | 读取工作树固定任务审阅基线 `{baseline}`（未记录时 `baseline: null`，仍为 200）                                                        |
| `PUT /task-baseline {baseOid?}`                                          | 将指定完整提交 OID（省略时取当前 `HEAD`，含 `unborn` 空仓）设为工作树任务基线，返回 `{baseline}`                                      |
| `DELETE /task-baseline`                                                  | 清除该工作树的任务审阅基线与私有保护引用，204                                                                                         |
| `GET /task-changes?cursor&q&nodeId`                                      | 计算相对任务起点的单份净变更分页列表（含 `commitCount`、`uncommittedCount` 与 `no_task_baseline` 状态）                               |
| `GET /task-diff?path&changeId`                                           | 读取单份任务净变更文件（`baseOid`/空树 → 当前工作区）冻结双侧原文，并写入 `/git/snapshots/{snapshotId}`                               |
| `GET /reviews?scope=uncommitted\|task`                                   | 读取工作树跨设备共享的已审阅文件列表与汇总计数（`totalFiles`、`reviewedFiles`、`files`）                                              |
| `PUT /reviews {scope, action, item?, path?}`                             | 按内容身份标记/取消标记已审阅（`mark` / `unmark` / `clear`）；文件已变化时返回 200 `{applied: false, staleRejected: true}`            |
| `GET /commit-draft`                                                      | 读取工作树跨设备同步的提交说明草稿 `{repositoryId, worktreeId, subject, body, revision, updatedAt}`                                   |
| `PUT /commit-draft {subject, body, baseRevision}`                        | 乐观锁保存提交说明草稿（上限 32 KiB）；`baseRevision` 不匹配时返回 409（`reason: "draft_conflict"`）并附带 `currentDraft`             |
| `GET /operation-state`                                                   | 无状态探查进行中的 `merge` / `rebase` / `cherry_pick` / `revert` / `unstash` 进度、冲突列表与继续/中止门禁                            |
| `POST /merge-preview {refKind, refName}`                                 | 预览将本地/远端分支合入当前工作树分支：返回方向语句、提交数、`git merge-tree` 冲突预判、活跃会话与限时 `confirmationId`               |
| `POST /merge {confirmationId, acknowledgeSessions?, clientRequestId?}`   | 复核快照与会话确认后执行非交互合并，返回 `{operationId?, status, inProgress, summary}`                                                |
| `POST /rebase-preview {refKind, refName}`                                | 预览将当前工作树分支变基到目标分支基底：返回方向语句、待重放提交列表、冲突预判、活跃会话与限时 `confirmationId`                       |
| `POST /rebase {confirmationId, acknowledgeSessions?, clientRequestId?}`  | 复核快照与会话确认后执行非交互变基，返回 `{operationId?, status, inProgress, summary}`                                                |
| `POST /cherry-pick-preview {commitOid}`                                  | 预览将指定提交遴选到当前工作树分支（合并提交需先按单亲提交拆解，否则返回阻塞提示）                                                    |
| `POST /cherry-pick {confirmationId, acknowledgeSessions?, ...}`          | 复核快照与会话确认后执行遴选，返回 `{operationId?, status, inProgress, summary}`                                                      |
| `POST /revert-preview {commitOid}`                                       | 预览在当前工作树分支生成还原指定提交的反向更改                                                                                        |
| `POST /revert {confirmationId, acknowledgeSessions?, clientRequestId?}`  | 复核快照与会话确认后执行还原，返回 `{operationId?, status, inProgress, summary}`                                                      |
| `POST /operation-continue {kind?, clientRequestId?}`                     | 仅当未合并索引条目（`1..3` 阶）清零时继续完成进行中的序列操作，返回 `{operationId?, status, inProgress, summary}`                     |
| `POST /operation-abort-preview {}`                                       | 预览中止进行中操作的回退锚点 OID（`ORIG_HEAD` 或 `refs/webmux/recovery/*`）、将回滚的路径清单、活跃会话与 `confirmationId`            |
| `POST /operation-abort {confirmationId, acknowledgeSessions?, ...}`      | 复核确认码与会话确认后中止进行中操作并清理 `refs/webmux/recovery/*` 锚点                                                              |
| `GET /conflict?path`                                                     | 读取单文件三方冲突详情：索引 `1/2/3` 阶与工作区快照、真实分支/提交角色标签、三方分段（`segments`）及非文本冲突变体                    |
| `POST /conflict/save {path, stageIdentityKey, content}`                  | 校验无遗留冲突标记且 `1..3` 阶身份未变后，经 `os.Root` 原子写回工作区文件（**不暂存**，保留 `1..3` 阶）                               |
| `POST /conflict/resolve {path, stageIdentityKey, resolution, diskSHA?}`  | 将已保存合并文本或显式 `choose_left` / `choose_right` / `delete` 写入索引 `0` 阶（或移除条目）并清除 `1..3` 阶，返回最新 `operation`  |

## 安全与一致性不变量

- **任意逐行暂存与取消暂存（Sub-hunk `lineSelections`）**：
  - `hunkIds` 与 `lineSelections` 互斥（同时传入返回 400）；二进制、符号链接、重命名、含 `.gitattributes` 过滤器/编码转换或冲突条目拒绝逐块/逐行操作（409）。
  - 逐行暂存（`index -> worktree`）：仅将选中的 `-` 行从 `index` 删除、未选中的 `-` 行保留在 `index`；仅将选中的 `+` 行写入 `index`、未选中的 `+` 行不进入 `index`。
  - 逐行取消暂存（`HEAD -> index`）：仅把 `index` 中选中的变更行还原回 `HEAD` 版本（未选中的已暂存行保留在 `index`），**全程不改写工作区文件任何字节**。
  - 严格保留原始文件模式（`100644` / `100755`）、行尾（`LF` / `CRLF`）与文件末尾无换行（`none`）语义，经 `git hash-object -w --stdin` 与 `git update-index --cacheinfo` 更新真实索引；`StageFileIdentity` 过期立即返回 409（`reason: "stale"`）。
- **受控丢弃更改（`discard-preview` / `discard`）**：
  - 与只改索引的 `unstage` 严格分离：仅还原工作区未暂存改动（回退到 `index` 或 `HEAD` 内容）或删除未跟踪文件，**绝不修改已暂存 `.git/index`**。
  - 执行写入前在仓库门闩下逐路径重算磁盘文件 SHA-256 摘要，任一路径自预览后被外部终端或 CLI Agent 修改则拒绝覆盖并返回 409（`reason: "stale"`）。
  - 改写磁盘前先将原工作区文件与未跟踪文件写入有界私有恢复提交 `refs/webmux/discard-recovery/<operationId>`（每仓保留最近 20 条，移除工作树时同步清理），并在操作账本记录 `recoveryOid` 与 `git restore --source=<oid> --worktree -- <paths>` 恢复命令。
- **基于可追踪受控 Stash 的三阶段 `Smart Checkout`**：
  - 绑定单一 `operationId` 并在每个阶段落盘持久化至 `git-operations.json`（含 `phase`、`stashOid`、`sourceBranch`、`targetBranch`）：
    1. `stashing`：创建带唯一标记 `webmux-smart-checkout:<operationId>` 的受控贮藏（可选包含未跟踪文件），确认解析出精确 `stashOid` 并落盘后才允许触碰 `HEAD`；
    2. `switching`：切换至目标分支；若切换失败，自动在原分支回放恢复 `stashOid`（恢复成功清理临时贮藏，恢复异常保留 `stashOid` 并返回恢复命令），确保零数据丢失；
    3. `unstashing`：在目标分支优先执行 `git stash apply --index --quiet <stashOid>`（索引无法原样恢复时回退不带 `--index` 应用）：
       - 仅当 100% 无冲突恢复成功时，按精确 OID 丢弃该受控临时贮藏（绝不误删用户自有贮藏）；
       - 若遇合并冲突，**100% 保留 `stashOid`**、记录 `outcome: "conflict_needs_resolution"`，并在 `GET /operation-state` 暴露 `"Smart Checkout 贮藏 <shortOid>"` 来源供内置三方冲突解决器裁决。
- **任务审阅基线（`git-task-baselines.json`）**：
  - 新建工作树自动记录 `source: "worktree_create"`；`unborn` 空仓首个会话启动记录 `unborn: true`（绝不伪造全零 OID）；手动设置记录 `source: "manual_select"` 并记录 `cleanAtStart` / `preExistingChanges`。
  - 非 `unborn` 基线写入 `refs/webmux/baselines/<worktreeId>` 防 `git gc` 回收，绝不随 `fetch`、上游更新或本地 `HEAD` 前进自动漂移；移除/释放工作树、注销仓库或 `DELETE /task-baseline` 同步清理。
  - `GET /task-changes` 将起点树与当前工作区合并为每路径至多一条净变更；未记录基线时 `GET /task-changes` 与 `GET /history?scope=task` 返回 200（`status: "no_task_baseline"`），`GET /task-diff` 返回 409（`reason: "no_task_baseline"`）。
- **锁与错误码（统一返回 409）**：
  - 外部 `.git/index.lock` 存在或并发写锁冲突返回 `reason: "busy"`，绝不删除外部锁文件；
  - 身份或确认码过期返回 `reason: "stale"`；存在未解决冲突条目返回 `reason: "conflicted"`；
  - 活跃会话未勾选 `acknowledgeSessions: true` 返回 `reason: "session_ack_required"`；Hook 或签名失败返回 `reason: "commit_failed"`。
- **分叉序列操作与三方冲突裁决**：
  - 发起序列操作前将原 `HEAD` 写入 `refs/webmux/recovery/<worktreeId>/<kind>`；完成、中止或删树时同步清理。
  - `GET /conflict` 在 `rebase` 期间保持界面左右列稳定（`left` 映射 `stage 3` 当前分支重放提交，`right` 映射 `stage 2` 变基目标基底）。
  - `POST /conflict/save` 拒绝含遗留 `<<<<<<<` / `=======` / `>>>>>>>` 标记的文本（409 `reason: "conflict_markers_present"`），经 `internal/worktreefs` 的 `os.Root` 原子写回工作区文件且不修改 `.git/index`。
  - `POST /conflict/resolve` 支持 `"text"`、`"choose_left"`、`"choose_right"` 与 `"delete"`；仍有未合并条目时 `POST /operation-continue` 返回 409（`reason: "unmerged_entries"`）。
- **贮藏、高级提交与只读文件溯源**：
  - `POST /stashes/apply` 的 `mode: "pop"` 先 `apply --quiet <expectedOid>`，仅无冲突成功时清理条目，冲突时 100% 保留原贮藏并报告 `kind: "unstash"`（`canContinue: false`）。
  - `POST /commit` 当 `amend: true` 且 `amendPushed: true` 时强制校验 `acknowledgePushedAmend: true`（否则 409 `reason: "pushed_amend_ack_required"`）；`authorOverride` 校验 `Name <email>` 格式并拒绝 `--` 选项注入（400）。
  - `GET /blame` 显式附加 `--no-optional-locks` 执行只读 `git blame --porcelain`。
- **持久操作账本（`git-operations.json`）与跨设备状态（`git-workbench-state.json`）**：
  - 所有受控 Git 写操作（含 `discard` 与 `smart_checkout`）统一写入 `git-operations.json`：
    - 同一 `(repositoryId, clientRequestId)` 参数一致时合并在途请求或幂等重放结果，参数不一致返回 409（`reason: "idempotency_conflict"`）；
    - 服务启动按真实引用、`refs/stash` 与 reflog 执行只读对账，从不自动重跑或执行 `git reset`。
  - `POST /commit` 使用 2 分钟受控执行窗口，日志与持久账本绝不记录文件内容、Diff 正文、草稿正文或凭据（输出摘要脱敏并截断 ≤ 4 KiB）。
  - `git-workbench-state.json` 保存工作树审阅标记（绑定模式、blob OID 与 SHA-256，过期 `mark` 返回 200 `{applied: false, staleRejected: true}`）与乐观锁提交草稿（`baseRevision` 冲突返回 409 `reason: "draft_conflict"` 并附带 `currentDraft`；仅真实提交落盘后清空草稿）。
