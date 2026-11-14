# Proposal

## Why

用户目前要在公网打开 WebMux，仍需自行配置 Tailscale、Cloudflare 账号与域名，或接受 Quick Tunnel 地址变化及无法使用 Web Push 的限制。官方自建 Relay 可以把开通流程收敛为一次账号配对和一次启用确认，同时避免第三方 Tunnel 的账号配额、平台用途与区域调度约束。

跨设备访问还要求复制 WebMux 登录 Token。本变更让用户在官方后台通过 GitHub 登录，点击自己的主机即可进入面板，由宿主验证一次性授权并建立浏览器会话。

现有 WebRTC P2P 已能在基线连接建立后降低延迟，但信令仍依赖既有公网入口，不能解决首次可达性。本变更以自建 Relay 提供可靠基线，P2P 继续在后台机会性升级。

## What Changes

- 新增官方 Relay 单节点服务：同一 Go 进程提供 GitHub 配对、主机注册、公共 HTTPS 入口、宿主出站连接和 HTTP/WebSocket 反向代理；账号与主机绑定使用 SQLite 持久化。
- 每个账号在 MVP 中绑定一台 WebMux 主机。宿主使用本地生成的独立 Ed25519 密钥配对，获得固定随机公共主机名；配对和跳转不提交或分发 WebMux 长期登录 Token。
- 宿主优先通过出站 QUIC 接入 Relay，UDP 不可用时回退 WSS + yamux，Relay 在多路 stream 上转发 REST、上传下载、WebSocket Upgrade 与端口转发流量。TLS 在 Relay 终止，本版本不宣称端到端加密。
- WebMux 新增官方 Relay 设置、配对、启停、重连、删除绑定和状态展示；启用仍要求至少 43 字符登录 Token 与公网风险确认。
- 新增官方后台 GitHub 登录、所有者主机列表和免 Token 跳转；宿主显式信任已配对 Relay 的所有者授权，独立直连入口继续使用 Token。
- MVP 部署在有独立公网 IP 的单台服务器，用户直连；提供 HTTPS 域名与证书，不引入 CDN、Spectrum 或 Cloudflare Tunnel 依赖。
- Relay 就绪后复用 `PublicOrigins` 登记公共 Host/Origin，断开或关闭立即撤权；固定 HTTPS 入口纳入现有访问链接、状态输出与 Web Push 来源。
- 开源客户端内置官方 HTTPS/QUIC 地址与官方服务身份公钥，用户无需填写端点或配置 GitHub OAuth 应用；官方私钥、OAuth Client Secret 和用户授权凭据仅由服务端保密管理。
- 官方 Relay 默认关闭，独立于 Cloudflare Tunnel 和 Tailscale。现有 P2P 配置、直连候选限制与无损切换协议不变，Relay 仅成为新的基线路径。
- MVP 仅支持单节点、单进程、每账号一台主机和邀请制运营；不包含多区域、高可用、自定义域名、匿名配额、TURN、端到端加密或独立稳定域名壳。

## Capabilities

### New Capabilities

- `official-relay-service`: 官方单节点 Relay 的账号配对、浏览器登录、所有者跳转、主机注册、密钥认证、公共主机名路由、HTTP/WebSocket stream 转发、配额、撤销与运行恢复。
- `official-relay-client`: WebMux 宿主的中继密钥与配对、短期登录会话与撤销、受控持久化、连接状态机、公网来源授权、设置界面、访问链接与推送集成。

### Modified Capabilities

- `effective-configuration`: 将 Relay 开关、服务端点、配对状态与运行状态纳入 configured/effective 诊断，并保证私钥和配对材料脱敏。
- `server-local-api`: 禁止 WebMux 会话内进程通过本机 API 修改 Relay 设置、发起配对或控制公网暴露，并在本机状态中提供脱敏的 Relay 状态。

## Impact

- 新增 `cmd/webmux-relay` 及 Relay 服务端、协议、宿主连接器与设置包；更新架构分层、持久化清单和部署配置。
- 新增 `/api/relay*` 宿主控制接口与 Relay 控制面/连接器协议，更新 `internal/contract`、`internal/wire`、`web/src/contracts/schemas.ts` 和 wire 快照。
- 更新 `cmd/webmux` 组装、`internal/api`、`internal/statefile`、本机状态、访问 URL 和 Web Push 来源登记。
- 前端新增 Relay 设置组件、配对流程和状态展示，复用现有传输、设置存储、确认对话框与设计系统。
- 复用 `coder/websocket`、`modernc.org/sqlite`、`net/http/httputil`；使用 `quic-go`、`hashicorp/yamux` 和 `golang.org/x/oauth2`，不自研传输、加密或 OAuth 客户端。
- 新增 Relay 服务端部署与运维说明、远程访问用户故事，以及单元、集成、浏览器和端到端测试。
