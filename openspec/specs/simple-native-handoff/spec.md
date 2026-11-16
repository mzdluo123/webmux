# simple-native-handoff Specification

## Purpose

为单用户 WebMux 提供从 Agent 原生历史读取、生成 Snapcompact 文本预览并向目标发送一次的交接能力。
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

### Requirement: 机械文本投影与当前任务

系统 SHALL 采用确定性工具输出截断、空白规范化及有界预算生成文本投影。
预览 SHALL 展示缺口、选中和省略数量，并标明历史为参考内容。
当前用户任务 SHALL 独立原样保留；原文补查 SHALL 返回同一范围内的可导出记录。

#### Scenario: 长工具输出

- **WHEN** 工具输出超过投影限制，当前任务包含代码缩进和中文
- **THEN** 预览显示省略信息，当前任务保持原样，用户可在有效范围内补查

### Requirement: 短期预览与失效

预览和快照 SHALL 有效五分钟，最多 32 项，manifest 与预览文本合计上限 64 MiB。
系统 SHALL 在请求时惰性清理过期项；发送中的范围 SHALL 计入容量。
预览 SHALL 绑定确切文本、任务、来源、目标身份和配置；目标变化或编辑后发送 SHALL 使用新预览。

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

#### Scenario: 双击并发提交

- **WHEN** 两个请求同时确认同一预览
- **THEN** 最多一次目标创建及一次 prompt 调用取得执行权，其余请求观察已有状态或收到冲突

#### Scenario: 修改操作内容后重试

- **WHEN** 已登记的操作 ID 被用于不同预览
- **THEN** 系统返回冲突错误

### Requirement: 操作结果与取消

操作结果 SHALL 最多保存 256 项，完成十五分钟后失效，满额返回容量错误。
外部调用无法确认时 SHALL 显示“可能已发送，请检查目标会话”。
结果 SHALL 包含已知目标和已创建资源，用户可查询操作并在检查目标后主动生成新预览。

#### Scenario: 调用后确认丢失

- **WHEN** prompt 可能已被目标接收，WebMux 未取得确认
- **THEN** 当前进程保留结果不明状态，同一操作查询返回该状态

#### Scenario: 取消准备中的操作

- **WHEN** 用户在外部调用前取消操作
- **THEN** 操作形成明确失败结果；外部调用开始后的取消按实际确认状态返回

### Requirement: 页面内存与刷新

交接编辑状态 SHALL 保存于当前页面内存，刷新 SHALL 清空预览及操作句柄。
服务实例变化后，旧句柄 SHALL 返回失效错误。

#### Scenario: 刷新表单

- **WHEN** 用户在预览后刷新浏览器
- **THEN** 表单重新加载来源信息，用户任务与预览为空

### Requirement: 新聊天接收文本投影

交接 SHALL 支持明确目标，或在确认后按普通创建流程新建聊天和工作位置。
新聊天 SHALL 接收本次投影及当前任务，目标 Agent 按原生规则保存自己的 session。
原文补查 SHALL 依赖有效范围和对应来源。

#### Scenario: 来源删除后的新聊天

- **WHEN** 交接完成后原生来源被删除
- **THEN** 已保存的目标会话按 Agent 原生规则存在，原文补查返回来源读取错误

#### Scenario: A 到 B 再回 A

- **WHEN** 用户从 A 交接到 B 后再次选择 A 为目标
- **THEN** 系统针对当前选择生成新预览，并发送用户确认的文本

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
REST、MCP、CLI 和 UI SHALL 使用新契约，旧 conversation/handoff 路由 SHALL 返回 410。
认证、Origin、文件权限、敏感信息排除及有界读取 SHALL 沿用现有入口。

#### Scenario: 升级时存在旧交接记录

- **WHEN** 旧数据库内有 pending 或 unknown 投递记录
- **THEN** 新版保留旧文件，用户通过匹配旧版本离线检查这些记录

#### Scenario: 旧客户端调用恢复接口

- **WHEN** 旧客户端请求旧持久交接路由
- **THEN** 服务端返回 410
