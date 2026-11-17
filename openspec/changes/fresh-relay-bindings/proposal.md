# Proposal

## Why

旧宿主离线时，保留地址的重绑会提交新密钥却一直等待旧宿主 ACK，造成客户端与服务端身份分叉。
当前处于快速迭代阶段，采用用户确认的全新绑定方式，不再维护旧版恢复兼容。

## What Changes

- **BREAKING** 配对及绑定删除使用 v2 契约，删除 rebind 参数和保留域名的恢复入口。
- 已有绑定必须先由宿主签名或 GitHub 所有者授权删除，再创建全新 host ID 和地址。
- **BREAKING** 本地绑定状态仅支持 v3；升级前备份并清除旧绑定状态，不迁移旧密钥。
- 删除绑定不等待离线宿主 ACK；旧直连是否关闭单独说明，不伪造撤权确认。
- 保留账号、邀请资格、本机机器 ID、认证和 Origin 边界。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `official-relay-client`：仅全新配对、删除后重新配对及旧状态清理。
- `official-relay-service`：绑定删除和新地址语义。

## Impact

影响共享协议、relay 客户端、绑定存储、设置界面、US-A08 至 US-A10 及部署说明。
客户端完整 CI 必须通过；服务端 CI 保持禁用，但运行本地定向验证。
用户授权两仓直接提交现有主线；部署清理绑定，不清理账号或机器 ID。
