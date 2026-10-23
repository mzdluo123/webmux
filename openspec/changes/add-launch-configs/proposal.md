## Why

新建会话时的“启动命令 + 参数”目前一部分来自启动时加载的 TOML `[[presets]]`，一部分来自浏览器 `localStorage` 中的上次预设与自定义命令。追加参数、自定义命令和环境变量无法保存为可复用的服务端配置，换浏览器或换设备就会丢失，也无法作为后续一键启动和 CLI 复用的稳定来源。

本变更建立将启动配置抽象为服务端持久化资源的规格：以“名称 + 完整 argv + 环境变量”为单位保存，由新建会话页、后续首页一键启动和 CLI 共同复用。本变更只提交 OpenSpec 工件，不包含实现代码。

## What Changes

- 新增服务端持久化的启动配置：名称、完整 argv 与环境变量，保存在 `<state_dir>/launch-configs.json`；跨服务重启保留，跨浏览器与设备共享。
- 启动配置保存的是创建时的命令快照：预设或其参数在 TOML 中后续被修改、删除，都不改变已保存配置的启动行为。
- 新增 `/api/launch-configs` 的 `GET`/`POST` 与 `/api/launch-configs/{id}` 的 `PUT`/`DELETE`；沿用现有认证、Origin 校验和严格 JSON 规则。文件损坏或存储不可用时只禁用启动配置功能并返回 503，会话创建与终端连接不受影响。
- 首次启动且状态文件不存在时，按当前 TOML 预设生成一组默认配置；此后不再与 TOML 同步，也不覆盖损坏文件。
- `POST /api/sessions` 新增 `configId` 启动方式，与现有 `preset`、`cmd`、`args`、`env` 互斥；`cwd`、`workspace`、`name` 继续由本次启动决定。通过配置启动的会话记录为 `preset: "custom"`，`cmd` 字段保存完整 argv；删除配置不影响已运行会话。
- 设置页新增“启动配置”卡片与编辑对话框（名称、命令、环境变量，值默认遮挡）；新建会话页改为以启动配置为主，保留折叠的“临时命令（不保存）”作为一次性使用与存储不可用时的回退。
- 保留现有 `preset`/`cmd`/`args`/`env` 创建方式，兼容脚本、E2E 与后续 CLI。
- 首页一键启动、在 `SessionInfo` 中暴露配置 ID、配置导入导出、变量模板与配置级默认工作目录不在本期范围。

## Capabilities

### New Capabilities

- `launch-configs`：启动配置的数据模型与命令快照语义、服务端持久化与损坏处理、CRUD API、校验与安全边界、设置页管理界面，以及启动配置在设备间共享的约定。

### Modified Capabilities

- `session-launch-environment`：新增通过 `configId` 创建会话的请求契约、与现有命令字段的互斥规则，以及启动时使用已保存 argv/env 快照的语义。

## Impact

- 后端：新增 `internal/launch` 包（存储与服务）、`internal/api` 的启动配置路由与会话创建扩展；`cmd/webmux/main.go`、`cmd/baseline/fixture.go`、托盘转发的测试夹具及所有 `api.New(...)` 调用点需要注入新服务。PTY 启动、回放、退出顺序与会话清理路径不变。
- 协议：增加 `/api/launch-configs` 资源与 `POST /api/sessions` 的 `configId` 字段；`SessionInfo` 契约本期不变，终端二进制帧协议不变。
- 前端：`web/src/schemas.ts` 增加启动配置 Zod 契约；`Settings.svelte` 新增卡片，新增编辑对话框组件；`NewSessionSheet.svelte` 与 `SessionList.svelte` 加载、选择和传递启动配置。
- 状态目录：新增 `launch-configs.json`（0600）；同一 `state_dir` 仍只允许一个服务实例使用。
- 文档：实施时更新 `docs/protocol.md`、`docs/usage.md`、`docs/deployment.md` 与 `docs/user-stories.md`（新增启动配置故事，并调整“上次预设/自定义命令”的表述）。
- 验证：存储持久化/重开/损坏/限制/重名、API CRUD 与认证继承、通过配置启动的 argv/env、删除配置不影响运行中会话、设置页到终端的隔离 E2E；UI 变更按仓库要求附桌面/移动与明/暗主题截图。
- 交付顺序：先落地服务端存储与 API，再接入前端；本变更的规格与任务由本 draft PR 保存，实现由后续 PR 完成。
