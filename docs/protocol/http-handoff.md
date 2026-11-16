# HTTP API：原生历史交接

交接接口使用运行时会话 ID、短期预览 ID 与操作 ID。
入口沿用 Bearer 认证、Origin 校验与 `chat` 功能选择，工作树目标同时使用 `workspace`。

## 创建与读取

```http
POST /api/sessions/{id}/handoff-previews
GET /api/handoff-previews/{previewId}
```

请求字段：`configId`、`targetSessionId`、`agentConfig`、`userText`、`location`。
创建新聊天时提供 `configId`；已有目标使用 `targetSessionId`，并沿用目标配置。
`location` 包含 cwd、repositoryId、worktreeId、newWorktreeBase 与 newWorktreeBranch。
返回 `id`、`instanceId`、`operationId`、`snapshotId`、`sourceSessionId`、确切发送 `text` 与独立 `userText`。
同时返回 `request`、`budget`、selectedCount、omittedCount、gapCount、createdAt 和 expiresAt。

## 选择与预算

Snapcompact 将可导出的原生记录转换成机械文本片段，按预算选入最近的完整片段并恢复来源顺序。
预算包括当前任务、历史文本、模型窗口与保留空间。默认交接上限由 `handoff_token_cap` 控制。
预览在五分钟内有效，最多 32 项；文本与 manifest 共用 64 MiB 预算。

## 确认发送与查询

```http
POST /api/handoff-previews/{previewId}/send
GET /api/handoff-operations/{operationId}
POST /api/handoff-operations/{operationId}/cancel
```

发送体为 `{ "operationId": "<预览返回的操作 ID>" }`，返回 202。
服务端在调用目标前原子消费预览并登记操作；同一操作的重复请求返回当前结果。
操作返回 id、previewId、instanceId、state、targetSessionId、worktreeId、worktreePath、error 与时间字段。
状态为 `preparing`、`sending`、`succeeded`、`failed` 或 `unknown`。
目标创建和普通 prompt 各执行一次，prompt 的已确认响应决定成功状态。
结果不明返回「可能已发送，请检查目标会话」。操作结果最多保留 256 项，完成十五分钟后失效。

## 原文补查与错误

来源读取见[原生历史 HTTP](http-conversation.md)，目标 MCP 读取见[thread_read](handoff-mcp.md)。
400 表示请求或已撤销字段无效，409 表示身份、内容或消费冲突，410 表示句柄或协议失效。
429 表示交接缓存已满，503 表示功能或原生来源暂不可用。
`/api/conversations/*` 与 `/api/handoffs/*` 的旧协议返回 410。
