# 核对记录：2026-11-18 起

承接 [verification-log-2026-11-17](verification-log-2026-11-17.md)（已接近 400 行文件上限），收录 2026-11-18 起的局部核对记录；索引见 [WebMux User Stories](../user-stories.md#核对记录)。

## Kimi TUI 直接使用后的归档恢复（US-Q12）

- 根因是候选型 agent 的原生 ID 只在切到聊天时写入记录；Kimi TUI 未切换便关闭时，恢复把空 ID 当作新对话处理。恢复现在按原记录创建时间重新判定候选，唯一候选直接恢复，多个候选返回 `session_ambiguous` 并复用现有选择弹窗，没有候选才开始新对话。
- 回归覆盖 `TestKimiResumeTUIOnlySession` 与 `TestKimiResumeTUIOnlySessionAmbiguous`；前者在修复前得到空的原生 ID，修复后恢复原 ID 与终端历史，后者验证两个候选的选择与恢复。新增 Playwright 用例经真实 HTTP 与页面完成同样的多候选流程。
- 本地已通过 `make check`、`go test -race -count=1 ./internal/hybrid`、`go test -race -count=1 ./...`、1257 项前端单元测试及 20 项归档浏览器组件测试。完整 E2E 的 592 项通过、56 项按条件跳过；`TerminalTouch` 的并发时序失败单独复跑通过。新增 E2E、Storybook 与性能套件交由 PR CI 验证。
- 首轮 PR CI 的 Go race、格式构建、Vitest、Storybook、性能及打包检查通过；新增桌面与手机 E2E 均发现 409 已返回两个候选，但临时 shell 的规范读取先卸载了非 `resumeOnOpen` 恢复组件。规范读取期间现在保留所有当前恢复记录，最终结果仍负责清除失效记录；首轮其余 E2E 分片通过。
- 同步更新 `docs/hybrid-sessions/agents.md`、`docs/protocol/http-hybrid.md`、`openspec/specs/agent-session-identity/spec.md`、`openspec/specs/hybrid-session-recovery/spec.md` 与 US-Q12；故事编号、总数和覆盖索引保持原状。

## 2026-11-19：批量管理工作树返回原选择行（US-J14）

- 隔离 Chromium 实测选择→预览→返回，以及确认→长结果→返回。验证桌面、平板、手机与手机减少动态效果。
- `RepositoryPanel.test.ts` 及 `worktrees.spec.ts` 的批量清理用例通过，E2E 覆盖桌面及 iPhone SE Chromium。
- 本轮使用无头 Linux Chromium，未在物理触屏或真机浏览器核对。

## 2026-11-18：会话上传读取响应头安全收敛（US-Q15）

### 范围

- 故事编号、总数（264）与覆盖索引保持原状，更新 US-Q15：
  - `internal/pathutil/pathutil.go`（`SniffContentType` / `IsAVIF`）：新增 ISO BMFF `ftyp`（`avif` / `avis`）识别并与 `internal/codeview/media.go` 复用同一判定；
  - `internal/featureapi/upload.go`（`readUpload`）：光栅图片白名单（`image/png`、`image/jpeg`、`image/gif`、`image/webp`、`image/avif`、`image/bmp`、`image/x-icon`）保留嗅探 `Content-Type` 与 `Content-Disposition: inline`；
  - 其余类型（含 `text/html`、`text/xml`、XML/SVG、`application/pdf`、纯文本及二进制）统一返回 `Content-Type: application/octet-stream` 与 `Content-Disposition: attachment`；
  - 所有 `GET` / `HEAD /api/sessions/{id}/upload` 成功响应固定附加 `Content-Security-Policy: default-src 'none'; sandbox`；
  - 同步更新 `docs/protocol/http-sessions.md`、`docs/user-stories/hybrid-sessions/composition-and-handoff.md`（US-Q15）及 `docs/user-stories/hybrid-sessions.md` 依据链接。

### 已执行验证与局限

- Go 与静态门禁：
  - `go test -race -count=1 ./internal/pathutil ./internal/codeview ./internal/featureapi ./internal/artifacts ./internal/archtest ./internal/docguard` 通过；
  - `go test -race -count=1 -parallel=4 ./internal/api`（含 `TestReadUploadServesSessionUploadedFileWithSniffedMediaType`、`TestReadUploadForcesAttachmentAndSandboxForNonRasterFiles`、`TestReadUploadTopLevelNavigationBlocksScriptExecutionAcrossAuthModes`）通过；
  - `make go-lint`（`golangci-lint` v2.15.0，0 issues）及 `go vet`（Linux、`GOOS=darwin`、Windows 托盘/客户端目标）通过。
- 前端与端到端回归：
  - Vitest 浏览器组件测试 `ChatView.test.ts`、`ChatUpload.test.ts`（共 53 项）通过；
  - Playwright 端到端测试 `upload.spec.ts`、`artifacts.spec.ts`（14 项通过、4 项按设备项目条件跳过）及 `hybrid.spec.ts -g "Claude sends image and embedded-resource attachments"`（桌面与 iPhone SE 共 2 项通过）验证跨请求加载图片缩略图正常。
  - 终端上传进度浮层（US-E06）：`TerminalViewport.test.ts`、`TerminalUpload.test.ts` 与 `TerminalNotices.test.ts`（共 22 项）通过，验证桌面（1280×720）与手机（390×844）视口下空闲状态全高保留、上传期间顶部浮层展示与取消、查找栏打开时自动下移避让，以及开始、完成与取消前后字符网格尺寸保持一致。
  - `upload.spec.ts` 的桌面与 iPhone SE Chromium 用例覆盖原始文件、回放、拖放、进度、错误和取消。上传首帧及完成时的字符网格尺寸作为回归断言。
- 局限：
  - 本机免密与 Relay 浏览器顶层导航的非执行行为通过 HTTP 响应头契约（`application/octet-stream`、`attachment`、`nosniff` 与 `sandbox` CSP）回归验证，未连接公网 Relay 节点演练；
  - 浏览器与端到端用例仅在 Linux 无头 Chromium 中执行，未在 macOS / iOS / Android 真机浏览器上复测。

## T045 终端查找与回放提示（US-D11）

- 查找栏在查询为空且未回放时省略状态行保持紧凑覆盖高度；有查询或回放提示时仅挂载单个命名的 `role="status"`（`查找结果`），切换匹配结果与回放提示期间高度保持一致，并在回放完成前后保留输入框或按钮焦点。
- `TerminalRendering.test.ts` 与 `terminal.spec.ts` 的桌面和 iPhone SE Chromium 用例覆盖紧凑初始高度、单一状态播报、回放焦点保留、匹配复制和关闭后的 PTY 输入。

## Codex 原生历史版本兼容（US-Q19、US-Q20）

- 修复前，`go test -count=1 ./internal/agent/codex/history -run '^TestRuntimeDiscoveryCurrentCaptureAndFreeze$'` 在三个新增版本上均复现 byte 0 格式错误。
- 本机通过临时只读探针 `go test -count=1 -v ./internal/agent/codex/history -run '^TestLocalRolloutCompatibilityProbe$'` 按日志写入版本核验。修复后通过 1 份 0.160.1、14 份 0.167.2、5 份 0.169.1，另有 1 份 0.160.1 触发既有资源上限。探针仅输出统计。
- 回归测试使用公开夹具正文和派生版本头验证较新版本、运行时来源发现、冻结后补查及同一身份冲突。无效写入版本和引用式分支返回不同格式错误。
- `go test -race -count=1 ./internal/codexrollout ./internal/agent/codex/history ./internal/historyimport ./internal/historysource ./internal/historyprojection ./internal/handoff` 六个包通过，覆盖默认分页图片、任务原文、来源冻结与预览到期。
- `make docs-check` 通过。

## T047 草稿恢复操作（US-F05、US-F06）

- 窄栏常规草稿编辑在无提交状态且无不可发送提示时不渲染状态行，保留完整编辑区高度；出现提交状态或不可发送提示时状态行以 `max-h-[2lh]` 展示。
- `TerminalDraft.test.ts` 浏览器用例验证重发原提交与转为草稿在 `tick()` 解除只读后于非触屏环境恢复编辑区焦点并保留选区，在 `(any-pointer: coarse)` 触屏环境下保持不自动聚焦（US-H02）。
- `composer.spec.ts` 的桌面和 iPhone SE Chromium 用例覆盖真实 PTY 输入隔离和草稿编辑。未测试真机软键盘。
- 320×568 与 200% 字号下，恢复按钮换行且全部在可用宽度内。草稿和满暂存列表的 2 项 Storybook 溢出回归通过。

## 原生历史最低版本基线（US-Q19、US-Q20）

- `go test -race -count=1 ./internal/codexrollout ./internal/agent/{codex,kimi,omp,qoder,qodercn,opencode,agy,claudecode,devin}/history ./internal/{historyimport,historysource,historyprojection,handoff}` 的 14 个包通过。
- 派生样本覆盖 Codex 0.170.0 和 9.99.0、Kimi Wire 1.10 和 2.0 与状态 v3、omp session v4、Qoder/QoderCN 较高版本，并核对正文和工具关联。OpenCode 在同一数据库内读取两种 SQLite 表结构。

## T049 终端端口提示（US-R04）

- `TerminalNotices.test.ts` 通过，验证晚发现的较小端口保留现有按钮位置与焦点，并可点击「查看全部」。
- `port-forwarding.spec.ts` 的桌面与 iPhone SE Chromium 两项通过。隔离 Chromium 中交错发现三个端口并输入 abc，字符按顺序交付 xterm。

## 侧栏添加仓库入口（US-C16）

- 「添加仓库」从「全部仓库」行菜单移到侧栏底部；按会话、按分支、窄栏及移动端抽屉共用登记弹窗和可用性条件。
- `make e2e-runtime` 构建通过。`WorkbenchTree.test.ts` 的 16 项浏览器用例通过；`page-stability.spec.ts`、`workspace-discovery.spec.ts` 和 `worktrees.spec.ts` 在桌面与 iPhone SE Chromium 上共 50 项通过。
- 隔离生产页面核对了 7 个视口的明暗主题、相邻按钮尺寸、窄栏与键盘打开，以及登记弹窗边界。未验证物理触屏或真机。
- PR 提供桌面、平板及手机的明暗前后截图，以及[手机打开与取消登记的录屏](../images/pr/sidebar-add-repository/interaction-phone.gif)。

## T051 矮窗格的选区与回到底部（US-D07）

- `TerminalViewport.test.ts` 通过，验证短窗格回放后悬浮热区分离，终端宿主几何一致。
- 隔离 Chromium 的 390 px 宽、150 px 高终端窗格中，按钮在回放恢复后无重叠。复制保留选区，回到底部保留终端输入焦点。

## 按会话侧栏的分组显示（US-C17）

- 会话缩进排列在项目标题下；无匹配会话的项目隐藏。侧栏顶部的「新建会话」仍可为这些项目创建首个会话。
- `make e2e-runtime` 成功；`session-creation.spec.ts` 与 `workbench-session-queue.spec.ts` 的相关桌面和 iPhone SE Chromium 用例共 6 项通过。
- 隔离 Chromium 中核对了 6 个视口的明暗主题、长标题和页面横向溢出，以及桌面键盘焦点与悬停提示。未验证实体触屏或真机。
- [手机上为隐藏的项目创建首个会话](../images/pr/sidebar-session-groups/interaction-phone.gif)。

## 2026-11-19：启动和回合改动的阅读位置（US-Q18、US-B01、US-Q05）

- 隔离 Chromium 复测 T026、T030、T037、T038。320×568 下，更新初始化指令引用提示时，输入与发送按钮坐标保持稳定，浮层留在视口内。
- 320×568 的项目列表与运行设置往返后，分别恢复项目列表滚动和设置页的 205px 滚动位置，并保留搜索、命令、名称及配置焦点。
- 隔离浏览器展开两个补丁或换到单补丁时，三个文件头的坐标一致。服务端假 Agent 的 Playwright 探针也确认桌面及移动 Chromium 展开两个补丁时，所选文件头的坐标保持不变。
- 缺快照、计算中与着色差异共用 256px 桌面边界。快照替换后保留第 19 行选区和首个可见行。
- `make e2e-runtime` 构建成功。`playwright test` 在 `quick-start.spec.ts`、`session-creation.spec.ts`、`hybrid.spec.ts` 的相关桌面与手机场景通过 19 项，跳过 1 项桌面快捷键用例的手机版。
- `pnpm --dir web test:unit test/turn-changes.test.ts test/chat-model.test.ts test/code-diff.test.ts` 的 38 项通过。减少动态效果模式在 320×568 独立复测。触屏交互使用 Chromium 模拟，实体设备和悬停待真机验证。

## 2026-11-19：关闭配置文件功能开关并固定启用全部内置模块（US-K18 退役、US-K03）

- 故事编号与总数（264）保持不变，将 US-K18 标为已退役并同步覆盖索引：
  - `deploy/config.example.toml` 移除 `[features]` 段，`internal/config/config.go` 将 `Config.Features` 设为 `toml:"-" json:"-"` 并移除 `[plugins]` 报错与 `cmd/webmux/config_migrate.go`；
  - 磁盘 `config.toml` 中遗留的 `[features]`（含 `enabled = []` 或旧 `immersive`）与 `[plugins]` 段在解析时直接忽略，原文件保持原状且固定启用全部 13 项内置功能；
  - `internal/config/view.go` 从配置诊断白名单移除 `features.enabled`；`internal/builtin`、`Config.Features`、`featureapi`/`featurehub`/`featurewire` 与前端 `src/features` 基建保留供程序化功能隔离测试使用；
  - 同步更新 `openspec/specs/{builtin-feature-lifecycle,effective-configuration}/spec.md`、`docs/deployment/{configuration,install-and-upgrade}.md`、`docs/{overview,faq,architecture,persistence,user-stories}.md` 与 `docs/user-stories/coverage.md`。
- 已执行验证：
  - `go test -race -count=1 ./cmd/... ./internal/config ./internal/builtin ./internal/archtest ./internal/docguard` 通过；
  - `pnpm --dir web check`、`pnpm --dir web check:test`、`pnpm --dir web lint`、`pnpm --dir web check:arch`、`make go-lint`（0 issues）、`go vet ./...`、`GOOS=darwin go vet ./...` 与 `make format-check` 通过；
  - Playwright E2E `agent-tools.spec.ts`、`relay-settings.spec.ts` 与 `worktrees.spec.ts`（直接目录创建降级）共 10 项通过。

## 2026-11-19：手机单行聊天输入框的上下文用量（US-Q05）

- `pnpm --dir web exec vp test run --config vitest.browser.config.ts test/browser/ChatView.test.ts` 中按用量与工具栏筛选运行的 4 项通过，覆盖 320×568 下从编辑态收回单行后打开用量明细。
- `pnpm --dir web check`、`pnpm --dir web lint`、`pnpm --dir web check:arch` 与源码格式检查通过。
- 临时 Vitest Chromium 界面探针在 7 个视口的浅色与深色主题验证明细打开、Escape 关闭与视口边界；未在真机上验证。

## 侧栏会话通知与待处理筛选（US-C06、US-C17）

- 侧栏「会话通知」仅在存在待处理、已暂缓事项或收件箱已打开时挂载；处理完最后一项后按 Esc 关闭空收件箱，焦点移回侧栏导航。「全部」与「待处理」保持 1:1 等宽排布，无待处理项时省略空计数占位并居中显示标签，有匹配时以等宽数字显示数量。
- 聚焦的末项解除后回到「待处理」，按方向键可继续选择「全部」。
- 运行 `pnpm --dir web exec vp test run --config vitest.browser.config.ts test/browser/AppShell.test.ts` 验证条件入口、空收件箱焦点移交、筛选栏等宽排布与主机状态单行传输路径。
- 隔离后端运行 `pnpm --dir web exec playwright test test/e2e/attention.spec.ts test/e2e/workbench-session-queue.spec.ts` 覆盖末项焦点与侧栏菜单。已在 Linux Chromium 验证。

## 侧栏分组切换使用纯图标（US-C16）

- 侧栏与抽屉的分组切换在各宽度下均显示图标，保留悬停提示、方向键切换和触控尺寸。
- `pnpm --dir web exec playwright test app-shell.spec.ts sidebar-grouping-evidence.spec.ts` 的定向子集：25 项通过，5 项按设备项目跳过。覆盖 100%、150%、200% 字号、320px 触控切换和减少动态效果。
- 临时视觉用例在隔离工作台核对 7 个视口的明暗主题、页面横向溢出、抽屉边界及键盘焦点。悬停核对命中状态与提示属性；未验证系统提示的实际绘制或物理触屏。

## 异步会话切换结果（US-C18）

- 三端 Chromium 复现空结果与聊天命中之间的输入框位移，桌面差值 168px。修复后桌面结果区保持 400px，手机保持 422px，键盘选中项仍可滚动进入视口。Esc 恢复原输入、焦点和文字选区。
- `pnpm --dir web exec vp test run --config vitest.browser.config.ts test/browser/SessionSwitcher.test.ts` 通过。隔离后端执行 `pnpm --dir web exec playwright test test/e2e/session-switcher.spec.ts` 通过，涵盖终端焦点和输入。

## 侧栏主机状态（US-A04）

- 隔离 Chromium 以本地夹具切换长短主机名，并控制健康探针、P2P RTT 和 Relay 路径。常规连接仅渲染单行传输路径，Relay 已认证会话预留混合传输行高，测量中、9→2999ms 和重连时延时行位置保持一致。
- 320px 视口在 200% 字号下可打开并滚动详情，重连状态即时更新。P2P 和 Relay 数据由夹具注入了客户端状态。
- 连接播报使用简短状态文本。修正重复重连文案后，隔离后端的 `workspace-discovery.spec.ts -g 'a disconnected session list reconnects'` 在桌面与手机 Chromium 均通过。
- `workspace-ui.spec.ts` 在主机状态按钮中核对长短主机名和首个操作的几何，并单独核对状态区域。与 `workspace-discovery.spec.ts` 合计 16 通过，2 个桌面专属用例在手机项目跳过。
