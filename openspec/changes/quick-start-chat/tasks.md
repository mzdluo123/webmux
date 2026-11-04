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
  - 先校验仓库可用、启动配置勾选了混合模式、连接器已安装（沿用 `hybrid-sessions` 的检查与错误码），再做任何写操作。
  - 依次创建 worktree（路径为 `DefaultParent/<branch>`）、以 `startSide: acp` 创建会话、应用配置与权限模式、拼接初始化指令并调用 `hybrid.Service.Prompt`。
  - 失败时返回带 `step`、已创建 worktree 和会话 ID 的错误。
- [ ] 3.2 按 `clientRequestId` 去重：执行中等待同一结果，完成结果保留 10 分钟；首条提示复用该请求 ID。测试并发重复提交只产生一个 worktree 和一个会话
- [ ] 3.3 新增 `contract.QuickStartRequest`/`QuickStartResult`/错误体和 `POST /api/quick-starts`，并重新录制 wire 快照：
  - 处理器使用 `context.WithoutCancel` 加超时，`internal/api/errors.go` 映射各错误。
  - `go test -race ./internal/api` 覆盖未授权、未勾选混合模式拒绝、`connector_missing` 不建 worktree、客户端断开后仍完成。
- [ ] 3.4 兼容性测试：`go test -race ./internal/workspace` 用同一仓库和假 ACP agent，分别走快速开始和“`CreateWorktree` → `CreateSession(startSide: acp)` → `Prompt`”，比较两边的会话信息和混合会话记录，除 ID、时间、路径外完全一致；argv 播种后只覆盖请求中改过的配置项

## 4. 应用外壳与共享数据

- [ ] 4.1 把 `SessionList.svelte` 的 `/ws/sessions` 订阅、仓库清单和归档记录加载抽成 `hooks/session-list.svelte.ts`、`hooks/repositories.svelte.ts`、`hooks/archived.svelte.ts`，仓库选择改为共享状态（沿用 `homeRepository` 键）；首页改用这些 hook，行为不变，现有首页 Vitest 与 E2E 用例通过
- [ ] 4.2 新增 `AppShell`、`ModeSwitch`、`SidebarFooter`，`App.svelte` 为 `/`、`/new`、`/s/{id}` 包上外壳；设置页、代码弹出窗口和登录页不变。Vitest 覆盖：
  - 模式记忆与 Zod 回退；打开 `/` 且记住聊天时用 `replaceState` 进入 `/new`。
  - 会话页切换模式只换侧栏、不导航。
  - 收起状态的保存与恢复。
- [ ] 4.3 移除 `WorkspaceHeader`：品牌、连接状态与主机名、设置、主题移到侧栏底部，重连提示改为底部状态文字，“新建会话”改为工作台侧栏的主要操作；更新受影响的首页测试
- [ ] 4.4 `.terminal-page` 按 `--app-sidebar-width` 偏移，侧栏收起和展开时终端按现有 resize 流程更新列数；Playwright 覆盖桌面端收起/展开后终端列数变化
- [ ] 4.5 手机端左侧 `Drawer` 和各页面顶栏的打开按钮；终端顶栏窄屏的返回按钮换成打开侧栏按钮，宽屏返回按钮回到当前模式的主页。更新 `terminal-header` 相关测试，Playwright 在 390×844 和 320×568 下覆盖顶栏单行、抽屉开合与焦点
- [ ] 4.6 在 `docs/user-stories/access-and-sessions.md` 新增 US-C16（侧栏与模式切换），更新 `docs/user-stories.md` 索引与覆盖表

## 5. 快速开始页面

- [ ] 5.1 `route.ts` 新增 `/new`，外壳内懒加载 `pages/QuickStart.svelte`；更新 `test/route.test.ts`
- [ ] 5.2 `ChatSelect` 增加显示当前值文字的用法，供仓库、基准分支、agent 选择复用，并登记到 `docs/frontend-architecture.md` 扩展表；`web/stories/DesignSystem.svelte` 增加示例
- [ ] 5.3 实现页面，按原型 `Workspace/Quick start` 的布局：
  - 输入卡片、初始化指令提示与 Popover，仓库/基准分支/agent/配置/权限一行。
  - 选择记忆存 `localStorage` 并用 Zod 校验；草稿使用 `terminal-draft` 的 `quick-start` 键。
  - 分支名用 `suggestWorktreeBranch` 生成，冲突时重新生成并换新请求 ID。
- [ ] 5.4 发送状态、失败报告（按 `step` 显示原因、worktree 入口、会话入口）和成功跳转 `/s/{id}`。Vitest 组件测试覆盖：缺仓库或 agent 禁用发送、记住的选择失效回退、失败保留输入、初始化指令拼接提示
- [ ] 5.5 Playwright `test/e2e/quick-start.spec.ts`：使用隔离的临时仓库和假 ACP agent，从工作台切到聊天模式，发送后进入聊天视图并看到首条消息；接着切到终端再切回聊天，关闭后在首页看到归档记录并能恢复。桌面 1280×720 与手机 390×844 下检查
- [ ] 5.6 在 `docs/user-stories/hybrid-sessions.md` 新增 US-Q18（快速开始聊天），更新索引与覆盖表，记录实际做过的验证

## 6. 侧栏内容

- [ ] 6.1 `ListButton` 的 `navigation` 变体增加 `aria-current` 选中样式（与现有 `aria-pressed` 相同的线条标记），登记到 `docs/frontend-architecture.md` 扩展表
- [ ] 6.2 新增 `components/ChatHistory.svelte`，Vitest 覆盖：
  - 合并运行中会话与归档记录：过滤纯终端会话，同 ID 去重，按最近活动排序。
  - 四种状态映射及可访问名称，当前行的 `aria-current`。
  - 点击归档记录走现有恢复流程；无法恢复时显示原因。
- [ ] 6.3 新增 `components/WorkbenchTree.svelte` 取代并删除 `RepositorySidebar.svelte`，Vitest 覆盖：
  - 三层结构与首页分组顺序一致；不可用仓库的标记；已退出会话显示“已结束”。
  - 节点点击：仓库与“直接会话”切换视图，工作树导航到 `/#wt-<id>` 并滚动，会话进入 `/s/{id}`。
  - 收起节点仍显示等待数量；当前会话路径自动展开；展开状态保存。
  - `tree`/`treeitem` 键盘导航。
- [ ] 6.4 Playwright：聊天模式在 390×844 下“打开抽屉 → 切换会话”；工作台模式从会话页点工作树回到首页并定位
- [ ] 6.5 在 `docs/user-stories/hybrid-sessions.md` 新增 US-Q19（聊天历史），在 `docs/user-stories/access-and-sessions.md` 新增 US-C17（工作台导航树），更新索引与覆盖表

## 7. 集成验证

- [ ] 7.1 `make check` 与 `make test` 通过；PR 附桌面/手机、浅色/深色截图，覆盖两种模式、会话页和收起状态
