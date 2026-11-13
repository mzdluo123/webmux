# Git 工作台与仓库级接口（US-J19–US-J28）

本组收录独立 Git 工作台与仓库级接口的用户故事（US-J19–US-J28），按基础闭环与进阶协同/日常高级操作分为两份子文档：

- [Git 工作台基础闭环与账本（US-J19–US-J23）](git-workbench/core.md)：
  - **US-J19**：零活跃会话可用的仓库与工作树 Git 只读、真实 `index` 暂存与两阶段提交复核 REST 接口；
  - **US-J20**：独立 `/git` 路由（`view=log|commit|branches|stash`）、仓库/工作树/会话代码区三入口联动、只读门禁与跨视图状态隔离；
  - **US-J21**：多分支 `%P` 拓扑分页历史图、引用筛选与检出解耦、合并父节点切换、任意两版本比较及 Pierre `CodeDiffReader` 阅读；
  - **US-J22**：`Changes` 四组树与阅读/审阅/暂存三态分离、常驻逐块暂存、全量提交预览、三处共享分支动作及 ~900px 宽幅双栏推送确认框；
  - **US-J23**：有界持久 Git 操作账本（`git-operations.json`）、`clientRequestId` 重连幂等重放、服务启动真实引用/reflog 只读对账与底部状态栏。
- [任务基线、协同、冲突解决与日常高级操作（US-J24–US-J28）](git-workbench/collab-and-advanced.md)：
  - **US-J24**：工作树固定任务审阅基线（`git-task-baselines.json`）、单份净变更审阅（含干净工作区）与任务交付收尾摘要；
  - **US-J25**：绑定内容身份的跨设备审阅进度同步、`baseRevision` 乐观锁提交草稿及返工意见草稿箱安全交回指定会话；
  - **US-J26**：合并/变基/遴选/还原两阶段方向预览、无状态重进恢复横幅、诚实 `Abort` 预览与内置三方冲突解决器；
  - **US-J27**：独立 `Stash` 视图与安全两步 `Pop`（冲突绝不丢弃贮藏）、受已推送保护的 `Amend`/作者覆盖/`Sign-off`、本地分支重命名/上游/标签/远程维护及单文件 `--follow` 历史与逐行 `Blame` 溯源；
  - **US-J28**：任意逐行（Sub-hunk）暂存/取消暂存、防陈旧覆盖与恢复快照保护的 `丢弃更改` 及基于可追踪 Stash 的三阶段 `Smart Checkout`。

关联章节：

- [仓库登记、工作树生命周期与分支维护（US-J01–US-J18）](lifecycle.md)
- 总索引：[仓库登记与 worktree 管理](../worktrees.md)
- 功能覆盖核对：[功能覆盖核对与后续维护](../../coverage.md)
