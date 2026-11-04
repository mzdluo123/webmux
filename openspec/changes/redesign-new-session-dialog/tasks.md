## 1. 默认值解析（纯逻辑）

- [ ] 1.1 新增 `web/src/newSessionDefaults.ts` 的 `resolveInitialTarget`：实现“上下文入口 > 主页选中仓库主工作区/其他可用非 bare 工作树 > 最近目录 > 服务默认目录”，选中仓库无可用目标返回 `unset` 并保留仓库；用 Vitest 覆盖全部仓库/直接会话视图、单仓库视图最近目录属于别的项目、无可用工作树、上下文入口覆盖选中仓库，运行 `pnpm --dir web exec vitest run --config vitest.config.ts test/newSessionDefaults.test.ts`。
- [ ] 1.2 实现 `resolveInitialLaunch` 与 `summarize`：记忆有效则采用（含混合起始端与自定义命令）；无记忆取第一个配置，无配置取 custom；记忆的配置已删除得到 `unset`；加载中不产生空 `configId`；用 Vitest 覆盖上述各分支与摘要文案，运行同一测试文件。

## 2. 对话框组件重构

- [ ] 2.1 新增 `LocationPicker.svelte`：位置分组（上下文仓库工作树、最近目录、其他仓库、输入或浏览目录、新建工作树…）、不可用/bare 条目禁用、无仓库时不显示新建工作树；桌面用 Popover+Command，窄屏用就地展开，复用 `DirectoryPicker` 与 mkdir 确认；用浏览器测试验证选择、搜索、Esc 只关选择器、键盘操作，运行 `pnpm --dir web exec vitest run --config vitest.browser.config.ts test/browser/LocationPicker.test.ts`。
- [ ] 2.2 新增 `LaunchPicker.svelte`：启动配置与末尾“自定义命令…”同列，显示图标/名称/环境变量数量；自定义时才显示命令输入；切换保留已选配置与已输入命令；混合配置内联“终端 / 聊天”起始端与连接器缺失提示；空状态链接设置页、存储不可用显示原因；用浏览器测试覆盖这些状态。
- [ ] 2.3 新增 `NewWorktreeFields.svelte` 包装 `WorktreeCreateFields`：展示仓库、新分支名、起始分支与“使用已有分支”切换，折叠摘要“新分支 ← 起始分支”，起始提交与存放路径放入“高级”；保持 `isReady()` 契约与上游刷新行为，运行 `pnpm --dir web exec vitest run --config vitest.browser.config.ts test/browser/WorktreeCreateFields.test.ts` 确认不回归。
- [ ] 2.4 以 `NewSessionDialog.svelte` 取代 `NewSessionSheet.svelte`：持有 target/launch/name/error/busy，汇总行与主按钮禁用规则，提交请求体保持 `cwd|workspace` 与 `configId|cmd` 互斥；仅在成功后更新 `recentDirs`、`lastLaunch`、`customCommand`、`hybridStartSide`；保留“创建后仅启动会话不重复建工作树”、mkdir 确认与连接器缺失流程；移除旧文件并更新全部 import，无别名。
- [ ] 2.5 实现“仅新建工作树”与分屏变体：仅新建工作树预选“新建工作树…”、隐藏命令行并提供默认关闭的“创建后启动会话”，关闭时不要求有效命令；分屏提供方向选择、预填被分屏会话目录或工作树、主按钮“创建并加入分屏”；用浏览器测试覆盖两种变体及请求体。

## 3. 调用点与联动

- [ ] 3.1 更新 `web/src/pages/SessionList.svelte`：向对话框传 `selectedRepositoryId`，保留上下文入口参数，移除页面内自行推导预选的逻辑；确认对话框不改变主页选中仓库；用浏览器测试或页面测试覆盖单仓库视图默认主工作区与对话框内改选不回写。
- [ ] 3.2 更新 `web/src/pages/Terminal.svelte` 的分屏调用，随新对话框 props 调整，并核对 `onworktreecreated` 的仓库状态更新保持不变。

## 4. 测试迁移

- [ ] 4.1 重写 `web/test/browser/NewSessionSheet.test.ts` 为对话框测试：覆盖默认值（主页选中仓库、最近一次启动方案、失效记忆）、提交请求体互斥、提交失败保留输入、创建失败不覆盖记忆；运行 `pnpm --dir web exec vitest run --config vitest.browser.config.ts test/browser/NewSessionDialog.test.ts`。
- [ ] 4.2 更新 `web/test/e2e/fixtures.ts` 增加选择位置与命令的统一助手，并迁移 `session-creation`、`launch-configs`、`hybrid`、`worktrees`、`lifecycle`、`page-stability` 中依赖“预先输入/手动填写/直接目录/已有工作树/新工作树”文案的用例；逐个运行 `pnpm --dir web exec playwright test <spec>`，不连接已有开发服务。

## 5. 文档与验收

- [ ] 5.1 更新 `docs/user-stories/access-and-sessions.md` 的 US-B01、US-B03、US-B06、US-B08 及 `docs/usage.md` 中新建会话的描述；保持故事编号与索引计数一致，在交付说明中列出受影响故事 ID，运行 `make docs-check`。
- [ ] 5.2 运行 `make check`、`pnpm --dir web check:arch` 与 `make test-fast`，修复本变更导致的失败，不新增 lint 豁免或架构基线。
- [ ] 5.3 在隔离演示数据上于桌面/移动、浅色/深色验证对话框：单仓库视图默认主工作区、最近启动方案填入、新工作树折叠摘要、分屏变体、键盘与 Esc 分层；留存可打开的 before/after 截图 URL 用于 PR，并确认没有遗留临时脚手架。
