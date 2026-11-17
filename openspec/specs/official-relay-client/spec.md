# official-relay-client Specification

## Purpose

WebMux 宿主通过独立中继密钥与官方 Relay 配对，以固定 HTTPS 地址提供低配置公网访问，并在连接断开时立即撤销入口授权、自动恢复连接，同时保持现有 Cloudflare、Tailscale、Web Push 和 P2P 能力的独立行为。
设备配对、机器身份与页内设置操作遵循 [official-relay-pairing](../official-relay-pairing/spec.md)。

## Requirements

### Requirement: 默认关闭与公网启用前提

官方 Relay SHALL 默认关闭，且与 Cloudflare Tunnel、Tailscale 的开关和凭据相互独立。只有完成主机配对、设置存储可用且当前 WebMux 登录 Token 不少于 43 字符时，用户才允许启用 Relay。

启用前 SHALL 显示公网 Shell 风险确认。启用失败 MUST NOT 阻止本机 HTTP 服务或现有终端会话；运行中启用或关闭 MUST NOT 重启 WebMux。

#### Scenario: 未配对时尝试启用

- **WHEN** 主机尚未完成 Relay 配对，用户请求启用
- **THEN** 系统拒绝启动连接器并说明需要先配对，不建立任何公网连接

#### Scenario: 登录 Token 过弱

- **WHEN** 已配对主机当前登录 Token 少于 43 字符
- **THEN** 启用被拒绝并给出修复方式，本机服务继续运行

### Requirement: 新版绑定持久化与安全降级

Relay 的启用意图、控制端点、主机标识、完整公共 HTTPS Origin 和私钥 SHALL 使用登记的 v3 状态文件，保持目录 0700、文件 0600、原子写入及独立锁；MUST NOT 保存公开控制 controltrust。
状态损坏、不可读或锁冲突 SHALL 保留原文件、报告不可用并禁止新公网连接，不得回退 TOML 开启。旧 v1/v2 格式 MUST NOT 自动迁移或恢复绑定；部署 SHALL 先备份清理旧状态，保留机器 ID。
数据面分配 SHALL 每次经已认证控制面重新取得，不作为手工第二端点或持久信任来源。关闭落盘失败仍 SHALL 立即停止公网授权并返回未持久化错误。TOML 仅提供现有引导配置，不保存绑定、私钥、旧别名或公开控制 pin；正常 v3 重启 SHALL 按持久启用意图连接。

#### Scenario: 损坏设置阻止自动暴露

- **WHEN** 状态损坏且 TOML 配置启用
- **THEN** 本机服务正常启动，Relay 关闭并显示存储错误，不连接公网

#### Scenario: 关闭时存储写失败

- **WHEN** 关闭状态不能落盘
- **THEN** 当前连接与公共来源授权立即撤销，响应指出未持久化

#### Scenario: 旧版绑定不再迁移

- **WHEN** 新宿主遇到 v1/v2 绑定状态
- **THEN** 拒绝恢复旧密钥及启用意图；完成备份清理后以未配对、关闭状态启动，独立机器 ID 不变

### Requirement: 连接状态机与公网来源撤权

Relay 客户端 SHALL 区分关闭、连接中、就绪、bulk 降级和失败状态，并单独表示未配对、配对中与已配对。只有 control 认证和 stream 服务均就绪后，客户端才将固定 HTTPS 来源登记为允许的 Host/Origin 并公开地址；bulk 未就绪时 SHALL 显示降级，新批量请求在 Relay 有界等待容量。

control 断开、绑定撤销、关闭或服务退出时，客户端 SHALL 立即撤销该来源的 Host/Origin 授权。新连接认证失败 MUST NOT 替换或撤销既有合法 control；仅 bulk 失败 SHALL 按同代际降级重连。迟到的连接事件 MUST NOT 在关闭或新配置后重新授权旧地址。

启用状态下发生意外断线时，客户端 SHALL 按有界指数退避和随机抖动自动重连；本地会话和其他远程入口不受影响。代际或等效机制 SHALL 防止并行重连产生多个活动连接器。

#### Scenario: 断线后恢复

- **WHEN** 就绪的 Relay 连接因网络中断失败，随后网络恢复
- **THEN** 中断期间公共来源被撤销，客户端自动重连，成功后使用原固定地址重新授权

#### Scenario: 关闭期间收到迟到就绪事件

- **WHEN** 用户已关闭 Relay，先前连接随后报告就绪
- **THEN** 该事件被丢弃，公共来源保持未授权，不创建新连接器

### Requirement: 经 Relay 的完整应用行为与 P2P 基线

Relay 就绪后，浏览器 SHALL 能通过公共地址使用现有登录、会话管理、终端回放与续接、聊天、语音、设置、上传下载和 `/forward/*` 端口转发。Relay 传输 MUST NOT 绕过 WebMux 的认证、Host、Origin 或会话内进程限制；认证可使用既有 Token 或下述绑定 Relay Origin 的有效登录会话。

浏览器 SHALL 把 Relay 视为 HTTP/WebSocket 基线，首屏、登录和信令不等待 WebRTC。宿主与浏览器 SHALL 在授权范围内支持直连及 TURN 候选；保留既有直连校验，不得把 TURN 配置应用到其他独立公网入口。

关闭 Relay SHALL 撤销 Relay 基线入口及派生的直连/TURN 会话，但 MUST NOT 暗中修改独立 P2P 配置或终止持久 PTY。

#### Scenario: Relay 上完成 P2P 升级与回落

- **WHEN** 浏览器经 Relay 打开 WebMux，后台直连或 TURN 路径通过健康与延迟收益检查，随后该路径断开
- **THEN** 首屏先经 WSS 可用，交互升级后可回落到同一 Relay 地址，恢复输出且不双发输入

#### Scenario: Relay 不绕过认证

- **WHEN** 未携带有效 WebMux Token 或 Relay 登录会话的浏览器经 Relay 请求 `/api/sessions` 或 `/ws/sessions`
- **THEN** WebMux 返回与直连公网入口一致的 401，Relay 不以代理头或注入长期 Token 代替宿主认证

### Requirement: 固定访问地址、状态与推送来源

Relay 就绪后，固定公共地址 SHALL 出现在现有访问 URL 汇总、本机状态和 Relay 设置页，且地址与二维码 MUST NOT 自动附带 WebMux Token。Windows 托盘 SHALL 通过现有通用访问入口展示该地址，无需新增 Relay 专属菜单协议。

配对的固定 HTTPS 来源 SHALL 纳入现有 Web Push 来源管理。新订阅只能从当前通过 Host/Origin 校验的入口开启；Relay 离线期间已有通知可以保留，点击后等待入口恢复。

状态与错误输出 MUST NOT 包含私钥、配对材料、GitHub 凭据或 WebMux Token。

#### Scenario: 查看远程访问入口

- **WHEN** Relay 与 Cloudflare Tunnel 同时就绪
- **THEN** 访问 URL 汇总分别列出两个固定来源，撤销其中一个不会移除另一个

#### Scenario: 从 Relay 地址开启推送

- **WHEN** 手机通过就绪的 Relay HTTPS 地址打开 WebMux 并开启通知
- **THEN** 订阅关联该固定来源，重启 WebMux 并重连 Relay 后通知仍指向同一地址

### Requirement: GitHub 跳转与宿主浏览器会话

宿主 SHALL 显式信任已配对控制面的所有者认证，并保存所有者 GitHub 数字 ID 与绑定代际。
该认证所派生的宿主登录会话 SHALL 是绑定浏览器及 Relay Origin 的所有者浏览器会话，不是宿主设备身份或连接器 control 会话。
独立直连、Cloudflare 与 Tailscale 入口 SHALL 保持既有 Token 认证；Relay 浏览器会话 MUST NOT 在其他 Origin 使用。

宿主 SHALL 为后台跳转创建绑定浏览器的单次 state 和 PKCE S256 事务，有效期不超过 5 分钟；回调必须校验事务，并通过宿主密钥认证的控制通道携 verifier 兑换 code。账号、host ID、受众或绑定代际不符 SHALL 拒绝登录，MUST NOT 仅相信浏览器携带的身份声明。

成功后宿主 SHALL 创建绝对有效期不超过 12 小时的内存会话，使用 host-only、Secure、HttpOnly、SameSite=Lax、Path=/ 的 `__Host-` Cookie；MUST NOT 写入 localStorage、sessionStorage 或 URL。回调 SHALL 禁缓存、禁止 Referer 泄露并跳至无凭据 URL。长期 WebMux Token MUST NOT 参与跳转或自动下发。

#### Scenario: 新设备免 Token 打开面板

- **WHEN** 所有者在新设备登录官方后台并点击自己的在线主机
- **THEN** 宿主验证一次性授权并打开可操作的面板，不要求输入 WebMux Token，不向浏览器下发该 Token

#### Scenario: 回调被其他浏览器或主机使用

- **WHEN** code 在没有对应临时 Cookie/state 的浏览器使用，或兑换身份与本机绑定不符
- **THEN** 宿主拒绝建立会话且不改变任何绑定，用户可重新开始登录

### Requirement: 会话认证覆盖与撤销

Relay 会话 SHALL 覆盖 HTTP、WebSocket、P2P 信令和 `/forward/*`，前端 SHALL 通过现有 API/WS 封装识别登录态。Cookie 写请求 MUST 校验同源 Origin 与 CSRF token，转发上游前 MUST 剥离宿主认证 Cookie 和 CSRF 材料。

宿主注销、会话过期、对应后台会话撤销、绑定撤销、换钥、移出邀请名单、Relay 关闭或断线 SHALL 撤销相关宿主会话。宿主重启 SHALL 清空会话。撤销 SHALL 拒绝后续请求并关闭该会话建立的 WebSocket、P2P 和转发连接，但 MUST NOT 终止持久 PTY。

#### Scenario: 关闭 Relay 后旧会话不能复用

- **WHEN** 用户关闭后重新启用 Relay，再使用旧会话 Cookie 访问
- **THEN** 旧会话被拒绝，用户需重新登录 GitHub 后台，主机配对和持久 PTY 保留

#### Scenario: 端口转发不泄露登录会话

- **WHEN** 已登录用户通过 `/forward/*` 访问目标应用
- **THEN** 目标应用收不到宿主认证 Cookie 或 CSRF 材料，跨源伪造写请求被拒绝

#### Scenario: 已升级 P2P 的会话过期

- **WHEN** Relay 登录会话过期且浏览器仍有活动 P2P 或 WebSocket 连接
- **THEN** 相关访问连接被关闭、后续请求被拒绝，持久 PTY 继续运行

### Requirement: WSS 连接隔离与重连诊断

宿主连接器 SHALL 使用已认证控制面分配的 `connectorUrl` 与 WSS + smux v2，基线传输固定 wss，不提供 auto/quic 或 UDP connector endpoint。
宿主 MUST NOT 接受浏览器指定或节点自行宣告的拨号地址；授权的 ICE/TURN 配置 SHALL 按下述独立流程获取。

宿主 SHALL 使用 webmux-relay.v2，在同代际认证一条 control 管理连接及固定 interactive/bulk 数据池，默认数据连接数为二和四。
宿主 SHALL 按固定槽位独立重连并保持 Scope；control 断开 SHALL 撤销派生访问并关闭数据池，数据连接丢失 SHALL 只关闭自身流。
普通流并发 SHALL 由 Relay 单点准入，宿主 SHALL 保留帧、缓冲和流头超时边界而不重复业务配额拒绝。

证书、pin、主机身份与协议错误 MUST NOT 通过重试绕过。网络重连 SHALL 使用有界退避，报告实际传输、角色就绪数与脱敏 reconnectReason；MUST NOT 自动重放非幂等业务请求。

#### Scenario: 续接后恢复可操作

- **WHEN** 网络恢复且原持久 PTY 仍存活，浏览器按 generation/offset 重新附加
- **THEN** SHALL 维持输出连续性且不重复提交已确认输入，以恢复后新输入的真实回显证明可操作；验收记录检测、建链、续接及回显耗时，不把手动重启夹具当作自动恢复

#### Scenario: UDP 黑洞不延迟 WSS 建连

- **WHEN** UDP 不可达而目标 TCP 443 可达
- **THEN** 客户端直接建立 WSS，不等待 QUIC 超时；实际可达性仍以 WSS 握手结果为准

#### Scenario: 错误证书保持失败

- **WHEN** WSS 服务证书或 pin 校验失败
- **THEN** 连接失败并报告安全错误，不切换信任配置或其他传输

### Requirement: 控制面发现与数据面分配

宿主 SHALL 只需配置 `controlEndpoint` 与独立受众，按公开控制 CA-only 策略配对和发现数据面，不要求用户手工填写 `connectorUrl`、公开控制 pin 或第二个服务地址。
独立 `webmux-control` SHALL 是唯一发现来源，通过已认证控制通道提供绑定当前主机、受众、分配版本与来源元数据的 `nodeId`、能力、`connectorUrl`、数据面 origin、固定公共 HTTPS 地址及对应服务 TLS 信任。

发现结果 SHALL 来自管理员经控制服务校验并登记的用途化端点，不接受任意浏览器输入、节点自报或未认证下载扩展信任。
宿主 SHALL 校验分配的主机/绑定代际、受众、版本及端点与服务身份的一致性，拒绝错误归属、重放或过期分配。
只有具备 `wss` 能力且就绪的节点可作为连接器目标；`turn`-only 节点只提供 TURN，不暴露 WebMux HTTPS/WSS 入口，MUST NOT 被当作连接器目标。

已配对控制面的数据面端点调整 SHALL 经认证、版本化分配完成，重新校验 TLS 与 pin 后切换，并使旧代际事件不能恢复旧入口授权；不得自动学习未知公钥或把数据面分配当作控制面信任变更。
浏览器 SHALL 始终使用固定宿主 HTTPS 来源及获授权的 ICE 配置，不自行选择 WSS 节点。获授权 TURN 可来自与 WSS 不同的 `turn`-only 节点，不要求两者同机。

#### Scenario: 控制面与数据面分离部署

- **WHEN** 宿主只配置控制面地址和独立受众，按 CA-only 策略配对后，控制面为其分配另一 origin 的就绪 WSS 节点
- **THEN** 宿主经认证发现 `connectorUrl`、受众和对应 TLS 信任，完成标准 TLS 与 pin 校验后建连，用户无需填写第二个端点

#### Scenario: TURN-only 节点不是 WSS 目标

- **WHEN** 控制面为浏览器授权独立 TURN-only 节点，同时为宿主分配 WSS 节点
- **THEN** 连接器仅拨号 WSS 节点，ICE 可使用独立 TURN 节点；若连接器分配指向不具备 wss 能力的节点则拒绝该分配，不尝试其管理通道

#### Scenario: 分配身份或信任不匹配

- **WHEN** 分配归属/受众/版本不符，或数据面返回与已认证分配不匹配的证书或 pin
- **THEN** 宿主拒绝分配或连接，不向该端点发送宿主凭据，不接受浏览器或节点自报的替代地址/公钥，也不扩大信任后重试

### Requirement: 内置官方端点与公开服务身份

开源客户端 SHALL 内置官方控制面 HTTPS `controlEndpoint` 与控制面受众；宿主公开控制连接 SHALL 使用 Go 系统 CA（可追加 RootCA）、域名、有效期和 TLS 1.3 校验，MUST NOT 额外要求控制 SPKI pin 或固定 CDN 边缘证书曲线。
用户 SHALL 无需填写官方地址、公钥或 OAuth 应用配置即可发起 GitHub 配对；数据面地址、受众及服务 TLS 信任由已认证控制面分配。
内置默认 MUST NOT 自动启用 Relay 或替代公网确认。

公开控制 HTTPS SHALL 允许 Cloudflare CDN 终止 TLS；CDN SHALL 被明确列为控制内容的可信处理方，不宣称宿主到源站端到端 TLS。分配 WSS/TURN SHALL 保持 DNS-only 直连，节点 mTLS、节点 SPKI 与分配服务身份验证 MUST NOT 放宽。
宿主 WSS SHALL 同时通过 TLS 1.3、证书链、有效期、主机名及分配 ECDSA P-256 SPKI pin 校验，缺失/错误 pin MUST 拒绝；MUST NOT 跳过校验、自动学习未知公钥或失败降级。数据面信任仅来自已认证有效控制分配，宿主私钥仍逐机生成并用于主机认证。
控制面及其 CDN TLS 终止方 SHALL 被视为首次分配 connectorUrl、公共来源及 pin 的可信授权方；分配 pin MUST NOT 被描述为可防御该控制信任边界内的分配篡改。

官方私钥、OAuth Client Secret 和用户 GitHub access token MUST NOT 进入开源仓库、客户端二进制或公开 API。OAuth Client ID SHALL 视为公开标识，不作为客户端认证秘密。

#### Scenario: 使用官方默认配置配对

- **WHEN** 用户安装带有有效官方控制地址的开源客户端并选择官方 Relay
- **THEN** 客户端使用内置地址与标准 CA/TLS 校验控制服务，用户无需填写控制 pin 或 OAuth 凭据，仍须完成 GitHub 配对与启用确认

#### Scenario: 控制 CDN 正常换钥

- **WHEN** 控制 CDN 更换边缘证书或公钥，新证书仍通过 CA、域名、有效期及 TLS 1.3 校验
- **THEN** 宿主继续访问公开控制 endpoint，不要求发布控制 pin 更新；数据面与节点 pin 策略不变

#### Scenario: 公开控制 TLS 校验失败

- **WHEN** 控制证书不受信、过期、域名不匹配，或无法使用 TLS 1.3
- **THEN** 客户端拒绝连接，不提交配对材料或宿主凭据，不跳过校验或通过重连绕过错误

### Requirement: 数据面公钥轮换与自建控制隔离

数据面公钥轮换 SHALL 通过已信任控制面的有效版本化分配明确授权，正常维护采用当前与下一公钥的重叠窗口，不得从目标证书自动学习；节点身份换钥 SHALL 保持先登记新 pin/代际再切换。
数据面或节点公钥泄露后的恢复 SHALL 更新相应信任；若需要客户端升级，MUST 明确提示，MUST NOT 声称离线客户端立即撤销旧公钥。公开控制证书续期/换钥 SHALL 使用标准 CA/TLS 策略，不保留控制 pin 轮换流程。

自建控制面 `controlEndpoint` 覆盖 SHALL 显式配置独立服务受众和适用 CA 环境，仍只使用公开控制 CA-only 校验，不接受旧 InitialTrust 配置别名。
变更控制面或受众 SHALL 先撤销旧连接、公共来源和会话，确认后重新配对；旧绑定、宿主凭据及未完成授权材料 MUST NOT 发送到新服务。
已信任控制面内的数据面版本化调整 SHALL 遵循严格分配与 pin 校验，不要求用户手工维护数据面信任，也不得借此替换控制面引导配置。

#### Scenario: 已分配数据面公钥正常轮换

- **WHEN** 有效版本化分配授权当前与下一数据面公钥，节点切换到下一公钥
- **THEN** 宿主按分配继续连接；未获分配授权的新公钥仍拒绝，不从证书自动学习

#### Scenario: 从官方切换到自建服务

- **WHEN** 用户将官方默认配置覆盖为自建控制地址和独立受众
- **THEN** 旧连接和会话失效，新服务通过 CA-only 校验后仍要求独立配对，不能取得旧服务的绑定凭据

### Requirement: 短期 ICE 配置与数据通道授权

浏览器 SHALL 经已认证的同源宿主接口获取 ICE 配置，宿主 SHALL 经密钥认证 control 通道申请两端独立的短期 TURN 凭据。
配置 SHALL 绑定当前账号、host ID、绑定/连接器代际及浏览器会话，包含明确到期时间，仅留内存且响应禁缓存；TURN 节点及端点仅来自控制面授权，可以独立于宿主所连 WSS 节点。

两端凭据申请 SHALL 以当前有效 GitHub 所有者登录所派生的浏览器会话为前提；仅有宿主密钥、设备身份、在线连接器 control 会话或非会话本机客户端权限均不得领取中继凭据。
客户端 SHALL 使用标准 TURN 认证，不依赖匿名 allocation；短期凭据只用于该浏览器会话协商的路径，不提供可复制的通用代理配置。

宿主 SHALL 将 PeerConnection、协商的 SDP/DTLS 指纹与单次建链授权绑定到该浏览器会话，验证后才放行业务。TURN 凭据 MUST NOT 替代宿主认证，MUST NOT 允许仅持有 ICE 配置的客户端访问终端或任意回源。

凭据刷新 SHALL 重新验证授权，失败不得领取新凭据。coturn 存量会话可能使用原认证继续刷新 allocation；客户端 SHALL 在凭据到期或刷新失败时主动关闭受影响连接并回落 WSS，MUST NOT 假定服务端因密码到期已关闭资源。长期共享密钥及短期凭据 MUST NOT 进入日志、诊断、URL、浏览器持久化或公开状态。

#### Scenario: 未授权浏览器索取 TURN 配置

- **WHEN** 浏览器无由有效 GitHub 所有者登录派生的浏览器会话，或请求其他账号/代际的 ICE 配置
- **THEN** 请求被拒绝，不签发凭据，不改变现有连接

#### Scenario: 配对后尚无用户登录

- **WHEN** 宿主已配对且 control 在线，但没有有效所有者浏览器会话
- **THEN** WSS 基础服务保持就绪，宿主不能仅凭设备身份获取 TURN 凭据

#### Scenario: TURN 凭据过期或刷新失败

- **WHEN** 两端短期凭据到期或刷新无法完成
- **THEN** 客户端停止使用受影响配置并主动关闭相关连接，交互回落 WSS；不阻塞基线或绕过认证，不将客户端关闭等同于已证明所有节点资源释放

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
- **THEN** 这些请求继续走 WSS bulk，不占用交互数据通道

### Requirement: 混合路径续接与即时业务撤权

切换 SHALL 先验证身份与恢复位置，保持每条业务流单一输入写入方。终端 SHALL 使用 generation/offset 续接并去除重叠输出；聊天/列表 SHALL 按业务游标或权威快照恢复，MUST NOT 使用终端 offset 冒充其他业务恢复协议。

两条路径 MUST NOT 同时发送同一输入；执行结果未知的输入、聊天提交与非幂等请求 MUST NOT 自动重放。无法判定执行结果时 SHALL 向用户明确展示不确定状态。

会话过期、注销、解绑、换钥、邀请撤销或 control 断开 SHALL 立即关闭关联直连/TURN 业务连接，停止凭据刷新并释放可释放的 allocation；MUST NOT 仅等待 TURN 密码过期撤销应用访问。持久 PTY SHALL 保留。

#### Scenario: 输入执行结果未知时掉线

- **WHEN** 已发送的终端输入或聊天提交尚未得到确认，活动 WebRTC 路径断开
- **THEN** 按业务协议恢复可确认的状态，不在 WSS 自动重发该输入，向用户提示无法确认的结果

#### Scenario: TURN 仍可达但 control 断开

- **WHEN** TURN 数据通道正常而权威 WSS control 断开或超时
- **THEN** 宿主立即撤销派生会话并关闭该数据通道，不允许绕过撤权继续访问，PTY 不退出

#### Scenario: 手机真实交互发布验收

- **WHEN** 验收混合数据面的生产实现
- **THEN** SHALL 在真实手机及桌面浏览器完成同 GitHub 账户免 Token 打开、Vim/CLI agent、重输出、Ctrl-C、切换续接、权限撤销和长时稳定性验证，记录失败及未覆盖边界，不引用旧实验充当 TURN 验证
