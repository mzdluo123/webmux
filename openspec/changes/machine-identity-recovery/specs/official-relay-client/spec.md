# Spec Delta

## MODIFIED Requirements

### Requirement: 本地密钥与账号配对

WebMux SHALL 在本机生成并保存独立的 Ed25519 中继密钥对。配对接口 SHALL 返回官方验证地址、一次性用户码和剩余有效期，供用户在浏览器完成 GitHub 授权；WebMux 登录 Token MUST NOT 出现在配对请求中。
私钥、轮询句柄和 GitHub 凭据 MUST NOT 出现在状态响应、诊断、日志或浏览器持久化中。配对失败或过期 SHALL 允许重试，不得改写 WebMux 登录 Token。
同一账号且非空机器 ID 相同的重新授权 SHALL 允许自动恢复；不同机器或旧绑定没有机器 ID 时 SHALL 需要显式替换确认。成功后旧私钥被替换，公共地址保持不变。

#### Scenario: 从设置页完成配对

- **WHEN** 用户打开 Relay 设置、开始配对并在官方页面完成 GitHub 授权
- **THEN** WebMux 显示已配对和固定公共地址，本机仅保存中继私钥与绑定信息，不保存 GitHub 访问凭据

#### Scenario: 配对接口不包含登录 Token

- **WHEN** 宿主创建或轮询配对请求
- **THEN** 请求只包含机器 ID、中继公钥或配对材料，不包含 WebMux 登录 Token

#### Scenario: 未配对状态恢复已有宿主

- **WHEN** 本机没有绑定而账号已有不同机器或旧空机器 ID 的绑定
- **THEN** 设置页仍提供明确的替换恢复入口，说明旧身份失效并要求确认，不自动扩大普通配对权限

## ADDED Requirements

### Requirement: 独立持久机器身份

宿主 SHALL 首次配对前生成具有至少 128 位随机熵的机器 ID，并独立于 Relay 绑定与私钥持久化。解绑或重新生成中继密钥 SHALL 保留机器 ID。机器 ID SHALL 仅用于同账号判重，MUST NOT 来自硬件指纹或替代 GitHub 授权。损坏或不可写的机器身份 SHALL 阻止新配对并保留原文件。

#### Scenario: 丢失绑定后同机恢复

- **WHEN** Relay 设置丢失而独立机器身份文件仍完整，用户重新完成同账号授权
- **THEN** 新请求复用原机器 ID，恢复原 host ID 和固定域名，原密钥与派生授权失效

#### Scenario: 机器身份损坏

- **WHEN** 独立机器身份文件格式非法、被锁定或写入失败
- **THEN** 配对明确失败，不静默生成替代 ID，不影响现有本地 PTY
