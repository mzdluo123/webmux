# Proposal

## Why

从首页开始一次 agent 编码任务现在要走四步：

1. 打开新建会话弹窗；
2. 选仓库并填写 worktree 分支；
3. 选启动配置并指定聊天端；
4. 等聊天就绪后再输入第一条消息。

Codex、Claude Code 网页版的做法是在一个输入框里写任务、选仓库和 agent，按发送即可开工。WebMux 已经具备仓库登记、worktree 创建、混合会话和聊天提示这些能力，只差一个把它们串成一步的入口。

另外，每个仓库常有一句固定的开场要求（例如“先读 AGENTS.md，改完跑 make check”），现在只能每次手动粘贴。

## What Changes

- 新增“快速开始聊天”页面（路由 `/new`，首页工具栏和仓库侧栏提供入口）：底部是聊天式输入框，下方一行放仓库和基准分支，以及 agent、该 agent 的会话配置（模型、思考强度、模式等）和 WebMux 权限模式；窄屏下 agent 与配置收进输入卡片。
- 按发送后，服务端一次完成：在所选仓库新建 worktree（自动生成分支名和路径）、以聊天端启动所选 agent、应用所选配置、把首条消息作为第一轮提示发送。成功后在当前页进入该会话的聊天视图。失败时停在原页面，保留输入文字，并说明失败的步骤和已创建的资源。
- 新增服务端接口 `POST /api/quick-starts`，请求带 `clientRequestId`，同一请求重复提交不会重复创建 worktree 或会话。
- 服务端记住每个支持聊天端的启动配置最近一次得到的 agent 配置项清单（名称、类别、可选值），供快速开始在 agent 启动前展示配置。清单还没有时，配置区说明“首次启动后可选”，并使用 agent 默认值。
- 仓库可设置“初始化指令”：添加仓库时可以填写，之后可在仓库菜单里修改或清空。快速开始时，初始化指令与用户输入合并为第一条消息，初始化指令在前，用空行分隔。输入框上方提示本次会附带初始化指令，点击可查看全文。
- 新增聊天侧栏：快速开始页和混合会话页左侧常驻，按最近活动平铺列出运行中和已归档的聊天会话，每行显示标题、agent 图标和运行状态；顶部是“新聊天”。桌面端可收起，手机端从左上角按钮打开抽屉。
- 新增 Storybook 原型 `Workspace/Quick start`，覆盖空状态、已选配置、无配置清单、发送中和失败几种状态，含聊天侧栏与手机抽屉；另有带初始化指令字段的添加仓库弹窗。

## Capabilities

### New Capabilities

- `quick-start-chat`：从一条任务描述一步创建 worktree、启动聊天 agent 并发送首条消息；包括页面交互、选择记忆、配置预选、幂等提交、失败报告和配置清单记忆。
- `chat-sidebar`：快速开始页与混合会话页的聊天侧栏，包括出现位置、列表内容与排序、行状态、归档恢复和手机抽屉。

### Modified Capabilities

- `repository-registry`：仓库登记新增可选的初始化指令，可在添加时设置、之后修改或清空，随仓库清单持久保存。

## Impact

- 后端：
  - `internal/workspace` 的登记记录与仓库请求新增初始化指令。
  - 新增快速开始编排（worktree → 混合会话 → 配置 → 首条提示），复用 `workspace.CreateWorktree`、`workspace.CreateSession`、`hybrid.Service.Prompt`。
  - `internal/hybrid` 记录每个启动配置的配置项清单；这份新的持久状态需要在 `internal/statefile` 目录登记。
- 接口：`internal/contract`、`internal/wire`、`internal/api` 新增 `QuickStartRequest`/`QuickStartResult`、`PATCH /api/repositories/{id}`，以及启动配置的配置项清单字段；`web/src/schemas.ts` 同步更新，并重新录制 `internal/api/testdata/wire`。
- 前端：
  - 新增 `pages/QuickStart.svelte` 与 `/new` 路由；输入框和配置控件复用 `ChatConfig`、`ChatSelect`、`ChatPermissionMode`。
  - `AddRepositoryDialog` 增加初始化指令字段，并新增修改指令的仓库菜单项。
  - 新增 `ChatSidebar` 组件，由快速开始页和 `pages/Terminal.svelte`（混合会话）共用；数据复用 `/ws/sessions` 会话列表与 `GET /api/hybrid-sessions` 归档记录，不新增接口。
- 文档：在 `docs/user-stories/` 新增快速开始、聊天侧栏和仓库初始化指令的故事，更新索引与覆盖表。
