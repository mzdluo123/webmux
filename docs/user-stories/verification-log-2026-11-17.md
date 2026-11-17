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

## 独立频道消息初版（US-S26、US-K18–US-K21）

### 行为

- 新增显式选择的 `messaging` 功能，在「设置 → 频道消息」创建频道、分页读取本机历史并幂等发送。
- 后端独立实现频道和消息存储，接口组织参考 Relaycast；状态目录中保存私有快照与生命周期锁。
- 固定旧版 14 项选择保留；禁用时不加载消息实现或打开消息存储。文档见[频道消息](../usage/messaging.md)。

### 已执行验证

- `umask 0022; go test -race ./cmd/webmux ./internal/api ./internal/messaging ./internal/featureapi ./internal/featurewire ./internal/config ./internal/builtin ./internal/localclient ./internal/mcpcontrol ./internal/statefile ./internal/archtest ./internal/docguard`：12 个包通过。
- 首轮现有迁移测试依赖 0640 权限，受本机 umask 影响失败；按该测试预期的 umask 重跑后通过，迁移代码保持原样。
- `pnpm --dir web check`、`check:test`、`lint`、`check:arch`：通过，包含 46 项架构与依赖测试；前端生产构建通过。
- `pnpm --dir web exec vp test run --config vitest.browser.config.ts test/browser/MessagingPanel.test.ts test/browser/SettingsNavigation.test.ts`：12 项通过，覆盖创建频道、历史分页、频道草稿、发送失败与过期响应。
- `pnpm --dir web exec playwright test test/e2e/messaging.spec.ts`：桌面与 iPhone SE 共 6 项通过，覆盖创建、发送、重启后读取、Token 与 Origin 拒绝。
- 功能选择 E2E 的 40 项通过，包含独立 `messaging` 和固定旧版选择；`test/capabilities.test.ts`、`test/wire-contract.test.ts`、`test/wire-arrays.test.ts` 的 437 项通过。
- `go vet` 对涉及的后端包通过；使用 Go 1.27.4 构建的固定 golangci-lint v2.15.0 检查全仓库，0 issues。
- 真实隔离服务检查 1280×900、768×800、390×844、375×667、320×568、667×375 六种视口，浅色/深色共 12 组均无页面横向溢出；320px 下长连续文本可换行。
- [桌面浅色](../images/messaging/before-desktop-light.png)、[桌面深色](../images/messaging/before-desktop-dark.png)来自真实消息面板。
- [平板浅色](../images/messaging/before-tablet-light.png)、[平板深色](../images/messaging/before-tablet-dark.png)来自真实消息面板。
- [手机浅色](../images/messaging/before-phone-light.png)、[手机深色](../images/messaging/before-phone-dark.png)来自真实消息面板。
- [创建与发送录像](../images/messaging/before-create-send.gif)使用隔离频道，展示创建、键盘 Tab 焦点与 Enter 发送；发送按钮的 focus-visible 已验证。
- 无头浏览器未验证真实悬停、粗指针与 iOS/Android 真机键盘。

## User-first 指南与消息工作区（US-L09、US-S26）

- 设计系统和 `webmux-ui` skill 以用户目的、任务路径、操作可发现性、上下文连续性和真实反馈为通用指导，替换原有克制风格指导；skill 验证通过。
- 消息移到侧栏独立入口与 `/messages`，桌面显示频道导航和对话，手机在列表与对话之间进入和返回；原设置入口随迁移移除。
- 创建频道按需弹出，搜索与最近消息预览辅助定位；底部输入区支持 Enter、换行与输入法组合，失败内容就近重试。
- 最近频道在当前浏览器按主机保存；当前页面保留频道草稿与阅读位置。日期、本地时间、头像与消息反馈支持阅读。
- 修复 InputGroup 对禁用发送按钮的整体淡化：输入仍可编辑时保持正常外观，新增真实浏览器回归和 Storybook 示例。
- `pnpm --dir web check:test`、`check:storybook`、`lint`、`check:arch` 与生产构建通过；架构和依赖检查共 46 项通过。
- 消息工作区与设置导航的 13 项浏览器测试通过，覆盖默认阅读、创建弹窗、搜索、手机返回、草稿、历史分页、重试身份和输入法保护。
- 消息与功能选择 E2E 共 46 项通过；消息场景验证重启后恢复最近频道与持久历史、服务端频道预览、Token 与 Origin 拒绝。
- `go test -race ./internal/clicmd ./internal/messaging ./internal/featureapi ./internal/archtest ./internal/docguard` 通过；固定 golangci-lint 全仓库 0 issues。
- 同步主分支通用 UI 壳迁移，修复初版合并 CI 中的组件路径失效；补齐 CLI 能力夹具的新功能映射并检查漏登。
- 最终真实服务检查六种视口、浅深主题共 12 组，无页面横向溢出，输入区和发送按钮均留在视口内；320×568 的创建浮层完整可见。
- [桌面](../images/messaging/desktop-light.png)、[平板](../images/messaging/tablet-light.png)、[手机对话](../images/messaging/phone-light.png)和[手机频道列表](../images/messaging/phone-list-light.png)为最终工作区。
- [最终创建与发送录像](../images/messaging/create-send.gif)展示创建弹窗、键盘聚焦与 Enter 发送，正常速度播放；无头验证未覆盖真机键盘、真实悬停或粗指针。

完整快速门禁：`umask 0022; make test-fast` 通过，包含全仓库 Go race、97 个 Node 测试文件的 1295 项测试，以及 127 个浏览器测试文件的 984 项测试。

## Agent 消息插件与主动唤醒（US-S26）

- 为 9 种内置混合 Agent 通过 ACP new/load/resume 自动注入独立命名的消息 MCP 插件；逐个验证描述符、环境凭据、会话与产品身份绑定。
- 插件提供发现频道、加入/离开、历史、未确认收件箱、发送与处理确认；通过真实 stdio 子进程完成读取、发送和确认，验证同 ID 重试与身份隔离。
- 新任务持久化后触发空闲处理，忙碌时按频道合并通知；普通 Agent 回复保持安静，显式请求可唤醒其他 Agent，自身消息从自己的收件箱排除。
- 处理游标持久化，读取保留游标；验证重启恢复、重新绑定使旧凭据失效、重连补投、等待取消及到达竞态，失败通知限速重试。
- 页面可邀请和移除聊天模式中的 Agent，每两秒自动同步回复并保留草稿；纯终端预设尚未接入自动唤醒，商业 Agent 的实际版本尚未联调。
- 后端 race 与架构检查通过；新增混合会话通知队列测试通过，插件在本机 API 的往返测试包含网络入口拒绝与伪造发送者入参拒绝。
- 桌面与手机的真实 stdio E2E 均通过，覆盖邀请、忙碌排队、回复确认、随后空闲唤醒与页面自动更新。
- Svelte/测试类型、Oxlint、前端架构门禁与格式检查通过；固定 golangci-lint v2.15.0 检查全仓库，0 issues。CLI 帮助完整性检查发现缺少第二个例子，补齐后该检查通过。
- 最终 `umask 0022; make test-fast` 全部通过：全仓库 Go race、97 个 Node 文件的 1295 项测试、128 个浏览器文件的 987 项测试。
- 成员浮层检查桌面、平板、390×844、375×667、320×568、667×375 六种视口与浅深主题共 12 组，均在视口内且无页面横向溢出；工作区同步完成相同范围检查。
- [桌面成员](../images/messaging/agents-desktop-light.png)、[平板成员](../images/messaging/agents-tablet-light.png)、[手机成员](../images/messaging/agents-phone-light.png)及[邀请与收发录像](../images/messaging/agents-invite-send.gif)来自真实隔离应用与 stdio 假 Agent；无头检查未覆盖真机键盘及粗指针。
- 消息、插件与功能选择 E2E 最终共 48 项通过；手机空状态的两个创建动作改用不同名称后，创建、发送、重启恢复路径通过。
- 补充 101 条安静 Agent 状态消息后的新请求回归：通知扫描独立于 100 条收件箱分页，后续任务仍能唤醒订阅 Agent。

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

## 同步主分支 VR 移除

合入 `cdcf67d3`，保留 Agent 消息插件与任务式 UI 指南；当前目录为 14 项，省略选择时保留的旧版集合为 13 项。
功能目录、能力数量、v1 本机客户端兼容、前端契约与部署文档同步更新；此前验证记录保留其执行时的数量。
合并后相关 Go race（目录、本机客户端、API、CLI、MCP、架构、文档）、前端类型/架构、格式和生产构建通过。
类型检查的旧 VR 生成缓存移至临时目录后重新生成，源文件无额外修改。
CI 的全仓格式检查发现 `architecture.md` 与频道消息说明的表格格式未对齐；按同一 Oxfmt 配置修复，并补跑包含文档的 `make format-check`。
