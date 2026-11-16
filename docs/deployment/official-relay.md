# 官方 Relay 部署记录

本页记录官方 Relay 的公开地址、客户端接入前提与实际部署阶段。
客户端操作见[官方 Relay 接入](remote-access.md#官方-relay)；运营者请使用下方私有管理员指南。

> [!WARNING]
> 截至 2026-11-16，控制面和数据节点已启动，公开控制就绪与节点内部状态检查通过。
> 实际连接按[客户端接入说明](remote-access.md#官方-relay)配置认证和宿主；下表列出域名、端口与服务就绪结果。

## 公开地址与计划端口

用户已授权公开生产服务器 IPv4：`172.93.187.102`。域名用途如下，地址记录不代表可达性验证。

| 用途             | 地址                                   | 当前阶段                                 |
| ---------------- | -------------------------------------- | ---------------------------------------- |
| 控制台           | `https://webmux-relay.rainbit.net`     | Cloudflare CDN，`/readyz` 返回 `ok`      |
| 宿主 WSS / HTTPS | `*.us1.webmux-relay.rainbit.net`       | DNS-only，节点内部就绪，业务待验收       |
| 宿主 connector   | `connect.us1.webmux-relay.rainbit.net` | DNS-only，业务待验收                     |
| TURN             | `turn.us1.webmux-relay.rainbit.net`    | DNS-only，内部 coturn 就绪，relay 待验收 |

已配置公网 WSS `443/TCP`、TURN `3478/UDP` 和 `3478/TCP`、TURN TLS `5349/TCP`，以及 relay `52000–52127/UDP`。
WSS/TURN 业务与 TURN 双向 relay 仍待验证；不使用 `443/UDP`。

## 客户端何时可以接入

1. 等待运营者完成真实登录、宿主配对及业务验收，并发布经过实际校验的接入材料。
2. 从受信发行配置或运营者取得 `[relay]` 的 `control_endpoint` 与 `control_audience`；不再提供 `initial_trust` 或控制 SPKI。
3. 按[接入步骤](remote-access.md#官方-relay)配对并显式启用；需要交互优化时选择 `p2p` 功能。宿主数据端点由已认证控制面分配，不手填 connector 或 TURN。
4. 以后用同一 GitHub 账号登录控制台，检查自己的主机是否在线；控制台可打开不等于宿主、WSS 或 TURN 已可用。

控制域名已启用 Cloudflare CDN；WSS/TURN 泛域记录为 DNS-only，直连上述 IPv4。
宿主访问公开控制 HTTPS 只校验 Go 系统 CA（可追加 RootCA）、域名、有效期与 TLS 1.3，不额外固定 SPKI；已分配 WSS 的 pin 与 TURN 服务身份校验不变。
宿主控制消息有效期为 60 秒，心跳发送间隔为 10 秒，失联撤权仍为 30 秒；请求和写入超时仍为 10 秒。三者独立，宿主、控制面与数据节点需同步升级；旧宿主不接受超过旧上限的消息。
所有参与方仍须准确校时，不能以延长有效期替代 NTP，或关闭证书、签名、受众及代际校验。
连接器新版本使用 `webmux-relay.v2`，需要同步升级宿主、控制服务和数据节点；旧 v1 握手返回 426，现有绑定及密钥保留。
默认一条独立管理 WSS、两条 interactive 和四条 bulk，各自使用独立 TCP；宿主主动建立固定槽位并在断线后有界退避补齐。
可在 `[relay]` 设置 `interactive_connections`、`bulk_connections`，范围 1–16，省略或零采用默认值；修改后重启服务。
Relay 统一执行宿主及账号普通流额度，按最少活跃流选择连接，同负载轮转；每 TCP 不再限制为四流，宿主不重复业务准入。
管理流独立于 HTTP 配额和数据带宽桶；总额度为 64 时可承载 64 条普通流，bulk 最多 56，给交互保留容量。
每账号最多 128 个准入等待，最长 15 秒；过载返回 429 和 Retry-After，连接器离线仍返回 503。
接收窗口按每连接配置限制，内存预算需乘以连接数量；普通流继续共享账号数据带宽额度。
数据连接丢失只关闭自身流；管理连接丢失撤销整个代际。已发出的 HTTP 请求不自动重放。
机器身份恢复版本增加独立 `relay-machine.json`，同账号同机器重新授权可自动恢复；旧空机器 ID 绑定仍需显式替换，不能把升级当作恢复授权。
账号控制台提供所有者确认的删除机器操作，保留固定域名及撤权记录；升级先部署控制面，再更新客户端，不自动删除、换钥或恢复现有绑定。

## Dokku 架构与当前阶段

生产环境已核实为 Debian 12、Dokku 0.38.22，采用独立控制面、数据节点与 PostgreSQL 的隔离部署。
已创建两个 Dokku 应用、独立私有网络及 PostgreSQL 17 数据库；数据库仅关联控制面，不暴露公网端口。
两应用已通过 Dokku 镜像部署启动；控制面报告节点在线，期望与实际配置 revision 均为 1，relay/coturn 组件均就绪，秘密版本匹配 v1。

数据节点通过私有网络直连控制面的同名 HTTPS，以节点 CA、SPKI 和 mTLS 验证身份，不绕经 CDN。
公网控制台和宿主入口使用 Dokku Nginx 的 HTTPS 上游代理；模板来自部署镜像 `/app/nginx.conf.sigil`，不改全局 Nginx。
代理保留 Host、WebSocket 与 Origin/CSRF 边界并校验上游证书；不要求 CDN 边缘与内部控制服务使用同一公钥。

真实控制源站证书及 P-256 节点泛域证书已签发并导入 Dokku，有效至 2027-02-13；OAuth 凭据受限保存，回调已确认为 `/v1/auth/github/callback`，Device Flow 必须启用。
服务端固定候选提交已通过私有 CI，主仓库 Relay 集成已通过完整 CI；主仓库 PR #639 仍为 Draft，不是正式发布或生产联合验收结果。
connector 域名的 `/readyz` 返回 404，不是数据面就绪入口；内部权威状态通过不能替代用户业务验收。

- 客户端 Go 控制 HTTPS 探针通过系统 CA 与 TLS 1.3；WSS 通过 CA 与已分配 SPKI 后返回 HTTP 101，未完成连接器认证。
- TURN UDP/TCP/TLS 的未认证 Allocate 均返回 401；生产控制面的设备码申请返回 201，浏览器到达真实 GitHub 登录页。
- 隔离宿主的一次配对请求约 10 秒后返回 503，原因尚未定位；上述分层探针不能替代真实配对、授权 allocation 或双向 relay 验收。
  尚未完成：真实 GitHub 授权/登录和宿主配对、WSS / TURN 业务验收及跨运营商真机长稳。

## 运营入口与信息边界

详细管理员步骤、需填写的配置、证书与秘密交付、启动顺序及运维方法见私有服务端仓库的
[DEPLOYMENT.md](https://github.com/mzdluo123/webmux-relay/blob/master/DEPLOYMENT.md)；运营者需要该仓库访问权限。

公开主仓库只解释客户端接入与运营入口，不公开 SSH 信息、管理用户名、内网地址或其他业务信息。
OAuth Secret、私钥、数据库连接串和管理套接字凭据不得进入本页、公开 PR 或客户端引导。
Relay 会终止 TLS，不是端到端加密；启用前阅读[安全边界](../../SECURITY.md#官方-relay)。
