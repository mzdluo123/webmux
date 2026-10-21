## Why

`webmux` 目前只有“启动服务”和 `version` 两种用法，外部程序要判断“服务是否在运行、实际监听在哪、该打开哪个链接、怎样安全地停止它”，只能各自解析 TOML、猜默认值、按端口或进程名去找 PID。Windows 托盘（`add-windows-wsl-tray`）、原生终端入口（#67 `webmux run/ls/attach/open`）和事件出口（#68 `webmux events/wait`）都需要这一层，而它们只是同一个服务的不同客户端。先确立客户端/服务端的分工与通讯方案（数据面沿用现有 HTTP/WS，本机控制面为 argv + 版本化 JSON），再把“定位服务、查询状态、停止服务、检查与初始化配置”做成 `webmux` 自身的客户端能力，后续客户端复用同一个核心，不再各写一遍。

## What Changes

- 确立客户端/服务端架构（design.md 决策 0）：服务端是会话、认证与配置解析的唯一权威；会话操作只走现有 HTTP/WS 数据面；启动、停止、状态与配置检查走不监听端口的本机控制面（子命令 argv + 标准输出 JSON + 运行时记录 + 信号），Windows 托盘经 `wsl.exe --exec` 使用同一控制面。
- `webmux` 增加子命令分发：无子命令或以 `-` 开头的参数保持现有“启动服务”行为（**完全兼容**现有 systemd/launchd 模板与脚本）；新增显式 `serve`（与无子命令等价）、`status`、`stop`、`config`；`version` 保持不变。未知子命令报错退出，不再被当成服务启动参数。
- 服务端在成功监听后写入**运行时记录**（状态目录内，0600），包含 PID、进程启动标识、版本、实际监听地址、配置路径、Token 指纹（不含 Token 本身）、启动方（systemd / launchd / windows-tray / manual）与日志路径；正常退出时删除。不新增任何 HTTP 或 WebSocket 接口。
- `webmux serve` 新增 `--log-file <path>`：服务输出写入文件（启动时保留上一份为 `.1`），用于没有日志收集器的后台启动方式；`--config` 与 `--log-file` 支持 `~/` 前缀。
- `webmux status [--json] [--reveal-token]`：按与服务端相同的规则解析配置，读取并校验运行时记录（防 PID 复用），对实际地址做健康检查，输出运行状态、会话数、版本、启动方、本机链接与远程链接候选、配置与运行实例不一致项；Token 仅在 `--reveal-token` 时输出。退出码区分“运行中 / 未运行 / 错误”。
- `webmux stop [--timeout] [--json]`：对运行时记录指向的实例发送 SIGTERM，超时后强制结束；由 systemd/launchd 管理的实例默认拒绝并提示对应命令（`--force` 覆盖）；未运行时幂等成功。
- `webmux config path|show|validate|init`：定位、查看生效配置（不含 Token）、校验、以随机 Token 初始化 0600 配置文件；不提供以程序方式改写已有配置的命令。
- 新增共享客户端核心（Go 包）：服务定位、链接推导、带认证的健康检查与状态组装。#67/#68 的 HTTP/WS 客户端后续加入同一包。

## Capabilities

### New Capabilities

- `server-runtime-record`: 服务端运行时记录的内容、位置、权限、生命周期、单实例约束，以及 `--log-file` 输出重定向。
- `cli-server-control`: `webmux` 子命令分发与兼容性、`status`/`stop` 的行为与 JSON 契约（v1）、链接推导规则、退出码。
- `cli-config`: `webmux config` 的定位、生效配置查看、校验与初始化，以及“客户端不改写已有配置”的约束。

### Modified Capabilities

当前 `openspec/specs/` 无已登记主规格。服务启动参数、配置文件格式、HTTP/WebSocket 契约、认证与 Origin/Host 校验均不变。

## Impact

- `cmd/webmux/main.go`：子命令分发；`serve` 增加 `--log-file`，监听成功后写运行时记录、退出时删除。
- 新增 `internal/contract/`（无依赖、可交叉编译到 Windows 的契约类型与健康分类）、`internal/client/`（定位、链接推导、健康检查、状态组装）与 `internal/runtime/`（记录读写、文件锁、进程身份校验；进程启动标识读取分 `_linux.go`/`_darwin.go`）。
- `internal/session/env.go`：会话环境不继承启动方标识变量 `WEBMUX_LAUNCHER`。
- 新增 `deploy/embed.go` 嵌入 `deploy/config.example.toml`，作为 `config init` 模板；`deploy/webmux.service` 与 `deploy/com.webmux.plist` 无需修改；`docs/deployment.md`、`docs/protocol.md` 增加 `status`/`stop` 与 JSON 契约说明。
- 不新增第三方依赖。
- 后续依赖方：`add-windows-wsl-tray`（本变更为其前置），#67、#68 的 CLI 子命令。
