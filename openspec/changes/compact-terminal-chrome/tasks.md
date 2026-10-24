## 1. 终端顶栏

- [ ] 1.1 `Terminal.svelte`：连接文字只在非 `connected` 时渲染并加 `whitespace-nowrap`；状态点的 `aria-label` / `title` 合并会话状态与连接状态；会话信息未加载时保留连接文字
- [ ] 1.2 `Terminal.svelte`：全屏、上传、滚到底按钮加 `max-sm:hidden`；抽出共享的禁用条件 `$derived`
- [ ] 1.3 `Terminal.svelte`：终端菜单顶部新增仅窄屏显示的一组“全屏/退出全屏”“上传文件”“滚动到最新位置”菜单项及分隔线，复用原处理函数（滚到底在窄屏代码视图下仍切回终端）
- [ ] 1.4 确认快捷指令入口、草稿按钮与代码切换的相对位置和显示规则不变（`terminal-quick-commands`、`terminal-input-composer`）

## 2. 回到最新浮动按钮

- [ ] 2.1 `terminal.svelte.ts`：新增 `atBottom` 状态，在 xterm `onScroll` / `onWriteParsed` 中按 `viewportY >= baseY` 更新，并在返回对象中暴露
- [ ] 2.2 `Terminal.svelte`：终端区域右下角新增 `sm:hidden` 浮动按钮（tabler 图标、可访问名称、提示），条件为未在底部且终端视图可见；点击调用 `t.scrollToBottom()`，不抢焦点、不唤起键盘，不遮挡软键栏与草稿面板

## 3. 代码区工作区栏与标签栏

- [ ] 3.1 `CodePanel.svelte`：将导航开关、标签条（行内横向滚动）、刷新全部、独立窗口合并为一行，独立窗口加 `max-sm:hidden`
- [ ] 3.2 `CodeNavigator.svelte`：顶部显示代码根路径；将 `navStatus` 提示与“刷新导航”移入导航面板顶部；刷新全部按钮的提示包含根路径
- [ ] 3.3 `CodePanel.svelte`：标签行末尾新增 `sm:hidden` 的代码区菜单，菜单内容由激活的 `CodeDocumentView` 提供

## 4. 文档栏与视图切换

- [ ] 4.1 `CodeDocumentView.svelte`：用 `Tabs.List` / `Tabs.Trigger` 实现“文件 / Diff”“Diff / 旧侧 / 新侧”“预览 / 源码”视图控件，替换“完整文件”“查看 Diff”“旧侧完整文件”“新侧完整文件”和 SVG 按钮组，选中项由现有状态推导
- [ ] 4.2 `CodeDocumentView.svelte`：宽屏文档栏保持单行（路径末尾优先、视图控件、引用或预览来源），窄屏加 `max-sm:hidden`
- [ ] 4.3 `CodeDocumentView.svelte`：为代码区菜单提供完整路径标签、视图单选组、来源单选组、刷新文档、刷新全部
- [ ] 4.4 `CodeDiffReader.svelte`：窄屏只常驻统一/并排切换、增删统计和查找，行号、换行等开关以 `CheckboxItem` 进入代码区菜单，状态与宽屏工具栏同源

## 5. 操作栏与图片工具栏

- [ ] 5.1 `CodeDocumentView.svelte`：操作栏单行化；“引用整个文件”仅在存在行选择时渲染；宽屏左侧显示选择提示（`max-sm:hidden`）
- [ ] 5.2 `CodeReader.svelte`、`CodeDiffReader.svelte`：移除底部独立提示行，提示文字交给操作栏
- [ ] 5.3 `CodeImageView.svelte`：工具栏单行（信息省略、缩放比例、适应窗口、1:1）；放大/缩小在窄屏且粗指针时隐藏；下载仍在操作栏

## 6. 测试

- [ ] 6.1 更新 `web/test/e2e/` 与 `web/test/browser/` 中依赖已移除按钮名称或位置的用例（完整文件、查看 Diff、旧侧/新侧完整文件、上传文件、全屏、滚动到最新位置、SVG 预览/源码、引用整个文件）
- [ ] 6.2 E2E：320 px 与 393 px 视口下，终端视图和代码视图的顶栏单行、标题可见、无横向溢出；终端菜单的上传、全屏、滚到底菜单项可用
- [ ] 6.3 E2E：393×851 打开文本 Diff，代码内容区高度不少于视口 70%；通过代码区菜单切换旧侧视图与引用来源
- [ ] 6.4 浏览器测试：滚动离开底部时显示“回到最新”，点击后回到底部并隐藏；位于底部时不显示
- [ ] 6.5 浏览器测试：“引用整个文件”在无选择时不渲染，选择后出现，点击后清除选择
- [ ] 6.6 E2E：1440 px 视口下终端顶栏按钮集合不变；标签行与工作区操作同一行，文档栏单行
- [ ] 6.7 运行 `make check` 与 `make test`

## 7. 文档与交付

- [ ] 7.1 更新 `docs/user-stories.md`：US-D07（窄屏浮动回到最新）、US-D08（窄屏全屏入口在菜单中）、H 组新增窄屏紧凑顶栏故事、US-I01、US-I07、US-I08（视图切换形态）、US-I09（引用操作栏）、US-I15（窄屏图片工具栏），同步功能覆盖索引与依据链接
- [ ] 7.2 更新 `docs/design-system.md` 的紧凑工具栏规则（窄屏常驻操作上限、菜单收纳、单行工具栏）
- [ ] 7.3 PR 附 1440 px 与 393 px、明暗主题的改前/改后截图（终端视图、代码文件、Diff、图片）
- [ ] 7.4 真机检查 iOS Safari 与 Android Chrome：浮动按钮与软键盘、代码区菜单与图片双指缩放
- [ ] 7.5 归档顺序：先归档 `add-code-image-preview`，再归档本变更
