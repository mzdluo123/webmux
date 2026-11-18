# 原生历史直读与快照

通过已登记会话的 runtime ID 读取 Agent 原生历史，取得短期冻结范围；交接默认生成首尾文本与中段图片的压缩归档，也可选择机械文本投影。
浏览器交接操作见[原生历史交接](hybrid-sessions/handoff.md)。

## 最短读取路径

```http
GET /api/sessions/<runtime-id>/native-history
Authorization: Bearer <token>
```

返回 snapshotId 和首批 records；后续页沿用该 snapshotId、nextPosition 与 nextTextOffset。
入口需要 `chat`，并使用已有认证和 Origin 校验。

## 读取器与版本证据

原生读取器覆盖 Codex、Claude Code、omp、OpenCode、agy、Kimi、Devin、Qoder 与 QoderCN。
每个读取器负责其来源格式、原生身份、分支及 cursor 校验，返回有界的中立记录。
OpenCode、agy 与 Devin 的 SQLite 由只读读取器访问。
格式夹具及版本记录见[原生历史夹具](native-history-fixtures.md)。

## 冻结范围与有效期

快照固定来源路径、Agent、原生 ID、分支、解析版本、记录 hash 与边界。
范围之后的追加属于后续历史；范围内修改和来源丢失产生明确错误。
快照在当前进程有效五分钟，最多 32 项，与预览文本及图片表示共用 64 MiB 预算。

## 运行身份与目录

来源使用 hybrid 记录中的 HistoryRoot 和原生 session ID。
创建与恢复进程时记录实际历史根目录；旧记录缺失目录时返回来源不可用。
服务启动保留旧会话数据库原件，升级操作见[持久化说明](persistence.md#旧会话数据库与升级)。

## 投影与图片交接

Snapcompact 对可导出用户/助手正文及可见工具调用/结果做机械投影，保留角色、来源顺序和工具关联。
结构 JSON、记录标题、私有推理和不可导出 provider 状态不替代正文；当前用户任务独立保持原样。
生产归档按模型选择高密度排版，首尾保持文本、短历史全为文本；容量不足时分级渲染中段并显式报告省略范围。
图片冻结后按相同字节发送；图片估计与文本共同计入上下文预算，字节限制单独校验。缺字、覆盖不全或能力错误阻止需要图片的发送。
`action=render` 的显式开发入口仍可查看页面、缺失字形和未覆盖范围。
历史独立实验见[图片实验](native-image-experiment.md)与[ACP 图片传输记录](native-image-transport.md)，其结果不代表新生产路径已验收。
完整字段见[原生历史 HTTP](protocol/http-conversation.md)和[交接 API](protocol/http-handoff.md)。

## 读取器与版本证据

以下叶读取器已实现并通过脱敏原生夹具验证；这不等于全部 CLI/连接器版本、TUI/ACP 交接组合已完成实测。
来源、缺失场景及脱敏方式见[真实夹具说明](native-history-fixtures.md)。

| Agent       | 原生存储证据                                                 | 已核验样本版本                                               |
| ----------- | ------------------------------------------------------------ | ------------------------------------------------------------ |
| Codex       | rollout JSONL、消息及工具闭环、compacted replacement history | 0.77.0、0.157.1、0.160.1、0.162.1、0.167.2、0.169.0、0.169.1 |
| Claude Code | projects JSONL、parent 图及 compact_boundary                 | 真实样本 2.1.316                                             |
| omp         | session JSONL、父链与 compaction 保留范围                    | 文件格式 3；runtime 18.12.2、18.22.0                         |
| OpenCode    | 两代 SQLite 原生表、消息及 part                              | 会话版本 1.18.3 / 2.1.4                                      |
| Kimi        | state.json 与 wire.jsonl，正文与工具参数/结果                | wire 1.4/1.5；1.5 实录 CLI 2.5.3                             |
| agy         | SQLite BLOB、已恢复 descriptor 的 protobuf 字段              | 旧六步实录版本未知；新增 1.5.7 两步实录                      |
| Devin       | SQLite committed 主链，排除 inference siblings               | SQLite 写入版本未知；不与独立 ATIF 混合                      |
| QoderCN     | projects JSONL，按活动祖先链恢复正文与工具参数/结果          | 实录 1.1.81、1.1.84                                          |
| Qoder       | 暂复用 QoderCN 读取器和 fixtureSource                        | 国际版路径/格式尚未经实录确认                                |

原生历史读取器接受最低基线及以上版本，并按原生记录结构校验内容。版本记录取自来源文件，安装版本不参与原生历史判定。
涉及上游已记录的存储格式变化时，以来源的实际格式选择解析路径。OpenCode 在同一数据库内按目标会话所在的旧版或 v2 表读取。

- Codex 独立 rollout 的 `session_meta.cli_version` 从 0.77.0 起。
- Kimi 的 Wire 协议从 1.4 起，`state.json` 声明的格式版本从 2 起。
- omp 的 session 格式版本从 3 起，详见[上游会话格式迁移](https://github.com/can1357/oh-my-pi/blob/main/docs/session.md#versioning-and-migration)。
- Qoder 和 QoderCN 共用 JSONL 读取器，写入版本从 1.1.81 起。
  0.160.1、0.167.2 和 0.169.1 的独立 rollout 已通过[本机只读核验](user-stories/verification-log-2026-11-18.md#codex-原生历史版本兼容us-q19us-q20)。
  原生格式、CLI、连接器和历史注入协议是独立版本维度，不能互相替代。
  未知格式、引用式 lineage、无法证明的分支或原生身份返回明确错误，不猜测会话归属。
  SQLite 读取在只读事务内完成；agy 生产读取 BLOB，不把 decoded JSON 研究文件冒充原生来源。

2026-11-17 当前部署的合成历史现场测试另发现来源兼容性问题：
omp 的根目录发现被无关旧格式阻断，Kimi wire 1.5 和 QoderCN 1.1.81 被版本限制拒绝，Codex 0.169.0 的版本失败被表现为身份冲突，agy 的 cascade/trajectory 身份不一致。
这些问题按独立修复处理，本轮图片投影变更不声明修复；连接器缺失与模型额度限制也不等于图片能力结论。
现场结果与当前图片路径的验证边界见[2026-11-17 核对记录](user-stories/verification-log-2026-11-17.md#默认图片交接与现场边界us-q19us-q20us-k04)。

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
0.169.0 capture（ACP 2.6.0 环境）保留 `response_item` 正文；`item_completed` 镜像、reasoning、工具定义与 provider 状态不重复导入。
`content_item_kinds` 明确标记的纯环境上下文不作为用户正文；未知或混合 provenance 的用户消息仍保留。
本次合成 capture 未调用工具；工具关联回归使用旧实录 envelope 补充，不视为当前版本工具实测证据。
`replacement_history` 改变模型上下文，不删除 rollout 中仍存在的旧原文；重复的原生 ID 按来源版本处理，不按文本合并执行。
独立物化 fork 可读取；较新的 `history_base` 等引用 lineage 尚不冒充旧版独立文件支持。
Codex 发现保留原有独立预算：元信息 64 KiB、累计 16 MiB、最多 20000 次遍历 / 10000 个结果，达到列表上限返回 `truncated=true`。
发现按 `session_meta` 内容确认身份。目标低于写入版本基线、版本字段无效或依赖引用式历史时，读取返回对应的格式错误。
同一原生 ID 出现多个候选时返回身份冲突。原生 ID 以记录内容为准。

<a id="投影图片与当前迁移边界"></a>

## 投影与图片

生产默认图片及显式文本共用独立 Snapcompact 核心；格式和预算见[交接 API](protocol/http-handoff.md)。
保留的[图片实验](native-image-experiment.md)与[ACP 图片传输](native-image-transport.md)只证明各自注明的模型、版本和样本，不代表全部 reader 或跨 Agent 配对已验收。
