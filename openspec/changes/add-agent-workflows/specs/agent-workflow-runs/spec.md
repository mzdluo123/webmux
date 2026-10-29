## Purpose

让用户启动一个已定义的工作流，由服务端按编译后的计划逐步驱动 agent 会话，在浏览器断开时继续推进；每一步都是可附着、可接管的普通会话，人工闸门、失败与完成通过现有提醒通知用户。

## ADDED Requirements

### Requirement: 运行由单写者 actor 按计划推进

启动运行时，系统 SHALL 用当前定义编译计划、保存定义快照与参数，并由该运行独有的 actor 推进节点。外部命令（取消、闸门答复、重试、跳过、恢复）SHALL 携带客户端请求 ID，30 分钟内重复的 ID SHALL 返回首个结果而不重复执行。

运行状态 SHALL 为 `running`、`waiting`、`succeeded`、`failed`、`cancelled` 或 `interrupted` 之一；节点状态 SHALL 为 `pending`、`running`、`waiting`、`succeeded`、`failed`、`skipped` 或 `cancelled` 之一。浏览器断开 SHALL NOT 影响运行推进。

同一工作目录同时 SHALL 至多有一个活动运行；同时活动的运行 SHALL 不超过 8 个。

#### Scenario: 手机锁屏后继续

- **WHEN** 用户在手机上启动运行后锁屏 20 分钟
- **THEN** 运行在服务端继续推进，用户回到运行页时看到最新节点状态

#### Scenario: 同目录重复启动

- **WHEN** 目录 `/repo` 已有活动运行，用户再以 `cwd` 模式在 `/repo` 启动另一个运行
- **THEN** 启动以冲突失败，提示改用 worktree 模式或等待现有运行结束

### Requirement: chat 执行器以 ACP 回合结束判定完成

`chat` 节点 SHALL 以引用的混合启动配置从聊天端创建混合会话（或按 `session.reuse` 复用先前节点的会话），以 `Prompt` 发送渲染后的提示词并记录 `TurnID`。只有匹配该 `TurnID` 且 `StopReason` 为 `end_turn` 的 `turn.end` SHALL 视为完成；`refusal`、`max_tokens`、`max_turn_requests`、`error`、`auth_required` 与超时 SHALL 视为失败；`cancelled` SHALL 视为取消。

节点的权限模式 SHALL 由定义显式给出。`ask` 模式下出现权限请求或表单时，节点 SHALL 显示为 `waiting` 并置注意力，SHALL NOT 视为失败。

用户在该会话中手动发出的回合 SHALL NOT 被当作节点回合。

#### Scenario: 回合正常结束

- **WHEN** `spec` 节点的回合以 `end_turn` 结束且结果文件合法
- **THEN** 节点标记成功，运行进入下一个节点

#### Scenario: 需要登录

- **WHEN** 回合以 `auth_required` 结束
- **THEN** 节点失败，原因提示切到终端登录，运行按 `onFailure` 处理

### Requirement: command 与 check 执行器以退出码判定完成

`command` 与 `check` 节点 SHALL 以渲染后的 argv 创建普通 PTY 会话，不经过 shell。会话退出且退出码为 0（或属于 `allowExitCodes`）SHALL 视为成功；其他退出码 SHALL 视为失败并记录退出码；超时 SHALL 关闭会话并视为失败。

提示词以 `{{step.promptFile}}` 传递时，系统 SHALL 把渲染后的提示词写入运行私有目录中的 0600 文件，并在该次尝试结束后删除。

#### Scenario: 测试失败回到编码

- **WHEN** `check` 节点 `make test-fast` 以退出码 2 结束，其 `onFailure` 为 `goto: code`，`maxVisits` 未耗尽
- **THEN** 运行回到 `code` 节点，`code` 的提示词可通过 `{{steps.test.exitCode}}` 得知失败

### Requirement: 节点结果经文件约定传递并严格校验

声明了 `result` 的节点，系统 SHALL 在会话环境中设置 `WEBMUX_WORKFLOW_RESULT`、`WEBMUX_WORKFLOW_RUN_ID` 与 `WEBMUX_WORKFLOW_NODE`，结果文件路径 SHALL 在节点的所有尝试间保持不变，并 SHALL 在每次尝试前删除旧文件。`resultInstructions` 为 `auto`（默认）时，系统 SHALL 在提示词末尾附加结果文件路径与字段说明。

尝试完成后，系统 SHALL 读取结果文件：不超过 16 KiB、严格 JSON、不是符号链接、无未知字段、满足声明的类型与 `pattern` / `enum` 与 `required`。不满足时该次尝试 SHALL 失败并给出可读原因。`expect` 列出的相对路径 SHALL 在运行根目录内存在，拒绝 `..`、绝对路径与指向根外的符号链接。

结果字段 SHALL 写入运行记录；提示词正文、agent 消息与工具输出 SHALL NOT 写入运行记录或日志。

#### Scenario: agent 未写结果

- **WHEN** 评审回合正常结束，但结果文件不存在
- **THEN** 该次尝试失败，原因为“结果文件缺失”，按 `onFailure` 处理

#### Scenario: 复用会话时读到旧结果

- **WHEN** `code` 节点复用会话进行第二次尝试，上一尝试的结果文件仍在
- **THEN** 发送提示词前旧文件已被删除，第二次尝试只会读到本次写入的结果

### Requirement: 运行共享工作区并可新建 worktree

运行 SHALL 以 `cwd` 或 `worktree` 模式确定运行根目录，所有节点共享该目录。`worktree` 模式 SHALL 经仓库服务从选定仓库与基线分支新建 worktree，分支名可使用模板。运行中的步骤会话 SHALL 经启动目录协调层登记，使运行进行中删除该 worktree 被现有确认流程阻止。

#### Scenario: 每次运行独立 worktree

- **WHEN** 用户在同一仓库以 `worktree` 模式先后启动两个运行
- **THEN** 两个运行分别在 `wf/<shortId>` 分支的独立 worktree 中执行，互不冲突

### Requirement: 步骤会话可见、可接管并按运行分组

每次节点尝试创建或复用的会话 SHALL 在会话信息中携带可选的 `workflow` 关联（运行 ID、节点 ID）。首页 SHALL 按运行分组显示这些会话；用户 SHALL 能照常附着、输入与切换混合会话两端。运行结束后步骤会话 SHALL 保留，直到用户关闭运行；关闭运行 SHALL 关闭其全部步骤会话。

#### Scenario: 中途手动纠正

- **WHEN** `code` 节点运行中，用户打开其聊天视图补充一句说明并等待回合结束
- **THEN** 用户回合不被当作节点完成，节点在自己的回合以 `end_turn` 结束后才推进

### Requirement: 人工闸门暂停运行并把意见交给后续节点

`gate` 节点 SHALL 使运行进入 `waiting`，在运行页显示标题、渲染后的说明与运行根内的链接，并接受 `approved` 或 `rejected` 决定与不超过 8 KiB 的意见。决定与意见 SHALL 写入运行记录，后续节点 SHALL 能以 `{{steps.<gate>.decision}}`、`{{steps.<gate>.feedback}}` 或 `{{feedback}}` 引用。

运行进入闸门等待、节点失败或运行结束时，系统 SHALL 在当前步骤会话上置相应注意力（question、error、done），沿用现有卡片提醒与 Web Push。

#### Scenario: 退回 spec

- **WHEN** 用户在 `spec-approval` 闸门选择退回并填写“补充离线场景”
- **THEN** 运行回到 `spec` 节点，其提示词的可选段包含该意见，`spec-approval` 的访问次数加一

### Requirement: 取消、重试、跳过与重启后恢复

取消运行时，系统 SHALL 先取消当前尝试（`chat` 发送 ACP cancel，`command` / `check` 关闭会话），确认其结束后再把运行标记为 `cancelled`。

用户 SHALL 能对失败节点重试；SHALL 只能在该节点的必需结果未被下游引用、或由用户填写这些字段时跳过它。

运行记录 SHALL 保存在 `<state_dir>/workflow-runs.json`，含定义快照、参数（≤ 32 KiB）、运行根、节点状态、访问计数、结果字段、闸门决定与意见、会话 ID 和时间，SHALL NOT 含渲染后的提示词、agent 输出或环境变量值。服务启动时 SHALL 把活动运行标记为 `interrupted`。恢复 SHALL 用快照重新编译，已成功的节点 SHALL NOT 重跑，当前节点 SHALL 以新尝试开始；`chat` 节点的原 agent 会话仍可恢复时 SHALL 优先恢复它。

#### Scenario: 服务重启后恢复

- **WHEN** 运行在 `code` 节点时服务重启，用户在运行页点击恢复
- **THEN** `spec` 与 `spec-approval` 保持成功，`code` 从新尝试开始并恢复原 agent 会话后重发提示词

#### Scenario: 定义已修改

- **WHEN** 运行中断后用户修改了工作流定义，再恢复该运行
- **THEN** 恢复使用运行保存的定义快照，修改只影响之后的新运行

### Requirement: 工作流 API、事件流与 CLI

系统 SHALL 提供 REST 接口读写定义（带修订号）、编译预览、导入导出，以及创建、列出、查询、取消、恢复、删除运行和对节点作出决定、重试、跳过。所有请求与响应 SHALL 是 `internal/contract` 类型，前端 SHALL 用 Zod 校验，认证与 Origin 检查 SHALL 与现有接口相同。

系统 SHALL 提供 `/ws/workflow-runs`，先发送快照，再推送运行变化；订阅队列有界，慢客户端 SHALL 被断开后重连。

CLI SHALL 提供 `webmux workflow ls|run|status|cancel`，经本机套接字调用同一套 API；`run --attach` SHALL 附着当前步骤会话并在节点切换时跟随。

#### Scenario: 未认证访问

- **WHEN** 不带 Token 的 TCP 请求访问 `/api/workflow-runs`
- **THEN** 返回 401，不泄露运行信息

#### Scenario: 从命令行启动

- **WHEN** 用户执行 `webmux workflow run spec-code-review --param-file requirement=req.md --repo webmux --base main`
- **THEN** 服务端新建 worktree 并启动运行，CLI 打印运行 ID 与运行页链接
