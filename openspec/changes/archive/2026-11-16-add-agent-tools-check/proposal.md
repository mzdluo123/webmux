# Proposal

## Why

WebMux 作为承载多种 CLI Agent 与混合（TUI ⇄ ACP 聊天）会话的持久工作台，用户在配置或排障时缺少直观手段查看当前服务运行环境下究竟安装了哪些 Agent 工具、各自解析到的可执行文件路径与版本号是多少、混合模式依赖的第三方 ACP 适配器是否已就绪，以及上游是否有可用更新。尤其当服务由 `systemd` 或 `launchd` 启动时，用户的登录交互式 Shell（`~/.bashrc`、`~/.zshrc`）与进程默认 `PATH` 存在差异，仅靠手工进入终端逐一排查效率低且容易遗漏。

## What Changes

- 新增内置 Agent 工具与 ACP 适配器本地环境探测服务（随 `launch` 内置功能装配）：按服务配置的默认启动 Shell（`bash` / `zsh` 交互式启动文件包装或直接 `PATH`）单次批量定位内置混合 Agent、独立 ACP 适配器与纯终端预置 Agent 的可执行文件路径，并以受界超时与独立进程组并发采集本地版本号。
- 新增按需上游版本检查能力：为具备公开包仓库（npm Registry、PyPI）的工具及适配器声明分发元数据，仅在客户端显式请求检查更新时通过只读 HTTPS 客户端查询最新稳定版本，结合内存 TTL 缓存与并发去重计算更新状态（`up_to_date`、`update_available`、`source_unavailable`、`check_failed` 等），同时保留与 WebMux 实测基线版本（`verifiedVersion`）的兼容性对比，绝不在启动或后台静默联网或自动修改用户环境。
- 新增受鉴权 REST 接口 `GET /api/agent-tools`（纯本机扫描并附带未过期的内存上游缓存）与 `POST /api/agent-tools/check`（触发已安装工具的上游版本检查），配套线上契约 DTO、投影转换与永久线上 JSON 快照。
- 在 Web 设置页紧邻「启动配置」下方新增「Agent 工具」（`#agent-tools`）常驻卡片及左侧锚点导航：默认展开已安装工具（展示品牌图标、本地版本、可执行文件路径、更新状态徽标、ACP 适配器子状态及一键复制升级/安装命令），底部以可折叠面板收纳未安装工具。
- 新增 CLI 子命令 `webmux agents`（支持 `--check`、`--refresh`、`--all`、`--json`），复用同一服务端接口输出表格或版本化 JSON。

## Capabilities

### New Capabilities
- `agent-tools-check`: 检测服务运行环境下本地已安装的 CLI Agent 工具与 ACP 适配器路径及版本号，按需查询公开上游版本源判断可用更新，并在 Web 设置页与 CLI 中呈现状态及升级/安装指引。

### Modified Capabilities

## Impact

- 后端包：扩展 `internal/agentprofile` 及各 Agent 适配器子包的分发元数据；新增共享目录包 `internal/agentcatalog` 与服务包 `internal/agentcheck`（登记于 `internal/archtest` 分层清单及 `outbound` 出站白名单）；在 `cmd/webmux`、`internal/contract`、`internal/featurewire`、`internal/featureapi` 接入装配与路由。
- 客户端与前端：在 `internal/clicmd` 新增 `agents` 子命令及能力门控；在 `web/src/contracts/schemas.ts`、`web/src/features/settings.ts`、`web/src/pages/Settings.svelte` 及新增 `web/src/components/settings/AgentToolsSettings.svelte` 实现前端契约与界面，补充中英文国际化词条与 Storybook 场景。
- 测试与文档：新增 Go 单元/集成测试、永久线上快照、前端组件测试与 Playwright E2E 测试，同步登记 `docs/specs.md` 与 `docs/user-stories/`。
