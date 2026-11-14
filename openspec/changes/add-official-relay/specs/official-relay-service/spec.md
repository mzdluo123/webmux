# official-relay-service Specification

## Purpose

官方单节点 Relay 为经过账号配对的 WebMux 主机分配固定 HTTPS 入口，并通过宿主主动建立的出站连接转发公网 HTTP 与 WebSocket 流量，使用户无需公网 IP、域名或第三方 Tunnel 即可远程访问自己的终端。

## ADDED Requirements

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

宿主连接时，Relay SHALL 使用随机质询验证签名。签名 transcript SHALL 绑定随机数、主机标识、协议版本和 Relay 受众，防止重放到其他会话或服务。无效签名 MUST NOT 替换该主机的现有有效连接。

除显式释放主机名外，重新启动和重新绑定密钥 SHALL 保持公共地址不变。

#### Scenario: 错误私钥不能劫持主机名

- **WHEN** 攻击者使用已知公共主机标识和错误私钥连接
- **THEN** Relay 拒绝认证，原绑定和现有活动连接不受影响

#### Scenario: 重放已捕获的认证响应

- **WHEN** 攻击者在另一次连接或另一 Relay 受众中重放旧签名
- **THEN** Relay 因随机数或受众不匹配而拒绝连接

### Requirement: 单活动连接器与撤销

每个主机标识 SHALL 同时只有一个活动数据连接器。新的合法连接完成认证后 SHALL 替换旧连接；未完成认证前不得影响旧连接。

Relay SHALL 只把公共请求路由到当前已认证且处于活动状态的连接器。连接器断开、绑定撤销或公钥替换后，对应公共入口 SHALL 立即停止转发；后续请求返回不可用响应，不得转入其他主机。

#### Scenario: 同一主机的重复连接

- **WHEN** 同一私钥建立第二条有效连接
- **THEN** 新连接成为唯一活动连接器，旧连接被正常关闭

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

### Requirement: 注册数据持久化与单节点恢复

Relay SHALL 将账号绑定、主机公钥、公共标识和撤销状态持久化到单节点 SQLite 数据库，并使用事务保证账号唯一绑定和重新绑定原子性。

Relay 重启后 SHALL 恢复注册数据；活动连接器不假装自动恢复，宿主必须重新完成认证。数据库不可用或迁移失败时，Relay SHALL 拒绝新的配对与连接并报告未就绪，MUST NOT 使用不完整状态分配重复主机名。

GitHub 访问凭据 SHALL 只用于授权时读取账号数字身份，MVP MUST NOT 长期保存该凭据。

#### Scenario: Relay 重启

- **WHEN** 已有多台配对主机的 Relay 进程重启
- **THEN** 原公共地址与公钥绑定保留，宿主重新连接后恢复公网转发，无需重新配对

#### Scenario: 重新绑定发生并发请求

- **WHEN** 同一账号同时提交两次重新绑定确认
- **THEN** 事务只允许一个最终公钥绑定，公共主机名不会重复分配或丢失

### Requirement: 敏感数据与信任边界

Relay SHALL 在公网入口终止浏览器 TLS，并在宿主出站连接上使用 TLS。运营文档与界面 SHALL 明确 Relay 能访问经其转发的 Bearer Token、WebSocket 子协议、Cookie 和终端内容，本版本 MUST NOT 宣称端到端加密。

Relay MUST NOT 记录 WebMux Token、GitHub Token、配对码、签名原文、Authorization、Cookie、请求或响应正文、WebSocket 帧及终端内容。运行日志 SHALL 只包含必要的主机标识、状态、字节计数、错误类别和时间等脱敏元数据。

#### Scenario: 认证失败日志

- **WHEN** 宿主使用错误凭据连接或浏览器请求未获 WebMux 授权
- **THEN** 日志包含主机标识和失败类别，不包含任何 Token、签名、Cookie 或正文

#### Scenario: 查看服务信任说明

- **WHEN** 用户阅读官方 Relay 的配对确认或部署文档
- **THEN** 可以看到 Relay 终止 TLS 并能访问转发内容，不会被误导为官方无法读取终端数据

### Requirement: GitHub 浏览器登录与所有者后台

Relay SHALL 提供 GitHub OAuth Authorization Code 登录，使用 PKCE S256、单次 state 和固定 HTTPS 回调 allowlist；MUST 复用 Go OAuth 组件，不自研 OAuth 客户端协议。设备配对与浏览器登录 SHALL 分别处理，登录成功本身 MUST NOT 新建、替换或撤销主机绑定。

账号授权 SHALL 以 GitHub 数字 ID 为键并检查邀请名单；后台 SHALL 只列出该账号绑定的主机与在线状态。GitHub access token SHALL 仅用于读取身份，随后丢弃，不申请仓库权限。

后台 SHALL 使用服务端不透明会话，其摘要存入 SQLite；Cookie MUST 为 host-only、Secure、HttpOnly、SameSite=Lax、Path=/ 的 `__Host-` Cookie，有效期不超过 12 小时，不无限续期。登录成功 SHALL 轮换会话 ID；登录与跳转材料 MUST NOT 写入浏览器持久化或日志。

#### Scenario: 登录后看到自己的主机

- **WHEN** 已邀请账号完成有效 GitHub 浏览器授权
- **THEN** 后台只列出该数字 ID 的主机，未绑定账号看到配对指引，不自动建立绑定

#### Scenario: 伪造 OAuth 回调

- **WHEN** OAuth 回调缺少对应浏览器事务，state 无效或 PKCE 校验失败
- **THEN** Relay 拒绝登录且不建立后台会话

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

Relay 重启 SHALL 使重启前的后台会话及跳转 code 失效，保留账号和主机绑定。GitHub 网站注销 SHALL 不被描述为自动注销 WebMux。

#### Scenario: 后台注销在线主机访问

- **WHEN** 用户注销后台，派生宿主会话仍在使用
- **THEN** Relay 撤销记录并通知宿主关闭该会话访问，收到确认后报告完成，不结束 PTY

### Requirement: 独立服务器与 QUIC/WSS 双路径

MVP SHALL 部署在有独立公网 IP 的单台服务器，浏览器直接访问公共 HTTPS 域名；部署 SHALL 配置泛域名 DNS、有效证书、TCP 443 与 UDP 443，不依赖 CDN 或 Cloudflare 产品。

宿主连接 SHALL 支持 quic-go 原生 TLS 1.3 双向 stream 与 WSS + yamux，提供相同的身份验证、Host 路由、流控和配额。QUIC SHALL 校验服务端证书，使用专用版本化 ALPN；两条路径 SHALL 共享每主机单活动连接器约束，且 MUST NOT 接受 0-RTT 认证或业务写入。

浏览器侧 SHALL 继续使用 HTTPS/WebSocket，不要求浏览器建立自定义 QUIC。传输切换 SHALL 仅在建连或断线后发生，MUST NOT 自动重放非幂等请求；已有终端依靠 offset 续接。KCP SHALL 不进入 MVP 数据面，不自研可靠 UDP、加密或拥塞控制。

#### Scenario: 两种传输具有相同应用行为

- **WHEN** 分别通过 QUIC 和 WSS 运行终端、上传下载、聊天与端口转发
- **THEN** 两条路径通过相同的认证、撤销、慢客户端、内存上限和应用协议测试，QUIC 不额外叠加 yamux

#### Scenario: 高延迟收益可验证

- **WHEN** 在 RTT 50/150/300 ms、丢包 0/1/3%、带宽限制和 UDP 黑洞下测试两条路径
- **THEN** 记录终端回显 p50/p95、吞吐、额外带宽、内存与回退耗时，以实测确定参数，不宣称未验证的延迟收益

### Requirement: 官方服务私钥与 OAuth 凭据隔离

官方 Relay SHALL 从受限运行环境或密钥文件加载身份私钥及 GitHub OAuth Client Secret，MUST NOT 将其提交到开源或其他代码仓库、打入公开客户端或通过控制面返回。用户 access token SHALL 继续短期使用后丢弃。

服务 SHALL 向客户端提供匹配公开信任配置的 TLS 身份；运维 SHALL 配合客户端重叠信任集合安排正常公钥轮换。公开地址、公钥、OAuth Client ID 与回调 URI SHALL 不被当作保密凭据；持有公开信息不能获得账号或主机权限。

#### Scenario: 客户端发布与服务部署分离

- **WHEN** 发布开源客户端并部署官方 Relay
- **THEN** 客户端仅含官方地址及公开信任材料，身份私钥与 OAuth Client Secret 由官方部署注入，不出现在产物、响应或日志中

### Requirement: 独立仓库与版本化集成

Relay 服务端 SHALL 位于独立仓库，独立构建、测试、发布和部署；WebMux SHALL 仅持有宿主连接器、设置和宿主认证，不构造 Relay 注册库或 GitHub OAuth 服务。两端 MUST NOT 跨仓库导入 `internal/` 包。

跨仓协议 SHALL 由 WebMux 仓库中的公开 Go 协议子模块发布，两端固定版本引用；模块 MUST 仅包含标准库依赖的协议 DTO、版本、算法标识与签名向量，不包含服务、OAuth 或传输实现。主版本不兼容 SHALL 拒绝，发布验收 SHALL 记录两端版本并覆盖真实联合流程。

#### Scenario: 服务端独立升级

- **WHEN** Relay 发布使用兼容协议的新版本而 WebMux 未同步升级
- **THEN** 两端继续通过规定的兼容测试；不兼容协议连接被明确拒绝，不要求跨仓源码一起构建

### Requirement: 椭圆曲线服务身份与宿主签名

官方 TLS 身份 SHALL 使用 ECDSA P-256，宿主质询签名 SHALL 使用 Ed25519；实现 MUST 复用 Go 标准库，不自研曲线、签名或加密算法。两类私钥 SHALL 分别生成、保管和轮换，不得复用或转换；协议 v1 MUST 拒绝未知算法、错误曲线及畸形密钥。

传输内容 SHALL 由 TLS 1.3 加密，MUST NOT 将身份签名密钥直接作为内容加密密钥；官方私钥仍只保存在部署环境中，公开客户端只分发服务身份公钥与地址。

#### Scenario: 错误算法或曲线被拒绝

- **WHEN** 宿主提交非 Ed25519 身份，或官方 TLS 身份使用非约定曲线的密钥
- **THEN** 对应验证拒绝该身份，不静默降级到其他算法或共享密钥
