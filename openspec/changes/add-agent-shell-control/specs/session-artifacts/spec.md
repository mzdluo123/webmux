# Spec Delta

## REMOVED Requirements

### Requirement: 临时文件夹提示词模板

**Reason**: 服务端设置 `artifactPrompt` 早已被快捷指令中的 `{{dir}}` 与预置指令“临时文件夹”取代，
服务端加载设置时丢弃这个旧字段（`LegacyArtifactPrompt`）。本变更把临时文件夹说明移入 shell 工具手册的“临时文件夹”一节，
由 `{{webmux}}` 简述引用。

**Migration**: 需要临时文件夹说明时，在快捷指令或初始化指令中写 `{{webmux}}`（见 `agent-shell-guide`）；
仍想自定义文字时，在快捷指令正文中使用 `{{dir}}`。已保存的 `artifactPrompt` 值不迁移，继续被丢弃。

### Requirement: 从快捷指令菜单插入提示词

**Reason**: ⚡ 菜单中已经没有固定项“插入临时文件夹提示词”，插入改由普通快捷指令完成：
正文中的 `{{dir}}` 与 `{{webmux}}` 在选择时展开，规则见 `terminal-quick-commands` 与 `agent-shell-guide`。

**Migration**: 使用预置指令“WebMux 工具”，或在任意快捷指令正文中写 `{{dir}}`。会话绑定、失败时不插入、清除锁定修饰键等行为由快捷指令的通用规则保证。

## ADDED Requirements

### Requirement: 临时文件夹的 shell 访问

会话临时文件夹 SHALL 可以从 shell 获取：`webmux dir SESSION` 调用确保接口并输出路径（见 `shell-session-control`）。
`{{webmux}}` 简述 SHALL 包含本会话临时文件夹的路径，以及交付物保存规则（见 `agent-shell-guide`）。

- 实例临时根下的 `guide/` 目录 SHALL 保存 shell 工具手册。它不是任何会话的临时文件夹，SHALL NOT 出现在代码栏的“临时文件夹”根中。
- 清扫与删除规则 SHALL 只作用于以会话 ID 命名的目录。

#### Scenario: agent 获取交付位置

- **WHEN** 会话内的 agent 执行 `webmux dir self`
- **THEN** 输出与代码栏“临时文件夹”根相同的绝对路径，该目录已存在

#### Scenario: 手册目录不被清扫

- **WHEN** 服务重启，启动时清扫遗留的会话文件夹
- **THEN** `guide/SKILL.md` 被重写而不是删除，其他会话文件夹按原规则处理
