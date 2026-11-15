# Codex 原生历史导入验证

开发者可通过已有认证 API 读取 Codex 原生文件、导入历史、冻结快照并生成机械投影。
这些操作不启动 Codex、不执行历史命令、不修改来源文件，也不向模型发送内容。
这是固定格式的开发验证入口，没有新增聊天或 fork UI。

## 固定格式与来源

首版解析目标为 Codex `rust-v0.77.0`，提交 `112f40e91c12af0f7146d7e03f20283516a8af0b`。
参考 [RolloutItem 和 metadata schema](https://github.com/openai/codex/blob/112f40e91c12af0f7146d7e03f20283516a8af0b/codex-rs/protocol/src/protocol.rs)、
[ResponseItem](https://github.com/openai/codex/blob/112f40e91c12af0f7146d7e03f20283516a8af0b/codex-rs/protocol/src/models.rs)、
[持久化策略](https://github.com/openai/codex/blob/112f40e91c12af0f7146d7e03f20283516a8af0b/codex-rs/core/src/rollout/policy.rs)
及 [resume/fork 重建](https://github.com/openai/codex/blob/112f40e91c12af0f7146d7e03f20283516a8af0b/codex-rs/core/src/codex.rs)。
当前上游 `ee344ca9458351a6ea102795fa73110ad3ede5dc` 已有 `history_base` 引用，不能当作旧版独立文件。
版本不符或出现引用式 lineage 时拒绝解析；不承诺所有 Codex 历史版本兼容。

- 首条 `session_meta` 提供原生 ID、cwd、时间、CLI 版本和 model provider。
- 模型来自 `turn_context.model`；turn context 是上下文边界，不能证明一次执行已经完成。
- `response_item.message` 提供 user/assistant 正文；`event_msg` 的消息是镜像，不重复导入。
- 消息 `id` 可缺省；工具 `call_id` 关联调用和结果，不代表所有记录都有 entry ID。
- 内部身份绑定来源 generation、字节位置、完整行 SHA256 及子项位置，相同正文的不同执行不会合并。
- `compacted` 保存压缩边界；`replacement_history` 是模型上下文替换，不能再当成新增聊天导入。
- 该版本 resume 继续已有记录；fork 持久化选定前缀，不追读父会话当前状态。
- reasoning、encrypted content 和非 user/assistant 指令正文不导出；未知记录标记诊断；图片保留 SHA256 引用，不保存内嵌内容或原 URL，并标记正文缺口。

## 验证入口

`POST /api/conversations/native` 使用现有 Bearer 认证和 Origin 校验，JSON 请求上限 64 KiB。
下列请求依次发送；返回 ID 填入下一步。服务必须启用 `history_capture`。

```json
{"action":"list","root":"/example/codex-home"}
{"action":"import","path":"/example/codex-home/sessions/rollout.jsonl"}
{"action":"freeze","sourceId":"上一步 sourceId"}
{"action":"read","snapshotId":"上一步 snapshotId","afterPosition":"0"}
{"action":"project","snapshotId":"快照 ID","currentTask":"原样保留的新任务","outputRunes":2000}
{"action":"render","snapshotId":"快照 ID","currentTask":"原样保留的新任务"}
```

`list` 按显式 root、`CODEX_HOME`、`~/.codex` 的顺序确定数据根，扫描 sessions 和 archived_sessions。
每个文件仅读取首条元信息，上限 10000 个文件；不会为了列表读取全部正文。
`read` 每页最多 20 项，每项首段 4000 字节；后续正文沿用 conversation Activity 精确读取入口。
`project` 返回来源范围和省略范围；`render` 返回 base64 PNG、投影范围、缺字数量、未覆盖范围和问题标记。
`problem` 非空、缺字或未覆盖范围不为空时，不能把结果当成完整可读历史。
预览同时返回快照 gaps；图片只覆盖可导出投影，无法补回缺口或未导入的图片正文。
函数参数由 Project 编码为 canonical JSON，arguments 范围指向该字段；原始参数字符串仍可从 Activity 精确读取。
PNG 成功不证明模型可读、传输支持或节省 token；本入口不执行付费模型测试。

## 存储与边界

解析每条记录上限 4 MiB，单次来源上限 64 MiB、100000 条记录；只消费带换行的完整记录。
每个消息或结构化输出最多 4096 个内容片段、64 个图片引用；元信息和工具身份字段也有独立长度上限。
半行末尾留待续写；中间完整行损坏则整次导入失败，游标不推进。
来源有独立 generation、已确认字节偏移、前缀 SHA256、文件身份、原生时间、模型与解析器版本。
Linux/macOS 同时验证 device/inode；读取后重读捕获前缀，并核对路径仍指向同一文件。
前缀改变、截断、文件替换或解析器版本变化建立独立 generation。
历史 Activity、来源映射和游标同事务提交；失败重试不会提前确认消费。

快照绑定不可变 Activity 集合、截止偏移、前缀校验、解析器版本和当时的 gap 集合。
源文件追加、删除不影响已存快照；压缩配置与快照身份分开，同一快照可反复生成投影。
使用独立模块 `github.com/mzdluo123/snapcompact-go v0.1.0`；未复制 #641 的渲染实现。
快照跟随所属 Conversation 删除，不改变现有 forget 的级联删除语义。
服务提供幂等 retain/release 引用；存在保留引用时显式删除返回冲突，释放后仍需显式清理。
尚未实现完整 fork 生命周期；导入历史没有 runtime binding，可用 `delete` action 和 sourceId 显式删除，原生文件保留。

## 当前限制与验证

合成 fixture 在 `internal/codexrollout/testdata/basic.jsonl`，不来自真实用户会话。
测试覆盖镜像去重、Unicode、多行、隐私排除、未知记录、版本拒绝、资源上限和半行。
导入测试覆盖重复、提交失败、重启恢复、追加、源文件消失和旧快照稳定。
当前文件读取使用有界捕获和前缀复核；每次重导入仍扫描文件，不是大文件尾部读取优化。
工具执行状态缺省保持 unknown；该版本持久化工具输出不会保留 success 标记，不能从普通字符串推断成功。
支持结构化工具文本及图片引用；图片内容不导入，缺少工具结果在快照中明确标记。
ACP 历史保留独立身份并标记 `native_acp_unreconciled`，没有文本去重或自动合并。
预算规划仍由 snapcompact 核心提供，本入口不自动选择模型或执行 Plan。
以上边界应在扩大支持版本或接入真实交接之前继续验证，不代表 #640 整体路线完成。
