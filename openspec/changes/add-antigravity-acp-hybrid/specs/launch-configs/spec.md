# launch-configs Specification Delta

## MODIFIED Requirements

### Requirement: 启动配置的混合模式开关

启动配置 SHALL 支持可选的布尔字段 `hybrid`，缺省为 `false`。

- 保存时为 `false` 的字段 SHALL 不写入文件。
- `launch-configs.json` 的版本 SHALL 保持 2；不含该字段的已有文件 SHALL 照常读取，所有配置视为未勾选。

`hybrid` 为 `true` 时，服务端 SHALL 按 `hybrid-agent-profiles` 校验 argv，校验不通过时 SHALL 返回 400 并给出原因，已有数据不变。

新安装预置的 Antigravity CLI（`agy`）、OpenCode 与 omp 配置 SHALL 为 `hybrid: true`，其 argv（含各自的自动批准参数）SHALL 通过 `hybrid-agent-profiles` 校验。修改已勾选配置的 argv 使其不再受支持时，保存 SHALL 返回 400，用户须先取消勾选。

新安装预置的 agent 配置 SHALL 带上各自的自动批准参数，使工具调用无需逐次批准：Claude Code `--dangerously-skip-permissions`、Codex CLI `--dangerously-bypass-approvals-and-sandbox`、Antigravity CLI（`agy`）`--dangerously-skip-permissions`、OpenCode `--auto`、omp `--auto-approve`。预置的 Antigravity CLI 配置图标 SHALL 为 `antigravity`；已有配置文件中保存的 `geminicli` 图标值 SHALL 继续被接受并正常渲染。Shell 配置 SHALL 不变。已存在的 `launch-configs.json` SHALL NOT 被改写。

#### Scenario: 新安装的预置配置

- **WHEN** 服务首次启动且 `launch-configs.json` 不存在
- **THEN** 预置的 Antigravity CLI 配置名称为 `"Antigravity CLI"`、图标为 `"antigravity"`、argv 为 `["bash","-ic","exec -- \"$@\"","webmux","agy","--dangerously-skip-permissions"]`、`hybrid` 为 `true`、`hybridSupport` 为 `{supported: true, agent: "agy"}`；OpenCode 与 omp 配置同样为混合配置；Claude Code 与 Codex CLI 带自动批准参数且 `hybrid` 为 `false`

#### Scenario: 已有安装不改写

- **WHEN** 升级前已存在 `launch-configs.json`（含旧的 `"icon": "geminicli"` 配置）
- **THEN** 服务正常加载且不修改其中任何配置，包括名称、图标、`hybrid` 与命令参数

#### Scenario: 勾选 omp 配置

- **WHEN** 用户对自己的 omp 配置把 `hybrid` 改为 `true` 并保存
- **THEN** 保存成功，摘要中 `hybrid` 为 `true`，`hybridSupport` 为 `{supported: true, agent: "omp"}`

#### Scenario: 旧文件

- **WHEN** 升级前保存的 `launch-configs.json` 不含 `hybrid` 字段
- **THEN** 服务正常读取，所有配置的 `hybrid` 为 `false`，文件不被改写

#### Scenario: 不支持的命令

- **WHEN** 用户对 Claude Code 配置提交 `hybrid: true`
- **THEN** 服务端返回 400，原因说明当前版本只支持 OpenCode、omp 与 agy
