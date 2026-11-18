# 原生客户端、CLI 与本机控制面

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
