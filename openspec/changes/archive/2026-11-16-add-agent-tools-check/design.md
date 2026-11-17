# Design

## Context

动机与范围见 `proposal.md`。当前 WebMux 已在两处维护了部分 Agent 元数据：

1. `internal/agentprofile` 及其各子包（`claudecode`、`codex`、`agy`、`opencode`、`omp`、`qoder`、`qodercn`、`kimi`、`devin`）声明了混合会话 Agent 的 `Title`、`Icon`、`Executables`、`VerifiedVersion` 与可选 `Connector`；
2. `internal/launch/presets.go` 额外列出了 4 个纯终端 Agent 预设（Cursor CLI `agent`、GitHub Copilot CLI `copilot`、Qwen Code `qwen`、CodeBuddy Code `codebuddy`）。

同时仓库架构守卫有四条硬约束直接决定实现结构：
- **R0 / R6 出站边界**（`internal/archtest/layers_test.go`）：只有显式登记在 `outbound` 表中的 `service` 层包才能调用 `http.Client` 发起外部请求；Controller 层严禁执行 `os/exec` 或文件系统探测（R3/R4）。
- **Agent 名称隔离**（`internal/archtest/agentnames_test.go`）：`internal/acp`、`internal/hybrid`、`internal/agentprofile`（根包）以及前端 `web/src/`（除两处图标映射文件外）禁止出现任何已注册 Agent 或连接器的名称字面量。
- **注册表装配边界**（`internal/archtest/agentassembly_test.go`）：`internal/` 下任何生产代码不得导入 `internal/agentprofile/agents`，内置集合只能由 `cmd/` 装配根注入。
- **持久化守卫**（`internal/archtest/persistence_*`）：禁止未登记的状态目录文件写入。

## Goals / Non-Goals

**Goals:**
- 在单次本机扫描通常 100–300ms 内完成全部内置 Agent 与 ACP 适配器的登录 Shell 路径解析和 `--version` 版本采集。
- 按需并发查询公开上游包仓库（npm、PyPI），具备超时、响应大小上限、重定向防护、镜像回退（npm 官方源失败回退 `registry.npmmirror.com`）与内存 TTL 缓存。
- 严格通过 `internal/archtest`、`internal/docguard` 与 `pnpm --dir web check:arch`，前端零硬编码具体 Agent 名称。

**Non-Goals:**
- 不提供一键在后台静默执行 `npm install -g` 或自升级脚本的写操作接口，避免权限不一致或破坏运行中的 Agent 会话；升级通过一键复制推荐命令由用户在终端可控执行。
- 不对专有闭源分发渠道逆向私有更新协议；无公开稳定包源的工具诚实标记为 `source_unavailable`。

## Decisions

### 1. 元数据分层：`agentprofile.Distribution` + `internal/agentcatalog`

- **方案**：
  - 在 `internal/agentprofile` 根包定义与具体 Agent 无关的 `UpstreamSource`（种类 `UpstreamNone` / `UpstreamNPM` / `UpstreamPyPI` 及包名 `Package`）和 `Distribution`（`InstallCommand`、`UpgradeCommand`、`DocsURL`、`Upstream`），挂在 `Profile.Distribution` 与 `Connector.Distribution`（`Connector` 已有的 `InstallCommand`、`DocsURL` 直接复用）。各混合 Agent 子包就地填写自己的分发与上游信息。
  - 新增共享层包 `internal/agentcatalog`（归类 `shared`），定义统一的只读探测描述符 `Entry`：既提供 `FromRegistry(*agentprofile.Registry)` 把注入的混合 Agent 档案转为 `Entry`，也提供 `TerminalPresets()` 返回 4 个纯终端预设（Cursor、GitHub Copilot、Qwen Code、CodeBuddy Code）的 `Entry`。
  - `cmd/webmux` 在启用 `launch` 时组合 `agentcatalog.Build(agentsRegistry)` 注入 `internal/agentcheck.Service`。
- **理由**：混合 Agent 差异继续留在 `internal/agentprofile/<agent>` 子包，纯终端预设集中在非中立共享包 `agentcatalog`，既不触犯 `agentnames_test.go`，也遵守 `agentassembly_test.go` 由 `cmd` 唯一选择内置集合的规则。
- **备选方案**：在 `internal/launch` 中硬编码所有 Agent 的包源映射表——这会让新增混合 Agent 时必须跨包同步两处列表，违背适配器单一职责。

### 2. 两阶段本机探测：单次批量交互式 Shell 定位 + 并发直连 `--version`

- **方案**：
  - **路径定位**：复用 `agentprofile.DefaultShell(shell)`。当默认 Shell 为 `bash` 或 `zsh` 时，构造单条无外部插值注入风险的参数化交互式脚本（将候选二进制名作为位置参数 `"$@"` 传入 `bash/zsh -ic`，循环输出固定前缀标记 + 名称 + `command -v -- "$name"` 结果），**仅启动 1 次交互式 Shell**（10 秒超时、64 KiB 输出上限、独立进程组）即可拿到全部 17 个候选二进制的真实路径；若交互式 Shell 失败或默认 Shell 不受包装支持，平稳回退到进程 `PATH` 查找。
  - **版本采集**：对解析出的唯一绝对路径集合，以有界并发（并发度 4）直接执行 `<absPath> --version`（不经 Shell、`Stdin` 置空、注入 `CI=1` 与 `NO_COLOR=1`、`Setpgid: true`、单进程 3 秒超时、4 KiB 输出上限）。使用通用点分版本号提取器扫描输出前若干行中的首个合法版本字段（复用 `agentprofile.CompareVersions` 语法规则）。
- **理由**：若为 15 个工具分别启动 `bash -ic "<tool> --version"`，在重度 `.zshrc`（含 nvm/conda 初始化）机器上会串行或并发加载十几次 Shell，耗时高达 3–8 秒；单次 Shell 批量 `command -v` 加直连二进制执行将开销压缩一个数量级。

### 3. 上游版本源与安全出站 HTTP 客户端（`internal/agentcheck`）

- **方案**：
  - 新增 `internal/agentcheck` 包，在 `internal/archtest/layers_test.go` 归类为 `service`，并在 `outbound` 登记出站理由。
  - 支持两类结构化公开 JSON 源：
    1. **npm**：请求 `https://registry.npmjs.org/<escaped-pkg>/latest`（直接源连接/状态失败时回退 `https://registry.npmmirror.com/<escaped-pkg>/latest`，兼顾中国大陆网络可用性），解析顶层 `{"version": "..."}`。
    2. **PyPI**：请求 `https://pypi.org/pypi/<pkg>/json`，解析 `{"info": {"version": "..."}}`。
  - 安全加固：使用专用 `http.Client`，单源请求超时 6 秒，`io.LimitReader` 限制响应体最多 64 KiB，禁用 Cookie，自定义 `CheckRedirect` 仅允许 HTTPS 且目标主机属于允许的公共仓库域名（最多 3 次跳转），杜绝 SSRF 或凭据泄露。
  - 仅对**当前本机已安装**且声明了 `Upstream` 的工具/适配器发起网络查询（未安装工具无需联网查新，直接节省 60%+ 请求量）。

### 4. 内存缓存与 Singleflight 并发控制

- **方案**：
  - `agentcheck.Service` 内部使用互斥锁与按操作维度的飞行合并（in-flight waiter 复用）：
    - 本机扫描缓存 TTL `30s`（`refresh=true` 强制失效重扫）；
    - 上游每个包源的版本缓存 TTL `30m`（`POST /api/agent-tools/check` 时默认强制刷新或按需更新已安装项缓存，保证用户点击「检查更新」总能拿到最新远端结果，而 `GET /api/agent-tools` 直接拼合当前未过期的远端缓存）。
- **理由**：零磁盘状态文件新增，无需维护文件锁或迁移逻辑，多浏览器标签页同时打开设置页也不会重复派生子进程。

### 5. Web UI 布局：设置页 `#agent-tools` 常驻卡片（方案 A）

- **方案**：
  - 在 `web/src/features/settings.ts` 中于 `launch` 功能下注册 `agent-tools` 设置分区（紧随 `launch-configs` 之后），对应组件 `web/src/components/settings/AgentToolsSettings.svelte`。
  - 严格复用现有设计系统组件：`Card.Root flat`、`Item.Group variant="divided"`、`Collapsible`、`Badge`、`Button`、`Skeleton`、`InlineNotice`、`InfoHint` 与 `LaunchConfigIcon`。
  - 状态与按钮层级：`检查更新` 为卡片内唯一 `default` 绿色主按钮；`重新扫描本地安装` 为 `outline` + `size="icon"`；行尾复制升级/安装命令为 `ghost` + `size="icon"`。已是最新仅用柔和文字展示，有更新使用 `Badge variant="info"`，缺失 ACP 适配器或低于 `verifiedVersion` 使用 `Badge variant="warning"`。未安装项默认收起在底部 `Collapsible` 中。

## Risks / Trade-offs

- **[Risk] 用户极慢或损坏的 `~/.bashrc` / `~/.zshrc` 阻塞路径定位** → **Mitigation**：单次批量定位设独立进程组与 5 秒硬超时，超时自动终止进程组并回退进程 `PATH` 查找，同时在响应中保留环境降级提示而不让接口报错失败。
- **[Risk] 部分专有 Agent（如 `agy`、`devin`、Cursor `agent`）无公开 npm/PyPI 包源** → **Mitigation**：明确区分 `source_unavailable` 与 `up_to_date`，UI 仅展示本地版本与已验证基线对比，绝不误导用户“已是最新”。
- **[Risk] 中国大陆网络直连 `registry.npmjs.org` 偶发超时** → **Mitigation**：内置与 `internal/fetch` 一致的镜像回退思路，npm 官方源失败时自动回退 `registry.npmmirror.com`（只读查询版本号，不下载执行任何代码）。
