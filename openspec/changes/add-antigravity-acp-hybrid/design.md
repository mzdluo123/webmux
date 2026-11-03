# Design

## Context

动机见 `proposal.md - Why`。本设计基于对官方 Antigravity CLI 文档（`antigravity.google/docs`）及本机实测 `agy 1.4.12`（`agy --help`、`agy acp --help`、ACP JSON-RPC stdio 握手与 PTY 交互会话）的核对结果：

1. **ACP 协议与会话能力**：
   - `agy acp` 原生启动 ACP v1 stdio 服务，`initialize` 声明 `loadSession: true` 以及 `sessionCapabilities: {resume: {}, close: {}, load: {}, list: {}, additionalDirectories: {}}`，`agentInfo` 为 `{name: "Antigravity ACP Server", version: "1.4.12"}`。
   - `session/new`、`session/load`、`session/resume` 返回标准 `configOptions`（`model` 与 `thought_level`），并发送 `available_commands_update`。
   - `agy` TUI 与 `agy acp` 共享同一个本地会话库（默认位于 `~/.gemini/antigravity-cli`）：TUI 创建的对话可由 `agy acp` 的 `session/list` 列出并经 `session/load` 回放历史；`agy acp` 创建或保存的对话可由 `agy --conversation <id>` 在 TUI 中恢复。
2. **命令行参数差异**：
   - `agy` TUI 支持 `--dangerously-skip-permissions`、`--sandbox`、`--agent`、`--add-dir`、`--log-file`、`--model`、`--effort`、`--mode` 等交互参数，以及 `--conversation <id>` 恢复指定会话。
   - `agy acp` 使用 Cobra 校验子命令参数，接受 `--dangerously-skip-permissions`、`--sandbox`、`--agent`、`--add-dir`、`--log-file`，但不接受 `--model`、`--effort`、`--mode`（模型与推理等级通过 ACP `configOptions` 控制）。
   - `--project` 与 `--new-project` 会将会话切入云端项目模式，忽略 `--conversation` 本地会话恢复。
3. **TUI 状态与会话身份信号**：
   - 在应答 `ESC ] 7501 ; ? BEL` 的终端中，`agy` TUI 原生发出 `OSC 7501` 根记录（`state=idle:app=antigravity`、`state=working:app=antigravity`、`state=done:app=antigravity:msg=...`、`state=blocked:kind=permission:app=antigravity` 等），但根记录不包含当前对话的 UUID。
   - `agy` 官方支持会话级 `--settings <json>`（只对当次会话生效，不写回磁盘 `settings.json`），其中 `title` 命令会在每次状态迁移（`authenticating`、`initializing`、`idle`、`working`、`tool_use`、`reviewing`、`error`）时执行，并通过标准输入接收包含 `conversation_id`（36 字符 UUID）、`cwd`、`agent_state`、`version`、`product: "antigravity"` 的 JSON。
   - 当 `title` 命令不向标准输出打印内容而将 `ESC ] 7501 ; state=working:id=<conversation_id> BEL` 写入 `/dev/tty` 时，既不会改动终端窗口标题，又能让 WebMux 的 `OSCScanner` 采集到当前对话 ID。

## Goals / Non-Goals

**Goals:**
- 在不改变 `internal/hybrid` 状态机与单写者编排的前提下，通过扩展 `internal/agentprofile` 支持 `agy` 混合会话（聊天 ↔ 终端双向切换、空闲休眠唤醒、异常重连与重启恢复）。
- 让从 TUI 启动或在 TUI 中执行 `/new`、`/fork`、`/resume` 的 `agy` 混合会话准确上报并跟踪 36 字符 UUID 会话 ID。
- 将新安装预置的 `Gemini CLI` 启动配置更新为默认开启混合模式的 `Antigravity CLI`（`agy --dangerously-skip-permissions`），并提供对应的 `antigravity` 图标，同时保证升级前含 `geminicli` 图标的 `launch-configs.json` 零迁移兼容。

**Non-Goals:**
- 不支持 `agy` 云端项目会话（`--project`、`--new-project`）的混合模式切换。
- 不修改已有用户磁盘上的 `launch-configs.json`。

## Decisions

### 1. 参数透传采用按参数标记而非全量开关

- **做法**：在 `internal/agentprofile` 的 `option` 结构中增加 `passToACP bool`（保留 `Profile.passToACP` 作为全量默认）。对 `agy`：
  - `--dangerously-skip-permissions`、`--sandbox`、`--agent`、`--add-dir`、`--log-file` 标记为保留且透传给 `agy acp`；
  - `--model`、`--effort`、`--mode` 标记为仅在 `ResumeArgv` 中保留、不透传给 `agy acp`，其中 `--mode` 列入 `acpNotice`（聊天端 `unsupported` 提示），而 `--model` 与 `--effort` 对应聊天端的 `model` 与 `thought_level` 配置项，无需列为不支持。
- **备选方案**：像 `OpenCode` 一样完全不向 `agy acp` 透传参数。未采用，因为 `--sandbox`、`--agent`、`--add-dir` 会直接影响工具执行边界与 agent 选择，两端不一致会导致切换后行为漂移。

### 2. 通过注入会话级 `--settings` + `/dev/tty` `OSC 7501` 子记录上报 TUI 会话 ID

- **做法**：
  1. `agy` 档案配置 `tuiInject` 为 `["--settings", <JSON>]`，其中 `title.command` 从标准输入提取 `"conversation_id":"<uuid>"`，非空时向 `/dev/tty` 写入 `\033]7501;state=working:id=<uuid>\007`，且不向 stdout 输出任何字符。原命令中的 `--settings` 归为 `argDrop`，避免覆盖该钩子。
  2. 将 `internal/session/osc.go` 中的 `maxProgramIDSegment` 从 `32` 调整为 `64`（总长上限 `maxProgramID = 128` 与最多 8 级保持不变），使 36 字符的标准 UUID 能通过 `programIDFirstSegment` 校验并写入 `Identity.ProgramChildID`。
  3. `agentprofile.Identify` 在 `s.ProgramApp == "antigravity"` 时返回 `Agent: "agy"`，当 `s.ProgramChildID` 为合法的 36 字符小写 UUID 时作为 `AgentSessionID`，`Cwd` 取 `s.RootCwd`。
- **备选方案**：
  - *方案 A：通过扫描 `/proc/<pid>/fd` 提取打开的 `conversations/<uuid>.db`*。未采用：平台相关性强，macOS 需额外调用 `proc_pidinfo` 遍历文件描述符，且不如终端带内信号统一。
  - *方案 B：让 `title.command` 把 UUID 打到 stdout 放入窗口标题 `OSC 2`*。未采用：会污染用户看到的终端标题，且 `OSC 2` 原本用于展示会话标题而非内部身份信号。

### 3. 启动配置图标新增 `antigravity` 并保留 `geminicli` 兼容

- **做法**：
  - 从固定提交 `eb3e353a5cf3720ef83d52058966ed51b10f10fa` 的 `lobehub/lobe-icons` 引入 `web/src/icons/lobehub/antigravity.svg`。
  - 新安装预置配置改为 `{Name: "Antigravity CLI", Icon: "antigravity", Cmd: bashrcCommand("agy", "--dangerously-skip-permissions"), Hybrid: true}`。
  - 前端图标选择器展示 `Antigravity CLI`（值 `antigravity`），自动匹配对可执行文件 `agy` 返回 `antigravity`，对 `gemini` 仍返回 `geminicli`；后端 `launch.normalize` 与前端 `LaunchConfigIcon.svelte` 同时保留 `geminicli` 与 `antigravity`，确保已有 `launch-configs.json` 不因未知图标值而返回 503。

## Risks / Trade-offs

- **用户在混合会话中自定义 `--settings` 被丢弃** → `--settings` 归入 `argDrop`，切换结果明确列出未保留的参数；长期设置仍可通过 `~/.gemini/antigravity-cli/settings.json` 配置（命令行 `--settings` 只覆盖内联提供的 `title` 键）。
- **`title` 命令依赖 POSIX `sed` 与 `/dev/tty`** → WebMux 仅支持 Linux 与 macOS，两者的 `/bin/sh` 与 `sed` 均支持该基本正则，且 `session` 启动 PTY 时始终设置 `Setsid: true, Setctty: true`，`/dev/tty` 恒可用；即使重定向失败也有 `2>/dev/null || true` 兜底，不影响 TUI 运行。
- **已有安装的 `launch-configs.json` 不会自动把 `Gemini CLI` 改名为 `Antigravity CLI`** → 遵循 `launch-configs` 规格“已存在的 `launch-configs.json` 不被改写”的原则，老用户可在设置页自行编辑或新建。

## Migration Plan

- 无需数据迁移：`launch-configs.json` 保持 v2，保留 `geminicli` 合法性；新安装自动获得 `Antigravity CLI` 混合启动配置。
- 回滚：若需回退到不支持 `agy` 的旧版本，先在设置页取消 `agy` 启动配置的混合模式勾选并将图标改回旧版支持的值，避免旧版校验 `launch-configs.json` 时报错 503。
