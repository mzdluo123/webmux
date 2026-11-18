# 核对记录：2026-11-18 起

承接 [verification-log-2026-11-17](verification-log-2026-11-17.md)（已接近 400 行文件上限），收录 2026-11-18 起的局部核对记录；索引见 [WebMux User Stories](../user-stories.md#核对记录)。

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

## T047 草稿恢复操作（US-F05）

- 隔离 Chromium 模拟 390 × 390 可视区域，发送、部分写入、结果未知、重发、转回草稿和取消清空前后的编辑区高度均为 32 px，选区保留在 8–14。
- `composer.spec.ts` 的桌面和 iPhone SE Chromium 用例覆盖真实 PTY 输入隔离和草稿编辑。未测试真机软键盘。

## 原生历史最低版本基线（US-Q19、US-Q20）

- `go test -race -count=1 ./internal/codexrollout ./internal/agent/{codex,kimi,omp,qoder,qodercn,opencode,agy,claudecode,devin}/history ./internal/{historyimport,historysource,historyprojection,handoff}` 的 14 个包通过。
- 派生样本覆盖 Codex 0.170.0 和 9.99.0、Kimi Wire 1.10 和 2.0 与状态 v3、omp session v4、Qoder/QoderCN 较高版本，并核对正文和工具关联。OpenCode 在同一数据库内读取两种 SQLite 表结构。

## 侧栏添加仓库入口（US-C16）

- 「添加仓库」从「全部仓库」行菜单移到侧栏底部；按会话、按分支、窄栏及移动端抽屉共用登记弹窗和可用性条件。
- `make e2e-runtime` 构建通过。`WorkbenchTree.test.ts` 的 16 项浏览器用例通过；`page-stability.spec.ts`、`workspace-discovery.spec.ts` 和 `worktrees.spec.ts` 在桌面与 iPhone SE Chromium 上共 50 项通过。
- 隔离生产页面核对了 7 个视口的明暗主题、相邻按钮尺寸、窄栏与键盘打开，以及登记弹窗边界。未验证物理触屏或真机。
- PR 提供桌面、平板及手机的明暗前后截图，以及[手机打开与取消登记的录屏](../images/pr/sidebar-add-repository/interaction-phone.gif)。

## 按会话侧栏的分组显示（US-C17）

- 会话缩进排列在项目标题下；无匹配会话的项目隐藏。侧栏顶部的「新建会话」仍可为这些项目创建首个会话。
- `make e2e-runtime` 成功；`session-creation.spec.ts` 与 `workbench-session-queue.spec.ts` 的相关桌面和 iPhone SE Chromium 用例共 6 项通过。
- 隔离 Chromium 中核对了 6 个视口的明暗主题、长标题和页面横向溢出，以及桌面键盘焦点与悬停提示。未验证实体触屏或真机。
- [手机上为隐藏的项目创建首个会话](../images/pr/sidebar-session-groups/interaction-phone.gif)。

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
