# HTTP API：原生历史交接

交接接口使用运行时会话 ID、短期预览 ID 与操作 ID。
入口沿用 Bearer 认证、Origin 校验与 `chat` 功能选择，工作树目标同时使用 `workspace`。

## 创建与读取

```http
POST /api/sessions/{id}/handoff-previews
GET /api/handoff-previews/{previewId}
```

请求字段：`configId`、`targetSessionId`、`agentConfig`、`userText`、`location`、`format`、`maxPages`。
创建新聊天时提供 `configId`；已有目标使用 `targetSessionId`，并沿用目标配置。
`location` 包含 cwd、repositoryId、worktreeId、newWorktreeBase 与 newWorktreeBranch。
`format` 仅接受 `image`，省略时同样生成图片。旧客户端提交 `text` 返回 400。`maxPages` 省略或为 0 时采用 8 页上限，显式范围为 1–8。
返回 `id`、`instanceId`、`operationId`、`snapshotId`、`sourceSessionId`、发送文本块 `text` 与独立原样 `userText`。
同时返回 `request`、`budget`、selectedCount、omittedCount、gapCount、createdAt 和 expiresAt。selectedCount 统计图片覆盖的片段，omittedCount 统计投影及分页省略范围。
图片字段为 `format`、`pages`、`imageBudget`、`blockedReason`、`capability` 与审阅用 `historyText`：

- `pages[]` 按页序包含 `index`（从 0 开始）、`mimeType`、base64 `data`、`sha256`、`width`、`height`、原始 `bytes`。
- `imageBudget` 包含 `maxPages`、实际 `pages`、原始 `bytes`、`maxBytes` 与编码请求上限 `maxRequestBytes`。
  另含投影正文字符数 `projectedRunes`、实际保留字符数 `retainedRunes`、图片 `estimatedTokens`、`tokenEstimateKnown`、`tokenBasis` 与 `shape`。
- `capability` 为 `pending`、`supported` 或 `unsupported`；需要图片的新目标在创建前为 `pending`，发送前检查真实 ACP 图片能力。零页归档无需图片能力。
- `blockedReason` 给出图片无法发送的原因。
- `historyText` 仅供审阅。ACP 按来源指引、历史图片及当前任务的顺序发送。

## 选择与预算

Snapcompact 从可导出的正文和可见工具参数、结果生成带角色及调用关联的图片。私有推理及 provider 状态留在来源。
归档在可用上下文、页数和字节上限内规划高低密度页面，省略量随预览返回。
短历史同样生成 PNG。渲染失败、缺字、目标能力不足或请求超限会阻止发送。
每条最多 8 张、单图 10 MiB、原始数据合计 20 MiB；编码请求最多 30 MiB，并受服务端更低配置限制。
上下文预算包含当前任务、历史和工作预留，受 `handoff_token_cap` 限制。
来源指引使用 `cl100k_base` 加 15% 余量估算，图片单独估算，`tokenBasis` 标注依据。
预览冻结文本、图片字节、顺序及 SHA-256；发送使用这些内容，不重新渲染。已有目标模型配置变化使原预览失效。
预览在五分钟内有效，最多 32 项；manifest、文本及图片表示共同计入 64 MiB 内存预算。

## 确认发送与查询

```http
POST /api/handoff-previews/{previewId}/send
GET /api/handoff-operations/{operationId}
POST /api/handoff-operations/{operationId}/cancel
```

发送体为 `{ "operationId": "<预览返回的操作 ID>" }`，返回 202。
服务端在调用目标前原子消费预览并登记操作；同一操作的重复请求返回当前结果。
操作返回 id、previewId、instanceId、state、targetSessionId、worktreeId、worktreePath、error 与时间字段。
状态为 `preparing`、`sending`、`awaiting_upstream`、`succeeded`、`failed` 或 `unknown`。
目标成功创建与普通 prompt 各执行一次；操作成功表示本轮已获得接收证据，不代表回合结束或任务验收。
接收证据包括本轮 Agent 正文、思考、工具调用、plan、当前授权/提问，或成功的 `session/prompt` 响应。
30 秒 commandTimeout 只约束校验与 actor 接受派发；确认观察绑定该 turn 的共享回执，慢模型不会仅因超过 30 秒变成 unknown。
尚无活动且连接仍存活时保持 `sending`；取消确认观察或连接断开且没有接收证据时为 `unknown`，不会自动重发。
准备阶段取消为 `failed`；用户须在目标会话检查指定 turn 的最终输出。
结果不明返回「可能已发送，请检查目标会话」。操作结果最多保留 256 项，完成十五分钟后失效。

新工作树上游有更新或获取失败时，操作停在 `awaiting_upstream`，`finishedAt` 为 null，`worktreeUpstream` 提供[上游确认信息](http-workspace.md#创建前的上游确认)。此时尚未创建目标资源：

- 确认后向同一 `/send` 提交 `{operationId, upstreamChoice: {confirmationId, useUpstream}}`，继续原操作与冻结页集。
- 重复相同确认只观察已有操作；上游状态再次变化时重新进入 `awaiting_upstream`。
- 取消操作或预览过期后无法继续创建，保留来源与原工作树。

## 原文补查与错误

来源读取见[原生历史 HTTP](http-conversation.md)，目标 MCP 读取见[thread_read](handoff-mcp.md)。
400 表示请求或已撤销字段无效，409 表示身份、内容或消费冲突，410 表示句柄或协议失效。
429 表示交接缓存已满，503 表示功能或原生来源暂不可用。
`/api/conversations/*` 与 `/api/handoffs/*` 的旧协议返回 410。

## CLI 与 MCP 入口边界

交接创建、预览和发送由以上 HTTP 接口及浏览器页面提供。
`webmux mcp-serve` 仅提供 `thread_read` 冻结原文补查；控制 MCP 的 `session_read` 提供原生历史分页。
当前没有专用交接 CLI 子命令或 MCP preview/send 工具；`webmux api PATH -d '<JSON>'` 可经本机套接字调用以上 HTTP 契约。
