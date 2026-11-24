# Proposal

## Why

官方节点与宿主分别维护 WSS/smux 业务连接池、逐流协议、双向复制和重连。目标是让自营节点用原版 frp 承担反向数据传输，保留 WebMux 对公网 Shell 的身份、限额和撤权责任。

## What Changes

- **BREAKING**：受管 `frpc` 与自营 `frps` 接管宿主和节点之间的 TCP 业务连接；移除自建 interactive/bulk 连接池与逐流协议。管理连接继续使用 `webmux-relay.v2` 的 WSS、smux 和管理 RPC，仅扩展 frp 授权与就绪消息。
- 固定 HTTPS Origin、GitHub 登录、一次性 code 与宿主 Cookie 不变。文件传输和 `/forward/*` 使用 frp 基线；获授权的浏览器交互继续支持 WebRTC 直连或 TURN。
- 控制面向已证明持钥的宿主签发节点及代际专属的短期传输授权；节点限制注册范围、工作连接归属与回源身份。用户注销、解绑和管理租约失效按不同粒度撤权。
- 宿主、控制面与节点配套升级，一次切换；不提供旧实现、旧参数或按绑定选择传输的开关。发布失败可停服整版回滚，不能恢复已撤销授权。
- 保留 coturn、端口、短期凭据与 ICE 选路策略。验收强制实际终端流量经 TURN 双向收发，再中断 TURN，验证回落到 frp 基线后输出连续、输入不重复。

## Capabilities

### New Capabilities

- `official-relay-data-transport`：受管发行物与生命周期、实际 WSS 信任、分配专属凭据和回源验证。

### Modified Capabilities

- `official-relay-client`：进程交付与管理、单路业务传输、连接状态及故障处理。
- `official-relay-service`：业务准入、回源和路由授权、流量隔离与撤销。
- `official-relay-node-management`：按节点能力部署、私有监听、探针与整版回滚。
- `effective-configuration`：移除旧池参数和就绪计数，分别报告业务基线和浏览器实际路径。
- `server-local-api`：单个 `relay` 状态对象报告管理与业务就绪，本机权限不变。

## Impact

- 主仓库：`internal/relay`、`cmd/webmux`、`protocol/relay`、配置、安装包、状态 API、诊断界面及线协议快照。
- 配套私有仓库 `mzdluo123/webmux-relay`：控制面分配、节点代理、`internal/dataplane`、Dokku 镜像、跨仓集成夹具和管理员手册。
- 保留 PostgreSQL 绑定、公共地址、宿主密钥和浏览器授权语义；实施时更新 US-A08、US-A09、US-A10。此提案不更改运行行为或生产部署。
