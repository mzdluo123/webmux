# Proposal

## Why

用户希望用脚本或 CLI agent 控制 WebMux：例如让一个 agent 在新 worktree 里启动另一个 agent、发出任务、等它完成再读取结果，
或者在长任务结束时通知手机。

通道其实已经有了：本机 Unix 套接字挂着与浏览器相同的 `/api/*`、`/ws/*` 路由，靠对端 UID 认证，不需要 Token。
但 shell 侧还有三类缺口：

- **只有 REST、没有命令**：删除、重命名会话，启动配置，worktree，端口转发，混合会话聊天。用 curl 能做，
  但调用方要自己找到套接字、拼 JSON。
- **只有 WebSocket**：curl 做不了，这是真正卡住编排的地方。包括向会话输入并确认已写入、跟随输出、
  等待状态变化（agent 完成或需要介入）、订阅聊天事件。
- **发现机制**：会话只注入了 `WEBMUX_SESSION_ID`。服务端用非默认配置启动时，会话里的 `webmux` 可能找不到实例或连错实例。

另外，初始化指令和快捷指令里预留了 `{{webmux}}` 变量，目前写了就会被拒绝。agent 也没有途径得知 WebMux 提供了哪些 shell 工具。
临时文件夹说明只是一条预置快捷指令，没有和其他 WebMux 能力放在一起。

## What Changes

- **P0 会话控制命令**，全部放在 `internal/sessioncmd`，Linux/macOS CLI 与 Windows 控制台客户端都能用：
  - `webmux api`：通用逃生口，经本机套接字发送任意 `/api/` 请求，原样输出响应；
  - `webmux get`、`kill`、`rm`、`rename`：补齐会话生命周期；
  - `webmux send`：以带回执的提交帧写入文字和命名按键，可选 `--wait` 等待状态转变；
  - `webmux wait`：订阅会话列表事件，等到指定状态（running、waiting、idle、exited、attention）；
  - `webmux logs`：读取回放缓冲区，`--follow` 以观察者身份跟随，`--strip` 尽力去除控制序列；
  - `webmux events`：以 NDJSON 输出 `/ws/sessions` 事件；
  - `webmux dir`：输出会话临时文件夹的路径（按需创建）。
- **P0 发现与身份**：
  - 每个会话注入服务端自有变量 `WEBMUX_SOCKET`（本机 API 套接字路径）；
  - CLI 定位顺序：显式 `--config`，然后 `WEBMUX_SOCKET`，最后默认配置；
  - 所有接受会话 ID 的命令都接受 `self`，表示 `$WEBMUX_SESSION_ID`；
  - 新增退出码 4 表示等待超时。
- **P1 结构化编排**：
  - `webmux chat send|watch|status|cancel|answer|switch`：经 ACP 结构化通道控制混合会话，`chat send --wait` 输出 agent 的最终回复；
  - `webmux notify`：经新接口 `POST /api/sessions/{id}/attention` 发出提醒，不依赖控制终端；
  - `webmux worktree ls|create|rm` 与 `webmux run --worktree`；
  - `webmux launch-config ls|show`（只读）；
  - `webmux forward ls|add|rm`。
- **`{{webmux}}` 变为 WebMux 工具说明**：
  - 服务端把版本匹配的 shell 工具手册写成实例级文件 `SKILL.md`（skill 格式，带 frontmatter），`webmux guide` 输出同一份内容；
  - `{{webmux}}` 展开为一段简短的 skill 式说明：会话 ID、手册路径、何时需要读取手册，以及本会话临时文件夹的路径与一句强制规则；
  - 初始化指令和快捷指令都可以使用 `{{webmux}}`；
  - 现有临时文件夹说明（上传位置、交付物保存规则、在代码栏查看、汇报相对路径、会话删除后清理）移入手册的“临时文件夹”一节；
  - 预置快捷指令“临时文件夹”替换为“WebMux 工具”，正文为 `{{webmux}}`。已保存的列表不迁移。
- **明确不做**：代码视图、文件补全、mkdir、上传、推送订阅、设置项的专用命令（用 `webmux api` 或直接操作文件系统）；
  会话内读取 Token、控制 tunnel 或 Tailscale（保持现有禁止）；服务端渲染屏幕文本（违反“服务端不做终端模拟”）；MCP 服务端（后续可在这些命令上薄封装）。

## Capabilities

### New Capabilities

- `shell-session-control`：通用 API 命令、会话生命周期命令、`send`、`wait`、`logs`、`events`、`dir`、`notify`，以及共同的输出与退出码约定。
- `shell-chat-control`：混合会话聊天的 shell 命令。
- `shell-workspace-control`：worktree、启动配置（只读）与端口转发命令，以及 `run --worktree`。
- `agent-shell-guide`：shell 工具手册文件、`webmux guide`、手册接口、`{{webmux}}` 的展开规则和手册内容要求。

### Modified Capabilities

- `terminal-session-cli`：连接约定加入 `WEBMUX_SOCKET` 与 `self`，帮助列出新命令。
- `session-launch-environment`：服务端为每个会话设置 `WEBMUX_SOCKET`，并把它列为服务端自有变量。
- `terminal-quick-commands`：预置指令改为恢复会话、新会话、WebMux 工具三条；快捷指令正文支持 `{{webmux}}`。
- `session-artifacts`：移除已由快捷指令取代、现在再移入手册的“临时文件夹提示词模板”与“从快捷指令菜单插入提示词”两条需求。

## Impact

- 后端：
  - 新增共享包 `internal/agentguide`（仅标准库，嵌入手册模板并渲染简述），在 `internal/archtest/layers_test.go` 中登记为 Shared；
  - `internal/artifacts` 负责写入手册文件、清扫时跳过它，并提供 `Guide(id)`；
  - `internal/workspace` 在快速开始中展开 `{{webmux}}`，取消保留变量的拒绝；
  - `internal/session` 支持外部提醒；
  - `internal/api` 新增 `GET /api/agent-guide`、`POST /api/sessions/{id}/agent-guide`、`POST /api/sessions/{id}/attention`；
  - `cmd/webmux` 注入 `WEBMUX_SOCKET` 并注册新子命令。
- 接口：`internal/contract`、`internal/wire`、`web/src/schemas.ts` 新增 `AgentGuidePrompt`、`AttentionRequest`，
  `AttentionSource` 新增 `cli`；重新录制 `internal/api/testdata/wire`。
- 前端：`quickCommands.ts` 把 `{{webmux}}` 当作内置变量；`SessionTerminal` 与聊天草稿在插入前调用手册接口；
  初始化指令字段和快速开始页去掉“保留变量”提示；更换预置指令。
- 安全：不新增权限边界。会话内调用者已能经套接字做同样的事，现有四项禁止保持不变。
  控制指示器与提醒标明来源会话，手册写明操作守则。
- 文档：新增用户故事分组（shell 与 agent 控制），更新 US-Q13 与快捷指令、临时文件夹相关故事，更新索引与覆盖表。
