# Spec Delta

## MODIFIED Requirements

### Requirement: 按能力部署的独立服务器与 WSS 数据面

MVP SHALL 使用单控制实例与至少一个就绪的 `wss` 节点，可显式配置附加 `turn` 节点，不承诺 HA 或多区域调度。控制面 HTTPS 用户控制台 SHALL 允许经 Cloudflare CDN；`wss` 节点的泛域名 HTTPS/WebSocket、连接器及 TURN SHALL 使用 DNS-only 直连。
宿主公开控制 CA-only 政策 MUST NOT 用于节点管理或秘密通道；节点 SHALL 保持独立 mTLS、节点 CA 与管理员登记 SPKI pin，MUST NOT 经终止 TLS 的公共 CDN 替代节点身份认证。
节点能力 SHALL 为 `wss`、`turn` 或两者。`wss`-only 节点 SHALL 在管理代理旁监督固定版本 frps，MUST NOT 启动 coturn；双能力节点还须独立监督 coturn；`turn`-only SHALL 运行 `webmux-relay` 管理代理与 coturn，不启动 frps，仅 coturn 对公网服务。
`turn`-only 的管理代理 SHALL 负责控制同步、配置/秘密应用与报告，MUST NOT 建立 WebMux 公共 HTTPS/WSS 监听。TURN 可独立部署，选择 SHALL 不依赖当前 WSS 节点；原监听、公布地址、凭据策略与防火墙/NAT 映射保持不变。
WSS SHALL 不依赖 coturn 就绪或开放 UDP；任一 TURN 故障 MUST NOT 使健康 WSS 或控制面被判为不可用。

#### Scenario: TURN-only 节点启动

- **WHEN** 管理员为节点仅启用 `turn`
- **THEN** 管理代理完成受信同步后启动 coturn，无 WebMux HTTPS/WSS 入口；宿主仍通过另一个就绪 `wss` 节点访问

### Requirement: 按能力监督的同镜像进程部署

数据面镜像 SHALL 包含 `webmux-relay`、固定版本原版 frps、coturn 及其运行库；Docker 部署 SHALL 使用 `--init` 提供 PID 1 信号转发与孤儿回收，不在镜像重复打包 Tini。
管理代理 SHALL 负责直接启动的 frps 和 coturn 的等待回收、按能力重启、显式停止及有界退出；Docker 运行时 SHALL 配置并执行容器停止宽限及超时强杀，不将重启策略或宽限执行归给 Tini。
coturn SHALL 为独立程序，MUST NOT 通过 CGo 链接进 Go 或自研替代协议。
适用能力 SHALL 分别报告 readiness/liveness 并有独立资源预算；未启用能力不得作为故障探针。TURN 失败 SHALL 降级并保留健康 WSS，MUST NOT 因聚合健康失败反复重启正常基线。
配置变更 SHALL 明确所需重启对象：控制面、Relay 管理/网关进程、frps 或 coturn。TURN-only 配置变更 MUST NOT 重启健康 WSS；整容器升级会中断其运行进程，部署 SHALL 说明共享 CPU/内存/OOM 故障域及同镜像版本发布/回滚。
节点监听、公网地址、中继端口和策略 SHALL 使用中央配置；DNS、防火墙、Docker 映射及 NAT 仍需外部运维配置，不得声称下发会自动修改。WSS 与 TURN MUST NOT 被假定共用 TCP 443；验收 SHALL 验证实际 NAT 双向中继及 TLS/SNI。
Go 程序 SHALL 保持 `CGO_ENABLED=0`；镜像 SHALL 包含 coturn 运行库并纳入固定版本、安全更新及许可审查。节点身份采用预置材料，其余所需秘密按认证通道运行时交付，MUST NOT 写入镜像或日志。

#### Scenario: coturn 崩溃但 WSS 正常

- **WHEN** 双能力容器的 coturn 子进程退出而 `webmux-relay` 健康
- **THEN** 管理代理等待回收并按策略重启 coturn，TURN 降级而 WSS 继续服务，不因单一 TURN 探针失败重启容器

#### Scenario: 数据面容器停止与重新部署

- **WHEN** 容器收到终止信号或升级到另一固定版本镜像
- **THEN** 所有已启用进程有界退出且无孤儿，重启后重新认证并同步配置/秘密后恢复；镜像无运行时秘密，不宣称存量连接未中断

#### Scenario: 控制面不可达时冷启动

- **WHEN** 节点重启且无法连接控制面，即使本地残留旧配置
- **THEN** 节点保持未就绪，不接入新宿主或开放 TURN allocation，不从本地缓存恢复旧秘密或授权

## ADDED Requirements

### Requirement: 自营隧道监听与就绪

自营网关 SHALL 在既有 TCP/443 入口区分公共应用、原管理 WSS 与 frp 专用 Upgrade 路径。
frp 入口只在已登记 connector 域名接受当前分配授权，并只转到节点 loopback 的 frps；公开应用域只接受正常业务路由。
frps 接入、插件、回源端口、控制接口、仪表盘及健康探针不得由公网 IPv4 或 IPv6 访问，不新增公网 UDP 映射。
节点 SHALL 独立报告管理同步、frps 和 TURN 实际状态。WSS 节点可分配前，相关同步、frps 和授权门禁须健康；组件探针、控制页面 200、容器启动或开放 socket 均不能替代每个当前分配的实际网关回源验证。TURN 就绪仍依其独立的配置、秘密同步和服务健康判断。

#### Scenario: 内部监听误发布

- **WHEN** 部署清单把 frps 接入、工作端口或插件接口发布至公网，或外部可越过网关直达回源
- **THEN** 发布门禁失败且不得开放候选入口，不修改既有 TURN 映射

#### Scenario: frps 故障独立降级

- **WHEN** frps 崩溃或插件失联，节点代理和 coturn 仍健康
- **THEN** 关闭受影响的业务准入和转发，清理残留路由，实际回源重新通过后才恢复；保留管理监督与健康 TURN，不因单一组件故障重启所有组件

### Requirement: 整版切换与回滚

宿主、控制面与节点 SHALL 使用同一发布配套制品，在同一维护窗口切换。停旧公网准入、撤销旧代际并确认旧路由、连接和派生访问关闭，在入口关闭时部署配套版本。
只有认证管理、实际 frp 回源、真实终端（含直连、TURN 及回落）和撤权验收均通过，才恢复公网服务。
固定 Origin、绑定、宿主密钥、持久 PTY 与已有授权/撤权记录 SHALL 保留。MUST NOT 并行运行新旧业务路径、按绑定灰度、协商模式或回退旧传输。

回滚 SHALL 先关闭整个公网准入及新签发，废止候选 frp JWT 和回源信任，确认其路由、连接、注册及受管进程已清理，再整体替换为预先验证且与当前权威数据兼容的宿主、控制面及节点制品。
恢复 SHALL 使用当前权威状态、新分配修订和新管理代际重新认证；不得恢复旧令牌、已撤销授权、数据库旧快照或旧活动路由。无法确认隔离或没有安全整版目标时保持入口关闭。

#### Scenario: 整版切换直到验收通过才开放

- **WHEN** 运营方将旧发布替换为配套的 frp 发布
- **THEN** 停服期间旧代际已撤销且所有旧路由和连接均已确认关闭，配套制品及联合验收均完成后才开放公网入口；不接入部分升级的宿主或节点

#### Scenario: 发布失败仅可停服整版回滚

- **WHEN** 准入、信任、撤权、数据完整性、管理或 TURN 验收失败
- **THEN** 停止整个候选版本的公网准入；候选资源确认关闭且整版目标通过验证后才可重新开放，否则保持关闭；已注销/解绑凭据仍不可用，不在运行版本内启用旧传输
