# 隔离 ACP 图片传输实测

贡献者可用本页的独立实验，确认 WebMux 的真实 ACP 客户端能把合成历史图片交给 `omp acp` 和实际模型读取。
本次三图输入正确读取全部七项；混合表示出现一次缩进错误，且图片成本高于同覆盖文本。不要据此默认启用图片。
实验不修改生产历史路径，也不代表 WebMux 网页上传、聊天 REST 或原生历史自动注入已端到端验收。

## 复现

需要已正常认证的 omp、仓库 Go 工具链，以及[合成样本生成器](../cmd/native-image-experiment/main.go)。
实验复用已集成的独立 snapcompact-go 核心，不重新移植渲染器。
运行会向真实模型发送合成数据并消耗额度或产生费用。

```sh
sample=$(mktemp -d /tmp/webmux-image-sample.XXXXXX)
evidence=$(mktemp -d /tmp/webmux-image-transport.XXXXXX)
go run ./cmd/native-image-experiment "$sample"
go run ./cmd/native-image-transport \
  -directory "$sample" -output "$evidence" \
  -model openai-codex/gpt-6.1-sol \
  -modes raw,projected,image,hybrid -timeout 100s
```

[实验命令](../cmd/native-image-transport/main.go)直接使用 WebMux 的 `internal/acp.NewConn`、`Initialize`、`NewSession`、`ImageBlock` 和 `Prompt`，不是 fake agent 或模拟响应。
图片以 PNG 的 base64 ACP image 块发送，不是文件路径占位或让模型调用工具读取文件。
每个表示使用新进程、新 session、新临时 cwd、session-dir 和空的临时配置 overlay。
通过私有 stdio 连接，不开启 HTTP listener，因此没有需要分配的 loopback 端口，也不连接现有开发服务。

- 禁用工具、扩展、skills、rules、上下文文件、LSP、标题、会话持久化及辅助服务的跨 provider 使用；handler 拒绝工具授权和表单请求。
- CLI 正常使用已配置认证；实验程序不读取、复制或修改认证文件，不登录、不恢复私人会话。空 overlay 不等于清除 CLI 的已有 provider 配置。
- 每次软成本上限为 USD 1，硬进程时限为 100 秒；已发出的请求可能超过软费用门槛。
- 临时 cwd/config/session-dir 随每次请求删除；原始 ACP stdout、stderr 和报告以 `0600` 保留在 evidence 目录。分享前检查原始输出。
- 只发送合成样本，不发送项目文件、真实历史或环境转储。生产代码、默认图片策略和历史导入均未改动。

## 本次实际结果

实测日期为 2026-11-15 UTC（北京时间 11 月 16 日），macOS arm64、omp `18.20.2`。
连接器初始化声明 `promptCapabilities.image=true`；新 session 的 model config 为 `openai-codex/gpt-6.1-sol`，thinking 为 off。
复用 `/tmp/webmux-image-final.V7fZ8K` 的同一组合成样本、原始 PNG 和固定七字段任务，没有重新渲染或更换评分任务。

| 表示 | 输入 token | 输出 token | 总 token | 估价 USD | 耗时秒 | 正确字段 |
| --- | --- | --- | --- | --- | --- | --- |
| raw | 5487 | 77 | 5564 | 0.011744 | 5.128 | 7/7 |
| projected | 2700 | 76 | 2776 | 0.006160 | 5.122 | 7/7 |
| image（三图） | 4602 | 153 | 4755 | 0.010734 | 16.306 | 7/7 |
| hybrid（两图与修正文） | 4207 | 128 | 4335 | 0.009694 | 11.403 | 6/7 |

输入/输出/总 token 来自实际 `session/prompt` JSON-RPC result 的 `usage` 扩展，而非 PNG 大小、本地 tokenizer 或初始零 usage。
WebMux 标准 `PromptResponse` 当前不暴露该扩展，因此实验另从原始 stdout 提取；`usage_update.cost` 是连接器估价，不是账单。
耗时包含连接器启动、初始化、新 session 和真实推理，不含 Go 编译；每个表示只有一轮回答，无工具调用、无补查，无失败后重试。

image 在用户文本只有固定任务的情况下，正确读出图片独有的中文、代码条件、八空格缩进、完整 Shell 标点、日志尾标记及最新端口。
hybrid 的中文、标点、端口等正确，但把首个 return 的八空格读成四空格；保留此失败，不用重新请求掩盖。
raw 正确读出中段 `JADE-120`；其他表示正确报告 `NEED_SOURCE`。这不是已完成原文补查，也不表示图片可无损恢复被机械截断的内容。

[精简测量 JSON](../cmd/native-image-transport/measurements.json)保存答案、逐字段评分、session ID、图片 SHA-256、token、费用和延迟。
原始事件与报告保留于 `/tmp/webmux-image-transport.pWqPX8`，未提交 PNG/base64 或完整模型列表。

## 已测边界与尚未验证的部分

三张 image PNG 分别为 209244、249908、23470 字节，尺寸为 1024×1012、1024×1012、1024×116。
两张 hybrid PNG 为 209244、244904 字节；它们是既有生成物，未在 ACP 实验中做预转换。
没有抓取网关请求，不能断言 omp、provider 或模型内部未缩放、转码或采用不同 detail；不能把本次尺寸当作支持上限。

WebMux 当前附件代码限制为每条 8 个、单个 10 MiB、合计原始数据 20 MiB、编码 prompt 30 MiB；ACP 单行上限为 32 MiB。
这些是代码边界，不是本实验对上传接口、边界值或网关最大图数/像素/请求体的实测。
本实验选择允许的隔离 ACP 客户端路径，没有运行 WebMux HTTP 服务、上传 API、聊天 API、浏览器或生产历史自动交接。
本次证据只能确认“WebMux ACP 客户端 → 真实 omp ACP → 所选实际模型”的合成图片读取，不能扩写为整个产品路径已支持。

前一实验的 Codex `401 token_revoked` 已复现，本次未重试；解除条件仍是账号所有者修复正常 CLI 认证。
其他连接器、最大图片数、detail、网关限额与最终收费未知；当前组合也未显示相对投影文本的成本优势，继续文本默认。
本页与[CLI 实验](native-image-experiment.md)共同记录 #640/#638 的图片候选证据，不改变文本默认策略。
