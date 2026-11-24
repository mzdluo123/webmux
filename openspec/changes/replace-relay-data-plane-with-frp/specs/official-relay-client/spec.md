# Spec Delta

## MODIFIED Requirements

### Requirement: 连接状态机与公网来源撤权

Relay 客户端 SHALL 区分关闭、连接中、就绪和失败状态，并单独表示未配对、配对中与已配对。
`controlReady` SHALL 来自当前认证管理流和有效租约，`dataReady` SHALL 来自当前授权分配经网关验证的实际 frp 回源。只有两者均有效才登记固定 HTTPS 来源为允许的 Host/Origin 并公布访问地址。
control 断开、绑定撤销、关闭或服务退出时，客户端 SHALL 立即撤销该来源的 Host/Origin 授权并关闭相应业务和派生连接。新连接认证失败 MUST NOT 替换或撤销既有合法 control。
单条工作连接的失败 SHALL 隔离受影响请求，不影响其他健康的交互或管理连接。失去当前回源就绪证据时，宿主 SHALL 撤销该来源授权；仍有效的管理流可处理撤权和业务恢复。
启用状态下的意外断线 SHALL 按有界指数退避和随机抖动重连。本机服务、持久 PTY 与其他入口不受影响；代际检查 SHALL 阻止重复连接器、迟到就绪和旧配置重新授权。

#### Scenario: 断线后恢复

- **WHEN** 就绪的 Relay 因网络中断失效，随后网络恢复
- **THEN** 中断期间公共来源未授权；宿主自动恢复，当前管理流及 frp 回源重新就绪后，使用原固定地址重新授权

#### Scenario: 关闭期间收到迟到就绪事件

- **WHEN** 用户已关闭 Relay，先前连接或探针随后报告就绪
- **THEN** 该事件被丢弃，公共来源保持未授权，不创建新连接器

#### Scenario: 管理已认证但回源未就绪

- **WHEN** 管理 WSS 在线且 `frpc` 已登录或已注册，但当代网关回源检查尚未通过，或通过后到期
- **THEN** `controlReady=true`、`dataReady=false`，不发布固定访问地址或允许该 Relay 来源；管理连接仍能处理撤销

#### Scenario: 管理失效时数据连接仍通

- **WHEN** 管理 WSS 断开或租约失效，而 frp 工作连接或 TURN 仍可达
- **THEN** 宿主立即撤销公共与派生访问并关闭关联连接，不等 JWT 或 TURN 凭据到期，持久 PTY 保留

### Requirement: 固定访问地址、状态与推送来源

Relay 就绪后，固定公共地址 SHALL 出现在现有访问 URL 汇总、本机状态和 Relay 设置页，且地址与二维码 MUST NOT 自动附带 WebMux Token。Windows 托盘 SHALL 通过现有通用访问入口展示该地址，无需新增 Relay 专属菜单协议。
配对的固定 HTTPS 来源 SHALL 纳入现有 Web Push 来源管理。新订阅只能从当前通过 Host/Origin 校验的入口开启；Relay 离线期间已有通知可以保留，点击后等待入口恢复。
`/local/v1/status` 和 `webmux status --json` SHALL 保持单个可选 `relay`，以 `transport=wss` 表示基线，报告 `controlReady`、`dataReady` 和脱敏失败原因；浏览器实际交互路径单独按会话报告，不返回第二个 Relay 对象、`dataPlane`、`bulkReady` 或角色就绪计数。
状态与错误输出 MUST NOT 包含私钥、配对材料、GitHub 凭据、WebMux Token、业务 JWT、桥接密钥、回源私钥或携带凭据的 URL。

#### Scenario: 查看远程访问入口

- **WHEN** Relay 与 Cloudflare Tunnel 同时就绪
- **THEN** 访问 URL 汇总分别列出两个固定来源，撤销其中一个不会移除另一个

#### Scenario: 从 Relay 地址开启推送

- **WHEN** 手机通过就绪的 Relay HTTPS 地址打开 WebMux 并开启通知
- **THEN** 订阅关联该固定来源，重启 WebMux 并重连 Relay 后通知仍指向同一地址

#### Scenario: 业务回源故障期间查询状态

- **WHEN** 管理连接仍有效，但当前 frp 回源探测失败
- **THEN** 单个 `relay` 报告管理就绪而业务未就绪及脱敏原因，不把进程存活、注册或单次 WSS 握手当作业务就绪

### Requirement: 会话认证覆盖与撤销

Relay 会话 SHALL 覆盖 HTTP、WebSocket、P2P 信令和 `/forward/*`，前端 SHALL 通过现有 API/WS 封装识别登录态。Cookie 写请求 MUST 校验同源 Origin 与 CSRF token，转发上游前 MUST 剥离宿主认证 Cookie 和 CSRF 材料。
宿主注销、会话过期、对应后台会话撤销、绑定撤销、换钥、移出邀请名单或 Relay 关闭 SHALL 撤销相关宿主会话。宿主重启 SHALL 清空会话。撤销 SHALL 拒绝后续请求并关闭该会话建立的 WebSocket、P2P 和转发连接，但 MUST NOT 终止持久 PTY。
临时 control 断线 SHALL 立即暂停业务并关闭派生连接；有界内存恢复记录仅存至原期限，不能单独授权。
新连接就绪后，宿主 SHALL 经认证管理 WSS 调用 `login.resume`，复核原 grant、后台会话、所有者、邀请及绑定；取得精确绑定新 Scope 的授权后才恢复原 Cookie 对应的访问。
恢复 SHALL 保持原 grant ID、所有者、Origin 和绝对期限，不延长 12 小时上限。明确撤销与恢复并发时 SHALL 以撤销为准；旧访问上下文与旧 Scope SHALL 保持失效。前端暂时恢复期间 SHALL 保留原路由并有界退避查询，恢复成功后按原终端续接协议重连，不自动重放业务写请求。
业务隧道登录、续约和回源 mTLS MUST NOT 签发、恢复或延长浏览器 grant。单个浏览器 grant 或后台会话被撤销 SHALL 关闭其访问，不停止其他合法会话共享的受管业务传输；管理代际或绑定撤销 SHALL 关闭对应的整个隧道。

#### Scenario: 短暂断线后恢复登录

- **WHEN** control 连接短暂中断后以新连接代际恢复，原后台会话、邀请、绑定及 grant 仍有效
- **THEN** 断线期间业务访问被拒绝，旧派生连接关闭；当前管理流和实际 frp 回源重新就绪且 `login.resume` 成功后，原 Cookie 才可继续访问，绝对到期时间不变

#### Scenario: 断线期间撤销授权

- **WHEN** 登录处于暂停状态时，后台注销、取消邀请、解绑或 grant 撤销
- **THEN** 控制面拒绝恢复，旧 Cookie 不能访问业务，即使新业务隧道已就绪；页面说明登录状态并提供重新登录入口，不显示直连 Token 错误

#### Scenario: 关闭 Relay 后旧会话不能复用

- **WHEN** 用户关闭后重新启用 Relay，再使用旧会话 Cookie 访问
- **THEN** 旧会话被拒绝，用户需重新登录 GitHub 后台，主机配对和持久 PTY 保留

#### Scenario: 端口转发不泄露登录会话

- **WHEN** 已登录用户通过 `/forward/*` 访问目标应用
- **THEN** 目标应用收不到宿主认证 Cookie 或 CSRF 材料，跨源伪造写请求被拒绝

#### Scenario: 已升级 P2P 的会话过期

- **WHEN** Relay 登录会话过期且浏览器仍有活动 P2P 或 WebSocket 连接
- **THEN** 相关访问连接被关闭、后续请求被拒绝，其他合法浏览器及持久 PTY 不受影响

### Requirement: WSS 连接隔离与重连诊断

宿主 SHALL 经认证分配的 `connectorUrl` 保留原管理 WSS：`webmux-relay.v2` 子协议、持钥证明、smux v2 和单条 `control` 流。
管理开流保留 `StreamHeader`/`StreamResult`、Scope/请求关联检查及长度前缀 JSON 编码。
管理 RPC SHALL 保留原帧格式、大小限制、心跳、`login.exchange`、`login.resume`、ICE 和撤权语义，仅新增 `frp.allocate`、`frp.renew`、`frp.grant`、`frp.status` 授权与就绪消息。MUST NOT 接受非控制开流或把管理 RPC 改成裸 WSS 消息。
宿主与 Relay 间的基线业务 SHALL 只由受管 frp WSS 回源承载；`tcpMux` SHALL 关闭，在途代理请求使用独立工作连接。MUST NOT 保留 `StreamHTTP`、interactive/bulk 连接器角色、业务帧、连接池或槽位重连。交互与批量仍是 Relay 网关准入类别，保留交互预留与有界等待，不代表独立传输或就绪状态。
普通请求的业务并发 SHALL 由 Relay 网关单点准入；宿主保留必要的管理帧限制及应用缓冲与超时边界。本地套接字上限不能替代网关准入，也不能重复施加业务配额。
基线传输固定 WSS，不提供 auto/quic 或 UDP connector endpoint，不接收浏览器指定或节点自报的拨号地址。授权 ICE/TURN 仍按独立流程取得。
证书、pin、主机身份与协议错误 MUST NOT 通过重试绕过。网络重连 SHALL 使用有界退避，报告实际传输和脱敏 `reconnectReason`；MUST NOT 自动重放非幂等业务请求。

#### Scenario: 原管理协议不变，拒绝旧业务开流

- **WHEN** 配套宿主和节点经原握手开启控制流，并分别交换现有登录、ICE、心跳和撤权请求及新增 frp 消息
- **THEN** 现有管理帧与请求关联保持不变；重复管理流及旧业务角色或 `StreamHTTP` 开流被拒绝，不协商或启用另一套业务传输

#### Scenario: 续接后恢复可操作

- **WHEN** 网络恢复且原持久 PTY 仍存活，浏览器按 generation/offset 重新附加
- **THEN** SHALL 维持输出连续性且不重复提交已确认输入，以恢复后新输入的真实回显证明可操作；验收记录检测、建链、续接及回显耗时，不把手动重启夹具当作自动恢复

#### Scenario: UDP 黑洞不延迟 WSS 建连

- **WHEN** UDP 不可达而目标 TCP/443 可达
- **THEN** 客户端直接建立管理与 frp WSS，不等待 QUIC 或 ICE 超时；只有当前管理流和实际回源验证均通过才宣告入口可用

#### Scenario: 错误证书保持失败

- **WHEN** 实际管理或业务 WSS 的证书、主机名或 pin 校验失败
- **THEN** 对应连接失败且报告脱敏安全错误，不发送凭据，不切换信任配置或其他传输

#### Scenario: 批量流中断不驱逐交互

- **WHEN** 大文件上传的 frp 工作连接中断，其他工作连接、管理连接和回源仍健康
- **THEN** 仅取消相关传输并释放网关额度；已预留的交互容量继续可用，不重放上传

### Requirement: 控制面发现与数据面分配

宿主 SHALL 只需配置 `controlEndpoint` 与独立受众，按公开控制 CA-only 策略配对和发现数据面，不要求用户手工填写 `connectorUrl`、公开控制 pin 或第二个服务地址。
独立 `webmux-control` SHALL 是唯一发现来源，通过已认证控制通道提供绑定当前主机、受众、分配版本与来源元数据的 `nodeId`、能力、`connectorUrl`、数据面 origin、固定公共 HTTPS 地址及对应服务 TLS 信任。
发现结果 SHALL 来自管理员经控制服务校验并登记的用途化端点，不接受任意浏览器输入、节点自报或未认证下载扩展信任。只有具备 `wss` 能力且就绪的节点可作为连接器目标；`turn`-only 节点只提供 TURN，不暴露 WebMux HTTPS/WSS 入口，MUST NOT 被当作连接器目标。
宿主 SHALL 校验节点分配的主机、绑定代际、受众、版本及端点与服务身份的一致性。frp 业务授权仅经该节点当前认证管理流取得，并绑定当前管理 Scope、节点、分配 ID、修订及有效期；宿主不从其他节点取得替代业务授权。
各类材料的有效期上限 SHALL 在签发端限制；接收端 SHALL 检查绝对到期时间，不根据本机时钟重新推算签发寿命上限。签名、单次 nonce、归属、受众、代际及本地操作超时 SHALL 保持独立校验；拒绝重放、过期和非当前分配。
已配对控制面内的数据面端点调整 SHALL 经认证的版本化分配完成，撤销旧代际和路由后重新校验实际连接的 TLS 与 pin；迟到事件不能恢复旧授权。分配 MUST NOT 协商、选择或回退到另一种业务传输。
浏览器 SHALL 始终使用固定宿主 HTTPS Origin 及获授权的 ICE 配置，不自行选择 WSS 节点。获授权 TURN 可来自不同的 `turn`-only 节点。业务传输替换和数据面端点调整 MUST NOT 改变固定 Origin、绑定或配对密钥。

#### Scenario: 控制面与数据面分离部署

- **WHEN** 宿主只配置控制面地址和独立受众，按 CA-only 策略配对后，控制面为其分配另一 origin 的就绪 WSS 节点
- **THEN** 宿主经认证发现 `connectorUrl`、受众和对应 TLS 信任，完成标准 TLS 与 pin 校验后建立管理流并从该流取得 frp 业务授权；用户无需填写第二个端点

#### Scenario: TURN-only 节点不是 WSS 目标

- **WHEN** 控制面为浏览器授权独立 TURN-only 节点，同时为宿主分配 WSS 节点
- **THEN** 连接器仅拨号 WSS 节点，ICE 可使用独立 TURN 节点；若连接器分配指向不具备 `wss` 能力的节点则拒绝该分配，不尝试其管理通道

#### Scenario: 分配身份或信任不匹配

- **WHEN** 分配归属、受众、版本或代际不符，或实际 WSS 服务证书或 pin 与认证分配不符
- **THEN** 宿主拒绝建立对应连接或注册，不交付相关凭据，不接受浏览器或节点自报的替代地址/公钥，也不扩大信任后重试

#### Scenario: 已认证控制面调整数据面端点

- **WHEN** 已配对控制面以新修订分配合法的新连接端点，且仍使用同一固定公共 Origin
- **THEN** 旧代际撤销并确认路由关闭后，新端点须通过实际 TLS/pin、管理与 frp 回源验证才上线；绑定与配对不变，迟到状态不能恢复旧授权

### Requirement: 数据面公钥轮换与自建控制隔离

数据面公钥轮换 SHALL 通过已信任控制面的有效版本化分配明确授权，正常维护采用当前与下一公钥的重叠窗口，不得从目标证书自动学习；节点身份换钥 SHALL 保持先登记新 pin/代际再切换。
公钥校验 SHALL 作用于实际使用的每条出站 WSS，包括管理、frp 登录和每条工作连接；同一分配的 WSS 端点在重叠窗口可使用明确授权的当前或下一 pin，预检其他连接不能替代实际握手。
数据面或节点公钥泄露后的恢复 SHALL 更新相应信任；若需要客户端升级，MUST 明确提示，MUST NOT 声称离线客户端立即撤销旧公钥。公开控制证书续期/换钥 SHALL 使用标准 CA/TLS 策略，不保留控制 pin 轮换流程。
自建控制面 `controlEndpoint` 覆盖 SHALL 显式配置独立服务受众和适用 CA 环境，仍只使用公开控制 CA-only 校验，不接受旧 InitialTrust 配置别名。
变更控制面或受众 SHALL 先撤销旧连接、公共来源和会话，确认后重新配对；旧绑定、宿主凭据及未完成授权材料 MUST NOT 发送到新服务。
已信任控制面内的数据面版本化调整 SHALL 遵循严格分配与 pin 校验，不要求用户手工维护数据面信任，也不得借此替换控制面引导配置。

#### Scenario: 已分配数据面公钥正常轮换

- **WHEN** 同一认证 WSS 分配先授权当前与下一公钥，节点随后在原端点换证
- **THEN** 每条实际出站 WSS 连接均按自身握手校验；重叠窗口接受两把明确授权的公钥，旧 pin 退役后拒绝使用旧公钥建立的连接，不从证书自动学习

#### Scenario: 预检成功而实际工作连接 pin 错误

- **WHEN** 分配端点的预检已成功，而后续工作 WSS 收到 CA 和主机名均有效但未获授权的公钥
- **THEN** 该工作 WSS 握手失败，不发送 Upgrade 认证头或 frp 凭据，不凭预检结果继续连接

#### Scenario: 从官方切换到自建服务

- **WHEN** 用户将官方默认配置覆盖为自建控制地址和独立受众
- **THEN** 旧连接和会话失效，新服务通过 CA-only 校验后仍要求独立配对，不能取得旧服务的绑定凭据

### Requirement: 按测量升级的交互路由

浏览器 SHALL 在 frp WSS 基线已可用后，以现有有界频率比较基线与候选直连/TURN 的应用层往返及健康状态。只有连续采样满足既有收益与健康门槛才升级；ICE 成功或候选类型本身 MUST NOT 作为升级依据。
活动路径 SHALL 持续检测健康，失败立即回落 WSS，性能退化按滞回策略回落；实现 SHALL 固定并验证收益阈值、采样窗口、冷却期与探测资源预算，防止频繁切换。
仅显式允许的终端、聊天、列表交互业务 SHALL 使用升级路径。文件上传下载、全部 `/forward/*`、登录、设置、语音与其他未分类业务 SHALL 保持 frp WSS；批量流量 MUST NOT 绕入交互通道。现有 ICE/TURN 配置、短期凭据的签发、刷新和撤销、端口、节点选择及生产选路策略 MUST NOT 因替换基线而改变。

#### Scenario: TURN 建链成功但不比 WSS 快

- **WHEN** TURN ICE 检查成功，但应用层采样未满足稳定收益或健康门槛
- **THEN** 业务保持 WSS，后台探测仍受预算约束，不因 TURN 可达强制升级

#### Scenario: TURN 全部不可用

- **WHEN** 节点不可达、UDP 黑洞、协议不支持或配置为空
- **THEN** 用户仍可经 WSS 登录和使用全部基础功能，首屏不等待 ICE 超时；诊断显示未升级原因

#### Scenario: 交互升级时下载或端口转发

- **WHEN** 终端已通过直连或 TURN 工作，用户下载文件或访问 `/forward/*`
- **THEN** 这些请求继续走 frp WSS 并按网关批量类别准入，不占用交互预留，不建立独立 bulk 池

### Requirement: 混合路径续接与即时业务撤权

切换 SHALL 先验证身份与恢复位置，保持每条业务流单一输入写入方。终端 SHALL 使用 generation/offset 续接并去除重叠输出；聊天/列表 SHALL 按业务游标或权威快照恢复，MUST NOT 使用终端 offset 冒充其他业务恢复协议。

两条路径 MUST NOT 同时发送同一输入；执行结果未知的输入、聊天提交与非幂等请求 MUST NOT 自动重放。无法判定执行结果时 SHALL 向用户明确展示不确定状态。

会话过期、注销、解绑、换钥、邀请撤销或管理流断开 SHALL 立即关闭关联直连/TURN 业务连接，停止凭据刷新并释放可释放的 allocation；MUST NOT 仅等待 TURN 密码过期撤销应用访问。持久 PTY SHALL 保留。

#### Scenario: 输入执行结果未知时掉线

- **WHEN** 已发送的终端输入或聊天提交尚未得到确认，活动 WebRTC 路径断开
- **THEN** 按业务协议恢复可确认的状态，不经 frp WSS 自动重发该输入，向用户提示无法确认的结果

#### Scenario: TURN 仍可达但 control 断开

- **WHEN** TURN 数据通道正常而权威管理 WSS 断开或租约失效
- **THEN** 宿主立即撤销派生会话并关闭该数据通道，不允许绕过撤权继续访问，PTY 不退出

#### Scenario: 真实 TURN 终端交互中断后自动续接

- **WHEN** 发布验收在隔离夹具使用真实浏览器、coturn、frp 和宿主 PTY，强制 relay-only ICE，并记录实际选中、已提名且成功的候选对及 relay 候选，经该路径提交带序号的新输入并收到真实 PTY 回显；随后仅阻断活动 TURN 路径，排除其他 ICE 替代，保持管理与 frp 回源健康
- **THEN** 浏览器自动经真实 frp 工作连接续接同一 PTY generation/offset；连续输出无缺失或重复，已接受输入仅执行一次，回落后新输入得到真实 PTY 回显；记录检测、切换、续接及回显耗时，不以候选收集、连通探针或手工重连代替

#### Scenario: WebRTC 直连实际承载交互

- **WHEN** 真实浏览器和宿主在可直连拓扑中协商实际选中且已提名的非 relay 候选对，并满足既有健康及收益门槛
- **THEN** 新终端输入及真实 PTY 回显经该直连双向传输；登录、文件和 `/forward/*` 仍走 frp 基线，未满足收益门槛则交互留在基线

#### Scenario: 手机真实交互发布验收

- **WHEN** 验收包含 WebRTC 优化的发布版
- **THEN** SHALL 在真实手机及桌面浏览器完成同 GitHub 账户免 Token 打开、Vim/CLI agent、重输出、Ctrl-C、切换续接、权限撤销和长时稳定性验证，记录失败及未覆盖边界，不引用旧实验充当 TURN 验证
