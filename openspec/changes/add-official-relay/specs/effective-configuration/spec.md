# effective-configuration Specification

## ADDED Requirements

### Requirement: Relay 配置与运行诊断

configured/effective 诊断 SHALL 包含官方 Relay 分组，至少区分默认开关、服务端点、网页启用覆盖、官方内置或自建覆盖来源、公开 key ID、配对状态和固定公共地址。

运行诊断 SHALL 区分固定 wss 基线与每浏览器会话的实际交互路径（wss、webrtc-direct 或 webrtc-turn），包含当前连接状态、control/bulk 就绪数、bulkReady、可观测的实际 TCP 算法与脱敏重连原因；无法读取算法时 SHALL 明示 unknown。配置来源 SHALL 按真实解析元数据标记 default、toml、web 或安全降级。

诊断 SHALL 显示优化是否启用、TURN 配置是否可用及来自已配对控制面的来源、公开节点标签、探测/未升级/回落原因。多浏览器路径不同 SHALL 按会话区分，不能把一个会话的 TURN 路径描述成全宿主传输；默认不输出候选 IP 或凭据化 URL。

诊断 MUST 使用安全 allowlist DTO，私钥、配对码、轮询句柄、GitHub 凭据、会话 Cookie、跳转 code、PKCE verifier、TURN 长期密钥/短期用户名和密码及 WebMux Token 只能以 configured 等布尔元数据表示，MUST NOT 输出原值。存储损坏时 SHALL 显示 fail-closed 后的关闭状态和安全错误，不得声称 TOML 的启用值已生效。

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
