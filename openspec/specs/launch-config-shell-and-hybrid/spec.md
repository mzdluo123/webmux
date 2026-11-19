# launch-config-shell-and-hybrid Specification

## Purpose

定义启动配置的混合模式开关（`chat` / `hybrid`）、只读混合支持判定（`chatSupport`）、设置页混合模式勾选框，以及交互式启动文件包装（`bash -ic` / `zsh -ic`）与服务端 `defaultShell` 规则。核心启动配置数据模型、持久化与新建会话选择契约见 [launch-configs](../launch-configs/spec.md)。

## Requirements

### Requirement: 启动配置的混合模式开关

启动配置 SHALL 支持可选的布尔字段 `chat`（界面称为“混合模式”），缺省为 `false`。HTTP 请求与响应 SHALL 使用 `chat`；`launch-configs.json` 中 SHALL 继续以键 `hybrid` 保存，由存储层映射。

- 保存时为 `false` 的字段 SHALL 不写入文件。
- `launch-configs.json` 的版本 SHALL 保持 2；不含该键的已有文件 SHALL 照常读取，所有配置视为未勾选。

`chat` 为 `true` 时，服务端 SHALL 按 `hybrid-agent-profiles` 校验 argv，校验不通过时 SHALL 返回 400
并给出原因，已有数据不变。校验 SHALL NOT 检查第三方连接器是否已安装。

新安装预置的 Antigravity CLI（`agy`）、OpenCode、omp、Codex CLI 与 Claude Code 配置 SHALL 为
`chat: true`，其 argv（含各自的自动批准参数）SHALL 通过 `hybrid-agent-profiles` 校验。
修改已勾选配置的 argv 使其不再受支持时，保存 SHALL 返回 400，用户须先取消勾选。

新安装预置的 agent 配置 SHALL 带上各自的自动批准参数，使工具调用无需逐次批准：

- Claude Code：`--dangerously-skip-permissions`
- Codex CLI：`--dangerously-bypass-approvals-and-sandbox`
- Antigravity CLI（`agy`）：`--dangerously-skip-permissions`
- OpenCode：`--auto`
- omp：`--auto-approve`

预置的 Antigravity CLI 配置图标 SHALL 为 `antigravity`；已有配置文件中保存的 `geminicli` 图标值
SHALL 继续被接受并正常渲染。Shell 配置 SHALL 不变。已存在的 `launch-configs.json` SHALL NOT 被改写。

#### Scenario: 新安装的预置配置

- **WHEN** 服务首次启动且 `launch-configs.json` 不存在
- **THEN** 预置的 Antigravity CLI 配置名称为 `"Antigravity CLI"`、图标为 `"antigravity"`、
  argv 为 `["bash","-ic","exec -- \"$@\"","webmux","agy","--dangerously-skip-permissions"]`
  （服务端 `$SHELL` 为 Bash 时）、`chat` 为 `true`、`chatSupport` 为 `{supported: true, agent: "agy"}`；
  OpenCode、omp、Codex CLI 与 Claude Code 配置同样为混合配置；文件中这些配置以 `"hybrid": true` 保存

#### Scenario: 已有安装不改写

- **WHEN** 升级前已存在 `launch-configs.json`（含旧的 `"icon": "geminicli"` 配置）
- **THEN** 服务正常加载且不修改其中任何配置，包括名称、图标、`hybrid` 键与命令参数

#### Scenario: 勾选 omp 配置

- **WHEN** 用户对自己的 omp 配置把 `chat` 改为 `true` 并保存
- **THEN** 保存成功，摘要中 `chat` 为 `true`，`chatSupport` 为 `{supported: true, agent: "omp"}`，文件中该配置为 `"hybrid": true`

#### Scenario: 未安装连接器时勾选 Codex 配置

- **WHEN** 服务上没有 `codex-acp`，用户对 Codex 配置把 `chat` 改为 `true` 并保存
- **THEN** 保存成功；连接器检查只在启动 ACP 进程之前进行

#### Scenario: 旧文件

- **WHEN** 升级前保存的 `launch-configs.json` 不含 `hybrid` 键
- **THEN** 服务正常读取，所有配置的 `chat` 为 `false`，文件不被改写

#### Scenario: 不支持的命令

- **WHEN** 用户对 Gemini CLI 配置提交 `chat: true`
- **THEN** 服务端返回 400，原因为“未识别到支持的 Agent”，已有数据不变

#### Scenario: 请求体使用旧字段名

- **WHEN** 客户端创建启动配置时在请求体中提供 `hybrid: true`
- **THEN** 服务端以未知字段返回 400，已有数据不变

#### Scenario: 新安装的 Qoder 与 QoderCN 预置

- **WHEN** 服务首次启动且 `launch-configs.json` 不存在，服务端 `$SHELL` 为 Bash
- **THEN** 预置的 Qoder 配置图标为 `"qoder"`、argv 为
  `["bash","-ic","exec -- \"$@\"","webmux","qoder","--dangerously-skip-permissions"]`、`hybrid` 为 `true`、
  `hybridSupport` 为 `{supported: true, agent: "qoder"}`；QoderCN 配置图标为 `"qoder"`、
  argv 以 `qodercn` 代替 `qoder`、`hybridSupport.agent` 为 `"qodercn"`

### Requirement: 启动配置的混合支持信息

启动配置的完整配置、摘要、创建与更新请求 SHALL 包含 `chat`。完整配置与摘要 SHALL 额外包含只读的 `chatSupport: {supported, agent?, reason?, connector?}`，由服务端按当前 argv 计算；请求体中提供 `chatSupport` 或 `hybridSupport` SHALL 视为未知字段而返回 400。

`connector` 只在 `supported` 为 `true` 且档案声明了第三方连接器时出现，内容为 `{executable, title, installCommand, docsUrl}`，
SHALL 只来自档案，SHALL NOT 反映连接器是否已安装。

#### Scenario: 请求体携带只读字段

- **WHEN** 客户端创建或更新启动配置时在请求体中提供 `chatSupport`
- **THEN** 服务端返回 400，已有数据不变

#### Scenario: Codex 配置的连接器信息

- **WHEN** 客户端调用 `POST /api/launch-configs/chat-support {cmd: ["codex"]}`
- **THEN** 返回 `{supported: true, agent: "codex", connector: {executable: "codex-acp", …}}`，无论服务上是否安装了 `codex-acp`

### Requirement: 设置页的混合模式勾选框

设置页的启动配置编辑器 SHALL 提供“混合模式（聊天 ↔ 终端）”勾选框，对应启动配置的 `chat` 字段。

- 勾选框是否可用 SHALL 由服务端判定，浏览器 SHALL NOT 复制档案规则。打开已有配置时使用其 `chatSupport`；编辑命令时，设置页 SHALL 去抖调用 `POST /api/launch-configs/chat-support {cmd}`（已认证，只读，返回同样的 `chatSupport`）。系统 SHALL NOT 提供 `POST /api/launch-configs/hybrid-support`。
- 不可用时，勾选框 SHALL 禁用，并在旁边显示原因。
- 可用且 `chatSupport.connector` 存在时，勾选框旁 SHALL 说明“聊天端需要另外安装 `<可执行文件>`”，并显示可复制的安装命令与说明链接。
- 启动配置列表 SHALL 为已勾选的配置显示“混合”标记。

#### Scenario: Shell 配置

- **WHEN** 用户打开 Shell 配置的编辑器
- **THEN** 混合模式勾选框不可用，并显示“未识别到支持的 Agent”

#### Scenario: Codex 配置

- **WHEN** 用户打开预置的 Codex CLI 配置编辑器
- **THEN** 混合模式勾选框可用且已勾选，旁边说明聊天端需要另外安装 `codex-acp` 并显示安装命令

### Requirement: 启动文件选择与包装命令

启动配置编辑器 SHALL 在命令下方提供“加载 shell 启动文件”复选框与 shell 选择（Bash `~/.bashrc` / Zsh `~/.zshrc`），并 SHALL 只在勾选后显示并应用所选 shell。

- 保存时 SHALL 以完整 argv 记录选择：Bash 为 `["bash","-ic","exec -- \"$@\"","webmux",<命令>,<参数>…]`，Zsh 为 `["zsh","-ic","exec -- \"$@\"","webmux",<命令>,<参数>…]`。
- 编辑器 SHALL 只识别上述两种精确前缀；重新编辑 SHALL 还原为普通命令并保持勾选与所选 shell，保存 SHALL NOT 重复包装；取消勾选 SHALL 恢复直接执行。
- 包装参数 SHALL 计入 `cmd` 的 1–128 项上限；SHALL NOT 展开 `$HOME`、管道或命令替换。
- 服务端 SHALL NOT 新增持久字段，`launch-configs.json` 版本 SHALL 保持 `2`；选择由 argv 表达。

#### Scenario: Zsh 往返

- **WHEN** 用户把命令 `omp --auto-approve` 勾选“加载 shell 启动文件”、选择 Zsh 并保存，之后重新打开编辑
- **THEN** 保存的 argv 为 `["zsh","-ic","exec -- \"$@\"","webmux","omp","--auto-approve"]`；编辑器显示普通命令、保持勾选与 Zsh，再次保存不重复包装

#### Scenario: 取消勾选

- **WHEN** 用户取消已包装配置的勾选并保存
- **THEN** 保存的 argv 只有普通命令与参数

#### Scenario: 用户手写其他包装

- **WHEN** 配置的 argv 为 `["sh","-c","opencode"]` 或 `["fish","-c","opencode"]`
- **THEN** 编辑器不把它们识别为“加载 shell 启动文件”，保存与执行保持原样

### Requirement: 默认启动 Shell

服务端 SHALL 在已验证的 `GET /api/config` 响应中提供只读字段 `defaultShell`，取值为 `"bash"`、`"zsh"` 或空字符串，由服务进程 `$SHELL` 的 basename 决定：

- `bash` → `"bash"`；
- `zsh` → `"zsh"`；
- 未设置或为空 → `"bash"`，保持既有默认行为；
- 其他 shell（fish、nu、sh 等）→ 空字符串。

该字段 SHALL NOT 包含路径、环境变量值或可执行文件探测结果。设置页 SHALL 用 `defaultShell` 预选启动文件的 shell；其值为空时 SHALL 在用户勾选后回退到 Bash。打开已有包装配置时 SHALL 以 argv 中的 shell 为准，忽略 `defaultShell`。

#### Scenario: macOS launchd

- **WHEN** 服务以 `SHELL=/bin/zsh` 启动，用户新建启动配置并勾选“加载 shell 启动文件”
- **THEN** `GET /api/config` 返回 `defaultShell: "zsh"`，编辑器预选 Zsh

#### Scenario: systemd 未设置 SHELL

- **WHEN** 服务环境没有 `SHELL`
- **THEN** `defaultShell` 为 `"bash"`，编辑器与预置继续使用 Bash

#### Scenario: fish

- **WHEN** 服务环境的 `SHELL` 为 `/usr/bin/fish`
- **THEN** `defaultShell` 为空字符串；用户勾选时编辑器回退到 Bash
