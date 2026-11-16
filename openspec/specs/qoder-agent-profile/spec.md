# qoder-agent-profile Specification

## Purpose

定义 Qoder 与 QoderCN 的档案识别、启动命令、参数分类、恢复命令与聊天配置推导，使终端和 ACP 聊天端使用匹配的原生会话身份。

## Requirements

### Requirement: Qoder 档案

系统 SHALL 内置 Qoder 档案（`Name: "qoder"`，`Title: "Qoder"`），
识别 `qoder` 与 `qodercli` 可执行文件及其版本化名称（`<可执行文件>-<后缀>`，例如 `qodercli-1.1.81`）。

- 终端信号：OSC 7501 根记录 `app` 为 `qoder` 或 `qoder-cli` 时识别为 Qoder，cwd 取根记录的 cwd；
  会话 ID 不来自终端信号，按 `agent-session-identity` 的候选判定取得。
- ACP 启动：`qoder --acp`（标志式，非子命令），原 argv 中的参数 SHALL NOT 透传。
- 恢复：`qoder --resume <uuid>`；中断：一次 Esc（`\x1b`）。
- 聊天菜单隐藏 `login`、`logout`；实测版本 1.1.81。
- 支持 `session/resume`；冷 load/resume 后 SHALL 重应用 `model` 与 `effort`；插话空闲行为为 `promptRequired`。

#### Scenario: Qoder 预置配置

- **WHEN** 配置的 argv 为
  `["bash","-ic","exec -- \"$@\"","webmux","qoder","--dangerously-skip-permissions"]`
- **THEN** 判定为可用，档案为 qoder

#### Scenario: Qoder 版本化可执行文件

- **WHEN** Qoder 混合会话的 PTY 根进程可执行文件为 `qodercli-1.1.81`
- **THEN** 该进程被视为 Qoder 进程

### Requirement: Qoder 恢复命令保留的模型与权限参数

Qoder 的恢复命令 SHALL 保留以下模型、推理、权限与调试参数：`-m`/`--model`、`--reasoning-effort`、
`--thinking`、`--thinking-budget`、`--context-window`、`--fallback-model`、`--max-output-tokens`、
`--max-model-request-retries`、`--permission-mode`，以及开关 `--dangerously-skip-permissions`/`--yolo`、
`-d`/`--debug`。

#### Scenario: Qoder 白名单参数进入恢复命令

- **WHEN** 混合配置的 argv 为 `["qoder","--model","gpt-4"]`，会话切回 TUI 恢复会话 `abc-123`
- **THEN** 恢复命令包含 `--model gpt-4` 与 `--resume abc-123`

### Requirement: Qoder 恢复命令保留的提示词、工具与配置参数

Qoder 的恢复命令 SHALL 保留以下参数：`--system-prompt`、`--append-system-prompt`、`--output-style`、
`--agents`、`--agent`、`--plugin-dir`、`--settings`、`--setting-sources`、`--mcp-config`、
`--allowed-mcp-server-names`、`--allowed-tools`、`--disallowed-tools`、`--tools`、`--add-dir`，
以及开关 `--strict-mcp-config`、`--bare`。

#### Scenario: Qoder 工具参数进入恢复命令

- **WHEN** 混合配置的 argv 为 `["qoder","--add-dir","/srv/lib","--strict-mcp-config"]`，会话切回 TUI 恢复会话 `abc-123`
- **THEN** 恢复命令包含 `--add-dir /srv/lib`、`--strict-mcp-config` 与 `--resume abc-123`

### Requirement: Qoder 丢弃的参数

系统 SHALL 在 Qoder 的恢复与 ACP 命令中丢弃位置参数（提示词）、`-c`/`--continue`、
`-r`/`--resume`（值可选）、`--attachment`、`-i`/`--prompt-interactive` 与 `-n`/`--name`，
这些参数 SHALL NOT 使配置不可用。

#### Scenario: Qoder 恢复参数被丢弃

- **WHEN** 配置的 argv 为 `["qoder","--resume","abc-123"]`
- **THEN** 判定为可用，档案为 qoder，`--resume abc-123` 不进入恢复命令与 ACP 命令

### Requirement: Qoder 打开会话、指定目录与改变会话库的参数

以下 Qoder 参数 SHALL 使配置不可用于混合模式：

- 打开已有会话（`session_arg`）：`--fork-session`、`--session-id`、`--from-pr`、`--remote-session`、
  `--remote-control`、`--remote`，以及首个位置参数为 `fork-session`；
- 指定工作目录（`directory_arg`）：`-w`/`--cwd`、`--worktree`；
- 使聊天视图无法找到会话（`session_store`）：`--config-dir`。

#### Scenario: Qoder 指定 worktree

- **WHEN** 用户对 argv 为 `["qoder","--worktree","feature"]` 的配置勾选混合模式
- **THEN** 保存返回 400，原因提示在新建会话时选择目录

### Requirement: Qoder 不启动交互界面的参数

以下 Qoder 参数 SHALL 使配置不可用于混合模式，原因为“该参数不启动交互界面”：
`-p`/`--print`、`-h`/`--help`、`-v`/`--version`、`--list-models`、`--list-sessions`、
`--no-session-persistence`、`-o`/`--output-format`、`--input-format`、`--delete-session`，
以及首个位置参数为子命令 `mcp`、`hooks`/`hook`、`rollback`、`update`、`config`、
`plugins`/`plugin`、`commit`、`agents`/`agent`、`skills`/`skill`、`feedback`、`security`、
`login`、`status`、`remote-control`、`sites`、`wiki` 的写法。

#### Scenario: Qoder 非交互子命令

- **WHEN** 用户对 argv 为 `["qoder","commit"]` 或 `["qoder","-p","fix bug"]` 的配置勾选混合模式
- **THEN** 保存返回 400，原因为该参数不启动交互界面

### Requirement: Qoder 的聊天端配置推导

首次在聊天端打开 Qoder 混合会话时，系统 SHALL 从 argv 推导配置种子：

- `-m`/`--model` → `model`；`--reasoning-effort` → `effort`；
- `--dangerously-skip-permissions` 或 `--yolo` → `mode=bypass_permissions`，且优先于 `--permission-mode`；
- `--permission-mode` 取值为 `default`、`accept_edits`/`acceptEdits`、`bypass_permissions`/`bypassPermissions`/`yolo`、
  `dont_ask`/`dontAsk`、`auto`、`plan` 之一时 SHALL 归一为 snake_case 写入 `mode`，其他取值 SHALL 列为不生效；
- 其余白名单参数 SHALL 在聊天视图列为“以下参数在聊天端不生效”。

#### Scenario: Qoder 权限模式别名

- **WHEN** 混合配置的 argv 为 `["qoder","--permission-mode","acceptEdits"]`，会话从聊天开始
- **THEN** 聊天配置种子包含 `mode=accept_edits`

#### Scenario: Qoder 未知权限模式

- **WHEN** 混合配置的 argv 为 `["qoder","--permission-mode","invalid"]`，会话从聊天开始
- **THEN** 不生成 `mode` 种子，`unsupported` 包含 `["--permission-mode invalid"]`

### Requirement: QoderCN 档案

系统 SHALL 内置 QoderCN 档案（`Name: "qodercn"`，`Title: "QoderCN"`），
识别 `qodercn`、`qoderclicn` 与 `qoder-cn` 可执行文件及其版本化名称
（例如 `qoderclicn-1.1.81`），SHALL NOT 把 `qoder` 或 `qodercli` 视为 QoderCN 进程。

- OSC 7501 根记录 `app` 为 `qodercn`、`qoder-cli-cn` 或 `qoderclicn` 时识别为 QoderCN。
- 其余行为（ACP 启动参数 `--acp`、恢复、中断、参数分类、聊天端配置推导、隐藏命令、
  实测版本与可选能力）SHALL 与 Qoder 档案相同。

#### Scenario: QoderCN 预置配置

- **WHEN** 配置的 argv 为
  `["bash","-ic","exec -- \"$@\"","webmux","qodercn","--dangerously-skip-permissions"]`
- **THEN** 判定为可用，档案为 qodercn

#### Scenario: QoderCN 由程序 id 识别

- **WHEN** OSC 7501 根记录的 `app` 为 `qoder-cli-cn`
- **THEN** 识别为 qodercn，cwd 取根记录的 cwd

#### Scenario: QoderCN ACP 启动

- **WHEN** 混合配置的 argv 为 `["qodercn","--model","qwen-max"]`，会话从聊天开始
- **THEN** ACP 进程的 argv 为 `["qodercn","--acp"]`
