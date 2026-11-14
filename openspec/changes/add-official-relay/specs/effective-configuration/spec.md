# effective-configuration Specification

## ADDED Requirements

### Requirement: Relay 配置与运行诊断

configured/effective 诊断 SHALL 包含官方 Relay 分组，至少区分默认开关、服务端点、网页启用覆盖、配对状态、当前连接状态、传输模式、实际传输、QUIC 端点、脱敏回退原因和固定公共地址，并按真实解析元数据标记 default、toml、web 或安全降级来源。

诊断 MUST 使用安全 allowlist DTO，私钥、配对码、轮询句柄、GitHub 凭据、会话 Cookie、跳转 code、PKCE verifier 和 WebMux Token 只能以 configured 等布尔元数据表示，MUST NOT 输出原值。存储损坏时 SHALL 显示 fail-closed 后的关闭状态和安全错误，不得声称 TOML 的启用值已生效。

#### Scenario: 查看已配对 Relay 的有效状态

- **WHEN** TOML 默认关闭、网页已配对并启用 Relay，用户查询 effective 诊断
- **THEN** 分组显示启用来源为 web、已配对、当前状态和无 Token 的公共地址，不包含私钥或配对材料

#### Scenario: Relay 状态文件损坏

- **WHEN** TOML 启用 Relay 但受控状态文件损坏，用户分别查询 configured 与 effective
- **THEN** configured 报告磁盘配置与损坏状态，effective 显示安全降级为关闭及原因，两个视图均不输出文件敏感内容
