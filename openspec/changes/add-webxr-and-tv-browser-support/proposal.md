## Why

WebMux 已覆盖桌面和触屏浏览器，但头显中的大窗口、系统键盘与控制器输入，以及电视的远距离阅读和遥控器导航仍没有产品契约。用户已确认采用分阶段 WebXR 与 TV 浏览器支持，使现有持久终端和 CLI agent 能扩展到头显与客厅屏幕，而不引入原生应用或另建会话系统。

## What Changes

- 第一阶段支持头显浏览器中的完整 2D WebMux：会话、终端、Agent 聊天、代码与 Diff、草稿和快捷指令继续可用；适配控制器指针、系统虚拟键盘、实体键盘与大窗口布局。
- 第一阶段增加设备本地的 TV 模式：提供十英尺可读布局、始终可见的方向键焦点、跨区域空间导航、遥控器确认/返回、外部键盘输入，以及明确的终端输入焦点进入和退出路径。
- 第二阶段增加独立沉浸式 VR 工作台：从现有会话进入 VR，展示可移动和缩放的终端、聊天与代码面板，支持多会话切换、控制器射线与选择、滚动、快捷指令和 VR 键盘。
- 沉浸式模式复用现有 PTY、WebSocket、回放、控制权、草稿和发送语义；面板尺寸、双眼帧缓冲和头显姿态不得意外修改共享 PTY 网格。
- TV 模式、显式头显浏览器模式与 VR 面板布局只保存在当前设备；不按 UA、视口或 coarse pointer 猜测设备。进入、退出或意外结束 XR 会话不重建或终止服务端会话。
- 对 WebXR 不可用、非安全上下文、会话请求被拒和设备能力缺失提供可理解的降级；普通 2D 浏览器功能保持可用。
- 不纳入原生 XR/TV 应用、多人 avatar、3D 代码关系图，以及首版手势、眼动或透视 AR。

## Capabilities

### New Capabilities

- `headset-browser-support`: 规定头显浏览器中的 2D 会话操作、终端与 Agent 工作流、控制器指针、系统/实体键盘、大窗口布局和会话连续性。
- `tv-browser-support`: 规定设备本地 TV 模式、十英尺布局、空间焦点导航、遥控器与外部键盘输入、终端焦点边界和弹层返回顺序。
- `immersive-xr-terminal`: 规定沉浸式 VR 的能力检测、进入退出、空间终端/聊天/代码面板、控制器与 VR 键盘、共享 PTY 尺寸稳定性和 XR 生命周期。

### Modified Capabilities

- `terminal-header`: 在宽屏既有操作集合之外定义 TV 模式收纳和沉浸式 VR 入口的显示条件、用户激活与失败反馈，避免与当前宽屏和全屏要求冲突。
- `server-settings-sync`: 明确 TV 模式与 VR 面板布局属于设备本地显示偏好，不进入服务端设置文档、API 或跨客户端同步。

## Impact

- 前端外壳与输入：`web/src/components/shell/AppShell.svelte`、`web/src/pages/Terminal.svelte`；终端组件位于 `web/src/components/terminal/`，输入适配位于 `web/src/hooks/terminal/terminal-input.ts`。
- 终端呈现与生命周期：`web/src/hooks/terminal/` 中的 `terminal.svelte.ts`、`terminal-renderer.ts`、`terminal-socket.svelte.ts` 与 `xr-*`；视口和全屏控制器位于 `web/src/hooks/shared/`。
- 空间导航复用 `web/src/platform/spatial-navigation.ts`；XR 布局、文本和面板契约位于 `web/src/domains/terminal/`，代码/Diff 适配位于 `web/src/domains/code/`，不复制协议或会话状态。
- 设备偏好：复用 `web/src/hooks/shared/device-preferences.svelte.ts` 与 Zod 校验，不扩大服务端设置契约。
- 依赖：第一阶段不引入 3D 引擎；第二阶段增加按需加载的 Three.js/WebXR 呈现依赖，xterm 的 WebGL 上下文与 XR 帧缓冲保持独立。
- 测试：扩展现有 Vitest 浏览器测试、Storybook 大视口矩阵和隔离后端 Playwright；以模拟 XR 会话验证状态机，用真实头显和电视验证安全上下文、控制器、系统键、IME 与立体显示。
- 部署与安全：沉浸式 WebXR 要求 HTTPS 安全上下文并保留现有认证、Host/Origin 校验；不新增匿名接口或放宽公网 Shell 安全边界。
- 用户故事：同步新增/更新头显、TV 与沉浸式工作台故事；不修改后端 REST/WebSocket JSON 契约。
