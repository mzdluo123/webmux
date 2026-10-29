## Context

- **会话是唯一执行单元。** `session.Manager.Create` 以 argv 启动 PTY 会话，`Session.Done()` 在退出时关闭，`Info.ExitCode` 给出退出码；会话与浏览器连接解耦，服务重启即结束。
- **混合会话已有结构化 agent 通道。** `hybrid.Service.Create` 可从聊天端（`SideACP`）启动，`Prompt` 返回 `TurnID`，`Subscribe` 的事件流以 `turn.end`（`StopReason`：ACP 原因、`error`、`auth_required`）结束回合；权限可设为 `auto`。目前 `agentprofile` 只内置 OpenCode 与 omp。
- **程序状态信号。** `OSC 7501` 根记录（`working` / `idle` / `done` / `error` / `blocked`）已由 Claude Code、Codex CLI、omp 实测发出（见 `add-program-status-signals` 基线），`Session.Submit` 提供带 ID、去重和写期限的提交。
- **启动配置**按名称唯一，保存 argv 与显式环境变量；新安装首次写入可编辑的 agent 配置，不存在硬编码预设。
- **工作区**由 `workspace.Service` 管理仓库清单与 worktree 创建 / 删除，所有会话创建经启动目录协调层登记。
- **持久化**统一经 `statefile.Open[T]`，每个存储独立锁、`unavailable` 只禁用对应功能，并在 `persistence_inventory.json` 登记。
- **分层**由 `internal/archtest` 的 R0–R10 强制：只有 `cmd/` 构造 Service；Controller 只调注入实例；线上 JSON 都是 `contract` 类型并经 `wire` 转换；Service 不编码客户端 JSON。
- **仓库自身的 spec 流程**已存在：`.claude/commands/opsx`、`.claude/skills/openspec-*` 提供 `/opsx:propose`、`/opsx:apply` 等命令，agent 在会话里就能执行“创建 spec / 按 spec 实施”。

## Goals / Non-Goals

**Goals:**

- 用户声明一次流程，之后由服务端按图推进；浏览器、手机断开不影响推进。
- 流程在启动前完整可见、可校验：步骤集合、引用、分支、回环上限都在编译期确定（“静态”）。
- 步骤、agent、流程三者可以独立替换；同一步骤模板可被多个工作流复用，工作流可以嵌入另一个工作流。
- 每一步都是一个普通会话，可以附着观察、手动接管，再交还给流程。
- 遵循现有分层、argv、内容安全与持久化规则，新增执行器只需在 `cmd/` 注册。

**Non-Goals:**

- 不做由 LLM 动态规划的编排（下一步做什么由模型临时决定）。
- 不做跨机器、分布式或多用户调度；不引入数据库、消息队列或外部编排引擎。
- 不让运行在服务重启后自动继续执行；也不持久化 agent 输出或对话正文。
- 不提供定时 / Git 事件触发、通用表达式语言、动态步骤生成或任意脚本钩子。
- 不改变混合会话切换、终端回放与输入语义。

## Decisions

### 1. 三层模型：Agent、步骤模板、工作流

| 层       | 回答的问题             | 来源                               | 变化频率 |
| -------- | ---------------------- | ---------------------------------- | -------- |
| Agent    | 谁来做（命令、环境）   | 现有启动配置，按名称引用           | 低       |
| 步骤模板 | 做什么、怎样算完成     | `workflows.json` 的 `steps` 库     | 中       |
| 工作流   | 按什么顺序、何时回退   | `workflows.json` 的 `workflows`    | 高       |

- 三层之间只有名称引用，没有包含关系：换 agent 只改步骤的 `agent` 字段；调整流程不复制步骤；同一个“代码评审”步骤可以挂在不同工作流里。
- Agent 不新增概念的理由：启动配置已经是“agent = argv + 环境 + 图标”的用户数据，混合资格也由它判定。按**名称**而非 ID 引用，使定义可导出、可在另一台机器导入；运行开始时解析为 ID 并写入运行快照。
- 备选：把 agent 写进步骤（内联 argv）。会复制环境变量（含密钥）并绕开启动配置的校验，拒绝；`command` / `check` 执行器例外地允许内联 argv，因为它们本来就是“一条命令”。

### 2. 定义格式与组合

定义保存为版本化 JSON（与 `launch-configs.json` 同风格），结构示意：

```json
{
  "version": 1,
  "steps": [
    {"name": "openspec-propose", "kind": "chat", "agent": "omp", "prompt": "使用 /opsx:propose 为下面的需求创建 OpenSpec 变更。\n需求：{{params.requirement}}{{#feedback}}\n评审意见：{{feedback}}{{/feedback}}", "result": {"fields": {"change": {"pattern": "^[a-z0-9][a-z0-9-]{0,63}$"}}, "required": ["change"]}, "expect": ["openspec/changes/{{result.change}}/proposal.md"], "timeoutMinutes": 30}
  ],
  "workflows": [
    {"name": "spec-code-review", "params": [{"name": "requirement", "type": "text", "required": true}], "workspace": {"mode": "worktree", "branch": "wf/{{run.shortId}}"}, "nodes": [
      {"id": "spec", "uses": "openspec-propose"},
      {"id": "spec-approval", "kind": "gate", "title": "确认 spec", "on": [{"when": {"decision": "rejected"}, "goto": "spec", "maxVisits": 3}]},
      {"include": "implement-and-review", "as": "impl", "with": {"change": "{{steps.spec.result.change}}"}}
    ]}
  ]
}
```

组合手段只有三种，都在编译期展开：

- **`uses`**：节点引用步骤模板，可用 `with` 覆盖模板里的 `{{with.*}}` 值、`agent`、`timeoutMinutes`；不能覆盖 `kind` 与结果约定，避免同名步骤语义漂移。
- **`include`**：嵌入另一个工作流，`as` 给出命名空间（被嵌入节点 ID 变为 `impl.code`），`with` 映射被嵌入工作流的参数。嵌套深度不超过 4，环引用（A include B include A）编译失败。
- **回环**：只能通过节点的 `on[].goto` 显式回到已出现的节点，并且必须带 `maxVisits`（1–10），超出即按 `onExhausted`（默认 `fail`）处理。

备选与取舍：

- 仓库内文件（如 `.webmux/workflows/*.json`）便于随项目版本化和评审。但 clone 下来的仓库会因此拥有“在本机自动执行 agent”的入口，与“只以服务用户执行用户明确选择的命令”冲突。首期只用状态目录的 `workflows.json`，并提供导入 / 导出；仓库文件作为后续“导入预览 + 显式确认”功能讨论（见 Open Questions）。
- TOML：`config.toml` 只承载启动配置，网页不可改；工作流需要网页编辑，因此用 JSON 存储。
- YAML / DSL：需要新依赖或解析器，且条件语法容易长成表达式语言，拒绝。

### 3. 静态编译：Definition → Plan

`workflow.Compile(defs, launchNames) (Plan, error)` 是纯函数，保存定义时和启动运行时各执行一次：

1. 展开 `include` 与 `uses`，生成带命名空间的扁平节点表；检测 include 环与深度。
2. 校验每个模板变量引用：`params.*` 必须已声明；`steps.<id>.result.<f>` 中的 `<id>` 必须是**一定先于当前节点执行**的节点，`<f>` 必须是该节点结果约定的字段。引用可能尚未执行的节点（例如只在某分支上）是编译错误，除非使用 `{{#x}}…{{/x}}` 可选段。
3. 校验图：入口唯一；所有 `goto` 目标存在；每个回边带 `maxVisits`；没有不可达节点；`gate` 节点的 `on` 只能匹配 `decision`。
4. 解析 agent 名称；保存时缺失只给警告（定义可以先写、配置后补），启动时缺失是错误。
5. 输出不可变 `Plan`：节点数组、边、每个节点渲染所需的变量清单、执行器种类与已解析的 agent ID。

收益：拼写错误、引用不存在的步骤、无界回环、错用执行器都在点击“运行”之前暴露；运行时 actor 只解释 `Plan`，不再处理定义层的组合。上限：每个工作流展开后最多 64 个节点，定义文件最多 100 个工作流、200 个步骤模板。

### 4. 模板变量：只查找，不计算

- 语法：`{{path}}` 查找替换，`{{#path}}…{{/path}}` 在值存在且非空时输出中间内容。没有函数、运算、循环或条件表达式。
- 命名空间：`params.*`、`run.id|shortId|cwd|branch`、`steps.<id>.result.*`、`steps.<id>.decision|feedback|exitCode|session`、步骤模板内的 `with.*`，当前步骤的 `result.*`（仅用于 `expect`）、`step.promptFile`（仅 `command` argv）与 `attempt.number|error`（上一次失败的可读原因，供重试提示词使用）。`feedback` 是指向当前节点的最近一次闸门退回意见的简写。
- **argv 边界**：`command` / `check` 的 argv 每一项独立渲染，渲染结果仍是一项，从不按空白拆分，也不经过 shell；空值项按 `omitEmpty` 声明删除或报错。
- 提示词文本上限 64 KiB（与混合会话一致）；渲染结果只在内存中交给执行器，不写日志、不写运行记录。
- 备选：`text/template`。表达力过强（管道、函数、range），编译期也难以列出引用集合，拒绝。

### 5. 执行器（Executor）

```go
// Executor starts one attempt of a planned node. Implementations live in
// internal/workflow and are registered by cmd/ through Options.Executors.
type Executor interface {
	Kind() string
	Validate(node PlannedNode) error
	Start(ctx context.Context, in StartInput) (Attempt, error)
}

// Attempt is one running node attempt owned by the run actor.
type Attempt interface {
	SessionID() string
	Done() <-chan struct{}
	Outcome() Outcome // valid after Done
	Cancel(ctx context.Context) error
}
```

| 种类            | 启动                                                          | 完成判定                                                      | 失败                                                      |
| --------------- | ------------------------------------------------------------- | ------------------------------------------------------------- | --------------------------------------------------------- |
| `chat`          | `hybrid.Create(SideACP)` 或复用会话；`Prompt` 记录 `TurnID`   | 订阅事件，匹配 `TurnID` 的 `turn.end` 且 `StopReason=end_turn` | `refusal`、`max_tokens`、`error`、`auth_required`、超时    |
| `command`       | `Manager.Create`，argv 来自定义或启动配置 + 渲染后的追加参数  | `Session.Done()` 后 `ExitCode == 0`                           | 非零退出、超时（关闭会话）                                |
| `check`         | 同 `command`，但只接受内联 argv、不引用 agent，默认 10 分钟   | 退出码 0；`allowExitCodes` 可声明其他成功码                   | 同上                                                      |
| `gate`          | 不启动进程；运行进入 `waiting`，发出注意力                    | 用户选择 `approved` / `rejected`，可附意见                    | 运行取消                                                  |
| `tui`（第二期） | 用启动配置开 PTY，等首个 `OSC 7501 idle`，经 `Submit` 提交    | 根记录 `working` 之后回到 `done` / `idle`                     | `error`、进程退出、启动 30 秒内无 `OSC 7501`、超时        |

- `chat` 是首选：完成信号来自协议而非启发式。权限模式由步骤显式声明 `permission: auto|ask`；`ask` 下出现权限请求时步骤显示为“等待作答”，不计为失败，超时计时照常进行。
- `command` 覆盖没有 ACP 档案的 agent（`claude -p`、`codex exec`、`gemini -p`）。提示词可以作为 argv 的一项，也可以用 `{{step.promptFile}}`：服务端把渲染后的提示词写到运行私有目录的 0600 文件，命令读取后由服务端在步骤结束时删除。
- `tui` 推迟到第二期：依赖 agent 发出 `OSC 7501`；不发的 agent 在编译期拒绝（档案缺少声明）而不是退回到“2 秒没输出就算完成”的猜测。
- 备选：所有步骤一律 `command`。可以工作，但失去多轮复用、权限交互、回合级取消和聊天视图，拒绝作为唯一方案。

### 6. 结果约定：工作区即总线，结构化结果只传“指针”

- 步骤之间的大块数据（spec、代码、评审报告）只经过共享的工作目录与 Git：上一步写文件 / 提交，下一步读。这与“服务端只传字节、不做持久存储”一致，也让人随时能在代码视图里看到中间产物。
- 每个可能被下游引用的步骤声明 `result`：字段名、类型（`string` / `enum` / `integer` / `boolean`）、`pattern` 或 `enum`、`required`。执行器为会话设置 `WEBMUX_WORKFLOW_RESULT=<run 私有目录>/<node>.json`，并在提示词末尾自动附加结果说明（`resultInstructions: auto|none`）。
- 服务端在每次尝试前删除该文件，完成后读取：上限 16 KiB、严格 JSON（复用 `jsonutil`）、拒绝未知字段与符号链接、按声明校验。不满足即该次尝试失败，原因可读（“结果缺少 verdict”）。
- 复用同一 `chat` 会话时进程环境不可变，因此结果文件路径按“节点”而不是“尝试”固定，由“尝试前删除”保证不读到旧值。
- `expect` 列出完成后必须存在的相对路径（可含模板），在运行根目录内逐段检查，拒绝 `..`、绝对路径与指向根外的符号链接。
- 备选：解析 agent 最后一条消息里的 JSON。脆弱（模型会加解释、代码围栏），且需要服务端保留消息正文，拒绝；最后一条消息只在界面里展示，不参与控制流。

### 7. 控制流

- 默认按节点顺序执行；节点可声明 `on: [{when, goto, maxVisits}]`，从上到下取第一个匹配。`when` 是结构化对象，不是字符串表达式：`{"result": "verdict", "equals": "changes_requested"}`、`{"result": "severity", "in": ["high", "critical"]}`、`{"decision": "rejected"}`、`{"exitCode": 1}`。
- 失败处理 `onFailure`：`fail`（默认，运行失败并停在该节点）、`retry`（`maxAttempts` 1–5，复用或新建会话由 `session` 决定）、`goto`（例如检查失败回到编码）、`gate`（插入一次人工决定：重试 / 跳过 / 终止）。
- `skip` 只能由人工在运行页对失败节点执行，并要求该节点没有被下游以 `required` 结果引用；否则需要人工填写这些结果字段。
- 第二期：`parallel` 组（例如安全评审与正确性评审并行），组内节点不得共享会话，结果按节点 ID 汇合；首期不做，以免把单写者 actor 变成调度器。

### 8. 会话策略

- `session: new`（默认）：每次尝试一个新会话，名称为“<运行名> · <节点标题>”。
- `session: {"reuse": "<节点 ID>"}`：复用某个先前 `chat` 节点的会话，用于“评审意见 → 回到同一个编码对话修改”，保留 agent 上下文。编译期要求被复用节点种类相同、agent 相同、一定已执行。
- 会话关联：`session.Info` 增加可选 `Workflow{RunID, NodeID}`，列表与卡片据此分组；用户可以照常附着、输入、切到终端。用户手动输入不会打断编排，但 `chat` 节点在用户接管期间发出的回合不会被误认为本步骤的回合（以 `TurnID` 匹配）。
- 运行结束后步骤会话保持存在，供回看；“关闭运行”会关闭其全部会话（混合会话进入可恢复分组，与现有行为一致）。

### 9. 工作区

- `workspace.mode`：`cwd`（在用户选择的目录直接执行）或 `worktree`（启动时经 `workspace.Service.CreateWorktree` 从选定仓库与基线分支新建 worktree，分支名可模板化）。
- 同一运行的所有步骤共享该目录；启动目录协调层照常登记，因此运行进行中删除该 worktree 会被现有确认流程阻止。
- 并发：同一目录同时只允许一个运行处于活动状态（`ErrConflict`），避免两个流程在同一工作树上交错写入；`worktree` 模式天然隔离，是推荐的默认值。

### 10. 运行生命周期与单写者

```text
created → running ⇄ waiting(gate | permission | user-takeover)
running → succeeded | failed | cancelled
任意活动状态 --服务重启--> interrupted --恢复--> running
```

- 每个运行一个 actor goroutine，拥有全部可变状态（当前节点、访问计数、尝试、结果、闸门决定），与混合会话 actor 同构；外部命令（取消、闸门答复、恢复、跳过）以带客户端请求 ID 的命令投递，30 分钟内重复 ID 返回首个结果。
- actor 不在持锁期间调用执行器的阻塞操作；执行器回调只通过 `post` 回到 actor。
- 取消：先 `Attempt.Cancel`（`chat` 发送 ACP cancel，`command` 关闭会话：SIGHUP → 5 秒 → SIGKILL），确认结束后才把运行标记为 `cancelled`。
- 上限：同时活动的运行最多 8 个，记录最多 200 条，超出时最旧的终态记录可由用户清理（不自动删除）。

### 11. 持久化与重启

| 文件                 | 内容                                                                                       | 不保存                         |
| -------------------- | ------------------------------------------------------------------------------------------ | ------------------------------ |
| `workflows.json`     | 步骤模板、工作流定义、版本号                                                               | —                              |
| `workflow-runs.json` | 运行 ID、定义快照（编译前 JSON）、参数、工作目录、节点状态 / 访问计数、结果字段、闸门决定与意见、会话 ID、时间 | 渲染后的提示词、agent 输出、环境变量值 |

- 两个文件各自 `statefile.Open` 与独立锁，失败策略 `unavailable`：定义不可读只禁用工作流功能，不影响会话与混合会话。
- 运行保存**定义快照**：之后编辑定义不改变进行中或已中断的运行，恢复时用快照重新编译。
- 参数是用户输入（例如需求文本），恢复需要它，因此保存；上限 32 KiB，界面在参数表单处说明它会写入状态目录。闸门意见同理。
- 服务启动时把活动运行改为 `interrupted`。恢复时当前节点从新一次尝试开始：`chat` 节点若原会话仍在混合会话可恢复记录中，优先恢复同一 agent 会话再重发提示词；否则新建。已成功的节点不重跑。
- 运行私有目录 `<state_dir>/workflow-runs/<runId>/`（0700）只放结果文件与提示词文件，运行记录被移除时删除；它是登记在持久化清单中的例外，不是 JSON 存储。

### 12. 人工闸门、权限与通知

- 闸门节点可配置 `title`、`instructions`（模板，可引用前序结果，例如“请审阅 `openspec/changes/{{steps.spec.result.change}}/`”）与 `links`（运行根内的相对路径，在运行页以代码视图打开）。
- 退回意见 ≤ 8 KiB，写入运行记录，后续节点用 `{{feedback}}` 或 `{{steps.<gate>.feedback}}` 引用。
- 运行进入闸门等待、节点失败、运行完成时，在运行的“当前步骤会话”上置注意力（question / error / done），复用现有卡片提醒与 Web Push，不新增推送类型。
- 无人值守与安全：`permission: auto` 等于允许 agent 以服务用户执行任意工具。定义编辑器在保存含 `auto` 的步骤时显示提示；运行页标记“自动批准”的步骤。服务端不替用户选择更宽松的模式。

### 13. 线上契约与 API

- REST：
  - `GET/PUT /api/workflows`（整体读写，带修订号防覆盖，与设置同步的乐观并发一致）；`POST /api/workflows/compile`（只编译不保存，返回错误与警告，用于编辑器实时校验）；`GET /api/workflows/export`、`POST /api/workflows/import`。
  - `POST /api/workflow-runs`（工作流名、参数、工作区选择、客户端请求 ID）；`GET /api/workflow-runs`、`GET /api/workflow-runs/{id}`；`POST .../{id}/cancel|resume`；`POST .../{id}/nodes/{nodeId}/decision|retry|skip`；`DELETE .../{id}`（仅终态）。
- WebSocket `/ws/workflow-runs`：快照后推送 `run.upsert` / `run.removed`，与 `/ws/sessions` 相同的“快照 + 事件、有界队列、慢客户端断开”模式。备选是把运行事件并入 `/ws/sessions`，可少一条连接，但会让会话列表契约承担第二个领域，首期分开。
- 错误种类：`workflow.ErrInvalid`（400，含编译错误列表）、`ErrNotFound`、`ErrConflict`（目录已有活动运行、修订冲突）、`ErrUnavailable`（503）、`ErrLimit`；映射只写在 `internal/api/errors.go`。
- 所有请求与事件是 `internal/contract` 类型，经 `internal/wire` 转换；前端在 `web/src/schemas.ts` 用 Zod 校验；新增线上快照组 `workflows`。

### 14. 前端

- 首页：运行卡片位于会话列表上方的“工作流”分组，显示名称、当前节点、进度（第 n / N 个，回环显示访问次数）、状态；其步骤会话折叠在卡片内，不再与普通会话混排。
- 运行页（窄屏优先）：纵向步骤条；每个节点显示种类图标（Iconify tabler）、状态、耗时、结果字段、“打开会话”；闸门节点在页面底部给出“批准 / 退回（附意见）”；失败节点给出“重试 / 跳过 / 终止”。
- 定义编辑器：首期为带语法校验的 JSON 编辑器 + 编译结果面板（错误定位到 JSON 路径）+ 只读流程预览；可视化拖拽编辑不在本变更范围。新建运行表单由 `params` 声明生成。
- 复用 shadcn-svelte 组件、`api.ts` 与 `ws.ts`，不新增请求或重连约定。

### 15. CLI

- `webmux workflow ls [--json]`、`webmux workflow run <名称> [--param k=v ...] [--param-file k=path] [--cwd 路径 | --repo 仓库 --base 分支] [--attach]`、`webmux workflow status <运行 ID>`、`webmux workflow cancel <运行 ID>`。
- 全部经本机套接字调用同一套 API；`--attach` 依次附着到当前步骤会话，步骤切换时自动跟随，闸门等待时退出并提示到 Web 端或 `webmux workflow approve`（第二期）。

### 16. 分层落点

- `internal/workflow` 归类为 Service；可以依赖 `session`、`hybrid`、`launch`、`workspace`、`statefile`、`jsonutil`、`attentionkind`，不依赖 Controller。
- 执行器实现放在 `internal/workflow` 内，`cmd/webmux` 通过 `workflow.Options{Executors: ...}` 显式注册；执行器种类集合因此在装配根可见，测试可以注入假执行器。
- `Compile` 若被编辑器路由调用，经注入的 `*workflow.Service` 方法暴露，不登记为纯函数例外。

### 17. 首次预置模板

新安装首次创建 `workflows.json` 时写入一个可编辑示例，引用首次预置的 omp 与 Codex 启动配置名（不存在时编译警告）：

1. `spec`（chat / omp）：`/opsx:propose`，结果 `change`，期望 `proposal.md`、`tasks.md` 存在。
2. `spec-validate`（check）：`openspec validate {{steps.spec.result.change}} --strict`，失败回到 `spec`（最多 2 次）。
3. `spec-approval`（gate）：退回回到 `spec`（最多 3 次）。
4. `code`（chat / omp）：`/opsx:apply {{steps.spec.result.change}}`，附带 `{{#feedback}}` 段。
5. `test`（check）：`make test-fast`，失败回到 `code`（复用会话，最多 3 次）。
6. `review`（command / Codex）：新会话、不同 agent，`codex exec` 按仓库 `AGENTS.md` 审查差异，结果 `verdict`（`approve` / `changes_requested`）与 `summary`；`changes_requested` 回到 `code`（复用会话，最多 2 次），回环耗尽进入人工闸门。
7. `final-approval`（gate）。

“评审用另一个 agent、编码回环复用同一会话”是模板体现的组合思路，用户可以任意改写；服务端没有硬编码这些步骤。

## Risks / Trade-offs

- **[无人值守的权限]** `auto` 让 agent 执行任意工具 → 显式字段、编辑器提示、推荐 `worktree` 模式；文档继续强调仅在回环地址或可信 VPN 部署。
- **[agent 不遵守结果约定]** 模型可能不写或写错结果文件 → 严格校验并给出可读原因；`retry` 时可在提示词中附上次失败原因（`{{attempt.error}}`）。
- **[回环发散]** 评审与修改来回拉锯 → 每条回边强制 `maxVisits`，耗尽后进入人工决定而非静默失败。
- **[重启语义]** 运行中断后从节点起点重来，agent 可能重复部分修改 → 节点应尽量幂等（spec 已存在则更新）；恢复前在运行页展示当前工作树状态。
- **[会话列表膨胀]** 每次尝试一个会话 → 运行卡片内折叠，关闭运行批量关闭；混合会话记录受 `MaxRecords` 限制，启动时检查剩余配额。
- **[只有两个 ACP 档案]** Claude Code / Codex 只能走 `command` 或第二期的 `tui` → 执行器接口不依赖档案数量，新增 ACP 档案后定义只需把 `kind` 改为 `chat`。
- **[定义快照与可读性]** 快照使记录变大 → 定义上限与记录上限共同约束文件体积。

## Migration Plan

1. 新增文件与功能，不改写现有状态文件；旧版本忽略 `workflows.json` 与 `workflow-runs.json`。
2. 回退到旧版本前应先结束活动运行；残留的步骤会话按现有会话与混合会话规则处理。
3. 首次预置只在 `workflows.json` 不存在时写入；已有安装需要在设置页手动“添加示例模板”。

## Open Questions

- 仓库内工作流文件：是否提供“从已登记仓库导入 `.webmux/workflows/*.json`，预览差异后显式确认”的流程？
- `parallel` 组与 `tui` 执行器排在第二期，是否需要更早？
- 运行页是否需要一个“汇总”节点视图（把各步骤的结果字段和关键文件链接汇成报告）？
- 闸门是否允许从 CLI 作答（`webmux workflow approve`），以及是否需要多人批准（当前为单用户产品，倾向不需要）。
