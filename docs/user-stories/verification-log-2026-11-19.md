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

## 2026-11-19：Windows 托盘局域网访问防火墙只读检测、按需放行与卸载清理（#705，US-O08、US-O12）

### 范围

- 将原 360 行的 `docs/user-stories/clients-network-tray.md` 按分组拆分为树状索引及子页 `clients-network-tray/clients-and-tunnels.md`（US-M01–US-M08、US-N01–US-N08、US-P01–US-P08）与 `clients-network-tray/windows-tray.md`（US-O01–US-O14），故事编号、总数与覆盖索引保持不变；
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
