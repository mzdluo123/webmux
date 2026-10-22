# Design

## Context

- `WSLService.Check`（`internal/tray/service.go`）先执行 `webmux -h`，以输出中是否有 `-log-file` 判断是否支持本机 API，然后执行 `webmux version` 并用 `ParseVersion` 解析 `webmux <版本>`。`version` 只能在探测通过后调用：更早的版本会把未知参数当作启动服务。
- `Supervisor.checked` 只在“已安装文件版本低于托盘”时设置 `UpgradeTo`；版本更高、`dev`、无法解析都视为可用且不提示。`Blocker` 有 `missing`、`incompatible`、`no-wsl` 三种。
- 运行中的实例经桥接返回 `contract.StatusV1`，其中有 `Version`，但托盘只把它拼进状态行，不与已安装文件比较。`Status` 在 `status --json` 回退路径上遇到 `v > contract.V` 时报“请升级托盘”，经本机 API 的路径没有同样的检查。
- `contract.V = 1`，同一契约版本内只增加可选字段；隧道等可选功能已按字段是否存在判断支持与否。
- `Makefile` 默认 `VERSION ?= 0.1.0`，本地构建的托盘与服务端都报告 0.1.0；Go 默认把 `vcs.revision` / `vcs.modified` 写入构建信息，`-trimpath` 不影响（`go version -m bin/webmux` 可见）。发布流水线从标签检出后构建，同样带有提交信息。

## Goals / Non-Goals

**Goals:**

- 兼容由机器可读的契约版本决定，而不是由某个帮助文本里的参数决定；对旧版本保持现有探测作为回退。
- 区分“能一起工作”和“是同一次发布 / 构建”，并让本地构建也能判断。
- 已安装文件与运行中实例分别判定，升级后未重启能被发现。
- 不增加 `wsl.exe` 调用次数：`version --json` 取代现有的 `version` 调用。

**Non-Goals:**

- 托盘自更新、后台自动安装或自动重启。
- Windows 控制台客户端 `webmux.exe` 的版本检查。
- 引入独立于契约版本的“最低服务端版本”或功能清单。

## Decisions

### 1. 版本信息来自 `webmux version --json`

输出 `{v, version, contract, revision, modified}`。`contract` 直接取 `contract.V`，`revision` / `modified` 取 `debug.ReadBuildInfo()` 中的 `vcs.revision` / `vcs.modified`。现有 `version` 分支忽略多余参数，所以旧版本收到 `version --json` 会打印文本：托盘先尝试解析 JSON，失败时用 `ParseVersion` 解析文本并把契约视为 1（能走到这一步说明已通过本机 API 探测）。

备选：在 `-h` 输出里加标记，或新增 `capabilities` 子命令。前者仍是字符串匹配；后者在旧版本上会被当作启动服务，必须先探测，并不比扩展 `version` 省事。

### 2. 兼容以契约版本集合判定

托盘内置 `supportedContracts`（当前只有 `contract.V`）。服务端契约低于集合 → 不兼容（WSL 过旧），高于集合 → 不兼容（WSL 过新）。可选功能继续按字段是否存在判断，不进入兼容判定。

备选：按语义化版本判定（例如同 `MAJOR.MINOR` 兼容）。0.x 阶段的次版本号不承载兼容含义，按版本号阻止会把能正常工作的组合挡住，也无法覆盖本地构建。备选：要求版本完全相同，否则阻止；对应关系由下面的“对应判定”单独表达，只提示不阻止。

### 3. 对应判定与分类

新增纯函数 `Classify(tray, server BuildInfo) Compat`，`BuildInfo{Version, Revision, Modified, Contract}`，返回 `matched`、`server-older`、`server-newer`、`build-differs`、`incompatible-older`、`incompatible-newer`、`unknown`。规则见 `windows-tray-server-compat` 的“对应判定”。托盘自己的 `BuildInfo` 同样取自 `debug.ReadBuildInfo()`。`Classify` 不做 I/O，用表驱动测试覆盖全部分支。

`Snapshot` 用 `Installed BuildInfo`、`Running *BuildInfo`、`Compat` 取代 `WebmuxVersion` / `UpgradeTo`；`Blocker` 保留 `missing`、`no-wsl`，`incompatible` 的方向由 `Compat` 表达，菜单文案由 `menuview.go` 统一生成。

### 4. 运行中实例

`StatusV1` 增加可选的 `Build *BuildStatus{Revision, Modified, Contract}`，由 `api.NewLocal` 用服务端进程的构建信息填充；`client.status` 的记录回退路径不填（记录中没有构建信息）。托盘在每次取得运行中状态时，对 `Status.Version` + `Status.Build` 再做一次 `Classify`，并与已安装文件比较以给出“重启后生效”。本机 API 路径也执行 `v > contract.V` 检查，与 `status --json` 回退路径一致。

### 5. 通知去重

`Supervisor` 维护一个进程内的 `notified map[string]bool`，键为 `发行版|路径|托盘版本+提交|服务端版本+提交|分类`；`matched` / `unknown` 不通知。托盘重启后会再通知一次，这是“每次启动最多一次”的含义。

### 6. 下载托盘入口

Release 页面地址为 `https://github.com/<ReleaseRepo>/releases/tag/v<版本>`，复用安装器已有的 `ReleaseRepo` 变量与 `winui.OpenURL`。只在服务端版本是 `MAJOR.MINOR.PATCH` 时提供。

## Risks / Trade-offs

- [WSL 中用 `go install` 或在无 Git 的目录构建，没有提交信息] → 退回只比较版本号；本地构建都报告 0.1.0 时会判为“对应”，状态行注明“构建提交未知”。
- [两侧提交相同但都有未提交改动] → 无法区分，判为对应并在状态行注明“含未提交改动”。
- [契约版本长期不变，增量功能的缺失不会被判为不兼容] → 这是有意为之：功能按字段降级；真正不兼容的变更必须提升契约版本，在 `docs/protocol.md` 的契约规则中已有约定。
- [通知打扰] → 每次启动每组版本最多一次，对应时不通知。

## Migration Plan

1. 服务端：`version --json` 与状态中的 `build` 字段，均为向后兼容的新增，单独可发布。
2. 托盘：`Check` 改用 `version --json` 并回退文本；新增分类、运行实例比较、通知与菜单入口。旧服务端得到“兼容 + 版本比较”，行为不比现在差。
3. 回滚：托盘回退到旧版本即恢复现有行为；服务端新增字段可保留。

## Open Questions

以下决策会改变规格，已按推荐方案写入，待在 PR 中确认：

- 兼容判定依据：契约版本（当前方案） / 同 `MAJOR.MINOR` / 版本必须完全相同。
- 兼容但不对应时的提示强度：状态行 + 每次启动一次通知（当前方案） / 仅状态行 / 弹窗。
- WSL 较新时：打开对应版本的 Release 页面（当前方案） / 托盘自更新 / 仅提示。
- 本地构建是否按提交区分（当前方案：区分）。
- 不兼容时是否允许“仍然启动”（当前方案：不允许）。
- 是否把同样的检查扩展到 Windows 控制台客户端 `webmux.exe`（当前方案：不包括）。
