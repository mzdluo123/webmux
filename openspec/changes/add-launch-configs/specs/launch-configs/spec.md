## Purpose

服务器持久化的启动配置让同一 WebMux 实例上的用户以“名称 + 完整 argv + 环境变量”复用启动命令，跨浏览器、跨设备共享，并作为后续首页一键启动与 CLI 的稳定输入。启动配置只描述如何启动会话，不改变 PTY、终端连接、回放或会话清理的生命周期。

## ADDED Requirements

### Requirement: 启动配置数据模型与命令快照

系统 SHALL 为每条启动配置保存服务端生成的 `id`、`name`、完整命令 `cmd`、环境变量 `env`、`createdAt` 与 `updatedAt`。`cmd` SHALL 是保存时解析出的完整 argv，包含预设命令与追加参数；服务端 SHALL NOT 在启动时重新读取 TOML 预设来重建命令。

`name` 去除首尾空白后 SHALL 为 1–64 个 Unicode 字符，并在同一存储内唯一；比较 SHALL 使用去空白后的精确值。`cmd` SHALL 包含 1–128 项，第一项 SHALL 非空，单项 SHALL NOT 超过 8 KiB。`env` SHALL 最多 64 项，值 SHALL NOT 超过 8 KiB，名称与值 SHALL 满足 `session.ValidateEnv` 的现有规则，包括 `WEBMUX_SESSION_ID` 保留。

`id` 与 `createdAt` SHALL 由服务端生成且客户端不可修改；`updatedAt` SHALL 在每次成功更新时由服务端刷新。时间 SHALL 以 UTC RFC 3339 表示。每条配置的 `env` 在序列化时 SHALL 为对象，空映射 SHALL 表示为 `{}`。单个存储 SHALL 最多保存 100 条配置。

#### Scenario: 预设后改不影响已保存配置

- **WHEN** 用户从 `claude` 预设加参数 `--resume` 创建启动配置，之后管理员在 TOML 中修改或删除该预设
- **THEN** 该启动配置仍保存并执行创建时的完整 argv `["claude", "--resume"]`

#### Scenario: 名称与数量边界

- **WHEN** 用户提交去空白后为空的名称或第 101 条配置
- **THEN** 服务端以 400 拒绝，已有配置不变

#### Scenario: 重名

- **WHEN** 用户提交与已有配置去空白后同名的配置
- **THEN** 服务端以 409 拒绝，已有配置不变

#### Scenario: 空参数保留

- **WHEN** 用户保存的命令文本包含一个合法的空参数
- **THEN** 保存的 `cmd` 保留该空字符串，启动时按 argv 原样传给 PTY

### Requirement: 服务端持久化与故障隔离

启动配置 SHALL 保存于 `<state_dir>/launch-configs.json`，文件权限 SHALL 为 0600，状态目录权限 SHALL 为 0700。每次变更 SHALL 通过同目录临时文件写入、`fsync`、关闭、`rename` 与目录同步原子替换；保存失败时 SHALL 回滚内存状态并返回错误。

服务端 SHALL 在启动时加载一次，此后 CRUD 立即生效，SHALL NOT 要求重启，也 SHALL NOT 写回 `config.toml`。

状态文件不存在时，服务端 SHALL 使用当前生效的 TOML 预设生成一次默认配置并保存：名称取预设的 `name`，命令取该预设解析后的完整 argv，环境变量为空。文件已存在时 SHALL NOT 再按 TOML 生成或同步配置。

状态文件无法解析时，服务端 SHALL 保留原文件，SHALL NOT 覆盖、改名或重建，并 SHALL 让启动配置接口返回 503；会话创建、终端连接、WebSocket 与其他设置 SHALL NOT 因此不可用。状态目录或文件不可读写时 SHALL 同样只禁用启动配置功能。损坏文件被手动修复或删除后，服务端 SHALL 需要重启才能重新加载或重新种子。

#### Scenario: 首次启动种子

- **WHEN** 服务首次启动且状态目录中没有 `launch-configs.json`
- **THEN** 服务写入包含当前 TOML 预设的配置，且后续重启或修改 TOML 不会重复种子或覆盖用户编辑

#### Scenario: 损坏文件不影响会话

- **WHEN** `launch-configs.json` 内容损坏，用户打开设置页并启动普通会话
- **THEN** 启动配置接口返回 503 且原文件保持存在，会话仍可创建、连接和退出

#### Scenario: 保存失败回滚

- **WHEN** 创建配置时临时文件写入或重命名失败
- **THEN** 接口返回错误，内存中不出现该配置，之前已加载的配置保持可用

### Requirement: 启动配置 API

服务端 SHALL 提供以下已认证接口：

- `GET /api/launch-configs`：返回按创建顺序排列的配置数组，存储不可用时返回 503；
- `POST /api/launch-configs`：接收 `{name, cmd, env}`，成功时返回 201 与新配置；
- `PUT /api/launch-configs/{id}`：以 `{name, cmd, env}` 整体替换可编辑字段，成功时返回 200；
- `DELETE /api/launch-configs/{id}`：成功时返回 204。

创建和更新 SHALL 校验数据模型中的全部限制。缺省 `env` SHALL 视为空对象；`PUT` 为整体替换，未提供的 `env` SHALL 清空原值。名称冲突 SHALL 返回 409；`id` 不存在 SHALL 返回 404；字段缺失、为空或非法 SHALL 返回 400；请求体 SHALL 拒绝未知字段且只接受一个 JSON 对象。更新 SHALL NOT 修改 `id` 与 `createdAt`。落盘失败 SHALL 返回 500 并保持原数据，存储不可用或文件损坏 SHALL 返回 503。

这些接口 SHALL 继承 `/api/*` 现有的认证、Host 与 Origin 校验，并 SHALL NOT 在错误信息、日志或诊断输出中记录环境变量值。列表以及创建/更新响应 SHALL 返回完整 `env` 供编辑回填。

#### Scenario: 创建与更新

- **WHEN** 用户创建 `{name: "Claude 恢复", cmd: ["claude"], env: {}}`，随后整体更新名称与命令
- **THEN** 创建返回 201，更新返回 200，`id` 与 `createdAt` 不变，`updatedAt` 刷新，列表中顺序不变

#### Scenario: 重名与未知 ID

- **WHEN** 用户提交与已有配置去空白后同名的配置，或更新不存在的 `id`
- **THEN** 分别返回 409 与 404，已有数据不被修改

#### Scenario: 认证与损坏存储

- **WHEN** 未认证请求访问启动配置接口，或文件损坏时已认证用户读取列表
- **THEN** 未认证请求按现有规则返回 401/403；已认证请求在文件损坏时返回 503 及包含状态文件路径的错误

### Requirement: 设置页管理启动配置

设置页 SHALL 提供“启动配置”卡片，列出每条配置的名称、命令预览与环境变量数量，并提供新建、编辑、删除（需确认）与空状态。页面 SHALL 支持 `#launch-configs` 深链接滚动定位，并在达到 100 条上限时阻止新增并提示。

编辑器 SHALL 包含名称、命令与环境变量三部分。命令 SHALL 使用现有 `parseArgs` 解析，并 SHALL 支持从 `GET /api/presets` 快速填入完整 argv；保存后 SHALL 是独立副本。命令文本与 argv 的往返 SHALL 保留空参数、空白、引号、反斜杠与 `$`。环境变量 SHALL 支持按行添加/删除，名称 SHALL 使用与 `session.ValidateEnv` 相同的规则校验并拒绝 `WEBMUX_SESSION_ID`，值 SHALL 默认遮挡并提供可访问名称明确的显示/隐藏控制。前端校验 SHALL 与服务端的数量与长度限制一致。

界面 SHALL 提示启动配置保存的是创建时的命令副本，修改或删除 TOML 预设不会更新已保存配置。列表与命令预览 SHALL NOT 显示环境变量值，删除确认 SHALL NOT 包含值。

存储不可用时，设置页 SHALL 显示不可用原因与状态文件路径，禁用写入，并保持其他浏览器本地设置可用。保存冲突 SHALL 保留用户输入：重名显示 409 原因，存储不可用显示 503 原因。

#### Scenario: 创建带环境变量的配置

- **WHEN** 用户在设置页填写名称、命令 `claude --resume` 和一条环境变量并保存
- **THEN** 列表出现该配置，环境变量值默认遮挡，数量正确，命令预览不包含值

#### Scenario: 命令往返

- **WHEN** 用户编辑的命令包含空格、单双引号、反斜杠、`$` 或空参数
- **THEN** 用 `joinArgs` 渲染的文本再次用 `parseArgs` 解析得到完全相同的 argv

#### Scenario: 删除与空状态

- **WHEN** 用户确认删除最后一条配置
- **THEN** 配置被删除，列表显示空状态与新建入口

#### Scenario: 存储损坏

- **WHEN** 启动配置文件损坏后用户打开设置页
- **THEN** 卡片显示存储不可用及文件路径，新建、编辑与删除不可用，快捷指令与输出预览仍可修改

### Requirement: 新建会话页使用启动配置

新建会话页 SHALL 以服务端启动配置为主路径：用户选择配置后，提交 SHALL 发送 `configId` 与本次选择的 `cwd` 或 `workspace`，可选发送 `name`；SHALL NOT 同时发送 `preset`、`cmd`、`args` 或 `env`。

页面 SHALL 只把上次选择的配置 ID 保存在当前浏览器，SHALL NOT 缓存配置内容或把它当作服务端数据。首次加载时若存在旧的 `lastPreset`，SHALL 优先选中同名的种子配置；否则选中第一条可用配置。

页面 SHALL 保留折叠的“临时命令（不保存）”入口，继续使用现有预设、追加参数与自定义命令，且不写入服务端。没有可用配置时 SHALL 显示空状态与前往 `/settings#launch-configs` 的链接；存储不可用时 SHALL 说明原因并保留临时命令回退。加载配置期间 SHALL 禁用提交，避免发送空的 `configId`。

#### Scenario: 从配置启动

- **WHEN** 用户选择名为“Claude 恢复”的配置与目录后提交
- **THEN** 请求只包含 `configId`、目标目录与会话名，会话按该配置保存的 argv 与 env 启动

#### Scenario: 存储不可用时回退

- **WHEN** 启动配置接口返回 503，用户仍想启动自定义命令
- **THEN** 页面显示存储不可用提示，展开的临时命令入口仍可使用现有创建路径

#### Scenario: 无配置引导

- **WHEN** 服务端没有启动配置
- **THEN** 页面显示空状态和前往设置页创建配置的链接，不自动创建配置

### Requirement: 设备间共享与本地选择边界

启动配置 SHALL 由服务端存储并在同一实例的所有已认证浏览器与设备间共享。浏览器 SHALL 只保存上次选择的配置 ID 和临时命令内容；SHALL NOT 在本地保存另一份可编辑的配置列表。

配置更新与删除 SHALL 在下一次读取时反映到其他设备；本期 SHALL NOT 提供实时推送、冲突合并或并发编辑提示。删除配置 SHALL NOT 影响已运行会话，也 SHALL NOT 自动切换其他浏览器中已失效的上次选择。

#### Scenario: 跨设备复用

- **WHEN** 用户在桌面浏览器创建启动配置，随后在手机上打开同一实例的新建会话页
- **THEN** 手机读取到该配置并可选择启动

#### Scenario: 删除后其他设备刷新

- **WHEN** 一个设备删除配置，另一个设备重新打开新建会话页
- **THEN** 已删除配置不再出现；其他设备之前选中的 ID 若失效，则回退到第一条可用配置或空状态
