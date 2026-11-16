# Design

## Context

现有配对会原地更新公钥，再把结果交付阻塞于旧访问 ACK；参见 proposal.md。
用户选择删除旧绑定、生成新地址，允许双端破坏性升级及直接提交主线。

## Goals / Non-Goals

**Goals:** 全新绑定契约、不保留旧版恢复分支、删除状态不误报安全关闭。
**Non-Goals:** 不改 WSS 池、全局退出和邀请撤销协议；不代替用户完成 GitHub 授权。

## Decisions

- 共享协议增加 BindingAPIVersion=2；配对创建、轮询、取消使用该版本及 /v2/enrollments，删除使用 /v2/binding，控制台绑定列表使用 /v2/console/hosts。
- EnrollmentRequest 移除 Rebind，MachineID 必填；已有绑定返回冲突，页面只提供配对及明确删除，不保留恢复别名。
- 删除返回独立 BindingDeletionResult，含 version、revocationId、deleted、accessClosed。LogoutResult 仅用于原全局退出语义。
- 新版仅读取 v3 relay-settings.json，移除 v1/v2 解码及迁移；部署备份后删除旧文件，保留 relay-machine.json。
- 删除记录后清除本机绑定、密钥和 pending 状态，默认关闭；明确告诉用户记录删除不等于所有旧直连已确认关闭。
- SDK 校验 v2 完成结果及删除字段，不重放不确定配对写入；保留明确拒绝和不确定失败的现有清理边界。

## Risks / Trade-offs

- [旧域名失效] → 升级及删除确认明确提示，新配对生成新地址。
- [离线 P2P 尚未关闭] → 不伪造 ACK，返回 accessClosed=false 并保留警示，全局退出规则不变。
- [旧状态阻止新版本启动 Relay] → 部署先备份清理；不宽松忽略未知或损坏状态。
- [直接主线和服务端 CI 禁用] → 客户端完整 CI 与服务端本地目标回归仍须通过。

## Migration Plan

先发布共享协议并固定两仓依赖，完成本地验证及客户端 CI。
备份数据库、状态及运行镜像；部署服务端清理迁移，再清理本地旧绑定状态并更新二进制。
本地服务重启会终止既有 PTY，切换前明确告知；实际验证新格式状态和节点健康，不声称替用户完成 GitHub 授权。
回滚必须同时恢复匹配版本的数据库与客户端状态；不得仅回滚二进制重新启用旧身份。
