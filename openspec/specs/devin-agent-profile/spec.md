# devin-agent-profile Specification

## Purpose

定义 Devin CLI 的档案识别、启动命令与恢复参数，覆盖模型选择、会话库、目录和交互模式判定，使终端与 ACP 聊天端共享原生会话身份。

## Requirements

### Requirement: Devin CLI 档案

系统 SHALL 内置 Devin CLI 档案，稳定标识为 `devin`，显示名称为 `Devin CLI`，图标为 `devin`，实测版本为 `3000.12.57`。直接执行 `devin` 或使用既有 Bash/Zsh 启动文件包装的 `devin` 配置 SHALL 判定为支持混合模式，不需要第三方 ACP 连接器。

Devin 的 ACP 启动命令 SHALL 为 `devin acp`。TUI 恢复命令 SHALL 保留白名单参数并追加 `--resume <sessionId>`；中断键 SHALL 为一次 Esc（`\x1b`）。版本低于 `3000.12.57` 时 SHALL 按通用规则提示但不阻止切换。

Devin 的 `agentInfo.version` 实测为 `0.0.0-dev`，版本核对 SHALL 使用 `devin --version` 的输出，SHALL NOT 以 `agentInfo.version` 判定。

Devin 的 `session/resume` SHALL NOT 使用：其实测返回 `Method not found`，崩溃重连与切到聊天一律使用 `session/load` 回放。

#### Scenario: 预置的 bashrc 包装

- **WHEN** 配置的 argv 为 `["bash","-ic","exec -- \"$@\"","webmux","devin","--permission-mode","dangerous"]`
- **THEN** 判定为可用，`hybridSupport.agent` 为 `devin`，且没有连接器信息

#### Scenario: 恢复到 TUI

- **WHEN** Devin 混合会话切回 TUI，agent 会话 ID 为 `luminous-acorn`
- **THEN** 恢复命令保留白名单参数并追加 `--resume luminous-acorn`

#### Scenario: 崩溃重连走加载回放

- **WHEN** Devin 混合会话的聊天端进程异常退出后重连
- **THEN** 系统使用 `session/load` 回放会话，不调用 `session/resume`

### Requirement: Devin 恢复命令保留的参数

Devin 的恢复命令 SHALL 保留带值参数 `--model`、`--permission-mode`、`--verbosity`、可选值参数 `--respect-workspace-trust`、`--export`，以及开关 `--sandbox`。这些参数 SHALL 支持 `--flag value` 与 `--flag=value` 形式。

`--prompt-file`、`--` 之后的位置参数与未识别参数 SHALL 按通用规则丢弃。

#### Scenario: 白名单参数进入恢复命令

- **WHEN** 配置的 argv 为 `["devin","--permission-mode","dangerous","--model","swe-2-high","--sandbox"]`，会话切回 TUI 恢复会话 `calm-fjord`
- **THEN** 恢复命令依次包含这些白名单参数与 `--resume calm-fjord`

### Requirement: Devin 会话选择、目录与会话库参数

`-c`/`--continue` 与 `-r`/`--resume`（值可选）SHALL 使配置不可用于混合模式，原因码为 `session_arg`。

`--cloud` 与 `--config <path>` SHALL 使配置不可用于混合模式，原因为会话库或配置不一致。

首个位置参数为 PATH 时（`devin [PATH]...` 打开 Devin Desktop）SHALL 使配置不可用于混合模式，原因提示在新建会话时选择目录。

#### Scenario: 恢复参数不可用

- **WHEN** 用户对 argv 为 `["devin","--resume","old-session"]` 的配置勾选混合模式
- **THEN** 保存返回 400，原因码为 `session_arg`

#### Scenario: 云端模式不可用

- **WHEN** 用户对 argv 为 `["devin","--cloud"]` 的配置勾选混合模式
- **THEN** 保存返回 400，原因为该参数使聊天视图无法找到会话

#### Scenario: Desktop 路径参数不可用

- **WHEN** 用户对 argv 为 `["devin","/home/u/proj"]` 的配置勾选混合模式
- **THEN** 保存返回 400，原因提示在新建会话时选择目录

### Requirement: Devin 不启动交互界面的参数

`-p`/`--print`（可选值）、`-h`/`--help`、`-V`/`--version` SHALL 使配置不可用于混合模式，原因为“该参数不启动交互界面”。

首个位置参数为 `acp`、`auth`、`cloud`、`desktop`、`doctor`、`forward`、`help`、`list`、`ls`、`mcp`、`migrate`、`models`、`plugins`、`rm`、`rules`、`sandbox`、`setup`、`skills`、`ssh`、`uninstall`、`update` 或 `version` 时，SHALL 以相同原因拒绝混合模式。

#### Scenario: 非交互打印

- **WHEN** 用户对 argv 为 `["devin","-p","总结改动"]` 的配置勾选混合模式
- **THEN** 保存返回 400，原因为该参数不启动交互界面

#### Scenario: 会话管理子命令

- **WHEN** 用户对 argv 为 `["devin","list"]` 或 `["devin","rm","old-session"]` 的配置勾选混合模式
- **THEN** 保存返回 400，原因为该参数不启动交互界面
