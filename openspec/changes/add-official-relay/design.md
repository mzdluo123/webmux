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
- 在 TCP 443 不可被 UDP 或 HTTP 代理阻断的环境中提供稳定基线，并完整复用 WebMux HTTP/WebSocket 行为。
- 宿主只主动连接配置的 Relay，只把 stream 接到本机 WebMux 回环地址。
- 密钥、绑定和启停状态可恢复，所有公网授权都能同步撤销。

**Non-Goals:**

- 不自研加密、可靠传输、HTTP 帧或 WebSocket 帧协议。
- 不在 MVP 引入 ACME 自动申请、分布式会话目录、跨节点转发或第二数据面。
- 不把 Relay 私钥交给浏览器，也不让官方账号代替 WebMux Token。

## Decisions

### 1. 单二进制、分包职责

新增 `cmd/webmux-relay`，在同一进程中监听公共 HTTPS 与连接器 WSS。包边界如下：

- `internal/relayproto`（shared）：协议版本、签名 transcript、challenge 类型和多路 stream 适配；DTO 仍放在 `internal/contract`。
- `internal/relaycontrol`（service）：GitHub 授权交换、邀请名单、账号/主机注册、重新绑定、撤销和 SQLite 事务，不依赖 HTTP 服务端。
- `internal/relay`（client）：宿主连接器、WSS 认证、yamux 会话、回环拨号和重连，不导入 WebMux 服务或控制器。
- `internal/relaysettings`（service）：宿主启用意图与私钥的受控持久化。
- `internal/api/relay*.go`（controller）：宿主 `/api/relay*` 和 Relay 控制面、连接器及公共代理处理器，复用统一错误映射。
- `cmd/webmux`、`cmd/webmux-relay` 分别组装上述服务，不共享进程内单例。

**备选方案**：独立 Relay 网关控制器包。否决原因是服务 sentinel 到 HTTP 状态的映射会绕开仓库规定的 `internal/api/errors.go`，并形成第二套错误约定。

### 2. WSS + yamux 双向 stream

宿主先以 `coder/websocket` 建立二进制 WSS，完成短控制帧认证后，用 `websocket.NetConn` 将连接适配为 `net.Conn`，再运行 `github.com/hashicorp/yamux` 会话。Relay 为每个代理连接调用 `OpenStream`，宿主从 `AcceptStream` 取得连接。

选择 yamux 的原因是双方均可主动建流、按 stream 窗口更新实现背压、带保活，并能直接在有序可靠字节流上运行。实现时固定：

- 每条 WSS 只有一个读取循环和一个序列化写入方；
- 每主机一条 yamux 会话，配置最大 stream、接受背压、窗口和保活参数；
- 所有 stream 由 context 和关闭路径统一回收，不在锁内执行网络读写；
- yamux 只提供传输，不承载业务 JSON。

**备选方案**：自定义 WebSocket 分片帧。否决原因是会重复实现流控、半关闭和背压。QUIC 虽原生支持双向 stream，但 UDP 被拦截时仍需 WSS 兜底，MVP 不维护两种数据面。

### 3. `httputil.ReverseProxy` 保留应用语义

公共 HTTPS 处理器按精确 Host 找到活动 yamux 会话，并使用 `httputil.ReverseProxy` 转发到虚拟回环目标。自定义 `Transport.DialContext` 打开 yamux stream；宿主接受后只拨号启动时固定的 `127.0.0.1:<listen-port>`。

代理使用 `Rewrite` 清理伪造转发头、保留原始 Host 和 Origin。Go 反向代理负责普通请求、流式正文和 WebSocket Upgrade，Relay 不解析终端帧、不重写 WebSocket 关闭码，也不完整缓冲上传或回放。

**备选方案**：把 API 重新封装成 RPC。否决原因是 `/forward/*`、Cookie、multipart、WebSocket Upgrade 与现有几十条路由会产生双轨行为，且容易遗漏鉴权。

### 4. GitHub 设备授权与 Ed25519 身份

配对流程：

1. WebMux 原子保存新生成的私钥，向 Relay 提交公钥。
2. Relay 创建 GitHub Device Flow 和本地配对记录，宿主展示 GitHub 验证地址与用户码。
3. Relay 轮询 GitHub，授权后只用访问凭据读取账号数字 ID 和登录名快照，随即丢弃访问凭据。
4. 邀请名单通过后，在一个 SQLite 事务内创建绑定；重新绑定必须带显式标志并替换旧公钥。
5. 宿主轮询自己的配对句柄取得 host ID 与固定公共地址。

连接认证在 WSS 控制帧内完成：宿主声明协议版本和 host ID，Relay 返回单次随机 nonce，宿主签名以下 transcript：

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

新增 `relay-settings.json`，记录版本、启用指针、host ID、公共地址和私钥 seed；TOML `[relay]` 只提供 `enabled` 默认值与 `endpoint`。私钥不进入 configured/effective DTO、API 响应或日志。

宿主控制器复用 tunnel 的模式：任何应用、关闭、失败和退出先 `PublicOrigins.Clear("relay")`，再递增代际并取消旧 worker；新 worker 等前任退出。认证和 yamux accept 循环均就绪后才 `Set("relay", publicURL)`。

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

MVP 由运维者提供通配符 DNS、证书和私钥文件，进程支持证书热重载或由服务管理器重启；ACME 留到后续。GitHub OAuth Client ID 与密钥通过环境或受限服务配置注入，不进入日志。

单节点配置包括邀请账号、配对/IP 速率、每主机 stream、每连接缓冲和保活参数。公共代理不设置会误杀终端 WebSocket 的短响应头超时；认证和建流使用独立短超时。健康接口只报告 readiness，不暴露账号或主机清单。

## Risks / Trade-offs

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

- 正式公共域名、首个 VPS 区域、证书续期方式和邀请名单由运营部署决定，不改变协议或任务拆分。
- yamux 的具体窗口、保活和 stream 默认值需通过第一阶段基准确定，规格只要求有界与可配置，不预设未经测量的数值。
