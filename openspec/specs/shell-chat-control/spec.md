# shell-chat-control Specification

## Purpose

让脚本和 agent 不打开浏览器也能驱动混合会话的聊天端：发送提示并取回回复、处理授权与提问、切换端、管理队列与运行时设置，以及恢复或清理历史会话。

## Requirements

### Requirement: 聊天命令的共同约定

`webmux chat <子命令>` SHALL 只包含聊天视图专有的操作：`status`、`watch`、`answer`、`set`、`permission` 与 `queue …`，并遵守 `shell-session-control` 的输出与退出码约定。
发送输入、中断、切换视图、列出、恢复与删除记录对所有会话通用，见 `shell-session-control` 与 `terminal-session-cli`；创建会话见 `shell-session-launch`。

- 目标会话没有聊天视图（`views` 不含 `chat`，或服务端返回 409 `no_chat_view`）时 SHALL 以 1 结束并说明原因。
- 需要聊天视图的子命令在会话处于终端视图时 SHALL 以 1 结束，并提示 `webmux view SESSION chat`。
- 每个写操作 SHALL 生成新的 `clientRequestId`，同一次调用内的重试 SHALL 复用它，使服务端去重生效。

#### Scenario: 普通终端会话

- **WHEN** 用户对一个 bash 会话执行 `webmux chat status <ID>`
- **THEN** 退出码 1，标准错误说明该会话没有聊天视图

#### Scenario: 会话在终端视图

- **WHEN** 带聊天视图的会话处于终端视图，用户执行 `webmux chat permission <ID> auto`
- **THEN** 退出码 1，标准错误提示 `webmux view <ID> chat`

### Requirement: 观察聊天

- `webmux chat watch SESSION [--json] [--history]` SHALL 连接聊天 WebSocket，直到被中断：
  - `--json` 时 SHALL 把每个服务端帧作为一行 JSON 输出；
  - 默认 SHALL 跳过连接时回放的历史事件，`--history` 时包含它们；
  - 默认 SHALL 以文本形式输出用户与 agent 消息、工具调用标题与状态，以及回合开始和结束。
- `webmux chat status SESSION [--json]` SHALL 输出一次快照后退出，内容包括当前视图（取自会话信息的 `view` 与 `transition`）、当前回合状态、队列长度和待回答请求列表（ID、类型、摘要、选项）。

#### Scenario: 列出待回答请求

- **WHEN** agent 正在等待授权，用户执行 `webmux chat status <ID> --json`
- **THEN** 输出中包含该请求的 ID 与可选的 `optionId`

### Requirement: 控制回合与请求

- `webmux chat answer SESSION REQUEST_ID (--option ID | --content JSON | --decline | --cancel)` SHALL 调用回答接口：
  授权请求用 `--option` 选择选项；问题表单用 `--content` 提交字段值对象、`--decline` 拒绝；`--cancel` 对两者都以取消作答。
  选项与请求类型不符时 SHALL 以 2 结束；请求已被回答或不存在时 SHALL 以 1 结束。
  （`--json` 在所有命令中都是输出开关，因此结构化内容用 `--content`。）
- 结束当前回合使用 `webmux interrupt SESSION`，切换视图使用 `webmux view SESSION terminal|chat`（见 `shell-session-control`）。

#### Scenario: 代为批准一次

- **WHEN** 用户执行 `webmux chat answer <ID> <请求 ID> --option allow_once`
- **THEN** agent 继续执行，所有设备上的请求卡片同时关闭

#### Scenario: 切到聊天后发送

- **WHEN** 会话在终端视图，脚本先执行 `webmux view <ID> chat`，再执行 `webmux send <ID> 继续`
- **THEN** 切换完成后提示被发送，`webmux attach` 中的客户端按 US-Q13 被动分离

### Requirement: 运行时配置与队列

- `webmux chat set SESSION ID=VALUE...` SHALL 对每一项调用 `PUT .../chat/config/{configId}`：
  select 类型取 `VALUE`，boolean 类型接受 `true`/`false`。任一项失败时 SHALL 报告失败的项并以 1 结束，已成功的项不回滚。
- `webmux chat permission SESSION ask|auto` SHALL 调用 `PUT .../chat/permission-mode`。
- `webmux chat queue ls SESSION [--json]` SHALL 从聊天快照列出队列：序号、ID、文字摘要。
- `webmux chat queue rm SESSION QID`、`send-now SESSION QID`、`clear SESSION`、`resume SESSION` SHALL 分别调用队列的删除、立即发送、清空与恢复接口；
  `edit SESSION QID [TEXT|-] [--up]` SHALL 调用修改接口（`--up` 上移一位）。子命令名在会话 ID 之前，与其他命令一致。
- 这些写操作 SHALL 立即同步到所有设备。

#### Scenario: 切换模型

- **WHEN** 用户执行 `webmux chat set <ID> model=sonnet`
- **THEN** agent 的模型配置变为该值，Web 端聊天视图的配置菜单同步显示

#### Scenario: 清空排队的提示

- **WHEN** 队列中有三条提示，脚本执行 `webmux chat queue clear <ID>`
- **THEN** 队列清空，所有设备不再显示这些提示

### Requirement: CLI 等待显式输出未完成工作

CLI 等待 SHALL 在现有 turnResult JSON 对象增加顶层 unfinished boolean，取自共享观察的真实标记，不从回复推断。
既有 stopReason、error、reply、events、request SHALL 保持原结构和语义，不增加伪事件或替换 envelope。
文本 SHALL 保留回复，unfinished 为 true 时在 stderr 提示后台工作未完成。
end_turn SHALL 仍退出 0，即使 unfinished 为 true；其他退出映射全部不变。unfinished 与独立业务验收 SHALL NOT 混为一谈。
命令入口 SHALL 依据当前命令表，不恢复旧 chat send/switch。

#### Scenario: 正常回合仍有后台工作

- **WHEN** 目标回合 end_turn 且 unfinished 为 true
- **THEN** JSON 顶层 unfinished 为 true，events 不变，文本 stderr 提示后台工作未完成，退出码仍为 0

#### Scenario: 正常结束没有后台工作

- **WHEN** 观察的 unfinished 为 false
- **THEN** JSON 仍显式输出 false，不根据任务是否经过验收改写，也不输出后台未完成警告

#### Scenario: 超时或待请求

- **WHEN** 等待超时或被当前 pending request 阻塞
- **THEN** 分别保持既有 124 或 1 退出映射，不取消执行、批准请求或重复发送操作
