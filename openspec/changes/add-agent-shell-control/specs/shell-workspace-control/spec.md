# Spec Delta

## ADDED Requirements

### Requirement: worktree 命令

系统 SHALL 提供 `webmux worktree` 子命令，并遵守 `shell-session-control` 的输出与退出码约定。仓库参数 `REPO` 可以是仓库 ID、已登记的仓库路径，也可以是 `.`（当前目录所属的已登记仓库）。

- `webmux worktree ls [REPO] [--json]`：不给仓库时列出所有已登记仓库及其 worktree；给了仓库时只列出该仓库。
  表格列为仓库、worktree ID、分支、路径、会话数。
- `webmux worktree create REPO (--branch NAME [--base REF] | --existing BRANCH) [--path DIR] [--json]`：
  调用创建接口，`--branch` 对应模式 `new`，`--existing` 对应模式 `existing`。
  不给 `--path` 时，SHALL 使用与快速开始相同的默认位置：仓库目录旁的 `<仓库名>.worktrees/<分支叶名>`。
  成功后 SHALL 输出 worktree 路径，`--json` 时输出 `Worktree`。
- `webmux worktree rm REPO WORKTREE [--force] [--yes]`：
  - SHALL 先调用移除预览，把受影响的会话、归档记录与阻止原因写到标准错误；
  - 有阻止原因且没有 `--force` 时，SHALL 以 1 结束；
  - 没有 `--yes` 且标准输入是终端时，SHALL 要求输入 worktree 路径确认；标准输入不是终端又没有 `--yes` 时，SHALL 以 2 结束；
  - 确认后以预览得到的确认 ID 调用移除接口：`--force` 时模式为 `force`，`confirmPath` 为 worktree 路径，`closeSessions` 为 true；
    否则模式为 `safe`。
- 仓库没有登记或参数有歧义时，SHALL 以 2 结束并列出候选。

#### Scenario: 为并行任务准备 worktree

- **WHEN** 编排脚本在仓库目录执行 `webmux worktree create . --branch fix/lint --base main`
- **THEN** 标准输出是新 worktree 的路径，所有浏览器的工作区随即显示它

#### Scenario: 非交互删除需要显式确认

- **WHEN** 脚本以管道运行 `webmux worktree rm . <ID>`，没有给 `--yes`
- **THEN** 不删除任何东西，退出码 2，提示需要 `--yes`

### Requirement: 仓库命令

- `webmux repo ls [--json]` SHALL 列出已登记仓库：ID、名称、主工作区路径、可用性、worktree 数量、是否设置了初始化指令。
- `webmux repo add [PATH]` SHALL 调用 `POST /api/repositories` 登记仓库，路径默认为当前目录。已登记时 SHALL 输出已有条目，并以 0 退出。
  路径不是 Git 仓库时 SHALL 输出服务端原因，并以 1 结束。
- `webmux repo rm REPO [--yes]` SHALL 取消登记，SHALL NOT 删除磁盘上的任何文件；确认规则与 `worktree rm` 相同。
- `webmux repo init REPO [TEXT|-|--clear|--show]` SHALL 读取或设置仓库的初始化指令（`PATCH /api/repositories/{id}`）：
  - `--show` 输出当前指令，不带参数时等同于 `--show`；
  - `TEXT` 或 `-`（标准输入）设置新指令，`--clear` 清空；
  - 指令中可以写 `{{webmux}}`、`{{dir}}`、`{{user}}` 与快捷指令引用；引用了不存在的快捷指令时，SHALL 在标准错误给出与设置页相同的提示，但仍然保存。
- 仓库参数 `REPO` 的解析规则与 `worktree` 相同。

#### Scenario: 为新仓库准备快速开始

- **WHEN** 脚本在新克隆的仓库目录中执行 `webmux repo add && printf '{{webmux}}\n\n{{user}}' | webmux repo init . -`
- **THEN** 仓库出现在所有浏览器的工作区中，之后的 `webmux chat new` 默认带上包含 WebMux 工具说明的初始化指令

### Requirement: 启动配置命令

- `webmux launch-config ls [--json]` SHALL 列出启动配置摘要：名称、图标、命令、是否混合模式，以及环境变量名（不含值）。
- `webmux launch-config show NAME [--json] [--reveal-env]` SHALL 按区分大小写的精确名称输出一条配置。
  默认 SHALL NOT 输出环境变量的值，`--reveal-env` 时才输出；`--json` 的结构与 `GET /api/launch-configs/{id}` 相同，值按 `--reveal-env` 决定是否替换为 `"<hidden>"`。
- `webmux launch-config add NAME [--icon ICON] [--env NAME=VALUE]... [--hybrid] [--preview-lines N] [--preview-skip N] -- <argv...>`
  SHALL 调用 `POST /api/launch-configs`。`argv` 原样作为命令，SHALL NOT 经过 shell 展开。
  `--hybrid` 时 SHALL 先调用混合模式支持检查，不支持时输出原因并以 1 结束。
- `webmux launch-config edit NAME [--rename NEW] [--icon ICON] [--env NAME=VALUE]... [--unset-env NAME]... [--hybrid|--no-hybrid] [--preview-lines N] [--preview-skip N] [-- <argv...>]`
  SHALL 读取现有配置，只修改给出的字段，再用 `PUT /api/launch-configs/{id}` 整体保存。未给出的字段 SHALL 保持原值，包括环境变量的值。
- `webmux launch-config rm NAME [--yes]` SHALL 删除配置，确认规则与 `worktree rm` 相同。
- `webmux launch-config presets [--json]` SHALL 列出内置预设；`webmux launch-config restore [PRESET]` SHALL 恢复全部或指定的预设。
- 名称不存在时 SHALL 以 2 结束并列出已保存的名称；名称冲突或校验失败时 SHALL 输出服务端原因并以 1 结束。
- 环境变量值 SHALL 只在请求体中传输，SHALL NOT 出现在命令的标准错误或诊断输出中。

#### Scenario: 用脚本添加 agent 配置

- **WHEN** 用户执行 `webmux launch-config add "Claude (sandbox)" --icon claude --hybrid --env CLAUDE_CONFIG_DIR=/w/.claude -- claude --dangerously-skip-permissions`
- **THEN** 配置出现在 Web 端新建会话的命令选择器中，且已开启混合模式

#### Scenario: 只改一个字段

- **WHEN** 用户执行 `webmux launch-config edit "Claude (sandbox)" --unset-env CLAUDE_CONFIG_DIR`
- **THEN** 只删除这个变量，命令、图标、混合模式与其他环境变量的值保持不变

#### Scenario: 默认隐藏变量值

- **WHEN** 用户执行 `webmux launch-config show "Claude (sandbox)" --json`
- **THEN** `env` 中每个值都是 `"<hidden>"`

### Requirement: 端口转发命令

- `webmux forward ls [--json]` SHALL 列出转发：端口、标签、状态、来源会话，以及本机可打开的转发地址。
- `webmux forward add PORT [--label TEXT] [--session SESSION] [--no-compat]` SHALL 调用新增接口。
  `--session` 接受 `self`，`--no-compat` 关闭全部开发服务器兼容选项。成功后 SHALL 输出转发地址。
- `webmux forward rm PORT` SHALL 移除转发。
- 端口非法、指向 WebMux 自己的端口或已经转发时，SHALL 输出服务端原因并以 1 结束。

#### Scenario: agent 启动开发服务器后开转发

- **WHEN** 会话内的 agent 启动 Vite 后执行 `webmux forward add 5173 --session self --label web`
- **THEN** 所有浏览器的转发列表出现 5173，CLI 输出 `/forward/5173/` 的完整地址
