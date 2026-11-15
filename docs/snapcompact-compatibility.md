# Snapcompact 官方用例兼容性

开发者可以运行 `CGO_ENABLED=0 go test ./snapcompact -run 'TestUpstream|TestPinnedUpstream'` 检查官方机械用例的 Go 移植。
本页区分原样复制的官方测试、语义移植、明确差异以及实际运行范围。
使用入口见 [Go 核心](snapcompact.md)，授权核对见 [参考版本](snapcompact-reference.md)。

## 官方测试来源

固定 `can1357/oh-my-pi@7c49c11aa5bbaf3f418406583096168ad5b8c6dc`。
2026-11-15 查询其 `main`，SHA 与固定版本相同，没有采用浮动最新测试。
完整 `packages/snapcompact/test/snapcompact.test.ts` 为 2134 行、17 个顶层测试分组、119 个静态 `it()` 声明。
[原样测试](../snapcompact/testdata/upstream/snapcompact.test.ts)及 [MIT 许可](../snapcompact/testdata/upstream/LICENSE)随代码保存。
[测试清单](../snapcompact/testdata/upstream/TEST_INVENTORY.txt)记录原文件行号、分组、名称和 SHA256。
Rust 渲染器内的 [测试模块](../snapcompact/testdata/upstream/snapcompact_render_tests.rs)也保留为开发期对照。
测试版权与来源见 [声明](../snapcompact/testdata/upstream/SOURCE.txt)。

## Go 用例映射

[预处理移植](../snapcompact/upstream_compat_test.go)使用官方相同输入，通过 provider-neutral DTO 调用 Go API。
同功能部分严格检查预期；序列化标记、来源边界和安全规则存在差异时，测试明确断言 Go 结果并写出官方结果。

| 官方分组或测试                                  | Go 测试                                                                                            | 对齐范围                                   |
| ----------------------------------------------- | -------------------------------------------------------------------------------------------------- | ------------------------------------------ |
| oversized tool results / configured budgets     | `TestUpstreamCompatibleToolResultBudgets`                                                          | 默认首尾、自定义限制、足够大限制和省略范围 |
| normalize                                       | `TestUpstreamCompatibleNormalizationCases`                                                         | 横向空白、Latin-1、ANSI、假名与韩文        |
| orphan result / content order                   | `TestUpstreamCompatibleOrphanResultAndOrder`                                                       | 孤立结果、原有顺序、调用关联               |
| oversized argument values / whole argument list | `TestUpstreamArgumentCasesWithDocumentedWholeObjectDifference`                                     | 相同输入，明确不同计量层级                 |
| symbols / newlines / emoji / NFKD               | `TestUpstreamNormalizationDeliberateDifferences`                                                   | 相同输入，记录结构与 Unicode 保留差异      |
| short final page                                | `TestUpstreamCompatibleShortFinalPage`                                                             | PNG 解码、尺寸和最小高度                   |
| Rust ASCII / CJK coverage                       | `TestUpstreamRenderPrintableASCIICoverage`、`TestUpstreamRenderCJKScriptCoverage`                  | ASCII 字形非空与官方三个 CJK 标量          |
| Rust black ink / bad shapes                     | `TestUpstreamRenderBWInk`、`TestUpstreamRenderRejectsBadShapes`                                    | 黑白墨色、非法几何拒绝                     |
| Rust height floor / overflow                    | `TestUpstreamRenderHeightHugsRowsWith64Floor`、`TestUpstreamRenderOverflowStaysBoundedAndReported` | 64px 下限、行高、溢出报告                  |

原有 [截断 helper 对照](../snapcompact/upstream_test.go)包含实际执行官方纯函数产生的固定 ASCII/BMP 中文结果。
首部预算已对齐官方 `Math.round` 的非负四舍五入规则，补充奇数预算与 0.5 边界。
默认工具输出和调用体上限为 2000、参数上限为 500、首部比例为 60%；这些并非经过模型评估的最优配置。

## 明确差异

- 官方 `toolArgMaxChars=500` 限制每个 JSON 编码参数值，`toolCallMaxChars=2000` 再限制整段 `name=value` 调用。
  Go 的 `ArgumentRunes=500` 限制整份排序后的 canonical JSON，`CallRunes=2000` 独立限制 CallBody；这两个策略不能宣称等价。
- 官方按 UTF-16 code unit 截断；Go 按 Unicode scalar 计量并避开组合簇及控制序列切口，返回原文半开区间。
- 官方角色使用 `¶user/¶ai/¶call` 紧凑标记，合并相邻同角色消息与配对工具结果，并可丢弃标为 useless 的调用。
  Go 保留独立来源记录与原有顺序，不采用语义有用性判断，关联由 CallID 保留。
- 官方把换行折成 FULL BLOCK，把部分符号、emoji 和兼容字符折为 ASCII，并删除部分格式和组合字符。
  Go 保留实际换行、显式代码/补丁/结构化输出和 Unicode；没有静默删除 emoji 后宣称完整覆盖。
- Go 采用 Noto 比例字体、grapheme 分段和单列排版；官方 bitmap 按 scalar cell grid，可使用两列 doc 布局及 Silver fallback。
- Go 不实现官方 sentence palette、dim marker、stretch、repeat 和 stopword dimming；字号不因预算自动缩小。
- Go 超页数时明确返回 `Uncovered` 和错误；官方部分渲染与归档路径只保留容量或预算允许的内容。
- 两者都有 64px 最后一页高度下限；具体 PNG 像素、字体度量、分页数量与调色板不保证一致。

## 排除与边界

以下官方分组未作为 Go 核心的等价 API 移植；保留原测试以便后续集成阶段评估。

- `computeFileLists`、`upsertFileOperations`、`isUrlSchemePath`：上游文件操作树和会话归档元数据，不属于本核心 DTO。
- `shape resolution`、`new shape variants`：上游 provider/model catalog、模型费率与 eval-winning 几何；Go 接受调用方能力和计费注入。
- `compact`、`archive helpers`、`frame data budget`：上游档案、文字边缘/图片中段选择、旧 summary 和投递状态；本任务仅生成与规划。
- `dimStopwords`、`wrap`、`doc layout`：官方颜色、cell-grid 与双列策略；Go 的比例字体分页采用独立测试验证覆盖。
- `data URL elision`：上游原生附件与 URL 自动省略策略；本核心接收调用方授权导出的文本，不引入额外附件内容判断。
- `scanRenderability`、`render`、`renderMany`：适用的字形、PNG、边界语义已移植；依赖具体 Silver/bitmap cmap、颜色或 provider detail 的断言不声称相同。
- agent/coding-agent/tui 下的 snapcompact 会话测试及研究目录模型评估：依赖其会话运行时、UI 或真实模型链路，不属于 provider-neutral 核心。

## 运行证据

Go 移植用例已在 `CGO_ENABLED=0` 下本地通过。
原样 TypeScript 套件依赖 `bun:test`、pi-catalog、pi-natives，不能直接用 Go 运行。
实际在隔离官方源码目录执行完整套件：Bun 1.4.4、官方发布的同版本 `pi-natives` 18.20.2 Linux x64 addon，未使用 mock。
结果为 **121 pass、0 fail、3947 expect 调用，4.69 秒**；参数化用例展开使执行数大于静态声明数。
首次尝试因缺失 native addon 在加载阶段失败；随后使用官方 npm 平台包重试成功，没有修改测试。
复现命令与哈希见 [运行证据](../snapcompact/testdata/upstream/official-run.txt)。
完整结果见 [官方运行日志](../snapcompact/testdata/upstream/official-suite.log)，首次失败见 [加载错误](../snapcompact/testdata/upstream/official-first-run.log)。
使用的是已发布 addon，未从固定 Rust 源码重新编译；这个结果证明官方测试环境可运行，不证明 Go 与其像素相同。
Bun、Node 和 native addon 均仅用于开发期，未加入 Go 核心生产要求。
尚未进行真实模型图片链路测试，也没有 token 节省比例、OCR 准确率或任务成功率结论。
