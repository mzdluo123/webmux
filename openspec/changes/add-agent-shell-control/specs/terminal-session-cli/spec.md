# Spec Delta

## MODIFIED Requirements

### Requirement: 连接与错误约定

`run`、`attach`、`ls`、`open`，以及 `shell-session-control`、`shell-chat-control`、`shell-workspace-control`、`agent-shell-guide` 中的命令，
SHALL 经本机套接字访问本机服务。它们 SHALL NOT 需要或输出 Token，SHALL 接受 `--config <path>`，并按以下顺序定位服务：

1. 显式给出的 `--config`，按与服务相同的规则定位；
2. 环境变量 `WEBMUX_SOCKET` 指向的套接字；
3. 默认配置路径，按与服务相同的规则定位。

`WEBMUX_SOCKET` 已设置但不可连接时，SHALL 视为服务未运行，SHALL NOT 回退到默认配置。

接受会话 ID 的位置参数和选项 SHALL 接受 `self`，在客户端解析为环境变量 `WEBMUX_SESSION_ID` 的值。该变量未设置时 SHALL 以退出码 2 结束。

服务未运行时 SHALL 以退出码 3 结束并提示启动方式，SHALL NOT 隐式启动服务。诊断信息 SHALL 只写标准错误。

`webmux help` SHALL 按组列出命令：服务与运维、会话、聊天、工作区、工具说明。每个新命令 SHALL 提供 `--help`。

#### Scenario: 服务未运行

- **WHEN** 没有 WebMux 运行，用户执行 `webmux run -- claude`
- **THEN** 不创建任何进程，标准错误提示服务未运行与启动方式，退出码 3

#### Scenario: 会话内连回所属实例

- **WHEN** 服务以 `--config /srv/webmux.toml` 启动，会话内的 agent 不带 `--config` 执行 `webmux ls`
- **THEN** CLI 经 `WEBMUX_SOCKET` 连接这个实例，而不是默认配置对应的实例

#### Scenario: 套接字失效不回退

- **WHEN** `WEBMUX_SOCKET` 指向的服务已经退出，而默认配置对应另一个正在运行的实例
- **THEN** CLI 以退出码 3 结束，不访问另一个实例

#### Scenario: self 在会话外

- **WHEN** 用户在普通终端执行 `webmux get self`
- **THEN** 退出码 2，标准错误说明 `self` 只能在 WebMux 会话内使用
