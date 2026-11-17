# Repository Guidelines

WebMux is a single-user Linux/macOS web terminal for persistent PTY sessions, including shells and CLI agents; supported agents also run as hybrid TUI/ACP-chat sessions. Browser disconnection does not end a session; server restart does. The Go binary embeds the browser app. Commands run as the service user. Deploy on loopback or a trusted VPN.

**This file has a hard 10 KiB budget**, enforced by `internal/archtest` (`TestAgentsGuideSize`). Keep only rules needed on almost every change here; put the rest in `docs/agent-rules/` and link it. Index: [docs/agent-rules.md](docs/agent-rules.md).

## Architecture

- `cmd/webmux` loads TOML, converts options via `cmd/internal/assemble`, and constructs core plus selected builtins in `feature_{assembly,start,bind,capabilities}.go`.
  Only `cmd/` constructs services. `api.New` receives typed transport contributions.
- Layers are enforced by `internal/archtest` (rules R0–R11 in `docs/architecture.md`; manifest in `internal/archtest/layers_test.go`, where every new `internal/` package must be classified).
  - Core controllers: `api`, `hub`, `wire`, `transport`; optional: `featureapi`, `featurehub`, `featurewire`. Services include `session`, `sessioncreate`, workspace, launch and chat.
    Shared includes `builtin`, `contract`, `streamproto`, config and runtime; clients stay outside services/controllers.
  - Shared packages and clients depend on shared packages. Only `cmd/` imports `internal/api`.
- `session` owns PTYs, bounded scrollback, core metadata, input/control and cleanup; `sessioncreate` owns argv/cwd/env with typed optional ports.
  `api` owns auth, Origin and static serving; `hub` owns raw transport. `featurewire.Projection` composes workspace/chat/forward overlays for all transports; core `wire` only converts core types.
- `internal/hybrid` runs hybrid sessions (one actor each) and normalizes ACP updates into chat events; `internal/acp` is a stdlib-only ACP v1 client; `internal/agentprofile` defines neutral profiles and `internal/agent/<name>` owns concrete adapters ([architecture](docs/architecture.md#混合会话)).
- REST handles configuration, session operations, and filesystem requests. `/ws/sessions` delivers a snapshot followed by list events; `/ws/s/{id}` carries terminal frames; `/ws/s/{id}/chat` carries chat events. Raw PTY bytes reach xterm without server-side terminal emulation; OSC/title and preview parsing are side channels.
- Replay capture and live subscription registration share a lock: preserve gap-free reattachment. Bounded queues disconnect slow clients and keep PTY readers moving. Each connection has one data writer; persistent sessions survive connection cancellation.
- JSON DTOs live in `internal/contract`; core `wire` and optional `featurewire` convert service types. Transport projections encode client-visible JSON.
  Update schemas, DTOs, projections, transport and consumers together; re-record `internal/api/testdata/wire` with `go test ./internal/api -run TestWireSnapshots -update`.

## Commands

Run from the repository root; the full list is in [conventions](docs/agent-rules/conventions.md#development-commands).

- `make web`: install frontend dependencies, run Svelte/TypeScript checks, build `web/dist`.
- `make build`: build the frontend, then the embedded Go binary `bin/webmux`. Rebuild the frontend first (`web/embed.go` embeds `web/dist`).
- `make check`: build the frontend, check test types and formatting, run the doc, persistence and architecture guards, Oxlint, `go vet` and `golangci-lint` (v2.15.0).
- `make format`: apply gofmt and Oxfmt.
- `pnpm --dir web check:arch`: frontend architecture guard; run it before finishing frontend work.
- `pnpm --dir web dev`: Vite+ at `http://127.0.0.1:5173`, proxying `/api` and `/ws` to backend port 7681. Run the backend separately and add `allowed_origins = ["http://127.0.0.1:5173"]` to its TOML or browser writes are rejected.
- `-config`, `-listen`, `-token` override startup settings.

## Conventions

- Go: follow [docs/go-style.md](docs/go-style.md); add no lint warnings (`make go-lint`). Return contextual errors. Services report sentinel errors; core `api` and optional `featureapi` controllers map them to HTTP status.
- Concurrency: protect state with existing locks; release locks before blocking network or PTY operations. Shared output chunks stay immutable. Exit publication follows PTY drain and process cleanup. Preserve cancellation/deadlines for PTY writes and join spawned connection goroutines.
- Execute argv directly. Creation selects exactly one source: saved `configId` (requires `launch`) or full `cmd`/`env`.
  Generated config selects core only (`features.enabled=[]`); omission retains the fixed legacy 14 builtins. Selection changes need restart. Only enabled `launch` seeds presets; `.bashrc` needs explicit `/usr/bin/env bash -ic`.
- Security: preserve authentication and Origin checks on API/WebSocket paths. Logs contain operational metadata only; configuration responses expose public fields only.
- Frontend: Svelte 5 runes per [docs/frontend-style.md](docs/frontend-style.md) (`$state` only for rendered state, `$derived` for computed values, `$effect` only for side effects with cleanup, typed `$props()`, `onclick` not `on:`). TypeScript is strict. Oxfmt: 80 columns, two spaces, semicolons, trailing commas.
- Validate HTTP responses, WebSocket JSON, and local settings with Zod before updating state; derive types with `z.infer`. Use the request and reconnect conventions in `api.ts` and `ws.ts`. PTY data stays binary. Failures use `APIError`; HTTP 401 triggers `webmux:auth`.
- UI: look first in `web/src/lib/components/ui/`, then `web/src/components/`, then the shadcn-svelte registry. Extend design-system components through variants or sizes. Resolve `check:arch` violations in the implementation ([docs/frontend-architecture.md](docs/frontend-architecture.md)).
- Any visible UI change: follow the `webmux-ui` skill (`.agents/skills/webmux-ui/SKILL.md`).
- Icons: Iconify with the `tabler` collection, bundled locally. Icon-only controls need an accessible name and a visible tooltip; decorative icons use `aria-hidden="true"` ([docs/design-system.md](docs/design-system.md#icons)).
- Names: PascalCase Svelte components, camelCase TypeScript functions, `*Schema` Zod contracts, Playwright `*.spec.ts`; files per [naming](docs/agent-rules/naming.md) (kebab-case TS, snake_case Go).
- Use pnpm with the frozen lockfile. Go 1.27+, Node 26.13.0 (`.nvmrc`), GNU Make. Release builds use `CGO_ENABLED=0`.

## Testing

- `make test` is the full gate (Go race plus all frontend suites); `make test-fast` skips Storybook, E2E and performance.
- Agents MUST run independent local checks concurrently; prefer these parallel Make targets. Preserve build dependencies and isolated performance runs; see [testing](docs/agent-rules/testing.md#agent-local-validation).
- Scope examples (run independent checks concurrently): Go `go test -race ./internal/session`; Vitest `pnpm --dir web exec vp test run --config vitest.config.ts test/input.test.ts`; E2E `pnpm --dir web exec playwright test test/e2e/terminal.spec.ts`.
- Preserve coverage for authentication/Origin rejection, argv boundaries, replay continuity, exit ordering, cancellation, terminal input/rendering, and touch interactions.
- Run tests against an isolated service. macOS temp paths sit behind `/var -> /private/var`; compare canonical paths.
- Test runtime, CI layout and diagnostics: [testing](docs/agent-rules/testing.md).

## Collaboration and Pull Requests

- Deliver changes via a Pull Request from a dedicated branch off `origin/main` (use an isolated `git worktree` when the current checkout is on `main` or shared). Direct commits or pushes to `main` require an explicit user request. Follow Conventional Commits (`feat(web): ...`, `fix: ...`).
- Follow every PR until all CI checks pass; fix CI failures on the branch until all checks are green.
- Once checks pass, leave the PR open for maintainer review. Merge when the user explicitly requests it.
- UI PRs require screenshots at desktop, tablet and phone sizes, before/after pairs for existing UI, and GIFs for interaction/motion changes. Follow [testing](docs/agent-rules/testing.md#ui-pull-request-evidence).

## Writing

- 所有语言的文本与指令输出统一遵循本节：对话、计划、提示词、代码注释、UI/CLI 文案、文档、提交与 PR 说明。
- 直接写事实、动作、条件与结果；避免说教式对立与臆想禁令（如“不是 X，而是 Y”“不要 X”“不得 X”）。用户要求的规则、安全与技术约束写清依据。
- 替代做法记录已探索失败的原因；验证记录已执行步骤与结果。

## Documentation and User Stories

- `docs/` and `openspec/` obey [docs/doc-governance.md](docs/doc-governance.md) (20–400 lines, ≤300 cols/line, ≤48 KiB, ≤20 child dirs) and the `webmux-docs` skill (`.agents/skills/webmux-docs/SKILL.md`). Write new OpenSpec artifacts in Simplified Chinese, keeping English structural headings and SHALL/MUST.
- Every implemented capability MUST have a story in `docs/user-stories/` (index: `docs/user-stories.md`). In the same change, update the affected stories when behavior changes: UI, APIs, CLI, configuration, security, lifecycle, deployment, contributor tooling. Pure refactors, formatting and dependency updates are exempt.
- Keep story IDs stable, mark removed stories retired, and keep both indexes and counts consistent. Verification records state commands run and observed results. Name the affected story IDs (or say there is no impact) in the delivery or PR summary. Full procedure: [user-stories-maintenance](docs/agent-rules/user-stories-maintenance.md).
- Every doc must be reachable by relative links from `README.md` or `AGENTS.md`, and relative links/anchors must resolve (`make docs-check`). Register new docs in [docs/README.md](docs/README.md) (specs in [docs/specs.md](docs/specs.md)); keep `README.md` a short landing page (budget in [doc-governance](docs/doc-governance.md#readme-预算)).
- Repository map (directories, important files): [repo-map](docs/agent-rules/repo-map.md).
