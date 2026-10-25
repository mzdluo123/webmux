## 1. 守卫工具链与基线（首个 draft PR）

- [x] 1.1 `web/package.json`：新增 `eslint`、`@shadcn/lint`、`eslint-plugin-svelte`、`svelte-eslint-parser`、`@typescript-eslint/parser`、`dependency-cruiser`、`jscpd`、`knip` 开发依赖，用 pnpm 12.11.2 更新锁文件，构建脚本白名单不变
- [x] 1.2 `web/eslint.config.mjs`：只加载架构规则（`@shadcn/lint` 六条规则、`svelte/no-restricted-html-elements`、原语/变体工具/`@tabler/icons-svelte` 导入限制），`ui/` 目录豁免，保留 `CardTitle`、`CardContent` 契约
- [x] 1.3 `web/.dependency-cruiser.cjs`：按扁平目录定义 `ui-is-a-leaf`、`lib-is-framework-free`、`only-app-composes-pages`、`components-do-not-know-pages`、`hooks-do-not-render`、`domain-does-not-render`、`primitives-only-in-ui`、`variants-only-in-ui`、`no-circular`、`no-orphans`
- [x] 1.4 `web/.jscpd.json`、`web/knip.json`：重复代码检查忽略测试、stories、`ui/` 与生成的字体 CSS；knip 的 `exports`/`types` 先为警告（2.1 完成后升为 `error`）
- [x] 1.5 `web/test/architecture.test.ts`：同名业务组件、与 `ui/` 同名的业务组件、跨 3 个以上文件复制的外观类；用临时文件确认能报出违规
- [x] 1.6 生成并格式化基线：`eslint-suppressions.json`（40 个文件、225 处）、`.dependency-cruiser-known-violations.json`（4 处）、`.jscpd-baseline.json`（3 处）
- [x] 1.7 `arch:*`、`check:arch`、`arch:ratchet` 脚本；`make check` 与 CI 格式和构建 job 运行 `check:arch`
- [x] 1.8 文档：新增 `docs/frontend-architecture.md`，更新 `AGENTS.md`、`CONTRIBUTING.md`、`docs/design-system.md`，在 `docs/user-stories.md` 新增 US-L05

## 2. 不改变 UI 的治理

- [x] 2.1 删除 knip 报告的未使用导出（`api.ts` 的 `statusText`、`attentionText`，`code-diff-model.ts` 的 `DiffResultSchema`，`code-diff.ts` 的 `DIFF_BUDGET_MS`，`fonts.ts` 的 `terminalFontFamily`，`homeWorktrees.ts` 的 `pinnedIds`，`launchConfigs.ts` 的 `LAUNCH_NAME_MAX`、`LAUNCH_ARGS_MAX`、`LAUNCH_VALUE_BYTES`、`envNameError`，`terminal-draft.svelte.ts` 的 `DraftSubmission`），只用于模块内部的改为不导出
- [x] 2.2 分层：`SearchSpec` 类型移入 `code-search.ts`；挂载 Svelte 组件的 `code-reader-panels.ts`、`codePopout.ts` 移到 `components/`；dependency-cruiser 基线清空，`code-search.test.ts` 仍在 Node 中运行
- [x] 2.3 Iconify 迁移：21 个业务文件从 `@tabler/icons-svelte` 改为 `~icons/tabler/*`，图标名逐个对照 `@iconify-json/tabler` 校验；未设尺寸类的图标都位于 Button、DropdownMenu.Item、Empty.Media 等以 `[&_svg:not([class*=size-])]` 设置尺寸的容器内，尺寸不变。`ui/` 中 6 个生成组件仍使用 `@tabler/icons-svelte`（`components.json` 的 `iconLibrary`），随生成器一并处理
- [x] 2.4 未知类名：`code-goto-line`、`code-reader`、`code-search`、`session-link` 改为 `data-*` 属性，同步源码与 `test/e2e`、`test/browser`、`test/performance` 中的选择器；未被引用的 `workspace-header`、`workspace-toolbar` 删除
- [x] 2.5 任意值（有完全等价内置工具类的部分，逐个编译确认 CSS 相同）：`[overflow-wrap:anywhere]` → `wrap-anywhere`、`[field-sizing:fixed]` → `field-sizing-fixed`、`[contain:strict]` → `contain-strict`、`[scrollbar-width:thin]` → `scrollbar-thin`、`flex-[2]` → `flex-2`、`focus-visible:ring-[3px]` → `focus-visible:ring-3`、`mt-[0.3125rem]` → `mt-1.25`（47 → 33 处）
- [x] 2.6 重复代码：`Settings.svelte` 的两个删除按钮与 `SoftKeys.svelte` 的三段发送键按钮改为 snippet；`design-tokens.css` 的 `.dark` 中与 `:root` 完全相同的 `--chart-1..5` 删除；jscpd 基线清空
- [ ] 2.7 其余 33 处任意值：重复出现的值（`max-h-[50dvh]`、`w-[3ch]`、`max-w-[calc(100vw-1rem)]`、对话框高度、安全区内边距、`[overflow-anchor:none]`、`[direction:rtl]`）在 `styles.css` 中用 `@utility` 或主题变量命名，单次出现的值在评审中决定
- [ ] 2.8 确认 `cn-rtl-flip` 与上游 registry 的关系后补充 `@utility` 或删除

## 3. 设计系统补充 size / variant（需要截图）

- [x] 3.1 Button：新增 `variant="quiet"`（ScratchPanel 清空）、`raised`（CodeLineAction 与“回到最新”浮动按钮；原 `TerminalViewport` 的 `border` 因基础样式边框色透明而无效，现在显示边框，`CodeLineAction` 阴影由 `shadow-sm` 统一为 `shadow-md`）、`size="icon-touch"` / `size="sm-touch"`（窄屏 44px 触控）；`CodeLineAction` 的 `size-9`、`NewSessionSheet` 的 `text-xs`、`SessionFilters` 的 `gap-1.5` 与基础样式重复，直接删除；`SessionFilters` 改用 `size="xs"`（标签与计数间距由 6px 变为 4px）；`CodeSearchPanel` 的等宽字形与 `ScratchPanel` 取用按钮的强调色移到按钮内容；`RemoveWorktreeDialog` 的“进入强制删除确认”改为 `variant="destructive"`
- [x] 3.2 删除 `lib/touch-targets.ts`：图标与小按钮用 3.1 的触控尺寸，菜单项写静态的 `min-h-11 sm:min-h-9` 与 `any-pointer-coarse:min-h-11`，消除 11 处 `require-static-classes`
- [x] 3.3 Input、Textarea：新增 `mono`（11 处等宽覆盖）与 Textarea `variant="ghost"`（暂存条目原地编辑）；会话搜索框改用 InputGroup（图标与 `/` 提示作为 addon），InputGroup 基础样式与本项目 Input 对齐（`bg-card`、2px 焦点环）
- [x] 3.4 容器：`TabsContent`、`CommandList`、`ItemGroup`、`ItemActions`、`Collapsible`、`CollapsibleContent`、`DialogContent`、`DialogHeader`、`PopoverContent`、`ResponsiveDialog` 写入“可调整内边距与间距”的契约；`Empty` 新增 `outline` / `card`（删除 `.workspace-empty` 样式），`Command` 新增 `outline`，`Item` 新增 `card` 与 `size="flush"`（删除 `.session-row` 样式，测试改用 `data-session-id`）
- [x] 3.5 其余：`DropdownMenuLabel` 新增 `variant="title"`（代码区菜单的路径标题由常规字重变为 `font-medium`）；`CollapsibleTrigger`、`PopoverTrigger` 为无样式原语，写入契约；`Pane`、`ResizableHandle` 的布局钩子类写入契约；`DirectoryPicker` 的布局钩子改为 `data-*` 属性；`TokenHelp` 的行高移到各段落
- [x] 3.6 用同一组隔离数据在改动前后各截取 78 张截图（21 个界面，明暗主题、桌面 1280 px 与 Pixel 7），逐像素对比：改前两次截图之间的噪声（会话状态与随机分支名）之外，只有 3.1、3.3、3.5 列出的有意变化；PR 附前后截图，并运行 `arch:ratchet` 提交收缩后的基线

## 4. 原生控件换成设计系统组件（需要截图）

- [ ] 4.1 `CodeNavigator.svelte`：3 个 `<button>`、4 个 `<input>`、2 个 `<label>`、1 个 `<select>` 换成 Button、Input、Label 与 DropdownMenu/Command；保持键盘导航与现有 E2E 行为
- [ ] 4.2 `CodePanel.svelte`（2 个 `<button>`）、`CodeChangesList.svelte`（虚拟列表行 `<button>`，确认 Button 的 ghost variant 不破坏行高与焦点样式）
- [ ] 4.3 `NewSessionSheet.svelte`（2 个 `<select>`、1 个 `<button>`）、`CodeDocumentView.svelte`（`<label>` + `<select>`，沿用 compact-terminal-chrome 中窄屏改菜单单选的做法）
- [ ] 4.4 `RemoveWorktreeDialog.svelte` 的复选框改用 Checkbox（需要先用 shadcn-svelte 添加）与 Label；`TerminalHeader.svelte` 的隐藏文件输入、`Settings.svelte` 的 2 个 `<button>` 逐个处理，隐藏的 `<input type="file">` 如无对应组件，在文档中登记为例外

## 5. 运行时几何值

- [ ] 5.1 `CodeChangesList`（虚拟列表行高与偏移）、`CodeImageView`（缩放与平移）、`CodeReader`（操作按钮位置）、`SessionRow` 与 `Settings`（预览行数）、`Terminal`（视口高度）中的 10 处 `style:` 改为 CSS 自定义属性加工具类；无法改写的登记为例外

## 6. 收尾

- [ ] 6.1 确认三个基线文件为空，ESLint 契约与例外清单都已写入 `docs/frontend-architecture.md`
- [x] 6.2 knip 的 `exports`、`types` 升为 `error`（2.1 完成后即升级）
- [ ] 6.3 更新 US-L05 与 `docs/frontend-architecture.md` 中的存量说明；运行 `make check`、`make test-unit`、`make test-browser`、`make test-e2e` 与 Storybook 检查
