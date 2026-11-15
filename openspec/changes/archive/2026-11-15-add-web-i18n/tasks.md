# Tasks: add-web-i18n

## 1. 基础设施与架构守卫

- [x] 1.1 安装 `@inlang/paraglide-js` 与 `@inlang/plugin-message-format`，配置本地离线 `web/project.inlang/` 及 `compile:i18n` 脚本。
- [x] 1.2 实现 `web/src/lib/locale.ts` 响应式语言控制器、`web/index.html` 首屏 `<html lang>` 初始化与 `storage.ts` 本机作用域键登记。
- [x] 1.3 更新 `.dependency-cruiser.cjs`、`eslint.config.mjs`、`knip.json`、`.jscpd.json` 与 `test/architecture.test.ts` 适配 `src/lib/paraglide/`。

## 2. 语言切换控件与全站文案国际化

- [x] 2.1 新增 `LocaleChoice.svelte` 并接入登录门禁 `ConnectionGate.svelte` 与设置页 `AppearanceSettings.svelte`。
- [x] 2.2 抽取并翻译应用外壳、页面、契约校验（Zod 动态错误回调）与平台传输提示文案。
- [x] 2.3 抽取并翻译设置、通知、会话列表、混合聊天、终端（含草稿、软键与 WebXR 面板）、代码阅读器（含 Diff Worker）及 Git 工作台与仓库管理全部界面文案。
- [x] 2.4 将界面中的 `toLocaleString` / `toLocaleTimeString` / `Intl` 调用接入当前生效 `locale.current`。

## 3. 测试门禁、文档与交付证据

- [x] 3.1 编写 `web/test/i18n.test.ts` 覆盖系统语言解析、响应式切换、双语键值/占位符一致性与源码硬编码中文防回流检查。
- [x] 3.2 编写组件浏览器测试与 Playwright E2E（含 `en-US` 默认启动、登录门禁与设置页实时切换、刷新保持及桌面/手机窄屏防溢出检查）。
- [x] 3.3 更新 `openspec/specs/web-i18n/spec.md`、`docs/specs.md`、`docs/frontend-architecture.md`、`docs/usage/sessions.md` 与 User Stories（`US-A07`）及核对记录。
