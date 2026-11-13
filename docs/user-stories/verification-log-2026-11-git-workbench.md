# Git 工作台功能核对记录（2026-11-12 起）

本页记录独立 Git 工作台（US-J19–US-J28）各阶段契约、路由、界面、持久操作账本与冲突解决能力的实现与验证证据，按提交顺序升序排列；故事索引与范围约定见 [WebMux User Stories](../user-stories.md) 与 [功能覆盖核对](coverage.md)。

## 2026-11-12：Git 工作台 02（V1a）仓库与工作树级 Git REST API 及契约快照（US-J19）

- 范围：交付零活跃会话下可用的仓库与工作树级 Git REST API、契约快照及前端请求封装：
  - 路由：`/api/repositories/{repoId}/git/*` 10 个只读端点与 `/api/repositories/{repoId}/worktrees/{worktreeId}/git/*` 16 个只读及暂存/提交端点；
  - 契约与错误映射：`internal/contract`、`internal/wire`、`internal/api` 错误细分（`readonly`、`stale`、`busy`、`context_mismatch`、`commit_blocked`、`commit_failed`）、26 份 wire 快照，以及前端 `git-api-schemas.ts` 与 `git-api.ts`。
- 依据：
  - `go test -race ./internal/api ./internal/wire ./internal/codeview ./internal/workspace ./internal/archtest ./internal/docguard` 通过：
    - 覆盖鉴权/Origin 拦截、零活跃会话读写、历史车道图分页、合并提交父切换、跨仓 404，以及跨工作树/过期上下文/只读/锁冲突 409；
    - 覆盖单 hunk 暂存与取消暂存、外部 CLI 暂存预览、客户端断连下已进入 `git commit` 的原子完成及日志脱敏；
  - `go test ./internal/api -run TestWireSnapshots` 与 `pnpm --dir web exec vitest run --config vitest.config.ts` 通过，校验全部后端响应快照、数组契约与前端 API 封装；
  - `make check`（含 `make web`、`pnpm --dir web check:arch`、`make docs-check`、`make persistence-check`、`go vet` 与 `golangci-lint`）通过。
- 限制：V1a 仅交付 REST 契约、后端路由与前端 typed client/schema，尚未挂载 Git 工作台浏览器页面，未执行工作台 UI 截图或真实移动端浏览器交互验收。

<!-- 后续 Git 工作台分层核对记录按阶段追加于下方 -->

---
