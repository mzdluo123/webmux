# official-relay-service Specification

## Purpose

官方 Relay 为经过账号配对的 WebMux 主机分配固定 HTTPS 入口，并通过宿主主动建立的出站连接转发公网 HTTP 与 WebSocket 流量，使用户无需公网 IP、域名或第三方 Tunnel 即可远程访问自己的终端。

本规范的“Relay 服务”包含独立控制面 `webmux-control` 与数据面 `webmux-relay`；数据面镜像包含独立 coturn `turnserver`，按节点能力运行，不表示控制面、Relay 与 coturn 合为同一进程。

节点管理、配置/秘密交付、按能力部署及冷启动要求见 [official-relay-node-management](../official-relay-node-management/spec.md)；本规范保留用户授权、应用转发及 TURN 访问规则，两者共同构成服务验收。

## Requirements

### Requirement: 账号配对与单主机绑定

Relay SHALL 支持以 GitHub 设备授权完成主机配对。未配对的宿主 SHALL 能提交新生成的公钥并取得一次性用户码、验证地址和轮询句柄；只有用户在有效期内完成授权后，Relay 才建立账号与公钥绑定。
每个账号 SHALL 同时最多绑定一台主机。已有绑定的账号再次配对时，Relay SHALL 要求显式重新绑定，不得静默增加第二台主机；重新绑定 SHALL 保留原公共主机名、撤销旧公钥并断开旧连接器。
用户码和轮询句柄 SHALL 单次使用且有有效期。Relay SHALL 对配对创建与轮询实施速率限制，且 MUST NOT 要求或接受 WebMux 登录 Token。
MVP SHALL 使用 GitHub 账号允许列表实施邀请制。未获邀请的账号 SHALL 在授权完成后收到明确拒绝，不占用主机绑定或公共主机名。

#### Scenario: 完成首次配对

- **WHEN** 宿主提交新公钥，用户在有效期内通过 GitHub 完成授权
- **THEN** Relay 将账号的数字身份与公钥绑定，并返回固定公共 HTTPS 地址

#### Scenario: 过期或重复使用的配对码

- **WHEN** 配对码已过期、已使用或轮询句柄无效
- **THEN** Relay 拒绝建立或修改绑定，并要求重新发起配对

#### Scenario: 未邀请账号完成授权

- **WHEN** 不在允许列表中的 GitHub 账号完成设备授权
- **THEN** Relay 返回邀请制拒绝原因，不创建绑定或分配公共主机名

#### Scenario: 显式重新绑定丢失的密钥

- **WHEN** 已绑定账号确认用新公钥替换旧公钥
- **THEN** 公共主机名保持不变，旧私钥不能再建立连接，旧活动连接被关闭

### Requirement: 主机身份与固定公共主机名

Relay SHALL 为每台主机分配具有至少 128 位随机熵的 DNS 安全公共标识，并将标识、账号数字身份与主机公钥持久化。公共标识 MUST NOT 由 WebMux Token、GitHub Token 或私钥派生。
宿主连接时，Relay SHALL 使用随机质询验证签名。签名 transcript SHALL 绑定随机数、主机标识、协议版本、Relay 受众、绑定代际、连接器代际与 control/bulk 角色，防止重放到其他角色、会话或服务。无效签名 MUST NOT 替换该主机的现有有效连接。
除显式释放主机名外，重新启动和重新绑定密钥 SHALL 保持公共地址不变。

#### Scenario: 错误私钥不能劫持主机名

- **WHEN** 攻击者使用已知公共主机标识和错误私钥连接
- **THEN** Relay 拒绝认证，原绑定和现有活动连接不受影响

#### Scenario: 重放已捕获的认证响应

- **WHEN** 攻击者在另一次连接或另一 Relay 受众中重放旧签名
- **THEN** Relay 因随机数或受众不匹配而拒绝连接

### Requirement: 单活动连接器与撤销

每个主机标识 SHALL 同时只有一个活动逻辑连接器，内部最多包含 control 与 bulk 两条已认证 WSS/smux v2 物理连接。角色、绑定及连接器代际 SHALL 纳入签名；同代际重复角色 SHALL 拒绝。新的合法 control 认证后 SHALL 原子替换整个旧代际；未认证连接不得影响旧连接。
Relay SHALL 只把公共请求路由到当前已认证且处于活动状态的逻辑连接器。control 断开、绑定撤销或公钥替换后，对应公共入口 SHALL 立即停止转发；后续请求返回不可用响应，不得转入其他主机。仅 bulk 失败 SHALL 按数据面要求降级，不能顶掉合法 control。

#### Scenario: 同一主机的重复连接

- **WHEN** 同一私钥建立第二条有效 control 连接
- **THEN** 新 control 成为唯一活动逻辑连接器，旧代际的 control 与 bulk 均正常关闭；同代际 bulk 接入不得触发此替换

#### Scenario: 主机断开

- **WHEN** 活动连接器网络断开且保活超时
- **THEN** 公共主机名不再路由到该连接，请求返回 503，其他主机不受影响

### Requirement: 完整 HTTP 与 WebSocket 反向代理

Relay SHALL 按精确公共 Host 路由请求，并保留原始 Host 与同源 Origin 供 WebMux 继续校验。Relay SHALL 清理客户端伪造的转发头，不得把请求转发到客户端指定的任意地址。
代理 SHALL 支持 WebMux 使用的 REST JSON、二进制下载、流式请求与响应、multipart 上传、WebSocket Upgrade、长期终端与聊天连接，以及 `/forward/*` 端口转发。状态码、非逐跳响应头和正文 SHALL 原样传递；请求体和响应体 MUST NOT 被完整缓存在内存中。
连接器离线、stream 配额耗尽或目标连接失败时，Relay SHALL 返回明确的 502 或 503，不得伪造成 WebMux 业务响应。

#### Scenario: 通过 Relay 使用终端

- **WHEN** 浏览器经公共 HTTPS 地址登录并建立 `/ws/s/{id}`，随后发送输入并接收回放与实时输出
- **THEN** 请求经宿主出站连接到达正确的 WebMux，WebSocket 帧、关闭码和终端字节保持原有语义

#### Scenario: 大文件上传期间执行其他请求

- **WHEN** 浏览器正在经 Relay 上传大文件，同时发起会话列表和终端 WebSocket 请求
- **THEN** 各请求使用独立 stream，上传按背压传输且不阻塞其他请求

#### Scenario: 未知或离线主机名

- **WHEN** 请求使用未登记 Host，或目标主机没有活动连接器
- **THEN** Relay 不建立到任意地址的连接，并分别返回未找到或 503

### Requirement: 流控、配额与故障隔离

Relay SHALL 提供可配置的配对速率、每主机活动 stream 数、每连接缓冲和账号配额限制。超出配对速率时 SHALL 返回 429；超出 stream 配额时 SHALL 拒绝新 stream，并为受影响请求返回 503。
所有等待认证、建立 stream、读写空闲连接和关闭连接的操作 SHALL 有适用于其语义的期限或保活检测。长期 WebSocket MUST NOT 因普通的短 HTTP 读取超时被断开。
单个慢浏览器、慢宿主或异常 stream SHALL 受有界缓冲和背压约束，不得无限占用内存或阻塞其他 stream 与主机。Relay SHALL 在无法取得进展时只关闭受阻连接或 stream。

#### Scenario: 慢客户端不耗尽服务内存

- **WHEN** 某主机持续产生输出而浏览器停止读取，缓冲达到配置上限
- **THEN** Relay 关闭该慢连接或 stream，其他主机和请求继续运行，内存保持有界

#### Scenario: 超过单主机 stream 配额

- **WHEN** 某主机达到活动 stream 上限后继续收到新请求
- **THEN** 新请求收到 503，已有终端流不被全局中断

### Requirement: 注册数据持久化与分进程恢复

控制面 SHALL 将账号绑定、主机公钥、公共标识和撤销状态持久化到 PostgreSQL，并使用事务和唯一约束保证账号唯一绑定、重新绑定原子性及授权单次消费；MUST NOT 使用 SQLite 回退。
控制面重启后 SHALL 恢复注册数据，但不直接重启独立数据面；重启前后台会话和 code SHALL 失效并传播撤销，不能承诺相关活动访问无中断。
数据面重启 SHALL 清空活动连接器，宿主必须重新认证。数据库不可用或迁移失败 SHALL 使控制面未就绪并拒绝新配对、登录、授权兑换和连接授权；MUST NOT 用不完整状态分配主机名或旧缓存绕过撤销。
GitHub 访问凭据 SHALL 只用于授权时读取账号数字身份，MVP MUST NOT 长期保存该凭据。

#### Scenario: 控制面独立重启

- **WHEN** `webmux-control` 重启而数据面仍运行
- **THEN** 原公共地址与公钥绑定保留，旧会话及 code 失效；数据面按控制失联规则撤权并重新同步，不要求重新配对

#### Scenario: Relay 数据进程重启

- **WHEN** `webmux-relay` 重启而控制面未重启
- **THEN** 活动连接器中断，节点重新取得配置、秘密与授权同步后，宿主重新认证恢复转发，不改变绑定或自动重启控制面

#### Scenario: 重新绑定发生并发请求

- **WHEN** 同一账号同时提交两次重新绑定确认
- **THEN** 事务只允许一个最终公钥绑定，公共主机名不会重复分配或丢失

### Requirement: 敏感数据与信任边界

Relay SHALL 在公网入口终止浏览器 TLS，并在宿主出站连接上使用 TLS。运营文档与界面 SHALL 明确 Relay 能访问经其转发的 Bearer Token、WebSocket 子协议、Cookie 和终端内容，本版本 MUST NOT 宣称端到端加密。
公开控制 HTTPS 与数据面 SHALL 保持不同信任边界：

- 公开控制 HTTPS SHALL 允许 Cloudflare CDN 终止 TLS，CDN 属于配对、后台会话与授权控制内容的可信处理边界。
- 宿主 SHALL 使用 Go 系统 CA（可追加 RootCA）、域名、有效期及 TLS 1.3 验证该 endpoint，不附加控制 SPKI pin。
- 此政策 MUST NOT 放宽节点 mTLS/SPKI、分配 WSS pin 或 TURN 服务身份验证；WSS/TURN 保持 DNS-only 直连。

Relay MUST NOT 记录 WebMux Token、GitHub Token、配对码、签名原文、Authorization、Cookie、请求或响应正文、WebSocket 帧及终端内容。运行日志 SHALL 只包含必要的主机标识、状态、字节计数、错误类别和时间等脱敏元数据。

#### Scenario: 认证失败日志

- **WHEN** 宿主使用错误凭据连接或浏览器请求未获 WebMux 授权
- **THEN** 日志包含主机标识和失败类别，不包含任何 Token、签名、Cookie 或正文

#### Scenario: 查看服务信任说明

- **WHEN** 用户阅读官方 Relay 的配对确认或部署文档
- **THEN** 可以看到 Relay 终止 TLS 并能访问转发内容，不会被误导为官方无法读取终端数据

### Requirement: GitHub 浏览器登录与所有者控制台

Relay SHALL 提供 GitHub OAuth Authorization Code 登录，使用 PKCE S256、单次 state 和固定 HTTPS 回调 allowlist；MUST 复用 Go OAuth 组件，不自研 OAuth 客户端协议。设备配对与浏览器登录 SHALL 分别处理，登录成功本身 MUST NOT 新建、替换或撤销主机绑定。
账号授权 SHALL 以 GitHub 数字 ID 为键并检查邀请名单；用户控制台 SHALL 只列出该账号绑定的主机与在线状态。普通 GitHub 登录 MUST NOT 获得管理节点或配置的管理员权限；本期不提供管理员管理网页。
GitHub access token SHALL 仅用于读取身份，随后丢弃，不申请仓库权限。
后台 SHALL 使用服务端不透明会话，其摘要存入 PostgreSQL；Cookie MUST 为 host-only、Secure、HttpOnly、SameSite=Lax、Path=/ 的 `__Host-` Cookie，有效期不超过 12 小时，不无限续期。登录成功 SHALL 轮换会话 ID；登录与跳转材料 MUST NOT 写入浏览器持久化或日志。

后台 SHALL 提供适配手机的网页入口：宿主完成 GitHub 配对后，手机使用同一数字 ID 登录即可查看并打开在线 WebMux，无需复制长期 Token 或填写 TURN 配置。后台仅展示 WebMux；其他 Web 应用仍从 WebMux 内端口转发使用，MVP 不提供独立应用发布、公开分享或远程启动离线宿主。

#### Scenario: 登录后看到自己的主机

- **WHEN** 已邀请账号完成有效 GitHub 浏览器授权
- **THEN** 后台只列出该数字 ID 的主机，未绑定账号看到配对指引，不自动建立绑定

#### Scenario: 伪造 OAuth 回调

- **WHEN** OAuth 回调缺少对应浏览器事务，state 无效或 PKCE 校验失败
- **THEN** Relay 拒绝登录且不建立后台会话

#### Scenario: 手机打开已配对 WebMux

- **WHEN** 宿主已绑定 GitHub 账号，手机在控制面网页登录同一账号并点击在线主机
- **THEN** 经一次性授权打开 WebMux，不要求输入或下发长期 Token；离线主机只显示不可用

### Requirement: 单次主机跳转授权

Relay SHALL 为已登录且获邀请的主机所有者签发不超过 60 秒有效的一次性不透明 code，绑定账号数字 ID、host ID、绑定代际、后台会话、固定宿主回调和 PKCE S256 challenge。
兑换时，Relay SHALL 在宿主密钥认证的控制通道中验证 verifier，再事务性消费所存 code 摘要；签发和兑换 SHALL 再次校验邀请名单、后台会话和主机所有权。

Relay MUST NOT 接受任意回调或开放重定向，MUST NOT 为非所有者、离线或撤销主机签发授权。浏览器 SHALL 只携带短期 code 和 state，不能接收 GitHub access token、宿主私钥或 WebMux 长期 Token。

#### Scenario: code 被重放或过期

- **WHEN** 宿主兑换已消费、过期、verifier 错误或绑定代际过时的 code
- **THEN** Relay 拒绝兑换且不生成宿主会话，重复请求不能再次消费

#### Scenario: 其他账号尝试打开已知主机

- **WHEN** 已登录账号以已知 host ID 请求另一个账号的跳转授权
- **THEN** Relay 拒绝签发，不泄露目标账号或主机详情

### Requirement: 后台注销与授权撤销传播

后台注销 SHALL 撤销当前后台会话、未消费 code 和派生宿主会话；换钥、解绑或移出邀请名单 SHALL 撤销相关全部授权。Relay SHALL 通过已认证控制通道传播撤销，事件送达前 MUST NOT 声称在线宿主已完成全局撤销；通道断开 SHALL 触发宿主本地会话失效。

控制面重启 SHALL 使重启前的后台会话及跳转 code 失效，保留账号和主机绑定；数据面重启 SHALL 清空活动连接。GitHub 网站注销 SHALL 不被描述为自动注销 WebMux。

#### Scenario: 后台注销在线主机访问

- **WHEN** 用户注销后台，派生宿主会话仍在使用
- **THEN** Relay 撤销记录并通知宿主关闭该会话访问，收到确认后报告完成，不结束 PTY

### Requirement: 双连接 WSS 传输与性能证据

宿主连接器 SHALL 使用 TLS 1.3 WSS + smux v2，嵌入库而非受管 sidecar；连接器 MUST NOT 探测 QUIC、配置 FEC 或自研可靠 UDP，此限制不禁止独立 WebRTC/TURN 交互优化。
TCP 运维 SHALL 区分服务端与客户端：

- 服务器 TCP SHALL 使用 BBR，部署 SHALL 核实实际接口队列，不能仅凭 default_qdisc 宣称使用 fq。
- 客户端 SHALL 如实记录操作系统算法，不假定 Windows 支持 BBR。

批量流量 SHALL 使用 bulk，交互与控制 SHALL 使用 control。两条物理连接 SHALL 共用账号总配额和资源预算。control 断开 SHALL 撤销来源及派生会话并关闭 bulk；仅 bulk 断开 SHALL 保留交互、报告降级并拒绝新批量请求，不得转入 control。

重连 MUST NOT 自动重放非幂等请求，终端 SHALL 沿用 offset 续接。窗口、stream 和账号配额 SHALL 有界，不能声称多 TCP 消除了所有排队。

#### Scenario: bulk 失败保持交互可用

- **WHEN** 已认证逻辑连接器的 bulk 中断而 control 仍可用
- **THEN** 终端继续服务，新批量请求返回 503，bulk 有界重连并重新认证同一代际，不能建立第三条连接

#### Scenario: control 断开立即撤权

- **WHEN** control 断开或保活超时
- **THEN** 来源和派生会话失效，bulk 关闭，不向孤立 bulk 路由请求

#### Scenario: 性能与内存有证据

- **WHEN** 运行自然链路、随机/突发丢包、带宽限制、大文件并发终端与慢端矩阵
- **THEN** 记录样本量、p50/p95/p99、有效吞吐、连接数、总预算与资源，区分已测参数和待验收场景

#### Scenario: 按键响应与输出完整性共同验收

- **WHEN** 同一 PTY 持续输出且按固定节奏独立安排输入，并加入另一终端重输出或批量传输
- **THEN** SHALL 以真实 PTY 回显确认，记录排队、初始化与超时失败、全输入期限封顶 p95/p99 和期限内比例；同时检查输出完整性、Ctrl-C 退出及恢复后新输入，不能仅凭吞吐或已成功样本的延迟判定通过

### Requirement: 独立仓库与版本化集成

Relay 服务端 SHALL 位于独立仓库，独立构建、测试、发布和部署；WebMux SHALL 仅持有宿主连接器、设置和宿主认证，不构造 Relay 注册库或 GitHub OAuth 服务。两端 MUST NOT 跨仓库导入 `internal/` 包。

跨仓协议 SHALL 由 WebMux 仓库中的公开 Go 协议子模块发布，两端固定版本引用；模块 MUST 仅包含标准库依赖的协议 DTO、版本、算法标识与签名向量，不包含服务、OAuth 或传输实现。主版本不兼容 SHALL 拒绝，发布验收 SHALL 记录两端版本并覆盖真实联合流程。

#### Scenario: 服务端独立升级

- **WHEN** Relay 发布使用兼容协议的新版本而 WebMux 未同步升级
- **THEN** 两端继续通过规定的兼容测试；不兼容协议连接被明确拒绝，不要求跨仓源码一起构建

### Requirement: 椭圆曲线服务身份与宿主签名

受控源站与节点 TLS 身份 SHALL 使用 ECDSA P-256，宿主质询签名 SHALL 使用 Ed25519；实现 MUST 复用 Go 标准库，不自研曲线、签名或加密算法。两类私钥 SHALL 分别生成、保管和轮换，不得复用或转换；协议 v1 MUST 拒绝未知算法、错误曲线及畸形密钥，此约束不用于固定公开控制 CDN 边缘证书曲线。
传输内容 SHALL 由 TLS 1.3 加密，MUST NOT 将身份签名密钥直接作为内容加密密钥；官方私钥仅在受控加密存储和授权运行环境使用，公开客户端只持有地址和适用的公开服务信任，不分发控制 SPKI pin。本次控制 HTTPS 政策不改变公开 `protocol/relay` 版本。

#### Scenario: 错误算法或曲线被拒绝

- **WHEN** 宿主提交非 Ed25519 身份，或受控节点 TLS 身份使用非约定曲线的密钥
- **THEN** 对应验证拒绝该身份，不静默降级到其他算法或共享密钥

### Requirement: 受控 TURN 节点与短期凭据下发

控制面 SHALL 登记受信 TURN 节点、区域标签、允许的标准 ICE URL/传输及独立签发密钥，复用成熟 TURN 服务与短期凭据机制，不自研 TURN。长期共享密钥 SHALL 按秘密交付要求隔离，MUST NOT 进入仓库、客户端或公开 API。
浏览器 SHALL 通过已认证的同源宿主接口申请配置；宿主 SHALL 在密钥认证 control 通道中申请。Relay SHALL 校验账号、host ID、绑定/连接器代际、当前所有者浏览器会话与邀请状态，为两端分别签发带明确到期时间的短期凭据。
响应 SHALL 禁缓存，刷新 SHALL 重验授权，不得公开匿名 ICE 凭据接口。
TURN 节点 SHALL 强制标准 TURN 身份认证并关闭匿名/免认证中继；未携带凭据、凭据错误或过期的请求 MUST NOT 创建 allocation。允许公开 STUN Binding 探测 MUST NOT 被解释为允许匿名 TURN 转发。
凭据签发与刷新 SHALL 要求有效 GitHub 登录所派生的所有者浏览器登录会话；仅有已配对宿主密钥、设备身份、裸宿主会话或公开节点地址 MUST NOT 领取通用中继凭据。
每份凭据 SHALL 在服务端关联账号、浏览器会话、host ID、代际与端点角色，不将账号身份原值暴露在公开诊断中。

首版 SHALL 使用原版 coturn 的账号 allocation 数限制、每会话带宽限制及节点总容量限制；MUST NOT 宣称提供跨会话共享账号带宽桶或累计流量额度。签发限频不能替代节点执行，带宽方向/单位及组合上界 SHALL 如实配置并实测。

账号 allocation SHALL 使用稳定不透明 quota key 聚合两端角色及不同短期凭据；用户名 SHALL 为 `<expiry>:<accountQuotaKey>`，不得附加角色/会话/随机后缀。角色与会话仅存签发关联，两端凭据 SHALL 在 TTL 上限内保留不同 expiry 秒，耗尽时拒绝，不延长有效期规避限额。

同账号 SHALL 固定在一个存活 TURN 配额域内，跨域分配前必须确认旧域中继停止或取得明确不超总额的预算；MUST NOT 声称多个 stock coturn 节点自动共享全局账号计数。节点失效时可回 WSS，不通过悄悄换域绕过 allocation 限额。

短期凭据 SHALL 被视为持有者凭证。到期限制新认证，但 coturn 已认证会话可能继续刷新 allocation；TTL 或单次 allocation lifetime MUST NOT 被当作会话绝对上限。发布 SHALL 验证节点实际资源限制，MUST NOT 声称 GitHub 注销或密码到期会自动清除已有 TURN 资源。

TURN 凭据 SHALL 仅授权受限网络资源，MUST NOT 作为 WebMux 登录凭据。节点 SHALL 对 allocation 生命周期、并发、带宽和账号/节点总预算实施限制，禁止向回环、私网、链路本地及云元数据地址滥用转发；部署验收 SHALL 验证 ACL 和双端连通性。

撤销后 SHALL 停止签发及刷新并通知宿主关闭关联业务连接；MUST NOT 把凭据到期说成已立即终止现有 allocation。短期凭据及认证材料 MUST NOT 进入日志、诊断或浏览器持久化。

#### Scenario: 两端获得受限 ICE 配置

- **WHEN** 有效所有者会话请求与当前在线宿主建立交互连接
- **THEN** Relay 校验绑定及代际，为浏览器和宿主分别提供短期配置，不暴露长期密钥或其他账号凭据

#### Scenario: 已注销会话继续申请凭据

- **WHEN** 已注销、过期或被撤销的会话请求签发或刷新 TURN 凭据
- **THEN** 请求被拒绝，现有应用访问由宿主关闭，不等待 allocation 自然过期

#### Scenario: 没有可用 TURN 节点

- **WHEN** TURN 未配置、全部失效或超出配额
- **THEN** 控制面网页、所有者跳转和内置 WSS 仍可工作，返回明确的优化不可用状态，不将整个 Relay 标为不可用

#### Scenario: 绕过网页直接请求 TURN

- **WHEN** 未登录用户直接向 TURN 节点发送匿名、伪造或过期凭据的 Allocate 请求
- **THEN** 节点拒绝创建 allocation，不转发业务流量；STUN 探测成功也不授予中继权限

#### Scenario: 仅凭宿主密钥申请 TURN

- **WHEN** 已配对宿主在没有有效所有者浏览器会话的情况下申请中继凭据
- **THEN** 控制面拒绝签发或刷新，不把设备身份替代用户登录授权

#### Scenario: 更换短期用户名绕过配额

- **WHEN** 同账号通过多个会话或重复签发取得不同短期用户名，累计资源达到账号总限额
- **THEN** 节点或受信配额组件拒绝额外资源，不因用户名变化重置账号预算

### Requirement: TURN 部署与性能声明边界

部署文档 SHALL 区分控制面 HTTPS 入口、WSS 连接器和可独立选址的 TURN 能力节点；coturn 与节点管理代理同镜像，不要求与 WSS 同节点。网页、登录与初始信令仍依赖 HTTPS 可达。
国内 TURN 性能 SHALL 以实际线路和应用层采样判断，MUST NOT 承诺消除跨境 RTT 或默认快于 WSS。

部署前 SHALL 向接入商及属地主管部门确认适用手续；MUST NOT 以 TURN 或非 HTTP 协议作为免备案、免许可的保证。历史 #604 报告及证据包 SHALL 保留原样，不得冒充混合路径生产验收。

#### Scenario: 宣布混合数据面可用

- **WHEN** 准备发布包含 TURN 优化的版本
- **THEN** SHALL 附真实浏览器、节点故障/过期、权限撤销、抖动防切换及长时验证结果，并注明线路与未覆盖边界；不以历史原型或节点所在地代替证据
