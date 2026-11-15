# server-local-api Specification

## ADDED Requirements

### Requirement: 会话内进程不得控制官方 Relay

来自 WebMux 会话内进程的本机请求 SHALL 被禁止访问 `/api/relay` 及其子路径，包括读取配对材料、开始或轮询配对、修改设置、启停、重新绑定、删除绑定及申请/刷新 ICE/TURN 凭据，统一返回 403。无论 ICE 配置接口放在 Relay 还是 P2P 路由，该限制 SHALL 同样生效，不能通过别名绕过现有会话识别边界。

同一用户的非会话本机客户端和经过 Token 或有效 Relay 会话认证并通过 Host、Origin 与适用 CSRF 校验的浏览器 SHALL 按各自权限使用 Relay 接口；会话内进程的其他非敏感本机能力保持不变。

#### Scenario: 会话中的 agent 尝试开启 Relay

- **WHEN** 某个 WebMux 会话中的进程通过本机套接字请求开启 `/api/relay`
- **THEN** 返回 403，启用意图、配对状态和当前连接均不改变

#### Scenario: 非会话本机客户端查看 Relay 状态

- **WHEN** 同一用户的非会话进程通过本机套接字读取脱敏状态
- **THEN** 请求按本机身份规则成功，不包含私钥、配对码或任何 Token

### Requirement: 本机状态中的 Relay 字段

`/local/v1/status` 与 `webmux status --json` SHALL 在运行中的实例支持官方 Relay 时包含可选 `relay` 对象，字段至少包括期望 `enabled`、`paired`、连接 `state`、当前 `transport`、`bulkReady`、角色就绪数及脱敏 `reconnectReason`、就绪时的 `publicUrl` 和失败时的脱敏 `error`。

仅 bulk 失败 SHALL 显示降级且保持交互；control 失败 SHALL 显示离线并撤销公共地址授权。

`transport` SHALL 表示固定 wss 基线；额外状态 SHALL 按浏览器会话区分交互实际路径 wss、webrtc-direct 或 webrtc-turn，以及脱敏探测/回落原因和 TURN 配置可用性，MUST NOT 把某一浏览器路径当作整个宿主连接器传输。

该字段 SHALL 作为状态契约 v1 的可选新增字段；未运行或不支持 Relay 的实例 SHALL 省略。字段内容 MUST NOT 包含私钥、配对码、轮询句柄、GitHub 凭据、登录 Cookie、跳转 code、PKCE verifier、WebMux Token、TURN 长期密钥、短期用户名/密码、候选 IP 或凭据化 URL。

#### Scenario: Relay 就绪

- **WHEN** Relay 已配对并就绪，客户端请求本机状态
- **THEN** `relay` 显示启用、已配对、ready 和无 Token 的固定公共地址

#### Scenario: Relay 配对中

- **WHEN** Relay 正在等待用户完成 GitHub 授权
- **THEN** 状态只表示未就绪和配对中，不返回用户码或轮询句柄

#### Scenario: 会话进程申请 TURN 凭据

- **WHEN** agent 从会话内经本机套接字请求任一 ICE 配置或凭据刷新入口
- **THEN** 返回 403，不签发凭据，不改变当前浏览器连接

#### Scenario: TURN 升级后的本机状态

- **WHEN** 某浏览器已升级到 TURN，非会话本机客户端读取状态
- **THEN** 基线 transport 仍是 wss，该浏览器交互路径显示 webrtc-turn，状态不含 ICE 候选地址或任何 TURN 认证材料
