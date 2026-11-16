## Why

单用户 WebMux 的交接只需要读取 Agent 原生历史、预览机械投影并发送一次。
现有持久交接事务、跨重启恢复、逐项覆盖及独立保留 Fork 超出已确认的产品需求，继续维护 SQLite 或重造文件账本都增加复杂性。

## What Changes

- 复用九个原生 reader、来源校验及独立 `snapcompact-go v0.1.0`，默认采用机械文本投影。
- 预览固定原生身份、分支、读取边界与内容 hash；当前用户任务独立原样保留。
- 进程内有界保存短期预览和操作 ID，单次发送；断线或取消不能触发自动重发。
- **BREAKING**：删除交接的数据库草稿、revision 协作、资源 checkpoint、重启续跑、逐项回执及 A→B→A 增量覆盖。
- **BREAKING**：删除交接专用原生历史注入与回退分支，统一通过普通文本 prompt 发送；不影响 TUI/ACP 切换与 Agent 原生恢复。
- **BREAKING**：新聊天继承一次投影，不再是拥有独立完整前缀的 Conversation Fork；不保证父会话或原生来源删除后的精确补查。
- **BREAKING**：移除 WebMux `conversations.sqlite` 服务、schema、旧版本迁移、存储描述符及全部生产消费者，不增加替代控制账本。
- 保留 `hybrid-sessions.json` 中原生身份与恢复所需的已有元数据，不持久化新增交接操作或历史正文。
- 旧数据库及其 sidecar 不自动打开、升级或删除；仅作为旧版离线备份保留，不提供旧草稿、回执和独立快照的在线兼容入口。
- Agent 自己的 OpenCode、agy、Devin SQLite 仍通过只读 reader 访问，不能删除相应驱动依赖而破坏原生历史。
- 同步 REST、MCP、CLI、前端协议和说明，移除旧恢复与投递模式入口；旧路由返回 410，客户端显示失效状态。

## Capabilities

### New Capabilities

- `simple-native-handoff`：原生历史冻结、Snapcompact 文本预览、进程内防重和一次性投递，以及失效、取消和旧数据边界。

### Modified Capabilities

- `persistence-governance`：移除 conversation 存储后同步实际 descriptor、契约和清单；历史遗留文件不再是运行依赖。

## Impact

- 删除目标以 `internal/conversation`、`internal/hybrid` 的 conversation/injection/dispatch 对接及 `internal/handoff` 的持久流程为核心。
- 重组 `internal/historyimport` 的存储依赖；保留 `internal/historysource` 中读取需要的领域值和 `internal/historyprojection`。
- 更新 `cmd/webmux` 装配、`featureapi/featurewire/contract`、`threadread`、MCP/本机控制客户端及相关 UI、Zod 和 wire fixtures。
- 普通聊天、Agent 原生恢复、身份占用检测、认证/Origin、PTY 持久会话及已有非 conversation 存储不在删除范围。
- 受影响用户故事以 US-Q20 为主，同时核对 US-Q12、US-Q19、US-K04 和依赖交接的工作区故事；保留故事 ID。
- 本变更刻意撤销 #651 的持久控制和独立保留快照承诺；#650 保持原状态，#640/#652 的范围调整为原生来源、机械投影及普通 prompt。
