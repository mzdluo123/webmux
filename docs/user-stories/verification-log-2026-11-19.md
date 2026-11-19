# 核对记录：2026-11-19 起

承接 [verification-log-2026-11-18](verification-log-2026-11-18.md)，收录 2026-11-19 起的局部核对记录；索引见 [WebMux User Stories](../user-stories.md#核对记录)。

## 2026-11-19：启动配置完成与介入提醒范围切换（T090，US-B08）

### 范围

- 故事编号、总数与覆盖索引保持不变，更新 US-B08 的提醒范围交互验收标准：
  - `web/src/components/settings/LaunchAttentionFields.svelte`：方向键可在「全部提醒」「仅需介入」「关闭」三档之间连续切换；
  - 选「仅需介入」时四类信号源保持可用，时间阈值原位禁用并维持与「全部提醒」一致的高度；
  - 选「关闭」时信号源、优先级说明与时间阈值通过 `useDockMotion` 收起并标记 `hidden` 与 `inert`，开启减少动态效果时直接切换；
  - 时间阈值超出 `0–300` 秒或 `0–60` 秒范围时按需渲染 `role="alert"` 实时错误文本，无错误时保持紧凑布局。

### 已执行验证与局限

- 前端类型、静态与文档门禁：
  - `pnpm --dir web check`、`pnpm --dir web check:test`、`pnpm --dir web lint` 与 `pnpm --dir web check:arch` 通过；
  - `go test -count=1 ./internal/docguard` 通过。
- 浏览器组件测试：
  - `pnpm --dir web exec vp test run --config vitest.browser.config.ts test/browser/LaunchConfigDialog.test.ts` 通过，覆盖提醒范围键盘切换、「关闭」收起高度、阈值原位禁用与范围错误按需播报。
- 局限：
  - 本轮验证在无头 Linux Chromium 中执行，未连接生产后端，未在物理触屏或真机浏览器上复测。
  - 视觉证据见 [PR #786](https://github.com/mzdluo123/webmux/pull/786)。

## 2026-11-19：移除 Git 工作台返工意见与会话协助起草说明（US-J07、US-J25、US-J28）

### 范围

- 故事编号与总数（264）保持原状，同步更新 `US-J07`、`US-J25`、`US-J28` 与覆盖索引：
  - 删除 `GitReworkDialog.svelte`、`git-rework-model.ts`、`git-rework-controller.svelte.ts`、`git-session-handoff.ts` 以及 `GitReworkItemSchema` 与 `ReworkSessionHandoff` Storybook 场景；
  - 移除 `GitWorkbenchTopBar.svelte` 与 `GitWorkbenchShellHeader.svelte` 的「返工意见」入口、`GitStagingHunkStrip.svelte` 的「加入意见」按钮，以及 `GitCommitComposer.svelte` 的「让会话协助起草说明」按钮；
  - `GitDiffSelectionBar.svelte` 收敛为 `Commit` 视图的逐行暂存、取消暂存与丢弃操作条（`US-J28`），在紧凑补丁回退模式下将阅读器行号还原为真实文件行号并拦截跨侧选区，`GitLogDiffPanel.svelte` 移除意见选区条；
  - 保留 `US-J25` 的跨设备审阅进度同步（`GET/PUT .../git/reviews`）与 `baseRevision` 乐观锁提交草稿及 `409` 并排恢复（`GET/PUT .../git/commit-draft`）。

### 已执行验证与局限

- 静态与治理门禁：
  - `make format-check`、`make docs-check`、`make persistence-check`、`make web`、`pnpm --dir web check:test`、`pnpm --dir web check:storybook`、`pnpm --dir web lint`、`pnpm --dir web check:arch`、`go vet` 与 `make go-lint`（`golangci-lint` v2.15.0，0 issues）通过。
- 前端与端到端回归：
  - Vitest 单元回归（`git-workbench.test.ts`、`git-collab.test.ts`、`git-v3-line-smart.test.ts`、`i18n.test.ts` 共 24 项）通过；
  - Vitest 浏览器组件回归（`GitWorkbenchRoute.test.ts`、`GitWorkbenchCommitPush.test.ts`、`GitWorkbench.test.ts`、`GitWorkbenchLineSmart.test.ts` 共 28 项，含紧凑补丁回退行号映射与选区条暂存/取消暂存/丢弃用例）通过；
  - Storybook 套件 `test/storybook/git-workbench.spec.ts`（26 项）通过；
  - 隔离实例 Playwright 端到端套件 `git-workbench-*.spec.ts`（9 个文件，桌面与 iPhone SE 共 52 项）通过；
  - 在隔离演示仓库采集桌面（`1280×900`）、平板（`768×800`）、手机（`390×844`）浅色与深色共 12 张前后对比截图，并在 `1280×900`、`1280×720`、`768×800`、`390×844`、`375×667`、`320×568`、`667×375` 视口下校验 `scrollWidth <= innerWidth`。
- 局限：
  - 验证在 Linux 本地隔离实例与无头 Chromium 中完成，未验证物理 `hover`、`any-pointer: coarse` 媒体查询及 macOS / iOS / Android 真机浏览器。

## 2026-11-19：Windows 安装程序卸载清理与清除个人数据选项（#704，US-O04、US-O12）

### 范围

- 故事编号、总数与覆盖索引保持不变；`docs/user-stories/clients-network-tray.md` 按主题拆分为树状索引与 `clients-network-tray/{native-cli,public-access,windows-tray}.md`，同步更新 US-O04 与 US-O12：
  - `internal/tray/traysys/quit.go`、`instance_windows.go`、`cmd/webmux-tray/main_windows.go`：`webmux-tray.exe quit` 新增 `--stop-keepalive`，在确认无托盘运行（退出码 0 或 3）后校验创建时间戳结束 `keepalive.json` 记录的 WSL 保活进程并删除该文件；其他位置托盘仍在运行（退出码 1）时保持保活现状。
  - `deploy/windows/webmux.nsi`：卸载前先 `SetOutPath "$TEMP"` 释放安装目录工作路径占用，默认调用 `quit --stop-keepalive`、删除 `%TEMP%\webmux-journal-*.log`、递归清理 `downloads` 与 `updates` 并对当前及旧路径执行 `firewall remove`；
  - `deploy/windows/webmux.nsi`、`internal/tray/traysys/autostart_windows.go`：清理指向本次安装路径的 `StartupApproved\Run\WebMuxTray`、`NotifyIconSettings`、`MuiCache` 与 `AppCompatFlags` 注册表记录及空父目录；
  - `deploy/windows/webmux.nsi`：新增“卸载选项”页与静默 `/PURGE` 参数递归删除 `%APPDATA%\WebMux` 与 `%LOCALAPPDATA%\WebMux`，并修正卸载完成页 `MUI_FINISHPAGE_TEXT` 宏绑定以按选项展示保留或 WSL 清理说明。
  - `.github/scripts/windows-installer-smoke.ps1`：新增 `quit --stop-keepalive`、多文件缓存递归清理、未登记防火墙规则清理、安装目录作为初始工作目录的卸载、`StartupApproved`/`NotifyIconSettings`/`MuiCache`/`%TEMP%` 痕迹清理、空父目录清理与 `/S /PURGE` 递归全量清理冒烟断言。

### 已执行验证与局限

- 本地跨平台单元测试、交叉编译与架构/文档门禁：
  - `go test -race ./internal/tray/...`、`GOOS=windows GOARCH=amd64 go vet ./cmd/webmux-tray ./internal/tray/...`、`GOOS=windows GOARCH=arm64 go build ./cmd/webmux-tray` 通过；
  - `make docs-check`、`make persistence-check` 与 `go test -count=1 ./internal/archtest/...` 通过。
- 局限：
  - 本地环境为 Linux，NSIS 安装包构建与真实 Windows 会话下的安装、防火墙与卸载冒烟由 GitHub Actions `Windows installer smoke` 作业执行验证。

## 2026-11-19：Windows 托盘局域网访问防火墙只读检测、按需放行与卸载清理（#705，US-O08、US-O12）

### 范围

- 更新 US-O08、US-O12 的防火墙检测、开启局域网访问放行交互、运行期状态复查及安装卸载衔接验收标准：
  - `internal/tray/traysys`：直接解析 Windows 防火墙注册表规则（本机、组策略与 MDM）与配置文件策略，并通过 Network List Manager 读取网卡类别（专用/域/公用）；
  - `firewall allow [--clear-blocks]` 在单次管理员提权内修复失效同名规则并可选清理指向当前托盘的本机入站阻止规则，安装版同步维护 `HKCU\...\Uninstall\WebMux\FirewallRule`；
  - `internal/tray` 与 `internal/tray/winui`：开启“允许局域网访问”需要转发时在绑定非回环监听前检查防火墙并按需单次申请放行，运行期菜单实时提示未放行、阻止规则覆盖或公用网络/策略受限并提供一键修复或说明；
  - `deploy/windows/webmux.nsi` 与 `.github/scripts/windows-installer-smoke.ps1`：卸载阶段对 `$INSTDIR\webmux-tray.exe` 及记录路径执行 `firewall remove`（无规则时零弹窗退出 0），补充规则禁用/篡改修复、阻止规则清理与运行期补加规则卸载回归。

### 已执行验证与局限

- 本地架构、文档与单元测试门禁：
  - `make format-check`、`make docs-check`、`make persistence-check` 与 `make go-lint` 通过；
  - `GOOS=windows golangci-lint run ./cmd/webmux-tray ./internal/tray/...` 通过；
  - `go test -race ./internal/tray/... ./internal/archtest/...` 通过，覆盖 `[MS-GPFAS]` 规则串解析、同名规则禁用/篡改/重复项修复、本机与策略阻止规则区分、多网卡专用/公用网络菜单评估及单批次提权序列；
  - Windows amd64 / arm64 交叉编译（`./cmd/webmux-tray`、`./cmd/webmux-windows`）与 `GOOS=windows go vet` 通过。
- 局限：
  - 本地环境为 Linux，未在本地物理 Windows 桌面会话上交互点击托盘菜单与 UAC 弹窗；Windows 注册表/NLM 实机探测与 NSIS 安装卸载防火墙冒烟由 GitHub Actions `Windows installer smoke`（`windows-latest`）执行验证。

## 2026-11-19：终端指针悬停分流与输入确认复位（US-D03、US-D06）

### 范围

- 故事编号、总数与覆盖索引保持不变，更新 US-D03、US-D06 的验收标准与 [WebSocket 接口](../protocol/websocket.md)：
  - 终端悬停分流（`web/src/domains/terminal/terminal-keys.ts`、`terminal-protocol.ts`、`web/src/hooks/terminal/terminal-socket.svelte.ts`、`terminal.svelte.ts`）：
    - 识别 DEC 1003 SGR 无按键指针悬停（`CSI < Cb ; Cx ; Cy M` 且 `(Cb & ~28) == 35`），仅在控制端按动画帧合并后经 `0x06` 自动回复通道发送，并在后续按键、点击或提交前按序刷出；
    - 观察端悬停保持观察身份，保留软修饰键与关注提醒；
  - `internal/hub/hub.go`：单条 `/ws/s/{id}` 连接的 FIFO 输入队列扩展至 256 帧与 2 MiB 待写字节上限，吸收程序启动期设备查询应答与鼠标追踪帧突发；
  - 输入确认跟踪（`web/src/platform/ws-handoff.ts`、`web/src/platform/ws.ts`）：
    - 按输入序号跟踪同连接的 `0x04` 查询回执与 `0x03` 终态回执（`written`、`failed`），覆盖此前输入后清除未确认标记；
    - 探测期间有后续按键时自动追加跟进探测，服务端以 `4410` 正常结束退出流时同步重置未确认标记。

### 已执行验证与局限

- 静态、架构、文档与 Lint 门禁：
  - `make check` 通过（覆盖 `pnpm --dir web build`、`format-check`、`docs-check`、`persistence-check`、`check:test`、`lint`、`check:arch`、`go vet` 与 `golangci-lint`）。
- Go 竞态与前端单元、浏览器、E2E 测试：
  - `make test-fast` 通过，并行运行 Go `-race` 全包、`protocol/relay`、Vitest Node 单元套件与无头 Chromium 浏览器组件套件；
  - 新增回归用例覆盖 `TestInputQueueAbsorbsStartupBurstAndBoundsBytes`、`test/input.test.ts`、`test/terminal-protocol.test.ts`、`test/ws.test.ts` 与 `test/browser/TerminalTransport.test.ts`；
  - `WEBMUX_E2E_BINARY=$(realpath .cache/webmux-e2e/webmux) pnpm --dir web exec playwright test test/e2e/terminal.spec.ts` 通过。
- 局限：
  - 浏览器与 E2E 验证在 Linux 无头 Chromium 中执行，未在物理触控设备或 Safari 真机上复测。
