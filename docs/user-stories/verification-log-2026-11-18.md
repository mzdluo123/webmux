# 核对记录：2026-11-18 起

承接 [verification-log-2026-11-17](verification-log-2026-11-17.md)（已接近 400 行文件上限），收录 2026-11-18 起的局部核对记录；索引见 [WebMux User Stories](../user-stories.md#核对记录)。

## Kimi TUI 直接使用后的归档恢复（US-Q12）

- 根因是候选型 agent 的原生 ID 只在切到聊天时写入记录；Kimi TUI 未切换便关闭时，恢复把空 ID 当作新对话处理。恢复现在按原记录创建时间重新判定候选，唯一候选直接恢复，多个候选返回 `session_ambiguous` 并复用现有选择弹窗，没有候选才开始新对话。
- 回归覆盖 `TestKimiResumeTUIOnlySession` 与 `TestKimiResumeTUIOnlySessionAmbiguous`；前者在修复前得到空的原生 ID，修复后恢复原 ID 与终端历史，后者验证两个候选的选择与恢复。新增 Playwright 用例经真实 HTTP 与页面完成同样的多候选流程。
- 本地已通过 `make check`、`go test -race -count=1 ./internal/hybrid`、`go test -race -count=1 ./...`、1257 项前端单元测试及 20 项归档浏览器组件测试。完整 E2E 的 592 项通过、56 项按条件跳过；`TerminalTouch` 的并发时序失败单独复跑通过。新增 E2E、Storybook 与性能套件交由 PR CI 验证。
- 同步更新 `docs/hybrid-sessions/agents.md`、`docs/protocol/http-hybrid.md`、`openspec/specs/agent-session-identity/spec.md`、`openspec/specs/hybrid-session-recovery/spec.md` 与 US-Q12；故事编号、总数和覆盖索引保持原状。

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
- 局限：
  - 本机免密与 Relay 浏览器顶层导航的非执行行为通过 HTTP 响应头契约（`application/octet-stream`、`attachment`、`nosniff` 与 `sandbox` CSP）回归验证，未连接公网 Relay 节点演练；
  - 浏览器与端到端用例仅在 Linux 无头 Chromium 中执行，未在 macOS / iOS / Android 真机浏览器上复测。

## Codex 原生历史版本兼容（US-Q19、US-Q20）

- 修复前，`go test -count=1 ./internal/agent/codex/history -run '^TestRuntimeDiscoveryCurrentCaptureAndFreeze$'` 在三个新增版本上均复现 byte 0 格式错误。
- 本机通过临时只读探针 `go test -count=1 -v ./internal/agent/codex/history -run '^TestLocalRolloutCompatibilityProbe$'` 按日志写入版本核验。修复后通过 1 份 0.160.1、14 份 0.167.2、5 份 0.169.1，另有 1 份 0.160.1 触发既有资源上限。探针仅输出统计。
- 回归测试使用公开夹具正文和派生版本头验证较新版本、运行时来源发现、冻结后补查及同一身份冲突。无效写入版本和引用式分支返回不同格式错误。
- `go test -race -count=1 ./internal/codexrollout ./internal/agent/codex/history ./internal/historyimport ./internal/historysource ./internal/historyprojection ./internal/handoff` 六个包通过，覆盖默认分页图片、任务原文、来源冻结与预览到期。
- `make docs-check` 通过。

## 原生历史最低版本基线（US-Q19、US-Q20）

- `go test -race -count=1 ./internal/codexrollout ./internal/agent/{codex,kimi,omp,qoder,qodercn,opencode,agy,claudecode,devin}/history ./internal/{historyimport,historysource,historyprojection,handoff}` 的 14 个包通过。
- 派生样本覆盖 Codex 0.170.0 和 9.99.0、Kimi Wire 1.10 和 2.0 与状态 v3、omp session v4、Qoder/QoderCN 较高版本，并核对正文和工具关联。OpenCode 在同一数据库内读取两种 SQLite 表结构。

## 侧栏添加仓库入口（US-C16）

- 「添加仓库」从「全部仓库」行菜单移到侧栏底部；按会话、按分支、窄栏及移动端抽屉共用登记弹窗和可用性条件。
- `make e2e-runtime` 构建通过。`WorkbenchTree.test.ts` 的 16 项浏览器用例通过；`page-stability.spec.ts`、`workspace-discovery.spec.ts` 和 `worktrees.spec.ts` 在桌面与 iPhone SE Chromium 上共 50 项通过。
- 隔离生产页面核对了 7 个视口的明暗主题、相邻按钮尺寸、窄栏与键盘打开，以及登记弹窗边界。未验证物理触屏或真机。
- PR 提供桌面、平板及手机的明暗前后截图，以及[手机打开与取消登记的录屏](../images/pr/sidebar-add-repository/interaction-phone.gif)。
