# Design

## Context

动机与范围见 [proposal.md](proposal.md)。现有代码已经提供三项关键基础：`cmd/webmux/tunnelctl.go` 的代际启停与 `PublicOrigins` 撤权模型、`settingsstore` 的 0600 原子持久化与 fail-closed 语义，以及可经任意同源 HTTP/WebSocket 基线运行的 P2P 传输。

Relay 同时引入一个新的公网服务端和 WebMux 出站客户端，必须遵守以下约束：

- WebMux 宿主 JSON DTO 仍集中在 `internal/contract`，宿主控制器归入 `internal/api`，服务错误只在 `internal/api/errors.go` 映射为 HTTP 状态；跨仓库协议 DTO 通过公开子模块定义，宿主 contract 使用别名衔接；
- WebMux 只有 `cmd/` 组装服务，新增宿主 `internal/` 包必须登记架构分层与出站网络用途；Relay 独立仓库建立自己的显式装配、服务和 HTTP 错误映射边界；
- Go 程序构建保持 `CGO_ENABLED=0`；控制面使用纯 Go PostgreSQL 驱动，coturn 作为独立 C 程序及其运行库随数据面镜像发布，不将其链接进 Go；
- Relay 不直接接触 PTY，必须保留现有认证、回放续接、慢客户端和会话退出语义。

## Goals / Non-Goals

**Goals:**

- 用独立 `webmux-control` 完成邀请制配对、账号与授权，使用 PostgreSQL；`webmux-relay` 只执行数据面认证、转发及配额。
- MVP 为单控制实例与单数据节点，可同机或分机部署；宿主使用 TCP 443 WSS 与两条有界物理连接。
- GitHub 登录官方后台后可进入自己的 WebMux，无需跨设备复制长期 Token。
- 同一数据面镜像运行 `webmux-relay` 与 coturn 两个进程，WSS 保底，TURN 仅为已授权交互提供可选路径。
- 宿主只主动连接配置的 Relay，只把 stream 接到本机 WebMux 回环地址。
- 密钥、绑定和启停状态可恢复，所有公网授权都能同步撤销。

**Non-Goals:**

- 不自研加密、可靠传输、HTTP 帧或 WebSocket 帧协议。
- 不在 MVP 引入 CDN、ACME 自动申请、分布式会话目录、跨节点转发或 KCP 数据面。
- 不把 Relay 私钥交给浏览器，不在独立直连入口提供 GitHub OAuth；本机恢复 Token 保留。

## Decisions

### 1. 独立仓库与明确的协议边界

Relay 服务端在独立仓库开发和发布，包含 `cmd/webmux-control` 与 `cmd/webmux-relay`。控制面负责 GitHub、网页后台、绑定、授权和 PostgreSQL；数据面负责公共 HTTPS/WSS、宿主连接器与转发，并与 coturn 打包为双进程镜像。WebMux 不增加这两个服务端命令，不构造注册库、GitHub OAuth 或后台。

两端职责如下：

- WebMux `internal/relay`（client）：WSS/smux 宿主连接器、认证、流适配、回环拨号和重连；
- WebMux `internal/relaysettings`（service）：受控设置、宿主私钥、配对身份与内存登录会话；
- WebMux `internal/api/relay*.go`（controller）：宿主 `/api/relay*` 与受限登录入口，沿用统一错误映射；`cmd/webmux` 负责装配；
- Relay 仓库 `cmd/webmux-control`：控制面装配；`internal/relaycontrol`：GitHub OAuth、账号绑定、后台会话与授权；`internal/store`：PostgreSQL 查询与 migration；
- Relay 仓库 `cmd/webmux-relay`：数据面装配；`internal/connector`：宿主认证、WSS/smux、活动连接和配额；HTTP 层按控制/数据面分开装配，数据面仅接收受信控制授权，不直接查询 PostgreSQL；
- coturn `turnserver`：标准 TURN 认证、allocation 与转发，进程管理器负责启动、回收和信号；不由 Go 重新实现 TURN。

共享内容在 WebMux 仓库 `protocol/relay/` 形成独立公开 Go 子模块，仅包含协议版本、wire DTO、签名 transcript、算法标识、错误码与一致性向量，依赖限于标准库。宿主 `internal/contract` 使用类型别名衔接 wire DTO，服务类型继续由 `internal/wire` 转换；WebMux 的既有契约与快照流程保持生效。

两端分别实现 WSS/smux 的薄适配，不把业务、数据库、OAuth、HTTP handler 或 transport 依赖塞入协议模块。Relay 仅依赖发布的协议模块版本，禁止导入 WebMux `internal/`、复制未版本化 DTO 或依赖本地相对路径。采用子模块避免第三个仓库；其路径与例外须同步到架构规则和 contributor 工具，不能静默绕开守卫。

协议规格继续由本 OpenSpec 管理，两仓实现 PR 链接相同 requirement 与跨仓库任务。首次发布固定协议 v1 与模块 v0.x；两端固定依赖版本，CI 独立测试本仓库，并以已发布产物/指定 commit 运行兼容矩阵；不兼容主版本必须拒绝，不能要求双方从各自主分支同步构建。

**备选方案**：同仓库新增服务端启动命令。否决原因是公网多账号服务与单用户宿主的依赖、部署、凭据和发布周期不同。跨仓库不需要共享进程内服务或 HTTP 控制器。

### 2. WSS + smux v2 与有界连接隔离

依据 [issue #604 验证报告](../../../docs/relay-validation-604.md)，MVP 使用嵌入两端 Go 进程的 `coder/websocket` v1.8.15 与 `xtaci/smux` v1.5.58：

- A 的基础入口保持 HTTPS/WebSocket，B 使用 TLS 1.3 WSS。
- 宿主连接器不引入 QUIC 探测、UDP 协议回退、FEC 或独立隧道进程。
- 第 14 节 WebRTC/TURN 是另一路交互优化，不替代此基线。

同一个逻辑宿主连接器最多两条已认证物理连接：control 承载控制和交互，bulk 承载已分类的上传下载。两条连接共用 host ID、绑定代际、连接器代际和账号配额，不能作为两个宿主领取资源。每条 WSS 独立读循环与序列化写入方，smux v2 提供双向建流、流级窗口及背压。

初始调参基线为每连接接收缓冲 4 MiB、每 stream 1 MiB、保活 10 秒、失活 30 秒。物理连接数增加会扩大总窗口和内存预算，必须同时限制每账号总 stream、总缓冲和速率；这些是实现起点，不能宣称完成容量验收。

路由按已验证的请求语义分类，上传、文件节点下载、归档及全部 `/forward/*` 请求使用 bulk；终端、会话事件和授权控制留在 control。未知通用转发不能仅凭后缀猜测大小，必须有明确且有界的分类策略和测试，不能整包读取后再决定。

control 验证完成后建立逻辑代际；bulk 认证必须绑定同一代际和角色，重复角色拒绝。新的合法 control 原子替换整个旧代际，未认证连接不影响旧连接。

control 断开立即撤销 PublicOrigins 及派生会话并关闭 bulk。只有 bulk 断开时保留交互，标记 bulkReady=false，新批量请求返回 503，并有界重建 bulk，不能挤入 control。

不迁移存量 stream，不自动重试非幂等 HTTP 请求；浏览器沿用 offset 续接。所有 stream 由 context、deadline 和关闭路径回收，长连接不使用普通请求短超时。证书、pin、身份和协议错误均 fail-closed，不能通过重连改变信任配置。

TCP 服务器采用 BBR，部署优先使用 fq 并检查实际接口队列；本轮物理接口保留 mq/fq_codel，未实测替换成 fq 的收益。

宿主使用操作系统实际算法，支持时按 socket 选择 BBR，不要求 Windows 提供 BBR 或用管理员权限更改系统全局设置。WSS 可达性只在本次路径验证，企业代理、移动网络和多运营商仍待验收。

**终端优先验收**：参见[补测报告](../../../docs/relay-terminal-validation-604.md)。同一 PTY 持续输出时独立安排按键，以真实 PTY 回显确认；排队、超时和初始化失败均保留，报告全输入期限封顶分位数及期限内比例。另查高输出完整性、Ctrl-C 退出与续接后的新输入回显；不能只用吞吐或小 echo 选择方案。

两段真实 RTT 仍构成反馈下限，MVP 选择不是本地 Vim 手感承诺。8 s 期限、2 s 观察比例属于本轮实验口径，不自动成为生产 SLO；发布目标须由维护者另行确认，并补真实浏览器/TUI 验证。

**备选判断**：首轮 quic-go Reno 的受损链路吞吐偏低，终端补测却在弱网轻输出下给出较低按键尾延迟；高刷新率和重输出完成检查又暴露退化。因此不能用单个吞吐、小 echo 或固定负载概括 QUIC/KCP。

Hysteria BBR 在补测中已有受限反向终端适配，应作为后续终端优化优先复验候选；其两条 smux 角色连接位于一个 QUIC 连接内，不等于原生每终端 stream 或两条独立 TCP。实验适配保留正常证书校验和固定 ACL，但没有实现生产宿主身份、代际、撤销与跨进程生命周期。

MVP 保持 WSS 宿主连接器，不因单出口短时成绩引入 QUIC/KCP 双栈或受管隧道进程；另按用户决定加入 WebRTC/TURN 交互路径。这项工程取舍不表示 WSS 或 TURN 具有通用终端延迟优势。frp/SSH 也仍需账号、固定 Host、配额和身份包装。生产发布须完成终端优先门禁及未覆盖网络验证。

### 3. `httputil.ReverseProxy` 保留应用语义

公共 HTTPS 处理器按精确 Host 找到活动传输会话，并使用 `httputil.ReverseProxy` 转发到虚拟回环目标。自定义 `Transport.DialContext` 打开已认证会话的双向 stream；宿主接受后只拨号启动时固定的 `127.0.0.1:<listen-port>`。

代理使用 `Rewrite` 清理伪造转发头、保留原始 Host 和 Origin。Go 反向代理负责普通请求、流式正文和 WebSocket Upgrade，Relay 不解析终端帧、不重写 WebSocket 关闭码，也不完整缓冲上传或回放。

**备选方案**：把 API 重新封装成 RPC。否决原因是 `/forward/*`、Cookie、multipart、WebSocket Upgrade 与现有几十条路由会产生双轨行为，且容易遗漏鉴权。

### 4. GitHub 设备授权与 Ed25519 身份

配对流程：

1. WebMux 原子保存新生成的私钥，向 Relay 提交公钥。
2. Relay 创建 GitHub Device Flow 和本地配对记录，宿主展示 GitHub 验证地址与用户码。
3. Relay 轮询 GitHub，授权后只用访问凭据读取账号数字 ID 和登录名快照，随即丢弃访问凭据。
4. 邀请名单通过后，控制面在一个 PostgreSQL 事务内创建绑定；重新绑定必须带显式标志并替换旧公钥。
5. 宿主轮询自己的配对句柄取得 host ID 与固定公共地址。

每条物理连接先通过 WSS 认证消息完成握手，再开始 smux 会话。宿主声明协议版本和 host ID，Relay 为 control 分配候选连接器代际并返回单次随机 nonce；bulk 只加入当前代际。宿主签名以下 transcript：

```text
WebMux-Relay-v1\n<relay-audience>\n<host-id>\n<binding-generation>\n<connector-generation>\n<role>\n<nonce>
```

nonce 只在单节点内存中短期保存并单次消费。新 control 连接必须验证成功后才替换旧逻辑会话；bulk 只能加入其认证的同代际。删除绑定使用独立的签名质询，避免仅凭公开 host ID 撤销。

宿主身份签名使用 Go `crypto/ed25519`；官方 TLS 证书身份使用 `crypto/ecdsa` 的 P-256 与 `crypto/x509`，均由标准库安全随机源生成。两者都是椭圆曲线签名，各自独立的密钥不能复用或互相转换。传输加密与临时密钥协商交给 TLS 1.3，不将签名密钥直接用于加密终端内容，不自研曲线运算、ECDH 或 ECIES。

协议 v1 固定宿主身份算法 Ed25519 和官方服务身份 ECDSA P-256；拒绝未知算法、错误长度或曲线，禁止收到算法名就动态降级到其他实现。后续改算法需显式版本协商及迁移。

参考：[Go Ed25519](https://pkg.go.dev/crypto/ed25519)、[Go ECDSA](https://pkg.go.dev/crypto/ecdsa)。

### 5. 固定标识与 PostgreSQL 数据模型

host ID 使用 128 位加密随机数的 32 字符小写十六进制形式，公共标签采用 `h-<host-id>`。公共地址由 Relay 配置的公共泛域名生成，数据库不保存 WebMux Token。

控制面 PostgreSQL 至少包含：

- `accounts`：GitHub 数字 ID 主键、登录名快照、唯一 host ID；
- `hosts`：host ID、唯一公共主机名、公钥、创建时间、撤销时间；
- `enrollments`：配对句柄摘要、公钥、临时 device code、用户码摘要、到期时间、重新绑定标志和消费状态。

唯一索引与事务保证每账号一台主机。device code 只在配对有效期内保存，完成、过期和取消时清除；GitHub access token 不落盘。使用版本化 migration、有界连接池和查询/事务超时，不提供 SQLite 回退。数据库故障使控制面未就绪并拒绝新增绑定与授权，数据面不得用过期缓存绕过。

只有控制面访问业务数据库；节点注册、主机路由、TURN 签发关联和配额策略归控制面持久化，在线连接、stream、PTY 内容及逐包转发不入库。数据面与 coturn 不持有业务数据库凭据，宿主受控状态文件不改为 PostgreSQL。

**备选方案**：宿主设置式 JSON 保存全部账号。否决原因是多请求并发、唯一约束和重新绑定需要事务，不能靠整文件锁提供可靠语义。

### 6. 宿主状态机与受控存储

新增 `relay-settings.json`，记录版本、启用指针、host ID、公共地址和私钥 seed；私钥不进入 configured/effective DTO、API 响应或日志。

TOML `[relay]` 的运维配置包括：

- `enabled`：默认启用意图；
- `endpoint`：默认取内置官方 HTTPS/WSS 地址；自建配置必须连同独立的服务身份公钥显式提供，不能沿用官方信任身份，不接受浏览器传入拨号地址；
- `transport`：MVP 固定 wss；不提供 auto/quic 配置。

宿主控制器复用 tunnel 的模式：配置应用、关闭、control 失败和退出先 `PublicOrigins.Clear("relay")`，再递增代际并取消旧 worker；新 worker 等前任退出。

control 认证和 accept 循环就绪后才 `Set("relay", publicURL)`；bulk 未就绪时显示降级，批量请求返回 503。仅 bulk 失败不递增逻辑代际、不撤销仍有效的 control 来源，按同代际重建该角色。

意外断线使用有界指数退避和随机抖动。关闭时即便持久化失败也先停连接并返回 `notPersisted`；存储损坏时禁止启动，不从 TOML 重新暴露。

### 7. 访问 URL、推送与 P2P 复用

Relay 使用独立 PublicOrigins source `"relay"`，与 Cloudflare、Tailscale 并存。就绪地址加入现有 `urls.access`，通用托盘逻辑无需认识新类型。固定来源加入推送来源登记，已有订阅不因暂时离线删除。

Relay 页面仍是同源 HTTP/WebSocket，`/api/p2p/signal` 和探测经已认证基线到达宿主。扩展现有 P2P 支持受控 TURN 候选，保留既有直连候选校验；配置只对当前 Relay 授权会话生效，不暗中改变 Cloudflare/Tailscale 等独立入口的 P2P 设置。升级与撤销细节见第 14 节。

### 8. API、设置页与协议版本

宿主接口：

- `GET /api/relay`：脱敏设置和状态；
- `PUT /api/relay`：修改启用意图；
- `POST /api/relay/enrollments`：开始配对或显式重新绑定；
- `POST /api/relay/enrollments/poll`：用请求体轮询，避免敏感句柄进入 URL；
- `DELETE /api/relay/enrollments`：取消等待中的配对；
- `DELETE /api/relay/binding`：经 Relay 确认后删除绑定。

Relay 控制面和连接器统一使用 `/v1/*`，主版本不兼容时拒绝连接且不替换旧会话。宿主响应由 Zod schema 和 Go contract 同步约束，wire 快照一起更新；跨仓库 DTO 以公开协议模块为准，Relay 在自己的契约测试中验证同一版本。

前端新增 `/settings/relay` 与设置入口卡片，复用现有 hook、请求和确认对话框模式。配对码只留在组件内存，离开页面即取消轮询并清除；不提供“记住 GitHub 凭据”能力。

### 9. 单节点运维与限制

MVP 由运维者提供解析到独立服务器 IP 的通配符 DNS、证书和私钥，并开放 TCP 443；独立 IP 不替代泛域名和 HTTPS 证书要求。进程支持证书热重载或由服务管理器重启，ACME 留到后续。

GitHub OAuth 使用固定后台 HTTPS 回调地址；Client ID 与密钥通过环境或受限服务配置注入，不进入日志。

单节点配置包括邀请账号、配对/IP 速率、每主机 stream、每连接缓冲和保活参数。公共代理不设置会误杀终端 WebSocket 的短响应头超时；认证和建流使用独立短超时。健康接口只报告 readiness，不暴露账号或主机清单。

### 10. GitHub 后台登录与所有者跳转

主机 Device Flow 与浏览器登录分开。浏览器使用 GitHub OAuth Authorization Code + PKCE S256 和单次 state，复用 `golang.org/x/oauth2`；服务端交换 code 并查询 GitHub 数字 ID，完成后丢弃 GitHub access token。GitHub 登录名仅作展示，不作为授权键，不请求仓库权限。

控制面后台采用服务端不透明会话，Cookie 为 `__Host-` 前缀、Secure、HttpOnly、SameSite=Lax、Path=/，不设置 Domain；会话摘要入 PostgreSQL，绝对有效期 12 小时，不无限续期。

认证边界按阶段校验：回调 URI 固定 allowlist，校验 state 与浏览器绑定，登录后轮换会话 ID；登录、签发跳转和兑换时均检查邀请名单，页面只展示该数字 ID 的绑定主机。

用户路径固定为“宿主 GitHub 配对 → 手机网页登录同一 GitHub 账户 → 查看自己的 WebMux 在线状态 → 点击打开”。手机不复制 Token，不填写 TURN 参数；离线主机显示不可用，不能签发打开授权。后台只列 WebMux，不登记或发布其他 localhost 应用；现有每账号一机与邀请制不变，不引入分享或远程启动宿主。

点击「打开 WebMux」按以下步骤完成，不把 GitHub token、WebMux Token 或长期会话放进 URL：

1. 后台只跳到登记的主机 HTTPS `/auth/relay/start`；宿主创建 5 分钟有效的单次 state、PKCE verifier 和 host-only 临时 Cookie，再跳回 Relay 授权入口。
2. Relay 验证后台会话与主机所有权，签发 60 秒有效的不透明随机 code，数据库只存摘要，绑定账号 ID、host ID、绑定代际、后台会话、固定回调 URI 和 S256 challenge。
3. 浏览器携 code 和 state 返回宿主 `/auth/relay/callback`。宿主校验临时 Cookie/state，再以现有宿主密钥认证的控制通道携 verifier 兑换 code；Relay 事务内单次消费，返回账号、主机和绑定代际。
4. 宿主检查返回值匹配本地配对身份及代际，建立仅此 Relay 公共 Origin 有效的内存会话；Cookie 与后台隔离，采用相同安全属性，绝对有效期不超过 12 小时。清除临时材料，以 303 跳到无凭据 URL；认证页面禁缓存并设置 `Referrer-Policy: no-referrer`。

这里复用 OAuth 的 state/PKCE 与成熟组件，不把 Relay 实现成通用 OAuth/OIDC 提供者，不自定义签名 Token。跨后台与主机不共享父域 Cookie。回调页面不加载第三方资源；所有代理/接入日志对 code、state 与认证 URL 查询参数脱敏。

### 11. 宿主认证、会话撤销与信任范围

宿主显式信任配对 Relay 的所有者认证。Relay 被攻破可签发主机访问授权，界面与部署文档必须说明；不把一次性 code 当作消除此信任的措施。GitHub 会话仅通过固定回调与控制通道换取宿主会话，禁止信任客户端可伪造的账号头或 Cookie，也不由代理注入长期 Bearer Token。

`internal/relaysettings` 管理宿主配对所有者、绑定代际与内存登录会话，`internal/api` 在公共 Relay Origin 接受有效会话作为 Token 的另一种认证方式。Host、Origin、会话内进程限制和权限保持生效；本机及其他公网入口仍按既有 Token 认证。保留至少 43 字符恢复 Token 的启用门槛，不要求用户在新设备输入该 Token。

浏览器 HTTP、WebSocket、P2P 信令和端口转发统一支持此会话。Cookie 写请求必须校验同源 Origin 和 CSRF token，不能复用仅面向 Bearer 的豁免；转发到 `/forward/*` 上游前剥离认证 Cookie 和 CSRF 材料。前端识别会话登录态并复用现有 API/WS 封装与 401 处理，不读取 HttpOnly Cookie，不强制进入 Token 输入页。

Relay 关闭、断线、换钥、解绑或绑定代际变化均使宿主对应会话失效。后台注销通过活动控制通道撤销该后台会话派生的宿主会话；事件送达前后台不得报告全局撤销成功，通道中断时宿主立即失效。移出邀请名单同样撤销。宿主注销只撤销当前宿主会话；界面区分两种注销。GitHub 网站注销本身不等于 WebMux 注销。

撤销或过期时拒绝后续 HTTP，并关闭该身份建立的 WebSocket、P2P 和转发连接；只断开访问，不结束持久 PTY。控制面重启使后台会话及跳转 code 失效；数据面重启清空活动连接，宿主重启清空内存会话，重新登录不需重新配对。

### 12. 数据、接口与组件复用补充

`accounts/hosts` 保存所有者数字 ID 和绑定代际；宿主配对结果及受控状态同步保存这两个字段。PostgreSQL 增加：

- `browser_sessions`：摘要、账号、有效期、撤销状态；
- `login_grants`：code 摘要、S256 challenge、账号、主机、代际、后台会话、回调、过期时间、消费状态。

短期 state/verifier 仅存服务端内存，Cookie 只放不透明句柄；过期记录定期清理且总量受限。

控制面接口包括 `/v1/auth/github/*`、`/v1/console/*` 和 `/v1/login-grants/*`；数据面连接器入口独立部署。宿主兑换及 ICE 申请经已认证 WSS control 转交控制面，保留宿主身份、会话、受众与代际校验，不能信任普通公网代理头声明。

宿主 `/auth/relay/start`、`/auth/relay/callback` 是受限认证入口，注销与会话状态归入 `/api/relay/auth/*`。无认证入口仅处理有界认证事务，不能读取业务数据或更改 Relay 开关；DTO、错误映射、Zod 与 wire 快照同步更新。

依赖固定经过兼容性和许可审查的版本：WSS 用 coder/websocket 与 smux v2，OAuth 用 x/oauth2，反向代理用 Go 标准库，PostgreSQL 用纯 Go 驱动，TURN 用独立 coturn。优先复用成熟库和进程，不自研拥塞控制、可靠 UDP、证书校验或 OAuth token 解析。

参考：[GitHub PKCE 支持](https://github.blog/changelog/2025-07-14-pkce-support-for-oauth-and-github-app-authentication/)、[x/oauth2](https://pkg.go.dev/golang.org/x/oauth2)、[smux](https://github.com/xtaci/smux)。

### 13. 内置官方地址与公开信任配置

开源客户端内置官方 HTTPS 控制面与 WSS 地址、Relay 受众与服务身份公钥。用户选择官方服务后只需 GitHub 登录配对并确认启用，无需填写地址、复制公钥或申请 OAuth 应用；默认关闭与公网风险确认保持不变，不因内置配置在启动时自动连接。

官方服务身份采用 ECDSA P-256，对应 TLS 证书的 SPKI 公钥或 SHA-256 SPKI pin 集合随客户端分发；证书与公钥解析复用 Go `crypto/tls` 和 `crypto/x509`。

所有 HTTPS/WSS 和兑换控制连接均先验证正常证书链、有效期及主机名，再校验公钥 pin；不采用 `InsecureSkipVerify`，不自研加密或另造服务端签名握手。

公钥轮换分为正常维护与泄露恢复：

- 正常维护：TLS 证书续期可保留服务身份公钥；换钥采用当前与下一公钥的重叠信任窗口，先发布客户端再切换服务端，最终版本移除旧 pin。
- 泄露恢复：发布移除泄露 pin 的安全版本，旧客户端可能需要升级才能恢复连接，不能承诺离线客户端立即撤销旧公钥。

未知公钥必须拒绝，不能自动学习首个公钥、通过失败连接下载替代 pin 或退回仅验证普通证书。

保密材料分开管理：

- 官方 TLS 身份私钥、GitHub OAuth Client Secret 和其他服务端秘密通过受限部署配置或密钥文件注入，不提交任何仓库、不放入客户端产物或 API 响应；
- OAuth Client ID 与回调 URI 本身不是秘密，可以公开；用户 GitHub access token 仍按既定流程短期使用后丢弃；
- 宿主私钥仍逐机本地生成，官方公钥不能作为共享客户端凭据，也不能替代宿主签名认证、账号所有权校验或邀请名单。

自建中转使用显式配置的独立 HTTPS/WSS 地址、受众与公钥集合，仍要求正常 TLS 校验。更换任一信任配置须先停连接、撤销旧来源及登录会话，再经确认重新配对；不得把旧 host ID、宿主凭据或未完成授权材料发送给新服务。诊断区分内置官方默认与自建覆盖，并可显示公开 key ID，私密材料继续脱敏。

正式域名与公钥尚未提供，本规划不填写虚构生产地址或生成官方私钥。实现发布前由运营者提供公开端点与公钥；缺失、占位或不匹配配置须使官方连接保持不可用并明确报错，禁止以占位配置发布可用声明。

### 14. 混合交互数据面与 TURN

`webmux-relay` 提供完整 WSS 转发，即使 coturn 未就绪或 TURN 全失效也能经控制面登录并使用全部基础业务。coturn 与它构建到同一个 Docker 镜像但保持独立进程，不是 WSS 加速代理，不嵌入控制面或链接进 Go。

控制面配置受信 TURN 节点、区域标签、允许的 `turn:`/`turns:` URL、传输和签发密钥。长期共享密钥仅在控制面与 TURN 部署环境保管，不进入宿主、浏览器、仓库、诊断或日志。协议只下发标准 ICE 配置、短期用户名/凭据与明确到期时间。

浏览器经已认证同源宿主接口请求 ICE 配置，宿主通过密钥认证 WSS control、数据面与控制面的受信通道申请；控制面校验账号、host ID、代际、浏览器会话及邀请状态，为两端分别签发短期凭据。响应禁缓存、只留内存；刷新重验授权，失败不得签发新凭据。

节点必须关闭匿名/免认证 TURN，直接到节点的 Allocate 也验证标准短期凭据；公开 STUN Binding 可单独允许，但不授予转发权限。凭据签发/刷新必须具有有效 GitHub 登录派生的所有者宿主会话，不能仅凭宿主密钥领取通用代理凭据。

每份凭据在服务端关联稳定账号、浏览器会话、host ID、代际和角色。签发限频及节点资源预算按账号跨用户名/角色聚合，由节点或受信配额组件强制执行；coturn 的逐用户名限制不能被当作账号总配额，部署实现须证明重复签发无法绕过。

标准短期凭据是持有者凭证，不自带持续 GitHub 在线验证。到期限制新认证，但 coturn 已认证会话可能继续刷新 allocation，TTL 或单次 allocation lifetime 不等于会话绝对上限；泄露风险还须由节点限额及经过验证的资源控制约束，不保证密码到期自动清空资源。

行为依据见 [coturn TURN REST API 与会话认证说明](https://github.com/coturn/coturn/blob/master/README.turnserver)；实现时固定 coturn 版本并对照其文档及实测，不把当前上游说明当作未经验证的目标版本保证。

TURN 凭据不是 WebMux 登录凭据。信令将 PeerConnection、SDP/DTLS 指纹和一次性建链授权绑定到当前宿主会话及代际，宿主验证后才放行业务；不把浏览器不能在 DataChannel 自动附带 Cookie 当作免鉴权理由。复用现有 P2P 帧与业务 handler，确需扩展时版本化并拒绝未协商特性，不另建通用代理协议。

TURN 节点限制 allocation 生命周期、并发、带宽和账号/节点总预算；禁用到回环、私网、链路本地及云元数据地址的滥用转发，发布前验证 ACL、传输兼容性及到宿主的可达性。共享密钥应按节点隔离。短期密码的到期不等于立即踢掉已有 allocation，必须如实记录剩余资源生命周期。

注销、会话过期、解绑、换钥、邀请撤销、control 断开时，宿主立即关闭对应直连/TURN 业务连接并拒绝新流，控制面停止签发与刷新凭据，两端释放 allocation。不能只依赖 TURN 凭据自然过期实施业务撤权，也不能声称离线浏览器已完成资源释放。

首屏先走 WSS，不等待 ICE 或 UDP 超时。后台以有界频率同时测量 WSS 与候选 WebRTC 的应用层往返和健康状态，不用 ICE 优先级、ping 或握手成功代替。只有连续采样满足延迟收益及健康门槛才升级，失败立即回 WSS；收益阈值、采样窗口和冷却期须在实现验收中固定并覆盖抖动测试。

直连与 TURN 均按测量结果选择，不强制 TURN 优于 WSS。持续监测活动交互路径，退化时按滞回策略回落；探测和备用连接受每会话预算约束，不能无限并行建链。路径切换不改变 WSS control 的权威性，其断线仍触发授权失效。

只有显式允许的终端、聊天及列表交互路由可升级；上传下载、全部 `/forward/*`、登录、设置与其他未分类请求继续 WSS。语音等未纳入的业务也保持基线，不因“所有 REST 可经 P2P”而被隐式迁移。通道背压与总缓冲有界，批量数据不得抢占交互路径。

切换前完成身份与恢复校验，每条业务流只有一个输入写入方，绝不双发。终端用 generation/offset 续接并丢弃重叠输出；聊天/列表用各自游标或权威快照恢复，不能套用终端 offset。执行结果未知的输入、聊天提交或其他非幂等请求不得自动重放；无法确定时显示不确定状态，由用户判断。

网页静态资源、GitHub 登录、首次授权和信令仍依赖 HTTPS 可达；国内 TURN 可能改变业务路由，但不保证消除跨境瓶颈或改善访问。TURN 不终止 DataChannel 的 DTLS，但控制面仍是登录与信令信任方，WSS 仍可看到代理内容，不据此宣传产品端到端加密。

部署前向接入商及属地主管部门核实适用手续；不以 TURN 或非 HTTP 协议作为无需备案/许可的保证。记录节点线路与实际测量，不把历史 #604 报告改成 TURN 证据。

发布必须以真实手机浏览器及桌面浏览器验证同账号打开 WebMux、Vim/CLI agent、同 PTY 高输出、Ctrl-C、输入无重复、各业务恢复、UDP 黑洞、TURN 不可达/过期、权限撤销及 8–24 小时稳定性。历史报告未测这些混合路径，必须独立记录版本、样本、失败与未覆盖边界。

### 15. 数据面双进程镜像与控制通道

控制面镜像仅运行 `webmux-control`，数据面镜像包含 `webmux-relay`、固定版本的 coturn 和进程管理器；PostgreSQL 独立服务。Go 构建无 CGo 不代表整个数据面镜像无 C 运行库，镜像须包含 coturn 依赖并承担相应安全更新。

进程管理器作为 PID 1，转发 SIGTERM、回收子进程并有界关闭两者；分别监督失败和重启，禁止裸后台启动无人回收。coturn 故障只将 TURN 标为降级，不能由聚合健康检查反复重启仍正常的 WSS；整个容器重启会中断两种转发，应如实说明。

分别提供 WSS/TURN readiness 与 liveness，按进程限制连接、缓冲和资源预算；仍存在容器级 CPU/内存及 OOM 共享故障域，不声称进程分离等于资源完全隔离。两者随同镜像版本发布和回滚，后续可以拆分部署但不增加首版多节点调度。

显式配置公共控制域名、数据面泛域名、WSS 监听、TURN UDP/TCP/TLS 监听及中继端口范围；同机端口不得冲突，同一镜像不表示共享 TCP 443。Docker 映射必须覆盖实际中继端口并正确公布公网 IP，验证 NAT 后双向流量及 TLS/SNI。

GitHub Secret、PostgreSQL 凭据仅注入控制面；节点身份、TLS 私钥和 TURN 共享密钥通过运行时受限 secret 注入相应进程，不写入镜像层或日志。coturn 使用 `use-auth-secret`，控制面按节点共享密钥签发标准短期凭据，无需 coturn 查询业务 PostgreSQL。

数据面节点通过双向 TLS 认证连接控制面，接收精确 Host/主机绑定、公钥与代际、授权、限额和撤销；节点身份预配置，禁止匿名注册。协议绑定节点受众、版本及代际，完整同步并确认后才接入新宿主；数据面不能自行发放 GitHub 用户权限。

控制面决定授权，数据面执行校验、转发、限流及聚合计量；WSS control 角色名不等于 `webmux-control` 进程。授权信息使用有界有效期且需健康控制通道；失联停止新授权/凭据，并在检测超时后关闭相关宿主连接和派生访问，禁止无限使用旧权限。

数据库不可用时拒绝需要数据库确认的配对、登录、兑换与新增连接授权；已有连接仅在仍有效且可监督的授权范围内运行，不能因拆分架构取消原有撤权要求。控制面、数据库和数据面分别做断连、重启与恢复验收，避免声称控制面升级绝不中断流量。

账号总配额仍需节点强制执行验证，不能以打包 coturn 或 PostgreSQL 签发记录代替。发布前验证凭据过期后存量会话持续刷新行为，并证明资源限制生效；若仅配置 TTL 和逐用户名配额，防滥用门禁不能通过。

## Risks / Trade-offs

- **[Risk] Relay 是宿主登录信任方** → 配对确认明示其可授权远程 Shell，控制通道断线即撤销派生会话，保留本机 Token 恢复路径。
- **[Risk] Relay 能看到所有转发内容** → 配对和部署文档明确信任模型，默认关闭敏感日志；不把本版本描述为 E2EE。
- **[Risk] 单节点故障影响入口** → MVP 无 SLA，宿主保留 PTY 并重连；生产前备份 PostgreSQL、配置及秘密，控制面与数据面分别监测。
- **[Risk] smux over WebSocket 叠加缓冲导致队头阻塞** → 每请求独立 stream、限制窗口和缓冲，使用大上传并发终端的真实集成测试验证；不在同一 stream 混合不同请求。
- **[Risk] 公共泛域名成为开放代理或扫描目标** → 精确 Host 映射、独立密钥认证、账号配额、固定回环目标和邀请名单；随机 ID 不作为认证替代。
- **[Risk] WebSocket/升级路径被普通 HTTP 超时误杀** → 分离超时策略，长期覆盖终端、聊天和语音连接。
- **[Risk] 私钥丢失导致用户无法恢复地址** → GitHub 显式重新绑定保留 host ID，撤销旧公钥并断开旧会话。
- **[Risk] 宿主与 Relay 版本错位** → `/v1` 和控制帧主版本握手、特性协商、宿主内置页面保持不变，不引入托管前端壳。
- **[Risk] 单节点带宽和 DDoS 成本不可预测** → 邀请制、每账号一机、stream/速率限制和只记录聚合字节指标，公开上线前另做容量评估。
- **[Risk] TURN 被当作通用代理或占用过期资源** → 短期分端凭据、节点 ACL、allocation/流量配额；业务撤权在宿主执行，不以密码到期冒充即时释放。
- **[Risk] ICE 成功但键感更差或频繁换路** → 应用层双路径采样、收益门槛与冷却；TURN 全失效仍以 WSS 可用。

## Migration Plan

1. 先在隔离环境部署测试控制面、PostgreSQL 和双进程数据面容器，用真实 WebMux 与浏览器完成配对、全协议及故障矩阵，不接触开发者现有服务。
2. 以默认关闭发布宿主能力；未配对或无 `[relay]` 配置的实例行为完全不变。
3. 邀请制部署单控制实例和单数据节点，逐批加入账号并观察 RTT、重连、内存和聚合流量。
4. 回滚宿主时先关闭 Relay 并撤销 PublicOrigins，再停止发布；旧版本不认识 Relay 设置，必须保留状态文件供恢复，不能删除后意外重绑新地址。
5. 回滚 Relay 服务时保留数据库、公共 DNS 和证书；恢复同一版本或兼容 migration 后，宿主自动重连。多节点迁移不在本 MVP 内。

## Open Questions

- Relay 独立仓库已确认；仓库正式名称、可见性及创建时机待确认，逻辑边界不依赖名称。本次仅更新规划，不创建仓库。

- 正式公共域名、首个 VPS 区域、证书续期方式和邀请名单由运营部署决定，不改变协议或任务拆分。
- WSS/smux 的初始参数见第 2 节；最终 stream/账号限额、多宿主容量及 8–24 小时稳定性仍需生产实现验收。
- TURN 节点位置、线路、URL/传输、配额和短期凭据 TTL 由部署确认；收益阈值、采样窗口与冷却期由真实浏览器验收固定，不影响 WSS 先行实现。
