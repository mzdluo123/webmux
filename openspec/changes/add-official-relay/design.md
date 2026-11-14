# Design

## Context

动机与范围见 [proposal.md](proposal.md)。现有代码已经提供三项关键基础：`cmd/webmux/tunnelctl.go` 的代际启停与 `PublicOrigins` 撤权模型、`settingsstore` 的 0600 原子持久化与 fail-closed 语义，以及可经任意同源 HTTP/WebSocket 基线运行的 P2P 传输。

Relay 同时引入一个新的公网服务端和 WebMux 出站客户端，必须遵守以下约束：

- 所有 JSON DTO 集中在 `internal/contract`，宿主与 Relay 控制器都归入 `internal/api`，服务错误只在 `internal/api/errors.go` 映射为 HTTP 状态；
- 只有 `cmd/` 组装服务，新增 `internal/` 包必须登记架构分层与出站网络用途；
- 构建保持 `CGO_ENABLED=0`，可复用 `coder/websocket` 和 `modernc.org/sqlite`；
- Relay 不直接接触 PTY，必须保留现有认证、回放续接、慢客户端和会话退出语义。

## Goals / Non-Goals

**Goals:**

- 用一个单节点 Go 二进制完成邀请制配对、注册、认证、反向代理和恢复。
- 在独立公网服务器部署，浏览器直连 HTTPS；宿主 QUIC 优先，UDP 不可用时以 TCP 443 WSS 提供基线。
- GitHub 登录官方后台后可进入自己的 WebMux，无需跨设备复制长期 Token。
- 宿主只主动连接配置的 Relay，只把 stream 接到本机 WebMux 回环地址。
- 密钥、绑定和启停状态可恢复，所有公网授权都能同步撤销。

**Non-Goals:**

- 不自研加密、可靠传输、HTTP 帧或 WebSocket 帧协议。
- 不在 MVP 引入 CDN、ACME 自动申请、分布式会话目录、跨节点转发或 KCP 数据面。
- 不把 Relay 私钥交给浏览器，不在独立直连入口提供 GitHub OAuth；本机恢复 Token 保留。

## Decisions

### 1. 单二进制、分包职责

新增 `cmd/webmux-relay`，在同一进程中监听公共 HTTPS/WSS（TCP 443）与连接器 QUIC（UDP 443）。包边界如下：

- `internal/relayproto`（shared）：协议版本、签名 transcript、challenge 类型和多路 stream 适配；DTO 仍放在 `internal/contract`。
- `internal/relaycontrol`（service）：GitHub 授权交换、后台会话、一次性跳转授权、邀请名单、账号/主机注册、重新绑定、撤销和 SQLite 事务，不依赖 HTTP 服务端。
- `internal/relay`（client）：宿主连接器、QUIC/WSS 认证、统一 stream 会话、回环拨号和重连，不导入 WebMux 服务或控制器。
- `internal/relaysettings`（service）：宿主启用意图与私钥的受控持久化。
- `internal/api/relay*.go`（controller）：宿主 `/api/relay*` 和 Relay 控制面、连接器及公共代理处理器，复用统一错误映射。
- `cmd/webmux`、`cmd/webmux-relay` 分别组装上述服务，不共享进程内单例。

**备选方案**：独立 Relay 网关控制器包。否决原因是服务 sentinel 到 HTTP 状态的映射会绕开仓库规定的 `internal/api/errors.go`，并形成第二套错误约定。

### 2. QUIC 优先与 WSS + yamux 回退

宿主优先使用 `github.com/quic-go/quic-go` 建立 TLS 1.3 QUIC 连接，校验服务端证书与主机名，使用专用 ALPN `webmux-relay-v1`。首个双向 stream 完成控制认证，后续原生双向 stream 承载代理连接；QUIC 不叠加 yamux。

QUIC 建连超时或网络不可达时，使用 `coder/websocket` 建立二进制 WSS，完成相同认证后以 `websocket.NetConn` 和 `github.com/hashicorp/yamux` 建立流会话。两条路径只共享应用层 stream 接口；QUIC stream 的地址、deadline、CloseWrite 与取消通过薄适配接入 `net.Conn`，不强行把整个 QUIC 连接当作 `net.Conn`。

选择 yamux 的原因是双方均可主动建流、按 stream 窗口更新实现背压、带保活，并能直接在有序可靠字节流上运行。实现时固定：

- 每条 WSS 只有一个读取循环和一个序列化写入方；
- 每主机一条已认证的活动传输会话，配置最大 stream、接受背压、窗口和保活参数；
- 所有 stream 由 context 和关闭路径统一回收，不在锁内执行网络读写；
- yamux 只提供传输，不承载业务 JSON。

切换仅发生在建连或断线重连时，不迁移存量 stream，不自动重试非幂等 HTTP 请求；浏览器使用既有 offset 续接。活动 WSS 不为探测 QUIC 而被打断。认证失败、证书错误或协议不兼容不得通过回退绕过；仅网络不可达或建连超时触发回退。禁用 QUIC 0-RTT，认证与业务写入都不接受早期数据。

**备选方案**：KCP 需要再组合 TLS、流复用和参数调优；MVP 选择原生 TLS 与多路流的 QUIC，保留 KCP 为后续测量驱动的备选。自定义可靠 UDP、加密或分片帧均不采用。

参数由可重复网络测试确定：RTT 50/150/300 ms、丢包 0/1/3%、UDP 黑洞与限带宽，测量终端回显 p50/p95、上传吞吐、额外带宽、内存和回退耗时；不把高 RTT 本身视为必然提速。

### 3. `httputil.ReverseProxy` 保留应用语义

公共 HTTPS 处理器按精确 Host 找到活动传输会话，并使用 `httputil.ReverseProxy` 转发到虚拟回环目标。自定义 `Transport.DialContext` 打开已认证会话的双向 stream；宿主接受后只拨号启动时固定的 `127.0.0.1:<listen-port>`。

代理使用 `Rewrite` 清理伪造转发头、保留原始 Host 和 Origin。Go 反向代理负责普通请求、流式正文和 WebSocket Upgrade，Relay 不解析终端帧、不重写 WebSocket 关闭码，也不完整缓冲上传或回放。

**备选方案**：把 API 重新封装成 RPC。否决原因是 `/forward/*`、Cookie、multipart、WebSocket Upgrade 与现有几十条路由会产生双轨行为，且容易遗漏鉴权。

### 4. GitHub 设备授权与 Ed25519 身份

配对流程：

1. WebMux 原子保存新生成的私钥，向 Relay 提交公钥。
2. Relay 创建 GitHub Device Flow 和本地配对记录，宿主展示 GitHub 验证地址与用户码。
3. Relay 轮询 GitHub，授权后只用访问凭据读取账号数字 ID 和登录名快照，随即丢弃访问凭据。
4. 邀请名单通过后，在一个 SQLite 事务内创建绑定；重新绑定必须带显式标志并替换旧公钥。
5. 宿主轮询自己的配对句柄取得 host ID 与固定公共地址。

连接认证在 QUIC 控制 stream 或 WSS 控制帧内完成：宿主声明协议版本和 host ID，Relay 返回单次随机 nonce，宿主签名以下 transcript：

```text
WebMux-Relay-v1\n<relay-audience>\n<host-id>\n<nonce>
```

nonce 只在单节点内存中短期保存并单次消费。新连接必须验证成功后才替换旧会话。删除绑定使用独立的签名质询，避免仅凭公开 host ID 撤销。

### 5. 固定标识与 SQLite 数据模型

host ID 使用 128 位加密随机数的 32 字符小写十六进制形式，公共标签采用 `h-<host-id>`。公共地址由 Relay 配置的公共泛域名生成，数据库不保存 WebMux Token。

SQLite 至少包含：

- `accounts`：GitHub 数字 ID 主键、登录名快照、唯一 host ID；
- `hosts`：host ID、唯一公共主机名、公钥、创建时间、撤销时间；
- `enrollments`：配对句柄摘要、公钥、临时 device code、用户码摘要、到期时间、重新绑定标志和消费状态。

唯一索引与事务保证每账号一台主机。device code 只在配对有效期内保存，完成、过期和取消时清除；GitHub access token 不落盘。单节点使用 WAL、有界 busy timeout 和版本化 migration，数据库故障使控制面和连接器 readiness 失败。

**备选方案**：宿主设置式 JSON 保存全部账号。否决原因是多请求并发、唯一约束和重新绑定需要事务，不能靠整文件锁提供可靠语义。

### 6. 宿主状态机与受控存储

新增 `relay-settings.json`，记录版本、启用指针、host ID、公共地址和私钥 seed；私钥不进入 configured/effective DTO、API 响应或日志。

TOML `[relay]` 的运维配置包括：

- `enabled`：默认启用意图；
- `endpoint` 与 `quic_endpoint`：默认取内置官方 HTTPS/QUIC 地址；自建配置必须连同独立的服务身份公钥显式提供，不能沿用官方信任身份，不接受浏览器传入拨号地址；
- `transport`：默认 auto；quic 模式失败则报错，wss 模式不探测 UDP。

宿主控制器复用 tunnel 的模式：任何应用、关闭、失败和退出先 `PublicOrigins.Clear("relay")`，再递增代际并取消旧 worker；新 worker 等前任退出。认证和传输 accept 循环均就绪后才 `Set("relay", publicURL)`。

意外断线使用有界指数退避和随机抖动。关闭时即便持久化失败也先停连接并返回 `notPersisted`；存储损坏时禁止启动，不从 TOML 重新暴露。

### 7. 访问 URL、推送与 P2P 复用

Relay 使用独立 PublicOrigins source `"relay"`，与 Cloudflare、Tailscale 并存。就绪地址加入现有 `urls.access`，通用托盘逻辑无需认识新类型。固定来源加入推送来源登记，已有订阅不因暂时离线删除。

P2P 代码不需要新传输分支：Relay 页面仍是同源 HTTP/WebSocket，`/api/p2p/signal` 和 ping 经反向代理到达宿主。P2P 继续只接受直连候选并执行原有 RTT 滞回与无缺口切换，Relay 只替代基线 origin。

### 8. API、设置页与协议版本

宿主接口：

- `GET /api/relay`：脱敏设置和状态；
- `PUT /api/relay`：修改启用意图；
- `POST /api/relay/enrollments`：开始配对或显式重新绑定；
- `POST /api/relay/enrollments/poll`：用请求体轮询，避免敏感句柄进入 URL；
- `DELETE /api/relay/enrollments`：取消等待中的配对；
- `DELETE /api/relay/binding`：经 Relay 确认后删除绑定。

Relay 控制面和连接器统一使用 `/v1/*`，主版本不兼容时拒绝连接且不替换旧会话。所有响应由 Zod schema 和 Go contract 同步约束，wire 快照一起更新。

前端新增 `/settings/relay` 与设置入口卡片，复用现有 hook、请求和确认对话框模式。配对码只留在组件内存，离开页面即取消轮询并清除；不提供“记住 GitHub 凭据”能力。

### 9. 单节点运维与限制

MVP 由运维者提供解析到独立服务器 IP 的通配符 DNS、证书和私钥，并开放 TCP/UDP 443；独立 IP 不替代泛域名和 HTTPS 证书要求。进程支持证书热重载或由服务管理器重启，ACME 留到后续。

GitHub OAuth 使用固定后台 HTTPS 回调地址；Client ID 与密钥通过环境或受限服务配置注入，不进入日志。

单节点配置包括邀请账号、配对/IP 速率、每主机 stream、每连接缓冲和保活参数。公共代理不设置会误杀终端 WebSocket 的短响应头超时；认证和建流使用独立短超时。健康接口只报告 readiness，不暴露账号或主机清单。

### 10. GitHub 后台登录与所有者跳转

主机 Device Flow 与浏览器登录分开。浏览器使用 GitHub OAuth Authorization Code + PKCE S256 和单次 state，复用 `golang.org/x/oauth2`；服务端交换 code 并查询 GitHub 数字 ID，完成后丢弃 GitHub access token。GitHub 登录名仅作展示，不作为授权键，不请求仓库权限。

Relay 后台采用服务端不透明会话，Cookie 为 `__Host-` 前缀、Secure、HttpOnly、SameSite=Lax、Path=/，不设置 Domain；会话摘要入 SQLite，绝对有效期 12 小时，不无限续期。

认证边界按阶段校验：回调 URI 固定 allowlist，校验 state 与浏览器绑定，登录后轮换会话 ID；登录、签发跳转和兑换时均检查邀请名单，页面只展示该数字 ID 的绑定主机。

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

撤销或过期时拒绝后续 HTTP，并关闭该身份建立的 WebSocket、P2P 和转发连接；只断开访问，不结束持久 PTY。Relay 重启使后台会话全部失效，宿主重启使内存会话失效；重新登录不需重新配对。

### 12. 数据、接口与组件复用补充

`accounts/hosts` 保存所有者数字 ID 和绑定代际；宿主配对结果及受控状态同步保存这两个字段。SQLite 增加：

- `browser_sessions`：摘要、账号、有效期、撤销状态；
- `login_grants`：code 摘要、S256 challenge、账号、主机、代际、后台会话、回调、过期时间、消费状态。

短期 state/verifier 仅存服务端内存，Cookie 只放不透明句柄；过期记录定期清理且总量受限。

Relay 接口分为 `/v1/auth/github/*` 登录与固定回调、`/v1/console/*` 所有者主机列表/打开入口/后台注销，以及仅在宿主密钥认证控制通道中可兑换的 `/v1/login-grants/*`。

宿主 `/auth/relay/start`、`/auth/relay/callback` 是受限认证入口，注销与会话状态归入 `/api/relay/auth/*`。无认证入口仅处理有界认证事务，不能读取业务数据或更改 Relay 开关；DTO、错误映射、Zod 与 wire 快照同步更新。

依赖固定经过兼容性和许可审查的版本：传输用 quic-go 与 yamux，OAuth 用 x/oauth2，反向代理用 Go 标准库，SQLite 与 WebSocket 沿用已有库。优先使用库的 deadline、限流和密码学原语，不自研拥塞控制、可靠 UDP、证书校验或 OAuth token 解析。

参考：[GitHub PKCE 支持](https://github.blog/changelog/2025-07-14-pkce-support-for-oauth-and-github-app-authentication/)、[x/oauth2](https://pkg.go.dev/golang.org/x/oauth2)、[quic-go stream](https://quic-go.net/docs/quic/streams/)。

### 13. 内置官方地址与公开信任配置

开源客户端内置官方 HTTPS 控制面地址、QUIC 地址、Relay 受众与服务身份公钥。用户选择官方服务后只需 GitHub 登录配对并确认启用，无需填写地址、复制公钥或申请 OAuth 应用；默认关闭与公网风险确认保持不变，不因内置配置在启动时自动连接。

官方公钥定义为官方 TLS 服务证书的 SPKI 公钥；随客户端分发公钥或 SHA-256 SPKI pin 集合，复用 Go `crypto/tls` 与 `crypto/x509` 的证书校验及公钥解析。HTTPS/WSS、QUIC 和兑换控制连接均先验证正常证书链、有效期及主机名，再校验公钥 pin；不采用 `InsecureSkipVerify`，不自研加密或另造服务端签名握手。

公钥轮换分为正常维护与泄露恢复：

- 正常维护：TLS 证书续期可保留服务身份公钥；换钥采用当前与下一公钥的重叠信任窗口，先发布客户端再切换服务端，最终版本移除旧 pin。
- 泄露恢复：发布移除泄露 pin 的安全版本，旧客户端可能需要升级才能恢复连接，不能承诺离线客户端立即撤销旧公钥。

未知公钥必须拒绝，不能自动学习首个公钥、通过失败连接下载替代 pin 或退回仅验证普通证书。

保密材料分开管理：

- 官方 TLS 身份私钥、GitHub OAuth Client Secret 和其他服务端秘密通过受限部署配置或密钥文件注入，不提交任何仓库、不放入客户端产物或 API 响应；
- OAuth Client ID 与回调 URI 本身不是秘密，可以公开；用户 GitHub access token 仍按既定流程短期使用后丢弃；
- 宿主私钥仍逐机本地生成，官方公钥不能作为共享客户端凭据，也不能替代宿主签名认证、账号所有权校验或邀请名单。

自建中转使用显式配置的独立 HTTPS/QUIC 地址、受众与公钥集合，仍要求正常 TLS 校验。更换任一信任配置须先停连接、撤销旧来源及登录会话，再经确认重新配对；不得把旧 host ID、宿主凭据或未完成授权材料发送给新服务。诊断区分内置官方默认与自建覆盖，并可显示公开 key ID，私密材料继续脱敏。

正式域名与公钥尚未提供，本规划不填写虚构生产地址或生成官方私钥。实现发布前由运营者提供公开端点与公钥；缺失、占位或不匹配配置须使官方连接保持不可用并明确报错，禁止以占位配置发布可用声明。

## Risks / Trade-offs

- **[Risk] Relay 是宿主登录信任方** → 配对确认明示其可授权远程 Shell，控制通道断线即撤销派生会话，保留本机 Token 恢复路径。
- **[Risk] Relay 能看到所有转发内容** → 配对和部署文档明确信任模型，默认关闭敏感日志；不把本版本描述为 E2EE。
- **[Risk] 单节点故障使全部入口离线** → MVP 明确无 SLA，宿主保留本机会话并自动重连；生产前备份 SQLite 和配置，后续再引入分片。
- **[Risk] yamux over WebSocket 叠加缓冲导致队头阻塞** → 每请求独立 stream、限制窗口和缓冲，使用大上传并发终端的真实集成测试验证；不在同一 stream 混合不同请求。
- **[Risk] 公共泛域名成为开放代理或扫描目标** → 精确 Host 映射、独立密钥认证、账号配额、固定回环目标和邀请名单；随机 ID 不作为认证替代。
- **[Risk] WebSocket/升级路径被普通 HTTP 超时误杀** → 分离超时策略，长期覆盖终端、聊天和语音连接。
- **[Risk] 私钥丢失导致用户无法恢复地址** → GitHub 显式重新绑定保留 host ID，撤销旧公钥并断开旧会话。
- **[Risk] 宿主与 Relay 版本错位** → `/v1` 和控制帧主版本握手、特性协商、宿主内置页面保持不变，不引入托管前端壳。
- **[Risk] 单节点带宽和 DDoS 成本不可预测** → 邀请制、每账号一机、stream/速率限制和只记录聚合字节指标，公开上线前另做容量评估。

## Migration Plan

1. 先在随机回环端口部署测试 Relay，用真实 WebMux 和浏览器完成配对、全协议与故障矩阵，不接触开发者现有服务。
2. 以默认关闭发布宿主能力；未配对或无 `[relay]` 配置的实例行为完全不变。
3. 邀请制部署单节点，逐批加入账号并观察 RTT、重连、内存和聚合流量。
4. 回滚宿主时先关闭 Relay 并撤销 PublicOrigins，再停止发布；旧版本不认识 Relay 设置，必须保留状态文件供恢复，不能删除后意外重绑新地址。
5. 回滚 Relay 服务时保留数据库、公共 DNS 和证书；恢复同一版本或兼容 migration 后，宿主自动重连。多节点迁移不在本 MVP 内。

## Open Questions

- Relay 独立仓库是建议方向，尚待确认。当前同仓库的包路径与构建任务暂保留；确认后须统一调整所有制品、协议共享和跨仓库验收，不在本次直接创建仓库。

- 正式公共域名、首个 VPS 区域、证书续期方式和邀请名单由运营部署决定，不改变协议或任务拆分。
- QUIC/yamux 的具体窗口、保活、回退建连期限和 stream 默认值需通过第一阶段基准确定，规格只要求有界与可配置，不预设未经测量的数值。
