## Context

动机见 proposal.md。与实现相关的现状：

- 服务端只支持 Linux/macOS：`internal/session` 的进程表与清理按 `_linux.go`/`_darwin.go` 拆分，`_other.go` 只是能编译的占位；`Makefile` 的 `build`/`release` 固定 `CGO_ENABLED=0`，`release` 只产出 linux/darwin 四个目标。Windows 用户的现实路径就是在 WSL 2 里运行 Linux 版 `webmux`。
- 启动参数：`webmux --config <path> --listen <addr> --token <t>`，`webmux version` 输出版本；配置默认 `~/.config/webmux/config.toml`，缺失时用默认值，显式 `--config` 指向不存在的文件会启动失败（`internal/config/config.go`）。`listen` 默认 `127.0.0.1:7681`，监听通配地址且无 Token 时仅打 `SHELL EXPOSED` 警告。
- 退出：WebMux 处理 SIGINT/SIGTERM，先 `Manager.Shutdown` 清理会话（10 秒上限）再关闭 HTTP；systemd 示例用 `KillMode=mixed`、`TimeoutStopSec=15`。日志用 `slog` 写 stderr。
- 健康检查：`GET /api/health` → `{"ok":true,"sessions":N}`；`/api/*` 在配置 Token 时要求 `Authorization: Bearer`，否则 401；`hostCheck` 只允许 `listen` 主机、`allowed_origins` 主机和（回环/通配监听时的）回环/IP 字面量，所以 Windows 侧以 `127.0.0.1:<port>` 访问回环监听实例是被允许的。
- 前端登录：`App.svelte` 先调 `/health`，401 时显示 Token 表单，Token 存 `localStorage`（键 `webmux.token`）；目前不接受 URL 中的 Token。
- 依赖与构建：`go.mod` 为 Go 1.26，已依赖 `golang.org/x/sys`；CI（`.github/workflows/test.yml`）在 Ubuntu/macOS 上跑 `go test -race ./...`，没有 Windows 作业。

WSL 侧需要特别处理的事实：

- `wsl.exe --list` 默认输出 UTF-16LE；设置 `WSL_UTF8=1` 后输出 UTF-8（较新的 WSL 版本），两种都要能解析。
- 发行版在没有 `wsl.exe` 客户端进程连接时，会在空闲超时后被终止（即使启用了 systemd）；保持一个长期存在的 `wsl.exe` 子进程是最可靠的保活方式。
- Go 程序向已关闭的 fd 1/2 写入会因 SIGPIPE 退出。如果把 WebMux 的 stdout/stderr 接到托盘的管道，托盘退出后 WebMux 下一次写日志就会被杀死——日志必须在 WSL 内落盘。
- WSL 2 NAT 模式默认开启 localhostForwarding，Windows 可经 `127.0.0.1` 访问 WSL 内的回环/通配监听；mirrored 模式共享回环。用户关闭转发时 Windows 侧不可达，需要明确诊断。
- 从 Windows 创建 `wsl.exe` 时若不加 `CREATE_NO_WINDOW` 会闪出控制台窗口。

## Goals / Non-Goals

**Goals:**

- 不修改服务端 `webmux` 与前端，托盘只作为外部进程管理器与 HTTP 客户端；所有现有认证、Origin、Host 约束保持不变。
- 平台无关逻辑（设置解析、argv 构造、TOML 读取、链接推导、状态机、退避）放在可在 Linux CI 上 `go test -race` 的包里；Windows API 只出现在一层薄的 `//go:build windows` 适配层。
- `go build ./...`、`go vet ./...`、`go test ./...` 在 Linux/macOS 上继续通过；Windows 目标保持 `CGO_ENABLED=0`。
- 所有 WSL 调用以 argv 传参，用户路径不经 shell 求值。

**Non-Goals:**

- WebMux 原生运行在 Windows（ConPTY）——这是另一个独立的大变更。
- 安装器（MSI/MSIX/winget）、代码签名、自动更新；在托盘中下载或升级 WSL 内的 `webmux` 二进制。
- 托盘内的会话列表、会话级操作、“需要关注”通知（见待确认 Q6）。
- 图形化设置窗口（v1 用子菜单 + 文本编辑设置文件，见待确认 Q5）。
- 管理 WSL 本身（安装发行版、修改 `.wslconfig`）。
- 多实例 / 多发行版同时托管。

## Decisions

### 0. 待用户确认的决策

以下各项按“推荐”列编写了 specs 与 tasks；用户确认或改选后需同步更新。

| # | 问题 | 推荐（当前写法） | 备选 |
|---|---|---|---|
| Q1 | “开机启动”的含义 | Windows **用户登录后**启动（HKCU `Run` 启动项） | 计划任务“登录时”触发（可延迟、可设失败重试）；计划任务“开机时、无论是否登录”（无托盘图标、WSL 行为不稳定） |
| Q2 | WSL 内 WebMux 的托管方式 | **托盘直接托管**：`wsl.exe` 前台运行 WebMux，兼作保活；已有 systemd 实例时只接管监控 | 优先使用 `systemctl --user`（需 WSL 启用 systemd，托盘另起保活进程）；两种模式都实现并可在设置中切换 |
| Q3 | 托盘技术栈 | **Go + `fyne.io/systray` + `golang.org/x/sys/windows`**，同仓库同工具链、无 CGO | C#/.NET WinForms（原生对话框更好，但引入 .NET SDK 与第二套 CI）；Rust/Tauri（体积小但引入 Rust 工具链） |
| Q4 | 带 Token 的一键链接 | **不做**：链接不含 Token，Token 单独复制且排除剪贴板历史，前后端零改动 | 前端支持 `#token=...` 片段（不发往服务端、读取后从地址栏移除并存入 localStorage），托盘提供“复制带 Token 的链接”；或生成手机扫码二维码 |
| Q5 | 设置界面 | **菜单 + 文本文件**：发行版用子菜单单选，其余项用记事本编辑设置 JSON、保存即热加载 | 原生 Win32 设置窗口（`lxn/walk` 等，体积与复杂度明显上升）；在 WebMux 前端加“Windows 托盘”设置页（需前后端改动） |
| Q6 | v1 范围扩展 | **只做服务托管 + 链接 + 基础管理** | 托盘菜单列出会话并可直接打开；agent “需要关注”时 Windows 通知（订阅 `/ws/sessions`）；托盘内安装/升级 WSL 中的 `webmux` |
| Q7 | 交付形式 | **`make release` 产出 `webmux-tray-windows-{amd64,arm64}.exe`**，用户自行放置；暂无安装器与签名 | 同时产出 zip（含 exe + 说明 + linux 版 webmux）；后续再做 winget / 签名 |
| Q8 | 退出托盘时默认行为 | **询问**（仅退出托盘 / 停止并退出 / 取消） | 总是只退出托盘；总是停止 WebMux |

### 1. 代码组织

```
cmd/webmux-tray/
  main_windows.go      // 入口：-H=windowsgui，组装依赖
  main_other.go        // !windows：打印“仅支持 Windows”并退出 1，保证 go build ./... 通过
internal/tray/          // 平台无关，Linux CI 可测
  settings.go          // 设置结构、默认值、JSON 读写与校验
  wsl.go               // Runner 接口；wsl.exe argv 构造；发行版列表解析（UTF-16LE/UTF-8）
  webmuxconfig.go      // 读取 WebMux TOML 的 listen/token（复用 BurntSushi/toml，只取两个字段）
  links.go             // 本机/远程链接推导
  health.go            // /api/health 客户端（超时、Bearer、401/非 WebMux 响应分类）
  supervisor.go        // 状态机：Stopped/Starting/Running/External/Stopping/Failed，退避
internal/tray/winui/    // //go:build windows
  tray.go menu.go notify.go clipboard.go autostart.go process.go singleinstance.go
```

依赖注入沿用项目的显式构造方式：`main` 读取设置 → 构造 `wsl.Runner`（Windows 实现基于 `os/exec` + `SysProcAttr{HideWindow, CreationFlags: CREATE_NO_WINDOW}`）→ `health.Client` → `Supervisor` → `winui.Tray`。`Supervisor` 只通过接口访问 WSL、HTTP 与时钟，单元测试注入假实现。

备选：把托盘放到独立仓库或独立 Go module。否决：链接推导需要与 `internal/config` 的默认值/校验保持一致，同 module 可直接复用默认值常量并在 CI 中一起编译；托盘依赖只出现在 Windows 构建中，不进入 `webmux` 服务端二进制。

### 2. 在 WSL 中启动：固定包装脚本 + argv

托盘以如下 argv 启动（单行展示）：

```
wsl.exe [-d <distro>] --cd ~ --exec /bin/sh -c '<固定脚本>' webmux-tray <pidfile> <logfile> <webmux> --config <config>
```

固定脚本（常量，不拼接用户输入）负责：展开 `~/` 前缀（只处理参数开头的 `~/`）、创建状态目录、把上次日志轮转为 `.1`、写入 `$$` 到 pidfile、`exec "$webmux" "$@" >>"$log" 2>&1 </dev/null`。用户路径只作为位置参数出现，`"$@"` 原样传递，满足“argv 而非 shell 字符串”的约定。

- pidfile / 日志位置：`${XDG_STATE_HOME:-$HOME/.local/state}/webmux/tray/`，与 WebMux 状态目录同级，权限 0700。
- 该 `wsl.exe` 进程由托盘以 detached 方式创建（不放入随托盘关闭而终止的 Job Object），因此托盘退出后它继续存在，同时充当发行版保活进程。
- 启动前用 `wsl.exe --exec test -x <webmux>` 预检；预检失败为不可重试错误（不进入退避）。

备选：`systemctl --user start webmux`。否决为默认值的原因：要求用户在 `/etc/wsl.conf` 启用 systemd 并安装 unit，且仍需额外保活进程；但保留“检测到外部实例即接管监控”，兼容已经这么部署的用户（Q2）。
备选：给 `webmux` 增加 `--pid-file`/`--log-file` 参数。否决：为单一调用方扩大服务端 CLI 表面，包装脚本即可完成。

### 3. 识别、停止与接管

- 识别“本托盘启动的实例”：读取 pidfile → 校验 `/proc/<pid>/exe` 解析结果等于设置中 `webmux` 路径（展开后）且进程存活。全部在一个固定脚本中完成并以退出码区分（0 存活且匹配 / 1 不存在 / 2 PID 被复用），托盘重启后据此恢复控制。
- 停止：同一校验通过后 `kill -TERM <pid>`；轮询至多 15 秒（WebMux 内部 shutdown 上限 10 秒 + 余量，与 systemd 示例 `TimeoutStopSec=15` 一致），超时 `kill -KILL`，再结束 Windows 侧对应的 `wsl.exe` 进程。PID 校验失败时拒绝发送信号并报告。
- 外部实例：健康检查成功但 pidfile 校验失败 → `External` 状态，禁用停止/重启（不去猜测 systemd 或手动启动的管理方式）。
- 非 WebMux 占用端口：健康检查得到非 JSON、或 JSON 不含 `ok` 字段 → 端口冲突。

### 4. 健康检查与状态机

`health.Client` 使用独立的 `http.Client`（不读取系统代理，避免公司代理拦截 127.0.0.1），单次超时 3 秒，结果分类为：`ok(sessions)` / `unauthorized` / `refused` / `timeout` / `notWebMux` / `other(err)`。

状态机（`Supervisor`，单 goroutine 事件循环，对外只发布不可变快照，UI 层订阅渲染）：

```
Stopped ──start──▶ Starting ──health ok──▶ Running ──user stop──▶ Stopping ──exited──▶ Stopped
   ▲                  │ 60s 未就绪                │ 进程退出/连续3次失败
   │                  ▼                          ▼
   └──── user stop ─ Failed ◀──退避耗尽─── Restarting(backoff 2s→60s, 10 分钟内 ≤5 次)
External：健康 ok 且非本托盘实例；健康失败后回到 Stopped（不自动启动外部实例）
```

运行中每 10 秒检查一次；菜单打开时立即补一次检查。进程退出事件来自 Windows 侧 `wsl.exe` 的 `Wait()`，与健康检查互为补充（`wsl --shutdown` 两者都会触发，以先到者为准，去重）。

### 5. 链接推导

纯函数 `Links(listen string, token bool, addrs []Iface) (local string, remote []Remote, warn bool)`，规则见 `windows-tray-access` spec。远程候选地址来源：WSL 内 `ip -4 -o addr`（缺失时降级为空），Windows 侧 `net.Interfaces()`；按接口名/网段 `100.64.0.0/10` 识别 Tailscale 并置顶；排除回环、链路本地与 WSL 虚拟网卡（`vEthernet (WSL*)`）。

### 6. 读取 WebMux 配置

通过 `wsl.exe --exec cat <config>` 读取（不经 `\\wsl.localhost\` UNC，避免 9P 在发行版未运行时触发启动、以及用户名映射问题），只解码 `listen` 与 `token` 两个字段，其余字段忽略。读取结果只在内存中缓存，记录“运行实例启动时的配置快照”用于“配置已修改需重启”提示。Token 不写盘、不进日志（日志层对已知 Token 做字面替换作为兜底）。

### 7. Windows 适配层

- 托盘：`fyne.io/systray`（Windows 实现基于 `Shell_NotifyIcon`，纯 Go）。图标为 4 个状态的 `.ico`，`go:embed`。左键“打开”、右键菜单。
- 通知：`Shell_NotifyIcon` 气泡通知（`NIF_INFO`），无需注册 AppUserModelID；如后续需要操作按钮再考虑 Toast。
- 确认对话框：`MessageBoxW`（`MB_YESNOCANCEL` 用于退出三选一）。
- 剪贴板：写入 `CF_UNICODETEXT` 的同时设置 `ExcludeClipboardContentFromMonitorProcessing`、`CanIncludeInClipboardHistory=0`、`CanUploadToCloudClipboard=0` 格式；记录 `GetClipboardSequenceNumber`，60 秒后序号未变则清空。
- 自动启动：`HKCU\Software\Microsoft\Windows\CurrentVersion\Run` 值 `WebMuxTray` = `"<exe 绝对路径>"`。
- 单实例：命名互斥量 `Local\WebMuxTray-<SID>`；第二实例通过向已有实例窗口 `PostMessage` 自定义消息请求显示状态通知。
- 打开浏览器 / 编辑器 / 日志：`ShellExecuteW`；WSL 内的配置文件与服务端日志通过 `\\wsl.localhost\<distro>\...` 路径交给记事本打开（此时发行版必然已运行）。
- 设置文件：`%APPDATA%\WebMux\tray.json`；托盘日志：`%LOCALAPPDATA%\WebMux\tray.log`（1 MiB 轮转，保留 1 份）。设置热加载用 `ReadDirectoryChangesW`/轮询 mtime（2 秒）二选一，实现时取简单者。

### 8. 构建与 CI

- `Makefile`：`tray` 目标 `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS) -H=windowsgui" -o bin/webmux-tray-windows-amd64.exe ./cmd/webmux-tray`，arm64 同理；`release` 依赖 `tray`。托盘不依赖前端构建。
- 可执行文件图标/版本资源：用 `go-winres` 生成并提交 `cmd/webmux-tray/rsrc_windows_*.syso`（生成工具不进入构建依赖）。
- CI：在现有 `go` 作业中追加 `GOOS=windows go vet ./cmd/webmux-tray ./internal/tray/...` 与两个架构的交叉编译；`internal/tray` 的单元测试随 `go test -race ./...` 在 Ubuntu/macOS 运行。不新增 Windows runner（GitHub 托管 Windows runner 不提供可用的 WSL 2），真实 WSL 行为由手动验收清单覆盖。

## Risks / Trade-offs

- [WSL 版本差异：`--exec`、`--cd`、`WSL_UTF8` 在旧版 inbox WSL 上不可用] → 声明最低支持“Microsoft Store 版 WSL”；启动时探测 `wsl.exe --version`，失败时提示更新 WSL。
- [托盘退出后 `wsl.exe` 继续运行，用户可能以为“退出就全停了”] → 退出时三选一明确说明；`仅退出托盘`后重新打开托盘可恢复控制。
- [PID 复用导致误杀] → 停止前校验 `/proc/<pid>/exe`；校验失败拒绝发信号。
- [Windows 侧 localhost 不可达（关闭 localhostForwarding、VPN 软件劫持回环）] → 区分“WSL 内进程存活”与“Windows 可达”，给出具体设置项提示。
- [公司代理/安全软件拦截 127.0.0.1 HTTP] → 健康检查客户端禁用系统代理。
- [Token 经剪贴板泄露] → 不进历史/云剪贴板、60 秒自动清除、仅在用户主动操作时复制；文档提示优先使用浏览器密码管理器。
- [`fyne.io/systray` 的菜单不支持禁用项提示文字、菜单动态刷新有限] → 状态说明放在状态行文本中；必要时退回直接调用 Win32 菜单 API。
- [`wsl --shutdown` 或 Windows 睡眠唤醒会结束会话] → 这是 WebMux“服务器重启即结束会话”语义的延伸；通知中明确“会话已丢失”，不尝试恢复。
- [第三方依赖引入] → 仅 Windows 构建引入 `fyne.io/systray`；服务端二进制依赖集不变，可用 `go version -m bin/webmux` 验证。

## Migration Plan

纯新增组件，无数据迁移。已用 systemd 在 WSL 中运行 WebMux 的用户安装托盘后会被识别为“外部实例”，行为不变；若想交给托盘托管，先 `systemctl --user disable --now webmux` 再在托盘中启动。回滚：关闭“登录时自动启动”并删除 exe 与 `%APPDATA%\WebMux`。

## Open Questions

- 托盘图标的视觉设计（是否沿用前端 `WM` 品牌标记）可在实现阶段与 UI 截图一起确认，不影响规格。
- 最低 WSL 版本号的精确值待在真实机器上验证 `--exec`/`WSL_UTF8` 支持后写入文档。
