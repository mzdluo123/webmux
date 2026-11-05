# Tasks

## 1. 发现与公共约定（P0）

- [ ] 1.1 服务端为每个会话设置 `WEBMUX_SOCKET`，加入 `contract.ServerOwnedEnv` 与会话 `env` 校验，启动配置保存时同样拒绝。
  用 `go test -race ./internal/session ./internal/launch ./internal/contract` 覆盖注入、400 拒绝、`run` 继承时跳过。
- [ ] 1.2 `cmd/webmux/sessions.go` 与 Windows 客户端的 `Env.Connect` 按“`--config` → `WEBMUX_SOCKET` → 默认配置”定位，
  `WEBMUX_SOCKET` 失效时返回 `ErrNotRunning` 而不回退。测试覆盖三种来源与不回退。
- [ ] 1.3 `sessioncmd` 增加 `self` 解析、`contract.ExitTimeout = 4`，以及共用的 `--json`/NDJSON 输出辅助函数。
  `webmux help` 改为分组列出命令，`subcommands` 同步更新。
- [ ] 1.4 新命令统一使用 `script` 连接标签，会话内调用时附加来源会话 ID。

## 2. 会话控制命令（P0）

- [ ] 2.1 `webmux api`：路径白名单、方法推断、`-d` 的字面值/`@文件`/`-`、JSON 校验、非 2xx 退出码。
  用 `httptest` 起的本机 API 测试会话内 403 透传与 `/ws/` 拒绝。
- [ ] 2.2 `kill`、`rm`、`rename`。测试覆盖 404、混合会话切换中的 409。
- [ ] 2.3 `send`：提交帧与回执、命名按键表、`-` 读标准输入、聊天端拒绝；`--wait` 先订阅再提交。
  测试覆盖 written/partial/failed、控制标签、未知按键退出码 2。
- [ ] 2.4 `wait`：快照判断、`--changed`、`--stable` 重新计时、`--any`、超时 4、移除 1、断线重连不误报。测试使用注入时钟，不 sleep。
- [ ] 2.5 `logs`：原始输出、`--strip`（CSI/OSC/DCS/ESC、CRLF、单独 CR）、`--tail`、`--follow` 观察者通路。
  用表驱动测试覆盖 `--strip`，`--follow` 测试确认不夺取控制。
- [ ] 2.6 `events`：NDJSON、`--session` 过滤、重连后输出新快照。
- [ ] 2.7 `dir`：调用确保接口并输出路径，失败时退出码 1。

## 2A. 启动命令（P0）

- [ ] 2A.1 `run --worktree`、自动关联（`--no-workspace`）、`--new-worktree`（`--base`、`--existing-branch`、`--worktree-path`、`--repo`），
  会话创建失败时保留 worktree。测试覆盖三者互斥与 `--cwd` 包含关系。
- [ ] 2A.2 `run --start-side`、`--prompt`（含 `-`）、`--init`、`--label`、`--json`、`--wait`、`--dry-run`（环境值脱敏）。
- [ ] 2A.3 `run` 在没有终端时自动分离，新增 `--attach` 强制附着。更新现有 `sessioncmd` 测试中依赖退出码 2 的用例。
- [ ] 2A.4 `chat new`：仓库与 agent 推断、`--set`、`--permission`、`--init/--no-init` 默认值、`--terminal` 互斥、附件上传、按 `step` 报告失败、`--wait`。
- [ ] 2A.5 `launch-config options`。

## 2B. 会话元数据（P0）

- [ ] 2B.1 `internal/session`：由对端 PID 找到所在会话 ID（Linux 读环境标记，macOS 查进程树）；`parentId` 与标签只保存在内存中。
  `go test -race ./internal/session` 覆盖进程树查找与标签校验。
- [ ] 2B.2 本机 API 的创建入口（`POST /api/sessions`、`POST /api/quick-starts`）记录 `parentId`，请求体无法指定；TCP 入口不记录。
- [ ] 2B.3 `GET /api/sessions/{id}/details` 与 `PATCH /api/sessions/{id}/labels`；契约 `SessionDetails`、`LabelsUpdate`，
  `SessionInfo.parentId/labels`、`CreateSessionRequest.labels`；同步 wire 与 schemas，重新录制 wire 快照。
- [ ] 2B.4 `webmux get`（会话内默认 `self`、`--field`）、`webmux label`、`ls --label/--parent/--status`。

## 3. 提醒接口与 `notify`（P1）

- [ ] 3.1 `internal/session` 支持外部提醒：来源 `cli`，覆盖当前锁存提醒，`message` 只保存在内存。
  `AttentionSource` 新增 `cli`，同步 contract、wire、`web/src/schemas.ts` 与提醒显示文案。
- [ ] 3.2 `POST /api/sessions/{id}/attention`（`AttentionRequest`），在 `internal/api/errors.go` 中映射 400/404。
  推送正文使用 `message`，重新录制 wire 快照。
- [ ] 3.3 `webmux notify`：默认 `self`，本地校验 200 字符与 `kind`。

## 4. 聊天命令（P1）

- [ ] 4.1 `chat send`：上传附件、`clientRequestId`、排队结果；`--wait` 先连聊天 WebSocket，再按 `promptId` 跟踪回合，
  按 `stopReason` 决定退出码，遇到请求时退出（或 `--auto-wait-requests`），超时 4。用 `internal/hybrid` 的假 agent 做集成测试。
- [ ] 4.2 `chat watch`（NDJSON 与文本两种形式）与 `chat status`。
- [ ] 4.3 `chat cancel`、`chat answer`、`chat switch`（需要选择、`--force`、会话歧义与 `--agent-session`）。

## 5. 工作区命令（P1）

- [ ] 5.1 `worktree ls|create|rm`：仓库参数解析（ID、路径、`.`），默认路径与快速开始一致，`rm` 先预览再确认，非交互时需要 `--yes`。
- [ ] 5.2 `launch-config ls|show`：文本形式不输出环境变量值。
- [ ] 5.3 `forward ls|add|rm`：输出转发地址，`--session self`，`--no-compat`。

## 6. shell 工具手册与 `{{webmux}}`

- [ ] 6.1 新建共享包 `internal/agentguide`（仅标准库）：用 `go:embed` 嵌入索引与各主题 Markdown，渲染 `/llms.txt`（可选版本行）、
  主题、全文与简述。在 `internal/archtest/layers_test.go` 中登记为 Shared。临时文件夹说明从 `web/src/quickCommands.ts` 的预置正文移入 `artifacts` 主题。
- [ ] 6.2 一致性测试：会话类子命令都在某个主题中出现；索引链接都能解析；每个主题都在索引中；TCP 内容不含版本、路径或主机名。
- [ ] 6.3 `internal/api` 在 SPA 回退之前注册 `/llms.txt`、`/llms-full.txt`、`/llms/{topic}.md`：TCP 不要求 Token 但经过 Host 检查，
  本机套接字附加版本行；未知主题 404。测试覆盖两个监听器与未认证访问。
- [ ] 6.4 `artifacts.Service.Brief(id)` 与 `POST /api/sessions/{id}/agent-guide`（`AgentGuidePrompt{prompt, guideUrl, dir}`），
  同步 contract、wire、schemas，重新录制 wire 快照。确认临时根中不会出现手册文件。
- [ ] 6.5 `internal/workspace`：移除对 `{{webmux}}` 的保留拒绝；`composePrompt` 先展开 `{{webmux}}` 再展开 `{{dir}}` 与 `{{user}}`；预检在临时文件夹不可用时报告不可用。
  更新 `instruction_test.go` 与 `quickstart_test.go`。
- [ ] 6.6 `webmux guide [TOPIC] [--full]`，服务未运行时退回内嵌副本，未知主题以 2 结束。
- [ ] 6.7 前端：
  - `quickCommands.ts` 把 `{{webmux}}` 当作内置变量；
  - `SessionTerminal` 与聊天草稿在插入前调用手册接口，失败时提示“WebMux 工具说明不可用”；
  - 预置列表改为恢复会话、新会话、WebMux 工具；
  - `InitInstructionField` 与 `QuickStart` 去掉保留提示，在可引用列表中列出内置变量。
  - 更新 `web/test/quickCommands.test.ts` 与相关组件测试，运行 `pnpm --dir web check:arch`。
  - 按 `webmux-ui` skill 截图：设置页可引用列表、快捷指令菜单，包括亮色/暗色与桌面/手机。

## 7. 文档与验证

- [ ] 7.1 新增用户故事分组（shell 与 agent 控制），覆盖 P0、P1、启动增强、会话元数据与 `{{webmux}}`；更新 US-Q13、`run` 相关故事，以及快捷指令、临时文件夹、初始化指令相关故事、索引和覆盖表。
- [ ] 7.2 `docs/protocol/http-workspace.md` 删除 `{{webmux}}` 保留说明；协议文档登记三个新接口；使用文档新增“用脚本和 agent 控制 WebMux”页面并在 `docs/README.md` 注册。
- [ ] 7.3 运行 `make check` 与 `make test-fast`。针对 `chat new --wait`、`send --wait`、`ls --parent`、`notify` 补一个隔离后端的 E2E，不连接开发者已有的服务。
