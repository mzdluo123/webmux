# Design

## Context

- 本机 API：`internal/api/local.go` 的 `NewLocal` 在 Unix 套接字上挂载与 TCP 相同的路由，外加 `/local/v1/*`。
  认证靠套接字对端 UID，不检查 Token、Host、Origin。会话内调用者（`InSession(pid)`）被禁止四件事：停服务、读 Token、控制 tunnel、改 Tailscale。
- 现有 CLI：`internal/sessioncmd` 实现 `run`、`attach`、`ls`、`open`，经 `localclient.Client` 访问套接字，
  Windows 控制台客户端通过 `webmux bridge` 复用同一套命令。
- 终端 WebSocket：`FrameInput` 与 `FrameSubmit` 都会调用 `s.Claim(sub)`，即“输入即夺取控制”。
  `FrameSubmit` 会得到 `SubmissionStatus` 回执（received、written、partial、failed、missing）。
- 状态：`SessionInfo.status`（running、waiting、idle、exited）与锁存的 `attention`。
  提醒按来源分层：`OSC 7501` 根记录高于 Warp 事件，高于 `OSC 9`/`777`，高于响铃。
- 快捷指令与初始化指令：`{{dir}}` 已能展开为会话临时文件夹，`{{user}}` 是首条消息的位置，`{{webmux}}` 保留但会被拒绝。
  预置快捷指令“临时文件夹”承载了临时文件夹说明。

## Goals / Non-Goals

**Goals**

- 脚本和 agent 只用 `webmux` 命令就能完成编排：创建、输入、等待、读取、结束会话，驱动混合会话聊天，准备 worktree，开端口转发，发出提醒。
- 输出可组合：机器可读输出用 JSON 或 NDJSON，数据只写标准输出，诊断只写标准错误，退出码稳定。
- agent 能按 skill 的方式发现这些工具：指令里只放一段简述，需要时再读取完整手册，手册与正在运行的服务版本一致。

**Non-Goals**

- 不新开 TCP 控制入口，远程控制用 `ssh host webmux …`。
- 不新增会话内权限限制，也不放宽现有限制。
- 不在服务端渲染屏幕文本，不为代码视图、上传、推送订阅、设置项做专用命令。
- 不实现 MCP 服务端。

## Decisions

### D1 命令放在 `sessioncmd`，传输只用本机 API

新命令与 `run`、`attach` 一样经 `Env.Connect` 获取 `localclient.Client`，Windows 客户端因此自动获得同样的命令。
任何命令都不读 Token，也不走 TCP。备选方案“在 TCP 上加控制 Token”被否决：
会多出一条需要保护的凭据，而远程场景用 SSH 加本地 CLI 已经足够。

### D2 `webmux api` 只开放 `/api/` 与 `/local/v1/`

`api` 是逃生口，让暂时没有专用命令的接口也能脚本化。路径必须以 `/api/` 或 `/local/v1/` 开头。
`/ws/` 路径以退出码 2 拒绝，并提示改用 `events`、`logs` 或 `chat watch`。
请求体只支持 JSON，multipart 上传不在范围内。方法默认 GET，给了 `-d` 时默认 POST，与 curl、`gh api` 的习惯一致。
服务端的会话内限制照常生效，CLI 不做额外判断。

### D3 发现：`WEBMUX_SOCKET` 优先于默认配置

服务端创建会话时设置 `WEBMUX_SOCKET=<本实例套接字>`，并把它加入 `contract.ServerOwnedEnv`，
因此 `run` 不会把调用方的值转发进新会话，创建请求也不能覆盖它。

CLI 的定位顺序是：显式 `--config`，然后 `WEBMUX_SOCKET`，最后默认配置。`WEBMUX_SOCKET` 指向的套接字不可达时，
CLI 以退出码 3 结束，不回退到默认配置。回退可能连到另一个实例，替调用方操作错误的会话。

`self` 在客户端解析为 `$WEBMUX_SESSION_ID`。变量未设置时以退出码 2 结束。

### D4 `send` 用提交帧，默认夺取控制

`send` 把文字作为一次 `FrameSubmit` 发出：正文是文字，提交键在 `--enter` 时为 `\r`，否则为空。
CLI 等到 `written` 才以 0 退出，`partial`、`failed`、`missing` 以 1 退出，从而知道输入确实进入了 PTY。
`--key` 指定的命名按键（`enter`、`esc`、`tab`、`ctrl-c`、`ctrl-d`、`up`、`down`、`left`、`right`、`backspace`）
在文字之后作为 `FrameInput` 发出。

控制权沿用“输入即夺取控制”：浏览器会变成观察者，控制指示器显示连接标签。
`send` 的标签固定为 `script`，在会话内运行时为 `script · <来源会话 ID>`，用户能看出是谁接管了终端。
“注入但不夺权”需要改动 hub 与 session 的控制模型，留作开放问题。

会话处于混合会话的聊天端或切换中时，`send` 以 1 退出，提示改用 `chat send`。

### D5 等待要避开“状态还没变”的竞态

`send` 之后立刻 `wait --until idle` 有竞态：agent 可能还没进入 running，`wait` 看到旧的 idle 就立即返回。设计两层防护：

- `wait --changed`：忽略订阅时的初始状态，只接受订阅之后发生的状态转变。
- `send --wait STATE`：先订阅 `/ws/sessions`，再提交输入，只接受提交之后的转变。推荐编排脚本使用这种方式。

`--stable DUR` 要求状态保持一段时间再返回，用于过滤 agent 在工具调用之间短暂的 idle。
超时以新退出码 4（`ExitTimeout`）结束。会话被移除时以 1 结束。等到 `exited` 时退出码为 0，`--json` 输出包含会话的 `exitCode`。

### D6 `logs` 默认输出原始字节

服务端不做终端模拟，`GET /buffer` 返回带控制序列的原始回放。`logs` 默认原样输出，可以交给 `less -R` 或其他工具。
`--strip` 在客户端尽力去除 CSI、OSC、DCS 和其他 ESC 序列，把 `\r\n` 规范为 `\n`，丢弃单独的 `\r` 之前被覆盖的同一行内容。
这对行式输出的 shell 足够，对全屏 TUI 不可靠，手册会引导改用 `chat`。

`--follow` 以 `attach --observe` 的同一观察者通路连接，不夺取控制、不改变尺寸，把输出写到非 TTY 的标准输出。

### D7 `notify` 走接口而不是控制终端

最初考虑过向 `/dev/tty` 写 `OSC 9`。但 agent 的工具子进程常常没有控制终端，而且 `OSC 7501` 报告过状态的会话会忽略 `OSC 9`，
Claude Code、Codex 正是这样，而它们恰恰是最常见的调用方。因此改为新接口
`POST /api/sessions/{id}/attention`，请求体为 `{kind, message?}`：

- `kind` 取 `notify`、`done`、`error`、`question`，来源记为新的 `cli`。
- CLI 提醒不参与来源分层，总是覆盖当前锁存的提醒，之后由原有规则清除（用户查看、`OSC 7501` 状态变化等）。
- `message` 最长 200 个字符，只用于推送通知正文和提醒提示，只保存在内存中，不写入磁盘，也不出现在会话列表的其他字段里。
  这与“终端输出里的正文从不保存”并不冲突，因为这段文字是调用方明确交给 WebMux 的。
- 默认目标是 `self`。

### D8 聊天命令复用聊天接口，`send --wait` 输出最终回复

`chat send` 调用 `POST .../chat/prompts`，`clientRequestId` 由 CLI 生成，附件先经会话上传接口上传再引用。
`--wait` 先连接 `/ws/s/{id}/chat`，再发送提示，跟踪对应 `promptId` 的回合：
回合结束时输出 agent 消息的最终文本（`--json` 时输出该回合的事件数组），按 `stopReason` 决定退出码：
`end_turn` 为 0，`cancelled` 与其他原因为 1。回合中出现权限或问题请求时，
默认以 1 退出并输出请求 ID，方便上层用 `chat answer` 处理。`--auto-wait-requests` 则一直等到用户在 Web 端回答。

### D9 手册：实例级 `SKILL.md`，内容随服务版本更新

- 内容源：`internal/agentguide` 用 `go:embed` 嵌入 `guide.md.tmpl`，渲染时填入服务版本。这是仅依赖标准库的共享包，CLI 与服务端都可以引用。
- 文件：服务启动时由 `internal/artifacts` 写到实例临时根下的 `guide/SKILL.md`（目录 0700，文件 0600），
  缺失时按需重写。清扫只处理以会话 ID 命名的目录，天然不会删除 `guide/`。实例临时根由状态目录的哈希决定，
  重启后路径不变，所以单一文件可以像 skill 一样从固定位置加载：用户把它链接到 `~/.claude/skills/webmux/SKILL.md` 后，升级会自动更新内容。
- 接口：`GET /api/agent-guide` 返回 Markdown 文本，`webmux guide` 优先读它，保证与运行中的服务一致；
  服务未运行时退回 CLI 内嵌的副本，并在标准错误说明。
- 格式：YAML frontmatter 带 `name: webmux` 与一句 `description`，正文分节：概览、何时使用、身份与连接、临时文件夹、
  常用流程、命令参考、退出码、操作守则。一个测试保证 `subcommands` 中的每个会话类命令都在手册中出现。

### D10 `{{webmux}}` 的简述与临时文件夹

`{{webmux}}` 展开为固定格式的简述，会话相关的值在展开时填入：

```text
本会话运行在 WebMux 中（会话 ID：<id>）。
- WebMux shell 工具手册：<guidePath>
  需要查看或控制其他会话、等待 agent、驱动混合会话聊天、准备 worktree、端口转发或发送提醒时，
  先完整阅读该手册，再按其中说明使用 `webmux` 命令；任务与此无关时不必读取。
- 本会话的临时文件夹：<dir>
  交付给我的截图、报告、导出数据等文件保存到这里而不是项目仓库，规则见手册“临时文件夹”一节。
```

临时文件夹说明原先是一条完整的预置快捷指令，现在只在简述里保留路径和一句强制规则，其余移入手册。这样做的原因：

- 交付物规则对几乎每个任务都适用，留一句在简述里，agent 即使不读手册也不会把文件放进仓库；
- 上传位置、在代码栏查看、汇报相对路径、会话删除后清理这些细节只在需要时读取，与 skill 的渐进加载一致；
- `{{dir}}` 仍然可用，用户可以继续写自己的临时文件夹说明。

简述由 `artifacts.Service.Guide(id)` 生成。它确保临时文件夹存在，确保手册文件存在，再返回 `{prompt, guidePath, dir}`。
快速开始和快捷指令都经它展开，服务端和浏览器的文字因此一致。

### D11 安全

会话内的 agent 本来就能经套接字结束、操控其他会话。同一个 UID 不构成安全边界（agent 可以直接 `kill` 进程），
所以本变更不增加新的禁止项。做三件事降低误用：

- 控制指示器和 CLI 提醒标明来源会话；
- 手册的“操作守则”要求：只操作自己创建或用户点名的会话；不要结束用户的会话；
  不要读取或输出 Token；不要向已开启自动批准的 agent 转发未经核实的外部内容；
- 服务端不记录 `send` 的内容或 `logs` 的输出，沿用“从不记录终端输入输出”。

## Risks / Trade-offs

- `send` 夺取控制会打断正在浏览器里输入的用户。缓解：标签标明来源，手册要求只操作自己创建的会话。
- `--strip` 对 TUI 输出不可靠。缓解：在手册与帮助中写明，并引导使用 `chat`。
- 手册写在系统临时目录下，可能被系统定时清理。缓解：`Guide(id)` 每次展开前检查并重写。
- 命令数量明显增加。缓解：所有命令共用一套参数解析、连接和输出约定，帮助文字按组列出。

## Open Questions

- 是否需要“注入但不夺权”的 `send --no-claim`？需要 hub 区分控制者与注入者，暂不实现。
- `notify` 是否应该在 Windows 托盘显示？托盘目前只转发推送，本变更不涉及。
