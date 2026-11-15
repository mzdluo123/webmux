# HTTP API：持久对话历史

使用 Conversation ID 直读 Agent 原生历史或已保留的不可变导出，不需要启动或恢复 Agent。
`activities` 是兼容的规范化响应名称，不表示服务端持续保存 Activity 正文；实时聊天继续使用 WebSocket 缓存。
本页需要选择 `chat`（依赖 `launch` + `files`）且开启历史读取；禁用功能时返回 404，不打开或迁移数据库。停用不删除旧正文，见 [持久化清单](../persistence.md#功能选择与数据保留)。

## 冻结范围与分页

```http
GET /api/conversations/{id}/activities?afterPosition=0&limit=20&maxBytesPerItem=4000
Authorization: Bearer <token>
```

- `afterPosition` 缺省为 `0`，不含当前位置；`limit` 缺省 20，范围 1–100。
- `maxBytesPerItem` 缺省 4000，范围 4–65536；每次最多返回 limit 项，每项按此字节上限分页。
- 返回 `items`、`snapshotIds`、`nextPosition`、`gaps`、`reconcileRequired` 和 `historyOrigin`。
- 首次未指定 scope 时冻结当前来源；后续页与长项请求必须带返回的 `snapshotIds`（逗号分隔），否则可能冻结新范围。
- 也可用 `handoffId` 读取该预览的完整冻结范围，包括预算省略项；不能同时提供 `snapshotIds`。
- scope 在读取正文前校验 Conversation 归属。position 是组合冻结记录的页序，不是跨快照的追加水位；新范围须从 0 读取。
- 无结果时 `items=[]`，`nextPosition` 保持请求游标；原生追加不改变已有快照的范围或页序。

## 读取长项余文

```http
GET /api/conversations/{id}/activities/{activityId}?textOffset=0&maxBytes=4000
```

`textOffset` 缺省 0，`maxBytes` 缺省 4000，范围 4–65536。
偏移按 UTF-8 **字节**计算；返回不切断 code point，`nextTextOffset` 是下一段的起始偏移。
偏移处于多字节字符内部或超出原文时返回 400。只允许读取该 Conversation 在指定冻结 scope 内的记录。

每项沿用 `activityId/conversationId/providerThreadId/runId`、`position/runOrdinal` 和 `kind/role/status/text/source/providerItemId/createdAt` 响应结构，并携带来源内容 hash。
原生记录不伪造运行时 Run 归属；稳定身份来自 reader，而非正文相等或浏览器缓存。
`byteLength` 表示完整规范化文本的字节长度，`nextTextOffset` 和 `truncated` 描述本次读取；附件仅返回元数据与来源引用。

## 完整性与导出

- `truncated=true` 仅表示本页未返回全部文字，不表示来源正文缺失。
- `historyOrigin` 保留 `unknown|bounded_import|complete` 契约；原生读取的缺失范围由 `gaps` 明示，不能仅凭 complete 推断没有压缩或外部祖先缺口。
- `gaps` 和 `reconcileRequired` 表示原生缺口或无法确认的来源，不与分页截断混淆；旧升级记录不伪造完整历史。
- `exportable` 和 `exclusionReason` 明确标记 portable projection；生命周期和权限控制项不可导出为用户指令。
- 消息保留角色；工具关联、错误、中断及 compaction 按 reader 证据表示。reasoning、密文、签名及私有 provider 状态不导出。
- 持久 position、ordinal、字节长度和偏移都使用十进制字符串，避免 JavaScript 浮点数损失精度；它们不是实时 chat generation/seq。

## 校验与错误

游标必须是规范的非负十进制 int64 字符串；不接受符号、前导零、指数、溢出、重复参数或未知参数。

| 状态      | 含义                                                              |
| --------- | ----------------------------------------------------------------- |
| 400       | ID、游标、字节边界或分页上限无效                                  |
| 401 / 403 | 缺少认证或 Origin 被拒绝，沿用统一 API 边界                       |
| 404       | Conversation / Activity 不存在，或 Activity 不属于该 Conversation |
| 409       | 冻结内容、来源身份或 scope 已变化                                |
| 503       | 对话数据库未启用或不可用                                          |

读取限时 10 秒，响应使用 `Cache-Control: no-store`，正文不写访问日志。删除记录或淘汰会清理关联数据，释放/重建 worktree 不隐式删除；备份与存储边界见[配置与持久化清单](../persistence.md)。

## 原生来源与不可变快照

`POST /api/conversations/native` 沿用现有认证边界。
支持 list、bind、import、freeze、read、project、render、retain 和 release；不提供删除 Agent 原生文件的 action。
`import` 创建仅含身份、绑定与冻结引用的历史 Conversation / ProviderThread，返回两者 ID；不启动 Agent 或建立正文镜像。
`freeze` 的 agent、path、nativeId 和可选 branch 由只读 reader 验证；默认保存引用 manifest，显式 retain=true 才固化一次导出。
`read` 返回 records 及原生 entry/调用身份、来源偏移、内容 hash；Generation、分支与冻结 hash 共同约束来源，boundary 不可用作页序计算。
引用来源丢失返回 503、冻结内容或身份变化返回 409；保留导出按独立引用存活，不以路径引用冒充备份。
project/render 返回冻结缺口、投影来源和省略范围，图片另有缺字和未覆盖范围；具体格式、预算及迁移现状见[原生历史说明](../codex-native.md)。
history_capture=false 时关闭读取、冻结及投影，不控制 Agent 自己写入原生历史；未决投递保护保留。
