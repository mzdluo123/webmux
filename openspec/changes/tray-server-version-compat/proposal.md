# Proposal

## Why

Windows 托盘与 WSL 中的 `webmux` 是分别安装、分别升级的两个程序，用户很容易让两边停在不同版本：只更新了托盘、在 WSL 里手动换了二进制、升级后没有重启正在运行的实例，或者两边都是本地构建但来自不同提交。目前托盘只在“缺少本机 API”时判为不兼容、在“已安装文件版本低于托盘”时于状态行提示可升级；WSL 中版本更高、正在运行的实例与已安装文件不一致、两个本地构建版本号相同但代码不同，这些情况都不会被发现，出问题时用户只能看到含糊的失败。托盘需要在启动时明确判断两边是否兼容、是否对应，并告诉用户该升级哪一边。

## What Changes

- `webmux version` 新增 `--json`，输出版本号、构建提交、是否含未提交改动以及所支持的控制契约版本；不带参数时的输出保持不变。
- 本机状态契约 `/local/v1/status`（及 `webmux status --json`）新增可选的构建信息，使托盘能识别正在运行的实例是哪一次构建。
- 托盘在启动、切换发行版或 `webmux` 路径、安装/升级之后检查 WSL 中的 `webmux`，把结果归为：对应、兼容但不对应（WSL 较旧 / WSL 较新 / 同版本不同构建）、不兼容（WSL 过旧 / WSL 过新）、缺失、无法判断。
- 兼容以控制契约版本为准：托盘声明自己支持的契约版本，WSL 中的 `webmux` 报告自己的契约版本；不在托盘支持范围内即不兼容。不支持 `--json` 的旧版本沿用现有的功能探测。
- 不兼容时禁止由托盘启动或管理 WebMux，并按方向给出修复入口：WSL 过旧 → 安装与托盘同版本的 `webmux`；WSL 过新 → 打开与之对应版本的托盘下载页。打开 WebMux 等访问类操作不受影响。
- 兼容但不对应时照常工作，在状态行显示两边版本，并在每次托盘启动时最多弹出一次通知；WSL 较旧提供已有的“升级 WSL 中的 webmux”，WSL 较新提示升级托盘。
- 正在运行的实例与已安装文件不是同一构建时，提示“重启 WebMux 后生效”；仅托盘管理的实例提供重启，SHALL NOT 自动重启。

## Capabilities

### New Capabilities

- `windows-tray-server-compat`: 托盘对 WSL 中 `webmux`（已安装文件与正在运行的实例）的版本兼容与对应判定、检查时机、状态展示、通知与修复入口。

### Modified Capabilities

- `cli-server-control`: 新增 `webmux version --json` 的机器可读版本信息（含契约版本与构建提交）。
- `server-local-api`: 状态中新增可选的构建信息字段，契约版本仍为 v1。

## Impact

- `cmd/webmux`（`version --json`）、`internal/contract`（版本信息结构与状态中的构建字段）、`internal/api/local.go` 与 `internal/client/status.go`（填充构建字段）。
- `internal/tray`：`Check` 改为优先解析 `version --json`，新增兼容/对应分类与运行实例比较；`Snapshot`、`supervisor.go`、`menuview.go` 与 `winui` 的状态行、通知、菜单入口。
- 与 `add-tray-wsl-install-and-runtime-tunnel` 中“WSL 中 webmux 的分类”一致并细化：该处“版本高于托盘视为可用”保留为“兼容但不对应”，新增的是提示与契约判定。
- 构建：`Makefile` 本地构建版本号固定为 `0.1.0`，对应判定依赖 Go 内嵌的 VCS 信息；`-trimpath` 不影响该信息，无需新增构建参数。
- 不涉及 Windows 控制台客户端 `webmux.exe`、托盘自更新与前端。
