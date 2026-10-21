## Context

动机见 proposal.md。相关现状：

- `cmd/webmux/main.go` 只在 `os.Args[1] == "version"` 时分流，其余参数全部交给 `flag.NewFlagSet("webmux")` 作为服务参数；`loadConfig` 已封装“默认路径 / 显式路径必须存在 / 命令行覆盖 / Validate”的完整规则，可直接复用。
- `internal/workspace/registry.go` 已在 `state_dir` 中用 `flock(LOCK_EX|LOCK_NB)` 保证“一个状态目录一个实例”，冲突时只禁用仓库功能、服务照常运行——运行时记录沿用同一取舍。
- 进程信息读取已按平台拆分：`internal/session/proc_linux.go` 读 `/proc/<pid>/stat`，`proc_darwin.go` 用 sysctl / `proc_pidinfo`。进程启动标识（Linux `stat` 第 22 字段 starttime，macOS `kinfo_proc.p_starttime`）可在同样的拆分下取得。
- `internal/session/env.go` 的 `sessionEnv` 已有“不让宿主标识变量进入会话”的过滤表，`WEBMUX_LAUNCHER` 加入同一表即可。
- 认证：`/api/*`、`/ws/*` 接受 `Authorization: Bearer`、`webmux_token` Cookie，`/ws/*` 另接受 `?token=`；`hostCheck` 与 `originAllowed` 在 `internal/api` 内实现。会话创建 `POST /api/sessions` 已支持 `preset: "custom"` + `cmd` argv + `cwd` + `name`。
- 配置只在启动时读取一次，没有热重载；修改配置必须重启，而重启会结束所有会话。

## Goals / Non-Goals

**Goals:**

- 确立 WebMux 的客户端/服务端分工与通讯方案（决策 0），本变更实现其中的公共基础。
- 把“服务定位、状态、停止、链接推导、配置检查与初始化”收敛到 `webmux` 二进制，成为唯一权威；托盘、脚本与后续 CLI 都是它的客户端。
- 不新增网络暴露面：控制能力只通过本机进程、文件与信号实现，HTTP/WS 契约不变。
- 现有启动方式（无子命令 + 参数）零改动可用。

**Non-Goals:**

- 拆分会话宿主与 HTTP 前端为两个进程、服务升级不丢会话。
- 本机控制套接字、HTTP 管理接口、配置热重载。
- 以程序方式修改配置项（`config set`）。
- #67 的 `run/ls/attach/open` 与 #68 的 `events/wait` 本身——本变更只提供它们复用的核心与子命令框架。
- Windows 原生运行 WebMux 服务。

## Decisions

### 0. 整体架构：客户端/服务端分工与通讯方案

#### 0.1 角色

| 角色 | 形态 | 职责 | 不负责 |
|---|---|---|---|
| **服务端** | Linux/macOS `webmux serve` | 唯一持有 PTY、会话状态、回放缓冲；唯一的认证与 Host/Origin 判定者；解析并校验配置 | 进程托管方式（交给 systemd/launchd/托盘/手动） |
| **本机客户端（CLI）** | 同一个 `webmux` 二进制的子命令，运行在服务端所在机器（含 WSL 内） | 启动与管理服务（`serve/status/stop`）、配置检查与初始化（`config`）、在终端中启动 agent 并纳入服务端管理（#67 `run/attach/ls/open`）、事件出口（#68 `events/wait`） | 持有会话状态；代替服务端做认证 |
| **Windows 客户端（托盘）** | `webmux-tray.exe` | 在 WSL 中启动与管理服务、提供链接与常用操作、WSL 网络特殊情况下的转发 | 直接解析服务端配置或进程；在非转发场景中转终端数据 |
| **浏览器** | 现有前端 | 终端交互 | — |

#### 0.2 两个通讯平面

```
              数据面：HTTP REST + WebSocket（Token 认证，Host/Origin 校验）
 浏览器 ───────────────────────────────────────────────┐
 CLI run/attach/ls/events（#67/#68）──────────────────┤
 托盘可达性检查 ──(WSL localhost 转发)────────────────┤──▶ webmux serve
 手机/局域网 ──▶ 托盘转发（仅 WSL NAT 特殊情况）──────┘          ▲
                                                                  │ 运行时记录 + 信号
              本机控制面：argv 调用 + JSON 标准输出（契约带 v）     │
 CLI status/stop/config ─────────────────────────────────────────┘
 托盘 ──▶ wsl.exe --exec webmux status|stop|config|serve ────────┘
```

- **数据面**是唯一的会话操作通道：创建会话、附着终端、订阅列表与事件都走现有 REST/WS 与 `docs/protocol.md` 的帧格式，CLI 与浏览器是同级客户端，权限完全相同。不引入第二套会话协议。
- **本机控制面**只做“服务之外”的事：服务未运行或无响应时也必须可用的启动、停止、状态、配置检查。它的传输是进程边界本身——本机直接执行子命令，Windows 侧经 `wsl.exe --exec` 执行；协议是 argv 入参 + 标准输出上的单个 JSON 对象（字段 `v` 为契约版本）+ 退出码。服务端通过运行时记录与信号参与，不监听任何控制端口。
- **转发**是数据面的传输扩展，不是新协议：只在 WSL NAT 模式下局域网设备无法直达 WSL 时由托盘提供（见 `add-windows-wsl-tray`）。

#### 0.3 对候选方案的取舍

| 方案 | 结论 | 理由 |
|---|---|---|
| A. 各客户端自理（各自解析 TOML、管 PID） | 否决 | 默认值与规则多处复制；托盘、#67、#68 各写一遍 |
| **B. 共享客户端核心 + 本机控制面 + 运行时记录** | **采用** | 单一权威；停止不依赖 HTTP 可达；契约可测、可演进 |
| C. 本机控制套接字（Unix socket） | 暂缓 | 服务未运行时不能用于“启动”；WSL 2 的 Unix 套接字不与 Windows 互通，托盘仍要经 `wsl.exe`；若 #67 需要免 Token 的本机附着再评估 |
| D. 会话守护进程 + 独立 HTTP 前端 | 否决 | 回放捕获与订阅注册共享锁的无缝续接保证要跨进程重做；违背“服务重启即结束会话”的边界；#76 已表明不引入分布式复杂度 |
| E. `POST /api/shutdown` 等 HTTP 管理接口 | 否决 | 服务无响应时恰好不可用；扩大浏览器侧 CSRF/Origin 面 |
| F. WSL 内常驻桥接进程（托盘经 stdio JSON-RPC 通讯） | 暂缓 | 能推送状态、减少 `wsl.exe` 启动次数，但多一份需要重同步的状态与协议；按需启动的 `wsl.exe --exec` 对 10–30 秒一次的频率已足够 |

#### 0.4 配置的归属

服务端配置文件由用户拥有、由服务端解析；客户端只做“定位、检查、初始化、打开编辑器”，不以程序方式改写其中的字段（BurntSushi/toml 无法保留注释与格式，逐行改写易损坏手写配置）。客户端自身的设置（例如托盘的发行版、启动方式、转发开关）保存在客户端自己的文件中。配置修改后的生效方式仍是重启服务，`status` 的 `drift` 负责提示“已修改未生效”。

#### 0.5 后续变更的落点

| 变更 | 平面 | 依赖本变更的部分 |
|---|---|---|
| `add-windows-wsl-tray` | 控制面（经 `wsl.exe`）+ 数据面可达性 + 转发 | `status/stop/config` 契约、`serve --log-file`、`WEBMUX_LAUNCHER` |
| #67 终端启动与附着 agent | 数据面：`POST /api/sessions`（`preset: "custom"` + argv + 调用方 cwd，新增可选 `env`）+ `/ws/s/{id}` 帧 | `internal/client` 的服务定位与认证；子命令框架 |
| #68 事件出口 | 数据面：`/ws/sessions` 订阅 + 客户端差分 | 同上 |
| Windows 控制台客户端 `webmux.exe`（未排期） | 数据面经 localhost 转发，控制面经 `wsl.exe` | `internal/contract`；届时把数据面客户端拆成不依赖 `internal/session` 的子包 |

### 1. 分层与包结构

```
cmd/webmux/          子命令分发：serve(默认) / version / status / stop / config   [#67/#68 追加 run ls attach open events wait]
internal/contract/   无依赖叶子包：本机控制面契约类型 StatusV1 / StopV1 / ConfigV1 与 Health 分类常量
internal/client/     Locate(config) → Target{listen, token, record}; Links(); Health(); Status()
internal/runtime/    记录读写、flock、原子替换；ProcessIdentity(pid) 分 _linux.go / _darwin.go
deploy/embed.go      以 go:embed 暴露 config.example.toml（与 web/embed.go 同一做法），供 config init 使用
```

`internal/client` 依赖 `internal/config` 的解析与默认值，而 `internal/config` 依赖只支持 Unix 的 `internal/session`，所以 `internal/client` 不交叉编译到 Windows。托盘只 import 无依赖的 `internal/contract` 反序列化 `wsl.exe` 返回的 JSON；可达性检查的 HTTP 分类逻辑也放在 `internal/contract`（只用标准库），供两侧共用。将来 Windows 控制台客户端需要数据面客户端时，再把 HTTP/WS 部分拆到不依赖 `internal/session` 的子包。

### 2. 运行时记录

- 位置：`<state_dir>/run/server.json`，锁 `<state_dir>/run/server.lock`。放在状态目录而非 `$XDG_RUNTIME_DIR`：后者在未启用 systemd 的 WSL 与 macOS 上常不存在；状态目录已有“一目录一实例”的语义。
- 写入时机：`net.Listen` 成功后、`server.Serve` 前；删除时机：`m.Shutdown` 与 `server.Shutdown` 之后（`run()` 末尾的 defer）。
- 进程身份：`(pid, startIdentity)`；读取方比较二者，并在 Linux 上额外比较 `/proc/<pid>/exe` 与记录中的可执行路径（去掉 ` (deleted)` 后缀）。
- Token 指纹：`sha256("webmux-token-fingerprint\x00" + token)` 的前 16 个十六进制字符。记录文件与配置文件同为 0600，指纹不提供额外泄露面，只用于 `drift` 判定。
- 启动方：`WEBMUX_LAUNCHER=windows-tray` > `INVOCATION_ID`（systemd）> `XPC_SERVICE_NAME` 非空且非 `0`（launchd）> `manual`。

### 3. `--log-file`

服务启动时（解析参数后、监听前）打开日志文件，把 fd 1、2 用 `dup3`（Linux，arm64 无 `dup2`）/`dup2`（darwin）指向它，并重建 `slog` 默认 handler。这样 Go 运行时的 panic 输出也落盘，且父进程关闭管道后不会触发 SIGPIPE 退出。轮转只做“启动时 rename 为 `.1`”。

### 4. `config init` 的 Token

32 字节 `crypto/rand` 随机数的 base64url（无填充）编码。生成的文件以 `deploy/config.example.toml` 为模板（经 `deploy/embed.go` 编译期嵌入），只替换 `token` 一行；`--listen` 指定时同时替换 `listen`。目标已存在时拒绝；`--force` 先把旧文件改名为 `.bak`。

### 5. 退出码

参照 LSB `status` 约定：`0` 运行中且健康 / 操作成功；`3` 未运行（仅 `status`）；`1` 其他错误（含无响应、拒绝停止、配置无效）；`2` 用法错误。

## Risks / Trade-offs

- [第一个参数原本可能是被误写的位置参数，例如 `webmux foo` 过去会照常启动] → 现在改为报错退出码 2，已与用户确认；`deploy/` 模板与文档写法都以 `-` 开头，不受影响。
- [旧版本服务没有运行时记录] → `status` 用健康检查兜底报告“运行中、无记录”；`stop` 拒绝操作并说明原因。
- [同一状态目录多实例] → 与仓库注册表一致：只有持锁实例写记录，另一实例日志警告。
- [配置不能热更新，改配置要重启并丢会话] → 本变更只提示漂移，不引入热重载；若后续需要，可为 `allowed_origins`、`prompt_patterns` 等安全子集增加 SIGHUP 重载，另立变更。

## Migration Plan

纯新增；不需要迁移。升级后第一次启动即写记录。回滚到旧版本时遗留的 `run/` 目录无害。
