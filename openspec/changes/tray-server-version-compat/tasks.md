# Tasks

## 1. 服务端版本信息

- [ ] 1.1 `internal/contract` 新增 `VersionV1{V, Version, Contract, Revision, Modified}` 与 `BuildStatus{Revision, Modified, Contract}`，`StatusV1` 增加可选 `Build *BuildStatus \`json:"build,omitempty"\``；新增读取 `debug.ReadBuildInfo()` 中 `vcs.revision` / `vcs.modified` 的辅助函数。验证：单元测试断言无构建信息时 `Revision` 为空、`Modified` 为 false，且旧格式的状态 JSON（无 `build`）仍能解析
- [ ] 1.2 `cmd/webmux` 的 `version` 支持 `--json`，输出 `VersionV1`；无参数与 `--version` 输出不变，未知参数退出码 2。验证：`cmd/webmux` 测试断言 JSON 字段与 `contract.V` 一致、不读取配置、文本输出仍为 `webmux <版本>`；`make build` 后 `./bin/webmux version --json` 的 `revision` 与 `git rev-parse HEAD` 一致
- [ ] 1.3 `api.NewLocal` 在运行中的 `/local/v1/status` 填充 `build`；记录回退路径与未运行时省略。验证：`internal/api` 本机接口测试断言运行中含 `build.contract == 1`、`v` 仍为 1；`internal/client` 测试断言未运行时无 `build`
- [ ] 1.4 `docs/protocol.md` 控制面契约中补充 `version --json` 与 `status` 的 `build` 字段。验证：`make format-check` 通过，文档字段与 1.1 的 JSON 标签一致

## 2. 托盘兼容与对应判定

- [ ] 2.1 `internal/tray` 新增 `BuildInfo` 与纯函数 `Classify(tray, server BuildInfo) Compat`（`matched`、`server-older`、`server-newer`、`build-differs`、`incompatible-older`、`incompatible-newer`、`unknown`）及托盘自身的 `BuildInfo`。验证：表驱动测试覆盖 `windows-tray-server-compat` 中“兼容判定”“对应判定”的全部场景，包括 `dev`、提交未知、契约过高/过低
- [ ] 2.2 `WSLService.Check` 在探测通过后执行 `version --json`，解析失败时回退 `ParseVersion` 文本并视契约为 1，返回 `BuildInfo`；`Target` 新增对应调用并保持 argv 无 shell。验证：`core_test.go` 的 argv 检查覆盖新调用；`service_test.go` 用假 Runner 覆盖 JSON、旧文本、无法执行三种输出
- [ ] 2.3 `Status` 经本机 API 取得状态后同样拒绝 `v > contract.V`，与 `status --json` 回退一致。验证：服务测试中本机 API 返回 `v: 2` 时得到“请升级托盘”的错误

## 3. 托盘状态、通知与菜单

- [ ] 3.1 `Snapshot` 以 `Installed BuildInfo`、`Running *BuildInfo`、`Compat` 取代 `WebmuxVersion` / `UpgradeTo`；`Supervisor` 在检查结果与每次运行中状态时分类，并在不兼容时禁用启动、重启与依赖本机 API 的操作、保留打开与复制链接；更新所有调用方与既有测试。验证：`supervisor_test.go` 覆盖不兼容禁用、升级后恢复、运行中实例与已安装文件不一致时的“重启后生效”（仅托盘管理实例可重启）
- [ ] 3.2 通知去重：同一发行版、路径、两侧版本与分类在一次托盘运行内只通知一次，`matched` / `unknown` 不通知。验证：supervisor 测试多次 recheck 后通知计数为 1，切换发行版后再次通知
- [ ] 3.3 `menuview.go` 统一生成状态行与修复入口文案：两侧版本、较旧一方、“同版本不同构建”的短提交号、“下载托盘 <版本>”（仅发布版本）；`winui` 菜单接入并用 `winui.OpenURL` 打开 Release 页面。验证：`menuview_test.go` 表驱动覆盖每种分类的文案与入口；`GOOS=windows go vet ./cmd/webmux-tray ./internal/tray/...` 通过
- [ ] 3.4 手动验收（真实 Windows + WSL）：对应、WSL 较旧（安装后恢复）、WSL 较新（打开 Release 页面）、运行中实例未重启、两个本地构建来自不同提交五种情况的状态行、通知与菜单。验证：PR 描述附截图
