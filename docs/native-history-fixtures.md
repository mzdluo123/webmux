# 原生 Agent 历史夹具

这些夹具供 [#652](https://github.com/mzdluo123/webmux/issues/652) 的读取器、快照与交接开发使用，无需安装或登录对应 Agent。
夹具包含维护者电脑上已有会话的脱敏节选，以及三份本机合成会话的脱敏实录；原生存储结构与实际验收范围分别记录。

## 从夹具开始开发

1. 打开 [夹具目录](../internal/agent/testdata/native-history/)，先读目标 Agent 的 `manifest.json`。
2. 按 `sourceLayout` 理解原生目录与身份关系，按 `files` 选择 JSON、JSONL 或 SQLite SQL 样本。
3. 在临时目录重建原生布局，针对实际字段、类型和引用开发只读适配器，不启动 Agent 或连接真实服务。
4. 使用 `coverage` 和 `limitations` 区分已观察行为、被脱敏信息及尚缺场景；版本兼容范围以对应版本的夹具结果为依据。

每份 manifest 记录来源形态、版本证据、脱敏方式、实际覆盖、限制以及发布文件的 SHA-256。
文件哈希只校验脱敏产物，不是私人源文件哈希，也不能当作 Agent 原生内容校验。

## 原生格式与覆盖

早期采集覆盖 Linux 用户目录与可访问的 Windows 用户目录，当时未启动 Agent 制造新会话。
2026-11-17 补充 Codex、omp、agy 三份本机合成会话实录，各 manifest 的 `currentCapture` 记录版本证据、脱敏方式与覆盖限制。
国际版 Qoder 未找到本地真实历史；维护者已明确决定暂按 QoderCN 的格式与夹具开发，不因此阻塞读取器实现。
Qoder 的 `fixtureSource` 指向 QoderCN manifest，共用文件与哈希，不复制或重标来源；国际版兼容性仍待实录确认。

| Agent       | 原生样本                                      | 已观察范围与限制                                                                                         |
| ----------- | --------------------------------------------- | -------------------------------------------------------------------------------------------------------- |
| Codex       | JSONL；CLI 0.162.1、0.157.1、0.169.0          | 旧实录覆盖工具闭环、compacted replacement history；0.169.0 补当前正文、镜像与环境来源标记，未覆盖 fork。 |
| Claude Code | JSONL；2.1.316                                | Bash 调用/拒绝/结果、compact_boundary 与保留消息；节选外祖先不补造。                                     |
| omp         | JSONL；文件格式 3                             | 旧实录覆盖工具、错误与 compaction；runtime 18.12.2、18.22.0，另有一份版本未知。                          |
| OpenCode    | 两代 SQLite SQL；会话版本 1.18.3、2.1.4       | project/session/message/part 与 session_v2/session_message；同库新 schema 不冒充旧版原始 schema。        |
| Kimi        | state.json + agents/main/wire.jsonl；协议 1.4/1.5 | 1.4 工具闭环；CLI 2.5.3 的 1.5 合成实录包含正文、Read 参数/结果和镜像事件。 |
| agy         | 原生 SQLite；新增 1.5.7 实录                  | 旧六步样本覆盖工具；新两步样本保留 cascade 与 trajectory 不同的身份关系，正文无工具调用。                |
| Devin       | SQLite SQL + ATIF-v1.7 JSON                   | 主消息链、工具状态、独立 transcript 前缀；transcript 明示 3000.12.57，SQLite 写入版本未知。              |
| QoderCN     | JSONL；1.1.81、1.1.84 | 旧版两份文件及 1.1.81 合成实录；后者覆盖正文与 Read 工具参数/结果，无 compaction 实录。 |
| Qoder       | 暂共用 QoderCN JSONL                          | 维护者确认的临时开发基线；国际版格式与路径尚未实测。                                                     |

各 Agent 的实际文件清单和记录覆盖以各自 manifest 为准。
版本证据必须区分记录自带版本与当前安装版本；当前安装版本不能证明旧会话由该版本生成。

2026-11-17 的 Kimi/QoderCN 补样来自专门创建的合成会话；其中工具样本只读取测试目录的 `fact.txt`。
旧样本保持原样，新文件独立记录来源版本、脱敏映射和工具覆盖；安装版本与原生记录内版本分别登记。
Kimi 1.5 的 injection 与 message 镜像按语义过滤；QoderCN 工具 input 投影为可导出的参数记录，与结果按 call ID 关联。
Qoder 与 QoderCN 共用上述有界解析规则；国际版仍只有中国版夹具的临时复用证据，未完成国际 CLI 实录验收。
未知版本、未知不兼容正文结构和冻结来源变化仍产生错误。
SQLite 样本保留真实 DDL 与经过脱敏的最小记录集合。

## 离线重建 SQLite

对 manifest 中 `format` 为 `sqlite-sql` 的文件，在临时目录执行：

```sh
workdir="$(mktemp -d)"
sqlite3 "$workdir/history.db" < path/to/fixture.sql
sqlite3 "$workdir/history.db" 'PRAGMA integrity_check; PRAGMA foreign_key_check;'
```

也可以用 Python 标准库 `sqlite3.Connection.executescript` 或项目已有的纯 Go SQLite 驱动重建。
SQL 只用于测试隔离数据库，不应导入真实 Agent 数据库；测试期间原生文件必须只读。
读取器测试应采用原生表与消息图，不能把 SQL 转成另一套自定义历史格式后宣称原生读取通过。

## 脱敏边界

- 会话与消息 ID 一致重映射，保留原生父子关系、工具关联、角色、事件类型、字段嵌套和记录顺序。
- 正文、工具参数与结果、标题、系统提示、思考、凭证、URL、账号和私人路径移除或替换；保留的工具名、类型与状态只用于格式识别。
- 时间、模型、工作目录、分支及内容摘要可能被固定化；不能据此测试真实耗时、token 计费、原始偏移或内容签名。
- 不提交认证文件、配置、环境变量、私人原文件路径、原始源文件哈希、完整数据库或不透明私密二进制块。
- JSON 字符串内嵌 JSON、SQL 中的 JSON 和工具载荷也属于脱敏范围，不只扫描最外层字段。

脱敏替换文本不是原用户的任务或真实工具执行结果。测试可断言这些公开值的解析，但不能把它们描述为原会话语义。

## 后续适配与补样

早期数据准备与后续 reader 实现分开记录；本机合成实录补样与来源修复覆盖 US-Q19、US-Q20。
每个 reader 仍需覆盖其来源适用的追加、半条记录、替换、分支、压缩、附件和取消等边界。
缺失场景可以另加明确标为合成的回归样本，不能将派生样本标记为本次电脑实录。
真实运行与模型效果须另行验收；离线 fixture 通过不证明 ACP/TUI 身份绑定、实际注入或图片成本。

三种来源的隔离文本交接验收与边界见[核对记录](user-stories/verification-log-2026-11-17.md#原生来源发现与身份绑定us-q19us-q20)；工具回归沿用旧实录与明确标记的合成输入。
