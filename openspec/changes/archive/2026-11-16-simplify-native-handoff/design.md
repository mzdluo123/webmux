## Context

单用户交接由 Agent 原生历史、机械投影预览和一次普通 prompt 组成。
原方案以 `origin/main` 的 `cda9edff` 为基线，已有九个原生 reader 和 Snapcompact。
本次实现覆盖 hybrid、历史浏览、归档、REST/MCP/CLI、前端及存储装配。

## Goals

- 复用九个 reader、来源校验和 Snapcompact 文本投影。
- 使用 hybrid 的 runtime ID、Agent、native ID、HistoryRoot 和 cwd 精确绑定来源。
- 交接预览、发送权及结果保存在当前进程的有界内存中。
- 普通聊天、队列、TUI/ACP 切换、原生恢复和单写者保护沿用现有机制。

## Decisions

### 1. 原生身份与只读来源

运行身份来自现有 hybrid 记录。原生文件按记录的 HistoryRoot 和 native ID 精确发现。
归档历史使用对应原生来源；身份缺失或来源不可读时返回具体错误。
`historyimport` 负责发现、冻结边界、hash 校验及 Unicode 分页。
agy、Devin、OpenCode 的 SQLite reader 和驱动继续提供原生只读访问。

### 2. 短期冻结范围

快照保存来源、分支、读取边界、解析版本和 hash。
预览绑定来源、目标身份、配置、预算及确切文本，当前用户任务独立原样保留。
冻结边界后的追加保持原预览有效；范围内修改、删除或身份变化要求重新生成预览。
快照与预览有效期五分钟，最多 32 项，manifest 与预览文本合计上限 64 MiB。
清理在请求时执行；满额返回容量错误，发送中的范围计入容量并保持 pin。
过期或重启后的范围返回失效错误。原文补查使用同一冻结范围。

### 3. 普通 prompt 单次发送

同 Agent 与跨 Agent 交接均发送机械投影和独立用户任务。
新聊天支持现有目录、现有 worktree 和新建 worktree，资源创建发生在确认发送后。
目标身份和 provider usage 在普通 prompt 的 actor 队列中再次核对。
首次真实 Agent 输出或 prompt RPC 响应形成发送确认。
失败结果包含已知目标和工作位置；外部调用结果无法确认时显示“可能已发送，请检查目标会话”。

### 4. 进程内消费与结果

随机实例标识绑定预览 ID 和操作 ID。登记操作与消费预览在同一把锁中完成。
相同操作 ID 与预览 ID 返回已有结果；冲突请求返回 409。
预览第一次接受发送即被消费。操作结果最多 256 项，完成后保留十五分钟。
准备阶段取消形成明确失败；发送阶段取消保留实际结果或结果不明状态。
HTTP 连接断开后，服务继续持有该操作。浏览器刷新清空交接编辑状态和句柄。
重启后的旧句柄返回失效错误，用户可以检查目标后主动生成新预览。

### 5. 新聊天与来源依赖

新聊天收到一次文本投影，目标 Agent 保存自己的原生 session。
预览展示选入数量、省略数量、缺口、预算和有效期。
原文补查依赖有效范围及其原生来源，来源消失后返回读取错误。
完整 Fork、持久前缀、引用计数、增量覆盖和独立快照属于已撤销的旧范围。

### 6. 协议与存储清理

删除 `internal/conversation`、schema、迁移和专属持久交接测试。
删除 hybrid conversation 同步、Run/receipt、历史注入、dispatch、checkpoint 和 reconcile。
REST、MCP、CLI、Zod 和 wire fixtures 使用短期快照与操作契约。
旧 conversation/handoff 路由返回 410。认证、Origin 和文件授权沿用现有入口。
删除 conversation statefile descriptor 和持久化清单条目，架构守卫覆盖当前实现。

### 7. 旧数据库离线保留

`conversations.sqlite` 及 journal/WAL/SHM、生命周期锁保留原文件。
新版运行路径从状态库装配中移除了这些文件，启动日志说明其离线备份用途。
升级前停旧进程并备份完整 state_dir；旧版专有正文通过匹配旧版本读取和导出。
其他现有 JSON 存储和 Agent 原生只读 SQLite 保持各自职责。

## Trade-offs

- 崩溃后的交接发送结果由用户检查目标会话确认。
- 每次交接发送投影文本，输入预算显示在预览中。
- 旧 SQLite 专有正文和 retained 前缀通过旧版离线备份访问。
- 多浏览器共享进程内消费保护，编辑内容各自保留在页面内存中。

## Validation

- Go race 覆盖并发消费、冲突、取消、过期、容量、身份变化和未知结果。
- 来源测试覆盖追加、替换、删除、Unicode 分页和九 reader 的版本夹具。
- API 测试覆盖认证、Origin、旧路由 410 和旧数据库文件保持原样。
- 隔离浏览器测试覆盖一次创建与发送、重启失效、原文补查及刷新清空。
- wire、前端类型、单元测试及文档、存储、架构守卫核对当前契约。
