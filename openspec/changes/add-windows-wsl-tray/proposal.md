## Why

WebMux 只支持 Linux/macOS，Windows 用户目前要在 WSL 里手动启动 `webmux`，再自己记住地址和 Token；WSL 发行版没有前台进程时会被回收，`systemd --user` 服务也不一定启用，导致“开机即用、浏览器断开不丢会话”的体验在 Windows 上不成立。提供一个 Windows 托盘程序，负责在登录后把 WebMux 拉起在 WSL 中、保持其存活，并让链接信息和常用管理操作在任务栏一键可达。

> 本提案中标注“待确认”的范围项见 design.md「Decisions › 0. 待用户确认的决策」，当前按推荐默认值编写。

## What Changes

- 新增 Windows 托盘程序 `webmux-tray.exe`（Windows amd64/arm64，独立于 `webmux` 服务端二进制，同仓库同 Go module 构建）：
  - 单实例运行，通知区域图标按状态区分（已停止 / 启动中 / 运行中 / 异常），悬停提示显示状态摘要。
  - 托盘菜单：状态行（状态、会话数、版本、发行版）、打开 WebMux、复制链接、复制 Token、启动 / 停止 / 重启、打开日志、打开配置文件、发行版选择、登录时自动启动开关、编辑托盘设置、退出。
- 在 WSL 中托管 WebMux 进程：
  - 通过 `wsl.exe` 以 argv 方式在选定发行版中启动 `webmux`，不弹出控制台窗口；进程输出写入 WSL 内的日志文件，而不是管道回托盘，托盘退出不会影响服务端存活。
  - 启动后轮询 `/api/health` 直到就绪或超时；检测到同地址已有健康的 WebMux 实例时接管监控而不重复启动；地址被其他程序占用时报告冲突。
  - 停止 / 重启发送 SIGTERM 让 WebMux 自行清理会话，超时后再强制结束；存在会话时先确认（停止或重启会结束所有终端会话）。
  - 非用户主动的意外退出（含 `wsl --shutdown`）按退避策略自动重启，并给出 Windows 通知。
- 登录自动启动：在当前用户的 Windows 登录启动项中注册/注销托盘程序，可从菜单切换；托盘启动时按设置自动拉起 WebMux。
- 链接信息：从 WSL 中的 WebMux 配置读取 `listen` 与 `token`，生成 Windows 本机可打开的链接及非回环监听时的远程链接；Token 只在用户主动复制时进入剪贴板，并排除出剪贴板历史与云同步。
- 构建：`Makefile` 新增 `tray` 目标并纳入 `release`；平台无关逻辑在 Linux CI 上单元测试，Windows 目标在 CI 中交叉编译与 `go vet`。
- 服务端 `webmux` 与浏览器前端在本变更中不做修改（是否支持带 Token 的一键链接为待确认项）。

## Capabilities

### New Capabilities

- `windows-tray-host`: 托盘程序本身——单实例、图标与悬停状态、菜单结构与可用性、托盘设置的存储与校验、发行版选择、托盘日志、退出行为、登录自动启动开关。
- `wsl-service-lifecycle`: 在 WSL 发行版中启动、监控、停止、重启 WebMux——进程托管方式、健康检查与就绪判定、已有实例接管、端口冲突、会话结束确认、意外退出自动重启与通知、服务端日志位置。
- `windows-tray-access`: 链接信息——从 WebMux 配置推导本机与远程链接、打开浏览器、复制链接与 Token、Windows 侧可达性诊断与安全约束。

### Modified Capabilities

当前 `openspec/specs/` 无已登记主规格，无可修改路径。本变更不改变 WebMux 服务端的认证、Origin、Host 校验和会话语义，托盘作为普通 HTTP 客户端访问既有 `/api/health`。

## Impact

- 新增代码：`cmd/webmux-tray/`（Windows 入口与非 Windows 占位入口）、`internal/tray/`（平台无关的设置、WSL argv 构造、配置解析、链接推导、状态机、退避策略）、`internal/tray/winui`（仅 Windows：托盘图标、菜单、通知、剪贴板、注册表启动项、进程创建）。
- 新增 Go 依赖：托盘图标库（推荐 `fyne.io/systray`，Windows 下无需 CGO），以及 `golang.org/x/sys/windows`（已有 `golang.org/x/sys` 模块）。保持 `CGO_ENABLED=0`。
- 构建与 CI：`Makefile` 增加 `tray` 并扩展 `release`；`.github/workflows/test.yml` 增加 Windows 交叉编译与 vet 步骤；`go test ./...` 与 `go vet ./...` 在 Linux/macOS 上继续通过。
- 资源：托盘状态图标（`.ico`，`go:embed`）。
- 文档：`docs/deployment.md` 增加 “Windows（WSL）” 一节——这是新的部署方式，不属于“每次变更都更新文档”的范畴。
- 不涉及：`internal/session`、`internal/api`、`internal/hub`、`web/` 与 WebSocket 协议。
