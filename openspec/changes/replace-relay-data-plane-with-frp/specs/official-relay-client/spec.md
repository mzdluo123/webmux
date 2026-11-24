# Spec Delta

## MODIFIED Requirements

### Requirement: 新版绑定持久化与安全降级

Relay 的启用意图、控制端点、主机标识、完整公共 HTTPS Origin 和私钥 SHALL 使用登记的 v3 状态文件，保持目录 0700、文件 0600、原子写入及独立锁；MUST NOT 保存公开控制 `controltrust`。
状态损坏、不可读或锁冲突 SHALL 保留原文件、报告不可用并禁止新公网连接，不得回退 TOML 开启。旧 v1/v2 格式 MUST NOT 自动迁移或恢复绑定；部署 SHALL 先备份清理旧状态，保留机器 ID。
数据面分配 SHALL 每次经已认证控制面重新取得，不能作为手工第二端点或持久信任来源。新模式的分配、桥接密钥、短期业务令牌和回源材料 SHALL 仅用于当前运行代际，不写入 v3 状态；必要的令牌文件仅存于受限运行目录。
关闭落盘失败仍 SHALL 立即撤销公网授权并停止受管连接器，返回未持久化错误。TOML 仅提供现有引导配置，不保存绑定、私钥、旧别名或公开控制 pin；正常 v3 重启 SHALL 按持久启用意图重新认证并连接。

#### Scenario: 损坏设置阻止自动暴露

- **WHEN** 状态损坏且 TOML 配置启用
- **THEN** 本机服务正常启动，Relay 关闭并显示存储错误，不连接公网

#### Scenario: 关闭时存储写失败

- **WHEN** 关闭状态不能落盘
- **THEN** 当前连接与公共来源授权立即撤销，受管进程停止，响应指出未持久化

#### Scenario: 旧版绑定不再迁移

- **WHEN** 新宿主遇到 v1/v2 绑定状态
- **THEN** 拒绝恢复旧密钥及启用意图；完成备份清理后以未配对、关闭状态启动，独立机器 ID 不变

#### Scenario: 重启遇到上一代际的材料

- **WHEN** WebMux 异常退出后重启，并发现前次进程留下的运行文件
- **THEN** 宿主只清理可确认属于旧实例的资源；自动连接须重新认证并取得新代际，不复用旧分配、令牌或回源身份

### Requirement: 连接状态机与公网来源撤权

Relay 客户端 SHALL 区分关闭、连接中、就绪和失败状态，并单独表示未配对、配对中与已配对。旧模式 SHALL 继续区分 bulk 降级，且只在 control 认证与 stream 服务就绪后授权固定来源；仅 bulk 不就绪时，交互保持可用，新批量请求在 Relay 有界等待。
新模式 SHALL 区分管理与业务就绪：`controlReady` 来自当前认证管理 WSS 和有效租约，`dataReady` 来自当前授权分配经网关验证的实际回源。只有两者均有效才登记固定 HTTPS 来源为允许的 Host/Origin 并公布访问地址。
control 断开、绑定撤销、关闭或服务退出时，客户端 SHALL 立即撤销该来源的 Host/Origin 授权并关闭相应业务和派生连接。新连接认证失败 MUST NOT 替换或撤销既有合法 control。
旧模式仅 bulk 失败 SHALL 按同代际降级重连；新模式仅工作连接失败 SHALL 隔离受影响请求，不能据此宣告整个 bulk 传输降级。若新模式失去回源就绪证据，宿主 SHALL 撤销该来源授权，仍有效的管理连接可处理撤权和恢复。
启用状态下的意外断线 SHALL 按有界指数退避和随机抖动重连。本机服务、持久 PTY 与其他入口不受影响；代际检查 SHALL 阻止重复连接器、迟到就绪和旧配置重新授权。

#### Scenario: 断线后恢复

- **WHEN** 就绪的 Relay 因网络中断失效，随后网络恢复
- **THEN** 中断期间公共来源未授权；宿主自动恢复并在当前代际重新满足对应模式的就绪条件后，使用原固定地址重新授权

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
`/local/v1/status` 在新模式 SHALL 输出可选 `relayFrp` 对象，报告 `dataPlane=frp-wss`、`controlReady`、`dataReady` 和脱敏失败原因，不输出旧模式的 `relay`、`bulkReady` 或 `readyConnections`。灰度期仅旧模式输出既有 `relay` 对象。
状态与错误输出 MUST NOT 包含私钥、配对材料、GitHub 凭据、WebMux Token、业务 JWT、桥接密钥、回源私钥或携带凭据的 URL。

#### Scenario: 查看远程访问入口

- **WHEN** Relay 与 Cloudflare Tunnel 同时就绪
- **THEN** 访问 URL 汇总分别列出两个固定来源，撤销其中一个不会移除另一个

#### Scenario: 从 Relay 地址开启推送

- **WHEN** 手机通过就绪的 Relay HTTPS 地址打开 WebMux 并开启通知
- **THEN** 订阅关联该固定来源，重启 WebMux 并重连 Relay 后通知仍指向同一地址

#### Scenario: 业务回源故障期间查询状态

- **WHEN** 新模式的管理连接仍有效，但回源探测失败
- **THEN** 状态只在 `relayFrp` 报告实际管理与回源状态和脱敏原因，不把进程存活、旧角色就绪数或单次 WSS 握手当作业务就绪

### Requirement: 会话认证覆盖与撤销

Relay 会话 SHALL 覆盖 HTTP、WebSocket、P2P 信令和 `/forward/*`，前端 SHALL 通过现有 API/WS 封装识别登录态。Cookie 写请求 MUST 校验同源 Origin 与 CSRF token，转发上游前 MUST 剥离宿主认证 Cookie 和 CSRF 材料。
宿主注销、会话过期、对应后台会话撤销、绑定撤销、换钥、移出邀请名单或 Relay 关闭 SHALL 撤销相关宿主会话。宿主重启 SHALL 清空会话。撤销 SHALL 拒绝后续请求并关闭该会话建立的 WebSocket、P2P 和转发连接，但 MUST NOT 终止持久 PTY。
临时 control 断线 SHALL 立即暂停业务并关闭派生连接；有界内存恢复记录仅存至原期限，不能单独授权。
新连接就绪后，宿主 SHALL 经认证管理 WSS 调用 `login.resume`，复核原 grant、后台会话、所有者、邀请及绑定；取得精确绑定新 Scope 的授权后才恢复原 Cookie 对应的访问。
恢复 SHALL 保持原 grant ID、所有者、Origin 和绝对期限，不延长 12 小时上限。明确撤销与恢复并发时 SHALL 以撤销为准；旧访问上下文与旧 Scope SHALL 保持失效。前端暂时恢复期间 SHALL 保留原路由并有界退避查询，恢复成功后按原终端续接协议重连，不自动重放业务写请求。
业务隧道登录、续约和回源 mTLS MUST NOT 签发、恢复或延长浏览器 grant。单个浏览器 grant 或后台会话被撤销 SHALL 关闭其访问，不停止其他合法会话共享的受管业务传输；管理代际或绑定撤销 SHALL 关闭对应的整个隧道。

#### Scenario: 短暂断线后恢复登录

- **WHEN** control 连接短暂中断后以新连接代际恢复，原后台会话、邀请、绑定及 grant 仍有效
- **THEN** 断线期间业务访问被拒绝，旧派生连接关闭；相应模式实际就绪且 `login.resume` 成功后，原 Cookie 才可继续访问，绝对到期时间不变

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

宿主连接器 SHALL 仅使用认证分配的 WSS 端点，保留独立的 WebMux 管理连接、持钥证明、心跳、`login.exchange`、`login.resume`、ICE 与撤权消息。新模式 SHALL 直接在管理 WSS 承载管理消息，业务与管理均不使用 smux。
迁移期间，旧模式 SHALL 继续使用 `webmux-relay.v2` 与 WSS + smux v2；同代际认证一条 control 管理连接及固定 interactive/bulk 数据池，默认槽位数分别为二和四。旧槽位独立重连并保持 Scope；control 断开关闭池及派生访问，数据槽位丢失只关闭自身流。
新模式 SHALL 关闭 frp `tcpMux`，工作请求使用独立连接；不运行旧业务流协议、预建的 interactive/bulk 池、旧槽位重连或另一套业务传输。旧池参数仅在旧模式生效。交互与批量是 Relay 网关准入类别，保留交互预留与有界等待，不代表独立传输或就绪状态。
普通请求的业务并发 SHALL 由 Relay 单点准入，宿主保持必要的缓冲与超时边界；旧模式仍保持帧和流头限制。宿主与节点的本地套接字上限不能替代网关准入，也不能重复施加业务配额。
基线传输固定 WSS，不提供 auto/quic 或 UDP connector endpoint，不接收浏览器指定或节点自报的拨号地址。授权 ICE/TURN 仍按独立流程取得。
证书、pin、主机身份与协议错误 MUST NOT 通过重试绕过。网络重连 SHALL 使用有界退避，报告实际传输和脱敏 `reconnectReason`；仅旧模式报告角色就绪数。MUST NOT 自动重放非幂等业务请求。

#### Scenario: 续接后恢复可操作

- **WHEN** 网络恢复且原持久 PTY 仍存活，浏览器按 generation/offset 重新附加
- **THEN** SHALL 维持输出连续性且不重复提交已确认输入，以恢复后新输入的真实回显证明可操作；验收记录检测、建链、续接及回显耗时，不把手动重启夹具当作自动恢复

#### Scenario: UDP 黑洞不延迟 WSS 建连

- **WHEN** UDP 不可达而目标 TCP/443 可达
- **THEN** 客户端直接建立 WSS，不等待 QUIC 超时；实际可达性仍以 WSS 握手和相应模式的回源检查为准

#### Scenario: 错误证书保持失败

- **WHEN** 实际管理或业务 WSS 的证书、主机名或 pin 校验失败
- **THEN** 对应连接失败且报告脱敏安全错误，不发送凭据，不切换信任配置或其他传输

#### Scenario: 批量流中断不驱逐交互

- **WHEN** 新模式的大文件上传工作连接中断，其他工作连接、管理连接和回源仍健康
- **THEN** 仅取消相关传输并释放网关额度；已预留的交互容量继续可用，不重放上传，不产生 `bulkReady` 或旧角色计数

### Requirement: 控制面发现与数据面分配

宿主 SHALL 只需配置 `controlEndpoint` 与独立受众，按公开控制 CA-only 策略配对和发现数据面，不要求用户手工填写 `connectorUrl`、公开控制 pin 或第二个服务地址。
独立 `webmux-control` SHALL 是唯一发现来源，通过已认证控制通道提供绑定当前主机、受众、分配版本与来源元数据的 `nodeId`、能力、`connectorUrl`、数据面 origin、固定公共 HTTPS 地址及对应服务 TLS 信任。
发现结果 SHALL 来自管理员经控制服务校验并登记的用途化端点，不接受任意浏览器输入、节点自报或未认证下载扩展信任。只有具备 `wss` 能力且就绪的节点可作为连接器目标；`turn`-only 节点只提供 TURN，不暴露 WebMux HTTPS/WSS 入口，MUST NOT 被当作连接器目标。
宿主 SHALL 校验分配的主机、绑定代际、受众、版本及端点与服务身份的一致性。新模式仅在当前认证分配明确选定且宿主与节点均支持时生效；业务分配还 SHALL 绑定当前管理 Scope、节点、分配 ID、修订及有效期。
各类材料的有效期上限 SHALL 在签发端限制；接收端 SHALL 检查绝对到期时间，不根据本机时钟重新推算签发寿命上限。签名、单次 nonce、归属、受众、代际及本地操作超时 SHALL 保持独立校验；拒绝重放、过期和非当前分配。
已配对控制面的端点或模式调整 SHALL 经认证的版本化分配完成，并重新校验 TLS 与 pin。切换或灰度回滚 SHALL 先撤销旧代际、确认其路由与连接关闭，再启动新代际；每绑定仅一个活动模式，不以探测结果自行降级。
浏览器 SHALL 始终使用固定宿主 HTTPS Origin 及获授权的 ICE 配置，不自行选择 WSS 节点。获授权 TURN 可来自不同的 `turn`-only 节点。模式切换不改变公共 Origin、宿主绑定或配对密钥。

#### Scenario: 控制面与数据面分离部署

- **WHEN** 宿主只配置控制面地址和独立受众，按 CA-only 策略配对后，控制面为其分配另一 origin 的就绪 WSS 节点
- **THEN** 宿主经认证发现 `connectorUrl`、受众和对应 TLS 信任，完成标准 TLS 与 pin 校验后建连；新模式业务授权从该节点已认证的管理 WSS 取得，用户无需填写第二个端点

#### Scenario: TURN-only 节点不是 WSS 目标

- **WHEN** 控制面为浏览器授权独立 TURN-only 节点，同时为宿主分配 WSS 节点
- **THEN** 连接器仅拨号 WSS 节点，ICE 可使用独立 TURN 节点；若连接器分配指向不具备 `wss` 能力的节点则拒绝该分配，不尝试其管理通道

#### Scenario: 分配身份或信任不匹配

- **WHEN** 分配归属、受众、版本或代际不符，或实际 WSS 服务证书或 pin 与认证分配不符
- **THEN** 宿主拒绝建立对应连接或注册，不交付相关凭据，不接受浏览器或节点自报的替代地址/公钥，也不扩大信任后重试

#### Scenario: 从旧模式切换或灰度回滚

- **WHEN** 控制面以新修订切换单个绑定的活动模式
- **THEN** 旧代际停止授权且确认关闭后新模式才可上线；固定 Origin 和配对不变，迟到状态不能恢复旧授权，未通过新模式门禁不会触发自动回落

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

浏览器 SHALL 在 WSS 已可用后，以有界频率比较 WSS 与候选直连/TURN 的应用层往返及健康状态。只有连续采样满足收益与健康门槛才升级；ICE 成功或候选类型本身 MUST NOT 作为升级依据。
活动路径 SHALL 持续检测健康，失败立即回落 WSS，性能退化按滞回策略回落；实现 SHALL 固定并验证收益阈值、采样窗口、冷却期与探测资源预算，防止频繁切换。
仅显式允许的终端、聊天、列表交互业务 SHALL 使用升级路径。文件上传下载、全部 `/forward/*`、登录、设置、语音与其他未分类业务 SHALL 保持 WSS；批量流量 MUST NOT 绕入交互通道。

#### Scenario: TURN 建链成功但不比 WSS 快

- **WHEN** TURN ICE 检查成功，但应用层采样未满足稳定收益或健康门槛
- **THEN** 业务保持 WSS，后台探测仍受预算约束，不因 TURN 可达强制升级

#### Scenario: TURN 全部不可用

- **WHEN** 节点不可达、UDP 黑洞、协议不支持或配置为空
- **THEN** 用户仍可经 WSS 登录和使用全部基础功能，首屏不等待 ICE 超时；诊断显示未升级原因

#### Scenario: 交互升级时下载或端口转发

- **WHEN** 终端已通过直连或 TURN 工作，用户下载文件或访问 `/forward/*`
- **THEN** 这些请求继续走 WSS 并按网关 bulk 类别准入，不占用交互预留；新模式不为此建立独立 bulk 池

