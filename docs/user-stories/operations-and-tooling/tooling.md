# 开发、验证与发布支持

## L. 开发、验证与发布支持

本节描述贡献者使用的工程能力，不增加面向终端使用者的产品承诺。

- **US-L01 本地开发与构建**：作为贡献者，我希望使用统一命令安装锁定依赖、运行前端和构建内嵌程序，以便复现开发与发布环境。
  验收要点：
  - CI 和发布构建使用 `go.mod` 固定的 Go 1.27.4 工具链；
  - 本地与 CI 使用 `.nvmrc` 固定的 Node.js 26.13.0；
  - 本地通过官方独立脚本安装 pnpm 12.18.0，CI 使用 pnpm/action-setup；
  - 使用 pnpm frozen lockfile，安装项目本地的 `vite-plus` 1.6.0，不要求全局安装 `vp`；
  - 构建使用 `pnpm --dir web exec vp build`；`pnpm --dir web build` 先运行拒绝 warning 的 `svelte-check --tsrs` 再构建；
  - Vite+ 开发服务器代理 API/WebSocket 到独立后端并需配置开发 Origin；
  - `make web/build/install` 提供检查、打包与安装；
  - CI 将 pnpm store、Go 模块与 Go 构建/测试缓存分开，构建缓存使用按周滚动且随模块文件变化的 key 在 `main` 刷新。
- **US-L02 分层验证与诊断**：作为贡献者，我希望运行 Go race、前端单元、浏览器与隔离 E2E 测试，以便验证会话、安全和交互行为。
  验收要点：
  - agent 本地验证必须并发启动互不依赖的检查并收集所有退出结果，优先使用已有并行 Make 入口，详见 [本地验证规则](../../agent-rules/testing.md#agent-local-validation)；
  - 共享产物先构建再消费，性能测试独占执行；因资源竞争降低并发或串行运行时说明具体原因，已通过的检查仅在后续变更、失败或未解决问题影响其结论时重跑；
  - 提供 `make test` 完整门禁与 `make test-fast` 快速入口、类型/格式/lint 检查；
  - Vitest 单元、浏览器与架构测试通过项目本地 `pnpm --dir web exec vp test run` 运行，保留各套件配置与覆盖率合并；Storybook、Playwright 与性能测试保持独立入口；
  - `pnpm --dir web exec vp fmt` 与 `pnpm --dir web exec vp lint` 统一格式化和通用 lint，保留格式规则、类型感知 Promise 规则与 warning 门禁；
  - 提供 `make cover`（`cover-go`、`cover-web`）与 CI 覆盖率报告：Go race 采集 atomic 语句覆盖率（排除测试守卫与假 agent 包）并输出包级摘要与 HTML，前端合并 Vitest Node 单元与 Chromium 浏览器组件测试的 V8 覆盖率，写入 `$GITHUB_STEP_SUMMARY` 并上传报告产物；
  - `make test` 一次构建前端与 E2E 程序供 E2E/性能复用，本地先并行运行 Go 与 Vitest，再并行运行 Storybook 与 E2E，性能套件随后单独运行；`make test-fast` 并行运行 Go 与 Vitest；
  - 本地 Vitest 浏览器、E2E、Storybook 按可用并行度分别最多 4、8、6 个 worker；CI 的 E2E 与 Storybook 均为 2 个，为长对话布局和滚动检查保留 CPU 余量；`WEBMUX_E2E_WORKERS` 可覆盖 E2E，Go 默认 `-parallel=4`（`GO_TEST_PARALLEL` 可调）；
  - Go 采用 Uber Style Guide 与登记的仓库例外，`make check` 和 CI 在前端构建后执行 `go vet ./...`（CI 另以 `GOOS=darwin` 检查 macOS 构建）及固定 v2.15.0 的 golangci-lint（errcheck/govet/ineffassign/staticcheck/unused），生产零告警，测试仅冻结 errcheck，新增代码零新增告警，豁免只能收紧；
  - CI 先由 `Classify changes` job 按路径裁剪：仅文档变更只跑格式、文档、持久化与 agent/web 静态守卫，仅 Go 变更跳过前端套件，仅前端变更跳过 Go race 与 Windows 交叉编译，未知路径和工具/依赖变更全量运行；Go wire 快照同时触发前端 Zod 契约测试，分类脚本有独立路径用例；
  - 每次推送到 `main` 都强制完整运行（跳过分类），供发版复用；GitHub 并发组只保留一个待运行，密集合并仍可能取消中间提交的待运行，该提交发版时回退完整重跑；
  - 独立的 `E2E runtime` job 按冻结锁文件统一安装并打包 Playwright 与能力契约校验所需的 Zod；
  - Makefile、CI 与本地 Playwright 从同一清单构建全部 agent 与连接器假程序（含 `agy`、`qoder`、`qodercn`、`kimi`），Qoder/QoderCN 也有双向往返 E2E，供 Ubuntu 4 个 E2E 分片及 Go race job 共用；
  - 分片仅准备 Node 与浏览器，不重复安装 pnpm、前端依赖、Go 或构建应用；
  - 显式预构建路径无效或缺少假 agent 时失败，本地默认仍从源码构建；
  - CI 不再使用 macOS runner（GitHub macOS arm64 容量不足导致 job 长时间排队后失败），只以 `GOOS=darwin go vet` 检查 macOS 构建；macOS 运行时行为需在 Mac 本机运行 `make test` 验证；
  - 性能测试保持单 worker 和 4x CPU 节流；
  - `Test result` 接受因变更分类而跳过的 `skipped`，并按分类结果交叉校验所有应运行的 job 均成功；
  - E2E 使用独立后端、目录、端口与假 agent 会话库（假 agent 位于后端 `PATH` 首位，混合会话用例由它驱动），等待隔离 Shell 提示符就绪后输入命令；
  - 失败有报告与 trace；
  - 测试不连接现有开发服务；在外层 WebMux 中运行时清除继承的会话标记，依赖真实外部父进程的少量集成用例明确跳过，并按服务器归属判定会话；
  - Codex 混合会话夹具清空继承的 `CODEX_PATH`，从夹具自己的 `PATH` 查找假 agent；往返测试用无效宿主路径验证不会误用真实 Codex；
  - PTY 取消写入测试由子进程真实 drain 后断言后续写入成功，attach 取消统一返回 `context.Canceled`，Codex 启动静默期前提失效时失败而非跳过核心断言。
- **US-L03 预览 UI 与生成证据**：作为贡献者，我希望用真实组件预览极端内容、明暗主题和屏幕尺寸，以便发现溢出并为 UI 变更提供证据。
  验收要点：
  - Storybook 提供设计系统、工作区与溢出场景；
  - 浏览器检查布局与可达性；
  - 截图脚本运行真实后端；omp 对话默认从仓库内录制的会话回放，不调用模型，`WEBMUX_SCREENSHOTS_RECORD=1` 时真实录制并导出新会话；
  - 截图脚本按当前侧栏外壳与新建会话弹窗操作，并额外生成端口转发提示（`desktop-port-forward.png`）与快速开始页（`desktop-quick-start.png`）两张功能截图；
  - `screenshots:hero` 由已生成的桌面与手机截图合成 README 主图，缺图时失败；
  - 可见 UI 的 PR 附最终实现截图。浏览器 UI 的 PR 同时覆盖桌面（1280 × 900）、平板（768 × 800）和手机（390 × 844）视口；
  - 修改已有界面时，每个视口提供主题、示例数据和交互状态一致的前后对比。
  - 可见 UI 的 PR 附最终实现截图；涉及交互或动画时还须嵌入真实运行界面的 GIF，展示初始态、操作与结束态，注明视口、主题和演示数据，验证公开链接可播放，后续修改时更新。
- **US-L04 基线评测与多平台发布**：作为维护者，我希望可复现地测量核心能力并构建各平台产物，以便比较回归和交付版本。
  验收要点：
  - `make baseline` 使用隔离环境评估创建、输入、回放、生命周期、安全等能力，报告通过率与延迟边界；
  - `BenchmarkAttachOutput` 用真实 PTY → Session → Attach → WebSocket 测量连续输出、控制边界及大块输出的分配，并校验字节与顺序；
  - `make release` 禁用 CGO，构建 Linux/macOS 服务及 Windows CLI/托盘双架构产物，嵌入服务摘要并生成 SHA256SUMS；
  - `make tray` 可单独构建托盘；
  - 发版先由 `verify` 查找标签提交在 `main` 上完整通过的 `push` Test 运行并复用；不在 `main`、无运行、失败、有跳过作业或等待超时则 `Checks` 以 `full: true` 完整重跑，两者都未通过时不签名、不发布；
  - 正式版与开发版安装 NSIS 时使用独立的 kernel.org Ubuntu HTTPS 镜像源，保留 Ubuntu 归档签名校验，不修改 runner 全局源；索引下载失败立即终止构建，不使用旧索引继续安装。
  - 现有 CI 执行相应质量检查。
- **US-L05 分层架构约束**：作为贡献者，我希望 Controller 与 Service 的边界和线上 JSON 形状由测试守住，以便越界依赖或协议漂移在 `go test` 中就被发现。
  验收要点：
  - `go test ./...` 包含 `internal/archtest`：未归类或失效的包、越界依赖、Controller 直接 I/O 或使用未登记的 Service 包级函数、Service 携带 HTTP 状态、编解码非 `contract` 类型、Controller 在登记函数之外读取请求体等违规一次列出位置、R0–R11 编号及修复提示；
  - 同一包还检查 `internal/acp`、`internal/hybrid`、`internal/agentprofile` 核心与 `web/src`（启动配置图标目录除外）的生产代码不出现任何已注册 agent 的名称或可执行文件名，agent 差异只能放在 `internal/agentprofile/<agent>` 适配器中。纯函数例外需理由且仍被使用；
  - 允许业务整数与非错误对象的领域 Status。R9 检查包级初始化及编码器函数值/方法值，登记函数不能借别名绕过。固定样例覆盖违规与合规写法。141 份线上快照分九组独立运行（另有不可用服务场景单独运行），仅归一化明确动态字段，业务正文、确定性 Git OID、摘要和固定输入保留；
  - 差异可见，有意变更用 `-update` 重新录制；本地状态快照的必填数组保持 `[]`，核心错误由共享传输映射，可选服务错误集中在 `internal/featureapi/errors.go`。
- **US-L06 前端架构守卫**：作为贡献者或 coding agent，我希望在提交前得到复用组件、设计令牌和分层方面的具体报错，以便新增 UI 复用设计系统。
  验收要点：
  - `pnpm --dir web check:arch` 依次运行 ESLint（`@shadcn/lint` 与 Svelte 模板规则）、dependency-cruiser、jscpd、knip 与 Vitest 架构测试；
  - 架构测试由 Vite+ `vp test` 承载，`make check` 与 CI 的格式/构建 job 同样执行完整架构守卫；
  - 报错说明应复用的组件、size/variant 或主题令牌；
  - 源码按契约、基础设施、领域主题、hooks、组件与页面分层；根目录只保留入口、声明与全局样式，领域模块、hooks 与组件必须有主题归属；
  - 依赖规则同时约束运行时与 type-only 导入，拒绝反向依赖、主题间循环与失效导入；临时真实模块样例验证允许和拒绝路径，迁移不保留旧路径垫片；
  - 圆角只允许设计系统刻度（标签 sm、控件 md、内容容器 lg，`rounded-xl` 仅限 Dialog/AlertDialog/Drawer 弹层），裸 `rounded`、`rounded-2xl` 以上与 CSS 固定像素圆角会列出文件位置；
  - 提交的基线为空，任何违规都会失败；
  - 引入带存量违规的新规则时可先记录基线，`arch:ratchet` 只删除已修复的记录；
  - Vite+ `vp lint`（Oxlint）仍是通用 lint，不替代 ESLint 的架构和模板规则。
- **US-L07 订阅模型辅助贡献**：作为维护者，我希望 Pullfrog 使用自己的 OpenCode Go 订阅运行任务，以便通过指定的 DeepSeek Flash 模型辅助仓库维护。
  验收要点：
  - Pullfrog workflow 接收 prompt 并显式指定 `opencode-go/deepseek-v4.1-flash` 与 `effort: high`，固定 High 推理等级；
  - 使用 GitHub Actions Secret `OPENCODE_API_KEY`，不再注入 DeepSeek 直连密钥；
  - 配置说明列出 OpenCode Global 区域授权、Pullfrog BYOK 与 Custom 模型设置以及订阅额度限制。
- **US-L08 文档与 spec 治理**：作为贡献者或 coding agent，我希望文档和 OpenSpec 规格有可执行的行数、单行长度、体积与目录扇出上限，以便单次读取保持可管理，失败信息直接给出拆分、合并、分段或整理指令。
  验收要点：
  - `make check` 与 CI 的格式/构建 job 运行 `make docs-check`，即 `go test -count=1 ./internal/docguard`；同一守卫检查故事 ID 唯一、主索引范围连续、覆盖索引引用存在，退役 ID 只在说明历史关系时引用；
  - `docs/**/*.md` 与 `openspec/specs/**/spec.md` 限制为 20–400 行且不超过 48 KiB；
  - 未归档的 `openspec/changes/**/*.md` 限制为不超过 400 行且不超过 48 KiB（不设行数下限）；
  - `openspec/changes/archive/**` 不参与行数、单行长度与体积检查；
  - 其余 Markdown 单行不超过 300 列（汉字与全角标点按 2 列计），仅围栏代码块豁免，表格行不豁免；
  - `docs/` 与 `openspec/` 的每一级直接子目录不超过 20 个，`docs/images/`、`openspec/specs/` 与 `openspec/changes/archive/` 的固定布局豁免；
  - 失败信息列出实测值、阈值与 AI 指令，要求按主题拆分或合并、在句子边界分段并保留旧路径为树状索引，规则与例外见 [文档治理](../../doc-governance.md)；
  - 超限时按主题迁移完整内容，保留原文事实与表格说明，接近上限时先拆分再新增，并在 PR 中说明新旧位置对应，原则见 [文档治理](../../doc-governance.md#超限时按主题迁移完整内容)。
  - 写作规则记录在 `AGENTS.md`：直接陈述事实、行为、条件和结果；已探索失败的方案记录尝试过程及失败原因。
- **US-L09 前端代码质量基线**：作为贡献者或 coding agent，我希望前端代码质量有成文基线和可执行门禁，以便一致审查响应式状态、异步行为、类型和前后端契约。
  验收要点：
  - `docs/frontend-style.md` 以 Svelte 官方 Best practices 为主规范、Google TypeScript Style Guide 为补充，登记本地化取舍与 7 条日常审查规则；
  - `pnpm --dir web lint` 通过 Vite+ 内置的 Oxlint 类型感知运行 Promise 规则，`check:arch` 保留 ESLint 的 Svelte 推荐模板规则；
  - `check:arch` 的 Knip 检查跟踪 CSS 导入，识别样式表直接使用的依赖；
  - 所有 `svelte-check` 入口保留 `--tsrs` 并拒绝 warning；Vite+ 的 lint 不代替 Svelte/TypeScript 类型检查；
  - `check:test` 检查源码和顶层 Node 测试并进入 CI；
  - 严格索引、请求体 Zod 校验、Go 强类型枚举及 `wire-contract.test.ts` 防止契约漂移；
  - 中途 401、上传失败重试、重复提交、Dialog 焦点循环和列表断线恢复各有失败路径回归；
  - `AGENTS.md` 与 `CONTRIBUTING.md` 提供入口；
  - 文件遵守 20–400 行与 48 KiB 的文档治理限制。
- **US-L10 持久化清单守卫**：作为贡献者，我希望新增存储和文件写入入口遗漏登记时自动失败，以便配置与状态清单不会继续漂移。
  验收要点：
  - `make persistence-check`、`make check` 和 CI 检查全部平台生产 Go 源码中的 JSON/锁名及选定写入函数引用；
  - 别名、函数值、调用次数、登记来源与原因、文档表格均受检查；
  - 负向样例证明漏登和过时登记会失败。运行时 descriptor 与文档、源码入口一致；
  - 所有领域通过统一 handle，逐描述符契约、提交阶段故障与严格读取测试防止退化。清单包含混合会话记录 `hybrid-sessions.json`（故障策略 unavailable）。
- **US-L11 AGENTS.md 体积预算**：作为贡献者或 coding agent，我希望 `AGENTS.md` 保持在固定体积内，以便每次会话加载的常驻指令精简，不常用规则下沉到按需阅读的文档。
  验收要点：
  - `internal/archtest` 的 `TestAgentsGuideSize` 在 `go test ./...` 中检查 `AGENTS.md` 不超过 10 KiB；
  - 超限时失败信息给出实测值与超出字节数，指示 agent 把最不常用的规则原文迁入 `docs/agent-rules/<主题>.md`、在 `docs/agent-rules.md` 登记、在 `AGENTS.md` 留下一行摘要与链接，并满足文档治理限制，且禁止靠删除规则或调高上限通过；
  - `TestAgentsSizeViolationMessage` 验证恰好超限一字节即失败并包含上述指令；
  - `TestAgentsRulePagesExist` 保证索引页链接的下沉页面都存在；
  - 下沉页面见 [Agent 规则](../../agent-rules.md)。
- **US-L12 文档导航与首页**：作为初次访问者、使用者或 coding agent，我希望从 `README.md` 或 `AGENTS.md` 出发总能沿链接找到任何一份文档，首页直接给出介绍、截图、快速开始与帮助入口，且用户文档按由浅入深组织，以便快速了解项目并找到答案。
  验收要点：
  - `make docs-check` 运行 `TestDocsNavigation`：从 `README.md`、`AGENTS.md` 沿相对链接遍历，链接到目录时视为到达其 `README.md`；
  - `docs/**/*.md`、`baseline/*.md`、`openspec/specs/*/spec.md` 与根目录 `*.md` 必须可达；在途 `openspec/changes/` 与归档不要求可达；
  - 上述文件与在途变更中的相对链接必须指向存在的文件，指向 Markdown 的锚点必须匹配 GitHub 生成的标题锚点或 HTML `id`/`name`；围栏代码块与行内代码中的示例不受检查；
  - `README.md` 不超过 170 行、14 KiB，单行不超过 300 列；须含“快速开始”“文档”“获取帮助”二级标题，链接 `docs/overview.md`、`docs/README.md`、`docs/faq.md`、`SECURITY.md` 与 `CONTRIBUTING.md`；
  - `README.md` 至少引用 4 张 `docs/images/` 下的界面截图（品牌图除外）、至多 20 张图片，每张 `<img>` 与 Markdown 图片都有替代文本；
  - 失败信息写明失效链接的行号与原因、不可达文档应登记的索引位置，以及 README 超限时把详细内容迁往 `docs/` 的去向；
  - `TestNavigationRules` 与 `TestSlug` 用内存目录树固定可达性、失效链接、锚点、代码豁免与 README 各项规则；
  - `.agents/skills/webmux-docs/SKILL.md` 定义面向网站读者的由浅入深（三层金字塔）写作规范、信息守恒原则、页面模板与近期提交同步核对清单，并在 `AGENTS.md` 中登记；
  - 文档入口见 [文档中心](../../README.md)，项目介绍见 [项目介绍](../../overview.md)，帮助见 [常见问题与获取帮助](../../faq.md)，规格索引见 [OpenSpec 规格索引](../../specs.md)。
- **US-L13 发布签名**：作为维护者，我希望每次发布自动签名校验和文件并在发布前验证，以便自动更新与用户手动校验都有独立于镜像和下载来源的信任根。
  验收要点：
  - `make release` 在构建前检查签名私钥（正式版为 `WEBMUX_RELEASE_SIGNING_KEY`，开发版为 `WEBMUX_DEV_SIGNING_KEY`），缺失即失败，依次执行 `release-build` 与 `release-sign`；
  - 所有发布二进制注入发布构建标记，本地构建不带；
  - 生成 `SHA256SUMS` 后 `releasesign sign` 产出 minisign `Ed` 格式的 `SHA256SUMS.minisig`（可信注释 `webmux <版本>`，不列入 `SHA256SUMS`），再以内置公钥 `verify`；
  - 发布 workflow 分为 build、sign、publish 三个 job，只有 sign 读取私钥，它在全新 runner 上只拿到 `SHA256SUMS`、不恢复缓存也不运行第三方构建代码，publish 再次验证后上传签名；
  - `releasesign keygen` 以 0600 新建私钥文件并输出公钥；
  - 轮换时先发布同时内置新旧公钥的版本（`internal/update` 与 `install.sh` 同步维护，由 `releasesign` 测试校验一致）；
  - 签名与 minisign 0.12 互通。
- **US-L14 发布开发版**：作为维护者，我希望 main 上每个通过 CI 的提交自动发布为签名的开发版，以便尝鲜用户在正式发布前拿到新构建，同时不让正式版用户收到它。
  验收要点：
  - `test.yml` 在 main 的 push 上成功后触发，构建通过测试的那个提交，版本为最近正式标签补丁号加 1 再加 `-dev.<提交数>`；提交数为 0、已不是 main 最新提交、版本不高于已发布开发版，或自 `dev` 标签以来只改了文档时跳过；
  - 只发布到预发布 Release `dev`（不设为 latest），产物名带版本号，`install.sh` 为固定文件名；先上传新产物，再替换 `install.sh` 与签名清单，移动 `dev` 标签，最后删除不在本次清单中的文件，只保留当前构建；
  - 签名 job 用独立的 `WEBMUX_DEV_SIGNING_KEY`，`releasesign` 按版本选择私钥与公钥组，sign 与 publish 不可取消；
  - 正式版发布说明从上一个 `v*` 标签生成，不以 `dev` 标签为起点；
  - 文档公布开发版公钥与轮换流程，说明正式标签须在 main 可达提交上、更新本地 `dev` 标签需 `git fetch --tags --force`。
- **US-L15 文件命名与大小约定及守卫**：作为贡献者或 coding agent，我希望仅凭职责就能预测源码文件名，且单个生产源码文件保持在 500 行以内，以便不用全仓搜索就能定位或放置代码，并在文件膨胀时收到明确的职责拆分指引。
  验收要点：
  - [文件命名](../../agent-rules/naming.md)规定前端非组件 TypeScript 用 kebab-case、组件与组件测试用 PascalCase、Go 文件用 snake_case 并按“名词/职责”命名；
  - `TestGoFileNames` 在 `go test ./...` 中拒绝非 snake_case 的 Go 文件名，以及含 `util`、`helper`、`misc`、`more`、`common` 等空泛词的名字；
  - `TestSourceFileSize` 在 `go test ./...` 中要求 `cmd/` 与 `internal/` 下的非测试生产 `.go` 文件少于 500 行（`<= 499` 行），超限时报错指示按职责拆分为同家族 `<family>_<part>.go` 文件并禁止删注释或改用空泛文件名；
  - `web/test/architecture.test.ts` 的 `file naming` 拒绝 camelCase 的 TypeScript 文件、非 PascalCase 的 Svelte 组件，以及 500 行及以上的生产 `.ts`/`.svelte` 文件（排除生成的 `src/lib/components/ui/`、测试与 stories），超限时提示按领域模块、子组件或架构分层拆分；
  - 规则页登记在 [Agent 规则](../../agent-rules.md)，`AGENTS.md` 只保留一行指针。

- **US-L16 仓库内的 Agent 任务交接**：作为接手任务的 coding agent，我希望从分支和文件路径恢复工作，知道前一位 agent 验证了什么以及下一步要做什么。
  验收要点：
  - 从 [Agent 任务交接](../../../.agents/handoffs/README.md) 找到模板，按任务创建或更新记录；
  - 接手者能列出待接记录、核对提交与验证结果，接手后更新负责人，完成后标明结果。

依据：

- [前端代码质量基线](../../frontend-style.md)、[AGENTS.md](../../../AGENTS.md)、[CONTRIBUTING.md](../../../CONTRIBUTING.md)
- [Go 质量基线](../../go-style.md)、[Go lint 配置](../../../.golangci.yml)、[数组契约 fixture](../../../internal/featurewire/arrays_test.go)
- [数组 Zod 用例](../../../web/test/wire-arrays.test.ts)、[推送 API 用例](../../../internal/api/push_test.go)、[Node.js 版本](../../../.nvmrc)
- [CI 工具链](../../../.github/actions/setup/action.yml)、[Makefile](../../../Makefile)、[前端命令](../../../web/package.json)、[Go 覆盖率汇总](../../../.github/scripts/go-coverage-summary.sh)、[前端覆盖率合并](../../../web/scripts/merge-coverage.mjs)
- [Vite+ 配置](../../../web/vite.config.ts)（构建、代理、格式化与 lint）
- [贡献指南](../../../CONTRIBUTING.md)、[Storybook](../../storybook.md)、[评测说明](../../../baseline/README.md)
- [UI PR 截图要求](../../agent-rules/testing.md#ui-pull-request-evidence)
- [真实终端分配基准](../../../internal/hub/hub_bench_test.go)、[CI](../../../.github/workflows/test.yml)、[E2E 初始化](../../../web/test/e2e/global-setup.mjs)
- [分层清单](../../../internal/archtest/layers_test.go)、[agent 名称守卫](../../../internal/archtest/agentnames_test.go)、[架构规则](../../../internal/archtest/arch_test.go)、[规则样例](../../../internal/archtest/testdata/fixture)
- [线上快照测试](../../../internal/api/wire_snapshot_test.go)、[归一化回归测试](../../../internal/api/wire_normalizer_test.go)、[分层说明](../../architecture.md#分层与架构测试)
- [前端架构守卫](../../frontend-architecture.md)
- [AGENTS.md 体积测试](../../../internal/archtest/agents_size_test.go)、[Agent 规则索引](../../agent-rules.md)、[文件命名约定](../../agent-rules/naming.md)
- [文件命名守卫](../../../internal/archtest/filenames_test.go)、[文件大小守卫](../../../internal/archtest/filesize_test.go)、[前端命名与大小守卫](../../../web/test/architecture.test.ts)
- [文档导航守卫](../../../internal/docguard/links_test.go)、[文档中心](../../README.md)、[首页](../../../README.md)
- [ESLint 架构规则](../../../web/eslint.config.mjs)、[前端分层规则](../../../web/.dependency-cruiser.cjs)、[前端架构测试](../../../web/test/architecture.test.ts)、[文档治理](../../doc-governance.md)、[文档治理规则与样例](../../../internal/docguard/doc_test.go)。

质量基线补充依据：

- [UI PR 证据规范](../../agent-rules/testing.md#ui-pull-request-evidence)、[UI 变更 skill](../../../.agents/skills/webmux-ui/SKILL.md)。

- [ESLint 与 TypeScript 配置](../../../web/eslint.config.mjs)、[测试类型配置](../../../web/tsconfig.test.json)、[wire 契约测试](../../../web/test/wire-contract.test.ts)
- [新建会话与 Dialog 浏览器回归](../../../web/test/browser/NewSessionDialog.test.ts)、[主题与可访问性 E2E](../../../web/test/e2e/frontend-quality.spec.ts)、[上传失败 E2E](../../../web/test/e2e/upload.spec.ts)
- [列表重连 E2E](../../../web/test/e2e/workspace-discovery.spec.ts)。

Pullfrog 依据：[workflow](../../../.github/workflows/pullfrog.yml)、[订阅模型配置](../../../CONTRIBUTING.md#pullfrog-模型配置)。

持久化守卫依据：

- [扫描器](../../../internal/archtest/persistence_scan_test.go)、[清单校验](../../../internal/archtest/persistence_test.go)、[负向样例](../../../internal/archtest/persistence_rules_test.go)
- [机器清单](../../../internal/archtest/persistence_inventory.json)、[治理说明](../../persistence.md)。

发布签名依据：[签名工具](../../../cmd/internal/releasesign/main.go)、[签名格式与验证](../../../internal/update/minisign.go)、[Makefile](../../../Makefile)、[发布 workflow](../../../.github/workflows/release.yml)、[密钥轮换](../../deployment/updates.md#密钥轮换维护者)。

开发版发布依据：[开发版 workflow](../../../.github/workflows/dev-release.yml)、[开发版 Release 说明](../../deployment/updates.md#开发版-release)、[版本与通道](../../../internal/update/channel.go)、[清单与产物](../../../internal/update/update.go)。
