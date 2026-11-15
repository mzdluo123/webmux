# official-relay-node-management Specification

## Purpose

定义官方 Relay 节点的管理员配置、能力与地址发现、秘密交付、冷启动和进程监督；与 [official-relay-service](../official-relay-service/spec.md) 的用户授权及转发要求共同验收。

本能力从原服务规范原文迁移节点管理条款，不改变已确认行为；节点配置/应用状态不等于 WebMux 宿主本地设置或用户登录授权。

## ADDED Requirements

### Requirement: 管理员配置与版本化应用

节点非秘密配置 SHALL 由受限管理员通过 CLI/API 提交控制面校验并事务性存入 PostgreSQL，MUST NOT 以直接 SQL 或普通用户权限管理。节点引导配置 SHALL 仅含控制地址、稳定 nodeId、预置身份私钥与初始信任。
其余监听、公网地址、中继端口、能力和策略 SHALL 由控制面集中管理。控制面 SHALL 通过 mTLS 按节点推送单调版本的完整快照；节点 SHALL 校验 nodeId、内容摘要、版本、能力和字段，再应用或拒绝，重连 SHALL 全量同步。
低版本、错节点或同版本不同摘要 SHALL 被拒绝；同版本同摘要 SHALL 允许幂等重同步和确认，MUST NOT 重复重启。节点 SHALL 分别报告 desired/applied 版本、实际状态及失败原因，不以送达确认冒充应用。
热变更 SHALL 在校验成功后应用；影响运行服务的破坏性变更 SHALL 保留当前有效配置并报告 pending-restart，只有管理员通过 CLI 显式批准精确版本及重启对象后才执行。新版本 MUST NOT 继承旧批准。
初始启动在在线完成配置、秘密及授权同步后 SHALL 可直接激活，不等待针对运行中破坏性变更的重启批准。应用失败 SHALL 保留最后有效状态并报告错误，MUST NOT 谎报 applied。
回滚 SHALL 创建新配置版本并遵循相同校验/批准；MUST NOT 回滚安全撤销、恢复过期授权，或假定配置下发能更改外部 DNS、防火墙及容器映射。

#### Scenario: 配置送达但等待重启

- **WHEN** 运行节点收到需要重启 coturn 的新版本，管理员尚未批准
- **THEN** desired 更新而 applied 保持旧版本，报告 pending-restart；精确版本批准后仅重启所需 coturn，健康 WSS 不重启

#### Scenario: 拒绝错节点或重放快照

- **WHEN** 已认证通道收到其他 nodeId、低版本或同版本不同内容的快照
- **THEN** 节点拒绝配置并报告原因，不改变实际配置、授权或运行进程

#### Scenario: 重连幂等同步

- **WHEN** 节点重连取得与已应用版本及摘要相同的完整快照
- **THEN** 节点校验后确认当前 applied，不重复执行重启，也不接受摘要不同的同版本配置

### Requirement: 控制面唯一发现源与用途隔离

控制面 SHALL 仅采用管理员登记并校验的 nodeId、capabilities 和用途字段，不接受浏览器任意地址或节点自报地址扩展信任。`wss` 能力 SHALL 登记 connectorUrl 与 publicHostSuffix；`turn` 能力 SHALL 登记 turnUrls 与 advertisedPublicIp。
需要 NAT 时 SHALL 显式登记公网映射；无对应能力的字段 MUST 被拒绝。TURN 选择 SHALL 独立于 WSS 分配，仅选就绪且秘密版本已激活的授权节点。
宿主 SHALL 仅预配置 controlEndpoint 与引导信任，经已认证控制面取得 connectorUrl、数据面 origin、受众及对应服务 TLS 信任；数据面仍 SHALL 验证标准 TLS 与 pin，不得因任意 URL 自动学习未知密钥。
控制信任变更 SHALL 撤销旧会话并要求重新配对；数据端点变更 SHALL 由现有受信控制分配版本化处理，MUST NOT 静默扩大信任。诊断 SHALL 区分配置的控制 origin、已分配数据端点和各自可达性。
浏览器 SHALL 继续访问固定宿主 HTTPS，仅使用经登录授权的 ICE 配置；文件及全部 `/forward/*` SHALL 保持 WSS，交互优化 SHALL 以 WSS 为基线并依据实测决定。

#### Scenario: 能力不匹配的发现字段

- **WHEN** 管理员向 turn-only 节点提交 connectorUrl/publicHostSuffix，或向 wss-only 节点提交 TURN 字段
- **THEN** 控制面拒绝保存和分配，不创建错误能力入口，也不尝试以自报字段绕过校验

#### Scenario: 未知端点或密钥不能扩展信任

- **WHEN** 浏览器或节点提供未登记地址，或数据端点未通过受信分配及 TLS/pin 校验
- **THEN** 服务拒绝发现或连接，不自动学习公钥；合法数据端点更新不改写 controlEndpoint

#### Scenario: 控制可达但数据端点不可达

- **WHEN** 控制面正常但分配的 WSS 地址不可达
- **THEN** 诊断分别显示控制和数据地址及状态，不将控制面可达等同于公网入口可用，也不切换到任意节点

### Requirement: 按能力部署的独立服务器与 WSS 数据面

MVP SHALL 使用单控制实例与至少一个就绪的 `wss` 节点，可显式配置附加 `turn` 节点，不承诺 HA 或多区域调度。控制面提供 HTTPS 用户控制台；`wss` 节点提供泛域名 HTTPS/WebSocket 与 WSS 连接器，不依赖 CDN。
节点能力 SHALL 为 `wss`、`turn` 或两者。`wss`-only MUST NOT 启动 coturn；双能力节点 SHALL 启动两进程；`turn`-only SHALL 运行 `webmux-relay` 管理代理与 coturn，仅 coturn 对公网服务。
`turn`-only 的管理代理 SHALL 负责控制同步、配置/秘密应用与报告，MUST NOT 建立 WebMux 公共 HTTPS/WSS 监听。TURN 可独立部署于国内等明确选定节点，选择 SHALL 不依赖当前 WSS 节点。
WSS SHALL 不依赖 coturn 就绪或开放 UDP；任一 TURN 故障 MUST NOT 使健康 WSS 或控制面被判为不可用。

#### Scenario: TURN-only 节点启动

- **WHEN** 管理员为节点仅启用 `turn`
- **THEN** 管理代理完成受信同步后启动 coturn，无 WebMux HTTPS/WSS 入口；宿主仍通过另一个就绪 `wss` 节点访问

### Requirement: 服务秘密加密存储与按节点交付

节点服务 TLS 私钥、TURN 共享密钥等秘密 SHALL 在 PostgreSQL 加密存储；加密主密钥 SHALL 仅运行时注入控制面，不得同库保存。GitHub OAuth Client Secret 与数据库凭据 SHALL 仅注入控制面；GitHub access token 仍短期使用后丢弃。
节点身份私钥和初始信任 SHALL 预置，不得从未认证下载引导。控制面 SHALL 以独立、双向 TLS 认证且绑定 nodeId 的秘密通道，只交付该节点需要的秘密；MUST NOT 放入通用配置快照、CLI 输出、日志或公开 API。
秘密 MUST NOT 进入仓库、客户端或镜像层。节点 SHALL 仅在内存或权限受限的临时运行文件保存所需秘密，为 coturn 生成运行配置，退出后清理；MUST NOT 使用持久秘密缓存恢复离线服务。
TURN 密钥版本 SHALL 与配置版本协调；只有目标 coturn 确认相应密钥版本已激活后，控制面才可签发对应凭据。旧密钥移除 SHALL 视为破坏性变更并经显式版本批准，不得声称凭据过期立即杀死已有 allocation。
服务 SHALL 提供匹配公开信任配置的 TLS 身份，并配合重叠信任集合安排正常公钥轮换。公开地址、公钥、OAuth Client ID 与回调 URI 不是秘密，不能授予账号或主机权限。

#### Scenario: 秘密加密及节点隔离

- **WHEN** 查看数据库、通用快照或 CLI 输出，或已认证节点请求另一个 nodeId 的秘密
- **THEN** 数据库仅有密文，快照与输出不含秘密，跨节点请求被拒绝；客户端发布产物仅含地址与公开信任材料

#### Scenario: 新 TURN 密钥尚未激活

- **WHEN** 控制面已保存新密钥但尚未收到目标 coturn 的激活版本确认
- **THEN** 不签发使用新密钥的凭据，旧密钥移除仍等待对应版本批准，不能以配置已送达代替激活

### Requirement: 控制面 PostgreSQL 与数据面隔离

`webmux-control` SHALL 负责 GitHub、用户控制台、主机绑定、授权、配置/秘密管理与 TURN 凭据签发；`webmux-relay` SHALL 按能力负责管理同步、宿主身份校验、精确 Host 路由、WSS 转发、节点配额与聚合计量，MUST NOT 自行授予 GitHub 用户权限。

只有控制面 SHALL 访问业务 PostgreSQL，使用版本化 migration、有界连接池及查询/事务超时。数据面和 coturn MUST NOT 持有业务数据库凭据或逐包查询数据库；宿主 SHALL 继续使用本地受控状态文件，不要求安装 PostgreSQL。

数据面 SHALL 经双向 TLS 认证控制通道取得绑定、公钥、节点受众、代际、限额及撤销。节点冷启动 SHALL 先取得有效配置、所需秘密并确认完整授权同步，之后才可将对应能力报告就绪或接入新宿主/TURN allocation。
控制面不可达时 MUST NOT 以本地快照或持久秘密冷启动离线服务。运行中授权 SHALL 有界有效且受健康通道监督；失联 SHALL 停止新授权及凭据。
检测超时后 SHALL 关闭关联宿主和派生访问，并停止失去监督的 coturn 公网中继，MUST NOT 无限使用过期权限。

#### Scenario: 数据面伪造或重放控制授权

- **WHEN** 节点未认证，或控制消息的受众、版本、代际或有效期不匹配
- **THEN** 拒绝授权/路由变更，不影响合法当前绑定，不通过普通公网代理头信任身份

#### Scenario: 数据库或控制通道故障

- **WHEN** PostgreSQL 不可用，或数据面丢失受信控制通道
- **THEN** 拒绝需要重新验证的授权操作，存量访问仅在有效监督范围内运行；失联超时撤权，不绕过数据库或要求宿主改用数据库

### Requirement: 按能力监督的同镜像进程部署

数据面镜像 SHALL 包含 `webmux-relay`、固定版本 coturn 与 PID 1 进程管理器，按能力分别监督进程、回收子进程、传递终止信号并有界退出。coturn SHALL 为独立程序，MUST NOT 通过 CGo 链接进 Go 或自研替代协议。
适用能力 SHALL 分别报告 readiness/liveness 并有独立资源预算；未启用能力不得作为故障探针。TURN 失败 SHALL 降级并保留健康 WSS，MUST NOT 因聚合健康失败反复重启正常基线。
配置变更 SHALL 明确所需重启对象：控制面、Relay 管理/WSS 进程或 coturn。TURN-only 配置变更 MUST NOT 重启健康 WSS；整容器升级会中断其运行进程，部署 SHALL 说明共享 CPU/内存/OOM 故障域及同镜像版本发布/回滚。
节点监听、公网地址、中继端口和策略 SHALL 使用中央配置；DNS、防火墙、Docker 映射及 NAT 仍需外部运维配置，不得声称下发会自动修改。WSS 与 TURN MUST NOT 被假定共用 TCP 443；验收 SHALL 验证实际 NAT 双向中继及 TLS/SNI。
Go 程序 SHALL 保持 `CGO_ENABLED=0`；镜像 SHALL 包含 coturn 运行库并纳入固定版本、安全更新及许可审查。节点身份采用预置材料，其余所需秘密按认证通道运行时交付，MUST NOT 写入镜像或日志。

#### Scenario: coturn 崩溃但 WSS 正常

- **WHEN** 双能力容器的 coturn 子进程退出而 `webmux-relay` 健康
- **THEN** 监督器回收并按策略重启 coturn，TURN 降级而 WSS 继续服务，不因单一 TURN 探针失败重启容器

#### Scenario: 数据面容器停止与重新部署

- **WHEN** 容器收到终止信号或升级到另一固定版本镜像
- **THEN** 所有已启用进程有界退出且无孤儿，重启后重新认证并同步配置/秘密后恢复；镜像无运行时秘密，不宣称存量连接未中断

#### Scenario: 控制面不可达时冷启动

- **WHEN** 节点重启且无法连接控制面，即使本地残留旧配置
- **THEN** 节点保持未就绪，不接入新宿主或开放 TURN allocation，不从本地缓存恢复旧秘密或授权
