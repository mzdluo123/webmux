## Purpose

为 `webmux` 提供面向客户端的服务控制命令：在不改变现有启动方式的前提下，以稳定的退出码和带版本号的 JSON 契约报告服务状态与访问链接，并能安全地停止本机实例，供脚本、托盘及后续 CLI 客户端复用。

## ADDED Requirements

### Requirement: 子命令分发与兼容

`webmux` SHALL 按第一个参数分发：

- 无参数，或第一个参数以 `-` 开头：启动服务，行为与本变更前完全相同；
- `serve`：启动服务，接受与无子命令时相同的参数，并额外接受 `--log-file`；
- `version`：输出版本，行为不变；
- `status`、`stop`：见下文；
- 其他值：向标准错误输出“未知子命令”与可用子命令列表，退出码 2，SHALL NOT 启动服务。

客户端子命令 SHALL 接受 `--config <path>`，配置解析规则（默认路径、缺失文件、显式路径不存在即失败、默认值）SHALL 与服务启动完全一致。

#### Scenario: 现有 systemd 模板不受影响

- **WHEN** 以 `webmux --config ~/.config/webmux/config.toml` 启动（现有 `deploy/webmux.service` 的写法）
- **THEN** 服务正常启动，与本变更前行为一致

#### Scenario: 未知子命令

- **WHEN** 用户执行 `webmux statsu`
- **THEN** 标准错误提示未知子命令及可用列表，退出码 2，没有进程监听端口

### Requirement: 运行状态判定

`webmux status` SHALL 依据配置所指状态目录中的运行时记录定位实例，并 SHALL 同时满足以下条件才判定为“运行中”：记录存在且格式受支持、记录中的 PID 存活、该进程的启动标识与记录一致。随后 SHALL 用记录中的实际地址发起健康检查（配置了 Token 时携带认证），健康检查单次超时 SHALL NOT 超过 3 秒。

没有有效记录时，`status` SHALL 再对配置中的监听地址做一次健康检查，以识别没有写入记录的实例（例如旧版本或同一状态目录的第二个实例），并将其报告为“运行中、无记录”。

`status` SHALL NOT 修改任何文件，包括失效的记录。

#### Scenario: 运行中

- **WHEN** WebMux 以默认配置运行且有 2 个会话，用户执行 `webmux status`
- **THEN** 输出运行中、2 个会话、版本、PID、启动方与本机链接，退出码 0

#### Scenario: 未运行

- **WHEN** 没有 WebMux 在运行且没有遗留记录
- **THEN** 输出未运行，退出码 3

#### Scenario: 记录有效但健康检查失败

- **WHEN** 记录指向的 WebMux 进程存活，但健康检查超时
- **THEN** 输出“进程存在但无响应”及原因，退出码 1

### Requirement: 状态 JSON 契约

`webmux status --json` SHALL 向标准输出写入恰好一个 JSON 对象，错误诊断只写标准错误。对象 SHALL 包含字段 `v`（契约版本，本变更为 `1`）、`state`（`running` / `stopped` / `unresponsive` / `error`）、`record`（是否来自有效运行时记录）、`pid`、`version`、`launcher`、`listen`、`sessions`（健康时的会话数，否则为 `null`）、`health`（`ok` / `unauthorized` / `refused` / `timeout` / `not-webmux` / `error` 之一及可选 `message`）、`urls`（`local` 字符串与 `remote` 数组）、`tokenConfigured`、`drift`（配置与运行实例不一致的字段名数组，取值限于 `listen`、`token`）、`config`（`path`、`exists`、`error`）、`logFile`。

字段语义在同一契约版本内 SHALL 保持向后兼容：只增加可选字段，不删除或改变既有字段含义；不兼容变更 SHALL 提升 `v`。

只有传入 `--reveal-token` 且配置了 Token 时，对象 SHALL 额外包含 `token` 字段；其他任何输出（含人类可读输出与标准错误） SHALL NOT 包含 Token。

#### Scenario: 默认不泄露 Token

- **WHEN** 配置了 Token，用户执行 `webmux status --json`
- **THEN** 输出含 `"tokenConfigured": true`，不含 `token` 字段，也不含 Token 字符串

#### Scenario: 配置已修改但未重启

- **WHEN** 运行中实例监听 `127.0.0.1:7681`，用户把配置改成 `127.0.0.1:7700` 且尚未重启
- **THEN** `drift` 为 `["listen"]`，`listen` 与 `urls` 仍对应运行中实例的 `127.0.0.1:7681`

### Requirement: 链接推导

`status` 报告的链接 SHALL 使用 `http`、不带路径、查询参数或 Token，端口取实际监听端口：

- 本机链接：监听回环地址或 `localhost` 时主机为 `127.0.0.1`（IPv6 回环为 `[::1]`）；监听通配地址（`0.0.0.0`、`::` 或空主机）时为 `127.0.0.1`；监听具体非回环地址时为该地址。
- 远程链接：监听具体非回环地址时为该地址一项；监听通配地址时为本机所有处于启用状态的非回环、非链路本地 IPv4 地址，每项标注网卡名与类型（`tailscale`（100.64.0.0/10）或 `lan`），Tailscale 排在最前；监听回环地址时为空数组。在 WSL 的 NAT 网络模式下，WSL 虚拟网卡的地址 SHALL NOT 作为远程候选（局域网设备无法直接访问）。
- 监听通配地址且未配置 Token 时，`status` SHALL 给出暴露警告（JSON 中为 `warnings` 数组的一项）。

#### Scenario: 通配监听

- **WHEN** `listen = "0.0.0.0:7681"`，主机有 `tailscale0` 100.64.0.5 与 `eth0` 192.168.1.20
- **THEN** 本机链接为 `http://127.0.0.1:7681`，远程链接依次为 `http://100.64.0.5:7681`（tailscale）与 `http://192.168.1.20:7681`（lan）

#### Scenario: 回环监听

- **WHEN** `listen = "127.0.0.1:7681"`
- **THEN** 远程链接为空数组

### Requirement: 停止服务

`webmux stop` SHALL 只对通过“运行状态判定”校验的实例操作：发送 SIGTERM，等待其退出（默认 15 秒，可用 `--timeout` 调整），超时后发送 SIGKILL 并报告“已强制结束”。没有有效记录时 SHALL NOT 向任何进程发送信号；没有运行中实例时 SHALL 报告“未运行”并以退出码 0 结束（幂等）。

记录的启动方为 `systemd` 或 `launchd` 时，`stop` 默认 SHALL 拒绝并提示对应的管理命令（例如 `systemctl --user stop webmux`），退出码 1；传入 `--force` 时照常停止。

`stop --json` SHALL 输出一个 JSON 对象，包含 `v`、`result`（`stopped` / `killed` / `not-running` / `refused` / `error`）与可选 `message`。

#### Scenario: 正常停止

- **WHEN** 由用户手动启动的 WebMux 有 3 个会话，执行 `webmux stop`
- **THEN** WebMux 清理会话后退出，命令输出已停止，退出码 0，运行时记录被删除

#### Scenario: systemd 实例

- **WHEN** WebMux 由 systemd 用户服务启动，执行 `webmux stop`
- **THEN** 命令拒绝停止并提示 `systemctl --user stop webmux`，进程继续运行

#### Scenario: 无记录时不误杀

- **WHEN** 配置端口上有一个未写记录的 WebMux（旧版本），执行 `webmux stop`
- **THEN** 命令报告找不到可停止的实例，不向任何进程发送信号，退出码 1
