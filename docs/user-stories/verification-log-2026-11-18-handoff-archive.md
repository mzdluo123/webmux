# 2026-11-18：混合压缩归档与手机预算编辑（US-Q19、US-Q20、US-K04）

- 依赖固定为 Snapcompact `969040513d9e`：按模型选择点阵布局，保留首尾文本，中段分级密度；文本、图片与省略范围精确划分投影正文。
- 预览冻结文本与图片，ACP 按首部、图片、尾部与任务发送；图片估计计入上下文，附件字节独立限制。无法容纳任何图片时阻止发送截断归档。
- 手机页数保留字符串草稿；清空、逐字输入、失焦校验及无效/变更草稿阻止旧预览发送均已覆盖。
- 独立模块：race、CGO-disabled 测试、vet、Darwin arm64/Windows amd64 cross-build 通过；依赖 PR 的 Go CI 通过。
- WebMux：`go test -race ./internal/handoff ./internal/historyprojection ./internal/hybrid` 对应包通过；`TestWireSnapshots -update` 完成。
- `golangci-lint run ./internal/handoff ./internal/historyprojection ./internal/hybrid ./internal/contract` 为 0 issues；`make docs-check` 通过。
- 前端 build、test 类型检查、lint、`check:arch` 通过；架构测试 49 项。
- handoff E2E 首轮 6 项通过，4 项因合成长记录超出默认 4000 字节原文页而未看到助手记录失败；夹具显式请求 16000 字节后，桌面/手机对应 4 项全部通过。
- [真实模型实验](../native-image-experiment.md#2026-11-18上游式混合归档)：约 10 万投影字符，工具/代码输入节省 40.6%，长段落节省 15.4%；各表示 13/13 字段正确，无预算省略或补查。
- 隔离嵌入应用实际操作：手机页数 `8 → 空 → 3` 可生成预览；`12` 失焦显示范围错误，改为 `3` 恢复。截图均为合成数据。
- 视觉覆盖 1280×900、768×1024、390×844 的浅深色；未做 iOS/Android 真机键盘、真实粗指针或悬停验证。

| 视口与主题 | 修改前                                                   | 修改后                                                  |
| ---------- | -------------------------------------------------------- | ------------------------------------------------------- |
| 桌面浅色   | [前](../images/handoff/archive-before-desktop-light.png) | [后](../images/handoff/archive-after-desktop-light.png) |
| 桌面深色   | [前](../images/handoff/archive-before-desktop-dark.png)  | [后](../images/handoff/archive-after-desktop-dark.png)  |
| 平板浅色   | [前](../images/handoff/archive-before-tablet-light.png)  | [后](../images/handoff/archive-after-tablet-light.png)  |
| 平板深色   | [前](../images/handoff/archive-before-tablet-dark.png)   | [后](../images/handoff/archive-after-tablet-dark.png)   |
| 手机浅色   | [前](../images/handoff/archive-before-phone-light.png)   | [后](../images/handoff/archive-after-phone-light.png)   |
| 手机深色   | [前](../images/handoff/archive-before-phone-dark.png)    | [后](../images/handoff/archive-after-phone-dark.png)    |

![390×844 深色手机：清空预算、校验无效值并重新输入](../images/handoff/archive-mobile-budget.gif)

## 2026-11-18：OMP 的 Go 移植（US-Q20、US-K04）

- Go 模块的 `go test ./...` 和 `go test -race ./...` 通过。OMP 18.22.2 固定样本覆盖 17 种布局的解码像素，以及短历史、完整与截断归档的摘要、文本边缘、顺序、计费估计和解码像素。
- `go run ./cmd/native-image-experiment -scenario tools -model openai-codex/gpt-6-astra -history-cap 64000 OUTPUT_DIRECTORY` 生成六张图，保留 103395 字符，估计 15704 图片 token。默认预算下报告 38470 个省略字符。其他结果见[离线对照](../native-image-experiment.md#go-移植与原版一致性)。
- `go test ./internal/api -run TestWireSnapshots -update` 更新默认压缩预览快照。交接发送继续使用预览冻结的图片字节及顺序，原文补查从相同快照读取。
