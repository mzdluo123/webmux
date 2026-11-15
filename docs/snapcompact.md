# Snapcompact Go 核心

贡献者可通过 `snapcompact` 包将统一历史转换为机械投影和 PNG，供后续解析器及交接服务调用。
当前没有用户界面或自动投递入口，不解析 Codex、Claude 或 omp 文件，也不创建 fork。

## 最短验证路径

在仓库根目录使用 `go.mod` 指定的 Go 工具链：

```sh
CGO_ENABLED=0 go test ./snapcompact
 go test ./snapcompact -run Example
 go test ./snapcompact -run '^$' -bench . -benchmem
```

[可运行示例](../snapcompact/example_test.go)展示投影与渲染。
[API 类型](../snapcompact/types.go)不依赖数据库、PTY、ACP 或具体 agent。
[参考审计](snapcompact-reference.md)列出固定上游版本、授权与差异。

## 调用流程

1. 构造 `Input`，将已授权导出的历史放入 `History`，当前独立任务放入 `CurrentTask`。
2. 调用 `Project(ctx, input, DefaultPreprocessConfig())` 获取投影；原文由调用方保留。
3. 调用 `NewRenderer`，再用 `Render` 获取 PNG、尺寸、页序和覆盖范围。
4. 由预算 API 检查目标能力及容量，调用方决定最终表示与投递。

调用方仍须遵守既有 reasoning、敏感信息和不可导出状态边界。
此 DTO 不提供存储或权限过滤，不应直接传入未经导出策略检查的原生文件。

## 预处理与来源

记录顺序由切片确定，稳定 ID 用于精确追溯，工具调用和结果通过 `CallID` 关联。
`Kind` 显式指定 prose、code、patch 或 structured；未知类型采用结构保留规则。
`Role=tool` 或带 `ToolName` 的记录正文按工具输出限制；其他正文仅受输入字节上限约束。
工具输出、JSON 参数、调用体分别设置上限，单位为 Unicode scalar（Go rune），不是字节或 token。
默认输出及调用体 2000 rune、整份参数 JSON 500 rune、60% 首部仅为实验起点，不是实测最优值。
首部数量使用非负 half-up 取整，与固定上游 Math.round 一致；Unicode 和 ANSI 安全边界可能进一步缩短保留区。
首尾保留产生机器可读省略范围；参数的来源坐标指稳定 JSON 编码。

规范化删除 ANSI、统一换行和 tab、折叠重复空白及空行。
显式结构保留缩进和行边界；规范化后的投影是有损表示，不能替代原文。
`Sources` 指向原文字段范围，不声称规范化字符与原文字节逐字对应。
渲染范围使用 `Field=projection`，其 rune 坐标指向 `Segment.Text`；两种坐标不可混用。
当前任务原样保留为独立文本，不参与历史截断或折叠，也不应转成历史图片。

相同输入与配置得到同一投影缓存键；渲染键另包含几何和字体身份。
缓存只节省本地生成与存储，不代表 provider 减免重复计费。

## 字体与资源边界

字体随包分发并附许可证和固定来源声明，生产仅使用 Go，不启动外部渲染程序。
支持 ASCII、中文及字体覆盖的 Unicode；按 grapheme 分片，避免在组合序列内换页。
不提供通用复杂文字 shaping 或彩色 emoji 保证；缺字以结构化结果和错误报告。
遇到缺字应改用原文文本，不把替代方框视为成功覆盖；当前构造器固定使用随包字体。

字号、行距、尺寸、页数、像素和内存均有边界，不通过缩小字号塞入无限历史。
最后一页至少达到 `MinHeight`；调用方须依据真实目标接口配置宽度及最小高度。
资源不足或取消不能视为完整覆盖，必须检查错误及返回的未覆盖范围。
输入超过硬字节/记录上限时，不按被拒绝输入分配范围数组；此时无页、无范围，调用方应将整份输入视为未覆盖。

## 成本与证据

预算分别记录文字 token、图片 token、图片数量、编码请求字节和上下文容量。
文字估算、图片计费与目标能力由调用方注入；未知能力不会伪装成已测量数据。
PNG 字节数只用于传输与存储，不用于推算 token 节省。
规划区分原文、投影、图片和引用，不执行模型调用或可靠投递。

固定夹具与本地基准只证明机械行为和本机性能。
没有付费模型调用，没有真实链路 OCR、任务成功率或 token 节省比例的证据。
阶段 3 的模型链路、会话解析、交接 UI 和 fork 生命周期仍由 #640 后续工作完成。

## 明确的预处理限制

`MaxInputBytes` 允许 1 到 64 MiB，记录数不超过 100000，JSON 参数嵌套不超过 128 层。
参数仅接受 JSON 数据类型，不接受自定义 `MarshalJSON` 对象。
字段上限为零表示保留零字符；tab 固定转换为四个空格，连续空行压到一行。
省略范围仅记录长度截断；规范化损失不逐字符记录，需通过原文引用精确读取。
截断对组合标记、ZWJ、emoji modifier 和旗帜边界采用保守保护，尚不声称完整 UAX grapheme 兼容；渲染换行使用 uniseg。

## 本地性能基线

2026-11-15，在 Linux amd64、Xeon Platinum 8573C、Go 1.27.4、4 vCPU 配额下单独运行基准（无其他构建或测试）。
固定输入为 `benchmarkHistory` 的 4 条合成记录，包含 300 行中英日志、工具参数和代码；使用默认机械预处理及渲染配置。
预处理会截断日志；图片覆盖的是同一投影，不是完整原文。
三轮 `-benchtime=1s -count=3` 的中位数如下：

| 操作    | 耗时       | 分配字节/次 | 分配次数/次 | 产物                     |
| ------- | ---------- | ----------- | ----------- | ------------------------ |
| Project | 0.195 ms   | 104011      | 135         | 确定性文本投影           |
| Render  | 108.358 ms | 13928566    | 506         | 3 页，共 476795 PNG 字节 |

渲染基准排除 `NewRenderer` 字体解析，包含每次 face 创建、布局与 PNG 编码。
整个预编译基准进程峰值 RSS 为 39928 KiB（Linux `getrusage(RUSAGE_CHILDREN)`），并非单次操作内存；`B/op` 是累计分配量。
原始记录与源文件哈希见[基线数据](../snapcompact/testdata/performance-baseline.txt)。
这些是单机本地结果，不代表目标模型传输、OCR 或 token 计费收益。
