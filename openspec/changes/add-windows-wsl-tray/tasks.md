## 1. 平台无关核心（`internal/tray`）

- [ ] 1.1 新增 `settings.go`：设置结构（发行版、`webmux` 路径、配置路径、托盘启动时自动启动、自动重启）、默认值、JSON 读写与校验；损坏时返回默认值与错误且不回写。验证：`go test ./internal/tray -run Settings` 覆盖缺失文件、损坏 JSON、未知字段、空路径、不含 Token 字段
- [ ] 1.2 新增 `wsl.go`：`Runner` 接口与 argv 构造（`-d`、`--cd ~`、`--exec /bin/sh -c <固定脚本> ...`），固定启动/探测/停止脚本常量；发行版列表解析同时支持 UTF-16LE（含 BOM）与 UTF-8。验证：单元测试断言含空格、`;`、`$()`、引号的路径只作为独立 argv 元素出现，`wsl --list --quiet` 的 UTF-16LE 样例与中文发行版名解析正确
- [ ] 1.3 在 Linux 上用 `/bin/sh` 直接执行固定脚本（不经 wsl.exe）的集成测试：`~/` 展开、日志轮转为 `.1`、pidfile 写入、`exec` 后 PID 不变、stdout/stderr 落盘、探测脚本对存活/不存在/PID 被复用返回 0/1/2、停止脚本 SIGTERM→超时 SIGKILL。验证：`go test -race ./internal/tray -run Script`
- [ ] 1.4 新增 `webmuxconfig.go`：只解码 `listen`/`token`，缺失文件使用 `internal/config` 一致的默认值，无法解析时报错。验证：单元测试与 `config.Load` 默认值对照，确保两处默认 `listen` 相同
- [ ] 1.5 新增 `links.go`：本机链接、远程链接、通配无 Token 警告、Tailscale 置顶与排除回环/链路本地/WSL 虚拟网卡。验证：表驱动测试覆盖 `127.0.0.1`、`localhost`、`[::1]`、`0.0.0.0`、`::`、`:7681`、`100.64.0.5`、具体 LAN 地址，结果不含 Token
- [ ] 1.6 新增 `health.go`：3 秒超时、Bearer、禁用系统代理，结果分类 `ok/unauthorized/refused/timeout/notWebMux/other`。验证：`httptest` 覆盖每种分类，以及使用真实 `api.New` 处理器时带/不带 Token 的结果
- [ ] 1.7 新增 `supervisor.go`：状态机（Stopped/Starting/Running/External/Stopping/Restarting/Failed）、60 秒就绪超时、每 10 秒检查、连续 3 次失败转异常、退避 2s→60s 且 10 分钟内 ≤5 次、用户停止不重启、预检失败不重试、托盘重启后通过探测恢复控制、外部实例与端口冲突判定、进程退出与健康失败事件去重。验证：注入假 `Runner`/健康客户端/时钟的单元测试逐条覆盖 `wsl-service-lifecycle` 场景，`go test -race` 通过
- [ ] 1.8 托盘日志写入器：1 MiB 轮转保留 1 份，对已知 Token 做字面替换兜底。验证：单元测试写入含 Token 的消息后文件中不出现 Token

## 2. Windows 适配层（`internal/tray/winui`，`//go:build windows`）

- [ ] 2.1 引入 `fyne.io/systray` 与 `golang.org/x/sys/windows`，确认 `CGO_ENABLED=0 GOOS=windows` 可构建且 `go version -m bin/webmux` 的依赖集不变。验证：`go mod tidy` 无额外变更，两条构建命令成功
- [ ] 2.2 `process.go`：基于 `os/exec` 的 `Runner` 实现，所有调用 `CREATE_NO_WINDOW`，托管进程 detached 且不加入随托盘关闭而终止的 Job Object，设置 `WSL_UTF8=1`。验证：Windows 手动验收中无控制台闪烁；`GOOS=windows go vet` 通过
- [ ] 2.3 `tray.go`/`menu.go`：状态图标（4 个 `.ico`，`go:embed`）、悬停提示、左键打开、右键菜单分组与启用规则、发行版子菜单单选、远程链接子菜单与警告项，订阅 `Supervisor` 快照刷新。验证：手动验收清单 4.2 中菜单相关条目
- [ ] 2.4 `notify.go` 与确认对话框：气泡通知（复制反馈、意外退出、单实例唤起），`MessageBoxW` 确认停止/重启/切换发行版（含会话数）与退出三选一。验证：手动验收 4.2
- [ ] 2.5 `clipboard.go`：`CF_UNICODETEXT` + 排除历史/云剪贴板格式，Token 60 秒后按剪贴板序号条件清除。验证：手动确认 Win+V 中不出现 Token，复制其他文本后不被清除
- [ ] 2.6 `autostart.go`：HKCU `Run` 读写、指向路径比较（大小写不敏感、规范化）。验证：手动注销重登后自动出现；移动 exe 后复选项显示未勾选
- [ ] 2.7 `singleinstance.go`：`Local\WebMuxTray-<SID>` 互斥量与唤起消息。验证：重复启动只保留一个图标且已有实例弹出状态通知
- [ ] 2.8 设置文件 `%APPDATA%\WebMux\tray.json` 与日志 `%LOCALAPPDATA%\WebMux\tray.log` 路径、“编辑托盘设置”/“打开日志”/“打开 WebMux 配置文件”（`\\wsl.localhost\<distro>\...`）与设置热加载。验证：手动修改设置后无需重启托盘即生效

## 3. 入口、构建与 CI

- [ ] 3.1 新增 `cmd/webmux-tray/main_windows.go`（显式依赖组装、`version` 子命令）与 `main_other.go`（非 Windows 打印不支持并退出 1）。验证：Linux 上 `go build ./...`、`go vet ./...`、`go test -race ./...` 通过
- [ ] 3.2 用 `go-winres` 生成并提交 exe 图标/版本资源 `.syso`。验证：资源管理器中 exe 显示图标与版本
- [ ] 3.3 `Makefile` 新增 `tray`（amd64/arm64，`-H=windowsgui`、`CGO_ENABLED=0`、`-X main.version`），`release` 依赖 `tray`；`format`/`format-check` 覆盖新 Go 目录（已在 `cmd`、`internal` 下，确认即可）。验证：`make tray` 产出两个 exe，`make format-check` 通过
- [ ] 3.4 `.github/workflows/test.yml` 的 `go` 作业追加 `GOOS=windows GOARCH={amd64,arm64} go build ./cmd/webmux-tray` 与 `GOOS=windows go vet ./cmd/webmux-tray ./internal/tray/...`。验证：PR 上 CI 绿色
- [ ] 3.5 `docs/deployment.md` 新增“Windows（WSL）”一节：前置条件（WSL 2、Store 版 WSL）、在发行版内安装 `webmux`、托盘放置与登录启动、与 systemd 共存、localhost 转发诊断、Token 剪贴板说明。验证：`make format-check` 通过

## 4. 验收

- [ ] 4.1 运行 `make format-check`、`go vet ./...`、`go test -race ./...`、`make tray`，全部通过
- [ ] 4.2 在真实 Windows 11 + WSL 2（NAT 与 mirrored 各一次）手动验收：首次运行默认设置启动、无控制台窗口、单实例、菜单启用规则、左键打开、复制本机/远程链接、复制 Token 不入 Win+V 且 60 秒清除、有会话时停止/重启确认、停止超时强制结束、`wsl --shutdown` 后通知与自动重启、连续崩溃 5 次后停止重试、仅退出托盘后会话保持且重开托盘恢复控制、systemd 实例识别为外部、端口被占用提示、关闭 localhostForwarding 时的诊断、登录自动启动与 WSL 冷启动、切换发行版确认、托盘日志不含 Token
- [ ] 4.3 按 AGENTS.md 的 UI 证据要求，在 PR 描述中附托盘图标各状态与菜单（浅色/深色任务栏）截图
