# launch-agent-presets Specification

## Purpose

规定启动配置中 Qoder、Kimi 与 Devin 的预置命令、图标和已有配置处理方式，同时定义完成提醒、介入提醒的数据模型与设置界面。

## Requirements

### Requirement: Qoder 图标

服务端图标白名单 SHALL 接受 `"qoder"`。前端图标下拉 SHALL 提供 `{value: "qoder", label: "Qoder"}`，
启动配置图标组件 SHALL 渲染自定义的 Qoder SVG 图标；未设置图标时，argv 的可执行文件基本名为
`qoder`、`qodercn`、`qoderclicn` 或 `qoder-cn` 的配置 SHALL 自动显示 `qoder` 图标。QoderCN 复用同一图标。

#### Scenario: 保存 qoder 图标

- **WHEN** 客户端保存 `{name: "Qoder", icon: "qoder", cmd: ["qoder"]}`
- **THEN** 保存成功，摘要中 `icon` 为 `"qoder"`

#### Scenario: 图标自动检测

- **WHEN** 未设置图标的配置 argv 可执行文件为 `qodercn`
- **THEN** 前端显示 `qoder` 图标

### Requirement: Kimi Code CLI 预置混合配置

新安装的 `Kimi Code CLI` 预置 SHALL 保持现有名称与 `kimi` 图标，argv 在既有启动写法上增加 `--auto`，并设置 `hybrid: true`。该预置 SHALL 通过 `hybrid-agent-profiles` 校验，`hybridSupport` 为 `{supported: true, agent: "kimi"}` 且不包含连接器。

`--auto` SHALL 作为新预置的显式自动批准行为呈现；用户可编辑配置移除它。预置总数、其他配置、JSON 结构与现有 `kimi` 图标目录 SHALL 保持不变。

#### Scenario: 新安装的 Kimi 预置

- **WHEN** 服务首次启动且 `launch-configs.json` 不存在，服务端 `$SHELL` 为 Bash
- **THEN** Kimi Code CLI 预置的图标为 `kimi`，argv 为 `["bash","-ic","exec -- \"$@\"","webmux","kimi","--auto"]`，`hybrid` 为 `true`，`hybridSupport.agent` 为 `kimi`

#### Scenario: 从 Kimi 预置创建会话

- **WHEN** 用户在新安装上选择 Kimi Code CLI 预置
- **THEN** 可选择从终端或聊天开始，聊天端按配置推导使用 `mode=auto`

### Requirement: 已有 Kimi 配置不被迁移

已存在的 `launch-configs.json` SHALL NOT 因新增 Kimi 档案而被改写。升级前已保存的 Kimi 配置，其命令、`hybrid` 字段与其他内容 SHALL 保持原值；没有 `--auto` 或没有勾选混合模式的旧配置 SHALL NOT 自动补上这些设置。

旧 Kimi 配置未勾选混合模式时，用户手动勾选后 SHALL 按新档案校验并启用，无需删除或重建配置。

#### Scenario: 升级后旧预置保持原样

- **WHEN** 升级前已存在命令为 `kimi`、未设置 `hybrid` 的 Kimi 配置
- **THEN** 升级后命令仍不含 `--auto`，`hybrid` 仍为 `false`，状态文件不被重写

#### Scenario: 手动启用旧配置

- **WHEN** 用户对升级前保存的 `kimi` 配置手动勾选混合模式并保存
- **THEN** 保存成功，`hybridSupport.agent` 为 `kimi`，命令参数保持用户保存的内容

### Requirement: 启动配置完成与介入提醒设置数据模型与校验

启动配置对象及摘要 SHALL 支持可选字段 `attention`（类型为对象或 `null`/省略，省略时表示跟随全局默认配置）。当 `attention` 非空时，对象 MUST 包含以下字段，且超出范围或非法枚举值 MUST 返回 `400 Bad Request`：

- `mode`：取值为 `"all"`、`"urgent"` 或 `"off"`；
- `programStatus`、`agentProtocol`、`notify`、`bell`：布尔值；
- `minTurnSeconds`：整数，范围 `0`–`300`；
- `doneDelaySeconds`：整数，范围 `0`–`60`。

#### Scenario: 创建或更新带有效提醒设置的启动配置

- **WHEN** 客户端提交 `POST /api/launch-configs` 或 `PUT /api/launch-configs/{id}`，且 `attention` 为 `{"mode":"urgent","programStatus":false,"agentProtocol":true,"notify":true,"bell":false,"minTurnSeconds":10,"doneDelaySeconds":3}`
- **THEN** 服务端接受并持久化该配置，且在 `GET /api/launch-configs`、`GET /api/launch-configs/{id}` 和 `GET /api/config` 的启动配置摘要中返回相同的 `attention` 字段

#### Scenario: 省略或置空提醒设置表示跟随全局默认

- **WHEN** 客户端提交启动配置时省略 `attention` 或将 `attention` 设为 `null`，或者服务端加载旧版 `launch-configs.json` 中不含 `attention` 字段的记录
- **THEN** 服务端正常解析该配置，且该启动配置创建的会话跟随服务端全局提醒配置（`program_status`、`agent_protocol`、`bell_attention`）与默认提醒范围（全部提醒，无最短耗时与确认延迟限制）

#### Scenario: 拒绝非法的提醒范围或超出边界的阈值

- **WHEN** 客户端提交启动配置时 `attention.mode` 不属于 `"all"`、`"urgent"`、`"off"`，或 `minTurnSeconds` 小于 `0` / 大于 `300`，或 `doneDelaySeconds` 小于 `0` / 大于 `60`
- **THEN** 服务端返回 `400 Bad Request` 错误，且不修改已持久化的启动配置

### Requirement: 启动配置管理弹窗的完成与介入提醒设置与信号优先级提示

前端启动配置新增与编辑表单 SHALL 提供“完成与介入提醒”可选配置区，默认勾选“跟随全局默认”（对应 `attention = null`）。当取消勾选“跟随全局默认”时，表单 MUST 满足以下交互：

- 提供提醒范围选择（“全部提醒” / “仅需介入” / “关闭”）；
- 在提醒范围不为“关闭”时展示四类信号来源开关，并明确提示信号源优先级（`OSC 7501` 状态协议 > Warp Agent 事件 > 桌面通知 > 终端响铃；高优先级生效时自动忽略低优先级信号）；
- 在提醒范围为“全部提醒”时展示最短回合耗时与完成确认延迟设置。

#### Scenario: 默认勾选跟随全局默认并隐藏细分项

- **WHEN** 用户新建启动配置，或编辑一个 `attention` 未设置的启动配置
- **THEN** “完成与介入提醒”分组中的“跟随全局默认”复选框处于勾选状态，细分配置项不展开，保存时提交 `attention` 为 `undefined`/`null`

#### Scenario: 取消跟随全局默认并查看信号源优先级说明

- **WHEN** 用户在启动配置表单中取消勾选“跟随全局默认”
- **THEN** 表单展示提醒范围分段控件（默认选中“全部提醒”）、四项信号来源开关（`OSC 7501` 状态协议、Agent 事件协议、桌面通知 `OSC 9 / 777`、终端响铃 `BEL`）以及最短回合耗时与完成确认延迟输入框
- **AND** 信号来源区域显式展示优先级提示文案，说明信号按“`OSC 7501` 状态协议 > Agent 事件 > 桌面通知 > 终端响铃”优先级生效，关闭高优先级协议后会自动回退到下一优先级信号

#### Scenario: 切换提醒范围时按需隐藏无关子项

- **WHEN** 用户将提醒范围切换为“仅需介入”
- **THEN** 表单保留四项信号来源开关，但隐藏仅对完成类提醒生效的“最短回合耗时”与“完成确认延迟”输入框
- **WHEN** 用户将提醒范围切换为“关闭”
- **THEN** 表单同时隐藏信号来源开关与防误报阈值输入框

### Requirement: Devin CLI 预置混合配置

新安装的预置 SHALL 增加 `Devin CLI`，图标为 `devin`，argv 在既有启动写法上追加 `--permission-mode dangerous`，并设置 `hybrid: true`。该预置 SHALL 通过 `hybrid-agent-profiles` 校验，`hybridSupport` 为 `{supported: true, agent: "devin"}` 且不包含连接器。

`--permission-mode dangerous` SHALL 作为新预置的显式自动批准行为呈现；用户可编辑配置移除它。其他预置与 JSON 结构 SHALL 保持不变。

#### Scenario: 新安装的 Devin 预置

- **WHEN** 服务首次启动且 `launch-configs.json` 不存在，服务端 `$SHELL` 为 Bash
- **THEN** Devin CLI 预置的图标为 `devin`，argv 为 `["bash","-ic","exec -- \"$@\"","webmux","devin","--permission-mode","dangerous"]`，`hybrid` 为 `true`，`hybridSupport.agent` 为 `devin`

#### Scenario: 从 Devin 预置创建会话

- **WHEN** 用户在新安装上选择 Devin CLI 预置
- **THEN** 可选择从终端或聊天开始，聊天端按配置推导使用 `mode=bypass`

### Requirement: 已有 Devin 配置不被迁移

已存在的 `launch-configs.json` SHALL NOT 因新增 Devin 档案而被改写。升级前已保存的 `devin` 配置，其命令、`hybrid` 字段与其他内容 SHALL 保持原值；没有 `--permission-mode` 或没有勾选混合模式的旧配置 SHALL NOT 自动补上这些设置。

旧 `devin` 配置未勾选混合模式时，用户手动勾选后 SHALL 按新档案校验并启用，无需删除或重建配置。

#### Scenario: 升级后旧配置保持原样

- **WHEN** 升级前已存在命令为 `devin`、未设置 `hybrid` 的配置
- **THEN** 升级后命令仍不含 `--permission-mode`，`hybrid` 仍为 `false`，状态文件不被重写

#### Scenario: 手动启用旧配置

- **WHEN** 用户对升级前保存的 `devin` 配置手动勾选混合模式并保存
- **THEN** 保存成功，`hybridSupport.agent` 为 `devin`，命令参数保持用户保存的内容
