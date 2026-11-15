# server-local-api Specification

## ADDED Requirements

### Requirement: 会话内进程不得控制官方 Relay

来自 WebMux 会话内进程的本机请求 SHALL 被禁止访问 `/api/relay` 及其子路径，包括读取配对材料、开始或轮询配对、修改设置、启停、重新绑定、删除绑定及申请/刷新 ICE/TURN 凭据，统一返回 403。
同等的控制面发现、数据面分配获取、端点/引导信任修改和凭据操作即使通过其他路由、别名或本机 API 提供，SHALL 使用相同限制；无论 ICE 配置接口放在 Relay 还是 P2P 路由，都不能绕过现有会话识别边界。

同一用户的非会话本机客户端和经过 Token 或有效 Relay 浏览器会话认证并通过 Host、Origin 与适用 CSRF 校验的浏览器 SHALL 按各自权限使用 Relay 接口；会话内进程的其他非敏感本机能力保持不变。
申请或刷新两端 TURN 凭据 SHALL 另要求由当前有效 GitHub 所有者登录派生的浏览器会话；本机客户端权限、WebMux Token、宿主设备身份或连接器 control 会话均不能单独满足该前提。
WebMux 宿主本地配置和 API MUST NOT 充当节点管理代理；节点配置与秘密管理属于独立控制服务的受限管理员 CLI/API，普通 GitHub 登录和宿主本机权限不授予节点管理员权限。

#### Scenario: 会话中的 agent 尝试开启 Relay

- **WHEN** 某个 WebMux 会话中的进程通过本机套接字请求开启 `/api/relay`
- **THEN** 返回 403，启用意图、配对状态和当前连接均不改变

#### Scenario: 非会话本机客户端查看 Relay 状态

- **WHEN** 同一用户的非会话进程通过本机套接字读取脱敏状态
- **THEN** 请求按本机身份规则成功，不包含私钥、配对码或任何 Token

### Requirement: 本机状态中的 Relay 字段

`/local/v1/status` 与 `webmux status --json` SHALL 在运行中的实例支持官方 Relay 时包含可选 `relay` 对象，字段至少包括期望 `enabled`、`paired`、连接 `state`、当前 `transport`、`bulkReady`、角色就绪数及脱敏 `reconnectReason`、就绪时的 `publicUrl` 和失败时的脱敏 `error`。
对象 SHALL 区分配置的 `controlEndpoint`/控制面 origin 与获认证分配的 `connectorUrl`/数据面 origin，显示各自真实来源、分配版本及独立可达性/信任状态；未探测或尚无分配时明确显示 unknown 或 unassigned，不要求用户配置第二个端点。
仅在与当前连接或获授权 ICE 有关时 SHALL 展示公开 nodeId、capabilities 和配置/分配版本，并区分 WSS 节点与可能独立的 TURN-only 节点；这些字段不是可编辑的节点管理状态。

仅 bulk 失败 SHALL 显示降级且保持交互；control 失败 SHALL 显示离线并撤销公共地址授权。

`transport` SHALL 表示固定 wss 基线；额外状态 SHALL 按浏览器会话区分交互实际路径 wss、webrtc-direct 或 webrtc-turn，以及脱敏探测/回落原因和 TURN 配置可用性，MUST NOT 把某一浏览器路径当作整个宿主连接器传输。

该字段 SHALL 作为状态契约 v1 的可选新增字段；未运行或不支持 Relay 的实例 SHALL 省略。
字段内容 MUST NOT 包含私钥、配对码、轮询句柄、GitHub 凭据、登录 Cookie、跳转 code、PKCE verifier、WebMux Token、TURN 长期密钥、短期用户名/密码、候选 IP 或凭据化 URL。
控制面加密主密钥、节点身份/服务 TLS 私钥、节点秘密下发内容及原始配置/秘密快照 MUST NOT 出现在本机状态、CLI 输出或脱敏错误中；响应 SHALL 仅使用安全 allowlist DTO。

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

#### Scenario: 会话进程尝试通过别名修改发现信任

- **WHEN** agent 从会话内通过 Relay 之外的等效路由修改控制面端点/引导信任或获取数据面分配
- **THEN** 返回 403，不改变配置、分配或已配对信任，不因路由不同绕过限制

#### Scenario: 本机身份不能代替浏览器所有者授权

- **WHEN** 非会话本机客户端仅凭本机身份或宿主已配对状态申请 TURN 凭据，没有有效 GitHub 所有者登录派生的浏览器会话
- **THEN** 请求被拒绝，不签发凭据；该客户端仍可按权限读取脱敏状态

#### Scenario: 分离部署的本机状态

- **WHEN** 控制面已认证并分配另一 origin 的 WSS 节点，而数据面握手失败，非会话本机客户端读取状态
- **THEN** 状态分别报告配置控制面可达和获分配数据面失败，显示来源、公开节点能力及版本，不把 TURN-only 节点当作 WSS 目标，也不输出任何节点或用户秘密
