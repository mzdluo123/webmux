# agent-shell-guide Specification

## Purpose

让脚本和 agent 按 llms.txt 约定，从正在运行的 WebMux 读取与当前版本一致的 shell 工具手册，并通过 `{{webmux}}` 在快捷指令与初始化指令中得到指向手册的简短说明，而不必把说明写进临时文件夹。

## Requirements

### Requirement: llms.txt 形式的 shell 工具手册

服务端 SHALL 按 llms.txt 约定在站点根路径提供 WebMux shell 工具手册。手写主题来自共享包 `internal/agentguide` 嵌入的 Markdown，`commands` 主题在启动时由命令表渲染，
两者都不写入任何磁盘文件。

- `GET /llms.txt` SHALL 返回索引，格式遵循 llms.txt：
  - 一个 H1 标题 `WebMux`；
  - 一段引用块摘要，说明 WebMux 是什么、命令以服务用户身份运行、不是沙箱；
  - 一段说明何时需要阅读（查看或控制会话、启动 agent、等待、驱动聊天视图、准备 worktree、端口转发、提醒、交付文件）；
  - 若干 H2 分组，每组是一个链接列表，每项为 `[标题](/llms/<主题>.md): 一句说明`；
  - 一个 `## Optional` 分组，列出不常用的主题。
- `GET /llms/<主题>.md` SHALL 返回单个主题。主题至少包括：
  - `connect`：如何找到可执行文件与套接字（见“手册中的可执行文件定位”）、`WEBMUX_SESSION_ID`、`self`，以及会话内被禁止的操作；
  - `sessions`：会话的列出（含已归档）、元数据、标签、输入（含 `--reply`）、中断、视图切换、等待、输出、屏幕、提醒、结束、恢复与删除记录；
  - `launch`：`run`（含 `--follow`、新建 worktree、聊天视图与首条提示）；
  - `chat`：聊天视图专有的操作：回答请求、运行时配置、队列、状态与观察；
  - `workspace`：仓库与仓库设置、worktree 与主页显示、启动配置与端口转发；
  - `artifacts`：临时文件夹；
  - `commands`：命令目录（见“手册与命令帮助的分工”）；
  - `recipes`：常用流程；
  - `exit-codes`：所有命令共用的退出码与输出约定；
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

### Requirement: 手册的访问边界

手册是静态说明文档。所有监听器上共用的手册正文 SHALL NOT 包含 Token、访问地址、主机名、文件路径、会话或仓库信息；
只有本机套接字附加的“本实例”信息可以包含下文列出的版本与两个绝对路径，且 SHALL NOT 包含 Token。

- 在 TCP 监听器上，`/llms.txt`、`/llms-full.txt`、`/llms/*` SHALL 与静态页面一样不要求 Token，但仍受 Host 检查约束。
  这一层的内容 SHALL NOT 包含服务版本。
- 在本机套接字上，同样的路径 SHALL 可以读取，并在索引的摘要后附加“本实例”信息：版本、`WEBMUX_BIN` 的绝对路径、套接字的绝对路径。
- 手册路径 SHALL NOT 出现在 `/api/` 与 `/ws/` 之下，因此不改变现有认证规则。

#### Scenario: 公网入口

- **WHEN** 未认证的客户端经 Cloudflare Tunnel 请求 `/llms.txt`
- **THEN** 返回不含版本与任何实例信息的静态手册

#### Scenario: 本机读取带版本

- **WHEN** 用户经本机套接字请求 `/llms.txt`
- **THEN** 摘要后列出本实例的版本、可执行文件路径与套接字路径，其余内容与 TCP 相同

### Requirement: 手册中的可执行文件定位

手册 SHALL 不假设 `webmux` 在 `PATH` 中。

- 索引摘要之后、第一个链接分组之前，SHALL 有一段“命令前缀”说明：
  下文的 `webmux` 指 WebMux 可执行文件，按以下顺序确定：
  1. 环境变量 `WEBMUX_BIN`（写作 `"$WEBMUX_BIN"`，带引号以容忍空格）；
  2. `{{webmux}}` 简述或“本实例”信息中给出的绝对路径；
  3. `PATH` 中的 `webmux`（可能与服务版本不同，用 `webmux version` 核对）。
- 各主题的示例 SHALL 统一写作 `webmux …`，SHALL NOT 硬编码任何安装路径（例如 `/usr/bin/webmux`）。
- `connect` 主题 SHALL 包含排错步骤：
  - 检查 `"$WEBMUX_BIN" version` 与 `"$WEBMUX_BIN" status`；
  - 环境变量丢失时，使用简述中的绝对路径，加 `--socket <路径>`，并把 `self` 换成简述中的会话 ID；
  - 没有可用的可执行文件时，用 `curl -s --unix-socket <套接字> http://webmux/...` 直接调用 `/api/` 与 `/llms*`；
  - 处在禁止访问套接字或外部可执行文件的沙箱中时，向用户说明无法控制 WebMux并给出该访问错误。

#### Scenario: PATH 中没有 webmux

- **WHEN** agent 按手册执行 `webmux ls` 失败，提示命令不存在
- **THEN** 手册的命令前缀说明让它改用 `"$WEBMUX_BIN" ls`，并且成功

#### Scenario: 示例不含安装路径

- **WHEN** 一致性测试扫描 TCP 层的全部手册内容
- **THEN** 不出现任何绝对路径形式的可执行文件，只出现 `webmux` 与 `$WEBMUX_BIN`

### Requirement: 手册内容要求

- `artifacts` 主题 SHALL 包含原预置快捷指令“临时文件夹”的全部说明：
  - 用 `webmux dir self` 获取路径；
  - 用户上传的文件在 `uploads/` 子目录中；
  - 有本机文件访问能力时，交给用户的截图、报告、图表、导出数据等交付物保存到该文件夹（可以建子目录）；
  - 用户在 WebMux 代码栏的“临时文件夹”中查看；
  - 完成后告知文件相对于该文件夹的路径；
  - 普通会话移除后清理；退出或 kill 未移除时保留；混合会话移出运行列表但保留可恢复记录时，目录保留且恢复后沿用；
  - forget 或记录淘汰清理，重启按可恢复记录清扫；目录不是长期存储，远程拿到路径不等于客户端可写。
- `recipes` 主题 SHALL 至少包含：
  - 用 `run --new-worktree --view chat --launch-config CONFIG_NAME --prompt TEXT --reply` 在新 worktree 中启动 agent 并取回回复，CONFIG_NAME 来自实际列表的名称；
  - 用 `run --follow` 在持久会话里运行命令并取得退出码；
  - 用 `send --wait` 驱动 shell 会话，用 `screen` 查看 TUI 当前画面；状态变化不证明命令完成或成功；
  - 用 `ls --parent self` 找到并清理自己创建的会话；
  - 长任务结束时 `notify`；
  - 开发服务器启动后 `forward add`。
- `rules` 主题 SHALL 要求：
  - 只操作自己创建（`parent` 为自己）或用户点名的会话；
  - 不结束、不输入用户的其他会话，除非用户要求；
  - 不读取或输出 Token；
  - 不向开启自动批准的 agent 转发未经核实的外部内容；
  - 对全屏 TUI 优先用聊天视图与统一 send 入口，需要看画面时用 `screen`，不依赖 `logs --strip` 的文本；
  - 不确定命令用法时先运行 `webmux <命令> --help` 或 `webmux help --json`。

#### Scenario: 临时文件夹说明迁移

- **WHEN** 用户阅读 `/llms/artifacts.md`
- **THEN** 原“临时文件夹”预置指令的交付位置、uploads、子目录、代码栏查看与相对路径规则均保留，路径通过 `webmux dir self` 获取，清理条件按准确生命周期说明

#### Scenario: 交付目录生命周期

- **WHEN** 混合会话 rm/archive、resume、最终 forget 或记录淘汰
- **THEN** 指南说明前两步保留沿用目录，最终清理；普通 rm 则清理，不再笼统说会话删除即清理

#### Scenario: 分派任务的流程

- **WHEN** agent 阅读 `/llms/recipes.md`
- **THEN** 其中用 `webmux run --new-worktree --view chat --launch-config NAME --prompt … --reply` 启动 agent 并取回回复，不出现 `chat new` 或 `chat send`

### Requirement: `webmux guide`

`webmux guide [TOPIC] [--full] [--path]` SHALL 经本机 API 读取手册并写到标准输出：

- 不带参数时读取 `/llms.txt`；
- `TOPIC` 对应 `/llms/<TOPIC>.md`；
- `--full` 对应 `/llms-full.txt`；
- `--path` SHALL 输出两行：服务端 `WEBMUX_BIN` 的绝对路径与套接字的绝对路径，取自状态契约新增的 `bin` 字段与 `socket`。
  服务端没有解析出可执行文件时，第一行为空并以 1 结束。

服务未运行时，读取手册 SHALL 输出 CLI 内嵌的同一份内容，在标准错误说明它可能与服务版本不同，并以 0 退出；`--path` SHALL 以 3 结束。
主题不存在时 SHALL 以 2 结束并列出可用主题。

#### Scenario: 作为 skill 安装

- **WHEN** 用户执行 `webmux guide --full > ~/.claude/skills/webmux/reference.md`
- **THEN** 文件包含全部主题，可以由用户自己的 `SKILL.md` 引用

### Requirement: `{{webmux}}` 的展开

`{{webmux}}` SHALL 是与 `{{user}}`、`{{dir}}` 并列的内置变量，展开为固定格式的简述。

- `POST /api/sessions/{id}/agent-guide` SHALL 确保会话临时文件夹存在，并返回 `AgentGuidePrompt`：`prompt`（简述）、`guideUrl`、`dir`。
  临时文件夹不可用时 SHALL 返回 503，会话不存在时 SHALL 返回 404。
- `guideUrl` SHALL 是本机访问链接加上 `/llms.txt`，例如 `http://127.0.0.1:7681/llms.txt`。
- `AgentGuidePrompt` SHALL 另含 `bin`（`WEBMUX_BIN` 的值，未设置时为 `null`）与 `socket`（套接字绝对路径，本机 API 不可用时为 `null`）。
- 简述由服务端生成，SHALL 依次包含：
  - 会话 ID；
  - WebMux 可执行文件的绝对路径与套接字的绝对路径，并说明 `webmux` 不在 `PATH` 中时用该绝对路径，环境变量丢失时加 `--socket`；
  - 手册地址 `guideUrl`，以及不需要 Token 的读取方式 `"<bin>" guide`
    （等价于 `curl -s --unix-socket <socket> http://webmux/llms.txt`）；
  - `bin` 为 `null` 时，SHALL 省略可执行文件一行，只给出 `curl` 读取方式；
  - “需要时先读索引，再按需读主题，任务无关时不必读取”的说明；
  - 本会话临时文件夹的绝对路径；
  - “交付给用户的文件保存到临时文件夹，规则见 `/llms/artifacts.md`”的规则。
- 路径与地址 SHALL 原样插入，不加 shell 引号。简述 SHALL NOT 包含 Token 或其他会话的信息。
- 快捷指令名称 SHALL 仍然不能是 `webmux`。

#### Scenario: 简述内容

- **WHEN** 客户端为会话 `ab2cd3ef` 调用该接口
- **THEN** `prompt` 包含 `ab2cd3ef`、可执行文件与套接字的绝对路径、`/llms.txt` 地址、`guide` 读取方式与临时文件夹路径，不包含 Token

### Requirement: 初始化指令中的 `{{webmux}}`

仓库初始化指令 SHALL 可以使用 `{{webmux}}`。

- 预检 SHALL 不再因为 `{{webmux}}` 返回 400。临时文件夹服务或简述不可用时，预检 SHALL 返回不可用（503），不创建 worktree。
- 会话创建后、发送首条提示前，服务端 SHALL 先把 `{{dir}}` 换成临时文件夹路径，再用该会话的简述替换所有 `{{webmux}}`，最后放入 `{{user}}`。简述中的文字 SHALL NOT 再被当作变量扫描。
- 展开后的提示 SHALL 计入提示大小上限，超出时按现有规则以 400 拒绝。
- 初始化指令字段和快速开始页 SHALL 在可引用列表中将 `{{webmux}}` 列为内置变量，并说明其展开值。

#### Scenario: 快速开始带 WebMux 工具说明

- **WHEN** 仓库初始化指令为 `{{webmux}}\n\n{{user}}`，用户在快速开始发送“修复 lint”
- **THEN** agent 收到的首条提示以本会话的简述开头，后接“修复 lint”

### Requirement: 快捷指令中的 `{{webmux}}`

快捷指令正文 SHALL 可以使用 `{{webmux}}`，处理方式与 `{{dir}}` 相同。

- 选择时 SHALL 绑定当前会话与输入目标。正文含 `{{webmux}}` 时 SHALL 调用该接口，先处理 `{{dir}}`，再用返回的 `prompt` 替换所有 `{{webmux}}`，简述中的文字不再被替换。
- 接口失败时 SHALL 提示“WebMux 工具说明不可用”及原因，SHALL NOT 插入或发送任何内容。
- 直接模式与草稿模式的插入、自动回车，以及聊天视图中的行为 SHALL 与其他快捷指令一致。

#### Scenario: 在运行中的 agent 里补充说明

- **WHEN** 草稿模式关闭，用户在 Codex 会话中选择正文为 `{{webmux}}` 的快捷指令
- **THEN** Codex 输入框出现本会话的简述，地址与路径都已替换为真实值

### Requirement: 手册与命令帮助的分工

命令表（见 `shell-cli-help`）SHALL 是单个命令用法的唯一来源；手册 SHALL 只承载命令帮助给不了的内容：能力索引、概念、选择命令的依据、跨命令流程与本实例信息。

- `commands` 主题 SHALL 是命令目录：
  - 按命令表的分组列出每个可见命令，每个命令一行：用法与一句话摘要，内容与 `webmux help` 的概览同源；
  - SHALL NOT 包含命令的详细说明、位置参数、选项、输出、退出码或示例；
  - 页首 SHALL 说明完整用法用 `webmux <命令> --help` 或 `webmux help --json` 查看。
- `commands` 主题 SHALL 在服务启动时由 `cmd/` 用命令表渲染一次并注入手册；`webmux guide` 在服务未运行时 SHALL 用 CLI 自己的命令表渲染同一页。
  仓库中 SHALL NOT 保存生成的 `commands.md`，也不需要“重新生成”的步骤。
- 手写主题 SHALL NOT 重复命令帮助：
  - SHALL NOT 列出某个命令的选项清单、按键或枚举取值清单、默认值；需要时写命令名并指向 `--help`；
  - 单命令的用法示例只放在命令表的 Examples 中；除 `recipes` 外，手写主题中的命令行 SHALL NOT 与命令表中任何一条示例相同。`recipes` 是跨命令流程，不受此限；
  - 会话状态、`self`、临时文件夹等概念 SHALL 只在手册中完整说明一次；命令帮助只给一句话定义并在 See also 中指向该主题。
- 一个测试 SHALL 保证：
  - 手写主题与 `docs/usage/shell-control.md` 中出现的每个 `webmux <命令> … --<选项>`，选项都属于该命令（或全局选项）；
  - 除 `recipes` 外的手写主题中，没有与命令表示例完全相同的命令行；
  - `commands` 主题列出命令表中每个可见命令，且不含 `Options:`、`Examples:` 等帮助小节。

#### Scenario: 命令目录只有一行

- **WHEN** agent 读取 `/llms/commands.md`
- **THEN** `send` 只占一行，内容为用法与摘要，页首提示用 `webmux send --help` 查看选项

#### Scenario: 手册中的选项过时

- **WHEN** 开发者把 `send --reply` 改名，但 `sessions` 主题仍写 `webmux send <ID> 继续 --reply`
- **THEN** 一致性测试失败，并指出该页与该选项

#### Scenario: 重复的单命令示例

- **WHEN** 开发者在 `sessions` 主题中加入与 `send` 帮助示例完全相同的命令行
- **THEN** 一致性测试失败，提示改为指向 `webmux send --help`

#### Scenario: 新增命令无需重新生成

- **WHEN** 开发者在命令表中新增一个命令并重新构建
- **THEN** `/llms/commands.md` 与 `webmux guide commands` 都列出该命令，仓库中没有需要更新的生成文件

#### Scenario: 一次加载的体积

- **WHEN** agent 读取 `/llms-full.txt`
- **THEN** 其中没有任何命令的选项清单，完整的选项说明只能经 `--help` 或 `help --json` 获得

### Requirement: 跨传输共享指南

初始化说明、CLI 指南与 control MCP instructions SHALL 共享操作语义，并按可信上下文裁剪入口。
CLI commands SHALL 继续由命令表生成；MCP SHALL 仅列当前 registry 的真实工具，不混入 freeze-history 的冻结 registry。

#### Scenario: 实际工具列表

- **WHEN** control MCP 注册工具发生变化
- **THEN** 指南和 capabilities 使用相同实际列表，不另维护硬编码清单

### Requirement: 逻辑操作重试与去重边界

指南 SHALL 要求同一逻辑写操作重试复用相同 ID 与负载，新操作才新 ID。
创建去重仅内存 10 分钟；混合命令仅每会话最近 256 项且 30 分钟，任一边界失效、淘汰或重启后不保证去重。
终端提交 SHALL 按独立 receipt、generation 与保留限制检查；原始按键没有通用 exactly-once，不确定时先 inspect。

#### Scenario: 响应丢失

- **WHEN** 创建响应丢失且仍在去重窗口内
- **THEN** 指南要求复用原 ID 和负载；超窗或重启后要求先检查资源，不盲目重发

#### Scenario: 容量先于时间失效

- **WHEN** 原混合请求已不在最近 256 项，即使未过 30 分钟
- **THEN** 指南不承诺不会重做，并要求先查询状态

### Requirement: 四类操作证据与业务验收

指南 SHALL 区分输入 accepted/written、会话状态变化、指定回合结束、进程退出与 exitCode 四类操作证据。
状态变化 SHALL NOT 证明指定回合完成；回合结束 SHALL NOT 证明进程退出。
任务验收是独立的第五类业务证据；idle、end_turn、退出码 0 或回复文本 SHALL NOT 单独证明业务成功。
超时仅停止观察，不取消执行，继续等待保持原目标。

#### Scenario: 观察到 idle

- **WHEN** 会话状态变化为 idle，但没有指定回合结束或进程退出证据
- **THEN** 指南只报告状态变化，不声称回合结束、进程成功退出或任务通过验收

### Requirement: 当前请求授权

指南 SHALL 先查询真实 pending request，区分授权与提问；超时、断线、取消或执行失败 SHALL NOT 被当作存在授权请求的证据。
授权 SHALL 使用实际 optionId，kind 仅为分类；只在用户授权范围内处理当前请求，不扩大到 auto 或未来请求。
终端按 Enter 选择默认项 SHALL 先确认画面中的请求内容与授权范围；MCP 缺少 answer 时 SHALL 说明需用户或已有其他客户端处理。

#### Scenario: 自定义授权 ID

- **WHEN** 当前选项 kind 为 allow_once，optionId 为 custom-approve-7
- **THEN** 使用 custom-approve-7 并核对当前请求和授权范围，不用 kind 替代

#### Scenario: 失败但没有授权请求

- **WHEN** send --reply 超时或断线，随后快照没有 pending permission request
- **THEN** 流程不调用 chat answer，不按 Enter 盲目批准，先查真实状态或报告失败原因

### Requirement: 版本与动态配置发现

指南 SHALL 将 WEBMUX_BIN/绝对路径仅视为文件定位，并要求比较客户端 version 与服务 status。
启动配置 SHALL 从真实列表选择；CLI 使用名称，MCP/API 使用配置 ID。模型 SHALL 从 agent 公布的配置项及允许值发现，不硬编码 provider/model；能力不足时说明限制，不伪造值。

#### Scenario: 同路径不同版本

- **WHEN** 二进制路径未变但文件已升级替换
- **THEN** 指南仍要求比较 version/status，不声称路径保证版本一致

#### Scenario: 自定义模型配置

- **WHEN** 实例只有自定义启动配置，agent 公布非默认模型值
- **THEN** CLI 使用真实配置名称，MCP/API 使用真实配置 ID，模型使用已公布的设置 ID 与允许值，不依赖 Codex 或 gpt-5

### Requirement: 显式上下文与文件能力边界

指南 SHALL 区分 transport（cli、mcp-stdio、mcp-http）与 locality（local、remote）。
远程 self SHALL 为 null，不从 env、PID 或调用者自称推断；stdio 也不自动证明会话身份。
服务端绝对路径 SHALL NOT 被描述为远程客户端写入能力，缺少 answer/view/upload 时不提供虚构调用。
TCP llms 路径 SHALL 保持无实例事实的静态文档；本机原有信息边界不变。

#### Scenario: 远程控制上下文

- **WHEN** 远程 MCP 没有文件写入或 upload 能力
- **THEN** 指南返回未知 self 和真实工具列表，不让客户端写服务端路径或调用虚构工具

#### Scenario: 静态公网文档

- **WHEN** TCP 客户端读取任一 llms 路径
- **THEN** 不包含服务版本、路径、会话身份或 Token

### Requirement: MCP 指南输入与主题

control MCP SHALL 提供只读 guide_read，输入为禁止额外字段的 object，仅可选 topic：1 至 64 字符并匹配 `^[a-z][a-z0-9-]*$`。
缺省返回索引；MCP 仅列 connect、operations、input、completion、requests、launch、artifacts、rules 精选主题，不复制完整 shell 手册。
工具 SHALL NOT 接受 URL、路径或任意命令。

#### Scenario: 按需读取

- **WHEN** 先以 `{}` 调用，再读取 completion
- **THEN** 分别返回索引和四类操作证据及独立业务验收说明

### Requirement: MCP 指南成功结果

成功 structuredContent SHALL 包含 kind（index|topic）、transport、locality、selfSessionId（string|null）、tools（实际工具名数组）、
topics（id/title/summary 对象数组）、topic（string|null）、markdown（string），所有字段必有。
索引 topic 为 null；主题 topics 为空数组。text content SHALL 沿用协议默认 JSON 编码，不增加专属 markdown envelope。

#### Scenario: 结构化结果

- **WHEN** guide_read 成功
- **THEN** 结构化字段完整且文本为相同结果的默认 JSON 表达

### Requirement: MCP 指南错误与大小限制

无效参数、未知主题和完整 UTF-8 编码结果超过 65536 bytes SHALL 沿用现有 handler 的 isError:true 工具错误文本。
文本 SHALL 分别说明参数问题、unknown_topic 与可用主题、guide_too_large 与上限；不得返回部分正文或敏感实例数据。
SHALL NOT 新增 -32602 handler 错误或结构化错误框架。

#### Scenario: 参数与大小边界

- **WHEN** topic 为 null、空值、错误类型、未知字段、合法未知主题或结果超限
- **THEN** 返回对应隔离工具错误文本，不返回成功结果或半篇正文
