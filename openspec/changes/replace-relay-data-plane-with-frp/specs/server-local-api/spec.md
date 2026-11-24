# Spec Delta

## MODIFIED Requirements

### Requirement: 本机状态中的 Relay 字段

旧数据模式的 `/local/v1/status` 与 `webmux status --json` SHALL 保留可选 `relay` 对象，字段至少包括期望 `enabled`、`paired`、连接 `state`、当前 `transport`、`bulkReady`、角色就绪数及脱敏 `reconnectReason`、就绪时的 `publicUrl` 和失败时的脱敏 `error`。
对象 SHALL 区分配置的 `controlEndpoint`/控制面 origin 与获认证分配的 `connectorUrl`/数据面 origin，显示各自真实来源、分配版本及独立可达性/信任状态；未探测或尚无分配时明确显示 unknown 或 unassigned，不要求用户配置第二个端点。
公开控制信任状态 SHALL 表示 CA-only/TLS 1.3 校验，MUST NOT 包含 `controlTrustPins` 或旧控制 pin 字段；分配 WSS 的 TLS/pin 与节点 mTLS/SPKI 策略不变。宿主受控文件的 v1→v2 迁移 MUST NOT 改变本机状态 API 契约 v1。
仅在与当前连接或获授权 ICE 有关时 SHALL 展示公开 nodeId、capabilities 和配置/分配版本，并区分 WSS 节点与可能独立的 TURN-only 节点；这些字段不是可编辑的节点管理状态。

旧模式仅 bulk 失败 SHALL 显示降级且保持交互；所有模式的 control 失败均 SHALL 撤销公共地址授权。
新分配 SHALL 在同一 v1 响应新增可选 `relayFrp`，与旧 `relay` 互斥。新对象保留期望启用、配对、固定地址、诊断与来源，返回 `dataPlane=frp-wss`、`controlReady`、`dataReady`，不返回旧角色计数或 `bulkReady`。
新消费者 SHALL 同时理解两种响应；旧消费者仍能读取通用访问 URL，无需消费新对象。未分配模式时沿用既有配对视图，不虚构已选择传输。

`transport` SHALL 表示固定 wss 基线；额外状态 SHALL 按浏览器会话区分交互实际路径 wss、webrtc-direct 或 webrtc-turn，以及脱敏探测/回落原因和 TURN 配置可用性，MUST NOT 把某一浏览器路径当作整个宿主连接器传输。

该字段 SHALL 作为状态契约 v1 的可选新增字段；未运行或不支持 Relay 的实例 SHALL 省略。
字段内容 MUST NOT 包含私钥、配对码、轮询句柄、GitHub 凭据、登录 Cookie、跳转 code、PKCE verifier、WebMux Token、TURN 长期密钥、短期用户名/密码、候选 IP 或凭据化 URL。
控制面加密主密钥、节点身份/服务 TLS 私钥、节点秘密下发内容及原始配置/秘密快照 MUST NOT 出现在本机状态、CLI 输出或脱敏错误中；响应 SHALL 仅使用安全 allowlist DTO。

#### Scenario: Relay 就绪

- **WHEN** Relay 已配对并就绪，客户端请求本机状态
- **THEN** 相应模式对象显示启用、已配对、ready 和无 Token 的固定公共地址

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

- **WHEN** agent 从会话内通过 Relay 之外的等效路由修改控制面端点/受众或获取数据面分配
- **THEN** 返回 403，不改变配置、分配或已配对信任，不因路由不同绕过限制

#### Scenario: 本机身份不能代替浏览器所有者授权

- **WHEN** 非会话本机客户端仅凭本机身份或宿主已配对状态申请 TURN 凭据，没有有效 GitHub 所有者登录派生的浏览器会话
- **THEN** 请求被拒绝，不签发凭据；该客户端仍可按权限读取脱敏状态

#### Scenario: 分离部署的本机状态

- **WHEN** 控制面已认证并分配另一 origin 的 WSS 节点，而数据面握手失败，非会话本机客户端读取状态
- **THEN** 状态分别报告配置控制面可达和获分配数据面失败，显示来源、公开节点能力及版本，不把 TURN-only 节点当作 WSS 目标，也不输出任何节点或用户秘密

#### Scenario: 新宿主与旧本机客户端

- **WHEN** 老客户端向采用 frp 的实例读取 v1 状态
- **THEN** 状态仍是有效 v1；客户端可忽略新增对象并使用通用访问链接，不收到形状不兼容的旧 relay 对象。新客户端只显示真实管理与回源状态。

#### Scenario: 会话进程申请业务授权

- **WHEN** 会话内进程或普通浏览器试图用本机状态接口获取隧道凭据或选择回源目标
- **THEN** 状态仅提供 allowlist 元数据；任何权限身份均不能通过状态导出凭据，宿主业务授权仅可从认证管理通道取得。
