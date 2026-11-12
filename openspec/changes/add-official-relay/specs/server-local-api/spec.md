# server-local-api Specification

## ADDED Requirements

### Requirement: 会话内进程不得控制官方 Relay

来自 WebMux 会话内进程的本机请求 SHALL 被禁止访问 `/api/relay` 及其子路径，包括读取配对材料、开始或轮询配对、修改设置、启停、重新绑定和删除绑定，统一返回 403。该限制 SHALL 与隧道和 P2P 控制采用相同的会话识别边界。

同一用户的非会话本机客户端和经过 Token、Host、Origin 校验的浏览器 SHALL 按各自权限使用 Relay 接口；会话内进程的其他非敏感本机能力保持不变。

#### Scenario: 会话中的 agent 尝试开启 Relay

- **WHEN** 某个 WebMux 会话中的进程通过本机套接字请求开启 `/api/relay`
- **THEN** 返回 403，启用意图、配对状态和当前连接均不改变

#### Scenario: 非会话本机客户端查看 Relay 状态

- **WHEN** 同一用户的非会话进程通过本机套接字读取脱敏状态
- **THEN** 请求按本机身份规则成功，不包含私钥、配对码或任何 Token

### Requirement: 本机状态中的 Relay 字段

`/local/v1/status` 与 `webmux status --json` SHALL 在运行中的实例支持官方 Relay 时包含可选 `relay` 对象，字段至少包括期望 `enabled`、`paired`、连接 `state`、就绪时的 `publicUrl` 和失败时的脱敏 `error`。

该字段 SHALL 作为状态契约 v1 的可选新增字段；未运行或不支持 Relay 的实例 SHALL 省略。字段内容 MUST NOT 包含私钥、配对码、轮询句柄、GitHub 凭据或 WebMux Token。

#### Scenario: Relay 就绪

- **WHEN** Relay 已配对并就绪，客户端请求本机状态
- **THEN** `relay` 显示启用、已配对、ready 和无 Token 的固定公共地址

#### Scenario: Relay 配对中

- **WHEN** Relay 正在等待用户完成 GitHub 授权
- **THEN** 状态只表示未就绪和配对中，不返回用户码或轮询句柄
