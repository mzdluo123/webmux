# Tasks

## 1. 仓库初始化指令

- [ ] 1.1 在 `internal/workspace/registry.go` 保存初始化指令，并用 `go test -race ./internal/workspace` 覆盖设置、清空、超长、重复登记、旧清单兼容：
  - `registration` 增加 `InitInstruction`（`omitempty`）；`Register` 接受可选指令，重复登记不覆盖。
  - 新增 `SetInitInstruction(id, text)`：超过 4000 字符返回 `ErrInvalid`，保存失败时内存清单不变。
- [ ] 1.2 暴露初始化指令接口，`go test -race ./internal/api` 覆盖未授权拒绝，并用 `-update` 重新录制 `testdata/wire`：
  - `internal/contract` 的 `Repository`、`RegisterRepositoryRequest` 增加 `initInstruction`，新增 `UpdateRepositoryRequest`；`internal/wire` 转换。
  - `internal/api/workspace.go` 新增 `PATCH /api/repositories/{id}`，沿用认证和 Origin 校验。
- [ ] 1.3 `web/src/schemas.ts` 同步字段；`AddRepositoryDialog.svelte` 增加可选的多行“初始化指令”字段，重复登记时提示到菜单修改；仓库菜单新增“初始化指令”弹窗（修改或清空）。补充对应 Vitest 组件测试，`pnpm --dir web check` 通过
- [ ] 1.4 在 `docs/user-stories/code-and-worktrees.md` 新增 US-J12（仓库初始化指令），更新 `docs/user-stories.md` 索引与 `coverage.md`，`make docs-check` 通过

## 2. agent 配置项清单

- [ ] 2.1 `internal/hybrid` 在 ACP 会话报告配置项时按 `configId` 记录配置项清单和当前值，保存到 `agent-options.json`；在 `internal/statefile/catalog.go` 登记，删除启动配置时清除。`go test -race ./internal/hybrid ./internal/statefile` 覆盖记录、更新、损坏文件报告和删除，持久化守卫通过
- [ ] 2.2 启动配置列表响应增加 `chatOptions`（结构同 `ChatConfig`），更新 contract/wire/schemas 并重新录制 wire 快照；`go test -race ./internal/api` 通过

## 3. 快速开始编排

- [ ] 3.1 新增 `internal/workspace/quickstart.go` 编排，用临时 Git 仓库和假 ACP agent 的 `go test -race ./internal/workspace` 覆盖成功、各步骤失败、不回滚 worktree、失效配置值产生 warning：
  - 先校验仓库可用、启动配置支持聊天端，再做任何写操作。
  - 依次创建 worktree（路径为 `DefaultParent/<branch>`）、以 `startSide: acp` 创建会话、应用配置与权限模式、拼接初始化指令并调用 `hybrid.Service.Prompt`。
  - 失败时返回带 `step`、已创建 worktree 和会话 ID 的错误。
- [ ] 3.2 按 `clientRequestId` 去重：执行中等待同一结果，完成结果保留 10 分钟；首条提示复用该请求 ID。测试并发重复提交只产生一个 worktree 和一个会话
- [ ] 3.3 `contract.QuickStartRequest`/`QuickStartResult`/错误体，`POST /api/quick-starts` 处理器使用 `context.WithoutCancel` 加超时，`internal/api/errors.go` 映射各错误。`go test -race ./internal/api` 覆盖未授权、非聊天启动配置拒绝、客户端断开后仍完成，并重新录制 wire 快照

## 4. 快速开始页面

- [ ] 4.1 `route.ts` 新增 `/new`，`App.svelte` 懒加载 `pages/QuickStart.svelte`；首页 `WorkspaceToolbar` 和仓库侧栏增加“新聊天”入口，在当前标签页导航。更新 `test/route.test.ts`
- [ ] 4.2 `ChatSelect` 增加显示当前值文字的用法，供仓库、基准分支、agent 选择复用，并登记到 `docs/frontend-architecture.md` 扩展表；`web/stories/DesignSystem.svelte` 增加示例
- [ ] 4.3 实现页面：输入卡片、初始化指令提示与 Popover、仓库/基准分支/agent/配置/权限一行；按原型 `Workspace/Quick start` 的布局。选择记忆存 `localStorage` 并用 Zod 校验；草稿使用 `terminal-draft` 的 `quick-start` 键。分支名用 `suggestWorktreeBranch` 生成，冲突时重新生成并换新请求 ID
- [ ] 4.4 发送状态、失败报告（按 `step` 显示原因、worktree 入口、会话入口）和成功跳转 `/s/{id}`。Vitest 组件测试覆盖：缺仓库或 agent 禁用发送、记住的选择失效回退、失败保留输入、初始化指令拼接提示
- [ ] 4.5 删除原型专用的 `web/stories/quick-start/` 数据桩，改为渲染真实页面组件的 story；`pnpm --dir web check`、`lint`、`check:arch`、`exec oxfmt --check src` 通过
- [ ] 4.6 Playwright `test/e2e/quick-start.spec.ts`：使用隔离的临时仓库和假 ACP agent，从首页进入快速开始，发送后进入聊天视图并看到首条消息；桌面 1280×720 与手机 390×844 下检查
- [ ] 4.7 在 `docs/user-stories/hybrid-sessions.md` 新增 US-Q18（快速开始聊天），更新索引与覆盖表，记录实际做过的验证

## 5. 集成验证

- [ ] 5.1 `make check` 与 `make test` 通过；PR 附桌面/手机、浅色/深色截图
