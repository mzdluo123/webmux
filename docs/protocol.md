# API 与协议

API 以 `/api` 为前缀，WebSocket 以 `/ws` 为前缀。禁用或未知 API/WS 路径在 SPA 回退前返回 404；其余前端路径返回嵌入应用。数据契约：

- 前端的完整数据契约定义在 [`web/src/contracts/schemas.ts`](../web/src/contracts/schemas.ts)。
- 后端线上类型在 [`internal/contract`](../internal/contract)；核心转换归 `internal/wire`，可选业务归 `internal/pluginwire`，会话出口共用 `pluginwire.Projection`。
- 两边的字段保持一致，`internal/api/testdata/wire` 的快照测试固定了每个端点和帧的 JSON 形状。

## 认证与 Origin

- 启用 token 时，HTTP 请求使用 `Authorization: Bearer <token>`；`/api/` 与 `/ws/` 不接受 Cookie 认证。
- WebSocket 握手不能设置请求头，因此：
  - 前端改为同时提供子协议 `webmux` 与 `webmux.token.<base64url(UTF-8 token)>`（无填充），服务端只选择 `webmux`、不回显 token。
  - 不接受 query `?token=`，token 不进入请求 URL。
  - 不使用 Cookie：Cookie 不按端口隔离，且浏览器会自动携带，服务端不接受 `webmux_token` Cookie。

- token 以定长摘要做常量时间比较。
- 带 `Origin` 的请求必须与 `Host` 同源，或在 `allowed_origins` 中。Origin 必须是完整的 HTTP(S) origin，不能含路径、query 或 fragment；`allowed_origins` 条目另须全小写，不带末尾斜杠与默认端口（`:80`、`:443`），否则启动失败。
- 静态页面公开；所有 API 与 WebSocket 都受认证和 Origin 检查保护。认证失败返回 401，Origin 拒绝返回 403。
- 端口转发网关 `/forward/{port}/` 是唯一使用 Cookie 的路径：`webmux_forward` 是独立于主 Token 的转发凭据，只能访问网关，见 [端口转发](protocol/http-forward.md)。
- 错误响应为 JSON `{"error": "..."}`。

## 能力发现

`GET /api/capabilities` 返回 `{version: 1, plugins: [...], capabilities: [...]}`，在 TCP、本机套接字与已建立 P2P RPC 上提供，同样遵循各传输认证策略。
`/api/health` 和 `webmux status` 说明实例是否健康、会话数与连接入口，不是插件发现接口。CLI、托盘和 MCP 用当前实例能力决定入口；不能靠端点试错或磁盘配置猜测。

- `plugins` 固定列出 14 个 ID；每项含 `id`、`enabled`、`status`（`disabled` / `ready` / `unavailable`）及可选 `reason`。
- `capabilities` 每项含 `id`、`available` 和可选 `reason`；核心 `terminal.pty` 始终可用，组合能力含 `workspace.git-workbench`。
- `enabled` 是启动选择；`ready` 不等于业务开关打开、外部命令存在或每个 agent 连接器可用。具体入口还须读取对应业务状态。
- 原因码仅为 `plugin_disabled`、`configuration_off`、`unsupported`、`storage_unavailable`、`command_unavailable`、`service_unavailable`，不包含命令、凭据、路径或底层错误正文。
- 浏览器在发现成功前不初始化可选功能；禁用的 REST/WS 返回 404，认证和 Origin 策略不因此放宽。已选择但存储故障仍按主题接口返回不可用错误。

完整选择与依赖见 [内置插件选择](deployment/configuration.md#内置插件选择)。`mcp` 被选中后仍需 `[mcp].enabled=true` 与 owner Token；精确 `/mcp` 仅 TCP，不经本机或 P2P 暴露。

## 文档地图

| 主题                                                               | 内容                                                                                        |
| ------------------------------------------------------------------ | ------------------------------------------------------------------------------------------- |
| [HTTP API：会话、启动配置与其他](protocol/http-sessions.md)        | 会话信息、按状态列出、输入与中断、结束与删除记录、冲突、上传与启动配置                      |
| [HTTP API：创建会话](protocol/http-session-create.md)              | 位置、命令、视图、首条提示、新 worktree、分步失败、创建前的上传与选项                       |
| [HTTP 与 WebSocket：聊天视图与混合会话](protocol/http-hybrid.md)   | 视图与 agent 字段、切换、恢复、已归档会话、聊天通道与聊天命令                               |
| [HTTP API：仓库与代码工作区](protocol/http-workspace.md)           | 仓库与 worktree、只读代码、Diff、图片与下载                                                 |
| [HTTP API：Git 工作台与仓库级接口](protocol/http-git-workbench.md) | 零会话仓库/工作树 Git 只读、整文件/逐块/逐行暂存、丢弃更改、贮藏、分叉冲突与 Smart Checkout |
| [HTTP API：端口转发](protocol/http-forward.md)                     | 转发管理接口、`/forward/{port}/` 网关、转发凭据与兼容改写                                   |
| [HTTP API：公网隧道设置](protocol/http-tunnel.md)                  | `/api/tunnel` 系列、持久化与错误语义                                                        |
| [HTTP API：Tailscale 状态与设置](protocol/http-tailscale.md)       | `/api/tailscale` 检测、运行状态与网页设置                                                   |
| [HTTP API：语音识别设置](protocol/http-voice.md)                   | `/api/voice/settings` 识别命令与环境变量的网页设置                                          |
| [HTTP API：注意力推送](protocol/http-push.md)                      | Web Push 订阅、来源资格与推送负载                                                           |
| [HTTP API：交接预览](protocol/http-handoff.md)                     | 冻结来源、动态预算、手工编辑与 revision                                                     |
| [HTTP API：持久对话历史](protocol/http-conversation.md)            | Activity 分页、UTF-8 偏移、完整性与导出边界                                                 |
| [服务端偏好](protocol/settings.md)                                 | `/api/settings`、`/ws/settings`、修订号与设备边界                                           |
| [WebSocket 接口](protocol/websocket.md)                            | 会话列表事件、终端帧、回放、提交、控制端与关闭码                                            |
| [本机 API](protocol/local-api.md)                                  | Unix 套接字认证与 `/local/v1/*` 控制面                                                      |
| [控制面 JSON 契约](protocol/control-json.md)                       | `version`、`status`、`stop`、`config` 与 `update check` 的 JSON 输出                        |

修改线上字段时，同时更新 [`web/src/contracts/schemas.ts`](../web/src/contracts/schemas.ts) 与 [`internal/contract`](../internal/contract)；每个子页只描述自己的主题，索引保持短小。
