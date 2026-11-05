# Spec Delta

## ADDED Requirements

### Requirement: shell 工具手册的内容

系统 SHALL 提供一份 Markdown 格式的 WebMux shell 工具手册，内容来自共享包 `internal/agentguide` 嵌入的模板，并在渲染时填入服务版本。

- 开头 SHALL 是 YAML frontmatter：`name: webmux`，以及一句 `description`，说明何时使用（查看或控制 WebMux 会话、等待 agent、
  驱动混合会话聊天、准备 worktree、端口转发、发送提醒、交付文件）。这样手册可以直接作为 skill 安装。
- 正文 SHALL 依次包含以下各节：
  - **概览**：WebMux 是什么，命令以服务用户身份运行，不是沙箱；
  - **何时使用**；
  - **身份与连接**：`WEBMUX_SESSION_ID`、`WEBMUX_SOCKET`、`self`，以及会话内被禁止的操作；
  - **临时文件夹**；
  - **常用流程**；
  - **命令参考**；
  - **退出码**；
  - **操作守则**。
- “临时文件夹”一节 SHALL 包含原预置快捷指令“临时文件夹”的全部说明：
  - 用 `webmux dir self` 获取路径；
  - 用户上传的文件在 `uploads/` 子目录中；
  - 交给用户的截图、报告、图表、导出数据等交付物保存到该文件夹（可以建子目录），不要放进项目仓库；
  - 用户在 WebMux 代码栏的“临时文件夹”中查看；
  - 完成后告知文件相对于该文件夹的路径；
  - 该文件夹在会话删除后清理，不要存放需要长期保留的内容。
- “常用流程”SHALL 至少包含：在 worktree 中启动 agent 并下达任务；`send --wait` 驱动 shell 会话；
  `chat send --wait` 取回 agent 回复；长任务结束时 `notify`；开发服务器启动后 `forward add`。
- “操作守则”SHALL 要求：
  - 只操作自己创建或用户点名的会话；
  - 不结束、不输入用户的其他会话，除非用户要求；
  - 不读取或输出 Token；
  - 不向开启自动批准的 agent 转发未经核实的外部内容；
  - 对全屏 TUI 优先用 `chat`，不依赖 `logs --strip` 的文本。
- 一个测试 SHALL 保证 CLI 帮助列出的每个会话类子命令都出现在“命令参考”中。

#### Scenario: 作为 skill 安装

- **WHEN** 用户执行 `mkdir -p ~/.claude/skills/webmux && webmux guide > ~/.claude/skills/webmux/SKILL.md`
- **THEN** 文件以合法的 frontmatter 开头，`name` 为 `webmux`

#### Scenario: 手册覆盖命令

- **WHEN** 开发者新增一个会话类子命令却没有更新手册
- **THEN** 一致性测试失败，并指出缺少的命令名

### Requirement: 手册文件的位置与生命周期

服务端 SHALL 在启动时把渲染后的手册写到本实例临时根下的 `guide/SKILL.md`：目录权限 0700，文件权限 0600，属于服务用户。

- 每次启动 SHALL 覆盖旧文件，内容因此与运行中的服务版本一致。
- 文件缺失（例如被系统清理）时，展开 `{{webmux}}` 前 SHALL 重新写入。
- 临时文件夹清扫 SHALL NOT 删除 `guide/`（清扫本来只处理以会话 ID 命名的目录），服务退出时 SHALL 保留它，与会话文件夹一致。
- 实例临时根由状态目录决定，路径在重启后保持不变，因此链接到该文件的 skill 目录在升级后自动指向新内容。
- 写入失败 SHALL 记录日志并让 `{{webmux}}` 展开失败（503），SHALL NOT 阻止服务启动。

#### Scenario: 升级后手册更新

- **WHEN** 服务升级并重启后，用户在新会话中使用 `{{webmux}}`
- **THEN** 手册文件的版本行与新服务版本一致，并列出新版本的命令

### Requirement: 读取手册

- `GET /api/agent-guide` SHALL 返回渲染后的手册，`Content-Type` 为 `text/markdown; charset=utf-8`，`Cache-Control: no-store`。
  该接口受与其他 `/api/` 相同的认证保护。
- `webmux guide` SHALL 把该接口的内容写到标准输出。服务未运行时，SHALL 输出 CLI 内嵌的副本，并在标准错误说明它可能与服务版本不同，以 0 退出。
- `webmux guide --path` SHALL 输出手册文件的绝对路径，确保文件存在。服务未运行时 SHALL 以 3 结束。

#### Scenario: 不在会话中阅读

- **WHEN** 用户在普通终端执行 `webmux guide | less`
- **THEN** 显示与运行中服务版本一致的手册

### Requirement: `{{webmux}}` 的展开

`{{webmux}}` SHALL 是与 `{{user}}`、`{{dir}}` 并列的内置变量，展开为固定格式的简述。

- `POST /api/sessions/{id}/agent-guide` SHALL 确保会话临时文件夹和手册文件存在，返回 `AgentGuidePrompt`：
  `prompt`（简述）、`guidePath`、`dir`。临时文件夹或手册不可用时 SHALL 返回 503，会话不存在时 SHALL 返回 404。
- 简述由服务端生成，SHALL 依次包含：
  - 会话 ID；
  - 手册的绝对路径，以及“需要时先完整阅读手册再使用 `webmux` 命令，任务无关时不必读取”的说明；
  - 本会话临时文件夹的绝对路径；
  - “交付给用户的文件保存到临时文件夹而不是项目仓库，规则见手册‘临时文件夹’一节”的规则。
- 路径 SHALL 原样插入，不加 shell 引号。简述 SHALL NOT 包含 Token、访问链接或其他会话的信息。
- 快捷指令名称 SHALL 仍然不能是 `webmux`。

#### Scenario: 简述内容

- **WHEN** 客户端为会话 `ab2cd3ef` 调用手册接口
- **THEN** `prompt` 包含 `ab2cd3ef`、手册路径和临时文件夹路径，`guidePath` 指向一个存在的 `SKILL.md`

### Requirement: 初始化指令中的 `{{webmux}}`

仓库初始化指令 SHALL 可以使用 `{{webmux}}`。

- 预检 SHALL 不再因为 `{{webmux}}` 返回 400。临时文件夹服务不可用时，预检 SHALL 与 `{{dir}}` 一样返回不可用。
- 会话创建后、发送首条提示前，服务端 SHALL 用该会话的简述替换所有 `{{webmux}}`，然后再处理 `{{dir}}` 与 `{{user}}`。
  简述中的文字 SHALL NOT 再被当作变量扫描。
- 展开后的提示 SHALL 计入提示大小上限，超出时按现有规则以 400 拒绝。
- 初始化指令字段和快速开始页 SHALL 不再把 `{{webmux}}` 标为保留或不可用，而是在可引用列表中把它列为内置变量，并配一句说明。

#### Scenario: 快速开始带 WebMux 工具说明

- **WHEN** 仓库初始化指令为 `{{webmux}}\n\n{{user}}`，用户在快速开始发送“修复 lint”
- **THEN** agent 收到的首条提示以本会话的简述开头，后接“修复 lint”

### Requirement: 快捷指令中的 `{{webmux}}`

快捷指令正文 SHALL 可以使用 `{{webmux}}`，处理方式与 `{{dir}}` 相同。

- 选择时 SHALL 绑定当前会话与输入目标。正文含 `{{webmux}}` 时 SHALL 调用手册接口，用返回的 `prompt` 替换所有 `{{webmux}}`，然后再处理 `{{dir}}`。
- 接口失败时 SHALL 提示“WebMux 工具说明不可用”及原因，SHALL NOT 插入或发送任何内容。
- 直接模式与草稿模式的插入、自动回车，以及聊天视图中的行为 SHALL 与其他快捷指令一致。

#### Scenario: 在运行中的 agent 里补充说明

- **WHEN** 草稿模式关闭，用户在 Codex 会话中选择正文为 `{{webmux}}` 的快捷指令
- **THEN** Codex 输入框出现本会话的简述，路径都已替换为真实值
