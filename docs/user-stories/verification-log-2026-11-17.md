# 核对记录：2026-11-17 起

本文记录 2026-11-17 起各次局部核对的原始记录，包括故事范围、执行过的验证、证据与限制。
此前的增量验证见 [2026-11-15 起记录](verification-log-2026-11-15.md)。

## 2026-11-17：固定壳层聊天输入的 IME 收起闩锁（US-Q05）

### 范围与根因

- 平板等固定壳层（窗格宽度不小于 1024px、主屏幕独立模式、分屏、代码栏打开）下，页面永不解除根文档滚动锁定，`keyboardDismissed` 闩锁在 IME 被观察收起后不再复位。
- 会话 chrome 随后把每一次新的 `chatEditing` 置位都视为待释放状态：点按消息框立即被 blur，输入框在收起一次键盘后失效；切换会话重挂页面（`{#key sessionId}` 重置 `useViewport`）后恢复，与报告一致。
- 修复为只在 `keyboardDismissed` 由 false 变 true 的当次观察沿上释放编辑状态；已观察过的闩锁不再拒绝之后的有意聚焦。US-Q05 补充验收要点，US-H02 的终端键盘开合行为不变。

### 回归证据

- 新增 `SessionChrome` 浏览器套件，以 stub 的会话 chrome 直接驱动 `keyboardDismissed` 与 `chatEditing`：闩锁标志下新聚焦不被释放、IME 重新开合后仍能再次释放、挂载前已置位的标志不影响首次聚焦；未修复的代码上两个用例均失败。
- 已运行 `SessionChrome`、`Viewport`、`TerminalTouch`、`TerminalHybrid` 浏览器用例全部通过；`pnpm check`、`pnpm lint`、`pnpm check:arch`、Oxfmt 与 `make docs-check` 通过。
- PR CI 首轮 `splits.spec.ts` 分屏用例因首字符输入竞态失败一次，与本改动无关，重跑通过。

### 限制

- 未做真实平板验证：无头浏览器无 `any-pointer: coarse` 与真实 IME 收起，收起路径以 `keyboardDismissed` 标志直接驱动；Android 返回键保留焦点与键盘按钮重新唤起路径未在真机复测。

## Relay 页内配对与宿主流程（US-A10）

本次覆盖宿主权威授权流程、刷新与导航恢复、原位验证码操作、连接诊断及独立删除回执。
保留 v2 全新绑定与 v3 状态；配对成功保持公网关闭，当前浏览器路径信息移入次级「连接诊断」。

### 自动验证

- `go test -race ./internal/relay ./internal/relaysetting` 通过。
- `umask 0022; go test -race ./cmd/webmux ./internal/api ./internal/archtest` 通过。
- 默认 `umask 0077` 的首次整包运行触发既有配置迁移测试权限断言；标准权限环境通过，配置迁移实现未修改。
- `golangci-lint run ./cmd/webmux ./internal/api ./internal/relay ./internal/contract ./internal/archtest`：0 issues。
- 前端 `check`、`build`、`check:test`、`check:storybook`、`lint` 与源文件格式检查通过。
- `check:arch` 通过，架构测试 46 项；未放宽重复代码、依赖或样式基线。
- `relay-auth.test.ts` 与 `wire-contract.test.ts` 共 376 项通过，包含新增的 6 份 Relay wire 快照。
- `RelaySettings.test.ts` 浏览器组件 20 项通过，覆盖重复创建、导航恢复、断网恢复、明确终止、取消竞争与未知结果核对。
- `relay-settings.spec.ts` 桌面与 iPhone SE 共 6 项通过，覆盖刷新、导航、暂时查询失败、响应丢失、显式启用及删除关闭证据。

### 界面证据与边界

- Storybook 浅深色状态矩阵检查 112 组视口与状态组合，页面无横向溢出。
- 视口覆盖 1280×900、1280×720、768×800、390×844、375×667、320×568、667×375。
- 另检查展开诊断、危险操作与长错误文案，并刷新验证码有效期及旧访问警告的最终截图。
- 真实隔离嵌入应用的健康、初始配对阶段与操作回执均正确，未配对主区没有公网开关。
- 截图与交互录像位于本次会话交付物目录，使用隔离演示数据并保留桌面、平板、手机的浅深色前后对照。
- 本轮未代用户执行真实 GitHub 授权，未修改远端 Relay 或生产绑定。
- 无头浏览器未验证真实悬停、粗指针、iOS/Android 真机键盘及触控行为。

实现依据：[宿主控制器](../../cmd/webmux/relay_enrollment.go)、[页内设置](../../web/src/components/settings/RelaySettings.svelte)、[配对规格](../../openspec/specs/official-relay-pairing/spec.md)。

## Relay 接收端有效期简化（US-A10）

### 范围与根因

- 主机与 Relay 相差约 3 秒时，30 秒认证质询在接收端显示剩余 33 秒，被未来寿命上界误判为过期。
- 共享协议和宿主配对响应改为只检查绝对到期时间；签发端继续限制寿命，签名、单次 nonce、受众、代际、请求超时和心跳失联检测保留。
- 删除控制消息专用的 5 秒时钟容差；宿主固定引用本次发布的共享协议版本。

### 已执行验证

- `protocol/relay` 中执行 `go test -race ./...` 通过，覆盖落后 3 秒时的质询签名与验证、实际到期拒绝及控制消息未来时间。
- `go test -race ./internal/relay ./internal/relaysetting` 通过，覆盖带时钟偏差的签名分配和配对响应。
- `umask 0022; go test -race ./cmd/webmux -run TestRelay` 通过。
- `make web` 通过；前端检查为 0 errors、0 warnings，随后完成 Go 二进制构建。
- 隔离实例临时使用现有绑定连接真实 Relay，返回 `ready`、WSS `2/2`，控制面与数据面均为 `reachable`，节点为 `us1`；验收结束后停止隔离实例并清理临时状态。

此记录覆盖接收端寿命上界变更及真实建链；生产 Relay 服务端未在本轮重新部署，大幅时钟偏差仍需校时。

## 中转浏览器会话恢复（US-A08、US-A10）

### 行为与边界

- control 临时断线立即暂停业务、关闭旧 HTTP/WS/P2P/转发访问，保留有界内存恢复记录；原 Cookie 仅在控制面重新校验并授权当前 Scope 后恢复。
- 恢复保留 grant ID、后台会话、所有者、Origin 和原绝对期限；12 小时上限不变，关闭 Relay、明确撤销及宿主重启仍使旧登录失效。
- 中转门禁区分重新连接、过期与撤销，提供官方登录入口；返回路径限定同源应用页面，暂时恢复保留已挂载终端与未发送草稿，不重放业务操作。

### 已执行验证

- 共享 `protocol/relay` 的 `go test -race ./...` 通过。
- 宿主 `go test -race ./internal/relaysetting ./internal/relay ./internal/api ./internal/wire ./cmd/webmux` 通过；覆盖暂停后重授权、并发撤销、绝对期限、CSRF、Origin、返回路径及 PTY 保留。
- `go vet` 对上述宿主包通过，`golangci-lint run ./cmd/webmux ./internal/api ./internal/relaysetting ./internal/relay ./internal/contract ./internal/wire` 返回 0 issues。
- 前端 `relay-auth.test.ts`、`relay-auth-recovery.test.ts`、`ws.test.ts` 共 67 项通过；`RelayAuthGate` 和 `FeatureHostTransitions` 浏览器测试共 13 项通过。
- 隔离 PostgreSQL 17 下 store/control/dataplane/node race 与相关 go vet 全部通过；真实宿主集成脚本的冷启动/连接池、设备身份配对及离线解绑三项通过。控制包原撤销用例补齐恢复后 Scope 的关闭 ACK 后通过，未放宽关闭证明。
- `go test ./internal/api -run TestWireSnapshots -update` 与前端 wire 契约 360 项通过。
- 服务端源码 `72e02348bb` 已部署控制面及数据节点；公开就绪返回 `ok`，节点在线且 relay/coturn 均 ready，现有宿主重新达到 WSS 2/2。服务端 Actions 保持运营者既有的手动禁用状态。
- 首轮架构检查发现控制器直接调用服务级 Cookie 工具函数，已改为传输层设置 Cookie；相关 API、文档与 Go 架构守卫复测通过。

### 界面证据与限制

- 真实 Storybook 的重连、暂停检查、过期与撤销四种状态，在浅深主题和七种视口下共 56 项检查无页面横向溢出。
- 视口为 1280×900、1280×720、768×800、390×844、375×667、320×568、667×375；桌面、平板、手机前后截图及状态控件录像随 PR 提供。
- 截图使用隔离的旧 Token 门禁和新中转门禁；录像为 Storybook 场景切换，不代表真实 GitHub 授权或生产网络恢复。
- 无头浏览器未覆盖真实悬停、粗指针、iOS/Android 真机键盘；真实 GitHub 登录成功回调需用户登录态。

## 终端历史查询应答隔离（US-D03）

### 行为与根因

- Devin 会话历史快照保留启动时的 OSC 10/11 颜色查询；重新打开页面时 xterm 再次应答，旧响应进入程序输入框。
- 自动应答发送路径复用快照解析的 `replaying` 标记，历史查询的响应留在浏览器；实时查询与用户输入保持原路径。
- 本次只修改终端协议行为，布局、控件与文案保持不变。

### 已执行验证

- `pnpm --dir web exec playwright test test/e2e/terminal-replies.spec.ts`：桌面与 iPhone SE 两项通过。
- 隔离服务的真实 PTY 逐字节记录输入：连续两次刷新没有新增历史颜色响应，刷新后的用户输入及实时颜色响应均正常到达。
- `pnpm --dir web check` 与 `check:test`：0 errors、0 warnings；`check:arch` 通过，包含 46 项架构与依赖测试。
- 诊断原会话时拦截浏览器终端发送，观察到重复颜色应答；修复验收使用隔离服务，生产会话保持原样。
- 首轮 CI 发现新增正则触发控制字符 lint，已有 attention 用例要求历史查询回写；前者改用可读转义标记匹配，后者改为通过步骤文件触发实时查询，验证应答到达 PTY 后提醒仍保留，并验证重开页面没有额外应答。
- 修订后运行 `pnpm --dir web lint` 通过；Playwright 按 `opening a session does not clear attention|reopening suppresses` 筛选两个文件，桌面与 iPhone SE 共 4 项通过。

## 原生来源发现与身份绑定（US-Q19、US-Q20）

### 范围与证据

- 修复范围为 omp 头部发现与角色分类、Codex 0.169.0 格式及错误保真、agy cascade/trajectory 身份映射；发送与图片链路保持原实现。
- 旧夹具保留，新增本机合成会话的脱敏捕获；当前 omp 与 Codex 最小捕获没有工具调用，工具参数、结果和关联由旧实录及明确标记的合成回归覆盖。
- 本机命令输出为 `omp/18.22.0`、`codex-cli 0.169.0`、agy `1.5.7`。
- `opencode models --standalone` 列出 `opencode-go/deepseek-v4.1-flash`，隔离接收验收选用该模型。

### 已执行准备

- `pnpm --dir web install --frozen-lockfile` 通过。
- `pnpm --dir web build` 通过，Svelte 检查为 0 errors、0 warnings，生成隔离服务的嵌入资源。
- 验收配置使用独立临时 state_dir 与 loopback 监听；生产服务及既有会话保持原样，未读取 Token 或处理账号认证。

### 回归与静态验证

- `go test -race` 覆盖 `internal/historyimport`、`internal/codexrollout`、三个 agent history 包、`internal/historyprojection`、`internal/handoff`、`internal/featureapi`。
- 首轮仅 Codex 新增 reader 单测失败：该断言把上层冻结摘要校验误放到仅持字节边界的 reader；删除此重复断言后，Codex 包复测通过。RuntimeSource 集成回归保留并通过冻结内容修改拒绝断言。
- `go test -count=1 ./internal/docguard ./internal/archtest` 通过。
- 对上述来源包与 `internal/featureapi` 运行 `golangci-lint run`，结果为 0 issues。
- `CGO_ENABLED=0 go build -o /tmp/webmux-native-history-identity/webmux ./cmd/webmux` 通过，真实验收运行该二进制。

### 隔离真实 Agent 验收

三种来源首轮均正常 `end_turn`；运行时原生读取 HTTP 200、预览 HTTP 201、发送 HTTP 202，交接操作均为 `succeeded`。
目标使用已查询确认的 `opencode-go/deepseek-v4.1-flash`；当前任务没有包含答案，目标从历史恢复三项合成事实。

| 来源  | 来源会话   | 目标会话   | 目标 t1 实际回复                           |
| ----- | ---------- | ---------- | ------------------------------------------ |
| omp   | `ts3xug6o` | `eoyhmzm6` | `HANDOFF_OK WMX-omp-659ade63 青鹭 49367`   |
| Codex | `srg3zjhn` | `mncv2vh7` | `HANDOFF_OK WMX-codex-ccee309a 青鹭 49367` |
| agy   | `qm5soray` | `hdkwvzw7` | `HANDOFF_OK WMX-agy-4e929a7a 青鹭 49367`   |

- 三个指定目标 t1 均收到 `turn.end` / `end_turn`；回复逐项核对校验码、项目名及端口。
- 六个自建会话关闭后，逐个 GET 确认 `state=closed`、`status=exited`；随后隔离服务正常退出，退出码 0。
- 完整本地证据：`/tmp/webmux-native-history-identity/evidence-complete.json`；清理证据：同目录 `cleanup.json`。
- 此轮证明三种来源到 OpenCode 的文本交接；工具链由 reader 回归覆盖，图片交接与 Claude 账号认证不在本次验收范围。

## 移除沉浸式 VR 模式（US-H15、US-K18、US-L02）

### 范围与兼容性

- 删除 WebXR 会话、Three.js 场景、空间面板、VR 键盘及终端、聊天、代码与 Diff 的 XR 适配；TV 和头显 2D 模式保留。
- 删除 `immersive` / `immersive.xr`、中英文案、专用测试及 IWER/Three.js 依赖；US-H15 标记为已退役，保留历史编号。
- 显式包含 `immersive` 的旧配置需要删除该项。使用隔离配置实际启动，返回 `features.enabled: unknown feature "immersive"`。
- 能力快照调整为 13 个功能和 18 项能力，严格目录校验与静态发现回归通过。

### 已执行验证

- `make check` 通过，包括前端构建、类型与测试类型、格式、文档、持久化、架构、Oxlint、Go vet 和 golangci-lint。
- `umask 0022; NO_PROXY=localhost,127.0.0.1 no_proxy=localhost,127.0.0.1 make test` 通过：Go race 与 Relay、1249 项单元、935 项浏览器组件、137 项 Storybook、558 项 E2E（52 跳过）及 18 项性能测试（2 跳过）。
- 初始 `umask 0077` 使两个未主动设置文件权限的既有测试失败；改为 `0022` 后原用例与完整门禁通过。Storybook 预览探测受本地代理影响，设置本地地址 `NO_PROXY` 后 137 项通过。
- 一次高负载 E2E 运行中，代码阅读用例在创建 Shell 时收到 129；该用例随后在桌面和手机项目均单独通过，最终完整 E2E 为 558 通过、52 跳过。
- 真实隔离应用的桌面、平板、手机明暗主题前后截图使用相同会话、功能选择与菜单状态；所有删除后视口保留 TV/头显入口、没有 VR 入口，且无页面横向溢出。
- 未验证真实头显/TV 硬件、真实悬停或粗指针行为。本次为静态入口删除，无交互动画 GIF。
