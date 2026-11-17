# HTTP API：原生历史读取

通过运行时或归档会话的精确原生身份，只读取得 Agent 历史并冻结记录范围。
这些入口使用现有 Bearer 认证、Origin 校验和 `chat` 功能选择。

## 冻结范围与分页

```http
GET /api/sessions/{id}/native-history
GET /api/sessions/{id}/native-history?snapshotId=<scope>&afterPosition=20
```

首次读取冻结当前来源，返回 snapshotId、expiresAt、records、nextPosition、nextTextOffset 与 more。
后续请求携带同一 snapshotId；afterPosition 从 0 开始，limit 缺省 20，上限 100。
maxBytes 缺省 4000，上限 65536。游标使用十进制字符串。
返回记录带原生 ID、父关系、工具调用关系、hash、角色、类型与原文。

## 读取长项余文

长项续读使用返回的 nextPosition 和 nextTextOffset；偏移位于 UTF-8 字节边界。
itemId 可指定同一冻结范围中的单条记录，textOffset 指定其正文偏移。
元数据与运行记录沿用原生读取器的可导出内容分类。

## 原生来源与投影

```http
POST /api/native-history
```

action=list 使用 agent 和 root 发现来源；action=freeze 使用 sessionId 创建短期范围。
omp 的 list 仅读取有界标题和 session 头，分支在 freeze 校验正文时确定。
无关会话的不兼容正文不影响目标；同一原生 ID 出现在多个候选文件时仍返回 409。
action=read 使用 snapshotId、afterPosition 与 textOffset 分页。
action=project 使用 snapshotId、currentTask 与可选 outputRunes 生成机械文本。
action=render 使用同一范围生成显式开发渲染结果；生产默认图片交接使用[交接预览接口](http-handoff.md)。
预览缓存有效五分钟、最多 32 项，manifest、预览文本与图片表示共同计入 64 MiB 上限。

## 来源校验与错误

冻结范围固定 Agent、原生 session、分支、cursor、解析版本和内容 hash。
范围后的追加保持原快照；范围内替换、删除和身份变化返回来源错误。
格式或请求错误返回 400，内容及身份变化返回 409，过期范围返回 410。
已找到 Codex 目标但 writer 版本不支持时返回 400，保留版本解析原因；当前支持的实录版本包含 0.169.0，旧版本夹具继续保留。
agy 使用数据库 `cascade_id` 匹配 ACP session，`trajectory_id` 与 cascade 一起绑定冻结范围；两个 ID 可以不同。
omp 的 `bashExecution` 保留为图节点，缺口标记 `local_execution_omitted`；工具消息导出参数、结果及原生调用关联，未知消息角色返回 400。
旧 `/api/conversations/*` 入口返回 410。
