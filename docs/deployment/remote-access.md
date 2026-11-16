# 远程访问与 HTTPS

手机、局域网与公网访问：Tailscale、官方 Relay、访问链接、Cloudflare Tunnel 与 HTTPS。

直接经可信 VPN 访问核心 HTTP/WebSocket 不需要可选功能。托管入口和交互优化分别选择：

- 托管 Tailscale 或 Cloudflare 入口分别需要 `tailscale` 或 `cloudflare`；官方 Relay 的设置与 WSS 基线属于核心。
- 交互 WebRTC 优化需要 `p2p`，不选择时 Relay 仍可使用 WSS。
- 先 [选择功能](configuration.md#内置功能选择) 并重启，再调整相应业务开关。

## 手机访问（Tailscale）

1. 在服务器和手机上安装 Tailscale 并加入同一网络。
2. 推荐在 `[tailscale]` 设置 `mode = "serve"`，让 WebMux 通过默认的 `127.0.0.1:7681` 监听并托管自动证书的 HTTPS 入口；也可以把 `listen` 设为 `0.0.0.0:7681`（或在 Windows 托盘中勾选“允许局域网访问”）后直连。
3. 托管 Serve 时手机打开 `https://<节点>.ts.net/`；HTTP 直连时打开 `http://<Tailscale IP 或 MagicDNS 主机名>:7681` 并输入一次 token（保存在浏览器 localStorage 中）。也可以打开启动时打印的访问链接并把主机换成 Tailscale 地址。
4. 托管 Serve 的 HTTPS 地址可以安装为应用；手工 HTTP 地址只能当作普通网页使用，见 [安装为应用（PWA）](pwa-and-push.md#安装为应用pwa)。

详细配置、端口、WSL Windows provider 与故障排查见 [Tailscale Serve / Funnel 入口](../tailscale.md)。

防火墙应只允许可信 VPN 接口访问该端口。

## 官方 Relay

使用同一个 GitHub 账号配对宿主并在手机打开后台，可访问自己的固定 HTTPS 入口，不必在设备之间复制长期 WebMux Token。

1. 官方 Relay 属于核心，无需选择 `relay` 功能；需要交互优化时选择 `p2p` 并重启。保持本机恢复 Token 至少 43 字符，并先确认公网 Shell 风险。
2. 官方控制地址 `https://webmux-relay.rainbit.net`、受众 `webmux-relay-control` 和 `wss` 传输已内置，不写 `[relay]` 即使用默认值；无需填写地址、受众、控制 SPKI 或 OAuth 凭据。
3. 在“设置”点击“管理官方 Relay”，进入 `/settings/relay`，按设备配对流程登录 GitHub；完成配对后再显式启用公网访问。
4. 手机打开该控制面的 HTTPS 后台，登录同一 GitHub 账号，选择自己的在线主机。后台只列出账号拥有的 WebMux，不是其他应用的公开目录。

显式 TOML 字段覆盖对应默认值，未覆盖字段仍使用内置值。自建 Relay 时应同时设置 `control_endpoint` 与 `control_audience`，切换控制服务后须重新配对，不能把旧主机凭据发给另一控制面。
内置地址不等于自动授权：Relay 仍默认关闭，须完成配对和公网风险确认后才启用。

宿主访问公开控制 HTTPS 时验证 Go 系统 CA（可追加 RootCA）、域名、有效期与 TLS 1.3，允许控制域名走 CDN，不额外固定 SPKI。
已认证控制面分配的数据节点仍强制服务身份校验，WSS pin 不得省略；不要求用户手填第二个节点。
本说明不表示默认官方服务已经上线；实际地址、接入前提与未完成项见[官方 Relay 部署记录](official-relay.md)，可信发布材料必须由实际部署提供。

浏览器先使用 WSS；可用时依据应用层往返测量选择 WebRTC 直连或授权 TURN 承载交互。上传、下载及全部 `/forward/*` 始终走 WSS 批量路径。
P2P 未选择或被裁剪时明确显示优化不可用，不自动加载或启用功能。TURN、UDP 不可达时保持 WSS，不能以“ICE 已连接”代替业务可用。

设置诊断分别报告控制面与数据节点、角色连接、授权来源和可达性；后台登录正常不等于宿主在线。批量角色故障不应挤占交互连接，控制角色故障会撤销派生访问。
宿主关闭立即停止公网连接；保存失败仍保持关闭并提示修复。解绑或后台退出可能等待逐 Scope 撤权确认，断连和超时不代表确认成功，也不结束 PTY。
Cookie 会话有绝对期限，写请求检查 Origin/CSRF；长期 Token 仍是需要独立保管的本机恢复凭据，不能把配对码、Cookie 或 TURN 凭据发给其他人。
控制消息有效期为 60 秒，接收端仅对未来时间戳上界留出 5 秒时钟偏差余量，不接受已到期消息。
心跳仍每 10 秒发送、30 秒失联撤权；双端都需更新共享协议，主机与服务端仍应启用时钟同步。

> [!WARNING]
> Relay 是受信中间人，会终止 TLS 并可接触转发内容与凭据，不是端到端加密或进程沙箱。风险边界见[安全说明](../../SECURITY.md#官方-relay)。

### 恢复或删除机器

- 本机首次配对前生成独立随机机器 ID，保存在 `<state_dir>/relay-machine.json`；解绑、更换中继密钥或丢失 `relay-settings.json` 不删除它。
- 同一 GitHub 数字账号重新授权且机器 ID 匹配时，普通配对自动恢复原固定地址，并使旧密钥、连接和派生登录失效；机器 ID 本身不是登录凭据。
- 本地未配对但服务端已有不同机器或旧空 ID 绑定时，在 Relay 设置选择「恢复已有绑定」，确认替换后重新完成 GitHub 授权；不根据账号或硬件信息猜测同机。
- 要删除机器，在 Relay 账号控制台选择「Delete machine」并确认。只有所有者可操作；未确认旧访问关闭时保持待完成，不把断线或超时当作删除成功。
- 删除后机器从列表移除，保留固定域名和必要撤权记录；之后重新通过同账号授权的普通配对可复用该域名。本地 PTY 不因此退出。
- 主机撤权保留账号控制台登录，便于继续查询待完成状态；退出控制台或撤销邀请仍会撤销相应账号授权。
- 请保留独立机器身份文件；整个状态目录丢失或复制不能证明硬件归属。文件损坏、锁冲突或写入失败时，先修复存储，不删除错误文件来绕过配对保护。

## 移动网络上的 P2P 直连

经高延迟隧道访问时，在“设置 → 网络”查看直连诊断；HTTP/WebSocket 入口始终可用，后台协商不会阻塞会话打开。
配置入口见[机会性 P2P 直连](configuration.md#机会性-p2p-直连p2p)。

### 提高可达性

1. 优先让服务器具备可路由的公网 IPv6，并确认手机蜂窝网络也有 IPv6；仅配置域名 AAAA 记录并不足够。
2. 在主机、路由器与云安全组中放行实际 ICE UDP 监听端口的必要流量；Linux 可用 `ss -lunp` 查找 WebMux 的 UDP 监听。
   当前端口由服务启动时选择，重启后可能变化；ICE 使用当前分配的 UDP 端口，HTTP 使用独立的 TCP 监听。
3. 没有端到端 IPv6 时，优先使用服务器公网 IPv4，或保持“本地网关自动端口映射”开启并检查映射状态。
   家宽路由器外还有运营商 CGNAT 时，本地 UPnP / NAT-PMP / PCP 映射不代表已打通外层 NAT。
4. 保留可达的国内与国际 STUN 节点；增加 STUN 只能扩展地址发现机会，不能保证穿过严格 NAT 或 UDP 封锁。

服务器处于严格的、端点相关映射的 NAT 后，也会主动向手机候选地址发送 ICE 探测。
offer/answer 角色不决定 UDP 发起方向，无需设置“反向连接”；若手机候选可达，仍可能通过实际探测来源建立 peer-reflexive 候选。
双方都严格限制映射和入站流量时不能保证直连；普通 Token 连接拒绝 TURN relay，并继续使用现有 WebSocket 回退。官方 Relay 所有者连接只接受按会话授权的 TURN，仍保留 WSS 基线。

### 诊断与移动网络验收

- 后台协商总窗口为 30 秒；每个会话串行交换候选，约每秒拉取一次服务端新增候选，即使浏览器已停止产生本地候选也不会漏掉晚到的服务器地址。
- “未能建立直连”表示协商或直连验证失败；“直连成功，延迟收益不足”表示已验证直连，但没有达到升级阈值，不应统计为穿透失败。
- 连续 3 轮应用层 RTT 比较确认快速路径比基线低超过 `100 ms` 后，仍在旧路径的终端、聊天与会话列表流会自动补迁移；恰好 `100 ms` 不触发这一强收益规则。
  无缝迁移失败时允许短暂断开并重新同步画面；已经升级的流不重复断开，上传下载不迁移，结果未知的输入不会自动重发。请检查逐流路径，而不只看整体连接显示为 P2P。
- 浏览器报告 `online` 或支持的 Network Information `change` 时，取消旧协商并退回基线；合并 1 秒内的连续变化后，用新的 PeerConnection 重新收集候选。
  不提供网络变化事件的浏览器依靠连接状态、心跳和回到前台后的探测恢复。
- 分别在可获得的移动、联通、电信蜂窝网络上，记录设备/浏览器、两端 IP 族、端口映射状态、尝试次数、直连成功次数、实际升级次数、协商耗时和双路径 RTT。
  保持相同服务端部署，覆盖首次打开、Wi-Fi 切蜂窝及反向切换；确认失败时终端仍可使用、回到前台能重新探测。

自动化浏览器和本机 IPv6 回环验证不能替代跨运营商、真机与严格 NAT 环境的实测。

## 访问链接

每次启动都会在标准输出打印 `WebMux local: http://<主机>:<端口>/`。

- 启动打印的本机、隧道与 Tailscale 链接及二维码均不附带 Token，避免凭据进入 journal 或日志文件；在终端执行 `webmux status --json --reveal-token` 查看当前 Token。
- 托盘“复制链接”仍会当场读取 Token 并生成 `#token=<token>` 片段。片段不会发送到服务器，前端读取并保存后立即从地址栏和当前历史记录中移除；剪贴板排除历史并在 60 秒后清除。
- WebSocket 握手通过子协议携带 token，token 不出现在请求 URL 或 Cookie 中。
- 浏览器的自动补全或同步历史仍可能记住托盘复制的带片段链接。

## 公网访问（Cloudflare Tunnel）

```sh
webmux --tunnel
```

选择 `cloudflare` 并重启后，可打开网页“设置 → 公网访问”，在线保存 Quick/Named 配置并启用；`--tunnel` 不会隐式选中功能。首次网页保存前也可在配置中设置：

```toml
[tunnel]
enabled = true
token = "" # 空 = Quick Tunnel；填写 = Named Tunnel
hostname = "" # Named Tunnel 的公共主机名（ASCII/punycode，不支持通配符）
cloudflared = "" # 可选：cloudflared 路径
# download_mirrors = ["https://ghfast.top", "https://gh-proxy.com"]
```

- **Quick Tunnel**：无需账号，每次启动得到随机 `https://*.trycloudflare.com` 地址，无 SLA，适合临时访问。
- **Named Tunnel**：在 Cloudflare 控制台创建隧道，把公共主机名指向 `http://127.0.0.1:<端口>`，在网页保存凭据与主机名（首次保存前也可用 TOML）。凭据经环境变量传给 cloudflared，不出现在命令行中。固定的 HTTPS 地址（Named Tunnel、Tailscale Serve/Funnel 等）也是手机推送的前提。
- 启用隧道要求至少 43 字符的 WebMux 登录 Token；启动需要认证且未配置时生成随机 Token。网页管理配置的恢复失败不会结束本地服务：弱 Token 或 cloudflared 故障显示 failed，可修复后重新开启。网页复制的公网地址不含登录 Token。
- cloudflared 依次取 `tunnel.cloudflared`、`PATH`、`<state_dir>/cloudflared/<版本>/` 缓存；都没有时下载 webmux 固定的版本。
  - 下载先连 GitHub，10 秒无响应或 20 秒无数据时依次改用 `download_mirrors` 镜像（默认 `ghfast.top`、`gh-proxy.com`；`[]` 表示只用 GitHub）。
  - 任何来源的文件都必须与内置 SHA-256 一致才会执行。
  - 镜像是第三方服务，能看到请求者 IP 和所下载的文件；不希望回退到它们时设置 `download_mirrors = []`。

- webmux 以 `--config /dev/null` 运行 cloudflared，`~/.cloudflared/config.yml` 不会影响隧道。
- cloudflared 意外退出时公网地址立即失效（返回 403），本地访问和会话不受影响；webmux 退出时 cloudflared 随之结束。Linux 上即使 webmux 被 SIGKILL 或 OOM 终止，内核也会结束 cloudflared；macOS 没有等价机制，依赖 launchd 结束作业的全部进程，手动运行时需自行确认没有残留。
- 网页配置保存于 `<state_dir>/tunnel-settings.json`（0600），优先于 TOML；`--tunnel` / `--tunnel=false` 只覆盖本次启动。恢复配置文件设置会删除网页配置并立即关闭，下一次启动按 TOML。
- 存储损坏、锁冲突或不可读时保持本地服务可用且公网关闭，不退回 TOML 意外重开。运行中关闭即使保存失败也先撤销公网授权并停止隧道，页面提示尚未持久化；修复存储后须重新保存关闭状态。

Cloudflare 控制台步骤、两个 token 的区别、Windows/WSL 托盘操作、验证方法与故障排查见 **[Cloudflare Named Tunnel 配置指南](../cloudflare-tunnel.md)**。

> 启动链接与二维码不包含 token，以 systemd / launchd 运行时也不会把登录凭据写入 journal 或日志文件。
>
> - 首次启动写入配置文件的 token 会跨重启保留，不会每次更换；只有未持久化的临时 token 会在重启时更换。
> - 当前 token 可通过 `webmux status --json --reveal-token` 查看；托盘复制链接仍按敏感凭据处理。

## HTTPS

宿主普通 HTTP 监听不提供公网 TLS 终止。需要浏览器安全上下文（系统剪贴板 API）时，可使用官方 Relay，或托管的 `[tailscale] mode = "serve"`（仅 tailnet）/ `mode = "funnel"`（显式公网）。Tailscale 的证书、端口、Host/Origin 与启动对账见 [Tailscale Serve / Funnel 入口](../tailscale.md)。

也可以手工使用 `tailscale serve`、Cloudflare Tunnel 或 Caddy 反向代理 HTTPS：

- 保留原始请求 `Host`；
- 把公开的 HTTPS Origin 加入 `allowed_origins`；非 443 端口必须写成带端口的完整 Origin。
