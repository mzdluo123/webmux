# 核对记录：2026-11-19

这些记录对应 WebMux 尺寸审计的智能检出、工作树容量和启动配置。验证使用隔离 Linux Chromium，未连接生产后端或使用真机。各条目的 PR 提供三端浅深前后截图和交互录屏。

## 智能检出恢复（T084，US-J28）

- `pnpm --dir web test:browser test/browser/GitWorkbenchLineSmart.test.ts` 通过，覆盖活跃会话确认和冲突解决入口。隔离 Chromium 核对回滚保留贮藏与冲突结果，复制了 238 字符恢复命令。

- [PR #782 的浏览器证据](https://github.com/mzdluo123/webmux/pull/782)。

## 工作树容量结果（T085，US-J02）

- `pnpm --dir web test:browser test/browser/WorktreeDetailsDialog.test.ts` 的 5 项通过。延迟的测量请求失败后可用键盘重试，下方分支与上游区的位置保持稳定。

- [PR #783 的浏览器证据](https://github.com/mzdluo123/webmux/pull/783)。

## 启动环境变量编辑（T088，US-B08、US-G07）

- `pnpm --dir web test:browser test/browser/LaunchConfigDialog.test.ts` 通过。添加和删除后焦点落到对应变量名或添加按钮，值继续默认遮挡，错误与对应输入关联。

- [PR #784 的浏览器证据](https://github.com/mzdluo123/webmux/pull/784)。

## 加载 shell 启动文件（T089，US-B08）

- `pnpm --dir web test:browser test/browser/LaunchConfigDialog.test.ts` 通过。切换 Bash 或 Zsh 包装时环境变量字段位置稳定，收起后 Tab 跳过选择框，按原样保存命令参数和环境变量。

- [PR #785 的浏览器证据](https://github.com/mzdluo123/webmux/pull/785)。

## 启动配置提醒范围（T090，US-B08）

- `pnpm --dir web test:browser test/browser/LaunchConfigDialog.test.ts` 通过。方向键连续切换全部提醒、仅需介入和关闭时依赖字段保留位置，阈值范围错误立即写入实时播报区。

- [PR #786 的浏览器证据](https://github.com/mzdluo123/webmux/pull/786)。
