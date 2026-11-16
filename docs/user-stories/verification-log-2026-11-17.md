# 局部核对记录（2026-11-17 起）

承接 [verification-log-2026-11-15](verification-log-2026-11-15.md)（已接近 48 KiB 文件上限），收录 2026-11-17 起的局部核对记录；索引见 [WebMux User Stories](../user-stories.md#核对记录)。

## 会话上传读取响应头安全收敛（2026-11-17，US-Q15）

- 范围（故事编号、总数与覆盖索引保持原状）：
  - `internal/featureapi/upload.go`（`readUpload`）：光栅图片白名单（`image/png`、`image/jpeg`、`image/gif`、`image/webp`、`image/avif`、`image/bmp`、`image/x-icon`）保留嗅探 `Content-Type` 与 `Content-Disposition: inline`；
  - 其余类型（含 `text/html`、`text/xml`、XML/SVG、`application/pdf`、纯文本及二进制）统一返回 `Content-Type: application/octet-stream` 与 `Content-Disposition: attachment`；
  - 所有 `GET` / `HEAD /api/sessions/{id}/upload` 成功响应固定附加 `Content-Security-Policy: default-src 'none'; sandbox`；
  - 同步更新 `docs/protocol/http-sessions.md`、`docs/user-stories/hybrid-sessions/composition-and-handoff.md`（US-Q15）及 `docs/user-stories/hybrid-sessions.md` 依据链接。
- 本地验证：
  - `go test -race -count=1 ./internal/featureapi ./internal/artifacts ./internal/archtest ./internal/docguard` 通过；
  - `go test -race -count=1 -parallel=4 ./internal/api`（含 `TestReadUploadServesSessionUploadedFileWithSniffedMediaType`、`TestReadUploadForcesAttachmentAndSandboxForNonRasterFiles`、`TestReadUploadTopLevelNavigationBlocksScriptExecutionAcrossAuthModes`）通过；
  - `make go-lint`（`golangci-lint` v2.15.0，0 issues）及 `go vet`（Linux、`GOOS=darwin`、Windows 托盘/客户端目标）通过；
  - Vitest 浏览器组件测试 `ChatView.test.ts`、`ChatUpload.test.ts`（共 53 项）通过；
  - Playwright 端到端测试 `upload.spec.ts`、`artifacts.spec.ts`（14 项通过、4 项按设备项目条件跳过）及 `hybrid.spec.ts -g "Claude sends image and embedded-resource attachments"`（桌面与 iPhone SE 共 2 项通过）验证跨请求加载图片缩略图正常。
- 局限：
  - 本机免密与 Relay 浏览器顶层导航的非执行行为通过 HTTP 响应头契约（`application/octet-stream`、`attachment`、`nosniff` 与 `sandbox` CSP）回归验证，未连接真实公网 Relay 节点演练；
  - 仅在 Linux 无头 Chromium 中执行浏览器与端到端用例，未在 macOS / iOS / Android 真机浏览器上复测。
