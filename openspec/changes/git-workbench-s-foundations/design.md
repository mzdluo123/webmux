# Design

## Context

动机与阶段边界见 [proposal.md](proposal.md)，行为契约见 `specs/` 下四个新 capability。结合当前代码库结构，S 阶段需在以下工程约束内完成底座落地与界面核对：

- **`internal/codeview` 的会话强耦合现状**：
  - `internal/codeview/service.go` 当前以 `rootKey{session, root string}` 缓存绑定，每次 `bind()` 先调 `s.manager.Get(id)`，后台 `sweep()` 每秒轮询 `s.manager.Get` 并在会话退出后立即清空该会话的根缓存与全部 `snapshot`。
  - `sourceFile()` 仅接受 `"worktree"`、`"index"`、`"head"` 三种来源，尚不支持仓库级无工作树绑定，也不支持按两个提交 OID 生成双侧历史快照。
- **`internal/workspace` 的锁粒度与运行器差异**：
  - `repositoryState.gate` 是单仓库互斥通道（容量 1），用于串行化分支切换、快进、推送复查与删除；若将单工作树的慢速 `pre-commit` 钩子放进仓库门控，会阻塞同一仓库下其他并行工作树的操作与快照刷新。
  - 只读运行器 `ReadGit`（`readonly.go`）注入 `GIT_OPTIONAL_LOCKS=0` 并在 `diff`/`status` 时临时置空全部 `filter.*.(clean|smudge|process)` 与 `required=false`；这套只读安全覆盖绝不能用于 `Stage`/`Unstage` 或 `git commit`，否则会破坏仓库配置的换行归一、LFS 指针转换与提交钩子。
- **后端分层与单文件规模约束**：
  - `internal/archtest` 规定 `codeview` 与 `workspace` 同属 `service` 层，且当前 `codeview` 已导入 `workspace`；因此反向依赖（`workspace` 导入 `codeview`）被禁止以避免循环依赖。
  - 全仓源码文件受 `<500` 行预算约束，新能力需按职责拆分为内聚子文件。
- **前端架构守卫与 Storybook 矩阵约束**：
  - `pnpm --dir web check:arch` 同时执行 `@shadcn/lint`（禁止业务处写原始颜色、内联尺寸或覆盖组件外观）、`dependency-cruiser`（无环与分层方向）、`jscpd`（零新增克隆）、`knip`（零不可达文件与未使用导出）和 `test/architecture.test.ts`（圆角刻度锁死在 `rounded-sm`/`md`/`lg`/`xl`/`full` 且 `<500` 行）。
  - `pnpm --dir web test:storybook` 在阻断 `/api/` 网络请求的隔离 Chromium 中，按 6 种视口/字号组合 × 明暗双主题逐项校验几何越界与弹层按钮可达性。

## Goals / Non-Goals

**Goals:**

- 在不改动任何线上 HTTP/WebSocket 契约的前提下，让 `internal/codeview` 与 `internal/workspace` 具备零会话可用的仓库/工作树上下文、流式分页拓扑、真实 `index` 逐块/整文件映射及并发安全的提交结果识别能力。
- 建立两级协调机制（仓库门控 + 工作树门控），使同仓库多工作树并行互不阻塞，同时对外部 CLI 与钩子保持诚实可验证的结果分类。
- 在现有设计系统与架构守卫体系内正式落地 IntelliJ 风格双主题令牌、受控组件变体及可复用的桌面/手机 Git 工作台组件骨架，通过 Storybook 自动化矩阵完成视觉与交互评审闭环。

**Non-Goals:**

- 不在 S 阶段注册 `/git/{repositoryId}` 生产路由，不修改 `web/src/hooks/shared/route.ts`、`App.svelte`、`AppShell.svelte` 或侧栏常驻菜单（属 V1a）。
- 不在 S 阶段新增 REST/WebSocket 控制器路由或改动 `internal/api/testdata/wire` 快照（属 V1a）。
- 不在 S 阶段引入跨服务重启的操作记录、提交草稿、任务基线或审阅标记的 `statefile` 描述符与 `persistguard` 登记（属 V1b）。
- 不在 S 阶段实现任意逐行（sub-hunk）暂存、三方冲突解决器、`Merge`/`Rebase`/`Cherry-pick`/`Stash` 写操作（属 V2/V3）。

## Decisions

### 1. `codeview` 泛化作用域键与单向 `TargetResolver` 协作接口

**方案**：保持 `internal/codeview -> internal/workspace` 的既有单向导入关系，在 `internal/codeview` 内将会话专属键泛化为多作用域目标键：

- **作用域键泛化**：将内部 `rootKey` 升级为 `scopeKey{kind scopeKind, owner, subroot string}`，按三类 `scopeKind` 隔离状态：
  - `scopeSession`：完全兼容现有 `Context(ctx, sessionID, root)` 等全部会话级签名；
  - `scopeRepository`：`owner = repositoryID`，仅绑定经过校验的 `commonDir`；
  - `scopeWorktree`：`owner = repositoryID + "\x00" + worktreeID`，绑定具体工作树目录。
- **单向只读定位接口**：在 `codeview` 定义窄接口 `TargetResolver`（由 `*workspace.Service` 实现并通过 `codeview.UseWorkspace(resolver)` 在 `cmd/webmux` 及测试中按需注入）：
  - `LocateRepository(ctx, repositoryID)`：返回规范化 `commonDir` 与可用状态；
  - `LocateWorktree(ctx, repositoryID, worktreeID)`：返回经仓库清单核实的工作树根路径、类型（`main`/`linked`/`bare`）与 `commonDir`。
- **仓库级只读与历史对象比较**：仓库级上下文（或 `bare` 工作树）核实 `commonDir` 的 `os.SameFile` 身份，标记 `Browse: false, Diff: false, Write: false` 并写明只读历史原因：
  - 开放基于完整提交 OID 的 `CommitFile` 与 `CommitDiff`；
  - 底层经 `workspace.ReadGit` 调用 `rev-parse --verify --end-of-options <oid>^{commit}`、`diff-tree -r --raw -z --find-renames` 与 `cat-file blob`。
- **生命周期与缓存回收**：工作树级上下文复用现有 `O_NOFOLLOW` 逐段目录打开、`.git` 边界阻断、文件树与修改扫描；`sweep()` 对 `scopeSession` 继续保留会话存活联动清理，对 `scopeRepository` / `scopeWorktree` 按空闲 TTL（`Lifetime = 10m`）回收根绑定与快照，并共享 `CacheBytes = 64 MiB` 的 LRU 内存上限。

**备选方案与取舍**：

- *新建第三个 `internal/gitcontext` 服务包同时导入 `codeview` 和 `workspace`*：会导致快照缓存池与 `O_NOFOLLOW` 文件读取逻辑分裂成两份，且增加 `internal/archtest` 包清单复杂度。直接在 `codeview` 内泛化作用域键既对全部现有会话级测试零侵入，又复用了同一套 64 MiB 快照内存治理。

### 2. 无常驻索引的分页历史游标与双通道真实 parent 拓扑模型

**方案**：在 `internal/workspace` 新增独立历史文件（如 `history.go`、`history_graph.go`），使用系统 Git 流式输出按页构建历史与车道拓扑：

- **分页锚点与游标**：
  - 查询输入包含范围（`all` / `branch` / `to-push` / `incoming`）、可选显式分支引用（在服务端经 `refs/heads/` 或 `refs/remotes/` 精确解析为起始 OID 集合，绝不拼接原始表达式）与筛选条件（`query`、`author`、`since`、`until`、`path`）；
  - 分页控制使用 `limit`（默认 100，最大 200）及不透明游标 `cursor`（编码起始引用 OID 摘要、筛选摘要与已跳过条数 `offset`）；
  - 若翻页时引用已移动导致锚点摘要变化，服务端基于首次请求锁定的不可变起始提交 OID 集合继续一致翻页，并附带 `refsChanged: true` 提示前端存在新提交可刷新。
- **真实 parent 与筛选省略识别**：
  - 未启用筛选时，单次执行 `ReadGit` 调用 `git log --topo-order -z --no-show-signature` 读取完整 OID、真实 `%P`（父提交 OID 列表）、作者/提交者元组与提交标题。
  - 启用路径、作者、时间或关键字筛选时，原始提交头中的 `%P` 保持真实直接父提交不变；同时在同页窗口内辅以轻量 `--parents` 映射比对：若可见提交 `C` 的简化祖先 `A` 并非 `C` 的真实直接 parent，则拓扑边显式标记为 `indirect`（筛选省略延续边），绝不渲染为 `direct` 实线父子边。
- **流式车道分配器（Lane Layout）**：
  - 服务端或前端纯函数按统一确定性规则为每行分配 `column` 与进出边集合 `(fromLane, toLane, targetOID, kind: direct | indirect | boundary)`；
  - 当某行的真实父提交 `P` 未落入当前页已返回集合时，保留该活跃车道指向页底 `boundary` 延续节点并附带 `P` 的完整 OID，供下一页无缝衔接。

**备选方案与取舍**：

- *在服务端维护全仓库内存提交图缓存*：首次打开大型仓库会带来明显冷启动开销与常驻内存负担，且需处理 `gc`/rebase 后的缓存失效；`git log --topo-order` 配合 Git 原生 `commit-graph` 在按页 100 条规模下耗时仅毫秒级，完全满足首期轻量部署定位。

### 3. 基于冻结快照确定性重组的真实 `index` 代码块映射引擎

**方案**：在 `internal/workspace` 新增暂存子模块（如 `staging.go`、`hunk.go`），将整文件操作与普通文本逐块操作统一建立在内容身份校验之上：

- **统一的内容身份凭证**：
  - 未暂存方向（`Stage`）：绑定 `{path, oldIndexOID, oldIndexMode, worktreeContentSHA256, worktreeMode}`（未跟踪新文件的 `oldIndexMode = "000000"`）；
  - 已暂存方向（`Unstage`）：绑定 `{path, headOID, headMode, indexOID, indexMode}`（未诞生 `HEAD` 或新添加文件的 `headMode = "000000"`）。
- **降级判定前置**：
  - 检测索引存在 stage `1..3` 冲突条目、非普通文件模式（非 `100644`/`100755`）、含 NUL 二进制字节、超过文本上限（2 MiB / 20000 行），或 `git check-attr -z filter working-tree-encoding` 非 `unset`/`unspecified` 的路径；
  - 上述路径一律标记 `hunkStageAvailable: false` 及原因，仅开放经身份核对的整文件 `Stage`/`Unstage`。
- **普通文本逐块重组与换行/无尾换行保真**：
  - **差异切分与规范化对齐**：
    - 服务端按保留原始行尾（`\n` 或 `\r\n`）及末行换行存在性的切片切分确定性差异块，为每块生成由位置与行内容摘要构成的稳定 `hunkID`；
    - 若仓库启用了 `core.autocrlf` 或 `eol`/`text` 属性，未暂存比较前先经无副作用的 `git hash-object --stdin --path=<path>` 探明规范化后的索引侧表示（或在同一规范化基准上计算差异块），避免全文件因行尾转换被误判为整篇改动。
  - **逐块 `Stage` 写入**：
    - 在工作树门控下重验索引条目与工作区文件摘要未变后，在内存中仅将所选 `hunkID` 对应的行替换应用到基础字节流；
    - 随后经 `git hash-object -w --path=<path> --stdin` 写入新 blob 对象，再以 `git update-index --add --cacheinfo <mode>,<newBlobOID>,<path>` 原子更新该路径的真实 `index`。
  - **逐块与整文件 `Unstage` 写入**：
    - 逐块取消暂存时，在 `HEAD` blob 与当前 `index` blob（均处于仓库规范格式）之间反向撤销所选 `hunkID`，经 `git hash-object -w --stdin` 生成目标 blob 后调用 `git update-index --cacheinfo` 写回索引；
    - 整文件 `Unstage` 在 `HEAD` 无该文件或仓库处于未诞生 `HEAD` 时统一调用 `git update-index --force-remove -- <path>`（避免 `git reset HEAD` 在空仓库报错），永不触碰工作区磁盘文件。

**备选方案与取舍**：

- *由前端拼接 unified diff 文本传给后端 `git apply --cached`*：
  - 扩大了客户端注入任意补丁内容的攻击面，且在无末尾换行、相邻块部分选中行号偏移与 `CRLF` 混合行尾时极易因上下文计数或空白规则报错；
  - 服务端基于已校验快照字节精确重组并通过 `hash-object` + `update-index` 写入，既杜绝任意补丁注入，又天然支持多块任意子集一次暂存。

### 4. 两级门控协调与 `GIT_REFLOG_ACTION` 提交归属核对协议

**方案**：在 `internal/workspace` 引入工作树级门控与四步提交核对状态机（如 `commit.go`）：

```text
PreviewCommit (read-only)
  └─ Snapshot { worktreeID, headOID|"unborn", indexStageDigest, stagedFiles, identity, blockers }
        │
        ▼ (confirmationId, 5m TTL)
ExecuteCommit (under worktreeWriteGate[canonicalWorktreePath])
  ├─ 1. Recheck worktree identity + HEAD + indexStageDigest + compute expectedTreeOID
  ├─ 2. Run normal `git -c core.logAllRefUpdates=always commit -F -`
  │     with env `GIT_REFLOG_ACTION=webmux-commit:<operationId>` (respects hooks/signing/filters)
  ├─ 3. Inspect exit & lock errors (`.git/index.lock` -> report busy, NEVER delete lock)
  └─ 4. Scan `HEAD` reflog (`%H%x00%gs`) for `webmux-commit:<operationId>`
        ├─ Found <actualOID>: compare actual {parents, tree} vs expected {parents, expectedTreeOID}
        │     ├─ Equal      -> Outcome: "matched"
        │     └─ Different  -> Outcome: "needs_review" (e.g. pre-commit restaged / post-commit amended)
        └─ Missing / ambiguous after exit or timeout -> Outcome: "uncertain"
```

- **两级协调**：新增按规范化工作树根路径索引的 `worktreeGate`（每个工作树容量 1 的互斥通道），`Stage`、`Unstage`、`PreviewCommit`、`ExecuteCommit` 仅获取目标工作树门控；只有在解析仓库注册信息或需要与分支切换/删除互斥的瞬间与仓库协调器交互，绝不在慢速提交钩子或远程网络期间阻塞其他工作树。
- **零副作用计算 `expectedTreeOID`**：执行前复核通过后，将当前 `.git/index` 复制到服务端受控临时文件并以 `GIT_INDEX_FILE=<tmp>` 运行 `git write-tree` 获取预期树对象 OID（随后立即删除临时副本），既不改动真实 `.git/index` 的时间戳，又能得到与 `git commit` 完全一致的基准 `tree` OID。
- **防外部并发提交误认（A06）与防钩子静默篡改（A05）**：
  - 通过 `-c core.logAllRefUpdates=always` 与唯一环境变量 `GIT_REFLOG_ACTION=webmux-commit:<operationId>`，Git 在更新引用锁内原子写入携带该标记的 reflog 记录；
  - 即使 `git commit` 退出后数毫秒内外部 CLI 立即提交了 `C2` 推进 `HEAD`，扫描最近 reflog 仍能精确锁定带 `webmux-commit:<operationId>` 的 `C1`；
  - 若 `post-commit` 钩子在同环境内执行了 `--amend`，最新匹配条目即为修正后的提交，且 `tree`/消息对比会如实触发 `needs_review`。

**备选方案与取舍**：

- *解析 `git commit` 标准输出首行的短哈希*：受钩子输出干扰、短哈希存在碰撞歧义，且进程超时中断时可能已落盘提交却未刷出完整 stdout；强制启用本次调用的 reflog 归属标记由 Git 引用事务锁保证原子性，拿到的是完整对象 ID。

### 5. 前端 Git 工作台语义令牌、受控变体与 Storybook 响应式架构

**方案**：严格遵循 `docs/design-system.md` 与 `docs/frontend-architecture.md` 的三层令牌与组件封装规范：

- **令牌层（`web/src/design-tokens.css`）**：在 `:root` 与 `.dark` 定义 Git 工作台语义变量并在 `@theme inline` 映射为 Tailwind 主题色：
  - 表面与分隔：`--git-surface`（`#ffffff` / `#1e1f22`）、`--git-panel`（`#f7f8fa` / `#2b2d30`）、`--git-border`（`#d9dce3` / `#393b40`）；
  - 选中与主动作：`--git-selected`（`#e8efff` / `#2e436e`）、`--git-selected-foreground`（`#25272d` / `#dfe1e5`）、`--git-primary`（`#2d65c7` / `#356bc1`）、`--git-primary-foreground`（`#ffffff`）。
- **基础组件层（`web/src/lib/components/ui/`）**：
  - 在 `Button` 的 `tv()` 新增 `variant="git-primary"`（消费 `bg-git-primary text-git-primary-foreground`，保留统一焦点环与禁用对比度）；
  - 在 `ListButton` 的 `tv()` 新增 `variant="git-row"` 与 `variant="git-row-action"`（紧凑行高，`aria-selected="true"` 或 `aria-current="true"` 时应用 `bg-git-selected text-git-selected-foreground` 并附左侧或外框焦点指示，圆角固定为 `rounded-sm` 4px；行尾动作在桌面悬停/聚焦覆盖、触屏常驻预留槽位）；
  - 分栏调节直接复用 `$lib/components/ui/resizable`（`PaneGroup`、`Pane`、`ResizableHandle`），运行时面板比例与拓扑列高一律通过 `style:--*` 自定义属性驱动，并在 `web/eslint.config.mjs` 的 `Pane`/`ResizableHandle` 契约中显式登记新增布局钩子类名。
- **业务组件拆分与可达性**：
  - 纯 TypeScript 模型：`web/src/contracts/git-workbench-schemas.ts`（Zod 契约与演示/视图状态类型）、`web/src/domains/workspace/git-workbench-state.ts` 与 `web/src/domains/workspace/git-graph-layout.ts`（车道几何与延续边计算）；
  - 视图组件（均控制在 `<500` 行内）：`GitWorkbenchShell.svelte`（顶栏、窄模式栏、响应式单栏/多栏容器与底部状态栏）、`GitLogPane.svelte`、`GitCommitPane.svelte`、`GitCommitPreviewDialog.svelte`、`GitBranchesPane.svelte`、`GitPushDialog.svelte`；
  - **Storybook 与守卫闭环**：
    - 在 `web/stories/GitWorkbench.stories.ts`（及 `Overflow.stories.ts` / `web/test/storybook/cases.ts`）导出交互故事，覆盖桌面三栏 `Log`、合并拓扑与筛选延续、`Commit` 部分暂存与完整 `index` 预览、`Branches` 占用引导、`Push` 宽对话框/手机全屏页及异常空态；
    - 同步在 `DesignSystem.svelte` 补充 Git 令牌与变体展示，使 `knip` 可达性、`check:arch` 与 `test:storybook` 全链路闭环。

## Risks / Trade-offs

- **[Risk] 临时索引副本计算 `expectedTreeOID` 在超大仓库索引（数十兆字节）下的 I/O 开销** → 仅在用户点击确认执行提交时复制一次单个 `.git/index` 文件（常规仓库在数毫秒内完成），且设置受控超时并确保 `defer os.Remove` 清理临时文件。
- **[Risk] 用户仓库配置了极慢的 `pre-commit` 测试钩子导致请求超时** → 提交执行设置独立的有界超时并通过进程组 `Setpgid` 管理生命周期；超时终止后仍执行一次 reflog 归属扫描，若钩子未完成提交则明确报告超时失败，若恰好已完成引用更新则诚实归类为可核对结果或 `uncertain`。
- **[Risk] S 阶段组件尚未挂载到 `App.svelte` 生产路由可能触发 `knip` 未使用告警** → `web/knip.json` 将 `test/**/*.ts` 及 Storybook 入口纳入项目图；所有新增领域函数与组件均只导出被组件测试与 Storybook 评审故事真实消费的符号，不预留悬空死代码。
- **[Risk] 手机 `320×568` 及 `200%` 根字号下多徽标提交行或长分支名发生横向裁剪** → 窄屏下隐藏次要列（作者、短 OID、绝对时间），分支/路径使用带 `title` 与可访问全称的截断或换行徽标，并强制纳入 `web/test/storybook/cases.ts` 自动防溢出回归。

## Migration Plan

1. 先落地 `internal/codeview` 的多作用域上下文与历史对象双版本比较能力，运行 `go test -race ./internal/codeview ./internal/api` 确认现有会话代码区零回归。
2. 落地 `internal/workspace` 的分页历史与真实 parent 拓扑模型，以隔离临时 Git 仓库（含空仓库、多分支 merge、路径筛选隐藏与分页边界）完成 `-race` 单测验收。
3. 落地 `internal/workspace` 的工作树协调锁、快照绑定文件/代码块 `Stage`/`Unstage` 及 `PreviewCommit`/`ExecuteCommit` 结果识别引擎，用真实系统 `git` 覆盖 `CRLF`、无尾换行、unborn `HEAD`、`.git/index.lock`、`pre-commit` 重暂存、`post-commit` 修正与外部并发 `git commit` 竞态测试。
4. 扩展前端 `design-tokens.css` 与 `ui/` 组件变体，更新 `docs/design-system.md` 与 `docs/frontend-architecture.md`，随后构建 Git 工作台响应式组件与 Storybook 评审场景，通过 `make check` 与 `make test` 全量门禁。因 S 阶段不改持久化文件与对外线上路由，随时回退无任何状态迁移负担。
