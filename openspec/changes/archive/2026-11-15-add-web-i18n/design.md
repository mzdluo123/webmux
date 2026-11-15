# Design: 网页前端中英文国际化架构

## Context

WebMux 前端采用 Svelte 5 runes、严格分层（`contracts` → `platform` → `domains` → `hooks` → `components` → `pages`）以及包含普通 Node 环境 Vitest 单元测试与 Chromium 浏览器测试的双层测试架构。国际化方案需要同时满足：

1. 零外网依赖、可复现的离线构建；
2. 切换语言无需刷新页面，不打断持久终端或混合聊天连接；
3. 在未启用 Svelte 编译器的 Node 单元测试（`vitest.config.ts`）与 Svelte 5 浏览器组件中均可直接调用；
4. 现有默认中文测试套件无需重写即可保持确定性通过。

## Goals / Non-Goals

- **Goals**:
  - 完整覆盖 `web/src/` 所有页面、弹窗、工具栏、无障碍名称、前端校验提示与本地化时间数字格式的中英文支持。
  - 提供 `system` / `zh-CN` / `en` 三态本机偏好，同时在登录门禁与设置页提供一致的切换体验。
  - 通过静态测试门禁长期防止键值缺失、占位符不一致或新增硬编码中文文案回流。
- **Non-Goals**:
  - 不翻译 Go 服务端错误原文、`webmux` CLI 帮助输出与 Windows 托盘程序。
  - 不把语言偏好加入服务端 `/api/settings` 跨设备同步，保持每台浏览器独立选择。

## Decisions

1. **选用 `@inlang/paraglide-js` + 本地 `@inlang/plugin-message-format`**：
   - `project.inlang/settings.json` 直接引用 `./node_modules/@inlang/plugin-message-format/dist/index.js`，杜绝构建期远程拉取插件。
   - 生成目标目录为 `web/src/lib/paraglide`（加入 `.gitignore`，由 `prepare`、`compile:i18n` 与 Vite 插件生成），产出按语言拆分的纯 ES 模块与 TypeScript 声明。
2. **用 `svelte/reactivity` 的 `createSubscriber` 桥接 Paraglide runtime**：
   - 在纯 TypeScript 模块 `web/src/lib/locale.ts` 中调用 `overwriteGetLocale` 读取订阅信号。
   - Svelte 5 模板与 `$derived` 读取任意 `m.key()` 时自动建立响应式依赖，修改偏好立即触发局部重渲染；Node 环境下 `createSubscriber` 安全退化为无操作读取，兼容全部纯函数单元测试。
3. **动态求值规避模块初始化期冻结**：
   - `contracts/*.ts` 使用 Zod v4 的 `{ error: () => m.xxx() }` 回调；
   - `domains/` 中导出的预设或选项常量数组通过对象 getter（`get label() { return m.xxx(); }`）保持原有导出类型与调用签名，同时每次读取按当前生效语言求值；
   - Diff Web Worker（`code-diff.worker.ts`）通过主线程 `postMessage` 传入当前生效 `locale`，保证 Worker 内限流提示与主线程语言一致。
4. **测试默认语言固定为 `zh-CN`，多语言用例显式覆盖 `en`**：
   - Playwright、Storybook 与 Vitest 默认 locale 设为 `zh-CN`，保证存量中文断言稳定；新增单元、浏览器与 E2E 套件显式测试 `en-US` 系统语言自动匹配、手动切换与 320px 窄屏无溢出。
