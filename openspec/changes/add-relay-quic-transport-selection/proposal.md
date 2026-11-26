# Proposal

## Why

宿主目前通过 WSS 连接官方节点。希望在同一节点和同一浏览器访问路径上试用 QUIC，判断不同网络下哪种传输更适合自己的终端。
这是一项可选实验；502 的发生率属于对照指标，不能预设切换后会降低。

## What Changes

- 「设置 → 官方中转」提供宿主级的「测试并应用」；WSS/TCP 保持默认，QUIC/UDP 标为实验。
- 节点显式发布受信任的 QUIC 能力；候选链路通过鉴权和实际回源后才接替当前链路。
- 设置与本机 CLI 区分已保存偏好、实际生效协议和最近试连结果。
- 浏览器使用原 HTTPS 地址；管理连接、登录、授权撤销和 PTY 生命周期保持原语义。

## Capabilities

### New Capabilities

- `official-relay-transport-selection`：节点能力、数据协议试连、提交或保留原连接、跨客户端诊断与恢复。

### Modified Capabilities

- 本增量基于 `replace-relay-data-plane-with-frp`：其 WSS 专属约束继续适用于默认路径；协商新能力后才适用本规范的 QUIC 路径。两项变更归档时需同步这一限定。

## Impact

宿主、共享协议、设置页与 CLI 属于 WebMux；UDP 入口、候选授权和路由切换涉及私有 Relay 服务。先隔离验收，再由运营者决定是否启用生产节点。
实施影响 US-A10（配对与接入）、US-A09（节点运营）；本提案不改变当前用户行为。
