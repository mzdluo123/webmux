# 配置、运行与服务管理

## K. 配置、运行与服务管理

- **US-K01 单二进制运行**：作为部署者，我希望下载一个匹配平台的二进制就能前台运行，以便减少安装成本。
  验收要点：
  - 支持 Linux/macOS、amd64/arm64；
  - 浏览器资源内嵌，运行端无需 Node、Go、tmux 或数据库；
  - `webmux` 与 `webmux serve` 均可启动；
  - `--log-file` 以 0600 文件保存 stdout/stderr，旧文件保留为 `.1`，启动方关闭管道后仍可运行。
- **US-K02 初始化私有配置**：作为部署者，我希望自动生成带随机 token 的配置，以便快速准备自己的服务。
  验收要点：
  - `config init` 使用模板与随机 token 创建私有目录和 0600 文件，不向输出打印 token；
  - 默认拒绝覆盖，显式 `--force` 时原文件保留为 `.bak`；
  - 支持 `--listen` 和配置路径；
  - `config listen <地址>` 原子更新配置文件中的 `listen`（文件缺失时按模板与随机 token 初始化，已有文件 `token = ""` 且切到非 loopback 时自动填入随机 token）；
  - `config path` 可显示存在状态或 JSON；
  - 未显式 `--config` 且默认文件缺失时，首次 `serve` / `start` 使用同一模板与 token 生成逻辑自动创建，启动输出说明创建/使用的路径并提示用 `webmux status --json --reveal-token` 查看 token；
  - 并发首次启动通过原子发布只创建一个文件，已有文件不重写；
  - `service install` 与托盘复用同一逻辑。
- **US-K03 检查与覆盖配置**：作为部署者，我希望启动前检查配置并用命令行覆盖地址或 token，以便发现配置错误和临时调整运行环境。
  验收要点：
  - `config check`/`validate` 校验而不启动服务，TOML 语法错误带行列号，可输出 JSON；
  - 项目目录清单 `projects.json` 与克隆账本 `project-clones.json` 的存储状态随 workspace 功能显示；未选择 workspace 时诊断不读取清单。
  - `config show` 只读查看磁盘配置，`--live` 查看已验证运行进程的内存快照（含 `[p2p]` 等网页覆盖后的生效值与各字段来源）；
  - 不可达报错、不回退磁盘，凭据仅输出是否配置；
  - 字段显式默认值仍保留来源且不含 token；
  - 显式指定缺失文件时报错且不会创建，默认文件缺失时 `config show`/`validate` 仍按默认值工作，只有 `serve` / `start` 会自动生成；
  - `--listen`、`--token` 优先且只覆盖本次进程，显式空 token 也能覆盖（非 loopback 地址仍自动生成 token，见 US-A03）；
  - 配置修改需重启生效。
- **US-K04 调整会话与提醒行为**：作为部署者，我希望配置默认目录、缓冲和状态判断，并单独管理启动命令，以便适配自己的 agent 与资源预算。
  验收要点：
  - 支持 `default_cwd`、`scrollback_bytes`、`idle_after_seconds`、`prompt_patterns`、`agent_protocol`、`bell_attention`、`program_status`、`agent_pretrust`，校验正则；
  - `agent_protocol`、`bell_attention`、`program_status` 属于核心 PTY 状态识别，不依赖通知或聊天功能；`agent_pretrust` 与 agent 专属提交延迟由 `launch` 启动集成提供，未选择时不装配；
  - 启动命令由显式 argv（核心）或 `launch` 保存配置提供，原 `shell`/`presets` 不提供内置命令；仅首次启用 `launch` 且文件缺失时写入常见 agent 预设；
  - 选择 `chat` 后，通过 Agent 原生身份读取历史，由内置 Snapcompact 离线生成最多 8 页历史图片；
  - `handoff_token_cap` 默认 16000、范围 1024–64000，约束来源指引与图片的 token 估计之和；附件字节与编码请求单独校验，未知费率明确标注；
  - 提醒集成方式见 [使用指南](../../usage/attention.md)。
- **US-K05 安装后台用户服务**：作为部署者，我希望以当前用户安装并启动后台服务，以便不需要一直开着前台终端。
  验收要点：
  - `service install` 使用 Linux systemd 用户服务或 macOS LaunchAgent，无需 sudo；
  - 引用二进制和配置的绝对路径；
  - 默认配置缺失时与首次 `serve` 共用生成逻辑，创建后重新读取并校验，显式配置必须已存在且有效；
  - 保留必要 PATH 与运行环境。
- **US-K06 预览与更新服务定义**：作为部署者，我希望先预览服务定义，并明确覆盖旧定义，以便检查安装位置与启动参数。
  验收要点：
  - `service install --dry-run` 只打印定义，不写文件或创建配置；
  - 定义输出失败时返回错误；
  - 已有定义须 `--force` 替换并重启；
  - 二进制移动后可重新安装更新路径。
- **US-K07 启停、诊断与卸载**：作为部署者，我希望用统一命令管理后台服务并查看日志，以便维护和排查故障。
  验收要点：
  - 提供 start、stop、restart、status、logs、logs --follow/-f、uninstall；
  - 停止、重启、卸载会结束会话；
  - 卸载保留二进制、配置、日志与状态目录；
  - 服务启用不等于端口已监听，应结合状态和日志诊断。
- **US-K08 保存仓库清单**：作为部署者，我希望指定状态目录并保护其内容，以便重启或升级后仍保留项目登记。
  验收要点：
  - `state_dir` 保存仓库清单、受保护的运行时记录/本机 API 信息及可选 cloudflared 缓存，不持久保存会话或终端输出；
  - 支持 XDG 默认路径；
  - workspace 的内部项目目录清单独立使用 `projects.json` 和生命周期锁；普通目录登记在服务重启后保留，已有仓库以原标识映射且不重写旧清单；可通过项目 API 登记，本层尚无网页入口。
  - workspace 的项目克隆账本与目录清单独立：重启识别未完成/待登记状态并保留目录；可通过克隆操作 API 查询和恢复登记，不重放网络克隆。
  - 各领域 JSON 使用统一私有权限、严格限长解码、持久提交与单实例锁；
  - 领域故障保留原文件并隔离，提交不确定时冻结写入；
  - 仓库或偏好存储失败不阻止直接终端工作。
- **US-K09 可信远程入口与升级**：作为部署者，我希望通过可信 VPN 或 HTTPS 代理访问，并知道升级的影响，以便维护远程终端。
  验收要点：
  - `listen`、`token`、`allowed_origins` 配合部署；
  - `listen` 端口无效、`allowed_origins` 条目不是 `scheme://host[:port]` 的精确小写形式（如带尾斜杠、路径或默认端口）时启动即报错；
  - TLS 由外部代理提供，正确保留 Host 并配置 Origin；
  - `version` 与 `help` 提供版本和命令信息，`version --json` 另含控制契约版本且不读取配置；
  - 替换二进制后需重启，重启结束会话但保留仓库登记与磁盘工作树。
  - 网络 MCP 配置和 URL+Bearer 接入见 [MCP 控制](../../usage/mcp.md#外部客户端网络-url)；代理保留 Host / Origin，VPN 不替代认证，OAuth-only 客户端不在此 profile 支持范围。
  - `[mcp] enabled=false` 并重启撤销入口；回退旧二进制前移除新配置表。MCP 会话不持久化，普通 PTY 不跨重启存活。
- **US-K10 定位设置分区**：作为使用者，我希望通过分区导航快速找到设置项，以便在桌面和手机上减少长页面翻找。
  验收要点：
  - 桌面左侧显示固定分区导航，手机顶部显示可横向滚动的吸顶导航；跨 1024 px 切换时保留当前分区和阅读位置，横条呈现在已聚焦的分区标题上方。
  - 按已选择的功能显示快捷指令、启动配置、网络诊断、公网访问、Tailscale 与通知推送分区；核心外观与阅读、草稿与暂存、安装应用、输出预览不因此消失；
  - 点击或键盘激活滚动到对应分区并转移焦点，滚动时高亮当前分区；
  - Home Screen App 中吸顶导航按实际顶栏高度定位，不遮挡顶栏；
  - 支持地址片段直达和浏览器前进后退；
  - 定位不卸载表单、不丢失尚未保存的输入。
  - 旧片段指向已禁用分区时不加载其组件或访问接口，回到可用设置；保存核心字段不会清除旧功能偏好，直接写入未启用功能字段被拒绝；
- **US-K11 一键安装**：作为部署者，我希望用一条命令安装匹配平台的二进制，并在网络受限时自动尝试镜像，以便不必手动分辨版本、架构和校验步骤。
  验收要点：
  - 提供 POSIX `sh` 脚本，支持 Linux/macOS 的 amd64/arm64；
  - Rosetta 终端优先选择 arm64，Windows 明确引导到 exe/托盘；
  - 默认最新 Release，`WEBMUX_VERSION` 可固定版本，`WEBMUX_INSTALL_DIR` 覆盖目录且不自动 sudo；
  - `WEBMUX_CHANNEL=dev` 从 `dev` Release 当前清单安装匹配的开发版，`WEBMUX_VERSION` 只接受当前开发版，通道与版本矛盾时报错，`dev` Release 自带同一份 `install.sh`；
  - 先 GitHub 直连，再按 `WEBMUX_MIRRORS`（默认 `ghfast.top`、`gh-proxy.com`，空值只用 GitHub）回退，每源有连接、低速与总时限；
  - 未设置 `WEBMUX_SHA256` 且本机有 `minisign` 或支持 Ed25519 的 `openssl` 时，先以对应通道的内置公钥验证同源 `SHA256SUMS.minisig`（覆盖文件内容、可信注释版本及正式版/开发版密钥组隔离）再校验二进制摘要；固定安装 `v0.11.0` 之前的无签名旧版时提示设置 `WEBMUX_SHA256`；
  - 缺少本地验签工具且未设置 `WEBMUX_SHA256` 时，只接受直接从官方 `https://github.com/mzdluo123/webmux` 下载的 `SHA256SUMS`（GitHub 已返回清单而二进制下载中断时允许镜像提供匹配摘要的二进制），拒绝非 GitHub 来源的未验证清单；失败保留旧文件并汇总来源错误，`WEBMUX_SHA256` 可固定摘要；
  - 安装入口使用 Release 资产、jsDelivr 或 gh-proxy 镜像，不依赖 `raw.githubusercontent.com`，并说明镜像可看到 IP 及经镜像拉取脚本本身的信任边界；
  - 脚本只装二进制，不改配置、服务或 shell。
- **US-K12 无服务管理器后台启动**：作为部署者，我希望在容器、WSL 或没有 systemd / launchd 的环境中让 WebMux 脱离当前 shell 运行，并能用客户端停止，以便不必保持前台终端。
  验收要点：
  - `webmux start` 接受与 `serve` 相同的参数，以新会话启动子进程并把 stdout/stderr 写入 `<state_dir>/webmux.log`（0600，旧文件保留为 `.1`）或显式 `--log-file`；启动链接与二维码不含 token，当前 token 仅通过 `webmux status --json --reveal-token` 查看；
  - 等待运行时记录后输出 PID、不含 token 的本机链接与日志路径并退出；
  - 已有实例时幂等打印状态并退出 0；
  - 子进程清除继承的 `WEBMUX_SESSION_ID`、`WEBMUX_LAUNCHER`、`INVOCATION_ID`、`XPC_SERVICE_NAME`，避免被启动方会话清理并保持 `manual` 记录，可用 `webmux stop` 停止；
  - 启动失败输出日志尾部并返回非 0；
  - 生产环境仍推荐 `service install`。
- **US-K13 Linux 安装包**：作为 Debian / Ubuntu 或 Fedora 等发行版的用户，我希望用 apt / dnf 安装、升级和卸载 WebMux，并能一条命令设置开机自启，以便按系统惯例管理它。
  验收要点：
  - Release 提供 amd64、arm64 的 `webmux_<版本>-1_<架构>.deb` 与 `webmux-<版本>-1.<架构>.rpm`，计入 `SHA256SUMS`，无依赖；
  - 包内的 `/usr/bin/webmux`（0755）与同版本 `webmux-linux-<架构>` 相同，另有用户单元 `/usr/lib/systemd/user/webmux.service`（0644）；
  - 自动更新的系统单元 `webmux-update.service` / `webmux-update.timer`（0644，见 US-K14）；
  - 安装脚本只管理更新定时器；
  - 安装、升级、卸载不为任何用户启用、启动或重启 `webmux.service`，运行中的服务升级后到重启才换新版本；
  - 用户执行 `systemctl --user enable --now webmux` 后以默认配置运行（首次启动生成带随机 Token 的配置）并在登录后自启；
  - 没有 `~/.config/systemd/user/webmux.service` 时 `webmux service status|start|stop|restart` 作用于包内单元，`uninstall` 只禁用并停止它且不删除文件，两者都没有时仍提示先 `service install`；
  - `service install` 写入的用户定义优先于包内单元。
- **US-K14 安装包自动更新**：作为用 deb / rpm 安装的部署者，我希望系统每天自动安装经过签名验证的新版本，同时不打断正在运行的终端会话，以便不必手动跟踪发布。
  验收要点：
  - 首次获得定时器时（新安装或从不含定时器的版本升级）在运行 systemd 的系统上启用 `webmux-update.timer`（每日、最多 6 小时随机延迟、错过补执行）并留下标记，无 systemd 时只装文件；
  - 有标记的升级只 `daemon-reload` 并保留管理员的 `disable`，卸载停用定时器并删除标记；
  - 服务以 root、无法访问 `/home` 与 `/root` 运行 `webmux update apply`：仅发布构建、仅 root、单实例锁、仅当 `/usr/bin/webmux` 属于 `webmux` 包，按签名清单下载同格式同架构安装包并用 argv 调用 `dpkg -i` / `rpm -U`，只升级不降级；
  - 联网前检查 `apt-mark hold` 与 dnf versionlock 锁文件（不依赖软件仓库），锁定时跳过，锁定状态无法确认时也按锁定跳过；
  - 检查与下载最长 30 分钟，`dpkg` / `rpm` 一旦开始不被停止信号、超时或 `systemctl stop` 中断（`KillMode=process`）；
  - dpkg 被占用等失败时原安装不变、次日重试，`rpm -U` 等待 rpm 数据库锁，下载的包总被删除；
  - journal 记录版本、来源与结果；
  - 不触碰任何用户的 `webmux.service`。
- **US-K15 检查更新与验证签名**：作为部署者，我希望随时查看是否有新版本，并能独立验证下载的发布文件来自官方，以便在任何安装方式下判断是否升级以及下载是否可信。
  验收要点：
  - `webmux update check` 在任意安装方式下输出当前版本、最新版本、是否有更新，`--json` 输出 `{v,current,latest,available,installMethod,channel}`（读取方把缺少 `channel` 的旧输出按 `stable` 理解），不下载、不改文件，失败时非零；
  - `install.sh` 或手动安装时提示重新运行 `install.sh`，deb/rpm 时提示定时器与 `sudo webmux update apply`；
  - 非发布构建报告“此构建不是发布构建，不支持更新”且不联网；
  - 每个 Release 附带 `SHA256SUMS.minisig`，可信注释 `webmux <版本>`，文档分别公布正式版与开发版公钥，`minisign -Vm` 可手动验证；
  - 更新程序只信任内置公钥，正式版密钥只签 `X.Y.Z`、开发版密钥只签 `X.Y.Z-dev.N`；GitHub 与镜像返回的清单都须验证签名，版本取自签名注释；GitHub 不可用时采用各镜像中最高的已验证版本。
- **US-K16 待生效版本提示**：作为使用者，我希望在服务端可执行文件被更新后得知新版本需要重启才生效、重启会结束会话，以便自己选择合适的时机重启。
  验收要点：
  - 运行中的服务端在状态被查询时（距上次至少 60 秒且文件标识变化）以 argv 运行自身路径的 `version --json` 得到已安装版本，读取失败时下次检查重试；
  - 不同于运行版本时 `webmux status` 增加提示行，`status --json` 与本机状态 API 含 `installedVersion`，`GET /api/config` 含 `pendingUpdate{installedVersion,launcher}`；
  - Web UI 主页顶部与设置页显示已安装版本、重启后生效、重启结束所有会话，并按启动方给出 `systemctl --user restart webmux`、`webmux service restart`、托盘菜单重启或重新运行；
  - 主页提示可按版本关闭（设备本地），设置页始终显示；
  - 不提供重启按钮，服务端从不因此自动重启。
- **US-K17 命令行选择更新通道**：作为 Linux 或 macOS 上的部署者，我希望选择跟随正式版还是开发版，以便在正式发布前尝试 main 上的新构建，并能随时退回正式版而不被降级打断。
  验收要点：
  - `webmux update channel` 显示当前通道，`webmux update channel stable|dev` 需要 root，必要时创建 `/etc/webmux`（0755），以临时文件加改名写入 0644 的 `/etc/webmux/update-channel`，下次检查生效；
  - 文件缺失时按正式版且不警告；内容无法识别、不归 root 所有或组、其他用户可写时按正式版并记录警告；
  - `update check` 与 `update apply` 都按该文件选择通道；开发版通道同时读取 `dev` 与 latest 清单，采用较高的已验证版本；
  - deb/rpm 开发版安装包文件名为 `-dev.N`，包内版本为 `X.Y.Z~dev.N`，排在 `X.Y.Z` 之前；
  - 只升级不降级：从开发版切回正式版且当前开发版高于最新正式版时不安装，说明“当前开发版 X 高于最新正式版 Y，正式版超过它后自动更新”。

- **US-K18 选择本次运行的内置功能（已退役）**：作为部署者，我曾通过 TOML `[features]` 选择本次运行的业务模块；现配置文件侧功能开关已关闭并退役，生产运行固定启用全部 13 项内置功能。
  验收要点：
  - 新生成配置不写入 `[features]` 段，已有 `config.toml` 中遗留的 `[features]`（含空数组或旧 `immersive`）与 `[plugins]` 段在解析时直接忽略，原文件保持不变且不再生成 `.pre-features.bak`；
  - `config show` 字段白名单不再输出 `features.enabled`；
  - 13 项内置模块目录（`launch`、`workspace`、`code`、`files`、`chat`、`notifications`、`quick-commands`、`voice`、`forward`、`cloudflare`、`tailscale`、`p2p`、`mcp`）、硬依赖校验与程序化 `Config.Features` 隔离基建继续保留供内部功能测试使用。
- **US-K19 不启用功能也能工作**：作为只需要终端的使用者，我希望核心模式仍能创建和接续 Shell，以便不依赖仓库或 agent 业务服务。
  验收要点：
  - 浏览器和 CLI 可用显式 argv、cwd、env 创建真实 PTY，保留输入、输出、控制、尺寸、回放、分屏和原生附着；
  - 新建对话框保留自定义命令和直接目录；没有配置、仓库、上传入口，不因缺少功能而要求先创建业务配置；
  - 旧 `configId` 要求明确重选命令，不猜 argv；旧聊天模式、仓库筛选、设置片段与分屏功能偏好不能触发禁用接口或阻止核心会话打开。
- **US-K20 发现实际可用能力**：作为使用者或客户端作者，我希望区分已选择与可用功能，以便不调用未开放入口。
  验收要点：
  - TCP、本机套接字与启用的 P2P 共用 `GET /api/capabilities` v2（`features` 字段、`feature_disabled` 原因码），保留各自认证及来源边界，提供核心 `terminal.pty`、功能选择及独立能力状态；本机客户端过渡期内仍可解码 v1 的 `plugins`/`plugin_disabled` 形状；
  - `enabled` 不代表 `available`；原因区分功能未选择、配置关闭、构建/平台不支持、存储、命令或服务不可用，不泄漏凭据与内部路径；
  - 浏览器依次读取健康、能力、本地动态入口再进入首页；能力读取失败关闭可选功能，不猜全量启用；未选择功能不发 REST/WS、不加载实现 chunk 或启动后台工作；
  - 已选择功能的 chunk 下载失败只影响该功能并给出就地错误，不结束既有 PTY；CLI、托盘与 MCP 同样按实际能力发现入口；
  - 已选择功能的能力从不可用恢复时，已有终端更新相应操作，不重建 PTY、连接所有者或草稿；能力再次不可用时收回相关操作；
  - 功能加载错误可以收起，其他设置仍能编辑；收起不等于恢复模块或改变服务端能力事实；
  - 禁用路由和 WebSocket 在 SPA 回退前返回 404；网络 `/mcp` 只在选择 `mcp` 且 `[mcp].enabled=true` 时开放，仍仅 TCP、owner Bearer。
- **US-K21 禁用后保留数据并有序退出**：作为部署者，我希望关闭某项功能不会改写其旧数据，以便之后重新启用时继续使用。
  验收要点：
  - 未选择功能不打开、加锁、补种、迁移或清扫其存储，不启动对应恢复、探测与 worker；已有文件、数据库与配置原样保留；
  - 共享设置保留未启用功能字段，核心设置修改不覆盖它们，API 不允许修改未启用字段；
  - 仅启用 `files` 而未启用 `chat` 时，对无法确认归属的历史上传保守保留，不借缺少聊天记录删除未知文件；
  - 退出先关闭准入并等待在途创建、恢复、交接与可选生产者退出，再停止 PTY，最后关闭存储；不在关闭后启动新会话或 actor。

依据：

- [CLI 命令](../../../cmd/webmux/main.go)、[启动](../../../cmd/webmux/main.go)、[后台启动](../../../cmd/webmux/start.go)
- [配置](../../../internal/config/config.go)、[配置命令](../../../cmd/webmux/configcmd.go)、[用户服务](../../../internal/service/)
- [安装包配置](../../../deploy/linux/nfpm.yaml)、[包内用户单元](../../../deploy/linux/webmux.service)、[安装包测试](../../../.github/scripts/test-linux-packages.sh)
- [更新命令](../../../cmd/webmux/updatecmd.go)、[签名与更新](../../../internal/update/)、[自动更新单元](../../../deploy/linux/webmux-update.service)、[定时器](../../../deploy/linux/webmux-update.timer)
- [一键安装脚本](../../../install.sh)、[安装脚本测试](../../../.github/scripts/test-install.sh)、[安装与升级说明](../../deployment/install-and-upgrade.md)
- [安装脚本](../../../deploy/linux/postinstall.sh)、[自动更新说明](../../deployment/updates.md)
- [通道文件](../../../internal/update/channel_unix.go)、[通道命令](../../../cmd/webmux/updatecmd.go)
- [已安装版本识别](../../../internal/update/installed.go)、[状态输出](../../../internal/clicmd/status.go)、[待生效提示组件](../../../web/src/components/shared/PendingUpdateNotice.svelte)
- [提示逻辑](../../../web/src/platform/pending-update.ts)、[提示用例](../../../web/test/pending-update.test.ts)
- [部署指南](../../deployment.md)、[设置页](../../../web/src/pages/Settings.svelte)、[设置导航](../../../web/src/components/settings/SettingsNavigation.svelte)
- [导航浏览器回归](../../../web/test/browser/SettingsNavigation.test.ts)。
- [功能选择](../../../internal/config/features.go)、[固定目录与依赖](../../../internal/builtin/catalog.go)、[装配](../../../cmd/webmux/feature_assembly.go)、[启动](../../../cmd/webmux/feature_start.go)。
- [能力状态](../../../cmd/webmux/feature_capabilities.go)、[能力接口](../../../internal/api/capabilities.go)、[路由贡献](../../../cmd/webmux/feature_bind.go)、[共享传输](../../../internal/transport/routes.go)。
- [前端能力宿主](../../../web/src/platform/feature-host.svelte.ts)、[动态入口](../../../web/src/features/registry.svelte.ts)、[核心创建](../../../internal/sessioncreate/service.go)。

配置诊断依据：[来源投影](../../../internal/config/view.go)、[只读与活快照](../../../cmd/webmux/configview.go)、[诊断测试](../../../cmd/webmux/configview_test.go)、[共享存储](../../../internal/statefile/store.go)。
