# Spec Delta

## MODIFIED Requirements

### Requirement: 首次使用的预置指令

系统 SHALL 提供以下预置指令，按顺序排列，每条带稳定的 `id`：

1. `preset-resume`：名称“恢复会话”，正文 `/resume`，开启“发送后自动回车”；
2. `preset-new`：名称“新会话”，正文 `/new`，开启“发送后自动回车”；
3. `preset-webmux`：名称“WebMux 工具”，正文 `{{webmux}}`，不自动回车。

服务端设置中的 `quickCommands` 为 `null` 时，菜单 SHALL 显示上述预置指令。预置指令 SHALL 与用户指令一样可编辑、排序和删除。

用户对列表做过任意一次保存（包括删除全部指令）后，服务端值 SHALL 变为数组（删除全部时为 `[]`），系统 SHALL NOT 再自动补回预置指令，列表为空时显示空状态。
所有客户端 SHALL 以服务端 `null` 与空数组的区别判断是否显示预置指令。设置页的“添加系统预置”SHALL 只提供列表中尚不存在 `id` 的预置指令。

原预置指令 `preset-artifacts`（“临时文件夹”）SHALL 不再出现在预置列表中，它的说明已移入 shell 工具手册。
已保存列表中的这条指令 SHALL 原样保留、继续可用，系统 SHALL NOT 迁移或删除它。

#### Scenario: 首次打开菜单

- **WHEN** 新浏览器首次打开终端，服务端 `quickCommands` 仍为 `null`
- **THEN** 菜单依次列出“恢复会话”“新会话”“WebMux 工具”，前两条带回车标记

#### Scenario: 删光后不再补回

- **WHEN** 用户在一个设备的设置中删除全部预置指令并确认，服务端保存为 `[]`，随后在另一台设备打开菜单
- **THEN** 两台设备均显示空状态，不出现预置指令

#### Scenario: 已保存旧预置

- **WHEN** 用户之前保存的列表含 `preset-artifacts`，升级后打开设置页
- **THEN** “临时文件夹”指令仍在列表中且可用，“添加系统预置”提供“WebMux 工具”
