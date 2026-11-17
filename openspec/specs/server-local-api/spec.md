# server-local-api Specification

## Purpose

为同一用户的本机客户端（Linux/macOS CLI、经 WSL 桥接的 Windows 托盘与控制台客户端）提供不开放网络端口、按操作系统身份认证的本机 API：复用现有 HTTP/WebSocket 接口，并增加只在本机可用的状态、停止与取 Token 接口，使所有客户端只使用一种协议。

## Requirements

### Requirement: 本机套接字监听

服务端 SHALL 在开始接受 TCP 请求之前，于运行时子目录中创建 Unix 域套接字并监听本机 API；套接字文件权限 SHALL 为 0600，所在目录 SHALL 为 0700。只有持有运行时锁的实例 SHALL 创建该套接字；创建前遗留的同名套接字文件 SHALL 被删除。服务端正常退出时 SHALL 在会话清理完成后删除套接字文件。

套接字路径超过平台长度上限时，服务端 SHALL 改用 `$XDG_RUNTIME_DIR` 下由状态目录派生的固定路径；仍不可用时 SHALL 照常提供 TCP 服务、在日志中警告“本机 API 不可用”，并在运行时记录中留空套接字路径。

#### Scenario: 默认启动

- **WHEN** 用户以默认配置启动 WebMux
- **THEN** 运行时子目录中出现 0600 的套接字，运行时记录包含其路径；服务退出后套接字文件不存在

#### Scenario: 遗留套接字

- **WHEN** 上一次 WebMux 被 SIGKILL 结束，遗留了套接字文件，用户再次启动
- **THEN** 新实例删除旧文件并正常监听，客户端可以连接

### Requirement: 本机身份认证

本机套接字上的每个连接 SHALL 在处理任何请求之前校验对端进程的有效用户与服务端进程的有效用户相同；不同时 SHALL 立即关闭连接且不返回任何数据。

通过校验的连接 SHALL NOT 再要求 Token，SHALL NOT 执行 Host 与 Origin 校验（浏览器无法连接 Unix 域套接字）。TCP 监听的认证、Host 与 Origin 校验 SHALL 保持不变。

#### Scenario: 同一用户免 Token

- **WHEN** WebMux 配置了 Token，同一用户执行 `curl --unix-socket <套接字> http://webmux/api/sessions`
- **THEN** 返回会话列表，无需 Token

#### Scenario: 其他用户被拒绝

- **WHEN** 另一个系统用户（例如通过放宽的目录权限）连接该套接字
- **THEN** 连接被立即关闭，没有任何响应数据

### Requirement: 接口范围

本机套接字 SHALL 提供与 TCP 监听完全相同的 `/api/*` 与 `/ws/*` 接口（语义、请求与响应格式、WebSocket 帧均相同），并额外提供以下接口；`/local/*` SHALL NOT 在 TCP 监听上提供（返回 404）：

- `GET /local/v1/status`：返回状态对象，字段与 `cli-server-control` 的状态 JSON 契约 v1 相同（`record` 为 `true`），其中 `drift` 由服务端重新读取配置文件计算；
- `POST /local/v1/shutdown`：请求体为可选的 `{"force": bool}`；启动方为 `systemd` 或 `launchd` 且未设置 `force` 时 SHALL 返回 409 及对应的管理命令提示；否则 SHALL 返回 202，并按与收到 SIGTERM 相同的流程退出；
- `GET /local/v1/token`：配置了 Token 时返回 `{"token": "<Token>"}`，否则返回 404。

`/local/v1/*` 的响应 SHALL 为 JSON，错误使用现有 `{"error": "..."}` 格式；该接口版本内 SHALL 只增加可选字段，不兼容变更 SHALL 使用新的路径版本。服务端 SHALL NOT 在日志中记录 `/local/v1/token` 的响应内容。

#### Scenario: TCP 上不可用

- **WHEN** 客户端经 TCP 携带正确 Token 请求 `/local/v1/token`
- **THEN** 返回 404

#### Scenario: 本机停止

- **WHEN** 由用户手动启动的 WebMux 收到本机 `POST /local/v1/shutdown`
- **THEN** 返回 202，WebMux 清理会话后退出，运行时记录与套接字被删除

#### Scenario: systemd 实例拒绝停止

- **WHEN** 由 systemd 启动的 WebMux 收到不带 `force` 的本机停止请求
- **THEN** 返回 409，提示使用 `systemctl --user stop webmux`，服务继续运行

### Requirement: 会话内进程的限制

服务端 SHALL 识别来自 WebMux 会话内部的本机连接（对端进程属于某个会话的进程树，或在 Linux 上携带会话标识环境变量）。对这类连接，`POST /local/v1/shutdown` 与 `GET /local/v1/token` SHALL 返回 403；其余接口（包括创建会话、列出会话、附着终端）SHALL 照常可用。

macOS 无法读取其他进程环境，识别 SHALL 以进程树为准，并接受已脱离会话进程树的后台进程无法被识别这一限制。

#### Scenario: 会话中的 agent 取 Token

- **WHEN** WebMux 会话中运行的程序请求本机 `/local/v1/token`
- **THEN** 返回 403

#### Scenario: 会话中启动兄弟会话

- **WHEN** WebMux 会话中运行的程序通过本机套接字请求 `POST /api/sessions`
- **THEN** 新会话按正常规则创建

### Requirement: 隧道控制接口

本机 API SHALL 提供 `POST /local/v1/tunnel`，请求体 `{"enabled": bool}`：`true` 开启、`false` 关闭隧道，成功时返回 202 与当前隧道状态对象。请求体无效时 SHALL 返回 400。该接口 SHALL 只在本机套接字上可用，TCP 监听上 SHALL 返回 404。

#### Scenario: 开启请求

- **WHEN** 同一用户向本机套接字发送 `POST /local/v1/tunnel` 与 `{"enabled": true}`，Token 满足要求
- **THEN** 返回 202，响应体中 `state` 为 `starting` 或 `ready`

#### Scenario: TCP 上不可用

- **WHEN** 持有 Token 的浏览器请求 TCP 监听上的 `/local/v1/tunnel`
- **THEN** 返回 404

### Requirement: 状态中的隧道字段

`/local/v1/status` 与 `webmux status --json` SHALL 在实例支持运行时隧道控制时包含 `tunnel` 对象，其字段为：

- `state`：`off`、`starting`、`ready`、`failed` 之一。
- `publicUrl`：`ready` 时给出，不含 Token。
- `error`：`failed` 时给出。
- `named`：是否 Named Tunnel。

`tunnel` SHALL 为可选新增字段，契约版本保持 v1。不支持运行时隧道控制的旧实例和未运行的实例 SHALL 省略该字段。

#### Scenario: 隧道就绪

- **WHEN** 隧道 `ready`，客户端请求状态
- **THEN** `tunnel` 为 `{"state":"ready","publicUrl":"https://abc.trycloudflare.com","named":false}`，任何字段都不含 Token

#### Scenario: 实例未运行

- **WHEN** WebMux 未运行时执行 `webmux status --json`
- **THEN** 输出不含 `tunnel` 字段

### Requirement: 状态中的推送字段

`/local/v1/status` 与 `webmux status --json` SHALL 在运行中的实例支持注意力推送时包含 `push` 对象，字段为：

- `enabled`：配置是否启用推送；
- `available`：推送存储是否可用；
- `error`：仅不可用时出现，说明原因；
- `subscriptions`：已登记订阅数；
- `publicOrigin`：仅配置了 Named Tunnel 时出现，为 `https://<tunnel.hostname>`。

该字段 SHALL 为可选新增字段，契约版本保持 v1；实例未运行或不支持推送时 SHALL 省略。字段内容 MUST NOT 含订阅 endpoint、订阅密钥、VAPID 私钥或任何 Token。

#### Scenario: Named Tunnel 实例

- **WHEN** 实例配置了 `tunnel.hostname = "term.example.com"` 与隧道凭据，已登记 1 个订阅，客户端请求本机状态
- **THEN** `push` 为 `{"enabled":true,"available":true,"subscriptions":1,"publicOrigin":"https://term.example.com"}`

#### Scenario: 未配置 Named Tunnel

- **WHEN** 实例只使用 Quick Tunnel 或未启用隧道
- **THEN** `push` 中没有 `publicOrigin` 字段

#### Scenario: 实例未运行

- **WHEN** WebMux 未运行时执行 `webmux status --json`
- **THEN** 输出不含 `push` 字段

### Requirement: 会话内进程不得控制官方 Relay

来自 WebMux 会话内进程的本机请求 SHALL 被禁止访问 `/api/relay` 及其子路径，包括读取配对材料、开始或轮询配对、修改设置、启停、重新绑定、删除绑定及申请/刷新 ICE/TURN 凭据，统一返回 403。
同等的控制面发现、数据面分配获取、端点/受众修改和凭据操作即使通过其他路由或本机 API 提供，SHALL 使用相同限制；不提供旧公开控制 pin 配置别名，无论 ICE 接口放在 Relay 还是 P2P 路由，都不能绕过现有会话识别边界。

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
公开控制信任状态 SHALL 表示 CA-only/TLS 1.3 校验，MUST NOT 包含 `controlTrustPins` 或旧控制 pin 字段；分配 WSS 的 TLS/pin 与节点 mTLS/SPKI 策略不变。宿主受控文件的 v1→v2 迁移 MUST NOT 改变本机状态 API 契约 v1。
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

- **WHEN** agent 从会话内通过 Relay 之外的等效路由修改控制面端点/受众或获取数据面分配
- **THEN** 返回 403，不改变配置、分配或已配对信任，不因路由不同绕过限制

#### Scenario: 本机身份不能代替浏览器所有者授权

- **WHEN** 非会话本机客户端仅凭本机身份或宿主已配对状态申请 TURN 凭据，没有有效 GitHub 所有者登录派生的浏览器会话
- **THEN** 请求被拒绝，不签发凭据；该客户端仍可按权限读取脱敏状态

#### Scenario: 分离部署的本机状态

- **WHEN** 控制面已认证并分配另一 origin 的 WSS 节点，而数据面握手失败，非会话本机客户端读取状态
- **THEN** 状态分别报告配置控制面可达和获分配数据面失败，显示来源、公开节点能力及版本，不把 TURN-only 节点当作 WSS 目标，也不输出任何节点或用户秘密
