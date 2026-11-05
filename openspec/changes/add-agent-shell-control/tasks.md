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
- [ ] 2.2 `get`、`kill`、`rm`、`rename`。测试覆盖 404、混合会话切换中的 409。
- [ ] 2.3 `send`：提交帧与回执、命名按键表、`-` 读标准输入、聊天端拒绝；`--wait` 先订阅再提交。
  测试覆盖 written/partial/failed、控制标签、未知按键退出码 2。
- [ ] 2.4 `wait`：快照判断、`--changed`、`--stable` 重新计时、`--any`、超时 4、移除 1、断线重连不误报。测试使用注入时钟，不 sleep。
- [ ] 2.5 `logs`：原始输出、`--strip`（CSI/OSC/DCS/ESC、CRLF、单独 CR）、`--tail`、`--follow` 观察者通路。
  用表驱动测试覆盖 `--strip`，`--follow` 测试确认不夺取控制。
- [ ] 2.6 `events`：NDJSON、`--session` 过滤、重连后输出新快照。
- [ ] 2.7 `dir`：调用确保接口并输出路径，失败时退出码 1。

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
- [ ] 5.2 `run --worktree`、`--start-side chat`、`--prompt`、`--init`，与 `--cwd` 的包含关系校验。
- [ ] 5.3 `launch-config ls|show`：文本形式不输出环境变量值。
- [ ] 5.4 `forward ls|add|rm`：输出转发地址，`--session self`，`--no-compat`。

## 6. shell 工具手册与 `{{webmux}}`

- [ ] 6.1 新建共享包 `internal/agentguide`（仅标准库）：嵌入 `guide.md.tmpl`，渲染手册（frontmatter、版本、各节）与简述。
  在 `internal/archtest/layers_test.go` 中登记为 Shared。临时文件夹说明从 `web/src/quickCommands.ts` 的预置正文移入手册。
- [ ] 6.2 一致性测试：CLI 帮助中的会话类子命令都出现在手册“命令参考”中；frontmatter 可以解析，`name` 为 `webmux`。
- [ ] 6.3 `internal/artifacts`：启动时写入 `guide/SKILL.md`（0700/0600），缺失时重写；`Guide(id)` 返回 `{prompt, guidePath, dir}`。
  代码栏的临时文件夹根只显示会话文件夹。用 `go test -race ./internal/artifacts` 覆盖权限、重写与清扫不删。
- [ ] 6.4 `GET /api/agent-guide` 与 `POST /api/sessions/{id}/agent-guide`（`AgentGuidePrompt`），同步 contract、wire、schemas，重新录制 wire 快照。
- [ ] 6.5 `internal/workspace`：移除对 `{{webmux}}` 的保留拒绝；`composePrompt` 先展开 `{{webmux}}` 再展开 `{{dir}}` 与 `{{user}}`；预检在临时文件夹不可用时报告不可用。
  更新 `instruction_test.go` 与 `quickstart_test.go`。
- [ ] 6.6 `webmux guide` 与 `guide --path`，服务未运行时退回内嵌副本。
- [ ] 6.7 前端：
  - `quickCommands.ts` 把 `{{webmux}}` 当作内置变量；
  - `SessionTerminal` 与聊天草稿在插入前调用手册接口，失败时提示“WebMux 工具说明不可用”；
  - 预置列表改为恢复会话、新会话、WebMux 工具；
  - `InitInstructionField` 与 `QuickStart` 去掉保留提示，在可引用列表中列出内置变量。
  - 更新 `web/test/quickCommands.test.ts` 与相关组件测试，运行 `pnpm --dir web check:arch`。
  - 按 `webmux-ui` skill 截图：设置页可引用列表、快捷指令菜单，包括亮色/暗色与桌面/手机。

## 7. 文档与验证

- [ ] 7.1 新增用户故事分组（shell 与 agent 控制），覆盖 P0、P1 与 `{{webmux}}`；更新 US-Q13，以及快捷指令、临时文件夹、初始化指令相关故事、索引和覆盖表。
- [ ] 7.2 `docs/protocol/http-workspace.md` 删除 `{{webmux}}` 保留说明；协议文档登记三个新接口；使用文档新增“用脚本和 agent 控制 WebMux”页面并在 `docs/README.md` 注册。
- [ ] 7.3 运行 `make check` 与 `make test-fast`。针对 `send --wait`、`chat send --wait`、`notify` 补一个隔离后端的 E2E，不连接开发者已有的服务。
