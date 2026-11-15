# web-i18n Specification

## Purpose

为 WebMux 网页前端提供基于 `@inlang/paraglide-js` 的中英文（`zh-CN` / `en`）国际化能力，支持默认跟随系统语言、按浏览器本机持久化的显式语言选择、登录页与设置页即时无刷新切换，以及随语言同步的 `<html lang>`、契约校验提示与日期数字格式。

## Requirements

### Requirement: 支持的界面语言与编译期目录校验

WebMux 网页前端 SHALL 内置简体中文（`zh-CN`，基准语言）与英语（`en`）两套完整消息目录，并通过本地 `@inlang/paraglide-js` 与 `@inlang/plugin-message-format` 在构建及测试前生成类型化消息模块，构建与运行期间 MUST NOT 访问外部 CDN 或远程翻译服务。

`zh-CN` 与 `en` 目录 MUST 定义完全一致的消息键集合、非空译文及匹配的占位符参数；网页业务源码（`web/src/`，除语言选择器自身的语言自称外）MUST NOT 残留未抽取的硬编码中文界面文案。

#### Scenario: 消息目录键或占位符不一致

- **WHEN** `web/messages/en.json` 缺少 `web/messages/zh-CN.json` 中的任意键、包含空字符串译文或遗漏 `{param}` 占位符
- **THEN** 前端国际化单元门禁失败并指出具体缺失键或占位符差异

#### Scenario: 离线可复现构建

- **WHEN** 在无外网访问的构建环境中执行 `pnpm --dir web compile:i18n` 与 `pnpm --dir web build`
- **THEN** Paraglide 从本地 `node_modules` 加载插件完成编译，构建以退出码 0 完成

### Requirement: 本机持久化的界面语言偏好

界面语言偏好 SHALL 支持 `system`（跟随系统，默认值）、`zh-CN`（简体中文）与 `en`（English）三个取值。

- 显式选择 `zh-CN` 或 `en` 时，前端 SHALL 将值写入当前浏览器 `localStorage` 键 `webmux.locale`；选择 `system` 时移除该键。
- `webmux.locale` MUST 登记为本机作用域存储键：认证失效清理凭据时保留本机语言偏好，且 MUST NOT 写入服务端 `/api/settings` 同步。

#### Scenario: 显式语言在刷新后保持且不影响服务端设置

- **WHEN** 使用者在英文系统浏览器中将界面语言切为“简体中文”并刷新页面
- **THEN** 首屏前内联脚本将 `document.documentElement.lang` 设为 `"zh-CN"`，应用以简体中文渲染，且不发起 `/api/settings` 写入请求

### Requirement: 跟随系统语言解析与环境同步

当偏好为 `system`（或 `localStorage` 不可用、存储值无效）时，前端 SHALL 按顺序检查 `navigator.languages`（回退 `navigator.language`）：

- 首个匹配 `zh` 或 `zh-*` 的标签解析为 `zh-CN`；
- 首个匹配 `en` 或 `en-*` 的标签解析为 `en`；
- 存在其他非空语言标签且无中文标签时回退为 `en`，语言列表为空时回退为 `zh-CN`。

浏览器触发 `languagechange` 事件，或同源标签页修改 `webmux.locale` 触发 `storage` 事件时，当前页面 MUST 自动重算生效语言并更新界面。

#### Scenario: 英文浏览器首次打开默认显示英文

- **WHEN** 当前浏览器未保存 `webmux.locale` 且 `navigator.languages` 为 `["en-US", "en"]`
- **THEN** 生效界面语言为 `en`，登录门禁、工作台、新建会话、设置页与终端界面全部显示英文，且 `<html lang>` 为 `"en"`

#### Scenario: 中文优先于排在后面的英文

- **WHEN** 当前浏览器未保存 `webmux.locale` 且 `navigator.languages` 为 `["zh-TW", "en-US"]`
- **THEN** 生效界面语言解析为 `zh-CN`，`<html lang>` 为 `"zh-CN"`

### Requirement: 登录门禁与设置页无刷新即时切换

前端 SHALL 在以下两处提供共享的界面语言分段选择控件（`跟随系统 / Follow system`、`简体中文`、`English`）：

1. 未认证或离线时的连接与 Token 登录门禁页（`ConnectionGate`），使尚未登录的使用者也能阅读提示并完成认证；
2. 已进入应用后的“设置 → 外观与阅读”分区（`AppearanceSettings`），且不受服务端设置尚未就绪（`!settings.ready`）的禁用态阻断。

切换语言偏好时，当前页面所有已挂载组件、派生状态、浏览器标签页标题、前端 Zod 契约校验错误提示以及日期/数字本地化格式（`toLocaleString`、`toLocaleTimeString`、`Intl`）MUST 在不整页刷新、不断开现有终端或聊天 WebSocket 连接的前提下立即切换为目标语言。

#### Scenario: 在登录门禁即时切换语言

- **WHEN** 使用者停留在共享 Token 登录页，点击 `English` 再点击 `简体中文`
- **THEN** 卡片标题、Token 帮助说明、连接按钮与 `<html lang>` 立即在英文与简体中文之间切换，已输入的 Token 草稿保持不丢失

#### Scenario: 会话连接期间在设置页切换语言

- **WHEN** 使用者在已有运行中会话时打开“设置 → 外观与阅读”并将语言切换为 `English`
- **THEN** 设置分区导航、侧栏、会话状态文案与时间数字格式立即更新为英文，页面不重新加载，终端与会话流连接保持不断开
