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
