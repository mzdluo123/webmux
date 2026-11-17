# shell-session-control Specification

## Purpose

让脚本和 agent 用 `webmux` 命令控制已有会话：发送输入、等待状态、读取输出与画面、订阅事件、结束与改名、发出提醒，以及访问没有专用命令的接口。

## Requirements

### Requirement: 共同的输出与退出码约定

本能力的所有命令 SHALL 经本机 API 访问服务，并遵守 `terminal-session-cli` 的连接约定。

- 数据 SHALL 只写标准输出，诊断 SHALL 只写标准错误。
- 提供 `--json` 的命令 SHALL 输出服务端契约类型的 JSON；流式命令 SHALL 输出 NDJSON，每行一个完整对象。
- 退出码：0 成功；1 服务端拒绝或操作失败；2 用法错误；3 服务未运行；124 等待超时（与 GNU `timeout` 一致）。退出码 124 SHALL 作为 `contract.ExitTimeout` 加入契约。
- 接受会话 ID 的参数 SHALL 接受 `self`。
- 这些命令 SHALL NOT 记录或回显 Token，服务端 SHALL NOT 记录 `send` 的内容或 `logs` 的输出。

#### Scenario: 管道组合

- **WHEN** 脚本执行 `webmux get self --json | jq -r .session.cwd`
- **THEN** 标准输出只有会话 JSON，`jq` 输出当前会话的启动目录

#### Scenario: 用法错误

- **WHEN** 用户执行 `webmux kill` 而不给会话 ID
- **THEN** 标准错误输出用法，退出码 2，不发送任何请求

### Requirement: 通用 API 命令

`webmux api [-X METHOD] PATH [-d DATA|@FILE|-]` SHALL 经本机 API 发送一次 HTTP 请求，并把响应体原样写到标准输出。

- `PATH` SHALL 以 `/api/` 或 `/local/v1/` 开头，可以带查询串；其他路径（包括 `/ws/`）SHALL 以退出码 2 拒绝，
  `/ws/` 的提示 SHALL 指向 `events`、`logs` 与 `chat watch`。
- 方法默认 GET；给了 `-d` 时默认 POST。`-d` 的值可以是字面 JSON、`@文件` 或 `-`（标准输入），
  SHALL 在发送前校验为 JSON，不是 JSON 时以退出码 2 结束。
- 2xx 响应 SHALL 以 0 退出；其他状态 SHALL 把服务端错误信息写到标准错误并以 1 退出，响应体仍写到标准输出。
- 服务端对会话内调用者的现有限制 SHALL 照常生效。

#### Scenario: 删除启动配置

- **WHEN** 用户执行 `webmux api -X DELETE /api/launch-configs/abc`
- **THEN** CLI 发送 DELETE，服务端返回 204 后退出码为 0

#### Scenario: 会话内读取 Token

- **WHEN** 会话内的 agent 执行 `webmux api /local/v1/token`
- **THEN** 服务端返回 403，标准错误给出拒绝原因，退出码 1

#### Scenario: WebSocket 路径

- **WHEN** 用户执行 `webmux api /ws/sessions`
- **THEN** 退出码 2，提示改用 `webmux events`

### Requirement: 会话生命周期命令

系统 SHALL 提供以下命令（读取元数据的 `webmux get` 见 `session-metadata`）：

- `webmux kill SESSION`：结束会话进程（`DELETE /api/sessions/{id}`），会话以已结束状态保留。
- `webmux rm SESSION [--forget] [--yes]`：移除会话（`DELETE /api/sessions/{id}?remove=1`），运行中的会话先被结束；带聊天视图的会话保留归档。
  - `--forget` SHALL 改为调用 `DELETE /api/sessions/{id}?forget=1`，同时删除归档记录与临时文件夹；对 `active` 的带聊天视图会话先关闭再删除。
    标准输入是终端且没有 `--yes` 时，SHALL 要求确认；非交互又没有 `--yes` 时，SHALL 以 2 结束并提示需要 `--yes`。
  - 不带 `--forget` 对已归档（`closed` 或 `interrupted`）的会话执行时，SHALL 以 1 结束，并提示使用 `--forget` 删除记录。
- `webmux rename SESSION NAME`：重命名会话（`PATCH /api/sessions/{id}`），带聊天视图的会话同时更新记录名称。

会话不存在时 SHALL 以退出码 1 结束；切换中拒绝结束时 SHALL 输出服务端原因并以 1 结束。

#### Scenario: 清理自己创建的会话

- **WHEN** 脚本先用 `webmux run --detach -- make test` 得到 ID，结束后执行 `webmux rm <ID>`
- **THEN** 会话从列表消失，所有浏览器随即收到移除事件

#### Scenario: 非交互删除记录需要确认

- **WHEN** 脚本以管道执行 `webmux rm <已归档会话 ID> --forget`
- **THEN** 不删除记录，退出码 2，提示需要 `--yes`

#### Scenario: 已归档会话需要 --forget

- **WHEN** 用户对一个已中断的会话执行 `webmux rm <ID>`
- **THEN** 退出码 1，标准错误提示改用 `webmux rm <ID> --forget`

### Requirement: 向会话发送输入

`webmux send SESSION [TEXT|-] [--enter] [--key NAME]... [--attach-file PATH]... [--mode steer] [--wait STATE | --reply] [--auto-wait-requests] [--timeout DUR] [--json]`
SHALL 调用 `POST /api/sessions/{id}/input`，由服务端按会话当前视图分派，CLI SHALL NOT 为此连接终端 WebSocket。

- `-` 表示从标准输入读取全部文字。文字、`--key` 与 `--attach-file` 都没有时 SHALL 以退出码 2 结束。
- `--key` 可重复，取值为 `enter`、`esc`、`tab`、`ctrl-c`、`ctrl-d`、`up`、`down`、`left`、`right`、`backspace`，未知名称 SHALL 以退出码 2 拒绝。
- 终端视图：
  - 文字 SHALL 作为一次带回执的提交写入：正文为文字，`--enter` 时 `submit` 为 true（追加提交键），否则不追加；`--key` SHALL 在文字之后按顺序写入。
  - 服务端回执为 `written` 时以 0 退出；`partial`、`failed`、`missing` 以 409 `input_failed` 答复，CLI SHALL 以 1 退出并报告状态。
  - 写入 SHALL 夺取控制：服务端以 `script` 为控制者标签，从会话内调用时为 `script · <来源会话 ID>`。
  - `--attach-file` 或 `--mode` SHALL 被服务端以 409 `wrong_view` 拒绝，CLI 以 1 退出并报告原因，不写入任何字节。
- 聊天视图：
  - 文字 SHALL 作为提示发送，语义同 Web 端发送；`--attach-file` 的文件 SHALL 先经会话上传接口上传，再作为附件引用，任一上传失败时 SHALL 不发送并以 1 结束。
  - `--mode steer` SHALL 映射为 `mode: "steer"`，缺省为 `auto`。`--enter` SHALL 没有额外作用（提示总是提交）。
  - `--key` SHALL 被服务端以 409 `wrong_view` 拒绝，CLI 以 1 退出并报告原因。
  - 不带 `--reply` 与 `--wait` 时，SHALL 在服务端接受后以 0 退出；`--json` 时输出输入结果（`view`、`queued`、`itemId`、`turnId`、`promptId`）。
    立即发送时结果为 `turnId` 与 `promptId`（与 `turn.start` 一致），排队时为 `itemId`，之后发送它的回合 SHALL 在 `turn.start` 中带同一个 `queueItemId`。
- 会话切换中（409 `switching`）或不是 `active`（409 `not_active`）时 SHALL 以 1 结束并报告原因。
- `--wait STATE` SHALL 对两种视图都适用：在发送前订阅会话列表，只接受发送之后的状态转变，语义同 `wait --changed`。
- `--reply` SHALL 只用于聊天视图，与 `--wait` 互斥（同时给出时以 2 结束）。会话在终端视图时 SHALL 不写入任何内容，以 1 结束并提示改用 `--wait STATE`。
  带 `--reply` 时，SHALL 在发送前连接 `/ws/s/{id}/chat`，跟踪本次输入开启的回合直到结束（`--mode steer` 跟踪正在进行的回合）：
  - 默认输出该回合 agent 消息的最终文本，`--json` 时输出该回合的事件数组；
  - `stopReason` 为 `end_turn` 时以 0 退出，其他原因以 1 退出，并在标准错误报告原因；
  - 回合中出现权限或问题请求时，SHALL 在标准错误输出请求 ID 与摘要并以 1 退出；
    带 `--auto-wait-requests` 时则继续等待，直到请求在其他客户端被回答；
  - 排队的提示在发送前被删除时 SHALL 以 1 退出；
  - 超时以 124 退出，SHALL NOT 取消回合。

#### Scenario: 让 shell 执行命令并等待

- **WHEN** 脚本执行 `webmux send <ID> 'make test' --enter --wait idle --timeout 10m`
- **THEN** 命令写入 PTY，CLI 在会话从 running 回到 idle 后以 0 退出，超时则以 124 退出

#### Scenario: 中断 agent

- **WHEN** 用户对终端视图的会话执行 `webmux send <ID> --key esc`
- **THEN** 会话收到一个 ESC 字节，浏览器控制指示器显示 `script`

#### Scenario: 让另一个 agent 完成任务并取回结果

- **WHEN** 编排脚本对聊天视图的会话执行 `webmux send <ID> "修复 lint 并汇报改动" --reply --timeout 20m`
- **THEN** 回合正常结束后，标准输出是 agent 的最终回复，退出码 0

#### Scenario: 排队

- **WHEN** 聊天视图的回合进行中再执行 `webmux send <ID> "下一步" --json`
- **THEN** 提示进入服务端队列，输出的结果中 `queued` 为 true，所有设备看到同一队列

#### Scenario: 遇到权限请求

- **WHEN** `--reply` 期间 agent 请求执行命令的权限
- **THEN** CLI 输出请求 ID 并以 1 退出，回合保持等待

#### Scenario: 混合会话在聊天端

- **WHEN** 目标会话处于聊天视图，用户执行 `webmux send <ID> --key esc`
- **THEN** 聊天中没有新提示，退出码 1，标准错误报告 `wrong_view`

#### Scenario: 终端视图上的 --reply

- **WHEN** 目标会话处于终端视图，用户执行 `webmux send <ID> hi --reply`
- **THEN** 不写入任何内容，退出码 1，提示改用 `--wait STATE`

### Requirement: 等待会话状态

`webmux wait SESSION... --until STATE[,STATE...] [--changed] [--stable DUR] [--any] [--timeout DUR] [--json]`
SHALL 订阅 `/ws/sessions`，在会话满足条件时退出。

- `STATE` 取 `running`、`waiting`、`idle`、`exited`、`attention`。`attention` 表示会话有锁存的提醒。多个取值为“任一满足”。
- 不带 `--changed` 时，订阅时的快照已满足就立即返回；带 `--changed` 时，SHALL 忽略快照状态，只接受之后的事件。
- `--stable DUR` SHALL 要求条件持续满足 DUR，期间条件被打破则重新计时。
- 多个会话默认全部满足才返回，`--any` 时任一满足即返回。
- 满足时以 0 退出，`--json` 时输出满足条件的 `SessionInfo`（多个会话时为数组）。超时以 124 退出。
  会话被移除以 1 退出。会话已结束而条件不含 `exited` 时以 1 退出。
- 连接断开时 SHALL 重新连接并以新快照继续判断，SHALL NOT 因断线误报满足。

#### Scenario: 等 agent 需要介入

- **WHEN** 脚本执行 `webmux wait <ID> --until attention,idle --changed --timeout 30m`
- **THEN** agent 请求授权或本轮结束时 CLI 以 0 退出

#### Scenario: 已经结束

- **WHEN** 会话已经以退出码 2 结束，用户执行 `webmux wait <ID> --until exited --json`
- **THEN** 立即以 0 退出，输出的 JSON 中 `exitCode` 为 2

### Requirement: 读取会话输出

`webmux logs SESSION [--follow] [--strip] [--tail N]` SHALL 输出会话的终端输出。

- 默认 SHALL 输出 `GET /api/sessions/{id}/buffer` 的原始字节。
- `--strip` SHALL 在客户端去除 CSI、OSC、DCS 与其他 ESC 序列，把 CRLF 规范为 LF，并按单独的 CR 丢弃同一行被覆盖的内容。帮助 SHALL 说明它对全屏 TUI 不可靠。
- `--tail N` SHALL 只输出最后 N 行，按换行计数，与 `--strip` 同用时按去除后的文本计数；它与 `--follow` 互斥。
- `--follow` SHALL 以观察者身份连接终端 WebSocket：先输出回放，再持续输出新输出，SHALL NOT 夺取控制或改变尺寸。
  会话结束时 SHALL 在标准错误报告退出码并以 0 退出。
- 输出 SHALL 不受标准输出是否为终端的影响。

#### Scenario: 读取最近输出

- **WHEN** 脚本执行 `webmux logs <ID> --strip --tail 50`
- **THEN** 标准输出是最后 50 行不含控制序列的文本

#### Scenario: 跟随不夺权

- **WHEN** 浏览器正在控制会话，用户执行 `webmux logs <ID> --follow`
- **THEN** 浏览器仍是控制者，CLI 持续输出新输出

### Requirement: 订阅会话事件

`webmux events [--session ID]...` SHALL 连接 `/ws/sessions`，把每条事件原样作为一行 JSON 输出，直到被中断。

- 给了 `--session` 时，SHALL 只输出与这些会话有关的 `created`、`updated`、`removed` 事件，快照只保留这些会话。
- 断线时 SHALL 重连，并输出一条新的快照事件。

#### Scenario: 交给 jq 过滤

- **WHEN** 用户执行 `webmux events | jq -c 'select(.type=="updated") | .session.status'`
- **THEN** 每次会话状态更新输出一行状态

### Requirement: 输出临时文件夹路径

`webmux dir SESSION` SHALL 调用确保接口（`POST /api/sessions/{id}/artifacts`），并输出会话临时文件夹的绝对路径，不带引号。临时文件夹不可用时 SHALL 以 1 结束。

#### Scenario: 交付文件

- **WHEN** agent 执行 `cp report.html "$(webmux dir self)/"`
- **THEN** 报告出现在 WebMux 代码栏的“临时文件夹”根中

### Requirement: 发出提醒

`webmux notify [--session SESSION] [--kind KIND] [MESSAGE]` SHALL 经 `POST /api/sessions/{id}/attention` 让会话产生提醒；
`webmux notify --clear [--session SESSION]` SHALL 经 `DELETE /api/sessions/{id}/attention` 清除会话当前锁存的提醒。

- 会话默认为 `self`；不在会话内又没给 `--session` 时 SHALL 以退出码 2 结束。
- `KIND` 取 `notify`（默认）、`done`、`error`、`question`。
- 服务端 SHALL 以来源 `cli` 锁存该提醒，SHALL 覆盖当前锁存的提醒而不受来源分层限制，之后按原有规则清除。
- `MESSAGE` 去除首尾空白后 SHALL 不超过 200 个字符，超出时 SHALL 以 400 拒绝（CLI 在本地以 2 拒绝）。
  服务端 SHALL 只在内存中保存它，并只用于推送通知正文与提醒提示。
- 推送 SHALL 遵循现有推送规则。
- 清除 SHALL 与用户输入清除提醒的效果相同：不论提醒来源，都清除锁存值与 CLI 消息，并经 `/ws/sessions` 同步。会话没有提醒时 SHALL 返回 204。
  `--clear` 与 `--kind`、`MESSAGE` SHALL 互斥。
- 混合会话在聊天端时，CLI 提醒 SHALL 保留到 agent 报告新的状态（新的提醒或开始新一轮）。
- 推送正文 SHALL 与会话标题一样受 `include_title` 控制：为 `false` 时使用固定正文。
- `send` 写入的输入 SHALL 与用户输入一样清除提醒。

#### Scenario: 长任务完成

- **WHEN** 会话内执行 `make release; webmux notify --kind done "release 构建完成"`
- **THEN** 会话显示“本轮完成”提醒，已订阅推送的设备收到正文为“release 构建完成”的通知

#### Scenario: agent 已报告状态

- **WHEN** Claude Code 已经通过 `OSC 7501` 报告工作中，它的工具子进程执行 `webmux notify --kind question "需要确认迁移方案"`
- **THEN** 会话显示“需要回答”提醒，来源为 `cli`；之后 Claude Code 报告新的状态时按原规则清除

#### Scenario: 脚本处理完后清除提醒

- **WHEN** 编排脚本用 `chat answer` 处理完子会话的权限请求后，执行 `webmux notify --clear --session <ID>`
- **THEN** 子会话的提醒消失，所有设备上的徽章同步清除

### Requirement: 读取当前屏幕

`webmux screen SESSION [--cursor] [--json]` SHALL 输出会话终端当前画面的文本，供 agent 查看全屏 TUI 的状态。

- CLI SHALL 以观察者身份连接终端 WebSocket，取得回放快照与控制者的网格尺寸（`ControlState` 的 `cols`、`rows`），
  在客户端用终端模拟器按该尺寸重放，得到当前可见画面后断开连接。SHALL NOT 夺取控制或改变尺寸。服务端 SHALL NOT 为此做终端模拟。
- SHALL 输出纯文本：每行去掉行尾空白，末尾的空行省略。本变更不输出颜色与样式。
- `--cursor` SHALL 在标准错误输出光标的行与列（从 1 开始）。`--json` SHALL 输出 `{cols, rows, cursor: {row, col}, altScreen, stable, lines}`。
- 回放到达后，CLI SHALL 在 150 ms 无输出时即返回（`stable` 为 true；会话已退出同样视为稳定），否则最多再等 1 秒就按已收到的数据输出，
  此时 `stable` 为 false，并在标准错误提示画面仍在变化；退出码 SHALL 仍为 0。
- 回放被截断（`StreamPosition.truncated`）时，SHALL 在标准错误提示画面可能不完整。这种情况通常不影响全屏 TUI：它们会重绘整个画面。
- 会话处于聊天端时 SHALL 以 1 结束，并提示使用 `webmux chat status`。
- 终端模拟复用服务端预览已在使用的 `xterm-go/vt`，在 CLI 进程内运行；服务端 SHALL NOT 为此新增任何模拟。

#### Scenario: 看到 TUI 的权限提示

- **WHEN** Claude Code 在终端端停在一个权限选择框，编排者执行 `webmux screen <ID>`
- **THEN** 标准输出是与浏览器所见一致的画面文本，包含选择框的选项，浏览器仍是控制者

#### Scenario: 结构化读取

- **WHEN** 脚本执行 `webmux screen <ID> --json | jq -r '.lines[-1]'`
- **THEN** 输出画面最后一行的文本

### Requirement: 中断会话

`webmux interrupt SESSION` SHALL 调用 `POST /api/sessions/{id}/interrupt`，由服务端按当前视图分派：聊天视图取消进行中的回合，终端视图写入档案定义的中断键或 Ctrl-C。

- 聊天视图没有进行中的回合时 SHALL 以 0 退出。
- 会话切换中或不是 `active` 时 SHALL 以 1 结束并报告服务端原因。

#### Scenario: 取消聊天回合

- **WHEN** 聊天视图的会话有回合进行中，用户执行 `webmux interrupt <ID>`
- **THEN** 回合以取消结束，退出码 0

#### Scenario: 中断 shell 命令

- **WHEN** 普通 shell 会话正在运行 `sleep 100`，用户执行 `webmux interrupt <ID>`
- **THEN** 终端收到 Ctrl-C，`sleep` 结束，退出码 0

### Requirement: 切换会话视图

`webmux view SESSION terminal|chat [--force] [--agent-session ID]` SHALL 调用 `POST /api/sessions/{id}/view`（`to` 为给定视图），并等待切换结束。

- 需要用户选择时（终端 TUI 在中断超时内没有空闲），带 `--force` 时 SHALL 选择强制切换；
  不带 `--force` SHALL 选择取消切换，并以 1 结束、提示可用 `--force` 重试，不让切换停在等待选择的状态。
- 服务端返回会话歧义时 SHALL 列出候选会话，并接受 `--agent-session ID` 重试；`--agent-session` 只在切到 `chat` 时有效，否则以 2 结束。
- 会话没有聊天视图（409 `no_chat_view`）时 SHALL 以 1 结束并说明原因。
- 每次调用 SHALL 生成新的 `clientRequestId`，同一次调用内的重试 SHALL 复用它。

#### Scenario: 切到聊天后继续

- **WHEN** 会话在终端视图，脚本先执行 `webmux view <ID> chat`，再执行 `webmux send <ID> 继续`
- **THEN** 切换完成后提示被发送，`webmux attach` 中的客户端按 US-Q13 被动分离

#### Scenario: 普通会话

- **WHEN** 用户对一个 bash 会话执行 `webmux view <ID> chat`
- **THEN** 退出码 1，标准错误说明该会话没有聊天视图

### Requirement: 恢复已归档会话

`webmux resume SESSION [--view terminal|chat] [--agent-session ID] [--without-env] [--detach]` SHALL 调用 `POST /api/sessions/{id}/resume`，只对已归档（`closed` 或 `interrupted`）的会话有效。

- `--view` 缺省为会话归档时的视图。
- 成功后行为与 `run` 相同：以终端视图恢复且有终端时附着，否则输出会话 ID。
- 启动配置已删除、需要确认缺失的环境变量时，不带 `--without-env` SHALL 以 1 结束并说明原因，带上则以 `confirmWithoutEnv` 重试。
- 服务端报告会话歧义时，SHALL 列出候选并接受 `--agent-session` 重试。
- 会话仍为 `active`（409 `not_archived`）时 SHALL 以 1 结束并说明原因。

#### Scenario: 恢复昨天中断的 agent

- **WHEN** 用户执行 `webmux ls --archived`，再执行 `webmux resume <ID> --view chat --detach`
- **THEN** 同一 agent 会话以聊天视图恢复，会话以原 ID 回到运行中列表，CLI 输出会话 ID

#### Scenario: 运行中的会话

- **WHEN** 用户对一个运行中的 shell 会话执行 `webmux resume <ID>`
- **THEN** 退出码 1，标准错误报告 `not_archived`
