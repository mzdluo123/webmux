## Context

动机见 proposal.md，服务端契约见 `unify-session-surface`（`design.md` 与 `specs/session-surface/spec.md`）。与 CLI 设计相关的现状：

- 命令由 `internal/sessioncmd` 的命令表定义，帮助、补全、`--help` 与 `internal/agentguide/llms/commands.md` 都由命令表生成，一致性测试要求每个选项有说明与默认值。
- `webmux send` 在客户端读取会话、检查所在端，再拨 `/ws/s/{id}` 发送带回执的提交；`chat send` 走 `chat/prompts`，`--wait` 先连接 `/ws/s/{id}/chat` 再发送，按 `promptId` 或 `queueItemId` 跟踪回合。
- `run --new-worktree BRANCH` 先在客户端调用 `POST /api/repositories/{id}/worktrees`，再创建会话，最后按 `--no-pin` 决定是否加入主页显示；
  `chat new` 调用 `POST /api/quick-starts`，分支名在客户端生成，失败按服务端返回的步骤报告。
- `run` 已有 `--attach` 开关，表示附着终端。

## Goals / Non-Goals

**Goals:**

- 每个意图只有一个命令；命令只按会话 ID 操作，不要求调用方先判断会话种类或所在视图。
- 视图不匹配时的错误统一：服务端 409 `wrong_view`、`no_chat_view` 映射为退出码 1 并给出下一步命令。
- `chat` 组只剩聊天视图真正独有的操作。

**Non-Goals:**

- 不改变 `wait`、`logs`、`screen`、`events`、`attach`、`kill`、`rename`、`label`、`notify` 的语义。
- 不改变退出码约定（0 成功、1 失败、2 用法错误、124 超时、130/143 信号）。
- 不提供旧命令的别名或迁移提示命令。

## Decisions

### C1 命令对照

| 现在 | 之后 |
|---|---|
| `run --new-worktree BRANCH`、`--start-side`、`chat new` | `run --new-worktree [--repo R] [--branch B] [--base REF]`、`--view`、`--set`、`--permission`、`--attach-file`、`--prompt`、`--reply` |
| `send` / `chat send` | `send [--enter] [--key K]... [--attach-file P]... [--mode steer] [--wait STATE \| --reply] [--auto-wait-requests] [--timeout D] [--json]` |
| `send --key esc` / `chat cancel` | `interrupt SESSION` |
| `chat switch --to` | `view SESSION terminal\|chat [--force] [--agent-session ID]` |
| `chat ls` / `ls` | `ls [--state STATE]... [--archived] [--repo REPO]` |
| `chat resume RECORD --side` | `resume SESSION [--view] [--agent-session ID] [--without-env] [--detach]` |
| `chat forget` / `rm` | `rm SESSION [--forget] [--yes]` |
| `chat status/watch/answer/set/permission/queue` | 保留在 `chat` 组 |
| `launch-config add\|edit --hybrid\|--no-hybrid` | `--chat\|--no-chat` |

备选：保留 `chat` 组作为“agent 会话”的全部入口，只把 `send` 做成分派。否决：创建、列表、恢复与清理仍然各有两套，脚本仍要先判断会话种类。

### C2 `--reply` 与 `--wait` 分开

`--wait STATE` 对两种视图都等待会话状态（状态在聊天视图上已由回合与请求推导）。`--reply` 只对聊天视图有意义：先连接聊天流再发送，
跟踪本次输入开启的回合并输出最终回复，退出码规则沿用 `chat send --wait`；在终端视图上不写入任何内容，以 1 结束并提示改用 `--wait STATE`。两者互斥。

备选：让 `--wait` 在聊天视图上自动输出回复。否决：同一个选项在两种视图上输出不同类型的结果，脚本无法依赖 `--json` 的形状。

### C3 `run` 一次请求完成创建

`run` 把位置、命令、视图、首条消息、配置项、权限模式与附件放进一次 `POST /api/sessions`，带新的 `clientRequestId`，同一次调用内的重试复用它。
`--attach-file` 先经 `POST /api/uploads` 上传。`--new-worktree` 时分支名在省略时由服务端生成；创建成功后，除非 `--no-pin`，CLI 再把新 worktree 加入主页显示。
失败时按服务端的 `step` 报告并列出已创建的 worktree 与会话。

`--init` 保持显式开启：`chat new` 原来在仓库有初始化指令时默认附带，`run` 不改变自己的默认值，迁移说明中写明需要加 `--init`。
`--view chat` 隐含 `--detach`；`--reply` 要求 `--view chat` 与 `--prompt`。附件选项命名为 `--attach-file`，因为 `--attach` 已表示附着终端。

### C4 视图不匹配的错误

- 会话没有聊天视图：`chat *`、`view ... chat`、`--reply` 以 1 结束，说明该会话没有聊天视图。
- 会话在终端视图而命令需要聊天视图：`chat *` 中需要聊天端的命令以 1 结束，提示 `webmux view SESSION chat`。
- `send` 的选项与当前视图不符：服务端 409 `wrong_view`，CLI 以 1 结束并报告原因，不写入任何内容。

### C5 生成物

补全的已归档会话来源改为 `GET /api/sessions?state=archived`；`worktree` 命令中对已归档会话的计数同样改读该接口。
`internal/agentguide/llms/commands.md` 由命令表重新生成，`launch`、`chat`、`sessions`、`recipes` 主题与 `agentguide.go` 的主题摘要手工改写。

## Risks / Trade-offs

- [脚本与 agent 提示中的旧命令失效] → 同一二进制内 CLI 与服务端同版本；`agentguide` 随版本更新；发布说明列出 C1 对照表。
- [`run --new-worktree` 由取值改为开关，旧写法 `--new-worktree fix/lint` 会把 `fix/lint` 当作位置参数] → `run` 在没有 `--` 时拒绝位置参数并以 2 结束，提示改用 `--branch`。
- [`rm --forget` 对运行中的会话也会删除记录] → 终端上要求确认，非交互时必须 `--yes`。

## Migration Plan

与 `unify-session-surface` 在同一 PR 中实现与发布；先完成契约与服务端，再切换 CLI，最后重新生成手册。回滚即回退整个版本。
