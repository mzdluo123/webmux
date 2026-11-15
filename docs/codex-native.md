# 原生历史直读与快照

开发者可通过认证 API 发现原生会话、冻结来源并读取原文或生成机械投影，不需要启动原 Agent。
先用 `list` 取得准确原生身份，再用 `freeze` 创建引用快照；需要脱离来源长期保存时显式设置 `retain=true`。
这些操作不执行历史命令、不修改原生存储，也不自动向模型发送内容。

## 最短验证路径

`POST /api/conversations/native` 沿用 Bearer 认证和 Origin 校验，JSON 请求上限 64 KiB。
服务需启用 `chat` 及 `history_capture`；功能选择改变后重启。

```json
{"action":"list","agent":"codex","root":"/example/codex-home"}
{"action":"freeze","agent":"codex","path":"/example/codex-home/sessions/rollout.jsonl","nativeId":"准确原生 session ID","owner":"my-reader"}
{"action":"read","snapshotId":"上一步 snapshotId","afterPosition":"0","textOffset":"0"}
{"action":"project","snapshotId":"快照 ID","currentTask":"原样保留的新任务","outputRunes":2000}
{"action":"render","snapshotId":"快照 ID","currentTask":"原样保留的新任务"}
```

需要把外部会话登记为可浏览历史时使用同一来源字段：

```json
{
  "action": "import",
  "agent": "codex",
  "path": "/example/codex-home/sessions/rollout.jsonl",
  "nativeId": "准确原生 session ID",
  "retain": false
}
```

- `freeze` 直接解析原生来源并保存 manifest，不先把正文导入 Activity。
- `read` 返回 `records`、`nextPosition`、`nextTextOffset` 和 `more`；后续请求同时使用两个返回游标。
- 页位置和正文偏移是十进制字符串；`boundary` 是 reader 私有的不透明来源边界，不能当作页位置计算。
- 每项保留规范 ID、原生 entry ID、原生调用 ID、工具关联、状态、原生偏移及内容版本 hash；相同正文不是去重依据。
- `project` 返回来源范围与明确省略；`render` 另返回 PNG、缺字、未覆盖范围和问题标记，不代表模型已经收到内容。
- `import` 冻结精确来源并建立仅含元数据的历史 Conversation / ProviderThread，返回两者 ID 及快照；不写 Activity 正文、不启动 Agent。`delete` 不提供，也不删除 Agent 原生文件。

## 读取器与版本证据

以下叶读取器已实现并通过脱敏原生夹具验证；这不等于全部 CLI/连接器版本、TUI/ACP 交接组合已完成实测。
来源、缺失场景及脱敏方式见[真实夹具说明](native-history-fixtures.md)。

| Agent       | 原生存储证据                                                 | 版本边界                                |
| ----------- | ------------------------------------------------------------ | --------------------------------------- |
| Codex       | rollout JSONL、消息及工具闭环、compacted replacement history | 0.77.0、0.157.1、0.162.1                |
| Claude Code | projects JSONL、parent 图及 compact_boundary                 | 真实样本 2.1.316                        |
| omp         | session JSONL、父链与 compaction 保留范围                    | 文件格式 3；一份 runtime 18.12.2        |
| OpenCode    | 两代 SQLite 原生表、消息及 part                              | 会话版本 1.18.3 / 2.1.4                 |
| Kimi        | state.json 与 wire.jsonl                                     | wire 1.4；应用版本未知                  |
| agy         | SQLite BLOB、已恢复 descriptor 的 protobuf 字段              | 应用版本未知，仅六步实录证据            |
| Devin       | SQLite committed 主链，排除 inference siblings               | SQLite 写入版本未知；不与独立 ATIF 混合 |
| QoderCN     | projects JSONL，按活动祖先链恢复                             | 实录 1.1.84；样本无工具调用             |
| Qoder       | 暂复用 QoderCN 读取器和 fixtureSource                        | 国际版路径/格式尚未经实录确认           |

原生格式、CLI、连接器和历史注入协议是独立版本维度，不能互相替代。
未知格式、引用式 lineage、无法证明的分支或原生身份返回明确错误，不猜测会话归属。
SQLite 读取在只读事务内完成；agy 生产读取 BLOB，不把 decoded JSON 研究文件冒充原生来源。

## 引用与保留快照

默认引用快照固定 Agent、native session、分支、完整记录边界、来源校验、解析器版本和冻结缺口。
文件追加不进入旧范围；来源缺失返回 503，冻结前缀、身份或解析结果变化返回 409，不用最新历史替换旧快照。
JSONL 只确认换行结束的完整记录；半行留待下一次冻结，冻结后出现的损坏后缀也不进入旧范围。
Generation 标识原生来源实例，branch 固定所选祖先链；记录 hash 标识内容版本，快照 hash 固定整个 manifest。
同一路径、相同正文或相同 native ID 不足以证明来源实例、分支和内容未变；不能混用父会话与 fork 的冻结游标。

设置 `retain=true` 时按需固化一次完整可导出正文及可保留附件，作为不可变内容寻址产物。
它没有实时追加或 Activity upsert，多个引用共享同一产物；来源删除与服务重启后仍可读取。
无法取得所引用附件时保留失败；原生已省略的附件正文只保留明确缺口，不伪造字节或阻塞纯文本导出。
`retain` action 增加已有快照的生命周期引用，不把引用快照升级成独立备份；需要备份应在冻结时选择保留模式。
`release` 释放指定 owner，最后引用及组合引用释放后清理快照和相关导出；未决投递保护不能借来源删除或释放解除。

```json
{"action":"freeze","agent":"codex","path":"/example/rollout.jsonl","nativeId":"准确原生 ID","owner":"independent-fork","retain":true}
{"action":"retain","snapshotId":"快照 ID","owner":"second-reader"}
{"action":"release","snapshotId":"快照 ID","owner":"second-reader"}
```

## 运行身份与目录

运行注册表与只读读取器由 cmd 分别显式装配，读取器不要求 ACP、信任写入或 HistoryInjector 能力。
创建/恢复按实际启动环境计算原生根目录，只保存目录路径，不保存环境转储。
HOME、CODEX_HOME、CLAUDE_CONFIG_DIR、KIMI_CODE_HOME 和适用的 XDG_DATA_HOME 覆盖按对应 Agent 规则解析。
原生身份相同但根目录改变时建立独立 ProviderThread；后续编辑启动配置不会改写旧历史绑定，unknown 不允许换根恢复。
需要显式指定来源时使用 `bind`，只有 reader 和当前 ProviderThread 的精确 native 身份都一致才保存绑定。
发现结果不唯一或原生身份缺失时拒绝自动配对，不使用 cwd、标题或相同正文推断身份。

```json
{
  "action": "bind",
  "agent": "codex",
  "providerThreadId": "ProviderThread ID",
  "nativeId": "准确原生 ID",
  "path": "/example/rollout.jsonl"
}
```

## 资源、隐私与 Codex 格式细节

- JSONL 单记录最多 4 MiB、单次来源最多 64 MiB / 100000 条，读取传递取消；阻塞内核文件操作不能由上下文保证立即中断。
- Linux/macOS 校验 device/inode、只读捕获和前缀复核；数据库来源校验其原生冻结边界与内容，不创建或迁移 Agent 数据库。
- 页面默认 20 项、总文字预算 4000 字节，UTF-8 字节边界继续读取，不截断 code point；原文不受投影截断影响。
- 原生未记录、已删除或已压缩丢弃的内容无法恢复；未知记录、外部祖先、缺失工具结果、附件和压缩缺口明确表示。
- reasoning、密文、签名、非可导出 provider 状态不进入正文；解析错误只返回安全类别及可用位置，不回显失败记录。
- 当前仍以有界完整捕获解析来源，不宣称实现大文件尾读优化；工具状态缺省为 unknown，不能由普通字符串推断成功。

Codex 0.77 基线固定上游 `112f40e91c12af0f7146d7e03f20283516a8af0b`：
[RolloutItem](https://github.com/openai/codex/blob/112f40e91c12af0f7146d7e03f20283516a8af0b/codex-rs/protocol/src/protocol.rs)、
[ResponseItem](https://github.com/openai/codex/blob/112f40e91c12af0f7146d7e03f20283516a8af0b/codex-rs/protocol/src/models.rs)、
[持久化策略](https://github.com/openai/codex/blob/112f40e91c12af0f7146d7e03f20283516a8af0b/codex-rs/core/src/rollout/policy.rs)、
[resume/fork](https://github.com/openai/codex/blob/112f40e91c12af0f7146d7e03f20283516a8af0b/codex-rs/core/src/codex.rs)。
`response_item` 是正文来源，`event_msg` 镜像不重复计入；配对证据不足保留 `mirror_pairing_unverified`。
`replacement_history` 改变模型上下文，不删除 rollout 中仍存在的旧原文；重复的原生 ID 按来源版本处理，不按文本合并执行。
独立物化 fork 可读取；较新的 `history_base` 等引用 lineage 尚不冒充旧版独立文件支持。
Codex 发现保留原有独立预算：元信息 64 KiB、累计 16 MiB、最多 20000 次遍历 / 10000 个结果，达到列表上限返回 `truncated=true`。

## 投影、图片与当前迁移边界

仅复用 `github.com/mzdluo123/snapcompact-go v0.1.0`，不复制 #641 核心或渲染器。
投影确定性截断工具输出、限制参数/调用体并规范化空白；未知结构保守保留行和缩进，当前用户任务独立原样保存。
默认投影输入预算 64 MiB；Render 独立限制输入、页数、像素和内存，缺字或未覆盖范围不能冒充完整覆盖。
字体默认完整内嵌，不新增运行时下载；[体积预算](binary-size.md#原生历史-png-的字体预算)约束增量 20 MiB。
OFL 与依赖许可随 `/fonts/licenses/` 和 Linux `/usr/share/licenses/webmux/` 交付。

[CLI 实验](native-image-experiment.md)与[真实 ACP 实验](native-image-transport.md)记录合成样本、计费估价和实际读取错误。
当前图片没有显示相对同覆盖投影文本的成本优势，混合表示出现缩进错误，因此文本继续默认。
这些证据不等于网页上传、生产自动交接或原生图片注入已完成验收。
聊天历史、预览、选择分页和只读历史工具统一消费冻结原生 scope；机械投影复用同一管线，当前新任务独立保存，不参与历史压缩。
schema 14 在删除旧正文表前，将可导出的旧 Activity 与旧快照迁移为不可变 exports；此兼容产物不是新的持续镜像。
完整交接、浏览器与 CI 的最终集成仍须分别验收，不能把读取器或局部测试通过称为 #640/#651/#652 全部完成。
