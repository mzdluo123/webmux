# Snapcompact 参考实现与授权核对

本页记录 Go 核心的参考版本、允许复用的素材和对照范围。
开发者可先运行 `CGO_ENABLED=0 go test ./snapcompact`；实际 API 与使用入口见 [Go 核心](snapcompact.md)。
这些记录不构成模型可读性或成本的实测结论。

## 固定参考版本

- oh-my-pi：`7c49c11aa5bbaf3f418406583096168ad5b8c6dc`。
  [TypeScript 核心](https://github.com/can1357/oh-my-pi/blob/7c49c11aa5bbaf3f418406583096168ad5b8c6dc/packages/snapcompact/src/snapcompact.ts)。
  [Rust 渲染器](https://github.com/can1357/oh-my-pi/blob/7c49c11aa5bbaf3f418406583096168ad5b8c6dc/crates/pi-natives/src/snapcompact.rs)。
- Pig：`bcc926d30a328786cf4b6f5a38c923ec2a0f9f3c`。
  [Go 核心](https://github.com/ImAlexBlock/pig/tree/bcc926d30a328786cf4b6f5a38c923ec2a0f9f3c/internal/compact/snapcompact)。
- owncode：`372004a23425d0eee103160a1b43be5e7ad90e01`。
  [snapcompact.go](https://github.com/muratmirgun/owncode/blob/372004a23425d0eee103160a1b43be5e7ad90e01/internal/llm/agent/snapcompact.go)。
- Rust CLI：`2322ed130f9fee3e40c0e4acbc59afc3b1d83de8`。
  [源码](https://github.com/knoopx/snapcompact-cli/tree/2322ed130f9fee3e40c0e4acbc59afc3b1d83de8)。

## 代码授权与复用结论

上游根目录及 snapcompact 包声明 MIT，版权人为 Mario Zechner、Can Bölük、Stencil Labs, Inc.。
Go 核心采用独立实现；没有引入上游 TypeScript、Rust 运行时或复制 Pig 的源文件。
owncode 根目录 LICENSE 为 MIT，版权人为 Kujtim Hoxha（2025）；该实现与自己的 message 模型和归档流程耦合。
owncode 使用 basicfont，把超出 ASCII 的字符转义，并按每页 2200 估算图片 token；这些行为没有移植为通用策略。

在固定 Pig 版本的递归文件树中，未找到 LICENSE 或 COPYING，也没有足以确认整个实现授权的声明。
因此不复制其代码或 glyphs.bin；这不阻止采用独立实现。
重新检查 serialize.go，确认以下风险仍存在：

- `headTail`、单参数和总参数的 `s[:n]` 使用字节切片，可以切断 UTF-8。
- `formatArgs` 直接遍历 Go map，并在第八个键停止；顺序和保留的参数集合均不确定。
- `Serialize` 接受 `[]*store.Entry`，渲染还依赖内部 rtk，无法直接作为 provider-neutral 库使用。
- `collapse` 使用 `strings.Fields`，把代码缩进和行边界也折成空格。
- render.go 已有 rune 遍历、宽字符布局、ANSI 去除及 CR/tab 处理；这些改进不能修复先前序列化造成的数据损坏。

固定 CLI 版本没有根目录许可证文件，Cargo.toml 也未声明 license。
README 说明渲染流水线参考 MIT 上游、BDF 字体来自上游；这不足以确认 CLI 全部代码与打包字体的授权。
本任务没有复制该 CLI，也没有将其作为生产依赖或启动外部可执行文件。

## 字体与依赖

上游 8x13 BDF 文件头声明 `Public domain font. Share and enjoy.`。
上游 Silver.LICENSE 明确 Silver 字体采用 CC BY 4.0，由 Poppy Works / Wolfgang Wozniak 发布。
主要贡献者为 Itou Hiro（PixelMPlus）、leedheo（DOSGothic）、ぶち（Buchidot）。
分发 Silver 时须保留作者、许可链接、来源与改动说明；不能把 MIT 代码许可当作字体许可。
Go 核心没有分发 Silver；实际采用 Noto Sans CJK SC（SIL OFL 1.1），其声明随包分发，见 [字体声明](../snapcompact/fonts/NOTICE.txt)。
渲染依赖固定为 x/image v0.48.0、x/text v0.44.0（BSD-3-Clause）及 uniseg v0.4.7（MIT）。
完整依赖声明见 [第三方许可](../snapcompact/THIRD_PARTY_NOTICES.txt)。
字体覆盖范围不能证明复杂文字 shaping 或所有 emoji 支持；缺字结果必须由调用方检查。

## 固定夹具与对照范围

`snapcompact/testdata/conversation.json` 为本任务人工编写的脱敏合成会话，不来自用户日志。
它包含中文、组合字符、显式代码、CRLF、ANSI、tab、连续空行以及工具关联和状态。

`upstream-reference.json` 固定上游 `truncateForSummary` 的 ASCII 和 BMP 中文结果。
开发期从上述固定 TypeScript 文件提取该函数，移除参数/返回值类型标注后，用 Node 单独执行生成结果。
只执行了该纯函数；没有运行完整 Bun 包、原生 addon 或 Rust PNG 流水线。
对照测试比较固定首尾覆盖和省略范围，不要求有损标记文案或 PNG 像素一致。
Go 与上游均对首部份额四舍五入；BMP 中文夹具覆盖该取整规则。
UTF-16 代理对、结构保留、来源映射及缺字处理采用 Go 核心明确规则，不追求复制上游缺陷。

## 有意保留的差异与未验证项

- 上游默认参数值上限 500、工具结果及整次调用上限 2000，首部比例 60%；这些数值只是实验起点。
- 上游截断按 JavaScript UTF-16 长度，Go 使用 Unicode scalar 计量，并返回机器可读来源范围。
- 上游把换行折为 FULL BLOCK 标记、折叠符号并删除部分 emoji；Go 保留显式结构并显式报告无法渲染内容。
- 上游可合并相邻角色、工具调用及结果；Go 保留独立来源边界和原有顺序。
- 上游内置模型几何、token 费率和归档筛选；Go 通过注入规划，能力未知时明确返回未知。
- 本地测试只验证机械投影、PNG、资源上限和预算结果；未测实际模型链路、OCR、token 节省比例或任务成功率。
- 没有调用付费模型。后续交接服务须验证实际图片传输、目标模型和网关能力，才能启用图片投递。
