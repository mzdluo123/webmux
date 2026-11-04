# Design

## Context

能力都已存在，只是分散在三个接口里，由前端依次调用：

- `POST /api/repositories/{id}/worktrees`（`workspace.CreateWorktree`）：`{path, mode, branch, base}`，由服务端用 `git check-ref-format` 校验分支；失败不回滚。分支建议来自 `GET …/branches`（`DefaultParent`、`DefaultBase`、`Bases`）和前端的 `branchName.ts` `suggestWorktreeBranch`。
- `POST /api/sessions`（`workspace.CreateSession` → `hybrid.Service.Create`）：`configId` 加 `startSide: "acp"`，返回前已完成 ACP `initialize` 和 `session/new`。
- `POST /api/sessions/{id}/chat/prompts`（`hybrid.Service.Prompt`）：按 `clientRequestId` 幂等。

`NewSessionDialog.svelte` 现在就是在浏览器里串联前两步。agent 的配置项（`ChatState.config`）只有会话运行后才知道；`hybrid.Record.Config` 持久保存了每个会话最后使用的配置值，但没有保存配置项清单（名称、类别、可选值）。仓库登记记录（`registration`）只有 `ID/CommonDir/AnchorPath/DisplayName`。

## Goals / Non-Goals

**Goals:**

- 一个请求在服务端完成 worktree、会话、配置和首条提示，浏览器断开不影响结果。
- 启动前就能选择配置，复用聊天输入框现有的配置控件。
- 新增持久状态最少，遵守 `persistence-governance`。

**Non-Goals:**

- 不支持直接目录或已有 worktree 作为快速开始目标；这种情况继续使用新建会话弹窗。
- 首条消息不带附件。
- 不做语音输入、云端环境选择（参考截图中的“默认环境”）。
- 失败时不自动删除已创建的 worktree。

## Decisions

### 1. 服务端编排，单一接口 `POST /api/quick-starts`

接口形状：

- 请求 `QuickStartRequest{clientRequestId, repositoryId, configId, base, branch, text, config: map[string]string, permissionMode}`。
- 成功响应 `QuickStartResult{sessionId, worktree, configWarnings[]}`。
- 失败时返回结构化错误 `{step: "worktree"|"session"|"config"|"prompt", message, worktree?, sessionId?}`：状态码由 `internal/api/errors.go` 映射，失败步骤和已创建资源放在错误体中。

编排放在 `internal/workspace`（新文件 `quickstart.go`）：它已经持有 worktree 创建和 `CreateSession`，并已依赖 `internal/hybrid`（`sessions.go`、`remove.go`），设置配置和首条提示直接调用 `hybrid.Service`，不引入新的服务包或层级例外。

处理器使用 `context.WithoutCancel(r.Context())` 加独立超时（与会话创建一致），客户端断开不会中断编排。

- 备选：继续在前端串联三步。缺点是关页面会停在中间（有 worktree 无会话，或有会话无首条消息），无法满足“浏览器断开不影响”。
- 备选：给 `CreateSessionRequest` 加 `initialPrompt`。会让普通会话创建接口也承担 worktree 和配置，职责混杂。

### 2. 分支名在前端生成，路径在服务端推导

- 前端加载 `GET …/branches`：`DefaultBase` 作为基准分支默认值，`Bases` 作为可选项；分支名用现有 `suggestWorktreeBranch` 生成后放进请求。这样分支命名只有一个实现。
- 服务端用 `DefaultParent/<branch>` 推导路径，沿用 `CreateWorktree` 的分支校验。
- 分支名或路径冲突时返回 `step: "worktree"` 的冲突错误；前端重新生成分支名，用新请求 ID 再发送。

### 3. 幂等：进程内按请求 ID 去重

`clientRequestId` 映射到一个 `singleflight` 风格的条目：执行中的重复请求等待同一结果；完成后的结果保留 10 分钟，然后清除。不持久化，服务重启本来就会结束会话。首条提示复用同一个请求 ID 作为 `Prompt` 的 `ClientRequestID`。

### 4. 配置项清单按启动配置持久记忆

- 记录：`hybrid` 在每次 ACP 会话报告配置项时（`session/new` 及后续更新），按 `configId` 记下最近的配置项清单（ID、名称、类别、类型、可选值）和当时的值。
- 保存：新的状态文件 `agent-options.json`，在 `internal/statefile/catalog.go` 登记；删除启动配置时一并清除。
- 读取：`GET /api/launch-configs` 的每个配置新增字段 `chatOptions`，结构直接复用 `ChatConfig`，因此 `ChatConfig.svelte` 可以原样渲染。

启动后由服务端按 `config` 逐项调用现有的设置配置路径，再发首条提示。agent 不接受的值记为 `configWarnings`，并作为聊天 notice 写入时间线；不阻断发送。

- 备选：选择 agent 时预启动一个 ACP 进程探测配置。代价是每次切换都起进程，而且 cwd 还未确定。
- 备选：只存在内存里。服务重启后首个会话之前都没有配置可选，体验差，且成本只是一份小 JSON。

### 5. 初始化指令存在仓库登记中

- 存储：`registration` 增加 `InitInstruction string`（`omitempty`）；`registry.Version` 不变，旧文件缺字段即为空。
- 接口：`RegisterRepositoryRequest` 增加可选 `initInstruction`；新增 `PATCH /api/repositories/{id}` `{initInstruction}` 用于修改或清空。
- 校验：长度上限 4000 个字符，由服务端校验。
- 重复登记：保留已有指令，响应照常返回 200 和已有条目，前端据此提示。

拼接在服务端完成：`text = instruction + "\n\n" + userText`（指令为空时只用用户文字）。这样首条提示与仓库当前保存的指令一致，前端提示只用于展示。

### 6. 前端页面

- 路由 `/new`（`route.ts` 新增 `quick-start`），`pages/QuickStart.svelte` 懒加载。首页 `WorkspaceToolbar` 增加“新聊天”主要入口，原“新建会话”保留在旁边。
- 布局参考 Claude Code 网页版：空白主区域，底部固定输入卡片，卡片下方一行放选择控件。
  - 仓库、基准分支、agent 使用显示当前值文字的下拉：给 `ChatSelect` 增加 `labeled` 用法，不另写一套。
  - agent 配置直接用 `ChatConfig`，权限用 `ChatPermissionMode`；桌面端 agent 与配置靠右，与仓库、分支分列两侧。
  - 窄屏与聊天输入框一致：agent 和 `ChatSessionOptions` 收进卡片底部，下方一行只留仓库和分支，避免配置被截断。
- 初始化指令提示放在输入卡片顶部的一行（与命令菜单、附件同一位置），点击用 Popover 显示全文。
- 上次的仓库和 agent 存在 `localStorage`，用 Zod 校验后读取。
- 草稿：输入文字用现有 `terminal-draft` 机制，以固定键 `quick-start` 保存，刷新不丢失。

## Risks / Trade-offs

- [编排耗时较长（agent 启动可能十几秒），请求一直挂着] → 处理器不依赖客户端连接；前端超时后用同一请求 ID 重试即可拿到结果。
- [配置项清单过时（agent 升级后选项变化）] → 启动后以 agent 实际报告为准，失效值只产生 warning；下一次会话会刷新清单。
- [失败遗留 worktree] → 错误体给出 worktree，并提供“在此 worktree 新建会话”入口；删除仍走现有的删除确认流程。
- [初始化指令过长挤占 agent 上下文] → 4000 字符上限，输入框上方提示让用户知道会附带。

## Migration Plan

新增字段都是可选的：旧的 `repositories.json` 直接兼容，`agent-options.json` 不存在时为空。回滚到旧版本时，旧版会忽略未知的 `initInstruction` 字段；如果旧版会在保存时丢掉该字段，指令会丢失，但不影响其他功能。
