# 任务基线、协同、冲突解决与日常高级操作（US-J24–US-J28）

本页收录 Git 工作台的任务审阅基线、跨设备审阅与返工交回会话、合并/变基/遴选/还原与三方冲突解决器、独立 `Stash` 视图与日常高级操作，以及任意逐行（Sub-hunk）暂存、受控丢弃更改与三阶段 `Smart Checkout` 故事（US-J24–US-J28）。
基础故事见[Git 工作台基础闭环与账本](core.md)，分索引见[Git 工作台与仓库级接口](../git-workbench.md)。

## J. 任务基线、协同、冲突解决与日常高级操作

- **US-J24 工作树固定任务审阅基线、单份净变更审阅与任务交付收尾摘要**：作为在任务工作树中委派 CLI Agent 多轮提交修改后再统一审阅交付的使用者，我希望工作树固定记录任务起点基线，支持在工作区完全干净时按单份净差异审阅全部任务变化、筛选任务范围提交，并在收尾摘要中完成返回会话、推送、归档与安全清理。
  验收要点：
  - 固定任务基线自动捕获、手动指定与 GC 保护生命周期：
    - 通过 WebMux 新建工作树（含快速开始新建任务工作树）时自动记录 `source: "worktree_create"` 的 `baseOid` 基线；`unborn` 空仓启动首个会话时自动记录 `unborn: true`（绝不伪造全零 OID）；
    - 存量未记录基线的工作树支持通过 `PUT .../git/task-baseline`（将当前 `HEAD` 或 `Log` 历史提交设为起点，记录 `source: "manual_select"`）与 `DELETE .../git/task-baseline` 管理；
    - 若手动设定基线时工作区存在未提交改动，`cleanAtStart` 为 `false` 并记录 `preExistingChanges`，界面显式提示“相对所选提交的全部变化（含设定前已存在修改）”；
    - 基线持久化于 `git-task-baselines.json`（纳入 `statefile.Catalog`、`persistence_inventory.json` 与 `webmux config show`），非 `unborn` 基线同步写入 `refs/webmux/baselines/<worktreeId>` 防止 `git gc` 回收对象；
    - 基线一经记录绝不随 `fetch`、远端默认分支推进或本地 `HEAD` 前进漂移；关闭或归档会话保留基线，移除/批量移除/释放工作树、注销仓库或清除基线时同步清理基线记录与私有引用；
  - 单份净变更聚合审阅与干净工作区完整可读（`A09`）：
    - `Commit` 视图顶部提供 `未提交改动` 与 `相对任务起点的变化` 双审阅范围切换并支持 `?view=commit&scope=task` 深链直达；
    - `GET .../git/task-changes` 与 `GET .../git/task-diff` 将任务起点树与当前工作区状态合并为每条路径最多一项净变更：同一文件历经多笔提交与未提交编辑绝不重复计数，修改后还原归零，重命名保留 `oldPath → newPath`，并接入 `/git/snapshots/{snapshotId}` 支持变更前后全文与原始字节下载；
    - 当 Agent 已将全部修改提交且工作区 100% 干净（`uncommittedCount === 0`）时，`相对任务起点的变化` 仍完整列出任务净变更文件并在 Pierre `CodeDiffReader` 中阅读差异，同时在 `未提交改动` 空状态展示一键切换审阅入口；
  - 任务交付与收尾摘要及非自动删树保护（`G12`）：
    - 底部收尾摘要汇总净变更文件数、任务提交数、未提交项、待推提交与工作树关联的运行中/已结束/已归档会话计数；
    - 提供“返回会话”、“推送提交…”（直达 ~900px 宽幅 `GitPushDialog`）、“归档已完成会话”与 `linked` 工作树的“移除工作树…”（复用 `RemoveWorktreeDialog` 安全检查）出口，推送成功绝不自动删除工作树。
- **US-J25 跨设备审阅进度同步、乐观锁提交草稿与返工意见交回指定会话**：作为多设备协同审阅者，我希望各工作树的“已审阅”进度与提交草稿经服务端安全同步，并可把带版本身份与行范围的结构化返工意见放入指定工作树会话草稿框，以闭合“编码 → 审阅 → 返工 / 提交”回路。
  验收要点：
  - 绑定内容身份的服务端审阅进度同步与自动失效（`GET/PUT .../git/reviews`）：
    - 按工作树在 `git-workbench-state.json`（`0600` 原子替换 + 独立锁，纳入 `statefile.Catalog`、`persistence_inventory.json` 与 `webmux config show`）跨设备共享 `uncommitted`（`staged`、`unstaged`、`untracked`）与 `task` 两套审阅范围进度；
    - 审阅键同时绑定两侧文件模式、Git blob OID 与工作区文件内容 SHA-256 摘要（从不依赖 `mtime`）：外部修改文件或变更暂存内容后旧标记自动失效；过期页面发起旧身份 `mark` 时返回 `200 {applied: false, staleRejected: true}` 与最新快照，绝不把新修改误标为已审阅；
    - `Commit` 与 `相对任务起点的变化` 顶部常驻展示已审阅进度与一键“下一处未审阅”按钮，自动循环定位下一处未审阅条目（窄屏自动切入 Diff 阅读页）；
  - 带 `baseRevision` 乐观版本检查的跨设备提交草稿（`GET/PUT .../git/commit-draft`）：
    - 按工作树持久保存提交 `subject`、`body`、`revision` 与 `updatedAt`（单工作树上限 32 KiB，日志绝不输出草稿正文）；工作树存在活动会话且已暂存文件 `> 0` 时提供“让会话协助起草说明”辅助入口；
    - 仅当真实提交成功落盘（`ExecuteCommit` 或服务重启对账确认为 `matched` / `needs_review`）后才自动清空草稿；预览过期、Hook/锁冲突报错、`uncertain` 或取消推送均完整保留草稿；
    - 多设备并发编辑导致 `baseRevision` 不匹配时返回 HTTP `409`（`reason: "draft_conflict"`，附带 `currentDraft`），界面原位并排保留“本机当前输入”与“另一设备已保存版本”，提供“保留本机覆盖保存”、“采用远端版本”与“合并两份内容”三种显式恢复操作，绝不静默覆盖；
    - 移除/批量移除/释放工作树或注销仓库时同步清理该工作树或仓库的提交草稿与审阅记录；
  - 差异摘录返工意见草稿箱与目标会话安全交回：
    - 在 `Commit`、`相对任务起点的变化` 与 `Log` 的任意 Pierre `CodeDiffReader` 中选择单行/连续行范围（或点击常驻 Hunk 条“加入意见”），均可连同修改要求加入按工作树隔离的“返工意见草稿箱”；编辑或发送返工意见绝不覆盖或清空正在编写的提交说明草稿；
    - 防串行号保护：来自历史提交、`HEAD`、`index` 或任务起点删除侧的摘录显式标明版本身份并附带代码片段，绝不伪装成当前工作区实时行号；来自当前工作区的摘录在发送前向服务端复核最新内容身份，若文件已更新则先提示确认；
    - 目标会话显式路由：工作树仅有 1 个活跃会话时默认预选并标注视图（`终端 TUI` / `聊天视图`）；有 2 个及以上活跃会话时强制显式单选目标会话后才允许发送；无活跃会话时提供“复制结构化审阅意见 Markdown”与“在该工作树新建会话并带入意见”（预填目标工作树与草稿）；
    - 只进草稿框、零自动执行：确认发送后跳转到目标会话并将结构化 Markdown 写入该会话草稿（终端会话展开输入草稿框且绝不注入 `\r` 回车自动执行，混合聊天视图填入聊天输入框），其余未选中会话保持原状。
- **US-J26 Git 工作台合并、变基、遴选、还原协调恢复与三方冲突解决器**：作为处理分叉分支或冲突提交的使用者，我希望在零活跃会话浏览器中显式预览执行 `Merge`、`Rebase`、`Cherry-pick` 与 `Revert`，重进恢复进行中横幅与诚实 `Abort` 预览，并用三方冲突解决器完成裁决与继续。
  验收要点：
  - 显式方向与爆炸半径预览及分叉推送闭环（`UI-05`、`UI-06`）：
    - 多处统一操作入口：
      - 顶栏分支弹层、`Log` 引用树与 `Branches` 视图提供“合并到当前分支… (Merge)”与“将当前分支变基到此处… (Rebase)”；
      - `Log` 提交行菜单与提交详情提供“遴选此提交… (Cherry-pick)”与“还原此提交… (Revert)”；
      - `GitPushDialog` 在远端非快进拒绝后原位展示“获取远程 (Fetch)”、“快进更新…”、“合并远端分支… (Merge)”与“变基到远端分支… (Rebase)”（全程禁止 `--force`）；
    - 两阶段预览与活跃会话确认：
      - 四类操作均采用 `*-preview` → 限时 `confirmationId` 确认，列出目标工作树、当前分支、源引用或提交、方向语句（`merge A into B` / `rebase B onto A`）、待合入或重放提交计数及 `git merge-tree` 预判冲突路径；
      - 目标工作树存在运行中会话时强制要求显式勾选 `acknowledgeSessions`（未确认返回 `409 session_ack_required`）；
    - 执行前自动写入私有保护引用 `refs/webmux/recovery/<worktreeId>/<kind>` 保存操作前 `HEAD`，完成后、中止后或移除/释放工作树时自动清理；
  - 无状态重进协调恢复横幅与诚实 `Abort` 预览（`A13`、`G10`）：
    - 无状态探查与重进恢复：
      - `GET .../git/operation-state` 直接从 `.git` / worktree gitdir（`MERGE_HEAD`、`rebase-merge/`、`rebase-apply/`、`CHERRY_PICK_HEAD`、`REVERT_HEAD`、`ORIG_HEAD` 及 `refs/webmux/recovery/*`）与索引未合并条目重建进行中状态；
      - 页面刷新、重连或从终端发起冲突操作后进入工作台均自动挂载 `GitInProgressBanner`；
    - `继续操作 (Continue)` 在工作树仍存在任何未合并索引条目（`1..3` 阶未清空）时保持禁用，服务端 `POST .../git/operation-continue` 同步拦截并返回 `409 unmerged_entries`；
    - `POST .../git/operation-abort-preview` 与 `GitAbortOperationDialog` 诚实展示回退目标提交 OID（`ORIG_HEAD` 或恢复锚点）、将回滚的路径清单与活跃会话确认，确认后干净恢复至操作前状态；
  - 三方冲突解决器 `GitConflictView`、非颠倒变基标签与四步递进（`UI-07`、`U14`、`UI-12`）：
    - 桌面三栏与窄屏常驻结果布局：
      - 桌面端（`>= 1280px`）固定展示 `左侧只读来源 | 中间可编辑结果 (Result) | 右侧只读来源` 三栏，共同祖先 `Base`（`stage 1`）通过顶部对照切换器与弹层查看，**绝不顶替中间 `Result` 栏**；
      - 窄屏（`< 1280px`，含 `320px` 与 `200%` 根字号）通过顶部 `SegmentedControl` 切换 `左侧来源 / 右侧来源 / 共同祖先 Base` 与逐块裁决卡，同时在底部**常驻显示可编辑 `Result` 区域**且零横向溢出；
    - `rebase` 期间服务端将 `left` 固定映射为 `stage 3`（当前分支重放提交）、`right` 固定映射为 `stage 2`（变基目标基底），界面左右列永不对调，并在列头显式展示真实分支名、提交短 OID 与语义角色；
    - 文本冲突四步递进门禁：
      1. 草稿态：逐块接受左/右/双侧、一键应用全部非冲突改动或手动编辑；
      2. `保存结果 (Save Result)`：拦截遗留 `<<<<<<<` 冲突标记，经 `internal/worktreefs` 的 `os.Root` 原子写回工作区文件，**不自动暂存**，`1..3` 阶保留；
      3. `标记已解决 (Mark as Resolved)`：核对 `stageIdentityKey` 与 `diskSHA` 后写入 `stage 0` 并清除 `1..3` 阶；
      4. `继续操作 (Continue)`：仅当全部未合并条目清零后启用并继续序列操作；
    - 二进制、删改冲突（`delete_modify`）、子模块、符号链接与超大文件展示专用裁决卡（`GitConflictVariantCard`），支持显式 `choose_left`、`choose_right` 或 `delete` 并清除未合并索引阶。
- **US-J27 Git 工作台独立 Stash 视图、高级提交保护、引用远程维护与单文件历史及 Blame 溯源**：
  作为日常封存工作区、修正本地提交、维护引用远程并查证代码演进的使用者，我希望在工作台直接使用独立 `Stash` 视图与安全两步 `Pop`、受推送保护的 `Amend`/作者/`Sign-off`、分支/标签/远程维护，以及跨重命名单文件历史与逐行 `Blame`。
  验收要点：
  - 独立仓库级 `Stash` 视图与安全两步弹出（绝不因冲突丢弃贮藏）：
    - 顶栏模式切换支持 `贮藏`（`?view=stash`）及提交面板快捷创建入口，因 `refs/stash` 归属仓库共享栈而在只读仓库与各工作树一致列出 `oid`、`stashRef`、`branch`、`baseOid` 与 `hasUntracked`；
    - 新建贮藏采用两阶段预览确认，支持自定义说明、`-u`（包含未跟踪文件）与 `--keep-index`，活跃会话存在时要求显式勾选知晓确认；
    - 贮藏详情合并展示已跟踪改动（`base -> S`）与第 3 父节点未跟踪文件（`S^3`，标注 `未跟踪 (S^3)`），复用 Pierre `CodeDiffReader` 只读检视差异；
    - `Apply` 与 `Pop` 支持 `--index` 恢复原暂存态：
      - 安全两步 `Pop` 先执行 `git stash apply --quiet <oid>`，仅在无冲突成功时才按精确 OID 清理条目；
      - 一旦产生合并冲突则**100% 保留原贮藏条目**并直接进入 `GitConflictView`（`operation.kind = "unstash"`，`canContinue = false`，隐藏多步序列 `Continue` 按钮）；
    - 单条 `Drop` 复核 `stashRef` 仍指向预期 `oid` 后删除并提供 `git stash apply <oid>` 恢复命令，全程不提供 `stash clear`；
  - 日常高级提交选项与已推送 `HEAD` 防误改历史门禁：
    - 支持 `--amend`（含空暂存区纯改说明）；勾选时若已有非空提交草稿则提示“替换为 HEAD 说明 / 保留当前草稿 / 追加在草稿后”，绝不静默覆盖；
    - 服务端基于本地远程跟踪引用只读判定 `amendPushed`：当 `HEAD` 已推送时，编写区展示分叉风险警告、**自动禁用一键“提交并推送…”**，且提交预览框与 `POST .../git/commit` 强制要求 `acknowledgePushedAmend: true`（未确认返回 `409 pushed_amend_ack_required`），绝不提供 `--force`；
    - 支持单次提交作者覆盖（严格校验 `Name <email>`，仅改 Author 不改 Committer）与 `Signed-off-by`（`-s`）实时尾注预览；
  - 本地分支重命名、上游绑定、标签管理与远程凭据保护：
    - 本地分支支持两阶段 `git branch -m` 重命名预览与执行（含当前检出分支与活跃会话确认），以及显式绑定本地远程跟踪分支上游或一键解除上游；
    - 支持在 `HEAD` 或 `Log` 任意提交创建轻量标签或附注标签（`git tag -a`），在 `Branches` 视图 `标签 (Tags)` 分组展示详情，删除时校验预期指向并返回 `git tag <name> <oid>` 恢复命令；
    - 支持添加、修改与移除远程仓库配置：服务端返回前将内嵌口令脱敏为 `***`，写入时校验允许的 URL 协议并拒绝包含字面量 `***` 的请求（`400`），防止脱敏占位符覆写 `.git/config`；
  - 单文件 `--follow` 历史、只读逐行 `Blame` 链式溯源与跨版本比较：
    - 从 Git 工作台变更/历史文件行及会话代码工作区头部按钮打开单文件检视器，提供 `文件历史 (--follow)`、`逐行溯源 (Blame)` 与 `跨版本比较` 三页签；
    - `GET .../git/file-history` 追踪文件跨重命名演进并展示 `oldPath → newPath` 与单提交文件 Diff；
    - `GET .../git/blame` 以 `--no-optional-locks` 执行只读 `git blame --porcelain`，支持按行查看提交信息、沿父提交与前身路径“追溯此提交之前版本”（带面包屑回退）及一键“在历史图中定位”跳转到 `Log`；跨版本比较支持任意提交版本与 `HEAD` 或实时 `worktree` 对照。
- **US-J28 Git 工作台任意逐行（Sub-hunk）暂存、受控丢弃更改与基于可追踪 Stash 的三阶段 Smart Checkout**：
  作为在同一文件中混杂多处修改或需带未提交改动原地切换分支的使用者，我希望按任意单行或行范围操作真实 Git `index`、在防陈旧覆盖与快照保护下丢弃局部更改，并通过可追踪受控 Stash 安全执行三阶段 `Smart Checkout`。
  验收要点：
  - 任意逐行（Sub-hunk）暂存与取消暂存（`UI-04`、`G02`、`A03`）：
    - `POST .../git/stage` 与 `POST .../git/unstage` 支持 `lineSelections: [{hunkId, lineIndices}]`（与 `hunkIds` 互斥），对二进制、符号链接、重命名、含 `.gitattributes` 过滤器或冲突文件拒绝逐行操作（`409`）；
    - 逐行暂存（`index -> worktree`）仅将选中的 `-` 行从 `index` 删除、选中的 `+` 行写入 `index`（未选中的 `-` 行保留在 `index`、未选中的 `+` 行不进入 `index`）；逐行取消暂存（`HEAD -> index`）仅把 `index` 中选中的变更行还原回 `HEAD`（含 `unborn HEAD`），**全程不改写工作区文件任何字节**；
    - 合成目标索引 blob 时完整保留文件权限（`100644` / `100755`）、行尾（`LF` / `CRLF`）与文件末尾无换行（`none`）语义，校验 `StageFileIdentity` 过期即返回 `409 stale`；
    - 前端提供桌面与触屏常驻可达的逐行选择面板（单行复选框、`Shift+点击` 连续范围、按块全选/清空，窄屏隐藏重复 Hunk 条保持 `320px` 与 `200%` 大字号零横向溢出），并支持在 Pierre `CodeDiffReader` 中拖选行范围直接一键暂存或取消暂存选中变更行；
  - 受控 `丢弃更改 (Discard Changes)`、写盘前新鲜度门禁与恢复快照（`G11`）：
    - 与只改 `.git/index` 的 `Unstage` 严格分离：仅面向 `Unstaged` 与 `Untracked` 开放单文件、左侧列表多选文件（含已跟踪 + 未跟踪混合批量）、单 Hunk 与选中行四级丢弃范围，**绝不触碰已暂存索引条目**；
    - 两阶段预览确认（`POST .../git/discard-preview` → `POST .../git/discard`）列出将被覆盖或删除的路径及活跃会话警告（有运行中会话时强制要求 `acknowledgeSessions`）；
    - 执行写盘前在仓库锁下逐一重算目标磁盘文件最新 SHA-256，若预览打开后文件被终端或 CLI Agent 修改则立即返回 `409 stale` 拦截误覆盖，支持一键按最新身份刷新预览；
    - 改写或删除前自动将原工作区内容保存至有界私有恢复提交 `refs/webmux/discard-recovery/<operationId>`（每仓保留最近 20 条，删树同步清理），在持久操作账本与顶栏操作账本浮层披露 `recoveryOid` 及 `git restore --source=<oid> --worktree -- <paths>` 恢复命令；
  - 基于可追踪受控 Stash 的三阶段 `Smart Checkout` 与活跃 Agent 会话保护（`UI-08`、`G11`、衔接 `US-J16`）：
    - 当 `SwitchBranchDialog` 仅因未提交改动受阻（无进行中序列冲突、无其他工作树占用）时，在保留绿色推荐按钮 **“改为在新工作树打开”** 的同时提供显式 `Smart Checkout`（`暂存当前改动 -> 切换到目标分支 -> 在目标分支恢复改动`，支持勾选包含未跟踪文件）；
    - **活跃 CLI Agent 会话门禁**：当工作树存在运行中会话时，“改为在新工作树打开”始终保持主绿色默认入口，原地 `Smart Checkout` 默认收纳在风险说明折叠区并要求显式勾选会话知晓确认；
    - 持久三阶段状态机（`POST .../git/smart-checkout-preview` → `POST .../git/smart-checkout`，`git-operations.json` 记录 `kind: "smart_checkout"`、`phase`、`stashOid`、`sourceBranch`、`targetBranch` 并支持重启只读对账）：
      1. `stashing`：创建带 `webmux-smart-checkout:<operationId>` 标记的受控贮藏并确认落盘精确 `stashOid` 后才允许切换 `HEAD`；
      2. `switching`：切换到目标分支；若切换失败则自动在原分支回放恢复 `stashOid`（零静默丢数）；
      3. `unstashing`：在目标分支优先以 `--index` 应用 `stashOid`：
         - 仅当 100% 无冲突恢复时按精确 OID 丢弃该受控临时贮藏（绝不误删用户自有贮藏）；
         - 若产生合并冲突，**100% 保留 `stashOid`**、给出 `git stash apply <oid>` 恢复命令，并一键打开 `GitConflictView`（标注 `"Smart Checkout 贮藏 <shortOid>"` 来源）完成三方裁决。

依据：

- 工作树固定任务审阅基线、单份净变更与收尾摘要：
  - 服务端存储与路由：[基线存储](../../../../../internal/workspace/task_baseline_store.go)、[基线捕获与引用保护](../../../../../internal/workspace/task_baseline.go)、[任务接口路由](../../../../../internal/api/git_task.go)；
  - 服务端净差异：[单份净变更与快照](../../../../../internal/codeview/task_diff.go)；
  - 服务端测试：[基线生命周期测试](../../../../../internal/workspace/task_baseline_test.go)、[单份净差异测试](../../../../../internal/codeview/task_diff_test.go)、[任务接口测试](../../../../../internal/api/git_task_test.go)；
  - 前端请求与状态：[任务接口封装](../../../../../web/src/platform/git-task-api.ts)、[任务审阅控制器](../../../../../web/src/hooks/workspace/git-task-review.svelte.ts)；
  - 前端面板组件：[任务净变更面板](../../../../../web/src/components/workspace/GitTaskReviewPane.svelte)、[任务收尾摘要](../../../../../web/src/components/workspace/GitTaskDeliverySummary.svelte)；
  - 前端测试：[任务接口单测](../../../../../web/test/git-task-api.test.ts)、[任务基线浏览器测试](../../../../../web/test/browser/GitWorkbenchTaskBaseline.test.ts)、[任务基线与收尾 E2E](../../../../../web/test/e2e/git-workbench-task-baseline.spec.ts)。
- 跨设备审阅进度同步、乐观锁提交草稿与返工意见交回会话：
  - 服务端存储：[协同存储](../../../../../internal/workspace/workbench_collab_store.go)、[提交草稿](../../../../../internal/workspace/workbench_draft.go)、[审阅扫描](../../../../../internal/workspace/workbench_reviews_scan.go)、[审阅服务](../../../../../internal/workspace/workbench_reviews.go)；
  - 服务端路由与测试：[协同路由](../../../../../internal/api/git_collab.go)、[协同契约](../../../../../internal/contract/git_collab.go)、[服务层测试](../../../../../internal/workspace/workbench_collab_test.go)、[接口测试](../../../../../internal/api/git_collab_test.go)；
  - 前端领域模型：[草稿模型](../../../../../web/src/domains/workspace/git-commit-draft-model.ts)、[返工意见模型](../../../../../web/src/domains/workspace/git-rework-model.ts)、[会话草稿交回](../../../../../web/src/hooks/workspace/git-session-handoff.ts)；
  - 前端控制器：[审阅同步控制器](../../../../../web/src/hooks/workspace/git-review-sync.svelte.ts)、[草稿控制器](../../../../../web/src/hooks/workspace/git-commit-draft.svelte.ts)、[返工控制器](../../../../../web/src/hooks/workspace/git-rework-controller.svelte.ts)；
  - 前端组件：[草稿冲突卡片](../../../../../web/src/components/workspace/GitCommitDraftConflictCard.svelte)、[Diff 选行摘录条](../../../../../web/src/components/workspace/GitDiffSelectionBar.svelte)、[返工草稿箱弹窗](../../../../../web/src/components/workspace/GitReworkDialog.svelte)；
  - 前端测试：[协同模型单测](../../../../../web/test/git-collab.test.ts)、[跨端协同与会话交回 E2E](../../../../../web/test/e2e/git-workbench-v1-collab.spec.ts)。
- 合并、变基、遴选、还原协调恢复与三方冲突解决器：
  - 原子写与分段合并：[工作区原子写入](../../../../../internal/worktreefs/write.go)、[三方分段合并](../../../../../internal/workspace/conflict_merge3.go)；
  - 冲突探查与解决：[冲突探查](../../../../../internal/workspace/conflict_inspect.go)、[冲突保存与解决](../../../../../internal/workspace/conflict_resolve.go)；
  - 服务端分叉预览：[进行中状态探查](../../../../../internal/workspace/operation_inspect.go)、[分叉预览](../../../../../internal/workspace/divergence_preview.go)；
  - 服务端执行恢复：[分叉执行](../../../../../internal/workspace/divergence_execute.go)、[继续与中止恢复](../../../../../internal/workspace/operation_recovery.go)；
  - 服务端路由契约：[分叉与冲突路由](../../../../../internal/api/git_divergence.go)、[分叉契约](../../../../../internal/contract/git_divergence.go)、[冲突契约](../../../../../internal/contract/git_conflict.go)；
  - 服务端测试：[服务层测试](../../../../../internal/workspace/conflict_test.go)、[接口测试](../../../../../internal/api/git_conflict_test.go)；
  - 前端契约与模型：[冲突契约](../../../../../web/src/contracts/git-conflict-schemas.ts)、[冲突请求封装](../../../../../web/src/platform/git-conflict-api.ts)、[冲突领域模型](../../../../../web/src/domains/workspace/git-conflict-model.ts)；
  - 前端控制器：[分叉控制器](../../../../../web/src/hooks/workspace/git-divergence-controller.svelte.ts)、[冲突控制器](../../../../../web/src/hooks/workspace/git-conflict-controller.svelte.ts)；
  - 前端横幅与弹窗：[进行中横幅](../../../../../web/src/components/workspace/GitInProgressBanner.svelte)、[分叉预览弹窗](../../../../../web/src/components/workspace/GitDivergencePreviewDialog.svelte)、[中止确认弹窗](../../../../../web/src/components/workspace/GitAbortOperationDialog.svelte)；
  - 前端冲突解决器：[三方冲突解决器](../../../../../web/src/components/workspace/GitConflictView.svelte)、[窄屏冲突面板](../../../../../web/src/components/workspace/GitConflictMobilePane.svelte)；
  - 前端测试：[冲突模型单测](../../../../../web/test/git-conflict.test.ts)、[冲突解决器浏览器测试](../../../../../web/test/browser/GitConflictView.test.ts)、[分叉恢复与三方冲突解决 E2E](../../../../../web/test/e2e/git-workbench-conflicts.spec.ts)。
- 独立 Stash 视图、高级提交保护、引用远程维护与单文件历史及 Blame 溯源：
  - 服务端贮藏与快照对比：[贮藏读取](../../../../../internal/workspace/stash_read.go)、[贮藏创建](../../../../../internal/workspace/stash_create.go)、[贮藏应用弹出](../../../../../internal/workspace/stash_apply.go)、[贮藏丢弃](../../../../../internal/workspace/stash_drop.go)；
  - 服务端代码差异与修订对比：[贮藏差异](../../../../../internal/codeview/stash_diff.go)、[修订差异](../../../../../internal/codeview/revision_diff.go)；
  - 服务端高级提交与引用远程：[Amend 辅助](../../../../../internal/workspace/commit_amend.go)、[分支重命名与上游](../../../../../internal/workspace/branch_manage.go)、[标签创建删除](../../../../../internal/workspace/tags.go)、[远程增删改](../../../../../internal/workspace/remotes_manage.go)；
  - 服务端文件溯源与账本对账：[跨重命名历史](../../../../../internal/workspace/file_history.go)、[逐行 Blame](../../../../../internal/workspace/blame.go)、[V2 启动对账](../../../../../internal/workspace/operations_v2.go)；
  - 服务端路由与契约：[贮藏路由](../../../../../internal/api/git_stash.go)、[V2 操作路由](../../../../../internal/api/git_v2_ops.go)、[贮藏契约](../../../../../internal/contract/git_stash.go)、[V2 操作契约](../../../../../internal/contract/git_v2_ops.go)；
  - 贮藏与提交测试：[贮藏测试](../../../../../internal/workspace/stash_test.go)、[Amend 测试](../../../../../internal/workspace/commit_amend_test.go)；
  - 引用与溯源测试：[分支标签远程测试](../../../../../internal/workspace/branch_tag_remote_test.go)、[文件历史与 Blame 测试](../../../../../internal/workspace/blame_history_test.go)；
  - 服务端差异与 API 测试：[贮藏与修订快照测试](../../../../../internal/codeview/stash_revision_diff_test.go)、[接口综合测试](../../../../../internal/api/git_v2_ops_test.go)；
  - 前端契约与请求：[贮藏契约](../../../../../web/src/contracts/git-stash-schemas.ts)、[V2 契约](../../../../../web/src/contracts/git-v2-schemas.ts)、[贮藏请求](../../../../../web/src/platform/git-stash-api.ts)、[V2 请求](../../../../../web/src/platform/git-v2-api.ts)；
  - 前端模型与控制器：[V2 纯函数模型](../../../../../web/src/domains/workspace/git-v2-adapter.ts)、[贮藏控制器](../../../../../web/src/hooks/workspace/git-stash-controller.svelte.ts)、[高级提交控制器](../../../../../web/src/hooks/workspace/git-commit-options.svelte.ts)；
  - 前端引用与检视控制器：[引用控制器](../../../../../web/src/hooks/workspace/git-refs-manage-controller.svelte.ts)、[远程控制器](../../../../../web/src/hooks/workspace/git-remotes-controller.svelte.ts)、[检视器控制器](../../../../../web/src/hooks/workspace/git-file-inspector-controller.svelte.ts)；
  - 前端视图组件：[贮藏主面板](../../../../../web/src/components/workspace/GitStashPane.svelte)、[高级提交选项](../../../../../web/src/components/workspace/GitCommitAdvancedOptions.svelte)、[分支详情卡片](../../../../../web/src/components/workspace/GitBranchDetailCard.svelte)；
  - 前端远程与溯源组件：[远程管理区](../../../../../web/src/components/workspace/GitRemotesSection.svelte)、[单文件历史与 Blame 弹窗](../../../../../web/src/components/workspace/GitFileInspectorDialog.svelte)、[逐行 Blame 视图](../../../../../web/src/components/workspace/GitBlameView.svelte)；
  - 前端测试：[贮藏与 V2 模型单测](../../../../../web/test/git-stash-and-v2.test.ts)、[浏览器组件测试](../../../../../web/test/browser/GitWorkbenchStashBlame.test.ts)、[全流程 E2E](../../../../../web/test/e2e/git-workbench-stash-blame.spec.ts)。
- 任意逐行（Sub-hunk）暂存、受控丢弃更改与三阶段 Smart Checkout：
  - 服务端逐行合成：[逐行补丁合成](../../../../../internal/workspace/hunk_lines.go)、[真实索引写入](../../../../../internal/workspace/staging_write.go)、[工作区受控文件替换](../../../../../internal/worktreefs/write.go)；
  - 服务端丢弃更改：[丢弃类型](../../../../../internal/workspace/discard_types.go)、[丢弃预览](../../../../../internal/workspace/discard_preview.go)、[丢弃执行与恢复快照](../../../../../internal/workspace/discard_execute.go)；
  - 服务端 Smart Checkout 预览与类型：[类型定义](../../../../../internal/workspace/smart_checkout_types.go)、[预览](../../../../../internal/workspace/smart_checkout_preview.go)；
  - 服务端 Smart Checkout 执行与对账：[三阶段执行](../../../../../internal/workspace/smart_checkout_execute.go)、[V3 账本对账](../../../../../internal/workspace/operations_v3.go)；
  - 服务端路由与契约：[V3 操作路由](../../../../../internal/api/git_v3_ops.go)、[V3 契约](../../../../../internal/contract/git_v3_ops.go)；
  - 服务端测试：[逐行测试](../../../../../internal/workspace/hunk_lines_test.go)、[丢弃测试](../../../../../internal/workspace/discard_test.go)、[Smart Checkout 测试](../../../../../internal/workspace/smart_checkout_test.go)；
  - 前端契约请求与模型：[V3 契约](../../../../../web/src/contracts/git-v3-schemas.ts)、[V3 请求封装](../../../../../web/src/platform/git-v3-api.ts)、[逐行选区纯函数模型](../../../../../web/src/domains/workspace/git-line-selection.ts)；
  - 前端控制器：[丢弃控制器](../../../../../web/src/hooks/workspace/git-discard-controller.svelte.ts)、[Smart Checkout 控制器](../../../../../web/src/hooks/workspace/git-smart-checkout.svelte.ts)；
  - 前端组件：[逐行选择面板](../../../../../web/src/components/workspace/GitStagingLineSelector.svelte)、[丢弃预览框](../../../../../web/src/components/workspace/GitDiscardPreviewDialog.svelte)、[Smart Checkout 面板](../../../../../web/src/components/workspace/SwitchBranchSmartCheckout.svelte)；
  - 前端单测与浏览器测试：[逐行与 V3 单测](../../../../../web/test/git-v3-line-smart.test.ts)、[切分支测试](../../../../../web/test/browser/SwitchBranchDialog.test.ts)、[逐行与丢弃测试](../../../../../web/test/browser/GitWorkbenchLineSmart.test.ts)；
  - 前端端到端测试：[全流程 E2E](../../../../../web/test/e2e/git-workbench-line-smart.spec.ts)。
