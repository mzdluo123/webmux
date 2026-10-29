## Why

用户在 WebMux 中已经能用启动配置开出 OpenCode、omp、Claude Code、Codex 等 agent 会话，混合会话还能以 ACP 结构化地发送提示词并观察回合结束。但“需求评审并创建 spec → 按 spec 编写代码 → 评审代码”这样的多阶段工作仍要靠人逐个开会话、复制上一步产物、盯着回合结束再手动推进；手机上尤其难以接力。

需要一种**静态声明**的编排机制：执行顺序、分支与回环在启动前就完整可见、可校验；步骤会变化，因此步骤、agent 与流程必须可以独立替换、复用和组合；同时保持 WebMux 的既有理念——每一步都是一个真实、可附着、可接管的会话，服务端不成为持久数据库，命令以 argv 执行。

## What Changes

- 新增服务层包 `internal/workflow`：保存工作流定义，把定义**静态编译**为不可变执行计划，并以每次运行一个 actor 的单写者方式驱动步骤。
- 三层可组合模型：
  - **Agent**：直接引用现有启动配置（按名称），不新增第二套 agent 概念。
  - **步骤模板（Step template）**：执行器种类 + agent + 提示词模板 + 结果约定 + 产物检查 + 超时，可被多个工作流引用并以 `with` 覆盖参数。
  - **工作流（Workflow）**：步骤图，支持顺序、条件跳转、有界回环、人工闸门，以及 `include` 嵌入另一个工作流（编译期展开，带命名空间与环检测）。
- 可插拔执行器（Executor）接口，首批内置：
  - `chat`：经混合会话的 ACP 通道发送提示词，以 `turn.end` 的停止原因判定完成；
  - `command`：以 argv 启动一次性 PTY 会话（如 `claude -p`、`codex exec`），以退出码判定；
  - `check`：不调用 agent 的确定性检查命令（如 `make test-fast`、`openspec validate --strict`）；
  - `gate`：人工批准 / 退回，退回意见作为后续步骤的输入。
  - 第二阶段计划 `tui`：向原生 TUI 提交提示词，以 `OSC 7501` 程序状态判定完成。
- 步骤之间以“工作区即总线”传递数据：共享同一工作目录或新建的 Git worktree；每步可声明一个小型结构化结果（agent 写入 `WEBMUX_WORKFLOW_RESULT` 指向的 JSON 文件，服务端按声明的字段校验），后续步骤以 `{{steps.<id>.result.<field>}}` 引用。模板只做查找替换，argv 中每项渲染后仍是一项。
- 运行记录 `workflow-runs.json` 只保存定义快照、参数、步骤状态与会话 ID；不保存提示词渲染结果、agent 输出或对话。服务重启后运行标记为 `interrupted`，可从当前步骤恢复。
- REST `/api/workflows`、`/api/workflow-runs` 与 WebSocket `/ws/workflow-runs`；首页按运行分组显示步骤会话，运行页以纵向步骤条展示状态、闸门操作与会话入口；闸门等待、失败与完成沿用现有注意力与 Web Push。
- CLI：`webmux workflow ls|run|status|cancel`。
- 新安装首次创建 `workflows.json` 时写入可编辑的“需求 → spec → 编码 → 评审”示例模板（与启动配置首次预置一致，不是内置预设）。

## Capabilities

### New Capabilities

- `agent-workflow-definitions`：步骤模板、工作流定义、组合（`uses` / `include`）、模板变量、条件与回环的静态编译与校验，以及定义的持久化与首次预置。
- `agent-workflow-runs`：运行生命周期、执行器语义、结果约定、工作区隔离、人工闸门、通知、重启后中断与恢复，以及运行的 API、界面与 CLI。

### Modified Capabilities

- 无需求级修改。会话信息新增可选 `workflow` 关联（运行 ID、步骤 ID），属于新能力的附加字段；混合会话、启动配置与仓库 / worktree 的既有需求保持不变。

## Impact

- 后端：新增 `internal/workflow`；`internal/contract`、`internal/wire`、`internal/api`（路由、错误映射）、`internal/hub` 或列表推送；`internal/session` 的 `Info` 增加可选关联；`cmd/webmux` 与 `cmd/internal/assemble` 装配；`internal/sessioncmd` 增加 CLI。
- 守卫：`internal/archtest/layers_test.go` 归类新包；`internal/archtest/persistence_inventory.json` 与 `docs/persistence.md` 登记 `workflows.json`、`workflow-runs.json`；线上快照新增端点与事件。
- 前端：`web/src/schemas.ts`、`api.ts`、首页运行分组、运行页、定义编辑器、设置入口。
- 文档：`docs/protocol/` 新增工作流协议页，`docs/user-stories/` 新增工作流故事组。
- 不涉及：会话 PTY 生命周期、回放与输入语义；混合会话切换协议；不引入数据库、任务队列或外部编排引擎。
