## Context

动机见 proposal.md。相关现状：

- `cmd/webmux/main.go` 只在 `os.Args[1] == "version"` 时分流，其余参数全部交给 `flag.NewFlagSet("webmux")` 作为服务参数；`loadConfig` 已封装“默认路径 / 显式路径必须存在 / 命令行覆盖 / Validate”的完整规则，可直接复用。
- `internal/workspace/registry.go` 已在 `state_dir` 中用 `flock(LOCK_EX|LOCK_NB)` 保证“一个状态目录一个实例”，冲突时只禁用仓库功能、服务照常运行——运行时记录沿用同一取舍。
- 进程信息读取已按平台拆分：`internal/session/proc_linux.go` 读 `/proc/<pid>/stat`，`proc_darwin.go` 用 sysctl / `proc_pidinfo`。进程启动标识（Linux `stat` 第 22 字段 starttime，macOS `kinfo_proc.p_starttime`）可在同样的拆分下取得。
- `internal/session/env.go` 的 `sessionEnv` 已有“不让宿主标识变量进入会话”的过滤表，`WEBMUX_LAUNCHER` 加入同一表即可。
- `/api/health` 返回 `{"ok":true,"sessions":N}`，配置 Token 时需要 Bearer；`hostCheck` 允许以 `127.0.0.1` 访问回环/通配监听实例。

### 相关 issue 的共同需求

| 需求来源 | 需要什么 | 与本变更的关系 |
|---|---|---|
| `add-windows-wsl-tray` | 判断运行状态、取链接、安全停止、知道日志在哪；在 WSL 内由 Windows 进程驱动 | 直接消费 `status --json` / `stop --json` / `serve --log-file` |
| #67 `webmux run/ls/attach/open` | 找到服务地址（通配 → `127.0.0.1`）、读取 Token、打印会话链接 | `open` 的链接推导与 `status` 同一函数；HTTP/WS 客户端加入 `internal/client` |
| #68 `webmux events/wait`（及手机推送） | 订阅 `/ws/sessions`、重连、差分去重 | 订阅客户端放入 `internal/client`；推送在服务端，不依赖本变更 |
| #69 下载文件 | “为脚本生成需要现有认证的下载入口” | 将来可复用带认证的 HTTP 客户端 |
| #76 多节点（wontfix） | — | 边界：客户端只连接一个服务，不做节点发现 |

## Goals / Non-Goals

**Goals:**

- 把“服务定位、状态、停止、链接推导”收敛到 `webmux` 二进制，成为唯一权威；托盘、脚本与后续 CLI 都是它的客户端。
- 不新增网络暴露面：控制能力只通过本机进程信号与文件实现，HTTP/WS 契约不变。
- 现有启动方式（无子命令 + 参数）零改动可用。

**Non-Goals:**

- 拆分会话宿主与 HTTP 前端为两个进程、服务升级不丢会话（见决策 1）。
- 本机控制套接字、HTTP 管理接口（见决策 1）。
- #67 的 `run/ls/attach/open` 与 #68 的 `events/wait` 本身——本变更只提供它们复用的核心与子命令框架。
- Windows 原生运行 WebMux。

## Decisions

### 1. 解耦到什么程度

评估了四个层次：

| 方案 | 做法 | 收益 | 代价 | 结论 |
|---|---|---|---|---|
| A. 各客户端自理 | 托盘自己解析 TOML、写 shell 包装脚本管 PID | 无服务端改动 | 默认值与解析规则多处复制；#67/#68 再写一遍；包装脚本难测、易违反 argv 约定 | 否决 |
| **B. 共享客户端核心 + 本机控制 CLI + 运行时记录** | `internal/client` 被 `webmux` 子命令与托盘共用；服务写运行时记录；`status/stop` 输出版本化 JSON | 单一权威；控制走本机信号，不依赖 HTTP 是否可达（服务卡死时也能停）；契约可测、可演进 | 少量服务端改动（记录、`--log-file`、子命令分发） | **采用** |
| C. 本机控制套接字 | 服务监听 `state_dir/run/control.sock`，CLI 免 Token 管理 | 本机免 Token；可加 reload 等管理操作 | 新的协议面与权限模型；WSL 2 的 Unix 套接字不与 Windows 互通，托盘仍要经 `wsl.exe` | 暂缓；#67 若需要免 Token attach 再评估 |
| D. 会话守护进程 + 前端进程 | `webmuxd` 持有 PTY，HTTP 层独立进程经 IPC 连接 | 升级 HTTP 层不丢会话、多前端 | 回放捕获与订阅注册共享锁的无缝续接保证要跨进程重做；两套生命周期；与“服务重启即结束会话”的产品边界冲突；#76 已表明不走分布式复杂度 | 否决；仅当“升级不丢会话”成为明确需求时重开 |

另一个被否决的选项是 HTTP `POST /api/shutdown`：持 Token 者本来就能在 shell 里 `kill`，所以不算提权；但服务无响应时 HTTP 停止也失效，而这正是最需要“停止”的时刻，且会扩大浏览器侧 CSRF/Origin 面。

### 2. 分层与包结构

```
cmd/webmux/          子命令分发：serve(默认) / version / status / stop   [#67/#68 追加 run ls attach open events wait]
internal/client/     平台无关：Locate(config) → Target{listen, token, record}; Links(); Health(); Status()
internal/runtime/    记录读写、flock、原子替换；ProcessIdentity(pid) 分 _linux.go / _darwin.go
```

`internal/client` 不依赖 `internal/api`/`internal/session` 的运行期对象，只依赖 `internal/config` 的解析与默认值；JSON 契约类型（`StatusV1`、`StopV1`）定义在 `internal/client`，托盘直接 import 同一类型反序列化，避免手写重复结构。

### 3. 运行时记录

- 位置：`<state_dir>/run/server.json`，锁 `<state_dir>/run/server.lock`。放在状态目录而非 `$XDG_RUNTIME_DIR`：后者在未启用 systemd 的 WSL 与 macOS 上常不存在；状态目录已有“一目录一实例”的语义。
- 写入时机：`net.Listen` 成功后、`server.Serve` 前；删除时机：`m.Shutdown` 与 `server.Shutdown` 之后（`run()` 末尾的 defer）。
- 进程身份：`(pid, startIdentity)`；读取方比较二者，并在 Linux 上额外比较 `/proc/<pid>/exe` 与记录中的可执行路径。macOS 无法读取其他进程环境，但同一用户的 `proc_pidinfo` 可取启动时间，足够区分 PID 复用。
- Token 指纹：`sha256("webmux-token-fingerprint\x00" + token)` 的前 16 个十六进制字符。记录文件与配置文件同为 0600，指纹不提供额外泄露面，只用于 `drift` 判定。
- 启动方：`WEBMUX_LAUNCHER=windows-tray` > `INVOCATION_ID`（systemd）> `XPC_SERVICE_NAME` 非空且非 `0`（launchd）> `manual`。

### 4. `--log-file`

服务启动时（解析参数后、监听前）打开日志文件，把 fd 1、2 用 `dup3`（Linux，arm64 无 `dup2`）/`dup2`（darwin）指向它，并重建 `slog` 默认 handler。这样 Go 运行时的 panic 输出也落盘，且父进程关闭管道后不会触发 SIGPIPE 退出。轮转只做“启动时 rename 为 `.1`”，不做运行中按大小轮转（运行期日志量很小，复杂度不值得）。

### 5. 退出码

参照 LSB `status` 约定：`0` 运行中且健康 / 操作成功；`3` 未运行（仅 `status`）；`1` 其他错误（含无响应、拒绝停止）；`2` 用法错误。

## Risks / Trade-offs

- [第一个参数原本可能是被误写的位置参数，例如 `webmux foo` 过去会因为 `flag` 解析结束而照常启动] → 现在改为报错退出码 2。这是更安全的行为；`deploy/` 模板与文档写法都以 `-` 开头，不受影响。
- [旧版本服务没有运行时记录] → `status` 用健康检查兜底报告“运行中、无记录”；`stop` 拒绝操作并说明原因。
- [同一状态目录多实例] → 与仓库注册表一致：只有持锁实例写记录，另一实例日志警告。
- [`/proc/<pid>/exe` 在二进制被替换（升级）后显示 `(deleted)`] → 比较时去掉 ` (deleted)` 后缀，并以启动标识为主判据。

## Migration Plan

纯新增；不需要迁移。升级后第一次启动即写记录。回滚到旧版本时遗留的 `run/` 目录无害。
