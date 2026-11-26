# Design

## Context

动机见 [proposal.md](proposal.md)。现有节点与宿主分别维护 WSS/smux 业务连接池、逐流协议和重连；本变更仅替换这条 TCP 业务路径。节点与宿主已有独立的 WSS/smux 管理连接，承载宿主持钥证明、浏览器会话授权、ICE 和撤权；该承载与现有消息语义均保留。

浏览器的固定 HTTPS Origin、宿主绑定和公网网关保持原样。控制面仍是分配和撤权的唯一权威；节点与宿主各自核对经过认证且未失效的授权，不从来路、Cookie 或 frp 注册结果推断控制面状态。浏览器到节点的业务协议不变，frp 只用于节点到宿主的基线回源；WebRTC 仍可直连或经 TURN。

## Goals / Non-Goals

**Goals:**

- 让单个上游 frp TCP 代理承担每个活动 scope 的字节传输，同时保留 WebMux 管理、回源认证、资源准入和撤权能力。
- 区分管理与数据故障，以实际回源探测判断 `dataReady`；整版切换和回滚期间始终只有一个公网路由权威。

**Non-Goals:**

- 不改变管理 WSS/smux 线协议、WebRTC 直连或 TURN 的凭据、端口与选路策略；不增加公网应用入口、用户自定义 frp 配置、浏览器 frp 客户端或 UDP 出站要求。
- 不把 frp 认证当作浏览器授权，不提供新旧传输协商、兼容分流、旧业务回退或旧池参数；不承诺断开的 HTTP 请求或 WebSocket 可无缝迁移。

## Decisions

### Topology and trust boundaries

| 路径 | 身份与用途 |
| --- | --- |
| 浏览器 → 节点网关，既有 HTTPS/443 | 原固定 Origin，GitHub 登录、一次性 code、宿主 Cookie 及应用授权语义不变；浏览器从不访问 frp。 |
| 宿主 → 节点，既有管理 WSS/443 | 保留固定路径、`webmux-relay.v2` 子协议、TLS/pin、宿主持钥证明、smux v2 和单条有帧管理流；`login.exchange`、`login.resume`、ICE、心跳和撤权继续在该流上运行。 |
| 节点 → 控制面，既有私有认证通道 | 下发节点配置、分配、租约、撤权和授权快照；代际及序号不一致时拒绝工作，并请求完整同步。 |
| `frpc` → 宿主桥 → 节点连接入口 → 私有 `frps` | 宿主 loopback WS → 固定 WSS/443 → 节点 loopback WS；frp 的 Login 与独立 work sockets 走同一路径。 |
| 节点网关 → 节点 loopback 已分配端口 → frp TCP 隧道 → 宿主专用入口 | 在隧道内部做端到端 mTLS；仅网关可送入应用 HTTP，`frpc`/`frps` 只转发密文。 |
| 已授权浏览器 → 宿主，WebRTC 直连或经 coturn | 保留原有 ICE/TURN 端点、传输端口、短期凭据签发、刷新、节点选择和会话撤权；WebRTC 只承载既有允许的交互，文件上传下载与 `/forward/*` 始终走 frp 基线。 |

权威主键是 `{nodeId, hostId, bindingGeneration, connectorGeneration, controlEpoch, nodeEpoch}`，其中 `Scope` 沿用后五项。`allocationId` 关联唯一的名字、节点 loopback 端口、凭据和固定 WSS 路径，不等同于授权。用户会话授权仍按账户、宿主绑定和浏览器 grant 判断；授权 JWT 不承载或替代浏览器 grant。

依据持久绑定中的宿主公钥完成挑战后才创建管理 scope。`ConnectorReady` 确认管理认证，管理流握手完成且租约有效才建立 `controlReady`，均不能证明回源可用。宿主身份私钥、JWT 签名私钥和回源私钥分别隔离，不下发给 frp。

### Management protocol and allocation

现有 `Assignment` 继续绑定节点、归属、版本、期限与 `ServiceTrust`，不附加业务模式选择，也不改变管理握手。
保留 `ConnectorHello`、`ConnectorChallenge`、`ConnectorProof`、`ConnectorReady` 的现有消息、签名约束和 `control` 角色；认证后以二进制 WSS 承载 smux v2。
由节点按现有方式开启一条管理流：宿主读取 `StreamHeader`（`kind=control`、`role=control`，核对当前 `Scope`），写回关联 `requestId` 的 `StreamResult`，随后该流读写 `ControlMessage`。
两端保留 `ReadFrame`/`WriteFrame` 的四字节大端长度加 JSON 编码，以及管理开流、控制帧的原有限制；拒绝重复管理流和非控制开流，不把 `ControlMessage` 改成裸 WSS 消息。

仅在认证管理流上新增下表的授权/就绪消息；管理信封、版本、匹配负载、请求关联、错误结构与已有 RPC 均不变。
时间字段沿用 Unix 秒，代际和序号用既有无损计数类型。PEM/CSR 只在认证通道内传递，须符合现有消息上限且拒绝多余/未知字段。
分配记录和修订校验通过既有认证节点同步通道扩展，不引入绑定级传输标志或传输协商。

| 消息 | 方向与负载 |
| --- | --- |
| `frp.allocate` | 宿主 → 节点 → 控制；`allocationRequest:{hostServerCsr}`。已认证 scope 每次只能有一个分配，节点通过私有通道附自己的回源客户端 CSR。 |
| `frp.renew` | 宿主 → 节点 → 控制；`allocationRenew:{allocationId, grantRevision}`。只续当前分配；不接受客户端报送路径、端口、算法或期限。 |
| `frp.grant` | 控制 → 节点 → 宿主；`allocationGrant:{allocationId, grantRevision, proxyName, remotePort, transportUrl, bridgeKey, jwt, issuedAt, notBefore, expiresAt, renewAfter, origin}`。`origin` 含宿主证书链、回源信任链和网关客户端公钥标识；不含任何回源私钥。 |
| `frp.status` | 节点 → 宿主；`allocationStatus:{allocationId, grantRevision, dataReady, reason?, checkedAt?}`。`checkedAt` 为节点实际完成探针的 Unix 毫秒；未采样时省略。观测仅匹配当前 scope 和分配。 |

`transportUrl` 由分配绑定到已验证连接域名的 `wss://…:443/v1/frp/<allocationId>`（省略默认端口亦可）；拒绝 userinfo、query、fragment、转义路径、跳转和任意后端。
节点先经私有通道得到对应分配、回源公钥和 JWT `jti` 授权记录，安装完成才允许下发和上线；无确认即不放行业务。
初次重试查询同一分配，不为丢失响应另建代理；丢失续约响应按同一修订取回结果，旧修订不能覆盖新值。

结构不符返回 `invalid_request`；未知协议版本仍按现有约束返回 `incompatible_version`，不触发协商或重试其他传输；持钥失败返回 `unauthenticated`/`invalid_signature`；节点或 audience 不符返回 `wrong_audience`。
失效租约/令牌返回 `expired`；旧代际返回 `stale_generation`；重复或越权分配返回 `conflict`/`forbidden`。管理错误以 `type:error` 回显请求标识，不返回凭据或上游异常文本。

### Stock frp OIDC and session admission

控制服务单独发布 OIDC discovery 和 JWKS，由 frps 用显式 issuer/audience 建立验证器；使用经过认证的 TLS，不跳过签名、issuer、audience 或过期检查。
只发布公钥材料，不开放向 frpc 提供客户端密钥或签发任意 token 的接口。轮换 `kid` 时先发布新公钥并验证节点读取，再签发；保留旧公钥到旧授权失效。撤权不等 JWKS 或 JWT 过期。

JWT 固定算法和密钥用途，必需字段为 `iss,aud,sub,jti,iat,nbf,exp,scope,nodeId,allocationId,grantRevision,role,proxyName,remotePort`。
`role=frp-data`；`sub` 为该分配在续约期间稳定、不同分配不复用的主体。每次签发使用不同 `jti` 并绑定当前授权记录；它不是逐消息随机数，同一有效值可用于该登录及其 work/Ping。
宿主只把 JWT 文本写入受限令牌文件；认证消息禁止记录 `privilege_key`。

- v0.72.0 frpc 配置 `auth.method="oidc"`、`auth.oidc.tokenSource.type="file"`、`auth.oidc.tokenSource.file.path`；两端都启用 `auth.additionalScopes=["HeartBeats","NewWorkConns"]`。文件模式不请求 OAuth token endpoint。
- 文件源在 Login、Ping 和 NewWorkConn 各自取值时重新读取并裁剪空白，写入 `privilege_key`，不是启动时一次性载入。以同目录受限临时文件加原子替换更新；无需为普通续约重启。发布前在支持平台（包括 WSL 内的 Linux）验证替换、并发读取和旧凭据交接。
- frps 原生核验签名、issuer、audience、过期；后续消息的原生主体检查只查服务端曾见过的 `sub` 集合，不绑定当前 run ID，也不读取 WebMux 角色或撤权记录。因此节点必须执行以下私有插件验证，且任一插件失败均关闭准入。

私有 HTTP 插件请求为 `{version:"0.1.0",op,content}`，只接受 loopback 的配置路径；公网代理、浏览器应用路由和分配 Upgrade 路由都不转发插件路径，对外返回 `404`。
放行返回 HTTP 200 与 `{reject:false,unchange:true}`；Login 改写用 `unchange:false` 和完整的 `content`；策略拒绝用 `{reject:true,reject_reason:"<固定代码>"}`。
非 200、解析失败和未就绪均关闭准入。stock HTTP 插件客户端没有总超时；插件用内存授权快照进行有界处理，不在回调中远程查询控制面，节点监控超时并关闭相应 Upgrade socket。

| Hook | 节点验证与结果 |
| --- | --- |
| `Login` | 在 stock 认证之前执行。独立验签、核对完整分配、角色、有效 `jti`、活跃管理租约与受信路由；确认旧控制连接已经关闭才允许重连。用服务端生成的唯一 `run_id` 替换输入，绑定该 scope 与连接，禁止客户端选择或抢占 ID。 |
| `NewProxy` | 没有 `privilege_key`；使用 frps 提供的 `user.run_id` 查找已绑定且授权仍活动的登录。只允许一条 TCP 代理、严格相等的有效注册名和非零分配端口；拒绝任意端口、其他类型、负载均衡组和多余代理选项。 |
| `NewWorkConn` | frps 用报文 `run_id` 先定位控制连接；比较它与服务器回填的 `user.run_id`，验证消息自己的 JWT、当前 `jti`、同一分配/节点/代际/角色、活跃租约及工作连接预算。 |
| `Ping` | 验证消息自己的 JWT 与 `user.run_id` 对应的当前会话，检查相同授权、角色和租约；仅成功方可刷新 frps 心跳，不刷新 WebMux 管理租约。 |

配置 `user` 为空以避免代理名称前缀；检查 `user`、忽略不受信的 `metas` 和 `client_id`，Login 只改写 `run_id` 与 `pool_count`，保留原 JWT 供 frps 核验。Login 改写不生成工作凭据。
两端关闭 frp TCP 多路复用（`transport.tcpMux=false`），并强制 `pool_count=0` 抑制预热，使每个在途回源请求使用独立工作连接。
frps 仍有有界请求/工作队列和补充行为，不能把无预热解释为没有空闲 socket。
设独立的 Upgrade/logical-work 上限，分配保留一个 frp 登录控制 socket 和有界补充余量；它不是 WebMux 管理连接。

`run_id` 在一个已接受的 frp 控制连接内不可变。每个 JWT 修订最多准许一次 Login；其后的 Ping/Work 可使用该票据，但不能以它再次登录。旧连接关闭后由管理通道取得新的登录许可；frpc 携带的前次 ID 只作提示，新登录使用新 ID。
先登记待确认的 Login 和对应入口 socket，只有 stock 完成认证、正确注册且探测成功才对外就绪；失败/超时释放登记，迟到事件不得复用旧 ID 或重开已撤销分配。

`NewWorkConn` 的插件数据没有工作连接源地址或代理名，不从其中推断业务分流；其关联依据是 frps 给出的控制会话、报文 ID 和同 scope 的 JWT。公网升级路由另以分配密钥和实时状态准入。
节点在授权每条公网 Upgrade 后登记分配、代际和该连接的关闭句柄；撤权先取消该代际准入，再关闭其全部管理和工作 socket。并发拨号完成时必须重新核对代际。
节点向私有 frps 转发 Upgrade 时覆盖 `Origin`，使用仅本次连接有效的随机标识。原版 v0.72.0 的 WebSocket Login 钩子 `client_address` 返回该 Origin，不是 TCP 对端地址。
Login 只消费节点登记且仍存活的一次性标识，将服务端 run ID 和已验证 JWT 对应到同一分配。标识不替代认证，不记录原值；连接失败、关闭和撤权均清理。两宿主真实收发与单宿主撤权是发布门禁。

拒绝分别触发 stock `LoginResp.Error`、`NewProxyResp.Error`、`StartWorkConn.Error` 或 `Pong.Error`；节点关闭详细客户端错误。Ping 拒绝不等于即时终止，不能用心跳超时替代撤权。

上游固定版本依据： [OIDC 验证](https://github.com/fatedier/frp/blob/v0.72.0/pkg/auth/oidc.go)、[文件值源](https://github.com/fatedier/frp/blob/v0.72.0/pkg/config/v1/value_source.go)、[插件负载](https://github.com/fatedier/frp/blob/v0.72.0/pkg/plugin/server/types.go)、
[插件响应](https://github.com/fatedier/frp/blob/v0.72.0/pkg/plugin/server/manager.go)、[frps Login/Work 顺序](https://github.com/fatedier/frp/blob/v0.72.0/server/service.go)、[NewProxy/Ping 与工作队列](https://github.com/fatedier/frp/blob/v0.72.0/server/control.go)、
[客户端重连](https://github.com/fatedier/frp/blob/v0.72.0/client/service.go)、[TCP 代理监听](https://github.com/fatedier/frp/blob/v0.72.0/server/proxy/tcp.go)。上述为源码观察；不代表发行二进制或反向代理兼容性已经验证。

### Pinned WSS bridge

原版客户端 TLS 配置提供 CA、ServerName 等设置，但没有 WebMux SPKI pin；单独直连 WSS、只做 CA 校验或在启动时探测一次都不能保护后续 work sockets。宿主 Go 进程内实现固定 WebSocket Upgrade 反代，复用 `Client.httpClient` 与 `ServiceTrust` 的 TLS 策略，避免改 frp 或再启动进程。

- `frpc` 只连接宿主随机分配的 IPv4 loopback 端口；stock 配置键为 `transport.protocol="websocket"`（线上是 `ws`）、`transport.tls.enable=false`。只接该 `Host` 上精确的 `GET /~!frp` WebSocket Upgrade；禁用其他路径、查询、绝对 URL、CONNECT 和普通 HTTP 转发。
- 桥只向该分配固定的 WSS/443 发起 Upgrade，覆盖 URL、Host、SNI 和所需 Origin；移除来路认证/转发头，注入仅桥保存的 `bridgeKey`（用认证头传递，不入 URL）。节点只在连接域名的分配路径接收，核对密钥和活动 scope 后固定转发到 loopback `frps` 的 `/~!frp`，不把头部密钥交给 frps。
- 实际外拨的每条连接都校验 TLS 1.3、CA 链、DNS 名称、证书期限以及分配信任中的 P-256 SPKI SHA-256 pin；禁用代理环境变量、重定向、用户自定义目的地和跳过验证。pin/信任更新由认证分配下发，先重叠认可新旧 pin，再换证并收回旧 pin；不自动信任握手提供的证书。
- 只中继标准 Upgrade 后的字节，不解释或增加 frp 帧；桥两端分别限并发、连接尝试率、请求头、拨号/握手超时，取消时关闭两侧。错误路径/方法拒绝为 `404`/`400`；无效凭据 `401`，超限 `429`，无活跃租约 `503`；外拨 TLS/pin 失败直接关闭，不重试不安全路径。

依据：[frpc 传输连接器](https://github.com/fatedier/frp/blob/v0.72.0/client/connector.go)、[TLS 配置](https://github.com/fatedier/frp/blob/v0.72.0/pkg/config/v1/common.go)、
[Upgrade hook](https://github.com/fatedier/frp/blob/v0.72.0/pkg/util/net/dial.go)、[frps WebSocket 路径](https://github.com/fatedier/frp/blob/v0.72.0/pkg/util/net/websocket.go)。
必须实测节点现有入口的 Upgrade、Host/SNI、二进制帧、错误与关闭语义。

### Generation-scoped origin mTLS and app admission

宿主在独立 loopback 端口启动专用 HTTPS 入口；本机 `frpc` TCP 代理的 `localIP`/`localPort` 固定指向它，而不是通用 WebMux 监听。网关只拨节点已分配的 loopback `remotePort` 并在其上发起 TLS 1.3。公网 TLS/桥、OIDC 签名与这对回源证书属于不同信任域，密钥不复用。

宿主与节点分别在各自进程中生成临时密钥并证明 CSR 持钥，控制面以专用回源 CA 分别签发 `serverAuth` 和 `clientAuth` 叶子。URI SAN 带唯一 `allocationId` 和角色，授权状态把 SAN 及其公钥绑定到完整 scope。
宿主证书另含由分配派生的内部 DNS SAN，网关用它作 TLS ServerName。两端检查链、期限、用途、角色、完整 scope 和确切对端公钥；禁用 TLS 会话票据，连接存活期间仍要求 scope 活跃。

专用入口只在 mTLS 成功且实时授权有效时赋予服务器内部可信回源上下文。它重用宿主对固定业务 Host、Origin、Cookie、CSRF、一次性 code 和授权 grant 的检查；浏览器头部不授予内部身份。内置转发仍受授权约束；应用请求若到达其他本机端口或篡改转发头，不会成为 Relay 回源请求。

网关沿用 `requestRole` 的显式交互白名单；上传、下载、转发和未知路由落入 bulk。先校验绑定和活动代际、剥离不可信身份头，再领取宿主与账户配额；保留交互名额、共享字节预算和有界等待队列。
资源不足尽早返回 `429`，已授权但节点/管理/分配未就绪返回 `503`，回源拨号或 mTLS 失败返回 `502`；响应已升级或开始发送后只关连接，不伪造第二个响应或重放写请求。

业务请求各用独立 TCP/TLS 回源连接；HTTP 流式和 Upgrade 不缓冲全体，也不建业务连接池或走旧业务逐流协议。授权/准入/预算待机、拨号、请求头和空闲阶段各自有界；连接活跃期间保留请求/连接所有权到取消或结束。独立 work sockets 消除业务共享 smux 写队列，不等于绕开物理链路拥塞，饱和 bulk 时必须实测交互延迟。

`controlReady` 来自认证管理流和有效租约。节点在当前注册的真实 frp 工作连接上完成当代 mTLS，首次取得专用 `HEAD` 探针的 `204` 后开放业务；重新注册、代际更替须重新验证。
宿主按当前授权分配登记回源，继续校验 mTLS、Host、Origin、Cookie 和 CSRF。`frp.status` 只更新诊断；失败或过期分别表示最近回源失败或观测未知，不撤销授权，不重试已发送的请求。
该路径只在已认证回源上存在，不创建浏览器 grant 或读取 Cookie，公网网关对它返回 `404`。节点现有 `/live`、`/ready/wss`、`/ready/turn` 的组件级成功也是 `204`，但不能代替逐 scope 实际回源探测。

`/local/v1/status` 和 `webmux status --json` 使用现有的单个可选 `relay` 对象，保留启用、配对、固定访问地址、来源与脱敏错误等字段，使用 `controlReady`、`dataReady` 取代旧池状态。
保留 `transport=wss` 表示独立于浏览器实际选路的公网基线；按浏览器会话区分 `wss`、`webrtc-direct`、`webrtc-turn` 及脱敏探测/回落原因。
进程存活、注册成功和 TURN 可用性均不能代替回源就绪证据。
删除 `bulkReady`、`readyConnections`、其它角色就绪计数及旧池配置 `interactive_connections`、`bulk_connections` 的解析、默认值、校验、组装与诊断映射。升级时从部署配置移除这些键；不保留旧参数兼容，也不增加 `dataPlane`、第二状态对象或独立 bulk 传输状态。

### Lifetime, renewal, and revocation

- 以已有 10 秒管理心跳和 30 秒管理租约为失联边界，另给 frp 配置独立心跳和超时。JWT 最长 120 秒，计划在签发后约 60 秒续约；时钟偏差容忍至多 5 秒且不得延长撤权、管理租约或 `exp` 的硬停止点。
- 同一分配保持端口、名字、bridge key 和回源密钥不变。先在节点激活新修订，经确认后宿主原子替换文件；旧 `jti` 只保留最长 10 秒交接窗口且不跨旧令牌到期。只允许当前和这一条前任修订；已撤销的修订没有交接。
- 活跃判定同时要求当前绑定、匹配的控制/节点 epoch、认证管理流、已生效配置与授权快照，以及未失效的运行授权。JWT 不能延长任何租约。节点或宿主一旦察觉失效，停止新请求并关对应连接。
- 回源证书与分配最长 24 小时；到期前由控制面计划新代际，先撤旧公网路由并关闭旧连接，再发放新代际并探测上线。普通 JWT 续约不影响长连接；强制代际轮换可断开传输，宿主 PTY 保持运行。不得把证书过期当撤权或依赖长连接在到期后自动重新验证。
- `frpc`/`frps` 崩溃或数据断连先清 `dataReady`；同节点管理 WSS 和授权未失效时保持 `controlReady`，按原分配限速恢复，探测成功再接受业务。
- 宿主监督器使用带抖动的 1–20 秒指数退避，连续 5 次子进程重启后按分钟冷却并等待有效授权；stock 的进程内重连受入口连接尝试率及租约限制。
- 管理 WSS 断开、管理流失效或租约失效即回收同节点的公网与派生路径；重连需重新持钥认证并分配新代际，新的宿主进程或节点/控制 epoch 也不能复活旧凭据。节点失效时只接受控制面签发的新归属，不把管理与业务拆到不同节点恢复。

管理撤权保持四种粒度：`grant` 或 `console-session` 关闭对应 HTTP/WS/P2P/转发并拒绝继续 `login.resume`，不停止其他已授权用户的 frp；`connector` 停单一代际；`binding` 停所有该绑定派生能力并移除网关路由。ICE 停止续签，不把关闭派生访问的确认误作销毁 coturn 分配或终止 PTY。

整代撤权顺序：原子停网关及入口准入、废止插件映射与 `jti`，闭合节点网关的回源 socket 和对应入口全部已升级 WSS，宿主关闭专用入口已有 HTTP/Upgrade 与桥并停止子进程。对话通道先送 `revoke`，完成关闭才回 `revoke.ack`；控制面收不到确认则按租约和同步异常处理，并封住旧路由，不把过期推断为确认。

节点以受控入口主动关闭 frps 控制连接触发原版清理；以 proxy close 通知或无监听/无旧活动连接的可观测状态确认释放，不允许迟到注册拿回已撤销端口。TLS/代理后端未确认关闭则扣留端口和分配名；不依赖公开 dashboard 或整节点重启才能撤单一宿主授权。

### Packaging, listeners, and supervision

- 固定上游 [v0.72.0 release](https://github.com/fatedier/frp/releases/tag/v0.72.0)；构建核对平台归档的固定 SHA-256 和来源，取对应独立可执行文件及许可证，纳入 WebMux 签名发布清单。
- 同一发布清单覆盖宿主 Linux/macOS amd64、arm64。Windows 托盘/CLI 是 WSL 桥，安装器须在对应 WSL 发行版安装 Linux `frpc`；Windows 侧没有第二个隧道进程。独立包、安装脚本和更新器同时交付匹配制品；裸 WebMux 缺件时本机功能不受影响。
- 缺件或摘要/版本不符报脱敏 `invalid_config`，不影响宿主其他入口；运行时不从网络取件，不使用 `PATH` 或系统安装的 frpc。
- 私有 Dokku 节点镜像同样含独立 Linux `frps`，随节点代理监管；同一受控网络命名空间内运行网关、插件及 frps loopback 监听。进程停止、健康失败或版本/摘要不符关闭新 transport，不重启或改动 coturn。
- 每个活动 scope 稳态最多一个 `frpc`；显式试连期间允许一个有期限的候选共享原配额。撤权收割两者，失败只收割候选。使用受管绝对路径，POSIX 目录 `0700`、文件 `0600`，凭据不进入 argv、环境或日志；异常退出后只清理可证明属于该实例的资源。

| 监听 | 可达性与配置 |
| --- | --- |
| 既有公网 TCP/443 | 浏览器 HTTPS、认证管理 WSS 和单独的 frp WSS 路径，沿用现有 TLS 终止与网关；此层负责公网 pin 对应的证书。 |
| frps 接入 TCP | `bindAddr=127.0.0.1`，专用不冲突 `bindPort`；仅节点入口拨入；外层 TLS 已在现有入口终止。 |
| frps TCP 代理范围 | `proxyBindAddr=127.0.0.1`，显式非空的 `allowPorts` 私有端口范围；插件要求当前 scope 分配的确切单一端口和名字，不接受 `remotePort=0`。 |
| 节点插件 | 独立 loopback HTTP 监听，不在 Dokku 公网映射或任何通配路由后；只启用受控 hooks。 |
| 宿主桥与专用回源入口 | 各自不同的动态 loopback TCP 监听；桥只受理 stock Upgrade，应用入口只受理 mTLS。 |
| TURN 与其它 frp 监听 | coturn 原端口、公布地址、映射与凭据策略保持；不增公网 UDP。frps 禁用 KCP、QUIC、SSH gateway、vhost、HTTP CONNECT mux、dashboard 和公网指标监听。 |

端口范围排除节点既有监听，按分配排他预留，实际 bind/探测失败不发布路由。配置确认 IPv4/IPv6、Dokku 映射和防火墙都没有发布 frps 或插件端口。frps 失效会打断该节点的基线回源，但管理 WSS、控制和 TURN 保持独立；禁止生产拨号降级成公网 TCP/UDP。

### WebRTC and TURN verification

保留既有生产策略：frp WSS 基线不等待 ICE，浏览器按现有收益与健康阈值升级允许的交互，按原滞回规则回落；TURN-only 节点不能作为 WSS 目标。
短期 TURN 凭据、刷新/撤权、监听及公开端口、TLS/SNI、NAT 映射与节点选择不变。
frp 不承载 TURN 流量；TURN 故障不得清除健康基线就绪，frp 故障不得触发健康 coturn 重启。登录、文件和转发继续走基线。

端到端门禁在隔离部署使用真实浏览器、宿主 PTY、coturn 及签名 frp 制品。
TURN 场景用测试网络或仅测试夹具强制 relay-only ICE，生产策略不变；从 ICE 统计记录实际选中、已提名、成功的候选对及其 relay 候选，而非仅观察候选收集、ICE connected、配置可用、探针或诊断标签。
通过该路径提交带序号的唯一输入并收到实际 PTY 回显；保持同一 PTY、管理流和 frp 回源健康，只阻断活动 TURN 路径并排除其它 ICE 替代。
验证现有健康检测自动回落：网关证据显示真实 frp 工作连接，同一 PTY 按原 generation/offset 续接，连续输出无缺失或重复、已接受输入仅执行一次，恢复后的新输入得到真实回显。
记录检测、切换、续接和回显耗时，不手动重启或重放输入。

另在可直连的拓扑验证实际选中且已提名的非 relay 候选对及双向 PTY 回显，并验证登录、上传下载与 `/forward/*` 仍经 frp，未满足收益门槛时交互留在基线。仅 TCP/443 出站的基线门禁在另一独立场景执行，不以禁用 UDP 的测试代替真实 TURN 转发。

### Cross-repository implementation map

| 仓库 | 现有落点与改动边界 |
| --- | --- |
| `webmux`：`protocol/relay/access.go`、`auth.go`、`sync.go` | 保留 `Assignment`、`ServiceTrust`、管理握手；仅扩展节点同步所需的 frp 授权。 |
| `webmux`：`protocol/relay/stream.go`、`frame.go` | 保留 `Scope` 与已有控制消息，仅增 frp 授权/就绪。保留 `StreamHeader`/`StreamResult` 管理用途、`ReadFrame`/`WriteFrame` 和控制帧限制；删除 `StreamHTTP` 和 interactive/bulk 建连角色。 |
| `webmux`：`internal/relay/client.go`、`dial.go`、`control.go` | 保留 `Client.Assignment`/`Client.httpClient` 的控制面及 pin 关联、管理 `smuxConfig`、`controlRPC` 和 `Client.Resume` 语义。 |
| `webmux`：`internal/relay/connector.go`、`stream.go` | 以受管子进程、固定桥及 mTLS 入口替换 `connectionPool`、业务槽位重连、业务流接受与复制。 |
| `webmux`：`cmd/webmux`、`internal/config` | `newRelayController`、`relayController.Reconnect` 管理 frp 生命周期；`accesslinks.go` 保留通用 URL；移除旧池配置。 |
| `webmux`：`internal/contract`、`web/src` | 移除旧池状态，在单个 `relay` 对象、wire fixtures、`RelayDiagnostics.svelte` 与 `relay-schemas.ts` 一次切换就绪表示；不改浏览器 ICE 与交互切换策略。 |
| `webmux`：`.github/workflows/release.yml`、`Makefile`、`deploy/`、`install.sh` | 平台资产/签名、安装和更新一起交付受管 `frpc`；用现有发布和配置兼容机制校验。 |
| `webmux-relay`：`internal/control`、`internal/httpcontrol`、`internal/store` | `Service.Assignment`/`SetAssignment` 保留节点归属和当前代际。同一节点同步分配、续约并储存/查询有效 `jti`。 |
| `webmux-relay`：管理授权 | 保留 `PublishRevocation`/`RevocationAck`；以恢复分支 `ResumeBrowserGrant` 和 `ControlResume` 为管理基线。 |
| `webmux-relay`：`internal/dataplane` 网关 | `Server.ServeHTTP` 保留网关，`requestRole`/`accountBudget` 保留准入；用专用 mTLS transport 替换 `newHTTPProxy` 的 `connector.OpenHTTP` 业务拨号。 |
| `webmux-relay`：`internal/dataplane` 连接 | 移除业务池与业务开流帧，保留管理 WSS/smux 的原有控制开流和 RPC。`Server.Revoke` 关闭实际代理及派生连接。新增私有插件、frp WSS 入口/注册状态和回源探测。 |
| `webmux-relay`：`internal/node`、`cmd/webmux-relay`、`deploy/dokku` | `Agent.reconcile`、`NewRuntime`/`componentRuntime` 接受 frps 配置、租约、进程健康及组件隔离；固定镜像资产及入口路由，延续 coturn supervisor。 |

## Risks / Trade-offs

- [上游 OIDC 与节点插件是双层认证] → 分别检查实现与配置；登录后核心 OIDC 验证失败不应残留可用会话，插件不可达不得放行。对私有 issuer/JWKS 使用有界访问和密钥轮换门禁；不得切为全局 token 作为兼容退路。
- [单 frpc 与独立 work sockets 增加握手、FD、TLS 和入口耗时] → 按节点和账户预算连接，保留交互名额并在 bulk 饱和时度量交互延迟；长流的普通续约不重启进程。
- [边缘代理、升级空闲超时、stock work 队列可能泄露生命周期差异] → 分别测长 WebSocket、慢上传、取消、发送/接收半关闭和撤权，若旧连接仍可接触应用则停止发布；不引入自定义数据帧弥补。
- [公网证书轮换或错误分配会导致 pin 不匹配] → 用认证配置重叠轮换，并从相同受限出站路径真实探测；宁可未就绪也不绕过 pin。
- [整版切换需要维护窗口] → 预先验证整套制品和回滚流程，关闭旧公网路由并核实撤权后才打开新路由；断开的业务连接由正常客户端机制重建，PTY 不迁移、不终止。无法证明旧路径已关闭则保持停用。
- [替换后自维护组件反而变多] → 在隔离环境按两仓列出替换前后的数据帧、调度/重连状态机、特权接口、密钥种类、发行件及值班步骤；让另一位维护者按手册完成整版升级、撤权和回滚。若仍需维护旧业务状态机或上游必须打私有补丁，发布门禁不通过。

## Migration Plan

1. **Reproduce the baseline.** 生产采用 Dokku 独立控制面与数据节点、PostgreSQL 17；节点同载 Relay 与 coturn。现有 TCP/443 承载公网入口，不要求 UDP/443。
   记录实际镜像摘要、Dokku 包装提交、Go 构建提交及可恢复制品；在隔离环境恢复实际制品，证明干净构建保留管理握手、`login.resume`、ICE、心跳及撤权行为。
   组件探针不能代替真实终端或 TURN。
2. **Gate the complete release in staging.** 使用配套宿主、控制面和节点制品及实际 Dokku 代理；验证制品摘要、TCP/443 受限出口、IPv4/IPv6 私有端口、实际 WSS pin、127.0.0.1 注册与当代 mTLS。
   验证插件改写 run ID、连接关联、跨 scope/role 与错名字/端口拒绝、OIDC/JWKS 故障、原子续约及撤权；任何 stock 门禁失败不改用私有 frp 分支。
   回归一次性 code、`login.resume`、撤权、长 WebSocket、文件/转发、限流与真实 PTY；通过直连、relay-only TURN 及中断后 frp 回落验收。
   旧制品仅在隔离环境作为对照和回滚候选，不进入新发布的运行路径。
3. **Cut over the whole release.** 在发布前即删除两仓旧业务连接池、业务帧分支、旧槽位重连、配置和状态，不删除管理 WSS/smux 承载、管理开流帧及共享帧帮助函数。
   维护窗口先停止旧公网路由和新准入，撤销旧管理代际、关闭旧业务与派生连接并确认清理；保留固定 Origin、绑定 ID、宿主密钥、持久 PTY 和数据库授权/撤权事实。
   入口保持关闭，安装配套的宿主、控制面和节点制品及配置；只有新权威、经当代认证的管理流和实际 frp 回源都已验证，才恢复公网服务。
   发布不按绑定切批，不允许版本混跑、双路径竞速或新旧协商。
4. **Roll back only as a release.** 检测到信任、撤权、数据完整性、资源隔离、管理或 TURN 回归时，停公网准入与签发，废止 frp JWT 和回源信任。
   关闭候选发布的路由、连接、私有注册与受管进程；无法确认隔离则保持关闭，不标记完成。
   整套恢复预先验证的宿主、控制面、节点制品及兼容配置，仅以当前权威数据和新分配修订、新管理代际重新认证和验证后开放。
   不恢复旧令牌、旧权限、撤销前数据库快照或旧活动路由。该动作是停服替换整版制品，不是当前版本的旧传输回退；没有可安全恢复的整版目标则继续关闭。
