# 原生客户端、公网隧道与 Tailscale

## M. 原生终端、CLI 与本机控制面

- **US-M01 从本地终端启动**（脚本用法与启动增强见 US-S07）：作为终端使用者，我希望用 `webmux run -- COMMAND ARG...` 创建并附着会话，或引用 Web 端保存的启动配置，以便保留原来的 CLI 工作习惯并随时转到浏览器。
  验收要点：
  - 显式命令属于核心，先读取实际能力，不为普通 `run` 探测启动配置、工作区或聊天接口；`--launch-config` 需 `launch`，混合配置需 `chat`；
  - `--` 后 argv 原样传递；
  - `--launch-config NAME`（`--config-id` 别名）按 Web 端显示的唯一名称选择已保存配置，以服务端保存的 argv 与环境变量创建，名称不存在时以用法错误退出并列出已有名称，且与命令、`--env` 互斥；
  - 默认当前目录，可指定名称与目录；
  - `--detach` 只创建并输出会话 ID；需要入口链接时用 `webmux open ID` 取得不含 token 的本机链接；
  - 默认附着前向 stderr 公告会话 ID 与分离键提示，stdout 保留给会话；
  - 非 TTY 的附着请求在创建前拒绝；
  - 会话进程默认看到 WebMux 的虚拟终端身份（`TERM_PROGRAM=webmux` 及服务版本），不会继承 Kitty 等宿主终端身份，启动配置显式提供的终端变量仍可覆盖默认值。引用勾选了混合模式的配置时创建从终端开始的混合会话（US-Q13）。
- **US-M02 显式传递运行环境**：作为进阶使用者，我希望指定启动目录和必要环境变量，以便让程序使用正确项目与工具环境。
  验收要点：
  - argv、cwd 与 env 在核心模式也可用；工作区关联和新 worktree 仅在 `workspace` 可用时使用，不因未启用功能而改变直接目录或参数；
  - `--cwd` 使用服务器目录规则；
  - Linux/macOS 的 `webmux run -- COMMAND` 默认继承调用方 shell 的环境（跳过 `TERM`、宿主终端身份、`WEBMUX_SESSION_ID`、`PWD`/`OLDPWD` 与不合法名称），重复 `--env NAME=VALUE` 在其上覆盖；
  - Windows 客户端与启动配置不继承调用方环境；
  - 未发送的变量来自服务环境；
  - 普通网页创建与本机 CLI 的环境传递权限按接口约定处理。
- **US-M03 附着与观察**：作为使用者，我希望 `webmux attach ID` 回到已有 PTY，或只观察，以便在手机和原生终端间接续同一任务。
  验收要点：
  - 初次附着完整回放并进入原始终端模式；
  - 默认取得控制并同步尺寸，`--observe` 不输入或改尺寸；
  - 程序已开启明暗报告（DEC 2031）时，取得控制后按本地终端背景色（OSC 11）报告明暗，使程序改用本地终端配色；
  - 会话退出时返回其退出码；
  - 服务未运行返回 3，不自动拉起服务；
  - 结束后恢复控制台状态；
  - 取消、会话退出或断线会中断无输入时的终端读取，`Run` 返回前等待输入、输出与尺寸协程退出，不关闭 stdin，也不终止持久会话；
  - Unix 的 poll/read 遇到信号中断时重试，不误判为终端关闭。混合会话在聊天视图或切换中拒绝附着并以 1 退出；
  - 附着期间被切到聊天视为被动分离，以 0 退出（US-Q13）。
- **US-M04 分离与重新接管**：作为原生终端使用者，我希望浏览器接管后本地按键不会误抢控制，并可明确接管或分离，以便安全切换设备。
  验收要点：
  - 被接管后显示控制提示，丢弃普通键及终端自动应答；
  - Enter 仅接管、不发给会话；
  - Ctrl+] 分离且进程继续运行，返回 0；
  - 兼容 modifyOtherKeys、kitty 与 Windows 编码按键，退出时关闭临时上报模式。
- **US-M05 命令行发现与打开会话**：作为使用者，我希望用 `ls` 列出会话并用 `open ID` 获取入口，以便不经过首页就能找到目标。
  验收要点：
  - 普通活动会话的 `ls`、详情与 `open` 使用核心接口；归档/聊天筛选需 `chat`，仓库筛选需 `workspace`，不为核心列表预取禁用业务数据；
  - `ls` 显示 ID、名称、状态、命令和目录，`--json` 输出原始会话数组；
  - `open` 校验目标并打印不含 token 的本机链接，`--browser` 可打开默认浏览器。`ls` 为混合会话在状态列附加所在视图，`--json` 原样包含 `views`、`view`、`transition` 与 `agent`，不含 `hybrid`（US-Q13）。
  - `ls` 可按标签、父会话与状态筛选，`--state`/`--archived`/`--repo` 列出已归档会话（US-S15），`open` 接受 `self`（US-S01、US-S03）；`run`、`attach`、`ls`、`open` 的 `--help` 写到标准输出并以 0 退出（US-S04）。
- **US-M06 诊断运行实例**：作为部署者，我希望用 `status` 看到健康状态、访问地址和配置漂移，以便确认正在使用哪个实例。
  验收要点：
  - 优先本机 API，必要时使用运行时记录与 TCP 健康检查；
  - 区分运行、未运行、无响应或错误并给出退出码；
  - `--json` 提供版本化结果，只有同时 `--reveal-token` 才揭示 token；
  - 提示 listen/token 配置与运行实例的差异；
  - 经本机 API 时在 `urls.access` 与文本 `Access:` 行列出服务端托管且已就绪的远程访问入口（Cloudflare Tunnel、Tailscale Serve/Funnel），不含 token。
- **US-M07 停止指定实例**：作为部署者，我希望可靠地停止自己启动的服务，以便清理后台运行实例。
  验收要点：
  - `stop` 优先经本机控制面关闭，必要时验证运行记录后退回 SIGTERM 和超时 SIGKILL；
  - 无记录不随意发信号；
  - systemd/launchd 实例默认要求使用服务管理命令，显式 `--force` 可覆盖；
  - 支持超时与 JSON 结果。
- **US-M08 本机桥接与权限边界**：作为本机使用者，我希望 CLI 和 Windows 桥接无需维护网络 token，以便直接访问当前用户的服务。
  验收要点：
  - 本机客户端同样读取 `/api/capabilities` v1；未选择的隧道、Tailscale、P2P 不被当作已初始化服务，不探测其接口，托盘只提供实际可用的控制入口；
  - Unix 套接字验证同用户身份，`bridge` 在标准输入输出间传输；
  - 浏览器网络入口仍要求认证；
  - 会话内进程不能用本机管理 API 读取 token、停止服务或控制隧道，也不能经套接字读取或修改 `/api/tunnel`、`/api/p2p` 及其子路径（403）；
  - 本机状态 `/local/v1/status` 汇总当前隧道、Tailscale 与 P2P 直连运行状态（开关、端口映射与活动直连对端数），不含敏感凭据；
  - 本机客户端忽略成功响应内容时，仍报告响应读取失败；非成功响应统一保留 HTTP 状态码与最多 64 KiB 错误体，供命令处理结构化错误；
  - 套接字不是不同账号间的共享授权机制。

依据：

- [会话命令](../../../internal/clicmd/clicmd.go)、[会话环境](../../../internal/session/env.go)、[会话管理](../../../internal/session/manager.go)
- [服务启动](../../../cmd/webmux/main.go)、[原生附着](../../../internal/attach/)、[控制命令](../../../cmd/webmux/control.go)
- [本机 API](../../../internal/api/local.go)、[运行时记录](../../../internal/runtime/)、[环境与本机请求](../../../cmd/webmux/sessions.go)。
- [本机客户端](../../../internal/localclient/client.go)、[响应处理回归](../../../internal/localclient/response_test.go)、[能力快照](../../../internal/contract/capabilities.go)。

## N. Cloudflare 公网隧道

- **US-N01 临时公网入口**：作为明确选择公网访问的部署者，我希望启用 Quick Tunnel，以便临时从无法接入 VPN 的设备连接自己的终端。
  验收要点：
  - 可在 Web 设置页保存 Quick 模式并确认启用，也可用 `--tunnel` 或 `[tunnel].enabled`；
  - 无需 Cloudflare 账号，获得随机 HTTPS 地址，不承诺固定地址或 SLA；
  - 两种模式均可复制当前公网地址。剪贴板失败时在同一地址区域显示可选中的只读字段，启停和刷新控件保留位置。
  - 未启用时不创建隧道。Web 设置在服务器 `<state_dir>/tunnel-settings.json` 原子保存（目录 0700、文件 0600、独占锁），跨浏览器与重启生效并优先于 TOML 的模式、主机名、凭据及开关；
  - 显式 `--tunnel` 只覆盖当前进程开关，不改写磁盘。
- **US-N02 固定域名隧道**：作为已有 Cloudflare 配置的部署者，我希望使用 Named Tunnel，以便通过预设的公共主机名访问。
  验收要点：
  - Web 页可保存 Named 模式、裸主机名及写入式隧道 token，并在 Cloudflare 配置回源；
  - 也支持 TOML 配置。省略 token 保留已有凭据，首次配置或显式空凭据不可保存 Named；
  - 切回 Quick 清除主机名与凭据；
  - 凭据不回显、不放入 URL 或浏览器持久存储，只显示是否已配置，经环境变量而非命令行传给 cloudflared；
  - 指定空配置避免本机其他 cloudflared 配置干扰。
- **US-N03 隧道认证与扫码**：作为部署者，我希望公网入口始终要求足够强的 token，并能扫码连接，以便明确控制入口凭据。
  验收要点：
  - WebMux 登录 token 与 Cloudflare 隧道 token 不同；
  - 首次启动可生成并持久化 256 位随机登录 token，已有配置缺少登录 token 时生成本次进程的随机值；
  - 运行中启用要求当前登录 token 至少 43 字符，不生成或更换登录 token，条件不足提示修复后重启；
  - 显式 CLI/TOML 隧道启动仍校验登录 token。所有启动链接与二维码均不含 token，避免凭据进入 journal 或日志文件；通过 `webmux status --json --reveal-token` 查看当前 token；
  - 持久化 token 重启保留，仅临时 token 重启更换；托盘复制链接仍可当场生成带 token 片段，并按敏感凭据处理剪贴板与浏览历史。
- **US-N04 获取 cloudflared**：作为部署者，我希望自动定位或下载匹配的 cloudflared，以便不用手工管理隧道依赖。
  验收要点：
  - 按显式路径、PATH、状态缓存定位，不存在时下载固定版本；
  - 支持有序下载镜像，空镜像列表禁用回退；
  - 各来源均校验内置 SHA-256 后执行；
  - 定位或校验错误明确报出；
  - 路径和下载镜像等运维项仍由 TOML 管理，不由 Web 设置覆盖。
- **US-N05 隧道故障隔离**：作为使用者，我希望隧道断开时本机会话仍继续，以便恢复远程入口后继续任务。
  验收要点：
  - 隧道退出立即撤销对应公网来源授权，本地访问和 PTY 保留，不自动重试；
  - Web 保存的启用状态在启动时遇到短登录 token、依赖下载或进程启动失败，只把隧道置为 failed，HTTP/本机 API 继续可用；
  - 设置文件损坏、不可读或锁冲突时保持原文件并禁止公网启动，不回退到 TOML/CLI 开启；
  - 关闭操作即使落盘失败也立即停止当前进程的隧道并撤销授权，返回 503 与 `notPersisted: true`，明确提醒修复存储后再重启，不能声称下次仍关闭；
  - 正常退出关闭隧道而不改持久开关。
- **US-N06 在线切换隧道**：作为浏览器或托盘使用者，我希望服务运行中开启或关闭公网隧道，以便控制入口而不中断会话。
  验收要点：
  - Web 页与本机控制面共用持久化开关，启用前确认，要求已有至少 43 字符登录 token；
  - 成功保存后异步应用，无需重启，重复启用不产生第二个 cloudflared；
  - 更换已启用的模式、主机名或凭据先撤销旧授权并停止旧进程，再启动新配置，迟到事件不能恢复旧入口；
  - 普通保存/启用落盘失败不应用新设置，关闭例外见 US-N05。托盘仅在 `settingsAvailable: true` 时提供 Web 设置入口；
  - 勾选以可选 `enabled` 表示的当前进程期望（含 CLI 覆盖）为准，不等于磁盘开关或 ready，旧服务缺该字段时回退到 starting/ready。
- **US-N07 配置说明与指南**：作为不熟悉 Cloudflare 的使用者，我希望在设置页和文档中按步骤完成 Named Tunnel 配置，以便独立获得固定域名入口。
  验收要点：
  - 在 Quick Tunnel 与 Named Tunnel 之间切换时，表单高度平滑过渡；启用系统「减少动态效果」时直接显示所选表单。
  - 设置入口打开 `/settings/tunnel`，区分 Quick/Named，显示配置来源、凭据是否已配置、启用前提与存储问题；
  - 保存与启用分开，提供 Named 配置步骤、文档链接和写入式凭据框，不读取真实凭据；
  - 恢复配置文件默认值前确认，成功后删除 Web 覆盖并停止当前隧道，不立即按 TOML 自动开启，下次启动才遵循 TOML；
  - 恢复失败不假装成功；
  - 文档覆盖 Windows/WSL、验证、排错与安全。
- **US-N08 查看隧道运行状态**：作为部署者，我希望在设置页看到当前公网隧道的运行状态、模式与主机名，以便确认隧道是否启用时不必查看托盘或服务日志。
  验收要点：
  - 独立显示当前进程期望 `enabled` 与 off/starting/ready/failed，启动失败仍可保留启用期望，不能把它当成可用公网入口；
  - 状态轮询反映托盘和其他设备变化，ready 才显示无 token 公网地址；
  - 只读 `/api/config` 的旧 `tunnel.enabled` 仍仅表示 starting/ready，模式和主机名动态跟随有效设置，推送资格按当前有效 HTTPS 入口（含 Named Tunnel、Tailscale Serve/Funnel）与本机回环判定；
  - GET 设置即使存储损坏仍可读状态及错误，不能启用时给出原因；
  - 接口不返回 Cloudflare 凭据，桌面和移动布局无横向溢出。

依据：

- [隧道控制](../../../cmd/webmux/tunnelctl.go)、[隧道令牌与启动](../../../cmd/webmux/main.go)、[持久设置](../../../internal/tunnelsettings/store.go)
- [设置契约](../../../internal/contract/tunnel.go)、[设置 API](../../../internal/featureapi/tunnel.go)、[设置 API 用例](../../../internal/api/tunnel_test.go)
- [启动与故障用例](../../../cmd/webmux/tunnelsettings_test.go)、[本机权限用例](../../../internal/api/local_test.go)
- [入口与二维码](../../../cmd/webmux/access.go)、[隧道进程与下载](../../../internal/tunnel/)、[公网来源授权](../../../internal/transport/public.go)
- [动态配置](../../../internal/api/config.go)、[推送来源](../../../internal/featureapi/push.go)、[Web 设置页](../../../web/src/pages/TunnelSettingsPage.svelte)
- [设置状态管理](../../../web/src/hooks/settings/tunnel-settings.svelte.ts)、[设置端到端用例](../../../web/test/e2e/tunnel-settings.spec.ts)
- [Named Tunnel 指南](../../cloudflare-tunnel.md)、[部署说明](../../deployment/remote-access.md#公网访问cloudflare-tunnel)。

## P. Tailscale Serve / Funnel

- **US-P01 检测 Tailscale 环境**：作为部署者，我希望只读查看 Tailscale 是否可用、节点 DNS 与证书状态，以便判断能否使用自动 HTTPS 入口。
  验收要点：
  - `GET /api/tailscale` 需要认证，返回 provider（linux/wsl-native/windows/none）、后端状态、节点 DNS、HTTPS 就绪、Funnel 属性、现有入口 host/port/public 与 WSL 网络模式；
  - 只执行 `status`/`serve status`/`funnel status`，短超时、无交互 stdin、缓存并合并并发请求；
  - CLI 缺失、未登录、输出损坏或超时不 panic，也不把缺失字段当作已就绪；
  - 响应不含 token 或其他入口 target。
- **US-P02 托管 Serve**：作为部署者，我希望通过配置让 WebMux 在 tailnet 内提供自动证书的 HTTPS，以便手机等可信设备安全访问。
  验收要点：
  - `[tailscale] mode = "serve"` 在监听开始后以 argv 执行 `tailscale serve --bg --https=<port> http://127.0.0.1:<实际端口>`；
  - 执行前要求 `CertDomains` 包含节点 DNS；
  - 自动端口按 443、8443、10000 选择，显式冲突失败不覆盖；
  - 回读确认唯一 `/` handler 指向当前 target 且 `AllowFunnel` 为假后才登记来源并报告 ready；
  - 停止/失败只移除精确端口，绝不调用 reset。
- **US-P03 显式 Funnel**：作为明确需要公网访问的部署者，我希望显式启用 Funnel 并看到风险，以便在无 VPN 的设备上连接。
  验收要点：
  - `mode = "funnel"` 只允许 443/8443/10000，要求强 Token；
  - 不因检测到 Tailscale 就自动公开；
  - 命令失败把 stderr 映射为 HTTPS 未启用、缺少 funnel 属性、权限不足或端口冲突；
  - 回读 `AllowFunnel[host:port] == true` 后才登记公开来源；
  - 从 Funnel 切回 Serve 或端口变化先关闭旧精确目标，失败时确认不残留并保持 failed；
  - 设置页持续显示公网风险。
- **US-P04 WSL NAT 阻塞**：作为 WSL 使用者，我希望知道为什么不能托管 Windows Tailscale，并看到可行替代方案。
  验收要点：
  - `provider = "windows"` 且 `wslinfo --networking-mode` 为 nat/unknown 时失败关闭，不执行任何 Windows 写命令；
  - 设置页与错误说明 mirrored（`.wslconfig` + `wsl --shutdown` 会结束会话）、WSL 内安装 Tailscale、Windows 托盘转发三种方案；
  - 不自动修改 `.wslconfig`、重启 WSL 或防火墙。
- **US-P05 显式 Windows provider**：作为 WSL 使用者，我希望显式选择 Windows 主机上的 Tailscale，以便在 mirrored 网络中使用 Windows 节点。
  验收要点：
  - `auto` 只检测并优先正在运行的 WSL 原生 Tailscale，永不被用来代管 Windows；
  - 只有显式 `provider = "windows"` 才允许写命令，URL/Host/Origin 使用 Windows 节点 `Self.DNSName`；
  - 非 WSL 配置 windows 启动失败；
  - 权限不足提示在 Windows 管理员终端执行 `tailscale set --operator=<用户>`，不 sudo、不自动提权；
  - WSL 关闭后保留的入口在下一次启动按状态文件对账。
- **US-P06 所有权与失败关闭**：作为部署者，我希望 WebMux 只清理自己创建的入口，并在无法确认时拒绝启动，以便不误改其他应用或暴露旧端口。
  验收要点：
  - `<state_dir>/tailscale-ownership.json`（0600、原子替换、独立锁）记录 provider、DNS、模式、端口、target 与 pending/active；
  - 写命令前先落盘 pending，命令与回读成功后标记 active；
  - `mode = off`、模式/端口/target/provider 变化或状态损坏时先对账，删除前用 `serve status` 复核 provider、DNS/端口与 target；
  - 读取失败则不删除并拒绝启动（错误给出状态文件路径）；
  - 记录端口已无入口或改指其他 target 时判定记录过期、不动该入口并删除记录；
  - 无法移除旧入口则拒绝启动并给出精确的 `serve --https=<port> off` 命令；
  - `serve off` 同时清除 Funnel 标记，重复移除视为成功；
  - 状态文件缺失或损坏但已有入口指向当前监听地址时仍视为所有权未知并拒绝接管，错误给出该入口端口与精确的 `serve --https=<port> off` 命令，供确认废弃后手工移除；
  - 绑定监听失败时关闭本次入口或报告风险；
  - 来源撤销后旧 Host/Origin 立即 403（若在 `allowed_origins` 手工保留了 ts.net 主机名，迁移时需删除），Cloudflare 与 Tailscale 可并存。 旧 tailscale.json 即使与新版记录并存也阻止启动，须先核实外部入口。
- **US-P07 网页修改 Tailscale 设置**：作为部署者，我希望在设置页修改 Tailscale 的模式、端口与 provider 并立即生效，以便不改配置文件、不重启服务就能开启、切换或关闭入口。
  验收要点：
  - `/settings/tailscale` 可选关闭/Serve/Funnel、端口（Serve 留空自动，Funnel 仅自动/443/8443/10000）与 provider（Windows 仅 WSL 显示）；
  - 切换关闭、Serve 或 Funnel 时，表单高度平滑过渡；启用系统「减少动态效果」时直接显示所选表单。
  - `PUT /api/tailscale/settings` 保留认证与 Host/Origin 校验，按 TOML 规则校验后写入 `<state_dir>/tailscale-settings.json`（0600、原子替换、独立锁），之后启动优先于 TOML，`command` 只来自 TOML；
  - 保存后先撤销来源并移除不再符合新设置的本实例入口再应用，`starting` 期间页面轮询，移除失败保持 failed 且不应用新设置；
  - 从非 Funnel 切到 Funnel 前确认公网风险，未确认 400；
  - 登录 Token 少于 43 字符或监听不可托管时 409 且不保存，页面显示原因；
  - 从当前 Tailscale 地址替换入口前提示断线；
  - 恢复配置文件设置前确认，删除覆盖并关闭本实例入口，TOML 下次启动生效；
  - 设置文件损坏或被锁时禁止写入且托管保持关闭；
  - 会话内进程经本机 API 修改返回 403。
- **US-P08 托盘一键开启 Tailscale**：作为 Windows 托盘使用者，我希望在托盘“公网访问”子菜单中一键开启或关闭 Tailscale Serve，以便不打开网页就能得到 tailnet HTTPS 入口。
  验收要点：
  - 托盘把 Cloudflare Tunnel 与 Tailscale 的开关、状态行和各自设置入口收进“公网访问”二级菜单；
  - `/local/v1/status` 的 `tailscale` 提供不触发检测的模式、端口、provider、状态、地址与错误，旧服务端缺少该字段时不显示 Tailscale 项；
  - 勾选跟随保存的模式，failed 仍勾选以便一键关闭；
  - 开启前经本机 API 刷新检测，未安装或未登录、未运行、HTTPS 证书未就绪、WSL 非 mirrored 却只有 Windows 节点、设置不可保存或 Token 过弱时弹窗说明原因并可打开 `/settings/tailscale`；
  - 只开启 Serve（Funnel 仍由网页确认），保留已配置端口；
  - 配置为 `auto` 而只检测到 Windows 节点时先确认，同意后才以 `provider = "windows"` 保存；
  - 保存失败或应用后转为 failed 同样引导到网页，ready 时通知地址；
  - 会话内进程经本机 API 修改仍为 403。

依据：

- [Tailscale 检测与命令](../../../internal/tailscale/)、[状态集合](../../../internal/contract/tailscale.go)、[控制器与对账](../../../cmd/webmux/tailscalectl.go)
- [控制器用例](../../../cmd/webmux/tailscalectl_test.go)、[网页设置存储](../../../internal/tailscalesettings/store.go)、[配置](../../../internal/config/config.go)
- [启动与 Token](../../../cmd/webmux/main.go)、[来源注册表](../../../internal/transport/public.go)
- [状态与设置 API](../../../internal/featureapi/tailscale.go)、[API 用例](../../../internal/api/tailscale_test.go)、[设置页](../../../web/src/pages/TailscaleSettingsPage.svelte)
- [设置表单](../../../web/src/components/settings/TailscaleConfiguration.svelte)、[设置卡片](../../../web/src/components/settings/TailscaleSettings.svelte)、[Tailscale 指南](../../tailscale.md)。
- 托盘一键开启：[托盘菜单视图](../../../internal/tray/menuview.go)、[视图用例](../../../internal/tray/menuview_test.go)、[托盘操作](../../../internal/tray/winui/links.go)、[本机客户端](../../../internal/localclient/client.go)、[本机状态](../../../cmd/webmux/local.go)。
