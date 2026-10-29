## 1. 定义模型与编译器（纯逻辑，先行）

- [ ] 1.1 `internal/workflow/def.go`：步骤模板、工作流、节点、`uses` / `include` / `with`、`on` / `onFailure` / `onExhausted`、结果约定、`expect`、工作区与参数类型；JSON 标签与上限常量
- [ ] 1.2 `internal/workflow/template.go`：`{{path}}` 与 `{{#path}}…{{/path}}` 解析，返回引用集合；argv 逐项渲染与 `omitEmpty`；64 KiB 提示词上限
- [ ] 1.3 `internal/workflow/compile.go`：展开 include（命名空间、深度 4、环检测）与 uses；节点唯一、入口唯一、目标存在、不可达检测、回边 `maxVisits`、64 节点上限
- [ ] 1.4 支配关系分析：判断 `steps.<id>` 是否在所有路径上先执行；可选段放宽；结果字段存在性检查；错误携带 JSON 路径
- [ ] 1.5 Go 测试：组合展开、环、无界回环、拼写错误字段、分支引用、argv 含空格与 shell 元字符、空值 `omitEmpty`、上限

## 2. 持久化

- [ ] 2.1 `workflows.json`：`statefile.Open`、独立锁、修订号、`unavailable` 策略；首次创建时写入示例模板
- [ ] 2.2 `workflow-runs.json`：定义快照、参数、节点状态、访问计数、结果字段、闸门决定、会话 ID；启动时活动运行改为 `interrupted`
- [ ] 2.3 运行私有目录 `<state_dir>/workflow-runs/<runId>/`（0700）的创建与随记录删除
- [ ] 2.4 `internal/archtest/persistence_inventory.json`、`docs/persistence.md` 登记；`make persistence-check` 通过

## 3. 运行 actor 与执行器

- [ ] 3.1 `internal/workflow/run.go`：actor、命令去重（客户端请求 ID）、状态机、访问计数、条件匹配、失败策略、取消顺序
- [ ] 3.2 `Executor` / `Attempt` 接口与假执行器；actor 的状态机测试全部基于假执行器
- [ ] 3.3 `chat` 执行器：`hybrid.Create(SideACP)` / 复用、`SetPermissionMode`、`Prompt` 记录 `TurnID`、订阅匹配 `turn.end`、`Cancel`、`ask` 模式下的等待态
- [ ] 3.4 `command` / `check` 执行器：`Manager.Create`、`Done()` 与退出码、超时关闭、`promptFile` 0600 写入与删除
- [ ] 3.5 结果文件：环境变量注入、尝试前删除、16 KiB 严格 JSON 读取、字段校验、`expect` 路径检查（拒绝 `..`、绝对路径与根外符号链接）
- [ ] 3.6 `gate`：等待态、决定与意见、注意力置位（question / error / done）
- [ ] 3.7 工作区：`cwd` / `worktree` 模式、同目录活动运行冲突、启动目录协调层登记
- [ ] 3.8 `session.Info` 增加可选 `Workflow{RunID, NodeID}` 覆盖信息
- [ ] 3.9 恢复：快照重新编译、跳过已成功节点、`chat` 节点优先恢复原 agent 会话
- [ ] 3.10 Go race 测试：用 `internal/acp/testdata/fakeagent` 驱动真实 `chat` 节点；真实 PTY 驱动 `command`；重启中断与恢复；取消顺序

## 4. 装配、分层与契约

- [ ] 4.1 `internal/archtest/layers_test.go` 把 `internal/workflow` 归类为 Service
- [ ] 4.2 `cmd/webmux` 构造 `workflow.Service`，经 `Options.Executors` 注册执行器，注入 `api.Deps`；关闭顺序在混合服务之前
- [ ] 4.3 `internal/contract` 定义、运行、节点、事件类型；`internal/wire` 转换；`internal/api` 路由与 `errors.go` 映射
- [ ] 4.4 `/ws/workflow-runs` 快照 + 事件，有界队列与慢客户端断开
- [ ] 4.5 线上快照新增 `workflows` 组并重新录制

## 5. 前端

- [ ] 5.1 `web/src/schemas.ts` 与 `api.ts`：定义、编译结果、运行与事件的 Zod 契约
- [ ] 5.2 首页“工作流”分组与运行卡片，步骤会话折叠
- [ ] 5.3 运行页：纵向步骤条、节点详情、闸门作答、重试 / 跳过 / 取消 / 恢复
- [ ] 5.4 设置页定义编辑器：JSON 编辑、编译预览面板、只读流程预览、导入导出、`auto` 权限提示
- [ ] 5.5 新建运行表单由 `params` 生成，含工作区选择
- [ ] 5.6 浏览器测试与桌面 / Pixel 7 E2E（使用假 agent），`pnpm --dir web check:arch` 通过

## 6. CLI 与文档

- [ ] 6.1 `internal/sessioncmd`：`webmux workflow ls|run|status|cancel` 与 `run --attach` 跟随
- [ ] 6.2 `docs/protocol/` 新增工作流协议页；`docs/user-stories/` 新增工作流故事组并更新两个索引

## 7. 第二期（不在本变更验收范围）

- [ ] 7.1 `tui` 执行器：`OSC 7501` 能力声明、`Submit` 提交、`working → done/idle` 判定
- [ ] 7.2 `parallel` 组与汇合
- [ ] 7.3 仓库内工作流文件的导入预览与显式确认
