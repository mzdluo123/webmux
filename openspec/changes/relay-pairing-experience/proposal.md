# Proposal

## Why

Relay 设置把配对、启用和低频诊断混在同一主区，浏览器持有授权上下文使导航中断与临时断网难以恢复。
本次建立宿主权威的授权流程，以页内面板清晰呈现下一步、有效期和操作结果。

## What Changes

- 宿主管理单个活动设备授权、按间隔查询、临时故障退避与明确终止。
- 经既有鉴权及会话内进程限制的专用本机接口恢复用户码与流程；公开状态保持脱敏。
- **BREAKING** 本机配对 API 取消浏览器轮询句柄，迁移全部调用方。
- 未配对、授权中、已配对分区切换；诊断与删除进入次级入口。
- 启停与删除区分提交、核对、完成和失败；删除记录与旧访问关闭独立呈现。
- 补确定性回归与响应式视觉证据，同步现行规范和 US-A10。

## Capabilities

### New Capabilities

- `official-relay-pairing`：从接近规模上限的客户端规格拆出配对、机器身份与设置流程，纳入本次宿主权威授权体验。

### Modified Capabilities

- `official-relay-client`：保留连接器、认证与数据面规范，配对细节引用拆分后的规格。

## Impact

涉及 cmd/webmux 宿主控制器、internal/api 与 contract、本机 Relay schemas、设置 hook/组件、Storybook 和定向测试。
保持 v2 全新绑定协议、v3 持久化、默认关闭、公网确认及原有严格撤权语义。
本地替换与重启按追加授权在 main CI 通过后执行，保留账号、机器 ID、绑定、配置与启用意图；GitHub 授权及远端 Relay 部署不在操作范围内。
