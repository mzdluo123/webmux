## Purpose

让用户把多个 agent 步骤静态声明为可复用、可组合的工作流：agent 引用现有启动配置，步骤模板描述做什么与怎样算完成，工作流描述顺序、分支与有界回环；定义在运行前完整编译与校验。

## ADDED Requirements

### Requirement: 定义分为 agent、步骤模板与工作流三层

系统 SHALL 以三层模型保存工作流定义：agent 引用已保存的启动配置名称；步骤模板（step template）声明执行器种类、agent、提示词或 argv 模板、结果约定、产物检查、权限模式与超时；工作流声明参数、工作区模式与节点图。

三层之间 SHALL 只通过名称引用。修改启动配置 SHALL NOT 要求修改步骤模板；修改步骤模板 SHALL 影响所有引用它的工作流的后续运行，但 SHALL NOT 影响已开始的运行。

`chat` 与第二期 `tui` 步骤 SHALL 引用启动配置；`command` 步骤 SHALL 引用启动配置或给出内联 argv 之一，不得同时给出；`check` 步骤 SHALL 只使用内联 argv。

#### Scenario: 更换评审 agent

- **WHEN** 用户把步骤模板 `code-review` 的 agent 从 `codex` 改为 `omp-review` 并保存
- **THEN** 所有引用 `code-review` 的工作流在下一次运行使用 `omp-review`，正在进行的运行仍使用其快照中的 `codex`

#### Scenario: 同时给出启动配置与内联 argv

- **WHEN** 用户保存一个 `command` 步骤，同时填写 `agent` 与 `argv`
- **THEN** 保存被拒绝，错误指出两者互斥

### Requirement: 工作流通过 uses、include 与显式回边组合

工作流节点 SHALL 能以 `uses` 引用步骤模板，并用 `with` 覆盖模板内的 `with.*` 值、`agent` 与 `timeoutMinutes`；`with` SHALL NOT 覆盖执行器种类与结果约定。

工作流 SHALL 能以 `include` 嵌入另一个工作流，并以 `as` 指定命名空间、以 `with` 映射其参数；被嵌入节点的 ID SHALL 带命名空间前缀。嵌套深度 SHALL NOT 超过 4；include 环 SHALL 导致编译失败。

回到已出现节点的边 SHALL 只能通过节点的 `on[].goto` 或 `onFailure.goto` 声明，并且 MUST 带 1–10 的 `maxVisits`。

#### Scenario: 复用实施与评审子流程

- **WHEN** 工作流 A 与 B 都 `include` 工作流 `implement-and-review`，分别以 `as: impl` 与 `as: fix` 嵌入
- **THEN** 两者编译后分别包含 `impl.code`、`impl.review` 与 `fix.code`、`fix.review` 节点，互不冲突

#### Scenario: include 成环

- **WHEN** 工作流 A include B，B 又 include A
- **THEN** 编译失败，错误列出成环路径 `A → B → A`

#### Scenario: 回边缺少上限

- **WHEN** 评审节点声明 `goto: code` 但没有 `maxVisits`
- **THEN** 编译失败，错误定位到该节点的 `on` 条目

### Requirement: 定义在保存与启动时静态编译

系统 SHALL 把定义编译为不可变执行计划，并在保存定义、请求编译预览和启动运行时执行编译。编译 SHALL 展开组合、检查节点 ID 唯一、入口唯一、所有跳转目标存在、无不可达节点、每条回边有上限、展开后节点数不超过 64。

编译 SHALL 检查每个模板引用：`params.*` 必须已声明；`steps.<id>.*` 引用的节点必须在所有到达当前节点的路径上先执行；`steps.<id>.result.<field>` 的字段必须在该节点结果约定中声明。只在部分路径上执行的节点 SHALL 只能在可选段 `{{#…}}…{{/…}}` 内引用。

agent 名称在保存时缺失 SHALL 只产生警告；在启动运行时缺失 SHALL 使启动失败。编译错误 SHALL 定位到定义中的 JSON 路径。

#### Scenario: 引用尚未执行的结果

- **WHEN** 节点 `spec` 的提示词引用 `{{steps.review.result.verdict}}`，而 `review` 位于 `spec` 之后
- **THEN** 编译失败，错误指出 `review` 不一定先于 `spec` 执行

#### Scenario: 拼写错误的结果字段

- **WHEN** 节点引用 `{{steps.spec.result.chnage}}`，而 `spec` 只声明了 `change`
- **THEN** 编译失败，错误给出可用字段 `change`

#### Scenario: 编辑器实时校验

- **WHEN** 用户在定义编辑器中修改 JSON
- **THEN** 页面调用编译预览接口，显示错误与警告列表及只读流程预览，不保存定义

### Requirement: 模板只做查找替换并保持 argv 边界

模板语法 SHALL 只包含 `{{path}}` 查找替换与 `{{#path}}…{{/path}}` 可选段；系统 SHALL NOT 提供函数、运算、循环或任意表达式。

`command` 与 `check` 的 argv 每一项 SHALL 独立渲染，渲染结果 SHALL 仍是恰好一项，不按空白拆分，不经过 shell。声明 `omitEmpty` 的项在值为空时 SHALL 被删除；未声明时空值 SHALL 使该次尝试失败。

渲染后的提示词 SHALL 不超过 64 KiB，SHALL NOT 写入日志或运行记录。

#### Scenario: 含空格的参数

- **WHEN** 参数 `requirement` 为 `支持 "导出" 功能; rm -rf /`，`command` 步骤的 argv 为 `["claude", "-p", "{{params.requirement}}"]`
- **THEN** 子进程收到恰好三项 argv，第三项为原文，不发生 shell 解释

### Requirement: 条件与失败处理使用结构化声明

节点 SHALL 能以 `on` 列表声明后继，每项含结构化 `when` 与 `goto`，按顺序取第一个匹配；未匹配时 SHALL 进入下一个节点。`when` SHALL 只支持对当前节点结果字段的 `equals` / `in` / `exists`，对闸门的 `decision`，以及对 `command` / `check` 的 `exitCode`。

节点 SHALL 能以 `onFailure` 声明 `fail`（默认）、`retry`（`maxAttempts` 1–5）、`goto`（带 `maxVisits`）或 `gate`。回边访问次数耗尽时 SHALL 按 `onExhausted`（默认 `fail`，可选 `gate`）处理。

#### Scenario: 评审要求修改

- **WHEN** 评审节点结果 `verdict` 为 `changes_requested`，其 `on` 声明 `{"when": {"result": "verdict", "equals": "changes_requested"}, "goto": "code", "maxVisits": 2}`
- **THEN** 运行回到 `code` 节点，`code` 的访问次数加一

#### Scenario: 回环耗尽转人工

- **WHEN** 上述回边已访问 2 次，评审仍为 `changes_requested`，且 `onExhausted` 为 `gate`
- **THEN** 运行进入人工决定，提示回环已耗尽，用户可选择再试一次、跳过或终止

### Requirement: 定义持久化、导入导出与首次预置

系统 SHALL 把定义保存在 `<state_dir>/workflows.json`，经统一状态文件基础设施读写，带独立锁与修订号；保存时修订号不符 SHALL 返回冲突而不覆盖。文件损坏、被锁或不可读时 SHALL 只禁用工作流功能，不影响会话、混合会话与启动配置。

定义上限 SHALL 为 100 个工作流与 200 个步骤模板。系统 SHALL 提供整体导出与导入；导入 SHALL 先编译并展示与现有定义的名称冲突，经用户确认后写入。

新安装首次创建 `workflows.json` 时，系统 SHALL 写入可编辑的“需求 → spec → 编码 → 评审”示例。示例 SHALL 是普通用户数据，服务端 SHALL NOT 以硬编码方式保留或恢复它。

#### Scenario: 首次启动预置

- **WHEN** 状态目录中不存在 `workflows.json`，服务首次启动
- **THEN** 文件被创建并包含示例工作流与其步骤模板，设置页可编辑或删除它们

#### Scenario: 定义文件损坏

- **WHEN** `workflows.json` 含非法 JSON
- **THEN** 工作流页面与接口报告不可用，原文件保留，普通会话照常创建
