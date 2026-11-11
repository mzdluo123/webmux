# Tasks

## 1. 独立于会话的仓库与工作树代码上下文（`internal/codeview`、`internal/workspace`）

- [ ] 1.1 在 `internal/workspace` 实现按已登记 `repositoryId` 与可选 `worktreeId` 返回规范化 `commonDir`、工作树根路径及 `main`/`linked`/`bare` 状态的只读定位方法，并在 `internal/codeview` 定义 `TargetResolver` 接口与泛化 `scopeKey`
- [ ] 1.2 在 `cmd/webmux` 及测试夹具完成 `codeview` 与 `workspace` 的只读定位注入装配，运行 `go test -race ./internal/workspace ./internal/codeview ./internal/archtest` 验证分层与既有会话行为零回归
- [ ] 1.3 在 `internal/codeview` 实现零会话仓库级与工作树级上下文绑定、文件系统身份复核（根目录、Git 目录、`commonDir` 的 `os.SameFile` 校验）、跨上下文快照/游标隔离及独立于 `session.Manager` 的 TTL/LRU 回收
- [ ] 1.4 编写覆盖零会话、`bare` 仓库、删除最后一个会话、工作树目录替换与跨工作树快照越权读取的 `-race` 单测，运行 `go test -race ./internal/codeview` 验证通过
- [ ] 1.5 在 `internal/codeview` 实现基于完整十六进制提交 OID 校验的历史文件快照读取与双提交差异快照比较（含起止互换、重命名、二进制、`filter`/`working-tree-encoding` 降级及拒绝 `HEAD~1`/`--flag` 非法修订输入），运行 `go test -race ./internal/codeview` 验证通过且 `.git/index` 未被触碰

## 2. 按需分页提交历史与真实 parent 拓扑引擎（`internal/workspace`）

- [ ] 2.1 在 `internal/workspace` 实现仓库/工作树范围的分页历史检索与快捷范围解析（`all`、`branch`、`to-push`、`incoming`），支持关键字、作者、时间与字面路径筛选及未诞生 `HEAD`/无上游状态说明，运行 `go test -race ./internal/workspace -run TestHistory` 通过
- [ ] 2.2 在 `internal/workspace` 实现基于真实 `%P` 父提交列表的车道（lane）拓扑计算与提交详情/变更文件查询，覆盖作者与提交者区分、根提交空树对比、合并提交指定任一父节点对比，以及分页 `boundary` 边和筛选省略 `indirect` 边标记
- [ ] 2.3 补充多分支合并、分页截断衔接、路径筛选隐藏中间节点及自定义 `diff.external`/`textconv` 零执行断言单测，运行 `go test -race ./internal/workspace` 验证通过

## 3. 真实 `index` 代码块映射与并发提交结果识别（`internal/workspace`）

- [ ] 3.1 在 `internal/workspace` 引入按规范化工作树路径隔离的 `worktreeGate` 协调器，实现绑定服务端快照内容身份（对象 OID、工作区文件 SHA-256、文件模式）的文件级与普通文本代码块级 `Stage` / `Unstage`
- [ ] 3.2 编写实仓 `-race` 测试覆盖双代码块单块暂存、未诞生 `HEAD` 首次暂存与取消暂存、`CRLF`、无末尾换行、`100755` 模式、外部修改后快照过期拒绝，以及二进制/重命名/`filter`/冲突条目降级，运行 `go test -race ./internal/workspace -run TestStage` 通过
- [ ] 3.3 在 `internal/workspace` 实现完整真实 `index` 提交预览（`PreviewCommit`），汇总含终端已有暂存项的全部 `HEAD -> index` 变更、未暂存剩余项计数与作者身份配置
- [ ] 3.4 为 `PreviewCommit` 补齐前置阻塞检查（空暂存区、缺 `user.name`/`user.email`、游离 `HEAD`、冲突或进行中的 `merge`/`rebase`/`cherry-pick`/`revert`/`bisect`），运行 `go test -race ./internal/workspace -run TestPreviewCommit` 验证通过
- [ ] 3.5 在 `internal/workspace` 实现尊重钩子与签名的提交执行前置与锁处理（`ExecuteCommit`）：复查 `HEAD`/索引摘要并用临时索引副本计算 `expectedTreeOID`，对外部 `.git/index.lock` 报告仓库忙且绝不删除锁文件
- [ ] 3.6 为 `ExecuteCommit` 注入 `GIT_REFLOG_ACTION=webmux-commit:<operationId>` 与 `-c core.logAllRefUpdates=always`，定位本次提交 OID 并核对 `{oid, parents, tree}` 精确归类 `matched` / `needs_review` / `uncertain`
- [ ] 3.7 编写覆盖预览后 CLI 篡改失效、`pre-commit` 格式化重暂存（A05）、`post-commit` 修正及提交后紧随外部 CLI 并发提交（A06）的实仓 `-race` 测试，运行 `go test -race ./internal/workspace -run TestExecuteCommit` 验证通过

## 4. Git 工作台语义令牌、受控组件变体与响应式布局骨架（`web/`）

- [ ] 4.1 在 `web/src/design-tokens.css` 新增 Git 工作台明暗双主题语义令牌（中性表面、1px 分隔线、蓝色选中态与蓝色主动作），并在 `$lib/components/ui/button` 与 `$lib/components/ui/list-button` 扩展 `git-primary`、`git-row`、`git-row-action` 变体（保持 `rounded-sm`/`rounded-md` 刻度）
- [ ] 4.2 按需在 `web/eslint.config.mjs` 登记分栏布局钩子，同步更新 `web/stories/DesignSystem.svelte`、`docs/design-system.md` 与 `docs/frontend-architecture.md` 变体登记，运行 `pnpm --dir web check:arch` 验证零违规
- [ ] 4.3 新建 `web/src/git-workbench-types.ts` 与 `web/src/git-graph-layout.ts`，定义 Zod 契约、视图状态归约、拓扑车道与延续边几何计算，以及阅读/已审阅/已暂存三态分离模型
- [ ] 4.4 编写 `web/test/git-workbench.test.ts` 覆盖分支单击仅过滤、合并父节点切换、部分暂存分组与完整 `index` 预览状态，运行 `pnpm --dir web exec vitest run --config vitest.config.ts test/git-workbench.test.ts` 验证通过
- [ ] 4.5 在 `web/src/components/` 构建单文件均少于 500 行的桌面多栏（`≥1280px` 键盘/指针可调分隔）与窄屏单栏步进（`320–1023px`）骨架组件：`GitWorkbenchShell.svelte`、`GitLogPane.svelte`、`GitCommitPane.svelte`
- [ ] 4.6 在 `web/src/components/` 构建 `GitCommitPreviewDialog.svelte`、`GitBranchesPane.svelte`、`GitPushDialog.svelte`（含行尾 `…` 菜单、`16px` 移动端输入、不含未交付 V2/V3 假按钮），编写浏览器组件测试并通过 `pnpm --dir web test:browser`

## 5. Storybook 桌面/手机界面核对与全量门禁验收

- [ ] 5.1 在 `web/stories/` 新增交互评审故事（覆盖 `Log` 合并拓扑与筛选延续、`Commit` 逐块暂存与完整 `index` 预览、`Branches` 工作树占用引导、`Push` 宽对话框/手机全屏确认）
- [ ] 5.2 在评审故事中补齐无工作树、空仓库、锁占用与 `needs_review` 异常态场景，并登记到 `web/stories/Overflow.stories.ts` 与 `web/test/storybook/cases.ts`
- [ ] 5.3 运行 `pnpm --dir web test:storybook` 验证 6 种视口/大字号 × 明暗双主题无横向溢出且弹层操作可达，随后运行 `make format-check`、`make docs-check`、`make check` 与 `make test` 全量门禁确认全部通过
