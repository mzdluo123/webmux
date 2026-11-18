# simple-native-handoff Specification

## Purpose

为单用户 WebMux 提供从 Agent 原生历史读取、默认生成 Snapcompact 混合压缩归档并向目标发送一次的交接能力；文本为显式选项。
交接状态保存在当前服务进程的有界内存中，原文通过 Agent 原生来源读取。

## Requirements

### Requirement: 原生来源与冻结范围

系统 SHALL 按 Agent、原生 session、HistoryRoot、分支和记录边界精确读取历史。
冻结范围 SHALL 保存来源校验、内容 hash 和解析版本。
范围内修改、来源丢失或解析版本不兼容 SHALL 返回具体错误。

#### Scenario: 预览之后追加

- **WHEN** 用户预览后 Agent 在原生历史尾部追加新消息
- **THEN** 原文补查和发送使用预览的原边界

#### Scenario: 冻结内容被替换

- **WHEN** 发送前冻结范围内的内容或原生身份发生变化
- **THEN** 系统返回来源变化或身份冲突错误，用户重新生成预览

#### Scenario: 无关历史格式不支持

- **WHEN** omp HistoryRoot 同时包含有效目标与正文不兼容的其他会话
- **THEN** 发现仅检查有界原生头，唯一目标经 reader 校验后可冻结；同 ID 的多个候选仍返回身份冲突

#### Scenario: 目标版本不支持

- **WHEN** Codex 目标原生 ID 存在，但 writer 版本未受支持
- **THEN** 系统 SHALL 返回来源格式错误并保留版本解析原因，身份不匹配仅用于缺失或歧义的身份

#### Scenario: ACP 与存储身份不同

- **WHEN** agy 数据库的 cascade ID 与 trajectory ID 不同
- **THEN** 系统 SHALL 用数据库 cascade ID 匹配 ACP 会话，并将两个 ID 纳入冻结来源校验

### Requirement: 可导出正文与机械投影

系统 SHALL 提取可导出用户、助手正文及可见工具调用/结果，保留来源顺序、角色及工具关联。
投影 SHALL 排除私有推理、密文、签名及不可导出 provider 状态，避免将结构 JSON 或记录标题当作正文。
系统 SHALL 采用确定性工具输出截断与空白规范化，并显示来源缺口和投影省略。
当前用户任务 SHALL 独立原样保留，不画入历史图片；原文补查 SHALL 返回同一范围内的可导出记录。

#### Scenario: 长工具输出

- **WHEN** 工具输出超过投影限制，当前任务包含代码缩进和中文
- **THEN** 预览显示省略信息，当前任务保持原样，用户可在有效范围内补查

### Requirement: 默认压缩归档与显式文本

请求 `format` SHALL 默认为 `image`，并支持显式 `text`；`maxPages` 省略或为 0 SHALL 采用 8 页上限，显式范围 SHALL 为 1–8。
图片预览 SHALL 包含按序的 `pages`，每页包含 index、mimeType、base64 data、sha256、width、height、bytes。
预览 SHALL 提供 `imageBudget` 的页数、字节、正文保留量、图片 token 估计及依据，以及 blockedReason、capability、historyText。
归档模式 SHALL 按首部文本、中间图片、尾部文本及当前任务发送；短历史 SHALL 完整保留为文本，historyText SHALL 仅供审阅。
文本模式 SHALL 按文本预算选取完整片段，保持来源顺序并报告省略。

#### Scenario: 请求省略格式

- **WHEN** 用户使用默认选项生成交接预览
- **THEN** 系统生成混合归档预览；短历史无图片，长历史图片按页序显示，任务始终保持独立文本

### Requirement: 完整页集与失败阻止

系统 SHALL 采用紧凑角色标签、模型相关排版和首尾文本，中段容量不足时采用高/低密度分级，再省略较旧中段范围。
文本范围、图片范围与显式省略范围 SHALL 无缺口、无重叠地划分投影正文，并明确报告正文保留字符数。
图片字节、页序和 hash SHALL 在预览时冻结；发送 SHALL 使用相同图片，不重新渲染。
需要图片时，缺字、覆盖不全、渲染失败、能力不足或附件/编码请求超限 SHALL 阻止发送，并提供 blockedReason。
图片 SHALL 遵守每条最多 8 张、单张 10 MiB、总原始数据 20 MiB、编码请求 30 MiB 及更低服务配置上限。
文本保守上界与图片 token 估计 SHALL 共同计入上下文预算；图片字节独立校验，未知费率 SHALL 明确标注，不声明为实际 usage。

#### Scenario: 历史超过页数上限

- **WHEN** 完整候选渲染超过 maxPages
- **THEN** 系统保留首尾文本并按容量规划中段分级页面，显式报告省略范围；成功页面集与文本范围覆盖所有未省略正文

#### Scenario: 新目标尚未确认图片能力

- **WHEN** 预览目标尚未创建
- **THEN** capability 为 pending；归档含图片时，确认后创建目标并检查实际 ACP 图片能力，能力不足时失败

#### Scenario: 用户改为文本

- **WHEN** 图片预览被阻止且用户明确选择 text
- **THEN** 系统生成新的文本预览，旧图片预览不会被隐式转换或发送

#### Scenario: 手机逐字编辑页数

- **WHEN** 用户删除原页数，再通过数字键盘输入新值
- **THEN** 编辑中允许空值，失焦或预览时校验 1–8 的整数；无效或已变更草稿阻止发送旧预览

#### Scenario: 比较长历史节省

- **WHEN** 评估归档相对文本的 token 节省
- **THEN** 比较相同投影覆盖的真实模型 usage，并单独记录机械截断、预算省略、精确字段错误和补查费用

### Requirement: 短期预览与失效

预览和快照 SHALL 有效五分钟，最多 32 项，manifest、文本和图片表示共同计入 64 MiB 上限。
系统 SHALL 在请求时惰性清理过期项；发送中的范围 SHALL 计入容量。
预览 SHALL 绑定确切文本、图片、任务、来源、目标身份和配置；修改任务、目标、格式或页数后发送 SHALL 使用新预览。

#### Scenario: 重启后再次确认旧预览

- **WHEN** 用户预览后服务重启，客户端再次提交旧预览
- **THEN** 请求返回失效错误，用户重新生成预览

#### Scenario: 预览缓存满

- **WHEN** 有效条目达到数量或总字节上限
- **THEN** 系统返回容量错误

### Requirement: 本进程内单次发送

系统 SHALL 在外部副作用前原子消费预览并登记实例绑定的操作 ID。
同一操作 ID 与预览 ID 的重复请求 SHALL 返回既有结果，冲突请求 SHALL 返回 409。
每个预览 SHALL 具有一次发送权，发送 SHALL 使用现有普通 prompt 和目标单写者保护。
派发的命令期限 SHALL 与目标 turn 的确认观察分离；同一 turn SHALL 使用共享广播回执，重复请求不能消耗其他观察者的确认。

#### Scenario: 双击并发提交

- **WHEN** 两个请求同时确认同一预览
- **THEN** 最多一次目标创建及一次 prompt 调用取得执行权，其余请求观察已有状态或收到冲突

#### Scenario: 修改操作内容后重试

- **WHEN** 已登记的操作 ID 被用于不同预览
- **THEN** 系统返回冲突错误

### Requirement: 操作结果与取消

操作结果 SHALL 最多保存 256 项，完成十五分钟后失效，满额返回容量错误。
外部调用无法确认时 SHALL 显示“可能已发送，请检查目标会话”。
操作 succeeded SHALL 表示本轮 Agent 正文、思考、工具调用、plan、当前授权/提问或成功 prompt 响应提供接收证据，不代表回合结束或任务正确。
30 秒命令期限 SHALL 仅约束派发；目标仍连接且未提供接收证据时 SHALL 保持 sending，不因慢响应而直接记录 unknown。
派发后取消确认观察或连接断开且未确认 SHALL 记录 unknown，系统 SHALL 不自动重发。
结果 SHALL 包含已知目标和已创建资源，用户可查询操作并在检查目标后主动生成新预览。

#### Scenario: 调用后确认丢失

- **WHEN** prompt 可能已被目标接收，WebMux 未取得确认
- **THEN** 当前进程保留结果不明状态，同一操作查询返回该状态

#### Scenario: 取消准备中的操作

- **WHEN** 用户在外部调用前取消操作
- **THEN** 操作形成明确失败结果；外部调用开始后的取消按实际确认状态返回

#### Scenario: 模型超过派发期限才响应

- **WHEN** prompt 已派发，目标连接保持有效，但首个 Agent 活动晚于 30 秒
- **THEN** 操作在等待期间保持 sending，获得本轮接收证据后 succeeded；回合最终输出另行核对

### Requirement: 页面内存与刷新

交接编辑状态 SHALL 保存于当前页面内存，刷新 SHALL 清空预览及操作句柄。
服务实例变化后，旧句柄 SHALL 返回失效错误。

#### Scenario: 刷新表单

- **WHEN** 用户在预览后刷新浏览器
- **THEN** 表单重新加载来源信息，用户任务与预览为空

### Requirement: 新聊天接收确认的投影

交接 SHALL 支持明确目标，或在确认后按普通创建流程新建聊天和工作位置。
新聊天 SHALL 接收本次投影及当前任务，目标 Agent 按原生规则保存自己的 session。
原文补查 SHALL 依赖有效范围和对应来源。

#### Scenario: 来源删除后的新聊天

- **WHEN** 交接完成后原生来源被删除
- **THEN** 已保存的目标会话按 Agent 原生规则存在，原文补查返回来源读取错误

#### Scenario: A 到 B 再回 A

- **WHEN** 用户从 A 交接到 B 后再次选择 A 为目标
- **THEN** 系统针对当前选择生成新预览，并发送用户确认的图片或文本及独立任务

### Requirement: WebMux 状态存储清理

系统 SHALL 删除 conversation 数据库、schema、迁移、descriptor 及生产消费者。
已有 hybrid 原生身份元数据、普通聊天、队列、TUI/ACP 切换和 Agent 原生恢复 SHALL 保留。
Agent 自己的 SQLite 历史 SHALL 继续按受支持格式只读访问。

#### Scenario: 旧数据库损坏

- **WHEN** 状态目录存在损坏、旧版本或被锁定的 WebMux 数据库
- **THEN** 新版按现有元数据启动，旧数据库及其 sidecar 保持原文件

#### Scenario: 读取 Agent SQLite

- **WHEN** 用户选择受支持的 OpenCode、agy 或 Devin 原生数据库
- **THEN** 系统通过只读 reader 和来源校验取得历史

### Requirement: 旧数据与协议切换

旧数据库及 sidecar SHALL 作为离线原件保留，启动 SHALL 说明旧版专有数据的备份用途。
REST 与 UI SHALL 使用新交接契约；MCP thread_read 与控制面 session_read SHALL 使用新原生范围读取契约。
CLI 的 mcp-serve SHALL 仅提供只读补查，不声明专用 preview/send 命令或工具；旧 conversation/handoff 路由 SHALL 返回 410。
认证、Origin、文件权限、敏感信息排除及有界读取 SHALL 沿用现有入口。

#### Scenario: 升级时存在旧交接记录

- **WHEN** 旧数据库内有 pending 或 unknown 投递记录
- **THEN** 新版保留旧文件，用户通过匹配旧版本离线检查这些记录

#### Scenario: 旧客户端调用恢复接口

- **WHEN** 旧客户端请求旧持久交接路由
- **THEN** 服务端返回 410
