# Spec Delta

## MODIFIED Requirements

### Requirement: 接口范围

本机套接字 SHALL 提供与 TCP 监听完全相同的 `/api/*` 与 `/ws/*` 接口（语义、请求与响应格式、WebSocket 帧均相同），并额外提供以下接口；`/local/*` SHALL NOT 在 TCP 监听上提供（返回 404）：

- `GET /local/v1/status`：返回状态对象，字段与 `cli-server-control` 的状态 JSON 契约 v1 相同（`record` 为 `true`），其中 `drift` 由服务端重新读取配置文件计算；
- `POST /local/v1/shutdown`：请求体为可选的 `{"force": bool}`；启动方为 `systemd` 或 `launchd` 且未设置 `force` 时 SHALL 返回 409 及对应的管理命令提示；否则 SHALL 返回 202，并按与收到 SIGTERM 相同的流程退出；
- `GET /local/v1/token`：配置了 Token 时返回 `{"token": "<Token>"}`，否则返回 404。

`/local/v1/*` 的响应 SHALL 为 JSON，错误使用现有 `{"error": "..."}` 格式；该接口版本内 SHALL 只增加可选字段，不兼容变更 SHALL 使用新的路径版本。
唯一例外是官方 Relay 业务传输整版切换时，`/local/v1/status` 与 `webmux status --json` SHALL 同步替换可选 `relay` 扩展内的旧池就绪字段，并移除仅用于旧池的 `state=degraded`；保留路径、状态 JSON 版本 v1 及其他状态字段语义。
该例外不适用于其他接口。服务端 SHALL NOT 在日志中记录 `/local/v1/token` 的响应内容。

#### Scenario: TCP 上不可用

- **WHEN** 客户端经 TCP 携带正确 Token 请求 `/local/v1/token`
- **THEN** 返回 404

#### Scenario: 本机停止

- **WHEN** 由用户手动启动的 WebMux 收到本机 `POST /local/v1/shutdown`
- **THEN** 返回 202，WebMux 清理会话后退出，运行时记录与套接字被删除

#### Scenario: systemd 实例拒绝停止

- **WHEN** 由 systemd 启动的 WebMux 收到不带 `force` 的本机停止请求
- **THEN** 返回 409，提示使用 `systemctl --user stop webmux`，服务继续运行

### Requirement: 本机状态中的 Relay 字段

`/local/v1/status` 与 `webmux status --json` SHALL 在运行中的实例支持官方 Relay 时包含单个可选 `relay` 对象。
对象 SHALL 保留期望 `enabled`、`paired`、连接 `state`、基线 `transport`、脱敏 `reconnectReason`、就绪时的 `publicUrl` 和失败时的脱敏 `error`，并报告 `controlReady` 和 `dataReady`。
MUST NOT 返回第二个 Relay 对象、`dataPlane`、`bulkReady`、角色就绪计数或旧池专用 `state=degraded`。
对象 SHALL 区分配置的 `controlEndpoint`/控制面 origin 与获认证分配的 `connectorUrl`/数据面 origin，显示各自真实来源、分配版本及独立可达性/信任状态；未探测或尚无分配时明确显示 unknown 或 unassigned，不要求用户配置第二个端点。
公开控制信任状态 SHALL 表示 CA-only/TLS 1.3 校验，MUST NOT 包含 `controlTrustPins` 或旧控制 pin 字段；分配 WSS 的 TLS/pin 与节点 mTLS/SPKI 策略不变。
仅在与当前连接或获授权 ICE 有关时 SHALL 展示公开 nodeId、capabilities 和配置/分配版本，并区分 WSS 节点与可能独立的 TURN-only 节点；这些字段不是可编辑的节点管理状态。

`controlReady` SHALL 反映当前管理流认证与租约有效性；`dataReady` SHALL 反映当前分配经网关验证的实际回源。只有两者就绪才公布 `publicUrl`；管理失效 SHALL 撤销公共地址及派生访问授权。frp 进程存活、单次 WSS 握手、注册成功或 TURN 可用均不能代替回源就绪。

`transport` SHALL 表示固定 wss 基线；额外状态 SHALL 按浏览器会话区分交互实际路径 wss、webrtc-direct 或 webrtc-turn，以及脱敏探测/回落原因和 TURN 配置可用性，MUST NOT 把某一浏览器路径当作整个宿主连接器传输。

未运行或不支持 Relay 的实例 SHALL 省略 `relay`。
字段内容 MUST NOT 包含私钥、配对码、轮询句柄、GitHub 凭据、登录 Cookie、跳转 code、PKCE verifier、WebMux Token、TURN 长期密钥、短期用户名/密码、候选 IP、frp JWT、桥接密钥或凭据化 URL。
控制面加密主密钥、节点身份/服务 TLS 私钥、回源私钥、节点秘密下发内容及原始配置/秘密快照 MUST NOT 出现在本机状态、CLI 输出或脱敏错误中；响应 SHALL 仅使用安全 allowlist DTO。

#### Scenario: Relay 就绪

- **WHEN** Relay 已配对并就绪，客户端请求本机状态
- **THEN** 单个 `relay` 显示启用、已配对、ready、`transport=wss`、两个就绪字段均为 true 以及无 Token 的固定公共地址

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

#### Scenario: 管理在线而业务未就绪

- **WHEN** 管理流仍有效，但实际 frp 回源探针失败或过期，客户端读取本机状态或 CLI JSON
- **THEN** 单个 `relay` 报告 `controlReady=true`、`dataReady=false` 与脱敏原因，不返回 `publicUrl` 或旧池状态；通用状态字段及 v1 版本不变

#### Scenario: 会话进程申请业务授权

- **WHEN** 会话内进程或普通浏览器试图用本机状态接口获取隧道凭据或选择回源目标
- **THEN** 状态仅提供 allowlist 元数据；任何权限身份均不能通过状态导出凭据，宿主业务授权仅可从认证管理通道取得。
