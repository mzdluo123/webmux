# Proposal

## Why

WebMux 规划新增以 IntelliJ Git 为参照的独立「Git 工作台」，服务持续运行的远程终端、CLI agent、多工作树并行与跨设备接续。在直接进入路由接线与端到端页面交付（V1a/V1b）之前，当前代码库在五个核心技术点与跨端界面组织上存在尚未固化边界的前置风险：

- `internal/codeview` 的根绑定、内容快照与定时回收完全耦合活动 `sessionId`，无法在零会话场景下安全承载已登记仓库与显式工作树上下文；
- `internal/workspace` 仅有面向起点选择的 `RecentCommits`（固定上限 50 条、无拓扑父边），尚未定义分页历史、筛选隐藏与分页截断处的真实 parent 拓扑延续语义；
- 现有修改与 Diff 只读展示 `unstaged`/`staged`/`untracked`，缺乏绑定服务端快照内容身份的文件级与普通文本代码块（hunk）级真实 `index` 暂存/取消暂存映射及降级边界；
- 外部 CLI、运行中 agent 与 `pre-commit`/`post-commit` hooks 可能在提交前后改写 `index` 或并发推进 `HEAD`，仅读取最新 `HEAD` 会将外部提交误判为本次提交或漏报 hook 篡改；
- IntelliJ 风格的多栏紧凑布局、蓝色选中/主操作语义令牌及手机单栏步进流程尚未与 WebMux 现有设计系统及 `pnpm --dir web check:arch` 守卫对齐，也缺少桌面与手机视口的可评审实物。

因此需要先完成 **S 阶段（技术与 UI 验证）**：在后端落地可复用的生产级 Git 上下文、分页拓扑、真实 `index` 代码块操作与并发提交结果识别底座，并在前端通过扩展受控设计令牌、组件 variant 与 Storybook 交互场景完成桌面/手机关键界面核对，暂不开放 `/git` 正式用户路由入口（留待 V1a 串接）。

## What Changes

- **解耦独立于会话的仓库与工作树代码上下文**（`internal/codeview`、`internal/workspace`）：
  - 在保留现有会话级 `/api/sessions/{id}/code/...` 行为完全兼容的前提下，为 `internal/codeview` 增加基于已登记 `repositoryId` 与可选 `worktreeId` 的独立上下文生命周期；
  - 服务端核实工作树根目录、Git 目录与 `commonDir` 文件系统身份，支持无工作树选择或 `bare` 仓库下的仓库级只读上下文，并按上下文隔离内容快照、修改游标与 TTL/LRU 回收；
  - 新增经服务端对象身份校验（完整十六进制提交 OID）的历史版本文件读取与双提交比较能力，拒绝客户端传入任意修订表达式。
- **落地按需分页的提交历史与真实拓扑图模型**（`internal/workspace`）：
  - 实现仓库/工作树范围的分页历史查询、提交完整详情（作者/提交者区分、父提交、refs、变更文件状态与重命名）及合并提交指定父节点比较；
  - 定义基于真实 parent OID 的车道（lane）拓扑与延续标识：分页截断或筛选隐藏节点时显式输出延续边，绝不将非直接父子提交连成虚假父子边；
  - 支持全部历史、当前分支、待推送、远端新增等范围查询，在缺少上游或未诞生 `HEAD` 时返回明确不可用原因而非伪造零条结果，且全程保持无副作用只读执行。
- **实现绑定快照身份的真实 `index` 暂存与并发提交结果识别**（`internal/workspace`、`internal/codeview`）：
  - 实现文件级与普通文本代码块级 `Stage` / `Unstage`，绑定服务端比较两端的对象 OID、工作区文件摘要与文件模式，覆盖未诞生 `HEAD` 首次暂存、增删文件、`CRLF` 与无末尾换行，并对 `filter`、`working-tree-encoding`、二进制、重命名、子模块与冲突条目执行明确降级；
  - 实现完整真实 `index` 提交预览与执行期复核：预览完整展示包含终端已有暂存项在内的全部 `HEAD -> index` 变更、身份配置与阻塞项；
  - 执行 `git commit` 时尊重用户身份、签名与全部 hooks，严禁套用只读过滤覆盖或隐式执行 `git add -A`；对外部 `.git/index.lock` 报告仓库忙且绝不删除锁文件；
  - 通过工作树级协调、`GIT_REFLOG_ACTION` 操作归属标记与提交后 `{oid, parents, tree}` 比对识别真实提交结果：
    - 在 hooks 改写暂存区或外部 CLI 紧随其后并发提交的场景下，精确区分 `matched`（已按预览提交）、`needs_review`（结果有差异需复核）与 `uncertain`（结果待确认）；
    - 本阶段聚焦内存与执行期契约，跨重启 `statefile` 持久化留待 V1b。
- **扩展 Git 工作台专属视觉令牌并交付桌面/手机 Storybook 评审骨架**（`web/src/`、`web/stories/`）：
  - 在 `web/src/design-tokens.css` 新增 Git 工作台作用域的双主题语义令牌（IntelliJ New UI 风格中性面板表面、1px 细分隔线、蓝色选中背景与蓝色主要操作色），并在 `web/src/lib/components/ui/` 相关组件 `tv()` 中扩展受控 variant（圆角保持 `rounded-sm` 4px 行与 `rounded-md` 6px 控件守卫兼容）；
  - 构建可复用的 Git 工作台界面骨架与核心视图组件（顶栏上下文与模式栏、`Log` 三栏拓扑与详情、`Commit` 分组暂存与完整 `index` 预览、`Branches` 占用与动作、`Push` 宽对话框及底部操作状态栏），支持桌面（`≥1280px`）多栏可调分隔与平板/手机（`320–1023px`）单栏步进适配；
  - 在 Storybook 中提供完整演示与异常态评审故事并接入 `web/test/storybook/cases.ts` 防溢出与无障碍矩阵检查，不在 S 阶段注册 `/git` 生产路由或修改全局侧栏常驻入口。

## Capabilities

### New Capabilities

- `git-workbench-context`: 独立于会话的仓库与工作树浏览/比较上下文，覆盖身份核实、仓库级只读与 `bare` 边界、按明确提交 OID 的历史文件与双版本比较，以及与会话解耦的快照和游标隔离回收。
- `git-history-graph`: 按需分页的提交历史、真实 parent OID 拓扑车道与分页/筛选延续边、合并提交父节点比较、快捷范围可用性判定及无副作用只读约束。
- `git-index-staging-and-commit`: 基于真实 Git `index` 的文件级与普通文本代码块级暂存/取消暂存、内容身份校验与特殊文件降级，以及完整 `index` 提交预览、hooks 兼容、锁冲突边界与外部并发下的提交归属及结果分类。
- `git-workbench-ui-foundation`: Git 工作台的 IntelliJ 风格双主题语义令牌与设计系统组件 variant，以及桌面多栏与平板/手机单栏的 `Log`、`Commit`、`Branches`、`Push` 预览交互布局骨架和 Storybook 核对规范。

### Modified Capabilities

无。

## Impact

- **后端代码（目标实现影响面）**：
  - `internal/codeview`：抽象底层绑定键与宿主接口，保留现有会话入口并新增仓库/工作树上下文入口及历史提交快照比较；
  - `internal/workspace`：扩展分页历史与拓扑计算、工作树级协调门控、真实 `index` 暂存/代码块应用、提交预览与基于 reflog 归属标记的提交结果核对（均留在既有 `service` 层包内，不破坏 `internal/archtest` R0–R11）；
  - `cmd/webmux` 及相关测试夹具：按需为 `codeview.Service` 注入 `workspace` 只读定位协作接口；S 阶段不新增对外 REST/WebSocket 路由、TOML 配置项或 `statefile` 持久化描述符，`internal/api/testdata/wire` 与 `internal/persistguard` 零破坏。
- **前端代码与文档（目标实现影响面）**：
  - `web/src/design-tokens.css`、`web/src/lib/components/ui/`（新增 Git 工作台令牌与受控 variant）、`web/eslint.config.mjs`（如需为新增分栏面板登记 `Pane`/`ResizableHandle` 布局钩子）；
  - `web/src/` 与 `web/src/components/` 新增可被 V1a 直接复用的 Git 工作台视图模型与布局组件，`web/stories/` 与 `web/test/storybook/cases.ts` 新增桌面/手机双主题评审故事（遵循 `knip` 可达性、`arch:dupes` 与单文件 `<500` 行预算）；
  - 实施阶段同步更新 `docs/design-system.md` 与 `docs/frontend-architecture.md` 的令牌/variant 登记表；因 S 阶段尚未向最终用户开放 `/git` 入口，用户故事将在 V1a 开放能力时随行为一并登记。
