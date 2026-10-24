# Frontend Architecture Guard

前端架构守卫把 [设计系统](design-system.md) 中“复用组件、使用令牌、页面只控制布局”的约定变成可执行的检查，使人和 coding agent 在提交前得到具体的修复提示。所有命令在仓库根目录运行。

```sh
pnpm --dir web check:arch    # 全部架构检查；make check 与 CI 也会运行
```

## 写 UI 之前：先找，再写

按顺序查找，找到就复用，不要重新实现：

1. `web/src/lib/components/ui/`：shadcn-svelte 基础组件（Button、Input、Dialog、DropdownMenu 等）。
2. `web/src/components/`：业务复合组件（ResponsiveDialog、InlineNotice、SessionStatus 等）。
3. shadcn-svelte registry：`pnpm --dir web exec shadcn-svelte add <name>`，生成后审查本地差异。

三处都没有，才新建组件。需要新外观时，在 `ui/` 对应组件的 `tv()` 中增加 variant 或 size，而不是在使用处用 class 覆盖。

## 分层

源码目录保持扁平，不划分 `features/`。dependency-cruiser 按下表检查导入方向：

| 层           | 路径                       | 可以依赖                         |
| ------------ | -------------------------- | -------------------------------- |
| 设计系统叶子 | `src/lib/components/ui/`   | `src/lib/`、第三方包             |
| 工具         | `src/lib/`                 | 不依赖业务组件、hooks 与页面     |
| 领域逻辑     | `src/*.ts`（除 `main.ts`） | 不依赖 `.svelte` 组件            |
| Hooks        | `src/hooks/`               | 领域逻辑；不依赖 `.svelte` 组件  |
| 业务组件     | `src/components/`          | 以上各层；不依赖页面             |
| 页面         | `src/pages/`               | 以上各层；只由 `App.svelte` 组合 |

`bits-ui`、`vaul-svelte`、`paneforge` 等 headless 原语与 `tailwind-variants` 只在 `ui/` 中使用。模块间不允许循环依赖（shadcn-svelte 生成的 barrel 自引用除外），未被引用的模块应删除。

## 检查项

| 命令          | 工具                    | 检查内容                                                                                                                                          |
| ------------- | ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| `arch:lint`   | ESLint + `@shadcn/lint` | 用 class 覆盖组件外观、原始调色板颜色、任意值、内联样式、未知类名、动态类名；手写原生控件；在 `ui/` 外导入原语、变体工具或 `@tabler/icons-svelte` |
| `arch:deps`   | dependency-cruiser      | 上节的分层、循环依赖与孤立模块                                                                                                                    |
| `arch:dupes`  | jscpd                   | 新增的复制粘贴代码块                                                                                                                              |
| `arch:unused` | knip                    | 未使用的文件、依赖与重复导出；未使用的导出和类型暂为警告                                                                                          |
| `arch:test`   | Vitest                  | 同名业务组件、与 `ui/` 组件同名的业务组件、6 个以上外观类在 3 个以上文件中重复                                                                    |

Oxlint（`pnpm --dir web lint`）仍是通用 lint。它只读取 `.svelte` 的 `<script>`，所以模板相关规则由只加载架构规则的 ESLint 执行。

`no-restyle` 只允许使用处调整布局（外边距、宽度、栅格与 flex 位置等）。例外由 `web/eslint.config.mjs` 中的组件契约声明：`CardTitle` 可调整字号但不能改字重，`CardContent` 可调整内边距。新增契约需要在 PR 中说明设计理由。

## 存量基线与收紧

存量违规记录在基线文件中，检查只拦截新增问题：

| 文件                                            | 记录内容                     |
| ----------------------------------------------- | ---------------------------- |
| `web/eslint-suppressions.json`                  | 每个文件、每条规则的违规数量 |
| `web/.dependency-cruiser-known-violations.json` | 已知的分层违规               |
| `web/.jscpd-baseline.json`                      | 已知的重复代码块指纹         |

- 修复存量问题后运行 `pnpm --dir web arch:ratchet`，它只删除已修复的记录并格式化基线文件，然后提交基线变化。
- ESLint 按“文件 + 规则”计数：同一文件同一规则的违规数超过记录时，该文件该规则的所有违规都会报告。
- 不要为了通过检查而扩大基线、添加 `eslint-disable` 或 `jscpd:ignore`。确有必要的例外，在 PR 中说明原因，并优先改为配置中的契约或白名单。

存量治理的范围与顺序见 OpenSpec 变更 `frontend-architecture-guard`。
