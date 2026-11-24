# Proposal

## Why

官方节点与宿主分别维护 WSS/smux 业务连接池、逐流协议、双向复制和重连。目标是让自营节点用原版 frp 承担反向数据传输，保留 WebMux 对公网 Shell 的身份、限额和撤权责任。

## What Changes

- **BREAKING**：新分配使用受管 `frpc` 与自营 `frps`；公网宿主出站只走 WSS/443。保留独立、已认证的 WebMux 管理 WSS；不在 frp 内嵌套旧业务流协议。
- 浏览器继续使用原固定 HTTPS Origin、GitHub 登录、一次性 code 与宿主 Cookie；上传、转发和长期 WebSocket 均通过授权网关。
- 控制面向已证明持钥的宿主签发节点及代际专属的短期传输授权；节点限制注册范围、工作连接归属与回源身份。用户注销、解绑和管理租约失效按不同粒度撤权。
- 隔离环境验证固定上游版本、TLS 身份、出口防火墙、并发与故障恢复后按宿主灰度；两个版本不得同时服务同一绑定。失败保留旧路径，完成切换后删除旧业务实现。
- 自营数据节点保留既有 TURN 能力；不新开公网 UDP、不引入 EasyTier、外部中转服务、frp 私有分支或浏览器客户端。

## Capabilities

### New Capabilities

- `official-relay-data-transport`：受管发行物与生命周期、实际 WSS 信任、分配专属凭据和回源验证。

### Modified Capabilities

- `official-relay-client`：进程交付与管理、单路业务传输、连接状态及故障处理。
- `official-relay-service`：传输准入、回源和路由授权、流量隔离、撤销和跨版本行为。
- `official-relay-node-management`：按节点能力部署、私有监听、灰度、探针与回滚。
- `effective-configuration`：配置兼容和结构化运行诊断，移除旧连接池参数的虚假含义。
- `server-local-api`：区分旧池与新传输的就绪字段，保留本机接口权限和旧客户端读取边界。

## Impact

- 主仓库：`internal/relay`、`cmd/webmux`、`protocol/relay`、配置、安装包、状态 API、诊断界面及线协议快照。
- 配套私有仓库 `mzdluo123/webmux-relay`：控制面分配、节点代理、`internal/dataplane`、Dokku 镜像、跨仓集成夹具和管理员手册。
- 保留 PostgreSQL 绑定、公共地址、宿主密钥和浏览器授权语义；实施时更新 US-A08、US-A09、US-A10。此提案不更改运行行为或生产部署。
