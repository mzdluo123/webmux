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
- `response_item.message` 提供 user/assistant 正文；`event_msg` 的消息按镜像处理，不重复导入。
  此版本不校验两份记录的一一配对，明确标记 `mirror_pairing_unverified`；只有镜像而没有正文副本时不会伪造文本或宣称完整。
- 消息 `id` 可缺省；工具 `call_id` 关联调用和结果，不代表所有记录都有 entry ID。
- 内部身份绑定来源 generation、字节位置、完整行 SHA256 及子项位置，相同正文的不同执行不会合并。
- `compacted` 保存压缩边界；`replacement_history` 是模型上下文替换，不能再当成新增聊天导入。
- 该版本 resume 继续已有记录；fork 持久化选定前缀，不追读父会话当前状态。
- reasoning、encrypted content 和非 user/assistant 指令正文不导出；未知记录标记诊断；图片保留 SHA256 引用，不保存内嵌内容或原 URL，并标记正文缺口。

## 验证入口

`POST /api/conversations/native` 使用现有 Bearer 认证和 Origin 校验，JSON 请求上限 64 KiB。
下列请求依次发送；返回 ID 填入下一步。服务必须启用 `chat` 内置功能及 `history_capture`；功能启用列表修改后需重启。

```json
{"action":"list","root":"/example/codex-home"}
{"action":"import","path":"/example/codex-home/sessions/rollout.jsonl"}
{"action":"freeze","sourceId":"上一步 sourceId"}
{"action":"read","snapshotId":"上一步 snapshotId","afterPosition":"0"}
{"action":"project","snapshotId":"快照 ID","currentTask":"原样保留的新任务","outputRunes":2000}
{"action":"render","snapshotId":"快照 ID","currentTask":"原样保留的新任务"}
```

`list` 按显式 root、`CODEX_HOME`、`~/.codex` 的顺序确定数据根，扫描 sessions 和 archived_sessions。
每个文件仅读取首条元信息，单个元信息记录最多 64 KiB、一次列表累计读取最多 16 MiB、遍历最多 20000 项。
读取预算包含实际缓冲预读，元信息使用 512 字节缓冲，避免小元信息按 4 KiB 消耗预算。
列表最多返回 10000 项；达到资源上限时保留已取得的文件并返回 `truncated=true`，单个不可读目录或文件有明确 problem 标记。
列表及元信息读取传递请求取消上下文，不会为了列表读取全部正文。
`read` 每页最多 20 项，每项首段 4000 字节；缺口和 reconcileRequired 来自冻结记录，不读取来源对话当前状态。
来源查询按相同页大小返回，每项同时携带 Activity ID 和 position，可按键关联而非依赖数组下标。
后续正文沿用 conversation Activity 精确读取入口。
`project` 返回来源范围和省略范围；`render` 返回 base64 PNG、投影范围、缺字数量、未覆盖范围和问题标记。
`problem` 非空、缺字或未覆盖范围不为空时，不能把结果当成完整可读历史。
预览同时返回快照 gaps；图片只覆盖可导出投影，无法补回缺口或未导入的图片正文。
函数参数由 Project 编码为 canonical JSON，arguments 范围指向该字段；原始参数字符串仍可从 Activity 精确读取。
默认 Project 输入上限与来源一致为 64 MiB；工具输出截断后才进入 Render。
Render 另有 8 MiB 投影输入、页数和内存限制，超限返回 problem 与可用的未覆盖信息，原始快照保持可读。
PNG 成功不证明模型可读、传输支持或节省 token；本入口不执行付费模型测试。

## 存储与边界

解析每条记录上限 4 MiB，单次来源上限 64 MiB、100000 条记录；只消费带换行的完整记录。
诊断按类别返回首个偏移；一次导入最多返回 256 种诊断，只有未报告类别存在时才增加 `diagnostics_limited` 标记。
写入缺口也按类别合并，不丢弃原生记录映射；重新完整解析可解除旧的诊断计数限制，但不会更改旧快照。
每个消息或结构化输出最多 4096 个内容片段、64 个图片引用；元信息和工具身份字段也有独立长度上限。
半行末尾留待续写；中间完整行损坏则整次导入失败，游标不推进。
来源有独立 generation、已确认字节偏移、前缀 SHA256、文件身份、原生时间、模型与解析器版本。
Linux/macOS 同时验证 device/inode；读取后重读捕获前缀，并核对路径仍指向同一文件。
前缀改变、截断、文件替换或解析器版本变化建立独立 generation。
文件重命名或归档只有在平台文件身份、原生会话 ID、解析版本及已确认内容都匹配时才复用来源；路径索引独立保存。
历史 Activity、来源映射和游标同事务提交；失败重试不会提前确认消费。

快照绑定不可变 Activity 集合、截止偏移、前缀校验、解析器版本和当时的 gap 集合。
冻结通过引用已提交且不能改写的 Activity 实现，不复制正文；当前没有逐项淘汰。未来增加正文清理时，必须保留快照引用的正文或拒绝该清理。
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

## 发行体积与许可

默认使用 Go 核心的完整 CJK 字库，保持离线、单文件中文 PNG 支持；关闭 chat 不会减少静态链接体积。
实测本次接入增加约 16.7 MiB，可执行文件预算和测量方法见[二进制体积](binary-size.md#原生历史-png-的字体预算)。
OFL、版权及依赖声明内嵌于 `/fonts/licenses/OFL-NotoSansCJK.txt` 和 `/fonts/licenses/Snapcompact-Go-NOTICES.txt`。
Linux deb/rpm 另安装到 `/usr/share/licenses/webmux/`；没有新增字体运行时下载或模型网络请求。
