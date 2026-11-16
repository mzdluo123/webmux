# Tasks

## 1. Agent 分发元数据与统一工具目录

- [x] 1.1 在 `internal/agentprofile` 新增中性 `Distribution` 与 `UpstreamSource` 定义，并在各内置混合 Agent 子包及连接器中补齐安装命令、升级命令、文档链接与上游包源，运行 `go test -race ./internal/agentprofile/...` 验证通过
- [x] 1.2 新增共享包 `internal/agentcatalog` 汇总混合 Agent 注册表投影与纯终端预设条目，在 `internal/archtest/layers_test.go` 归类为 `shared`，运行 `go test ./internal/agentcatalog ./internal/archtest` 验证分层与 Agent 名称隔离规则通过

## 2. 本机环境扫描与按需上游更新服务

- [x] 2.1 新增 `internal/agentcheck` 服务包（在 `internal/archtest/layers_test.go` 归类为 `service` 并登记 `outbound`），实现单次批量交互式启动 Shell 路径定位、有界独立进程组 `--version` 并发采集、版本解析与内存 TTL 缓存，编写单元测试覆盖 `.bashrc` 路径解析、多候选可执行文件名、超时清理与 `refresh` 强制重扫并通过 `go test -race ./internal/agentcheck ./internal/archtest`
- [x] 2.2 在 `internal/agentcheck` 实现只读安全 HTTPS 上游查询（npm 主源及镜像回退、PyPI JSON）、重定向校验、单源错误隔离与 `verifiedVersion` 基线对比，编写本地测试服务器单元测试并通过 `go test -race ./internal/agentcheck`

## 3. REST 接口、线上契约与 CLI 子命令

- [x] 3.1 在 `internal/contract`、`internal/featurewire`、`internal/featureapi` 与 `cmd/webmux` 装配根接入 `GET /api/agent-tools` 与 `POST /api/agent-tools/check`（受 `launch` 内置功能门控），补充接口鉴权/Origin/禁用 404 测试并重录 `internal/api/testdata/wire` 快照，更新 HTTP 协议文档并运行 `go test -race ./internal/api ./internal/featureapi ./internal/wire ./cmd/webmux` 验证通过
- [x] 3.2 在 `internal/clicmd` 新增 `webmux agents` 子命令（支持 `--check`、`--refresh`、`--all`、`--json`）、帮助文本与能力检查，编写 CLI 测试并通过 `go test -race ./internal/clicmd`

## 4. Web 设置页 Agent 工具卡片与端到端验证

- [x] 4.1 在 `web/src/contracts/schemas.ts` 新增 `AgentToolsReportSchema` 等 Zod 契约并在 `web/test/wire-contract.test.ts` 对齐线上快照，补充中英文 Paraglide 词条，运行 Vitest 契约测试验证通过
- [x] 4.2 实现 `web/src/components/settings/AgentToolsSettings.svelte` 并在 `web/src/features/settings.ts`、`web/src/pages/Settings.svelte` 紧邻「启动配置」下方接入 `#agent-tools` 常驻卡片与锚点导航（默认展开已安装项、展示 ACP 适配器子行、折叠未安装项、一键复制命令），补充 Storybook 场景与组件测试并通过 `pnpm --dir web check`、`pnpm --dir web check:arch` 及浏览器组件测试
- [x] 4.3 新增 Playwright E2E 用例验证设置页 `#agent-tools` 本地扫描、未安装折叠展开、复制升级命令及桌面/手机视口无横向溢出，同步更新 `docs/user-stories/` 与验证日志，运行 `make check`、`make docs-check` 与相关 E2E 测试验证通过

## Workflow follow-up

- 完成全部实现与校验后同步主规格 `openspec/specs/agent-tools-check/spec.md`、登记 `docs/specs.md` 并归档变更，确保 `openspec/changes` 目录扇出与文档门禁全部通过。
