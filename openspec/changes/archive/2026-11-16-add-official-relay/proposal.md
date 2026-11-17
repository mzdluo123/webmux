# Proposal

## Why

用户目前要在公网打开 WebMux，仍需自行配置 Tailscale、Cloudflare 账号与域名，或接受 Quick Tunnel 地址变化及无法使用 Web Push 的限制。官方自建 Relay 可以把开通流程收敛为一次账号配对和一次启用确认，同时避免第三方 Tunnel 的账号配额、平台用途与区域调度约束。

跨设备访问还要求复制 WebMux 登录 Token。本变更让宿主先通过 GitHub 配对，用户再在手机控制面网页登录同一 GitHub 账户，查看自己的主机与在线状态，点击即可进入 WebMux，由宿主验证一次性授权并建立浏览器会话。

现有 WebRTC P2P 已能在基线连接建立后降低延迟，但信令仍依赖既有公网入口，不能解决首次可达性。本变更以自建 Relay 提供可靠基线，P2P 继续在后台机会性升级。

## What Changes

- 新增独立控制面 `webmux-control`，提供 GitHub 配对、网页后台、主机注册与授权，使用 PostgreSQL 持久化；数据面 `webmux-relay` 提供公共 HTTPS/WSS、宿主连接器和 HTTP/WebSocket 转发，不直接访问业务数据库。
- 宿主身份使用 Ed25519，受控源站与节点 TLS 服务身份使用 ECDSA P-256；分别生成、保管和轮换，复用 Go 标准密码库。公开控制 CDN 边缘证书不受节点算法或 pin 约束。
- 每个账号在 MVP 中绑定一台 WebMux 主机。宿主使用本地生成的独立 Ed25519 密钥配对，获得固定随机公共主机名；配对和跳转不提交或分发 WebMux 长期登录 Token。
- 宿主通过 TCP 443 出站 WSS + smux v2 接入 Relay，以一个逻辑连接器内的两条物理连接隔离控制/交互与批量流量，Relay 在多路 stream 上转发 REST、上传下载、WebSocket Upgrade 与端口转发流量。TLS 在 Relay 终止，本版本不宣称端到端加密。
- WebMux 新增官方 Relay 设置、配对、启停、重连、删除绑定和状态展示；启用仍要求至少 43 字符登录 Token 与公网风险确认。
- 新增适配手机的官方后台 GitHub 登录、所有者主机列表和免 Token 跳转；仅展示 WebMux，其他应用仍从 WebMux 内端口转发入口使用，不提供公开分享。宿主显式信任已配对 Relay 的所有者授权，独立直连入口继续使用 Token。
- MVP 为单控制实例，可登记不同能力的数据节点：wss、turn 或两者。至少一个就绪 wss 节点提供基线，可另选 TURN-only 节点优化交互；按管理员配置分配，不引入多区域智能调度、HA 或节点间 WSS 转发。
- Relay 就绪后复用 `PublicOrigins` 登记公共 Host/Origin，断开或关闭立即撤权；固定 HTTPS 入口纳入现有访问链接、状态输出与 Web Push 来源。
- 开源宿主只需内置或显式配置可信控制面地址及受众；公开控制 HTTPS 使用 Go 系统 CA（可追加 RootCA）、域名、有效期与 TLS 1.3 校验，不附加 SPKI pin。配对后控制面下发 WSS 分配、公开地址与节点服务信任，WSS 仍直连并强制校验分配 pin，浏览器另取授权 ICE。
- 公开控制域名使用 Cloudflare CDN，CDN 是控制 HTTPS 的受信 TLS 终止方；WSS/TURN 域名保持 DNS-only 直连。节点 mTLS、节点 SPKI 与分配服务身份验证不变，用户无需手填数据端点或 OAuth 应用。
- 宿主状态从严格 v1 读取迁移到不含 `controltrust` 的 v2；读取不改磁盘，保存只写 v2，不丢失密钥、绑定或固定 Origin，也不保留旧控制 pin 配置别名。
- 官方 Relay 默认关闭，独立于 Cloudflare Tunnel 和 Tailscale。统一数据面镜像包含 `webmux-relay` 与 coturn，按能力启动：wss-only 不启动 coturn；TURN-only 运行管理 agent 与 coturn，不开放 WebMux/WSS 入口；双能力运行两种服务。
- 首屏与登录不等待 WebRTC；后台比较应用层延迟和健康状态，仅在候选路径稳定更优时升级，失败回 WSS，并防止频繁切换。终端、聊天、列表等交互业务可升级，文件上传下载与全部 `/forward/*` 保持 WSS。
- 每账号一台主机、邀请制与单控制实例保持；控制面容器、按能力运行的数据面容器和 PostgreSQL 分开管理。不包含自定义域名、匿名配额、公开分享、端到端加密或独立稳定域名壳。
- 管理员 CLI/API 经控制面校验写入 PostgreSQL，不直接改表、不新增运营管理网页。非秘密节点参数集中管理，以单调递增版本完整下发并回执；重启性变更暂存为 pending-restart，CLI 显式批准精确版本后应用，配置回滚不恢复撤销授权。
- 控制面将节点秘密加密存入 PostgreSQL，解密主密钥从部署环境提供；节点经独立双向 TLS 秘密接口领取，只存受限运行时文件。冷启动须连通控制面并取得有效配置、秘密及授权同步，禁止用离线缓存恢复服务。

实测依据见 [issue #604 验证报告](../../../../docs/relay-validation-604.md)。同时参考[终端优先补测](../../../../docs/relay-terminal-validation-604.md)，以按键回显、输出完整性和恢复优先判断，不把 WSS 说成所有场景延迟最低。
生产实现已在 WebMux PR #639 与独立 Relay 仓库提供，但未完成真实部署的端到端验收；QUIC、KCP/FEC、Hysteria 与完整产品保留为后续研究。

用户已选择直接实现 WSS 基线及混合交互优化，不再以前置多协议竞赛阻塞开发。历史报告与证据包保留原样，不作为 TURN 已验证的依据；真实手机浏览器、Vim/CLI agent、TURN 故障与长时稳定性是发布门槛。国内 TURN 不保证改善跨境路径，也不以协议名作免备案或其他合规豁免承诺。

## Capabilities

### New Capabilities

- `official-relay-service`: 官方 Relay 的账号配对、网页后台、所有者跳转、主机身份、WSS 转发、短期 TURN 凭据、账号配额、撤销与应用恢复。
- `official-relay-node-management`: 管理员 CLI/API、节点能力与地址登记、配置版本/应用批准、加密秘密交付、服务发现、冷启动及进程监督。
- `official-relay-client`: WebMux 宿主的中继密钥与配对、短期登录会话与撤销、受控持久化、WSS 基线、WebRTC 直连/TURN 交互升级、公网来源授权、设置界面、访问链接与推送集成。

### Modified Capabilities

- `effective-configuration`: 将控制面引导地址、已分配 WSS 节点/端点、TURN 来源及可达性分别纳入 configured/effective，保证宿主诊断与节点运营配置状态不混淆，秘密材料脱敏。
- `server-local-api`: 禁止 WebMux 会话内进程通过本机 API 修改 Relay 设置、发起配对或控制公网暴露，并在本机状态中提供脱敏的 Relay 状态。

## Impact

- Relay 服务端置于独立仓库，发布 `webmux-control` 与 `webmux-relay` 两个 Go 程序及包含 coturn 的数据面镜像；WebMux 仅持有宿主连接器、设置和会话认证。coturn 作为独立进程复用，不嵌入或链接到 Go。
- 在 WebMux 仓库维护小型公开 Go 协议子模块供两端固定版本复用；更新宿主架构分层、持久化清单和跨仓库兼容性验证，不跨仓库导入 `internal/`。
- 新增 `/api/relay*` 宿主控制接口与 Relay 控制面/连接器协议，更新 `internal/contract`、`internal/wire`、`web/src/contracts/schemas.ts` 和 wire 快照。
- 更新 `cmd/webmux` 组装、`internal/api`、`internal/statefile`、本机状态、访问 URL 和 Web Push 来源登记。
- 前端新增 Relay 设置组件、配对流程和状态展示；扩展现有 P2P 的 TURN 候选、会话认证、交互路由与状态，替换“永不使用 TURN”的界面和规则，复用现有传输、存储和设计系统。
- 复用 `coder/websocket`、`net/http/httputil`、`xtaci/smux` v1.5.58 和 `golang.org/x/oauth2`；控制面使用经过版本及许可审查的纯 Go PostgreSQL 驱动，不提供服务端 SQLite 回退。coturn 固定版本，不自研传输、加密、TURN 或 OAuth。
- 新增 Relay 服务端部署与运维说明、远程访问用户故事，以及单元、集成、浏览器和端到端测试。
