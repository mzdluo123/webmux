## Context

动机见 [proposal.md](proposal.md)。本设计基于隔离 worktree 的 `origin/main` 提交 `8f3f127`；原工作区的未提交聊天改动不属于本方案的实现产物。
以下目录、接口形状及拆分均为设计，不表示当前代码已经插件化。

现状中的关键约束：

- `internal/api/router.go:newServer` 要求 Manager、Workspace、Code、Files 同时存在，routes 无条件挂载业务。
- `internal/api/sessions.go:create` 使用 `Workspace.Create`；普通输入/中断使用 Hybrid 的结果及错误类型。
- `cmd/webmux/main.go:serve` 无条件打开 launch、conversation、hybrid 等状态，并注册相互清理/保留的回调。
- `session.Info`、`session.Overlay`、`wire.Session` 和 hub 包含仓库、聊天和端口业务语义。
- `server_config.go` 在 serve 之前已经打开远程访问/P2P 设置，不能只在 serve 内判断启用集。
- `web/src/App.svelte:check` 在健康检查后启动 settings、P2P 与推送同步；AppShell 读取仓库和启动配置。
- 核心终端 stage 静态引用聊天、语音、交接等；已有页面 lazy import 不是插件边界。
- Web 自定义命令目前不提交 env；API/CLI 支持显式 env。本次不新增环境变量编辑器。
- 架构 R0–R11 要求 cmd-only 构造服务、Controller/Service 分离、contract/wire 编码和集中请求体边界。

## Goals / Non-Goals

**Goals:**

- 一个功能的页面、后端、状态与后台任务具有同一插件归属；禁用是运行边界，不只是界面条件。
- 核心独立创建普通 PTY；不以构造空 Workspace 或 Hybrid 替身实现“零插件”。
- 插件通过窄接口和固定 UI 位置增强核心；依赖由装配根明确连接。
- 同一启用结果控制 TCP、本机接口、P2P、浏览器、CLI 和托盘的能力呈现。

**Non-Goals:**

- 外部插件安装、动态 Go 库、远程 JavaScript、跨版本第三方 SDK、插件市场。
- 热加载、热卸载、插件权限沙箱或进程崩溃隔离。
- 裁剪二进制和静态资源体积、重写业务协议、改用通用事件总线转发 PTY 字节。
- 新建 env 编辑器、多用户权限体系或跨服务重启保留普通 PTY。

## Decisions

### 1. 纵向功能包，横向保持分层

```text
+------------------------------------------------------------------+
|                          WebMux Binary                           |
|                                                                  |
|  Browser                          Server                         |
|  +---------------------------+    +---------------------------+  |
|  | Core shell / Terminal     |    | Core HTTP / WS / Security |  |
|  | Core list / Mobile input  |    | PTY / Replay / Control    |  |
|  +-------------+-------------+    +-------------+-------------+  |
|                |                                |                |
|  +-------------v-------------+    +-------------v-------------+  |
|  | UI contribution registry  |    | Bound route contributions |  |
|  +-------------+-------------+    +-------------+-------------+  |
|                |                                |                |
|       +--------v--------------------------------v--------+       |
|       |                 Code Plugin                      |       |
|       | Page / Panel / Actions <--> API / File / Diff     |       |
|       +--------------------------------------------------+       |
|       |              Other vertical plugins              |       |
|       +--------------------------------------------------+       |
+------------------------------------------------------------------+
```

业务所有权不要求把 Svelte 和 Go 放进同一物理目录；复用现有构建系统，不增加单独插件打包工具。

建议源文件归属：

```text
cmd/webmux/
  plugin_catalog.go            # Static selection and dependencies
  plugin_assembly.go           # Explicit construction and lifecycles
internal/
  api/                         # Core host and core endpoints
  transport/                   # Controller helpers and route policies
  session/                     # PTY and generic process-host ports
  sessioncreate/               # Core create coordination, no HTTP
  pluginapi/<id>/              # Optional controllers, prebound handlers
  wire/                        # Core projection only
  pluginwire/<id>/             # Optional typed projections
  workspace/ codeview/ ...     # Existing business services, reused
  contract/                    # All named wire DTOs, stdlib only
web/src/
  platform/plugin-host/        # Bootstrap and contribution composition
  plugins/<id>/entry.ts        # Lazy UI registration and disposal
  domains/<existing-domain>/   # Existing domain logic, reused
```

新 Controller 包不导入 `internal/api`；仅 cmd 导入 api 的 R2 保持不变。
将 decode/reply、消息连接与路由策略抽至 Controller 基础设施，仍登记 R8/R11 的编码和请求体汇点。
不通过扩大 archtest 豁免让插件服务读取 HTTP 请求或编码线上 JSON。

### 2. 核心与插件归属

核心保留终端渲染与输入体验，包括分屏、查找、复制、图形、草稿和送达确认。
核心状态仅含进程/终端事实、基础标题与退出状态；业务状态增强由插件拥有。
配置、基础偏好、运行记录、native attach/bridge、service hosting、安装更新与基础 CLI 帮助属于宿主基础设施。
agent shell guide 使用实际命令/能力贡献生成；基础帮助不得为了列命令构造插件服务。

插件目录及硬依赖：

| ID | 完整业务所有权 | 硬依赖 |
| --- | --- | --- |
| launch | 保存配置、预设、agent profiles/trust 与启动准备 | 无 |
| workspace | 仓库、worktree、Git 操作、协作及操作账本 | 无 |
| code | 文件树、代码/Diff/媒体预览、文件操作和独立窗口 | 无 |
| files | 目录浏览/补全/mkdir、上传、staging 和会话产物 | 无 |
| chat | ACP、混合切换、归档、恢复、conversation 身份/历史/交接 | launch、files |
| notifications | agent 注意力增强、页面提醒、Web Push | 无 |
| quick-commands | 保存的快捷指令及其菜单/设置 | 无 |
| voice | 外部识别命令、音频连接及设置 | 无 |
| forward | 监听端口发现、转发登记和网关 | 无 |
| cloudflare | 隧道进程、设置、访问入口 | 无 |
| tailscale | Serve/Funnel、设置、访问入口 | 无 |
| p2p | WebRTC 直连、映射、设置与传输升级 | 无 |
| mcp | stdio 控制、ACP 注入、历史 reader、owner TCP MCP | 无 |
| immersive | XR/沉浸式专用界面与入口 | 无 |

chat 暂将身份数据库与交接作为同一业务边界，避免禁用子模块意外失去未决投递保护。
chat 依赖 launch 的档案/目标配置及 files 的附件归属和保留机制；不强依赖 Git/workspace、code、notifications 或 MCP。
这比把当前服务图逐一变成独立插件更少组合、更容易守住恢复与交付不变量。

workspace 可以直接 cmd 创建会话；launch 不要求 workspace；code 以会话目录为根，不要求登记仓库。
Git 工作台页面属于 workspace + code 的组合贡献，以 `workspace.git-workbench` 能力公开；workspace-only 仍保留仓库、worktree 与分支管理，不暗中启用代码阅读器。
workspace 与 chat 的快速开始、代码引用插入聊天、快捷指令初始化等属于双方启用时的组合贡献，不引入反向硬依赖。
MCP-control 可以只依赖核心；历史工具仅在 chat 对应能力存在时注册，ACP 注入仅在 chat 与 mcp 同时启用时发生。
基础 TV/键盘可访问性留核心，沉浸式专用模式归 immersive。

### 3. 静态目录与确定的配置语义

```toml
# New generated config: terminal core only.
[plugins]
enabled = []

# Example replacement for terminal + code + chat:
# enabled = ["code", "launch", "files", "chat"]
```

`enabled` 使用能区分 missing 与空数组的解析模型。未声明字段使用固定的既有功能目录；新生成配置显式为空。
固定目录不意味着开启网络业务：tunnel.mode/enabled、MCP owner token、voice cmd 等原业务开关与前置条件仍约束运行。

解析顺序：TOML/CLI 解析与无副作用校验 → 插件 ID/依赖验证 → 仅启用插件读取网页覆盖 → 转换 service options → 构造。
模块禁用优先级高于 TOML、网页及业务 CLI 的启用请求；CLI 显式请求冲突时报错，不能悄悄启用。
不添加运行时插件管理写 API；用户编辑 TOML 后重启。既有已启用模块内的网页业务开关仍可动态工作。

未知、重复 ID、缺失依赖、循环在业务构造前失败；不自动补依赖。构建裁剪如现有 nop2p 要报告 unsupported，不能宣称模块正常可用。

### 4. 后端显式装配，不建立服务定位器

静态 descriptor 只有 ID、依赖和能力定义，不持有全局服务实例，也不自行做 I/O。
cmd 按固定拓扑构造具体服务及 Controller，将预绑定路由、流处理器、类型化投影和生命周期交给宿主。

```text
Config --> Resolve IDs --> Validate dependencies
                                 |
                                 v
                     cmd: Construct concrete services
                                 |
                    +------------+------------+
                    |                         |
                    v                         v
             Plugin controllers        Lifecycle records
                    |
                    v
       Core host: validate and mount bound routes
                    |
                    v
       Auth / Origin / Caller policy --> Handler
```

路由贡献含 method/pattern、预绑定 handler、插件 owner 和允许的传输/调用者策略。
ready 前校验精确冲突、模式重叠和核心保留路径；各传输入口只挂载其允许子集。
已启用模块保留既有业务路径，不迁到第二套 `/api/plugins/...` 别名。
禁用 API/WS 返回 404；页面深链接由前端宿主显示功能未启用，不能把 API 404 变成 SPA HTML。
新流贡献复用同一 HTTP/P2P 消息处理器，不再增加两份业务 switch。

不提供 `GetService(id)`、`map[string]any` 容器、全能 Host.Services 或可任意执行代码的配置入口。
允许宿主持有不具备业务对象的 ID→状态表；这不是用于业务依赖查找的服务注册表。

### 5. 普通会话创建和数据通道先归核心

从 Workspace.Create 抽出 HTTP-free 创建协调，复用现有幂等、输入门与失败结果。
对普通 cwd+cmd/env，核心直接解析/校验并创建；location、configId、chat、attachments 等由类型化扩展处理。
所有 capability precheck 先于 worktree/进程/配置副作用；保留已有 precheck→worktree→session→config→prompt 顺序和部分成功语义。
浏览器自定义命令保持不带 env 的既有规则；不删除 API/CLI 的显式 env 支持。

普通输入 DTO、错误、回执与 interrupt 归核心；聊天的私有中断和视图 gate 通过显式接口接入。
核心 Driver/sidecar、ID 预留和子进程清理可以保留为通用进程宿主端口，不包含 provider/ACP/归档业务。

session 内移除 Workspace/Hybrid/ListeningPorts 业务类型；插件拥有元数据，并通过类型化 Controller 投影组合已有 Session wire 字段。
主列表使用统一顺序协调器：先登记源订阅并捕获快照，再按序发布增量；增强状态先更新缓存再发通知。
不在 PTY 锁内做插件 I/O，不给每个输出块增加通用事件对象或复制；详情、快照、增量及归档投影保持一致。

### 6. 页面、面板与后台资源共同注册

```text
Authenticated browser
         |
         v
 GET /api/capabilities --> Validate version and schema
         |
         v
 Fixed local import map: enabled plugin entries only
         |
         +--> Pages / Navigation
         +--> Session views and side panels
         +--> Session and toolbar actions
         +--> Settings sections
         +--> Creation sources and resource start/dispose
```

贡献 ID 使用插件前缀，route/slot/order 均由本地代码定义，服务端能力响应不包含脚本 URL 或任意组件描述。
UI host 只接受已定义的贡献类型和窄上下文，如 sessionId、基础会话摘要、导航和已校验的输入提交接口。
不把 Shell 改成几十个可空业务字段；仓库、聊天等控制器由插件自己持有。

插件 entry 通过固定动态 import 加载；entry 之前不得静态导入插件实现。
提取 AppShell 初始化、SessionSwitcher 聊天搜索、SessionTerminalStage、设置导航和卡片的业务引用。
settings 导航与真实 section 来自同一贡献列表，避免隐藏卡片后残留空 DOM 引用。
资源 start 返回 dispose；认证失败、离开宿主、重新发现新实例时取消请求并释放订阅。
模块加载失败只影响该插件入口并显示错误，核心终端可用；发现协议失败则不给插件默认全开。

旧 `shellMode=chat`、`lastLaunch=configId`、`codePanelShown` 先按能力归一化，再解析首页和加载模块。
workspace 禁用则直接会话列表；chat 启用但 workspace 禁用则可直接目录启动聊天，不出现需要 Git 的快速开始。
files 禁用仍可输入已有 cwd；不自动调用 mkdir。launch 禁用时不把旧配置猜成 argv，用户确认自定义命令。

### 7. 一个能力契约，所有客户端消费

新增 `GET /api/capabilities`，使用现有 api/ws 客户端、contract DTO 和 Zod；TCP 与本机返回同一实例的能力事实，鉴权方式仍各自独立。

```json
{
  "version": 1,
  "plugins": [
    {"id": "code", "enabled": true, "status": "ready"},
    {"id": "chat", "enabled": false, "status": "disabled"}
  ],
  "capabilities": [
    {"id": "terminal.pty", "available": true},
    {"id": "code.browse", "available": true},
    {"id": "chat.acp", "available": false, "reason": "plugin_disabled"}
  ]
}
```

实际响应包含完整静态目录，不只示例两项。status 使用 disabled/ready/unavailable；reason 使用固定原因码。
模块 ready 不意味着业务已经打开；能力可用性合并配置、平台和服务健康，但不为探测而启动禁用服务。
单个 agent 连接器可用性仍使用其现有细粒度契约，不把所有 agent 合并成一个模糊布尔值。

能力快照描述可注册入口/当前健康；业务操作仍在服务端执行即时校验，不能依赖启动时快照代替校验。
既有网页业务设置改变后，按现有设置/配置刷新路径重新获取快照；不另造一套重连协议。

CLI 和托盘使用 localclient/client 的相同 DTO。基础 run/attach 不因插件发现而先读 launch/repositories。
MCP 工具发现按可用能力生成，不再硬编码 chat 或要求 launch-configs 接口成功。
静态 help 可列插件命令，但明确所需能力；连接实例后的可执行菜单只提供实际能力。

### 8. 生命周期与数据保留

```text
Resolve --> Construct --> Start --> Ready
                |                     |
                | failure             v
                +------------> Stop ingress and producers
                                      |
                                      v
                              Join plugin work and agents
                                      |
                                      v
                              Shutdown core PTYs
                                      |
                                      v
                              Close stores and listeners
```

cmd 持有分阶段生命周期表，不能简单对所有 Close 按逆序调用而提前关库。
失败路径释放已构造资源；正常退出先取消/等待交接及恢复 worker，再停 hybrid，最后释放 conversation/artifacts。
普通用户关闭与 server shutdown 的恢复记录语义继续区分。

禁用模块不执行恢复、schema migration、seed、扫描或 sweep；宿主必要的 runtime 进程清理不等于业务恢复。
files 启用而 chat 禁用时，不打开聊天库来判断 retention，无法证明可删的产物保守保留。
公共 settings 文件可继续承载已存在字段；禁用字段不暴露为可写 UI，更新核心偏好必须保留未启用插件的数据。
插件专属 store 健康贡献只在启用时读取；configured 诊断始终无副作用。

## Risks / Trade-offs

- 同进程插件不能隔离 panic/权限 → 明确只承诺模块化，不承诺沙箱；安全边界仍由核心统一执行。
- 运行时关闭不缩小二进制 → 本次不增加构建 profile，也不以资源大小声称收益。
- 创建、列表与生命周期跨层移动可能破坏并发语义 → 先覆盖已有真实消费者行为，保留 continuity、投递与清理验证。
- chat 对 launch/files 的硬依赖减少自由组合 → 换取现有目标配置、附件归属和恢复不变量；无需新增退化模式。
- 全量配置默认与新安装默认不同 → 区分 missing/[]，config view 显示来源；升级不偷偷关闭现有功能。
- agent 业务与普通终端类型混合 → 移出核心而非保留 hybrid nil receiver 或复制旧实现。
- 在途规格与主规格存在漂移 → 尊重 `unify-session-cli` 和 `share-agent-control-core` 的已实现接口，不把旧 chat new/startSide 重新引回。
- 旧浏览器缓存与深链接可能触发禁用功能 → bootstrap 前不 enterHome；能力归一化后才激活 UI。

## Migration Plan

1. 在同一变更中建立目录、启用解析与类型化契约，保留既有配置默认功能组合；中间提交不作为完成交付。
2. 先抽核心创建、输入与流投影，再将全部业务服务和路由迁至明确装配，移除旧硬依赖与重复分派。
3. 接入浏览器贡献、设置/导航及客户端发现；不保留旧全量装配路径或 API 别名。
4. 使用隔离真实服务验证零插件、单插件、依赖组合及既有完整组合，记录桌面/平板/手机前后截图及交互 GIF。
5. 同步配置示例、协议/架构/持久化与受影响故事；新规格归档时登记 docs/specs.md，不提前把规划写成已实现。
6. 回滚不依赖新数据库格式；保留既有插件数据。回滚到旧二进制前先停服务，并恢复变更前配置，避免旧程序误解新字段。
