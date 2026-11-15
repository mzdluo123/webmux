# effective-configuration Specification

## ADDED Requirements

### Requirement: Relay 配置与运行诊断

configured/effective 诊断 SHALL 包含官方 Relay 分组，区分默认开关、网页启用覆盖、配对状态和固定公共地址。
configured SHALL 标明 `controlEndpoint`、引导信任的公开 key ID 及官方内置或自建覆盖来源；effective SHALL 单独标明已认证控制面分配的 `connectorUrl`、数据面 origin、受众公开标识及分配版本/来源，不把它们当成手工第二端点。

运行诊断 SHALL 区分固定 wss 基线与每浏览器会话的实际交互路径（wss、webrtc-direct 或 webrtc-turn），包含当前连接状态、control/bulk 就绪数、bulkReady、可观测的实际 TCP 算法与脱敏重连原因；无法读取算法时 SHALL 明示 unknown。
配置来源 SHALL 按真实解析元数据标记 default、toml、web 或安全降级；有效数据面分配 SHALL 另标记 authenticated-control 来源，包含关联配置版本，不伪装成本机 TOML 或网页设置。

诊断 SHALL 分别显示配置的控制面 origin 与有效数据面端点的可达性、认证/信任状态及脱敏错误；未探测或没有有效分配时显示 unknown 或 unassigned，MUST NOT 以一个端点可达推断另一个可达。
诊断 SHALL 显示优化是否启用、TURN 配置是否可用及来自已配对控制面的来源、相关公开 nodeId/节点标签与 capabilities、探测/未升级/回落原因；WSS 节点和获授权 TURN 节点可以不同，MUST NOT 将 TURN-only 节点标为 WSS 目标。
多浏览器路径不同 SHALL 按会话区分，不能把一个会话的 TURN 路径描述成全宿主传输；默认不输出候选 IP 或凭据化 URL。

诊断 MUST 使用安全 allowlist DTO；私钥、配对码、轮询句柄、GitHub 凭据、会话 Cookie、跳转 code、PKCE verifier、TURN 长期密钥/短期用户名和密码及 WebMux Token 只能以 configured 等布尔元数据表示，MUST NOT 输出原值。
控制面加密主密钥、节点身份私钥、节点服务 TLS 私钥及任何下发秘密 MUST NOT 进入宿主诊断；只允许相关公开节点、能力、配置/分配版本及公开 key ID，不暴露节点秘密快照或原始配置对象。
存储损坏时 SHALL 显示 fail-closed 后的关闭状态和安全错误，不得声称 TOML 的启用值已生效。

节点管理配置、期望/已应用配置版本及 pending-restart SHALL 属于独立控制服务的管理员状态，不属于 WebMux 宿主本地配置。
宿主 SHALL 仅展示与当前分配有关的公开版本/状态；普通 GitHub 登录、宿主设置页或本机配置权限 MUST NOT 被解释为节点管理员权限，宿主诊断也不得提供节点配置或秘密编辑入口。

#### Scenario: 查看已配对 Relay 的有效状态

- **WHEN** TOML 默认关闭、网页已配对并启用 Relay，用户查询 effective 诊断
- **THEN** 分组显示启用来源为 web、已配对、当前状态和无 Token 的公共地址，不包含私钥或配对材料

#### Scenario: Relay 状态文件损坏

- **WHEN** TOML 启用 Relay 但受控状态文件损坏，用户分别查询 configured 与 effective
- **THEN** configured 报告磁盘配置与损坏状态，effective 显示安全降级为关闭及原因，两个视图均不输出文件敏感内容

#### Scenario: 不同浏览器使用不同交互路径

- **WHEN** 一个浏览器升级到 TURN，另一个浏览器保持 WSS，用户查询运行诊断
- **THEN** 基线仍显示 wss，各会话分别显示实际路径和脱敏原因，不输出 TURN 凭据或候选 IP

#### Scenario: TURN 故障但 WSS 正常

- **WHEN** TURN 不可达或凭据刷新失败而 WSS control/bulk 均就绪
- **THEN** 诊断显示优化不可用或回落原因，不把基线标成离线，不泄露短期认证材料

#### Scenario: 控制面可达但数据面不可达

- **WHEN** 配置的控制面认证成功并分配了另一 origin 的 WSS 节点，但该节点的 WSS 握手失败
- **THEN** configured 显示控制面配置来源，effective 显示认证分配来源及版本；控制面显示可达，数据面单独显示失败，不把 Relay 标为就绪

#### Scenario: 数据面仍通而控制面不可达

- **WHEN** 已有数据面连接仍可观测，但配置的控制面暂不可达
- **THEN** 诊断分别记录两个端点状态，不把已有 WSS 可达当成控制面授权同步成功，也不据此绕过既有断线撤权规则

#### Scenario: 独立 TURN 节点与秘密脱敏

- **WHEN** WSS 节点与当前浏览器获授权的 TURN-only 节点不同，用户查询 effective 诊断
- **THEN** 仅显示各自相关公开节点标识、能力、版本及脱敏可用性，不把 TURN 节点当成宿主连接器目标，不输出节点主密钥、TLS 私钥、TURN 共享密钥或短期认证材料
