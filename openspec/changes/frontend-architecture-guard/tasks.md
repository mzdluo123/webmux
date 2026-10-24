## 1. 守卫工具链与基线（首个 draft PR）

- [x] 1.1 `web/package.json`：新增 `eslint`、`@shadcn/lint`、`eslint-plugin-svelte`、`svelte-eslint-parser`、`@typescript-eslint/parser`、`dependency-cruiser`、`jscpd`、`knip` 开发依赖，用 pnpm 12.11.2 更新锁文件，构建脚本白名单不变
- [x] 1.2 `web/eslint.config.mjs`：只加载架构规则（`@shadcn/lint` 六条规则、`svelte/no-restricted-html-elements`、原语/变体工具/`@tabler/icons-svelte` 导入限制），`ui/` 目录豁免，保留 `CardTitle`、`CardContent` 契约
- [x] 1.3 `web/.dependency-cruiser.cjs`：按扁平目录定义 `ui-is-a-leaf`、`lib-is-framework-free`、`only-app-composes-pages`、`components-do-not-know-pages`、`hooks-do-not-render`、`domain-does-not-render`、`primitives-only-in-ui`、`variants-only-in-ui`、`no-circular`、`no-orphans`
- [x] 1.4 `web/.jscpd.json`、`web/knip.json`：重复代码检查忽略测试、stories、`ui/` 与生成的字体 CSS；knip 的 `exports`/`types` 暂为警告
- [x] 1.5 `web/test/architecture.test.ts`：同名业务组件、与 `ui/` 同名的业务组件、跨 3 个以上文件复制的外观类；用临时文件确认能报出违规
- [x] 1.6 生成并格式化基线：`eslint-suppressions.json`（40 个文件、225 处）、`.dependency-cruiser-known-violations.json`（4 处）、`.jscpd-baseline.json`（3 处）
- [x] 1.7 `arch:*`、`check:arch`、`arch:ratchet` 脚本；`make check` 与 CI 格式和构建 job 运行 `check:arch`
- [x] 1.8 文档：新增 `docs/frontend-architecture.md`，更新 `AGENTS.md`、`CONTRIBUTING.md`、`docs/design-system.md`，在 `docs/user-stories.md` 新增 US-L05

## 2. 不改变 UI 的治理

- [ ] 2.1 删除 knip 报告的未使用导出（`api.ts` 的 `statusText`、`attentionText`，`code-diff-model.ts` 的 `DiffResultSchema`，`code-diff.ts` 的 `DIFF_BUDGET_MS`，`fonts.ts` 的 `terminalFontFamily`，`homeWorktrees.ts` 的 `pinnedIds`，`launchConfigs.ts` 的 `LAUNCH_NAME_MAX`、`LAUNCH_ARGS_MAX`、`LAUNCH_VALUE_BYTES`、`envNameError`，`terminal-draft.svelte.ts` 的 `DraftSubmission`），只用于模块内部的改为不导出
- [ ] 2.2 分层：把 `code-search.ts`、`code-reader-panels.ts`、`codePopout.ts` 中挂载 Svelte 组件的部分移到组件层（如 `components/` 下的挂载适配模块），领域模块只保留纯逻辑；`code-search.test.ts` 等单元测试保持在 Node 中运行
- [ ] 2.3 Iconify 迁移：21 个业务文件从 `@tabler/icons-svelte` 改为 `~icons/tabler/*`，图标与尺寸不变；确认 `@tabler/icons-svelte` 只剩 `ui/` 使用，或随 `components.json` 一起切换
- [ ] 2.4 未知类名：`code-goto-line`、`code-reader`、`code-search`、`session-link`、`workspace-header`、`workspace-toolbar` 改为 `data-*` 属性（同步 `test/e2e`、`test/browser`、`test/performance` 中的选择器）；确认 `cn-rtl-flip` 与上游 registry 的关系后补充 `@utility` 或删除
- [ ] 2.5 任意值：能用 Tailwind 内置工具类表达的直接替换（`[overflow-wrap:anywhere]` → `wrap-anywhere`、`[field-sizing:fixed]` → `field-sizing-fixed`、`[contain:strict]` → `contain-strict`、`[scrollbar-width:thin]` → `scrollbar-thin`；`[overflow-anchor:none]` 没有内置工具类，用 `@utility` 命名），其余重复出现的值（`max-h-[50dvh]`、`w-[3ch]`、`max-w-[calc(100vw-1rem)]`、对话框高度、安全区内边距）在 `styles.css` 中用 `@utility` 或主题变量命名，单次出现的值在评审中决定
- [ ] 2.6 重复代码：`Settings.svelte` 中两段相同的模板、`SoftKeys.svelte` 中的重复块提取为片段或组件；`design-tokens.css` 的明暗主题重复段确认是否保留，保留时从基线移到 jscpd 忽略配置并注明原因

## 3. 设计系统补充 size / variant（需要截图）

- [ ] 3.1 Button：为现有覆盖补充 size 或 variant（触控尺寸 `size-11 sm:size-9` 等、浮动按钮的边框与阴影、等宽小字），替换 `CodeLineAction`、`CodeSearchPanel`、`NewSessionSheet`、`ScratchPanel`、`SessionFilters`、`SessionRow`、`TerminalViewport`、`WorkspaceGroup` 中的覆盖
- [ ] 3.2 `lib/touch-targets.ts`：动态拼接的触控类改为 Button/DropdownMenuItem 的 size 或 variant，消除 `ScratchPanel`、`QuickCommandMenu` 的 11 处 `require-static-classes`
- [ ] 3.3 Input、Textarea：为 18 处覆盖（字号、内边距、高度、等宽字体）补充 size 或 variant，或确认属于合法契约后写入 ESLint 契约
- [ ] 3.4 容器类组件：PopoverContent、DialogContent、DialogHeader、TabsContent、CommandList、ItemGroup、ItemActions、Collapsible 系列与 Empty 的内边距和边框覆盖，逐个决定增加 variant、写入契约或删除覆盖
- [ ] 3.5 其余：CardTitle 的字重、DropdownMenuLabel 的颜色、Badge/ItemTitle/ResizableHandle/ScrollArea 的覆盖
- [ ] 3.6 每个 PR 附受影响界面的前后截图（明暗主题、桌面与 393 px 手机），并运行 `arch:ratchet` 提交收缩后的基线

## 4. 原生控件换成设计系统组件（需要截图）

- [ ] 4.1 `CodeNavigator.svelte`：3 个 `<button>`、4 个 `<input>`、2 个 `<label>`、1 个 `<select>` 换成 Button、Input、Label 与 DropdownMenu/Command；保持键盘导航与现有 E2E 行为
- [ ] 4.2 `CodePanel.svelte`（2 个 `<button>`）、`CodeChangesList.svelte`（虚拟列表行 `<button>`，确认 Button 的 ghost variant 不破坏行高与焦点样式）
- [ ] 4.3 `NewSessionSheet.svelte`（2 个 `<select>`、1 个 `<button>`）、`CodeDocumentView.svelte`（`<label>` + `<select>`，沿用 compact-terminal-chrome 中窄屏改菜单单选的做法）
- [ ] 4.4 `RemoveWorktreeDialog.svelte` 的复选框改用 Checkbox（需要先用 shadcn-svelte 添加）与 Label；`TerminalHeader.svelte` 的隐藏文件输入、`Settings.svelte` 的 2 个 `<button>` 逐个处理，隐藏的 `<input type="file">` 如无对应组件，在文档中登记为例外

## 5. 运行时几何值

- [ ] 5.1 `CodeChangesList`（虚拟列表行高与偏移）、`CodeImageView`（缩放与平移）、`CodeReader`（操作按钮位置）、`SessionRow` 与 `Settings`（预览行数）、`Terminal`（视口高度）中的 10 处 `style:` 改为 CSS 自定义属性加工具类；无法改写的登记为例外

## 6. 收尾

- [ ] 6.1 确认三个基线文件为空，ESLint 契约与例外清单都已写入 `docs/frontend-architecture.md`
- [ ] 6.2 knip 的 `exports`、`types` 升为 `error`
- [ ] 6.3 更新 US-L05 与 `docs/frontend-architecture.md` 中的存量说明；运行 `make check`、`make test-unit`、`make test-browser`、`make test-e2e` 与 Storybook 检查
