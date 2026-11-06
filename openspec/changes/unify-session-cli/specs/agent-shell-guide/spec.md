## MODIFIED Requirements

### Requirement: llms.txt 形式的 shell 工具手册

服务端 SHALL 按 llms.txt 约定在站点根路径提供 WebMux shell 工具手册。内容来自共享包 `internal/agentguide` 嵌入的 Markdown，不写入任何磁盘文件。

- `GET /llms.txt` SHALL 返回索引，格式遵循 llms.txt：
  - 一个 H1 标题 `WebMux`；
  - 一段引用块摘要，说明 WebMux 是什么、命令以服务用户身份运行、不是沙箱；
  - 一段说明何时需要阅读（查看或控制会话、启动 agent、等待、驱动聊天视图、准备 worktree、端口转发、提醒、交付文件）；
  - 若干 H2 分组，每组是一个链接列表，每项为 `[标题](/llms/<主题>.md): 一句说明`；
  - 一个 `## Optional` 分组，列出不常用的主题。
- `GET /llms/<主题>.md` SHALL 返回单个主题。主题至少包括：
  - `connect`：如何找到可执行文件与套接字（见“手册中的可执行文件定位”）、`WEBMUX_SESSION_ID`、`self`，以及会话内被禁止的操作；
  - `sessions`：会话的列出（含已归档）、元数据、标签、输入（含 `--reply`）、中断、视图切换、等待、输出、屏幕、结束、恢复与删除记录；
  - `launch`：`run`（含 `--follow`、新建 worktree、聊天视图与首条提示）；
  - `chat`：聊天视图专有的操作：回答请求、运行时配置、队列、状态与观察；
  - `workspace`：仓库与仓库设置、worktree 与主页显示、启动配置与端口转发；
  - `artifacts`：临时文件夹；
  - `notify`：提醒；
  - `recipes`：常用流程；
  - `exit-codes`：退出码；
  - `rules`：操作守则。
- `GET /llms-full.txt` SHALL 按索引顺序拼接全部主题，供一次性加载。
- 响应 SHALL 使用 `text/markdown; charset=utf-8`（`/llms.txt` 与 `/llms-full.txt` 使用 `text/plain; charset=utf-8`），并带 `Cache-Control: no-store`。
  主题名不存在时 SHALL 返回 404。
- 主题内的链接 SHALL 使用以 `/` 开头的站点路径，使同一份内容经 TCP 或本机套接字读取时都能解析。
- 一个测试 SHALL 保证：CLI 帮助列出的每个会话类子命令都在某个主题中出现，索引中的每个链接都能解析，每个主题都在索引中出现。

#### Scenario: 按需加载

- **WHEN** agent 执行 `curl -s --unix-socket "$WEBMUX_SOCKET" http://webmux/llms.txt`
- **THEN** 得到索引；agent 随后只读取 `/llms/sessions.md`，用 `webmux send --reply` 完成任务

#### Scenario: 不存在的主题

- **WHEN** 客户端请求 `/llms/token.md`
- **THEN** 服务端返回 404

### Requirement: 手册内容要求

- `artifacts` 主题 SHALL 包含原预置快捷指令“临时文件夹”的全部说明：
  - 用 `webmux dir self` 获取路径；
  - 用户上传的文件在 `uploads/` 子目录中；
  - 交给用户的截图、报告、图表、导出数据等交付物保存到该文件夹（可以建子目录），不要放进项目仓库；
  - 用户在 WebMux 代码栏的“临时文件夹”中查看；
  - 完成后告知文件相对于该文件夹的路径；
  - 该文件夹在会话删除后清理，不要存放需要长期保留的内容。
- `recipes` 主题 SHALL 至少包含：
  - 用 `run --new-worktree --view chat --prompt … --reply` 在新 worktree 中启动 agent 并取回回复；
  - 用 `run --follow` 在持久会话里运行命令并取得退出码；
  - 用 `send --wait` 驱动 shell 会话，用 `screen` 查看 TUI 当前画面；
  - 用 `ls --parent self` 找到并清理自己创建的会话；
  - 长任务结束时 `notify`；
  - 开发服务器启动后 `forward add`。
- `rules` 主题 SHALL 要求：
  - 只操作自己创建（`parent` 为自己）或用户点名的会话；
  - 不结束、不输入用户的其他会话，除非用户要求；
  - 不读取或输出 Token；
  - 不向开启自动批准的 agent 转发未经核实的外部内容；
  - 对全屏 TUI 优先用聊天视图（`view`、`send --reply`、`chat`），需要看画面时用 `screen`，不依赖 `logs --strip` 的文本；
  - 不确定命令用法时先运行 `webmux <命令> --help` 或 `webmux help --json`，不要猜测选项。

#### Scenario: 临时文件夹说明迁移

- **WHEN** 用户阅读 `/llms/artifacts.md`
- **THEN** 原“临时文件夹”预置指令中的每一条说明都在其中，路径通过 `webmux dir self` 获取

#### Scenario: 分派任务的流程

- **WHEN** agent 阅读 `/llms/recipes.md`
- **THEN** 其中用 `webmux run --new-worktree --view chat --launch-config NAME --prompt … --reply` 启动 agent 并取回回复，不出现 `chat new` 或 `chat send`
