# Spec Delta

## MODIFIED Requirements

### Requirement: 按能力部署的独立服务器与 WSS 数据面

MVP SHALL 使用单控制实例与至少一个就绪的 `wss` 节点，可显式配置附加 `turn` 节点，不承诺 HA 或多区域调度。控制面 HTTPS 用户控制台 SHALL 允许经 Cloudflare CDN；`wss` 节点的泛域名 HTTPS/WebSocket、连接器及 TURN SHALL 使用 DNS-only 直连。
宿主公开控制 CA-only 政策 MUST NOT 用于节点管理或秘密通道；节点 SHALL 保持独立 mTLS、节点 CA 与管理员登记 SPKI pin，MUST NOT 经终止 TLS 的公共 CDN 替代节点身份认证。
节点能力 SHALL 为 `wss`、`turn` 或两者。`wss`-only MUST NOT 启动 coturn；启用新传输的 WSS 节点 SHALL 在管理代理旁监督固定版本 frps；双能力节点还须独立监督 coturn；`turn`-only SHALL 运行 `webmux-relay` 管理代理与 coturn，仅 coturn 对公网服务。
`turn`-only 的管理代理 SHALL 负责控制同步、配置/秘密应用与报告，MUST NOT 建立 WebMux 公共 HTTPS/WSS 监听。TURN 可独立部署于国内等明确选定节点，选择 SHALL 不依赖当前 WSS 节点。
WSS SHALL 不依赖 coturn 就绪或开放 UDP；任一 TURN 故障 MUST NOT 使健康 WSS 或控制面被判为不可用。

#### Scenario: TURN-only 节点启动

- **WHEN** 管理员为节点仅启用 `turn`
- **THEN** 管理代理完成受信同步后启动 coturn，无 WebMux HTTPS/WSS 入口；宿主仍通过另一个就绪 `wss` 节点访问

### Requirement: 按能力监督的同镜像进程部署

数据面镜像 SHALL 包含 `webmux-relay`、固定版本 frps、coturn 及其运行库；Docker 部署 SHALL 使用 `--init` 提供 PID 1 信号转发与孤儿回收，不在镜像重复打包 Tini。
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

自营网关 SHALL 仅在已登记 connector 域名的专用 WSS 路径转入 frps。公开应用域只接受正常业务路由；节点插件、回源端口、控制接口、仪表盘及健康探针均不得由公网访问。数据转发不依赖新增公网 UDP 映射。
节点 SHALL 独立报告管理同步、frps 和 TURN 实际状态。只有同步成功、插件可拒绝非法注册且数据路径经认证实测可用，才按相应能力分配或公布就绪；控制页面 200、容器启动或开放 socket 均不能替代业务验证。

#### Scenario: 内部监听误发布

- **WHEN** 部署清单把工作端口或插件接口发布至公网，或外部可越过网关直达回源
- **THEN** 发布门禁失败；不启用新能力，不影响既有 TURN 映射和其他使用 443/UDP 的服务。

#### Scenario: frps 故障独立降级

- **WHEN** frps 崩溃或插件失联，节点代理和 coturn 仍健康
- **THEN** 对新模式停止分配和转发，清理残留路由；保留管理监督与健康 TURN，不用聚合失败重启所有组件。

### Requirement: 互斥灰度与安全回滚

控制面 SHALL 只向显式具备匹配能力的宿主及节点分配新模式。按宿主串行撤销旧代际、核实隔离、激活新代际；保留绑定、地址与既有撤销记录。单宿主切换允许短暂 503。
回滚 SHALL 使用新分配版本和新代际、实际已保存镜像与兼容宿主；不恢复旧授权、未经验证的数据库快照或未撤销的旧路由。无安全兼容目标时保持关闭。

#### Scenario: 混合版本与陈旧事件

- **WHEN** 老宿主遇到新节点，或新宿主遇到不支持新模式的节点
- **THEN** 只按明确授权的兼容组合分配；不兼容时报告升级要求。安全拒绝或握手失败不能触发自行降级。

#### Scenario: 灰度失败回滚

- **WHEN** 某批宿主的准入、信任、数据完整性或网络验收失败
- **THEN** 停止扩批，排空并关闭该批新数据路径；用新版本分配回原受信路径，已注销/解绑凭据仍不可用，其他宿主不切换。
