## 1. 创建与输入

- [ ] 1.1 `run`：`--new-worktree` 改为开关并配合 `--branch`（省略时由服务端生成），`--start-side` 改为 `--view`，新增 `--set`、`--permission`、`--attach-file`、`--reply`；创建只发一次 `POST /api/sessions` 并按步骤报告失败
- [ ] 1.2 `run` 在没有 `--` 时拒绝位置参数并提示 `--branch`；`--reply` 要求 `--view chat` 与 `--prompt`；删除 `chat new`
- [ ] 1.3 `send` 改为调用 `/input`，增加 `--attach-file`（经会话上传接口）、`--mode`、`--reply`、`--auto-wait-requests`，`--reply` 复用原 `chat send --wait` 的回合跟踪；删除 `chat send`

## 2. 会话操作

- [ ] 2.1 新增 `interrupt`、`view`、`resume`，沿用原 `chat switch` 的等待选择与候选重试、原 `chat resume` 的附着与 `--without-env` 规则；删除 `chat cancel`、`chat switch`、`chat resume`
- [ ] 2.2 `ls` 增加 `--state`、`--archived`、`--repo`，表格按 `view`/`transition` 显示视图；`rm` 增加 `--forget`、`--yes`，对已归档会话不带 `--forget` 时以 1 提示；删除 `chat ls`、`chat forget`
- [ ] 2.3 `chat status|watch|answer|set|permission|queue` 改读新字段，统一 `no_chat_view`/`wrong_view` 的错误与 `webmux view SESSION chat` 提示
- [ ] 2.4 `launch-config add|edit` 的 `--hybrid`/`--no-hybrid` 改为 `--chat`/`--no-chat`，`launch-config options` 改用 `POST /api/launch-configs/{id}/options`
- [ ] 2.5 `attach` 的移动提示与 4409 处理改用 `terminal`/`chat` 取值

## 3. 生成物与测试

- [ ] 3.1 补全的已归档会话来源与 `worktree` 命令的已归档计数改读 `GET /api/sessions?state=archived`
- [ ] 3.2 更新命令表一致性测试与 `internal/sessioncmd` 单元测试
- [ ] 3.3 更新 `cmd/webmux` 端到端测试（真实服务进程与假 ACP agent），覆盖 `run --new-worktree --view chat --prompt --reply`、`send --reply`、`send` 的 `wrong_view`、`interrupt`、`view`、`resume`、`ls --archived`、`rm --forget`
- [ ] 3.4 重新生成 `internal/agentguide/llms/commands.md`，改写 `launch`、`chat`、`sessions`、`recipes` 主题与 `agentguide.go` 的主题摘要

## 4. 文档与用户故事

- [ ] 4.1 更新 `docs/usage/shell-control.md` 的命令示例与说明
- [ ] 4.2 更新 `docs/user-stories/shell-control.md`（US-S07、US-S08、US-S09、US-S11、US-S12、US-S14、US-S15、US-S16、US-S17、US-S18）与 `docs/user-stories/hybrid-sessions.md` 中引用旧命令的条目，同步覆盖索引
- [ ] 4.3 运行 `make docs-check`

## 5. 验证

- [ ] 5.1 与 `unify-session-surface` 一起运行 `make check` 与 `make test`
- [ ] 5.2 用真实服务进程冒烟：普通 shell 的 `send --enter --wait`、`interrupt`；聊天视图会话的 `run --new-worktree --view chat --prompt --reply`、`send --reply`、`view`、`kill`、`ls --archived`、`resume`、`rm --forget --yes`；旧命令 `chat send` 以 2 结束
