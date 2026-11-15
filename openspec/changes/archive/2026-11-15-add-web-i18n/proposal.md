# Proposal: 网页前端中英文国际化（web-i18n）

## Why

WebMux 网页前端原先将所有界面文案、无障碍标签、前端契约校验错误与日期数字格式硬编码为简体中文。英文系统环境或偏好英文界面的开发者在登录门禁、会话工作台、终端、混合聊天、代码阅读器与 Git 工作台中无法以英文操作界面。

## What Changes

- 引入本地离线编译的 `@inlang/paraglide-js` 与 `@inlang/plugin-message-format`，建立 `web/messages/zh-CN.json` 与 `web/messages/en.json` 双语目录，在构建与测试前生成类型安全的 `src/lib/paraglide/` 模块。
- 新增 `web/src/lib/locale.ts` 响应式语言控制器（基于 `svelte/reactivity` 的 `createSubscriber`）与 `web/index.html` 首屏 `<html lang>` 同步脚本：
  - 支持 `system`（默认跟随浏览器语言）、`zh-CN`、`en` 三档选择；
  - 偏好以本机设备键 `webmux.locale` 存储在当前浏览器 `localStorage`，不跨设备写入服务端 `/api/settings`；
  - 监听 `languagechange` 与跨标签页 `storage` 事件动态同步。
- 新增共享 `LocaleChoice.svelte` 分段控件，同时接入未登录 `ConnectionGate` 与“设置 → 外观与阅读”（`AppearanceSettings`），切换时全站文案、Zod 动态校验提示及日期/数字格式即时无刷新生效，不中断已有 WebSocket 会话连接。
- 抽取并翻译 `web/src/` 全部页面的用户可见文案、无障碍属性与前端错误提示，补充单元、组件浏览器与端到端多语言及窄屏防溢出回归。

## Capabilities

### New Capabilities

- `web-i18n`: 网页前端中英文消息目录、默认跟随系统与本机持久化语言偏好、登录页与设置页无刷新切换。

### Modified Capabilities

- `frontend-architecture-guard`: 允许 `src/contracts/` 依赖生成的纯函数国际化消息模块 `src/lib/paraglide/`，并将生成目录纳入构建工具忽略与架构守卫边界。

## Impact

- 前端依赖与构建：`web/package.json`、`web/vite.config.ts`、`web/project.inlang/`、`web/messages/`。
- 前端源码：`web/index.html`、`web/src/lib/locale.ts`、`web/src/platform/storage.ts`、`web/src/components/shared/LocaleChoice.svelte` 及 `web/src/` 各业务层组件、hooks、domains 与 contracts。
- 后端、CLI、Windows 托盘与线上 REST/WebSocket wire 契约无变更。
