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

### Requirement: 在 worktree 中启动会话

`webmux run` SHALL 接受 `--worktree REPO:WORKTREE`，以及简写 `--worktree PATH`（已登记 worktree 的路径）。

- 创建请求 SHALL 带 `workspace` 引用，`cwd` 为该 worktree 的路径。
- 同时给了 `--cwd` 时，`--cwd` SHALL 位于该 worktree 内，否则以 2 结束。
- `--worktree` SHALL 可以与 `--launch-config` 或 `--` 后命令同用，其余规则不变。
- 新增 `--start-side chat` 时，SHALL 要求启动配置开启混合模式，并以聊天端启动；`--prompt TEXT` 作为首条消息，`--init` 在首条消息前加入仓库初始化指令，与快速开始一致。

#### Scenario: 一条命令启动 agent 并下达任务

- **WHEN** 脚本执行 `webmux run --detach --worktree ../repo.worktrees/fix-lint --launch-config "Claude Code" --start-side chat --init --prompt "修复 lint"`
- **THEN** 会话在该 worktree 中以聊天端启动，首条提示包含展开后的初始化指令，CLI 输出会话 ID 与链接

### Requirement: 启动配置只读命令

- `webmux launch-config ls [--json]` SHALL 列出启动配置摘要：名称、图标、命令、是否混合模式，以及环境变量名（不含值）。
- `webmux launch-config show NAME [--json]` SHALL 按区分大小写的精确名称输出一条配置。
  文本形式 SHALL NOT 输出环境变量的值，`--json` 输出与 `GET /api/launch-configs/{id}` 相同。
- 名称不存在时 SHALL 以 2 结束并列出已保存的名称。新增、修改和删除不提供专用命令，SHALL 通过 `webmux api` 完成。

#### Scenario: 选择启动配置

- **WHEN** agent 执行 `webmux launch-config ls --json | jq -r '.[] | select(.hybrid) | .name'`
- **THEN** 输出所有开启混合模式的启动配置名称

### Requirement: 端口转发命令

- `webmux forward ls [--json]` SHALL 列出转发：端口、标签、状态、来源会话，以及本机可打开的转发地址。
- `webmux forward add PORT [--label TEXT] [--session SESSION] [--no-compat]` SHALL 调用新增接口。
  `--session` 接受 `self`，`--no-compat` 关闭全部开发服务器兼容选项。成功后 SHALL 输出转发地址。
- `webmux forward rm PORT` SHALL 移除转发。
- 端口非法、指向 WebMux 自己的端口或已经转发时，SHALL 输出服务端原因并以 1 结束。

#### Scenario: agent 启动开发服务器后开转发

- **WHEN** 会话内的 agent 启动 Vite 后执行 `webmux forward add 5173 --session self --label web`
- **THEN** 所有浏览器的转发列表出现 5173，CLI 输出 `/forward/5173/` 的完整地址
