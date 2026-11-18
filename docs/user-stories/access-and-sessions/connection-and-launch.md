# 连接、认证与启动会话

## A. 连接、认证与访问边界

- **US-A01 连接服务**：作为使用者，我希望在浏览器中连接自己的 WebMux，以便在不安装终端客户端的情况下操作服务器。
  验收要点：
  - 登录页、工作区、设置页与浏览器标签页使用统一的 WebMux 标志，明暗主题下均可辨认；
  - 打开页面后检查服务可用性；
  - 显示连接中、连接失败或可用状态；
  - 未设置 token 的服务可直接进入；
  - 设置页按需加载，显示加载状态，下载失败时提示刷新重试。
- **US-A02 共享 token 登录**：作为使用者，我希望输入并在本机记住共享 token，以便以后方便地访问自己的会话。
  验收要点：
  - 登录页在共享 Token 右侧提供问号提示，默认收起说明；
  - 鼠标悬停、键盘激活或手机点击可查看，指针可移入说明，Escape 或点击外部可关闭；
  - 说明涵盖共享密码用途、初始化生成与保存、配置路径及启动参数覆盖；
  - 配置路径独立成行、窄屏可换行且无横向溢出；
  - 启用认证后，正确 token 才能访问 HTTP API 与 WebSocket；HTTP 只接受 `Authorization: Bearer`，WebSocket 只接受 Bearer 或 token 子协议，`webmux_token` Cookie 与 URL 参数都不能认证；
  - 端口转发凭据 Cookie `webmux_forward` 独立于 token，只能访问 `/forward/` 网关，不能认证 `/api/` 与 `/ws/`，见 US-R01；
  - 当前浏览器保存 token；
  - 错误或变更后的 token 触发重新认证提示；
  - 本机套接字采用同用户身份校验，见 US-M08。
  - 启用网络 MCP 时 `/mcp` 只接受同一 owner Bearer；会话 ID、Cookie 和 URL 不能认证，凭据仍可调用完整 REST 并启动任意 argv，不是工具级权限。
- **US-A03 可信网络访问**：作为部署者，我希望控制监听地址和允许的浏览器来源，以便只从预期的本机或可信 VPN 入口操作终端。
  验收要点：
  - 默认仅监听本机回环（127.0.0.1:7681），局域网或可信 VPN 访问可改为 0.0.0.0:7681；
  - 未显式指定配置且默认文件缺失时，首次启动生成并保存 256 位随机 token，非隧道启动不打印该 token，可经本机状态显式查询；
  - 已有配置的 token 为空且监听地址不是 loopback 时，每次启动生成临时随机 token；启动链接与二维码不携带 token，通过 `webmux status --json --reveal-token` 查看；
  - 只有 loopback 且未启用隧道时允许无 token；
  - 隧道启动链接与二维码同样不含 token，持久 token 不因重启失效；
  - 监听 loopback 时校验请求 Host（防止无 token 下的 DNS 重绑定），非 loopback 监听（强制要求 token）接受任意合法 Host（如局域网主机名或 Tailscale MagicDNS），并始终校验 Origin（同源、allowed_origins 或受控公网来源）；
  - 认证和来源检查同时覆盖终端与文件、仓库、代码接口。
  - 网络 MCP 默认关闭；`[mcp] enabled=true` 必须已有非空根 token，空值在自动生成启动 Token 前失败，loopback 也不豁免。
  - 所有 MCP 方法保留 Host / Origin 校验，仅在 TCP 精确 `/mcp` 开放，不暴露 Unix / P2P 端点；详见 US-S25。
- **US-A04 查看服务与能力**：作为部署者，我希望知道服务是否在线、会话数量及 Git/仓库能力是否可用，以便定位连接或环境问题。
  验收要点：
  - 健康检查返回可用状态和会话数；
  - 配置接口提供默认目录、主机名和工作区能力，不返回 token；
  - 首页展示连接与能力异常。
  - 侧栏底部的主机状态入口可点击查看主机名、连接状态、当前传输路径（`WebSocket` 或含候选类型与 IP 族的 `P2P 直连`，见 US-H12）、往返延时（直连生效时优先采用 P2P 控制通道 RTT，否则采用健康检查 RTT）、系统负载（1/5/15 分钟与核心数）、可用/总内存和已用/总交换空间；
  - 往返延时高于 200 毫秒，或 1 分钟负载高于核心数的 50% 时，状态灯变黄并在提示中说明原因；平台无法提供负载、内存或交换空间（含主机未配置交换空间）时对应行不显示。
- **US-A05 脚本访问**：作为进阶使用者，我希望通过 REST 和 WebSocket 操作会话，以便接入自己的脚本或客户端。
  验收要点：
  - 提供会话、配置、目录、上传、仓库与代码（含文件操作）接口；
  - 列表和终端分别提供实时流；
  - 遵循相同认证、错误和生命周期边界，详见 [协议](../../protocol.md)。
- **US-A06 带凭据链接登录**：作为使用者，我希望打开带 token 片段的访问链接就能登录，以便从终端或托盘把入口带到手机。
  验收要点：
  - 前端读取并保存 `#token=` 后立即移除地址栏和当前历史记录中的片段；
  - 片段不发送到服务器；
  - WebSocket 认证通过子协议携带 token，不放入请求 URL 或 Cookie；
  - 链接仍是敏感入口。
- **US-A07 切换界面语言**：作为在中文或英文环境下工作的开发者，我希望网页界面默认跟随系统语言，也能随时在登录页或设置页手动切换简体中文与 English，以便用熟悉的语言浏览工作区、终端和设置而不中断当前会话。
  验收要点：
  - 网页前端内置简体中文（`zh-CN`）与英语（`en`）消息目录，编译与运行期间不请求外部翻译服务或 CDN；
  - 语言偏好支持“跟随系统 / Follow system”（默认）、`简体中文`、`English` 三项：显式选择保存在当前浏览器的 `webmux.locale`，选择“跟随系统”移除该键，认证失效清理凭据时保留本机语言偏好，且不写入服务端 `/api/settings`；
  - “跟随系统”按浏览器 `navigator.languages` 顺序匹配首个 `zh*` 或 `en*` 标签，无中文的非空其他语言标签回退为 `en`，空列表回退为 `zh-CN`；系统触发 `languagechange` 或同源标签页修改 `webmux.locale` 时自动同步；
  - 未登录门禁页（`ConnectionGate`）与“设置 → 外观与阅读”均提供语言切换，服务端设置尚未就绪时也不阻塞语言切换；
  - 切换语言后，`<html lang>`、浏览器标签页标题、全部已挂载界面文案（含终端 OSC 8 外部链接确认对话框）、前端契约校验提示及本地化日期数字格式立即无刷新更新，不依赖浏览器自身弹窗语言，不重新加载页面、不断开现有终端或聊天连接，已输入的表单草稿保留。
- **US-A10 配对并启用官方中转**：作为宿主使用者，我希望确认公网风险后通过 GitHub 配对自己的主机，以便获得固定 HTTPS 入口而不复制长期 Token 到手机。
  验收要点：
  - Relay 配对、设置与 WSS 基线属于核心，默认关闭；配对由用户显式发起，启用前须确认公网 Shell 风险，保留足够长的本机恢复 Token；
  - 官方 HTTPS 控制地址、控制受众与 WSS 传输内置于开源代码，省略 `[relay]` 即可开始配对；显式 TOML 字段覆盖对应默认值，未覆盖字段保留默认值，不内置私钥、OAuth Secret 或用户凭据；
  - 设备配对由宿主持有单个活动尝试，可取消、到期和重新发起；v2 协议必须提供有效机器 ID，账号已有绑定一律冲突；
  - 创建防重复，浏览器刷新、切标签与导航返回恢复原用户码和有效期；离开页面保持宿主查询，进程重启丢失授权上下文时明确报告中断；
  - 机器 ID 独立保存于 `relay-machine.json`，不由硬件或密钥派生，解绑和丢失 Relay 设置不删除；损坏、锁冲突或初始化落盘失败阻止新配对，不静默换 ID；
  - 未配对主区展示用途、信任说明和「使用 GitHub 配对」；授权中原位展示用户码、复制、GitHub 入口、失效倒计时、等待说明与取消，倒计时仅表示验证码有效期；
  - 完成后显示账号与公网开关，配对成功保持关闭；启用前说明公网 Shell 风险及 Relay 可读取转发内容，就绪后可复制或打开地址；
  - 网络、上游及限流失败保留同一尝试和取消，宿主按间隔、Retry-After 与退避继续查询；明确拒绝或过期清理待配对密钥，由用户重新开始；
  - 创建、启停、重连及删除的响应未知时先核对宿主操作结果，不盲目重发；异常处可检查状态，核对成功后清除过期错误；
  - 删除绑定位于次级危险操作区；记录删除即清除身份并允许新配对，旧直连关闭未确认时单独警告，旧地址失效与新配对取得新地址均明确说明；
  - 公开控制面可经 CDN；宿主使用 Go 系统 CA（可追加 RootCA）、域名、有效期和 TLS 1.3，不额外固定控制 SPKI；分配 WSS 的 pin 与 TURN 服务身份校验不变；
  - 本机只接受 v3 绑定状态，不迁移 v1/v2；旧格式阻止 Relay 连接，破坏性升级先备份清理旧绑定状态，保留独立机器 ID；
  - 切换界面语言时，配对状态、有效期与已打开的风险确认保持不变；文案与已保留的授权错误立即更新，不重新发起配对或自动确认公网访问。
  - 「连接诊断」按连接健康、分配和会话候选分组，保留角色数量、端点来源、可达性、TURN、TCP、节点及版本；主流程仅显示账号和当前操作相关的状态或阻塞；
  - 关闭立即停止连接，保存失败仍保持关闭；状态文件损坏或锁冲突不能恢复旧授权或自动重新暴露公网；
  - 控制角色断开使派生访问失效，批量角色断开只降级文件业务，不将文件挤入交互连接；
  - 有效期上限由签发端限制，接收端只检查是否到期，几秒时钟偏差不会因超出未来上界误拒绝新质询或分配；控制消息签发有效期为 60 秒，心跳每 10 秒发送、失联撤权仍为 30 秒，签名、单次 nonce、请求关联、受众及代际校验保持生效；
  - 连接器使用 `webmux-relay.v2`，默认一条独立管理、两条交互与四条 bulk WSS/TCP；宿主固定槽位并行认证、同代际退避补齐，配置数量为 1–16，每 TCP 无四流业务限制；
  - 辅助认证遇到过期质询或暂时代际冲突时按原槽位退避重试；鉴权拒绝使连接失败并保留权限边界。管理连接存活且交互、bulk 池均可用时才显示 WSS 2/2；交互池丢失显示交互连接不可用，保留管理连接所授权的登录会话与公共地址。
  - Relay 单点执行普通流准入，每账号最多 128 个等待者、最长 15 秒；过载返回 429，宿主清理旧流期间不因重复计数误拒绝新流，已发送请求不重放；
  - 未选择或裁剪 P2P 时仍能使用 WSS，明确显示交互优化不可用，不偷偷加载 Pion 或启动 P2P I/O。
- **US-A08 GitHub 登录自己的主机**：作为跨设备使用者，我希望在手机登录同一个 GitHub 账号并打开自己的在线主机，以便免输长期 Token，同时保留账号与主机隔离。
  验收要点：
  - 后台仅列出当前数字账号拥有的主机，GitHub 名称只作显示；无邀请、跨账号、撤销或离线访问不能取得授权；
  - 后台「Open WebMux」在新标签页进入主机，原标签页保留主机列表；首屏与状态刷新后的入口行为一致，新页面不能通过 `window.opener` 控制原页面；
  - 固定回调、浏览器绑定 state 与 PKCE、一次性短期 code 共同保护跳转；兑换并发只能成功一次，不重放未知结果；
  - GitHub 回调兼容可选 `iss`，提供时须精确匹配 GitHub issuer；重复、未知及错误 issuer 参数仍拒绝，不绕过浏览器事务 Cookie、state 或 PKCE；
  - 宿主签发 Secure、HttpOnly、SameSite=Lax、限定公共 Origin 的绝对期限 Cookie；不在 URL、日志或网页设置中暴露长期 Token 或秘密；
  - 浏览器登录绝对有效期最多 12 小时；暂时 control 断线立即暂停访问并关闭派生连接，恢复后由控制面重新验证原后台会话、邀请、所有者及绑定，原 Cookie 才能继续使用，到期时间保持不变；
  - 暂时重连、登录过期、授权撤销与直连 Token 无效分别呈现；重连期间保留已挂载页面，左下角状态指示提供再次检查和官方登录入口，过期、撤销与未登录以门禁页提供官方登录入口，校验同源应用返回路径后回到原终端页面，不自动重放输入或业务写请求；
  - 断线期间注销、撤销、解绑、换钥、移出邀请名单及明确关闭 Relay 都阻止旧会话恢复；宿主重启清空内存登录记录。
  - Cookie 写请求验证精确 Origin 和 CSRF，WebSocket 校验 Origin；设备密钥、本机会话内进程或长期 Token 不能单独签发浏览器 TURN 凭据；
  - 退出和撤销关闭相关 HTTP、WebSocket、P2P 和转发访问，但保留 PTY 进程；不能将节点断连、超时或新代际 ACK 冒充旧代际全部访问已关闭；
  - 所有者可在账号控制台确认「Delete machine」，请求校验登录、邀请、Origin、CSRF、归属和绑定代际；确认记录删除即从列表移除，旧访问关闭未确认时明确警告，不等待离线旧宿主 ACK；
  - 删除后旧地址失效，全新账号授权生成不同标识和地址；迟到删除不能撤销新绑定，机器 ID 不能用于跨账号覆盖或删除；
  - 整个绑定或连接代际撤销时，先拒绝新访问并关闭派生访问，再发送精确关联的 ACK；管理流半关闭后等待对端读完并关闭，最迟到原撤销消息期限结束，不恢复授权或重放请求；
  - 端口转发仅作为 WebMux 内部入口，不提供任意网站的独立 GitHub 访问网关。
- **US-A09 管理中转节点与授权**：作为中转运营者，我希望经受限 CLI 集中管理节点、邀请与秘密，以便分开部署 WSS 和 TURN，而不让普通登录账号获得运营权限。
  验收要点：
  - 独立控制面持久化 PostgreSQL；管理 CLI 经同用户 Unix socket 调用管理 API，不开放运营网页或公网管理端口；
  - 配对和删除保留事务互斥、单次消费、邀请和代际检查；破坏性升级清理旧绑定、分配、派生授权及配对状态，保留账号、邀请、节点和密钥配置；
  - 首次连接前预置节点 CA 签发的客户端证书及匹配私钥，控制面同时校验证书链与管理员登记的节点 SPKI，不从首次连接学习身份；
  - 支持 WSS-only、TURN-only 和双能力节点；按用途发现地址，禁止向 TURN-only 分配 WSS 流量，也不自动改变既有宿主 Origin；
  - 配置按版本和摘要校验并区分 desired、实际 applied、pending-restart；运行中的中断变更须批准精确版本及组件，失败保留最后有效状态；
  - 秘密以部署主密钥加密保存，经认证通道按节点和用途交付；冷启动控制面不可达时不恢复旧秘密或授权，运行中失联有界撤权；
  - 每数据角色最多 16 条物理连接；普通流额度由 Relay 聚合计算，管理流及其带宽独立，bulk 留出交互余量；接收窗口按每连接限制，运营者按连接数量预留内存；
  - 原版 coturn 使用账号 allocation 数、每会话带宽及节点容量限制，不宣称共享账号带宽桶、累计流量额度或凭据到期立即清除 allocation；
  - Docker 使用 `--init`，代理监督并回收 coturn；TURN 故障不重启健康 WSS，容器升级仍会中断存量连接。
  - 服务端镜像记录固定源码与共享协议版本；本次按运营策略禁用服务端远程 CI，以隔离本地 PostgreSQL、真实客户端和节点验证为依据，客户端仍须完整 CI 通过；不把构建或镜像发布等同于生产联合验收。
  - Dokku 分离控制面、数据节点和不暴露公网的 PostgreSQL；节点私网直连控制面，保留 CA、SPKI 与 mTLS，不跟随公开控制 CDN 放宽；
  - 公网 HTTPS 上游模板来自镜像 `/app/nginx.conf.sigil`，保留 Host、WebSocket 与 Origin/CSRF，不改全局 Nginx 或占用 UDP 443；
  - 服务证书持久 P-256 同 key 续签；节点 CA 签发私钥离线，运行容器仅最小私密挂载；宿主既有 BBR/cake 策略不由容器更改；
  - 公共文档只记录授权公开的地址、客户端接入前提与真实上线阶段；管理员配置和秘密交付留在受限服务端指南，不能以 CI、应用创建或计划端口冒充生产可用，见[部署记录](../../deployment/official-relay.md)。

依据：

- [统一品牌标识](../../../web/src/components/shell/Brand.svelte)、[品牌图标](../../../web/public/icon.svg)、[浏览器图标入口](../../../web/index.html)
- [应用入口](../../../web/src/App.svelte)、[Token 说明](../../../web/src/components/shared/TokenHelp.svelte)、[配置命令](../../../cmd/webmux/configcmd.go)
- [语言控制器](../../../web/src/lib/locale.ts)、[语言切换组件](../../../web/src/components/shared/LocaleChoice.svelte)、[消息目录](../../../web/messages/en.json)
- [启动与 token 生成](../../../cmd/webmux/main.go)、[API 路由与安全检查](../../../internal/api/router.go)、[Host 校验](../../../internal/api/host.go)
- [Relay 公共授权协议](../../../protocol/relay/access.go)、[节点配置及身份协议](../../../protocol/relay/node.go)、[撤销与流边界](../../../protocol/relay/stream.go)
- [Relay 配对设置](../../../web/src/components/settings/RelaySettings.svelte)、[宿主授权流程](../../../cmd/webmux/relay_enrollment.go)、[配对规格](../../../openspec/specs/official-relay-pairing/spec.md)
- [安全说明](../../../SECURITY.md)。

## B. 创建会话与选择启动位置

- **US-B01 启动 Shell 或 agent**：作为使用者，我希望通过自定义启动配置或显式命令运行 Shell、Open Code、Claude Code、Codex、omp 等程序，以便快速开始终端工作或遥控 agent。
  验收要点：
  - 显式 argv、cwd 与环境变量属于核心能力；保存的配置与预设需 `launch`，仓库位置需 `workspace`，混合模式需 `chat`，附件及临时文件引用需 `files`。关闭这些功能不妨碍创建普通 PTY（US-K19）；
  - 启用 workspace.projects 时，全局新建会话先进入可搜索的项目列表，选定后显示项目与运行设置；明确仓库、工作树和分屏上下文直接进入设置，缺少项目能力时保留“位置”与“运行”的直接创建方式；
  - 选定项目后，键盘焦点进入可见的工作位置、运行或命令控件，不停留在隐藏的项目搜索框；
  - 启用 `launch` 时提供保存的启动配置与末尾的“自定义命令…”，首次启用且存储缺失时才写入常见 agent 预设，没有不可修改的内置预设；
  - Claude Code、Codex CLI、Antigravity CLI、Open Code、Qoder、QoderCN、Kimi、Devin 与 omp 的预置配置默认自动批准工具调用：
    - Claude Code `--dangerously-skip-permissions`；
    - Codex CLI `--dangerously-bypass-approvals-and-sandbox`；
    - Antigravity CLI `--dangerously-skip-permissions`；
    - Open Code `--auto`；
    - Qoder `--dangerously-skip-permissions`；
    - QoderCN `--dangerously-skip-permissions`；
    - Kimi `--yolo`；
    - Devin `--permission-mode dangerous`；
    - omp `--auto-approve`；
  - Cursor、Copilot、Qwen、CodeBuddy 使用各自默认授权行为；
  - 十二款 CLI 的启动命令见 [接入说明](../../cli-agents.md)；
  - Claude Code、Codex CLI、Antigravity CLI、Open Code、omp、Qoder、QoderCN、Kimi 与 Devin 预置配置默认为混合模式（US-Q01）；
  - 命令选择器可用键盘操作（Enter 打开、方向键与搜索、Esc 只关闭选择器），选中行和每个选项同时显示图标、名称与命令，长命令保留完整提示；
  - 保存的配置按 ID 启动，临时命令按完整 argv 启动，均创建真实 PTY；
  - Open Code 可通过“加载 shell 启动文件”保存为 Bash（`~/.bashrc`）或 Zsh（`~/.zshrc`）包装，新安装按服务端 `$SHELL` 预选，其他 shell 回退为不包装或手动选择，不自动安装程序；
  - 勾选了混合模式的配置可选择从终端或聊天开始（US-Q02）；
  - 首页与工作树统一使用“新建会话”；会话页经“打开面板 → 分屏 → 新建会话…”复用同一对话框，打开创建表单前关闭侧栏；
  - 无论哪种启动方式，对话框都有可选的“用户指令”，创建后自动发送：
    - 聊天端作为第一条消息；终端端若所选混合模式 agent 已支持命令行初始提示参数（且启动命令本身未带初始提示），通过首次 TUI 启动参数直接传入（不写入 `SessionInfo.Cmd` 或持久化记录），尚未迁移的 agent、不支持该消息形式的边界情况与自定义命令仍在程序输出安静后输入并自动回车；
    - 未填会话名称的终端启动以该指令为会话名（US-B07）；
    - 位置属于已登记仓库且该仓库设置了初始化指令时，另有“使用初始化指令”（从聊天开始时默认勾选，从终端开始时默认不勾选），指令先于用户指令发送（模板规则见 US-J12）；
    - 引用无效、临时文件夹不可用等发送前即可发现的问题使创建失败并保留输入，不留下半启动的会话；
  - 更换项目保留运行配置、自定义命令、名称与用户指令，重置项目专属分支和工作树字段；用户指令中的 Enter 保持换行；
  - 底部汇总“将在 X 运行 Y”，项目与默认值齐全时可直接创建；会话名称收在“高级：会话名称”中；
  - 创建成功后按入口进入终端或加入当前布局。
- **US-B02 自定义命令与参数**：作为进阶使用者，我希望填写完整命令及参数，或选用已保存的配置，以便运行自己的 CLI 工作流。
  验收要点：
  - `launch` 未选择时只提供自定义命令，不读取配置列表；浏览器遗留 `configId` 不能转换成猜测命令或另一配置，必须显式选择并填写新命令；
  - 在启动配置与“自定义命令…”之间切换时保留手动输入和已选配置，但只提交当前方式；
  - 空手动命令不能误用保存配置；
  - 正确保留引号、空参数与参数边界，不进行 shell 展开；
  - 未闭合引号或空可执行程序阻止提交；
  - `configId` 与非空 `cmd`、`env` 互斥；
  - 临时命令不落盘，保存配置不可用或仍在加载时可改选“自定义命令…”。
- **US-B03 选择直接目录**：作为使用者，我希望直接指定任意可访问目录并复用最近使用的位置，以便启动普通项目而不必先登记 Git 仓库。
  验收要点：
  - `workspace` 未选择时仍可输入或浏览直接目录、使用最近目录与默认目录；不读取仓库或工作树清单，不显示其位置选项，也不按旧仓库偏好切换项目；
  - 启用项目能力时通过项目列表或“本地文件夹”明确选择目录，普通项目不显示 Git 或额外工作目录字段；关闭项目能力时保留直接目录输入、浏览与最近目录；
  - 从首页的“新建会话”打开时范围收窄：选中仓库只列该仓库的工作树与“新建工作树…”；选中工作树（或点工作树行的“新建会话”）只列该工作树与“从当前工作树分支新建…”，起始分支默认为该工作树的分支；选中“全部仓库”、“直接会话”时先选项目，分屏保持源目录并可显式更换项目；
  - 明确上下文优先：工作树入口或分屏源目录 > 主页选中仓库的可用主工作区（否则其他可用工作树）；
  - 全局项目流程等待明确选择，不以最近目录替代项目；兼容的直接创建路径才按最近目录、服务默认目录依次预填；
  - 主页选中仓库但没有可用工作区时保留仓库并要求选择，不回退到其他项目；对话框内改选位置不改变主页选中的仓库；
  - 保留最近 10 个启动目录，并在成功创建后记住最近一次启动方案（启动配置及混合起始端，或临时命令），下次打开直接填入，对所有位置通用；
  - 记住的配置已删除或不可用时显示“选择启动方式”并禁止提交，不隐式启动另一配置；创建失败不覆盖记忆；
  - 分屏以源会话的完整启动目录覆盖最近目录，目录未改时保留其工作区归属。
- **US-B04 浏览目录**：作为使用者，我希望逐层浏览和补全服务器目录，以便在手机上也能准确选中启动位置。
  验收要点：
  - 支持路径筛选、主目录、上一级、显示隐藏目录；
  - 支持方向键、Enter/Tab 进入和 Alt+↑ 返回；
  - 显示读取错误并能选择当前目录。
- **US-B05 创建缺失目录**：作为使用者，我希望在目录不存在时确认创建再启动，以便直接为新任务准备工作位置。
  验收要点：
  - 直接目录启动失败且目录不存在时才提示创建；
  - 确认后创建目录并重试；
  - 不因此登记仓库。
- **US-B06 在已有或新工作树启动**：作为使用者，我希望从已有 worktree 启动，或先创建 worktree 再启动 agent，以便在指定的代码副本中工作。
  验收要点：
  - 项目流程的“工作位置”只列当前项目的已有工作树（显示分支与路径）与“新建工作树…”；不包含其他项目或任意目录；直接创建兼容入口保留“位置”选择器；
  - 不可用仓库、不可用或 bare 工作树显示但禁用；
  - 新工作树在位置行下方展开：“创建新分支 / 使用已有分支”、起始分支与新分支名称，起始提交与存放目录收在“高级：起始提交与存放目录”，位置行摘要显示“新分支 ← 起始分支”；改选其他位置再选回同一仓库的“新建工作树…”时保留已填写内容；
  - 新工作树成功后可启动会话或仅保留工作树；
  - agent 启动失败时保留新工作树与分支，位置自动变为该工作树，重试不重复创建；改选其他位置后仍可在列表中选回它；
  - 侧栏仓库菜单的“新建工作树…”默认只创建工作树，勾选“创建后启动会话”才显示命令并启动；未勾选时改选已有工作树或目录会禁用“创建工作树”并提示，不会无操作地关闭；
  - 启动配置不可用不阻止单独创建工作树。
- **US-B07 命名会话**：作为使用者，我希望给新会话填写名称，以便在多个相似终端中识别任务。
  验收要点：
  - 创建名称优先于首条提示标题与程序标题（终端 OSC 0/2、聊天端 ACP `session_info_update`）；
  - 未命名且从终端视图启动、填写了用户指令时，以该指令为名（折叠空白，最多 80 个字符，超出加“…”；不含初始化指令，去掉 `{{dir}}`、`{{webmux}}` 引用，普通 Shell 命令同样适用），之后程序标题不再覆盖；名称输入框的占位提示“留空则用首条提示，没有则跟随终端标题”；
  - 没有用户指令（含只有初始化指令）时使用程序标题，再回退到目录名；聊天端从首条消息取临时标题，见 US-Q02；
  - 首页、终端、聊天页头与浏览器标签使用会话标题；终端或 agent 标题中的 Nerd Font 私有区图标在首页、侧栏、会话行和页头等界面沿用内置图标的字形显示。
- **US-B08 管理启动配置**：作为使用者，我希望把常用的命令、参数和环境变量保存为服务端启动配置，以便在任何设备上一键选用而不必重复输入。
  验收要点：
  - 本故事的存储、预设与设置入口仅在选择 `launch` 时存在；未选择时不打开、锁定、初始化或补写配置文件。混合开关需 `chat`，不因禁用聊天而清除已有混合字段；
  - 已选择的聊天或通知编辑器加载失败时，修改名称等其他字段仍保留已读取的混合模式与提醒策略，不把缺少编辑器当成用户清空选项；
  - CLI 在只选择 `launch` 时可新增并实际运行普通配置，也可修改名称、环境和预览；省略未启用功能拥有的写入字段，保留已存混合与提醒选项。显式聊天开关不因省略而伪装成功；
  - 设置页的启动配置以不带边框的行列表呈现（行高不低于 44px）：图标、名称、一行摘要（终端或混合会话、环境变量数、预览行数；完整命令只在编辑对话框中显示），整行点击编辑，行尾“⋯”菜单提供编辑与删除；
  - 设置页可新建、编辑、删除自定义启动配置，并选择自动匹配、通用终端或 LobeHub 品牌图标；配置最多 100 条且文件总大小不超过 4 MiB，超出时保存被拒绝并提示精简（400），其余配置与功能不受影响；
  - 图标随配置持久保存并在设置列表、新建会话选择器中显示，缺少图标的旧配置按可识别的直接命令或标准 Bash/Zsh 包装自动匹配，不改写原配置；
  - “加载 shell 启动文件”复选框将普通命令安全包裹为所选 Bash 或 Zsh 的交互式 argv，保留空参数、空格和字面量 shell 字符；
  - 切换“加载 shell 启动文件”时，下方字段保持原位；关掉后 Tab 跳过 shell 选择，并保留命令与环境变量；
  - 重新编辑还原普通命令并保持勾选与所选 shell，保存不重复包裹，取消勾选恢复直接执行；
  - 新建时按服务端只读 `defaultShell` 预选，空值回退 Bash；
  - 包装参数计入 argv 上限；
  - 可为每条配置选择跟随全局默认或单独设置输出预览行数与跳过末尾行数；
  - 可为每条配置选择跟随全局默认或单独设置“完成与介入提醒”：
    - 支持设置提醒范围（`all`/`urgent`/`off`）、四类信号源开关（`OSC 7501`、Warp Agent 事件、`OSC 9·777` 通知、BEL 响铃）、最短回合耗时（`0–300` 秒）与完成确认延迟（`0–60` 秒）；
    - 编辑器显式说明信号源优先级（`OSC 7501 > Agent 事件 > 桌面通知 > 终端响铃`），配置列表摘要显示“仅需介入”“提醒已关闭”或“自定义提醒”；
  - 新安装首次启动时自动写入 Shell、十一款 CLI agent 与 omp 共 13 条带图标的启动配置（完整列表见 [接入说明](../../cli-agents.md)），其中 omp 附带 `PI_FORCE_IMAGE_PROTOCOL=kitty` 以启用 US-D12 图片显示；
    - Shell 为通用终端，agent 使用本地打包的 LobeHub 图标（Antigravity CLI 为 `antigravity`，同时保留旧 `geminicli` 兼容渲染），omp 使用 Pi；
    - agent 预设按服务端 `$SHELL` 经“加载 shell 启动文件”包裹（Bash 或 Zsh，未设置时 Bash，其他 shell 直接执行）；
    - 各命令的授权参数见接入说明，Claude Code、Codex CLI、Antigravity CLI、Open Code、omp、Qoder、QoderCN 与 Kimi 为混合模式，可编辑或删除；
  - 文件已存在时启动服务（包括用户编辑或删空后）不再自动补写或覆盖，但可在设置页「新建启动配置」旁的「添加系统预设」菜单中补回单个或全部缺失的系统预设（已有同名配置时菜单项置灰并标记“已存在”，不覆盖用户已修改的配置）；
  - 配置跨重启与设备共享，环境变量值默认遮挡；
  - 按配置 ID 启动，修改或删除配置不影响运行中会话的命令与环境，会话信息保留配置 ID 供首页解析预览；
  - 存储不可用时显示原因，临时命令仍可用。编辑器可勾选混合模式，可用性由服务端判定并显示原因，列表显示“混合”标记（US-Q01）。
- **US-B09 启动 agent 时不卡在“信任此文件夹”**：作为使用者，我希望 Claude Code、Codex、GitHub Copilot CLI、Antigravity CLI、Qoder、QoderCN、Kimi 和 Devin 在新目录启动时不停在信任对话框，以便无人看着的会话也能直接开始工作。
  验收要点：
  - 选择 `launch` 时默认开启 `agent_pretrust`；新建会话的命令被识别为这八款 CLI（直接执行、绝对路径、版本化二进制名或 Bash/Zsh 启动文件包装）时，先把启动目录记入该 CLI 自己的配置，或为 Kimi 注入进程级目录信任，再启动进程；
  - 未选择 `launch` 时，普通 argv 仍可运行，但不装配信任预写或 agent 专属提交延迟，`agent_pretrust = true` 不会隐式开启启动集成；
  - 写入位置优先跟随会话环境变量，其次是服务进程的同名变量，最后回退到主目录下的默认位置：
    - Claude Code：`CLAUDE_CONFIG_DIR` 或 `~/.claude.json`；
    - Codex：`CODEX_HOME` 或 `~/.codex/config.toml`；
    - GitHub Copilot CLI：`COPILOT_HOME` 或 `~/.copilot/config.json`；
    - Antigravity CLI：固定写主目录下的 `~/.gemini/antigravity-cli/settings.json`（`trustedWorkspaces`）；
    - Qoder 与 QoderCN：分别跟随 `QODER_CONFIG_DIR`（默认 `~/.qoder/settings.json`）与 `QODERCN_CONFIG_DIR`（默认 `~/.qoder-cn/settings.json`），写入 `permissions.trustDirectories`（不跟随命令行 `--config-dir`）；
    - Devin：`XDG_DATA_HOME`（默认 `~/.local/share`）下的 `devin/cli/trusted_workspaces.json`，写入 `trusted_paths`；
  - Kimi 不写持久配置，只在会话环境补充 `KIMI_CODE_TRUST_WORKSPACE=1`；恢复 TUI 与 ACP 继承同一环境，会话或服务已显式设置该变量（包括 falsy 值）时不覆盖；
  - 已有配置的其他内容与权限保持不变；已经信任时不写；Codex 保留注释且不改写用户显式设为 `untrusted` 的目录；
  - Claude Code、Copilot、Antigravity CLI、Qoder 与 QoderCN 的配置文件缺失、或无法解析时不创建、不改写；Codex 只在 `~/.codex` 已存在时创建 `config.toml`；Devin 会创建缺失的目录与文件，内容为仅含 `trusted_paths` 的最小 JSON，已有文件的其他字段保持不变；
  - 主目录、其上级目录与文件系统根永不写入或注入；目录先解析符号链接；
  - 任何失败只记录日志，会话照常启动；`agent_pretrust = false` 时不写任何文件，也不注入 Kimi trust 环境；
  - 其他 CLI 与已运行会话不受影响，说明与限制见 [CLI agent 接入](../../cli-agents.md#文件夹信任预写)。

- **US-B10 打开或创建本地项目**：作为使用者，我希望先登记项目再启动会话，以便复用服务端的普通文件夹或 Git 仓库。
  验收要点：
  - 选择 workspace 后提供项目列表、已有目录登记、父目录浏览和新建目录接口；不需要 files、launch 或 chat；
  - 普通目录无需 Git；新建默认只创建目录，显式开启初始化才创建 main 分支的空 Git 仓库，不要求作者身份或生成初始提交；
  - 相同规范路径去重，Git 子目录归一到已登记仓库；不会覆盖既有目标或修改原仓库初始化指令；
  - 创建后登记失败时保留目录并返回恢复路径，可再次登记；认证和 Origin 校验覆盖所有项目接口；
  - workspace.projects 单独报告目录能力，Git 不可用不阻断普通目录，未选择 workspace 时接口不挂载；
  - 全局新建会话以项目列表开始，可搜索名称与路径；无项目时显示来源列表，支持新建与本地文件夹；操作成功进入实际会话设置，重复提交禁用，错误保留输入；
  - 返回来源列表保留各来源草稿；Esc 逐层返回并恢复搜索焦点，根层关闭。

- **US-B11 幂等克隆远程 Git 项目**：作为使用者，我希望远程克隆在断线后继续并可恢复登记，以便创建项目时不重复下载或覆盖文件。
  验收要点：
  - workspace.clone 可用时，POST /api/projects/clone 接受 requestId、url、path，先持久记录再返回 202；同一请求与参数返回同一操作，参数不符返回 409；
  - 允许不含凭据/查询参数的 HTTPS、SSH 和 user@host:path；拒绝本地及 ext 协议、控制字符和选项输入，不执行 shell 拼接命令；
  - 两个网络克隆并发，其余排队；浏览器离开不取消任务，服务停止取消并等待整个子进程组；
  - 只在自有临时目录克隆，原子发布绝不覆盖已有目标，即使它是空目录、文件或符号链接；失败只清理本操作创建的临时目录，替换过的目录拒绝递归删除；
  - GET /api/projects/operations/{id} 可重连查询结果；克隆成功但登记失败保留目标，通过 POST /api/projects/operations/{id}/register 单独重试，不再次克隆；
  - Git URL 来源建议未填写的目标目录但保留手动修改；显示克隆进度，刷新或重新打开后恢复跟踪，登记失败可重试；显式选择其他项目会停止本浏览器观察，迟到结果不覆盖新选择；
  - 公开错误不包含 Git stdout/stderr 或服务端凭据；禁用 workspace 时相关路由不挂载。

- **US-B12 通过服务端 GitHub 身份选择仓库**：作为使用者，我希望浏览和搜索服务用户有权访问的仓库，以便选择私有或协作项目并复用现有登录。
  验收要点：
  - GET /api/projects/github 返回服务端登录名、owner/name、描述、私有状态和 HTTPS 克隆地址；默认列出自己的与协作仓库，搜索支持可访问仓库；
  - GitHub 来源可搜索、翻页、选择仓库与目标目录；未安装或未登录时仍可进入配置说明，刷新后重读能力，不探测不可用接口；
  - 每页 20 条，页码 1–50；搜索限 200 字节且拒绝控制字符，参数独立传给 gh，不执行 shell；
  - 缺少 gh、身份不可用、权限或网络失败返回 available=false 与可读原因，仓库数组仍为 []；失败不阻断普通目录；
  - 只在显式查询时验证身份与 gh 克隆语法（需要支持 --no-upstream 的 2.88+ 版本）；workspace.github 能力仅检查所需命令与存储，不启动登录或修改认证配置；
  - source=github 的克隆使用服务端 gh 认证，来源限 github.com 的 HTTPS 仓库地址；source 参与幂等身份，普通 Git URL 克隆保持原行为；
  - 请求最长 20 秒，输出有界，取消会终止整个进程组；公开响应不包含 gh 原始输出或凭据；禁用 workspace 时接口不挂载。

- **US-B13 查看本机 Agent 工具与检查可用更新**：作为使用者与部署者，我希望在设置页或 CLI 查看本机已安装的 CLI agent 及外部 ACP 连接器、解析路径、当前版本号与可用更新，并一键复制安装或升级命令，以便无需先开终端逐个排查环境。
  验收要点：
  - 依赖 `launch` 内置功能；禁用时不构造扫描服务，`/api/agent-tools` 与 `/api/agent-tools/check` 经认证与 Origin 校验后返回 404，设置页隐藏 `#agent-tools` 分区与导航且不发请求，`webmux agents` 按 `launch.configs` 能力报功能未开启（退出码 1）；
  - 本机扫描复用服务端交互式启动 Shell（`bash -ic` 或 `zsh -ic`，其他回退进程 `PATH`）单次批量定位候选路径与启动后 `PATH`（5 秒、输出上限 64 KiB），在独立进程组并发执行有界超时（3 秒、输出上限 4 KiB）的 `--version` 采集并做内存 TTL 缓存，`?refresh=1` 或 `--refresh` 强制重扫；
  - 打开 `/settings#agent-tools` 或请求 `GET /api/agent-tools` 时零外发网络请求，仅合并未过期的内存远端版本缓存；
  - 仅在点击“检查更新”或执行 `POST /api/agent-tools/check` / `webmux agents --check` 时，才对已安装且具公开包源的工具及连接器查只读 HTTPS 上游（npm 主源失败回退 `registry.npmmirror.com`、PyPI JSON、GitHub Releases），重定向限白名单 HTTPS 域名且不带鉴权头；
  - 上游 JSON 响应上限为 1 MiB，支持含完整发行历史的 Kimi PyPI 元数据；超过上限时该项返回 `check_failed`；
  - 单个上游失败只将该条标记为 `check_failed` 并保留其余结果；无公开稳定包源的自更新二进制（如 Antigravity CLI、Cursor CLI、Devin）明确返回 `source_unavailable`（“由工具自管理更新”），不猜测版本；
  - 已安装的混合模式 agent 或连接器版本号低于档案 `verifiedVersion` 时标记 `belowVerified` 兼容性提示；外部 ACP 连接器（`claude-agent-acp`、`codex-acp`）随主工具以内嵌子卡片或 CLI 缩进子行展示独立路径、版本、更新状态与安装/升级命令；
  - 设置页 `#agent-tools` 紧接在 `#launch-configs` 下方，默认展开已安装项，未安装工具默认收起在可展开折叠区；仅提供可复制的安装/升级命令与官方文档链接，绝不在服务端后台自动执行安装或升级；
  - `webmux agents [--check] [--refresh] [--all] [--json]` 默认列出已安装工具与连接器（`--all` 包含未安装项），`--json` 输出带 `"v": 1` 的 `AgentToolsV1` 对象。

依据：

- [启动图标](../../../web/src/components/settings/LaunchConfigIcon.svelte)、[LobeHub 图标来源与许可](../../../web/src/icons/lobehub/README.md)
- [Agent 工具目录](../../../internal/agentcatalog/catalog.go)、[本机扫描与更新检查](../../../internal/agentcheck/service.go)、[Agent 工具接口](../../../internal/featureapi/agent_tools.go)
- [CLI agents 命令](../../../internal/clicmd/agents.go)、[设置页 Agent 工具卡片](../../../web/src/components/settings/AgentToolsSettings.svelte)、[Agent 工具单行](../../../web/src/components/settings/AgentToolRow.svelte)
- [新建会话](../../../web/src/components/sessions/NewSessionDialog.svelte)、[位置选择](../../../web/src/components/workspace/LocationPicker.svelte)、[命令选择](../../../web/src/components/sessions/LaunchPicker.svelte)、[默认值解析](../../../web/src/domains/sessions/new-session-defaults.ts)
- [启动配置编辑](../../../web/src/components/settings/LaunchConfigDialog.svelte)、[系统预设菜单](../../../web/src/components/settings/LaunchPresetMenu.svelte)、[启动配置存储](../../../internal/launch/store.go)、[首次启动预设](../../../internal/launch/presets.go)
- [启动配置接口](../../../internal/featureapi/launch.go)、[CLI agent 接入](../../cli-agents.md)
- [信任预写](../../../internal/agenttrust/agenttrust.go)、[会话钩子](../../../internal/session/manager.go)
- [会话标题来源](../../../internal/hybrid/actor.go)、[核心生命周期](../../../internal/session/lifecycle.go)、[聊天页头](../../../web/src/hooks/sessions/hybrid.svelte.ts)
- [目录选择](../../../web/src/components/workspace/DirectoryPicker.svelte)、[命令解析](../../../internal/session/run_command.go)、[核心创建编排](../../../internal/sessioncreate/service.go)。
- [可选创建端口](../../../internal/sessioncreate/types.go)、[工作区位置贡献](../../../internal/workspace/create_location.go)、[功能创建入口](../../../web/src/features/create.ts)。

来源补充：[统一创建回归](../../../web/test/e2e/session-creation.spec.ts)。

项目来源：[项目 API](../../protocol/http-projects.md)、[本地项目服务](../../../internal/workspace/project_local.go)、[项目接口](../../../internal/featureapi/project.go)、[项目 wire 回归](../../../internal/api/project_test.go)。

远程克隆来源：[克隆服务](../../../internal/workspace/project_clone.go)、[克隆执行](../../../internal/workspace/project_clone_worker.go)、[克隆回归](../../../internal/workspace/project_clone_test.go)。

GitHub 来源：[发现服务](../../../internal/workspace/project_github.go)、[受控进程回归](../../../internal/workspace/project_github_test.go)、[接口契约回归](../../../internal/api/project_github_test.go)。

项目来源界面（US-B10–US-B12）：[Project picker Storybook](../../../web/stories/ProjectPicker.stories.ts) 展示项目列表、四种来源、配置降级及登记失败。
生产入口通过 workspace 创建贡献接入；[项目会话组件回归](../../../web/test/browser/ProjectSessionDialog.test.ts) 覆盖项目切换、上下文与能力降级。
