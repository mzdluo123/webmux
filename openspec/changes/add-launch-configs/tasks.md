## 1. 服务端存储

- [ ] 1.1 新增 `internal/launch/types.go`：配置结构（`id`、`name`、`cmd`、`env`、`createdAt`、`updatedAt`）、默认与最大限制、名称去空白与唯一性校验、argv/env 防御性复制、`[a-z2-7]{8}` ID 生成与冲突重试
- [ ] 1.2 新增 `internal/launch/store.go`：从 `<state_dir>/launch-configs.json` 加载、首次按 TOML 预设种子（名称取预设 `name`、完整 argv、空 env）、CRUD、互斥锁、原子保存（临时文件 + `chmod 0600` + `fsync` + `rename` + 目录同步）与保存失败回滚
- [ ] 1.3 实现损坏与不可用处理：文件无法解析时保留原文件、进入只读不可用状态并记录路径；目录或文件不可读写时同样只禁用该功能；不自动重建，不覆盖用户数据
- [ ] 1.4 新增 `internal/launch/store_test.go`：重开保留、首次种子只发生一次、预设变更不影响已保存配置、空列表、重名、名称/argv/env 上限、损坏文件不被覆盖、保存失败回滚、ID 唯一性

## 2. API 与接线

- [ ] 2.1 新增 `internal/api/launch.go`：注册 `GET`/`POST /api/launch-configs` 与 `PUT`/`DELETE /api/launch-configs/{id}`；复用 `decode` 严格 JSON 与 64 KiB 上限；返回 200/201/204、400、404、409、500、503；错误文本包含状态文件路径但不包含 env 值
- [ ] 2.2 在 `internal/api/router.go` 的 `POST /api/sessions` 请求中增加可区分缺省与空值的 `configId`；实现与 `preset`/`cmd`/`args`/`env` 的互斥、未知 ID 404、`session.ValidateEnv` 二次校验，并调用 `Workspace.CreateSession(..., "custom", cfg.Cmd, nil, name, workspace, cfg.Env)`
- [ ] 2.3 在 `cmd/webmux/main.go` 创建并注入启动配置服务；同步更新 `cmd/baseline/fixture.go`、`internal/tray/forward/forward_test.go` 与所有 `api.New(...)` 调用点，以编译错误为准逐个处理
- [ ] 2.4 新增 `internal/api/launch_test.go`：认证与 Origin 继承、CRUD、严格 JSON、重名 409、未知 ID 404、存储损坏 503、保存失败 500 回滚
- [ ] 2.5 新增会话创建测试：`configId` 启动的 argv/env 正确、互斥字段 400、空 `configId` 400、未知 ID 404 且不创建会话、伪造 `WEBMUX_SESSION_ID` 被拒绝、删除配置不影响运行中会话、现有 `preset`/`cmd`/`env` 路径不回归

## 3. 前端

- [ ] 3.1 在 `web/src/schemas.ts` 增加 `LaunchConfigSchema`（`id`、`name`、`cmd`、`env`、`createdAt`、`updatedAt`）与 `LaunchConfigsSchema`（最多 100 条、ID 唯一），导出 `z.infer` 类型
- [ ] 3.2 在 `web/src/api.ts` 或独立工具模块新增 `quoteArg`/`joinArgs`，覆盖空白、单双引号、反斜杠、`$`、空参数与 `parseArgs` 的往返一致性；新增 Node 单元测试
- [ ] 3.3 新增 `web/src/components/LaunchConfigDialog.svelte`：名称、命令（等宽 Textarea）、环境变量行（添加/删除、默认遮挡与显示切换、名称校验与 `WEBMUX_SESSION_ID` 拒绝）、预设快捷填入、保存错误（400/409/503）保留输入
- [ ] 3.4 在 `Settings.svelte` 新增 `id="launch-configs"` 卡片、列表（名称/命令预览/env 数量）、空状态、编辑、删除确认与 100 条上限；支持 `#launch-configs` 滚动定位；加载或写入失败时显示状态文件路径与本页其他设置不受影响；修改顶部“仅保存在当前浏览器”的说明以区分服务端配置
- [ ] 3.5 调整 `SessionList.svelte` 与 `NewSessionSheet.svelte`：加载并传递启动配置；主路径选择配置并发送 `{configId, cwd | workspace, name}`；`lastLaunchConfig` 记住选择、首次按 `lastPreset` 选中同名种子；保留折叠的“临时命令（不保存）”路径；无配置或存储不可用时显示空状态、设置页链接与临时命令回退
- [ ] 3.6 增加前端 Node/浏览器测试：schema 校验、`quoteArg`/`joinArgs` 往返、设置对话框 env 遮挡与校验、新建会话选择/空状态/存储不可用回退

## 4. 测试、文档与评审

- [ ] 4.1 新增 Playwright E2E（隔离后端）：设置页创建带环境变量的启动配置 → 新建会话选择并启动 → 终端输出包含该变量 → 刷新或重新打开设置后配置仍在；删除配置后已运行会话继续工作
- [ ] 4.2 更新 `docs/protocol.md`（接口、`configId` 互斥与状态码）、`docs/usage.md`（配置管理与临时命令）、`docs/deployment.md`（`launch-configs.json` 状态文件、0600、单实例与损坏处理）
- [ ] 4.3 更新 `docs/user-stories.md`：新增 `US-B08 管理启动配置`，并更新 `US-B01`–`US-B03` 中“上次预设/自定义命令”的表述；核对索引、来源链接与计数
- [ ] 4.4 运行 `pnpm --dir web check`、`pnpm --dir web lint`、`make format-check`、`make test-unit`、`make test-browser`、`make test-e2e` 与相关 `go test -race ./internal/launch ./internal/api ./internal/workspace ./internal/session`
- [ ] 4.5 UI PR 附最终实现的桌面/移动与明/暗主题截图；截图使用不含真实密钥的示例配置
- [x] 4.6 在 `openspec/changes/add-launch-configs/` 撰写 proposal、design、spec delta 与任务清单（中文正文、英文结构标题与 SHALL/MUST）；本 draft PR 仅保存这些工件
