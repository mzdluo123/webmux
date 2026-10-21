## 0. 前置

- [ ] 0.1 确认 `add-cli-server-control` 已实现并合入（`webmux serve --log-file`、`status --json` v1、`stop --json`、`WEBMUX_LAUNCHER`）。验证：Linux 上 `webmux status --json` 输出 `"v":1`

## 1. 平台无关核心（`internal/tray`）

- [ ] 1.1 新增 `settings.go`：设置结构（发行版、启动方式 `auto/tray/systemd`、`webmux` 路径、配置路径、日志路径、托盘启动时自动启动、自动重启）、默认值、JSON 读写与校验；损坏时返回默认值与错误且不回写。验证：`go test ./internal/tray -run Settings` 覆盖缺失文件、损坏 JSON、未知字段、非法启动方式、不含 Token 字段
- [ ] 1.2 新增 `wsl.go`：`Runner` 接口与设计表中每种调用的 argv 构造（`-d`、`--cd ~`、`~/x` → `./x` 改写、`WEBMUX_LAUNCHER`/`WSLENV` 合并）；`--list` 解析同时支持 UTF-16LE（含 BOM）与 UTF-8。验证：单元测试断言含空格、`;`、`$()`、引号的路径只作为独立 argv 元素出现，`WSLENV` 已有值时正确追加，中文发行版名解析正确
- [ ] 1.3 新增 `mode.go`：`auto` 判定（`/proc/1/comm`、`systemctl --user cat`）、显式 systemd 条件不满足时报错、外部实例判定（`record`、`launcher` 与方式对应）。验证：注入假 `Runner` 的表驱动测试
- [ ] 1.4 新增 `supervisor.go`：状态机（Stopped/Starting/Running/Unreachable/External/Stopping/Restarting/Failed）、60 秒就绪、10 秒 Windows 可达性与 30 秒状态刷新、连续 3 次无响应转异常、托盘托管方式退避 2s→60s 且 10 分钟内 ≤5 次、systemd 方式不自行重启、用户停止不重启、版本过旧/可执行文件缺失不重试、托盘重启后按启动方恢复控制、进程退出与状态事件去重。验证：注入假 `Runner`/状态/可达性/时钟的单元测试逐条覆盖 `wsl-service-lifecycle` 场景，`go test -race` 通过
- [ ] 1.5 托盘日志写入器：1 MiB 轮转保留 1 份，对已知 Token 做字面替换兜底。验证：单元测试写入含 Token 的消息后文件中不出现 Token

## 2. Windows 适配层（`internal/tray/winui`，`//go:build windows`）

- [ ] 2.1 引入 `fyne.io/systray`，确认 `CGO_ENABLED=0 GOOS=windows` 可构建，且 `go version -m bin/webmux` 的依赖集不变。验证：`go mod tidy` 无额外变更，两条构建命令成功
- [ ] 2.2 `process.go`：基于 `os/exec` 的 `Runner`，所有调用 `CREATE_NO_WINDOW` 与 `WSL_UTF8=1`；长生命周期进程 detached、标准流为 NUL、不入 Job Object；systemd 保活进程 PID+创建时间记录与校验。验证：`GOOS=windows go vet` 通过；手动验收中无控制台闪烁、仅退出托盘后进程仍在
- [ ] 2.3 `tray.go`/`menu.go`：状态图标（4 个 `.ico`，`go:embed`）、悬停提示、左键打开、右键菜单分组与启用规则、发行版与启动方式子菜单、远程链接子菜单与警告项、打开日志子菜单；订阅快照刷新且菜单弹出不阻塞。验证：手动验收 4.2 中菜单相关条目
- [ ] 2.4 `notify.go`/`dialog.go`：气泡通知（复制反馈、意外退出、单实例唤起、systemd 单元失败），`MessageBoxW` 确认停止/重启/切换（含会话数）与退出三选一。验证：手动验收 4.2
- [ ] 2.5 `clipboard.go`：`CF_UNICODETEXT` + 排除历史/云剪贴板格式；Token 仅在点击时经 `status --reveal-token` 获取，60 秒后按剪贴板序号条件清除。验证：手动确认 Win+V 中不出现 Token，期间复制其他文本后不被清除
- [ ] 2.6 `autostart.go`：HKCU `Run` 读写、指向路径比较（大小写不敏感、规范化）。验证：手动注销重登后自动出现；移动 exe 后复选项显示未勾选
- [ ] 2.7 `singleinstance.go`：`Local\WebMuxTray-<SID>` 互斥量与唤起消息。验证：重复启动只保留一个图标且已有实例弹出状态通知
- [ ] 2.8 Windows 侧可达性检查（`Proxy: nil`、3 秒、不带 Token、200/401 视为可达）、设置文件 `%APPDATA%\WebMux\tray.json` 与 2 秒热加载、“编辑托盘设置”“打开 WebMux 配置文件”“打开日志”（托盘托管为 `\\wsl.localhost` 下的日志文件，systemd 为 journal 导出）。验证：手动修改设置后无需重启托盘即生效；关闭 localhostForwarding 时状态行给出诊断

## 3. 入口、构建与 CI

- [ ] 3.1 新增 `cmd/webmux-tray/main_windows.go`（显式依赖组装、`version`）与 `main_other.go`（非 Windows 打印不支持并退出 1）。验证：Linux 上 `go build ./...`、`go vet ./...`、`go test -race ./...` 通过
- [ ] 3.2 用 `go-winres` 生成并提交 exe 图标/版本资源 `.syso`。验证：资源管理器中 exe 显示图标与版本
- [ ] 3.3 `Makefile` 新增 `tray`（amd64/arm64、`-H=windowsgui`、`CGO_ENABLED=0`、`-X main.version`），`release` 依赖 `tray`。验证：`make tray` 产出两个 exe，`make format-check` 通过
- [ ] 3.4 `.github/workflows/test.yml` 的 `go` 作业追加 Windows 交叉编译与 `GOOS=windows go vet ./cmd/webmux-tray ./internal/tray/...`。验证：PR 上 CI 绿色
- [ ] 3.5 `docs/deployment.md` 新增“Windows（WSL）”一节：前置条件（WSL 2、Store 版 WSL）、在发行版内安装 `webmux`、放置托盘与登录启动、两种启动方式及与现有 systemd 单元的关系、localhost 转发诊断、Token 剪贴板说明。验证：`make format-check` 通过

## 4. 验收

- [ ] 4.1 运行 `make format-check`、`go vet ./...`、`go test -race ./...`、`make tray`，全部通过
- [ ] 4.2 在真实 Windows 11 + WSL 2（NAT 与 mirrored 各一次）手动验收：首次运行默认设置启动、无控制台窗口、单实例、菜单启用规则与不阻塞、左键打开、复制本机/远程链接、复制 Token 不入 Win+V 且 60 秒清除、有会话时停止/重启确认、停止超时强制结束、托盘托管下 `wsl --shutdown` 后通知与自动重启、连续崩溃 5 次后停止重试、两种方式下仅退出托盘后会话保持 5 分钟且重开托盘恢复控制、`auto` 在启用/未启用 systemd 的发行版上分别选对、手动启动的实例识别为外部、端口被占用提示、旧版 `webmux` 提示升级、关闭 localhostForwarding 时的诊断、登录自动启动与 WSL 冷启动、切换发行版/启动方式确认、托盘日志不含 Token
- [ ] 4.3 按 AGENTS.md 的 UI 证据要求，在 PR 描述中附托盘图标各状态与菜单（浅色/深色任务栏）截图
