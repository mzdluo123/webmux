# WebMux User Stories

本文从用户目的出发，描述 WebMux 当前各项功能用来做什么，并给出可观察的验收要点。适用于理解产品、评审变更和检查功能覆盖。

基线核对日期：2026-10-23（之后各次局部核对见[核对记录](#核对记录)）。范围为主分支 `5195154` 的实现，另含 US-N07–US-N08 所述的设置页公网访问说明与运行时隧道状态；OpenSpec 中只有规划、没有实现的内容不视为现有能力。验收要点描述行为契约，不代表本次已重新执行全部测试。

固定多 TCP 与服务端单点准入变更覆盖 US-A09、US-A10，详见[连接与启动](user-stories/access-and-sessions/connection-and-launch.md)。
机器身份独立持久化、仅全新配对、破坏性旧绑定清理及所有者直接删除覆盖 US-A08–US-A10；删除记录与旧访问关闭分别报告，不改变故事数量或既有编号。
Relay 页内配对与宿主授权生命周期覆盖 US-A10：刷新恢复、暂时失败继续查询、未知结果核对及删除/关闭证据分别呈现，编号和总数保持不变。
Relay 有效期上限由签发端限制、接收端仅检查绝对到期时间的调整覆盖 US-A10；编号与总数保持不变。
中转浏览器会话断线暂停、控制面重新授权、原路径重新登录与 12 小时绝对期限覆盖 US-A08、US-A10；编号与总数保持不变。
原生历史图片交接、冻结页集、手机页数编辑与失败阻止覆盖 US-Q20；正文/工具提取覆盖 US-Q19，统一上下文预算说明覆盖 US-K04。故事编号与总数不变，实际验证见最新核对记录。
工作树创建前的上游检查与显式选择覆盖 US-J04、US-J05、US-S07、US-S16、US-Q18、US-Q20；独立创建、会话与交接共享确认，故事编号与数量保持不变。

本次局部核对（2026-11-03）：

- 托盘“复制链接”列出服务端统一报告的远程访问入口（`urls.access`）；
- Cloudflare Tunnel 与 Tailscale 开关收进“公网访问”子菜单，支持一键开启 Tailscale Serve；
- 对应更新 US-M06、US-O09 并新增 US-P08；
- `cmd/webmux`、`internal/tray`、`internal/localclient`、`internal/api` 测试、Windows 托盘交叉编译与 vet、golangci-lint 通过；
- 在真实 WSL 实例经本机套接字读取状态与 Tailscale 检测；
- 未在原生 Windows 托盘上点击验收，也未在真实节点上开启 Serve。

## 阅读约定与产品边界

- **使用者**：通过桌面、手机或平板 浏览器使用服务器上的 Shell、CLI agent 和其他终端程序的人。
- **部署者**：安装、配置和维护自己的 WebMux 服务的人，通常也是使用者。
- **贡献者**：开发、测试和发布 WebMux 的人；相关故事单独列在 L 节。
- WebMux 面向单用户，多设备连接仍共享同一服务用户的权限，不存在团队账号、成员角色或设备间文件隔离。
- **会话**是服务端持有的 PTY 与进程；**工作树**是磁盘上的 Git worktree。浏览器断开不会结束会话，关闭会话不会删除工作树。
- 会话、终端缓冲和发送确认记录不是跨服务重启的持久数据（混合会话持久化运行恢复记录及独立 Conversation / ProviderThread 稳定身份，见 Q 节）；仓库清单持久保存在服务端，草稿、暂存和显示偏好保存在当前浏览器。
- 本文不承诺进程沙箱、公网访问安全、文件编辑、自动 clone、后台定时 fetch、自动安装 agent 或跨服务重启恢复终端。
- 生产运行固定启用全部 13 项内置模块，配置文件侧 `[features]` 选择开关已退役（US-K18 已退役）；各能力仍按运行环境、存储与业务设置报告可用性，程序化功能隔离与核心终端基建见 US-K19–US-K21。
- `launch` 对应保存的启动配置；`workspace` 对应仓库与工作树；`code` 对应代码浏览。Git 工作台要求同时选择 `workspace` 与 `code`，仅选 `workspace` 仍能管理仓库及工作树。
- E 组上传依赖 `files`；Q 组依赖 `chat`（同时显式选择 `launch`、`files`）；G 组快捷指令/语音分别依赖 `quick-commands`/`voice`。
- C 组后台推送依赖 `notifications`；N/P/R 组分别依赖 `cloudflare`/`tailscale`/`forward`；P2P 与 MCP 分别依赖 `p2p`、`mcp`。核心 PTY、普通会话与原生附着不依赖这些功能。
- 故事编号保持稳定。新增功能应补充故事；改变行为时同步修改验收要点与相关测试，不因排序调整而重新编号。

## 功能索引

| 分组 | 功能范围                               | 故事编号      |
| ---- | -------------------------------------- | ------------- |
| A    | 连接、认证、品牌标识与访问边界         | US-A01–US-A10 |
| B    | 创建会话与选择启动位置                 | US-B01–US-B13 |
| C    | 会话总览、搜索与提醒                   | US-C01–US-C18 |
| D    | 终端操作、分屏、续接、多设备与生命周期 | US-D01–US-D15 |
| E    | 剪贴板与文件上传                       | US-E01–US-E07 |
| F    | 草稿编写、保存与发送确认               | US-F01–US-F09 |
| G    | 快捷指令与语音输入                     | US-G01–US-G07 |
| H    | 移动端与慢速网络                       | US-H01–US-H15 |
| I    | 代码工作区、Diff、引用与文件操作       | US-I01–US-I19 |
| J    | 仓库登记与 worktree 管理               | US-J01–US-J29 |
| K    | 配置、升级迁移、功能选择与服务管理     | US-K01–US-K21 |
| L    | 开发、验证与发布支持                   | US-L01–US-L16 |
| M    | 原生终端、CLI 与本机控制面             | US-M01–US-M08 |
| N    | Cloudflare 公网隧道                    | US-N01–US-N08 |
| O    | Windows/WSL 托盘管理                   | US-O01–US-O14 |
| P    | Tailscale Serve / Funnel               | US-P01–US-P08 |
| Q    | 混合会话、归档恢复与永久删除           | US-Q01–US-Q20 |
| R    | 端口转发预览                           | US-R01–US-R04 |
| S    | 脚本与 agent 控制                      | US-S01–US-S25 |

目录共 266 条故事；A 组为 10 条，K 组为 21 条，L 组为 16 条，S 组为 25 条。编号范围包含已保留的历史/退役条目，不表示本次重新验收全部能力。

## 分组文档

各分组的完整故事、验收要点与依据位于以下文件；故事编号与上方索引保持一致。

| 文件                                                                              | 分组               |
| --------------------------------------------------------------------------------- | ------------------ |
| [连接、认证与访问边界](user-stories/access-and-sessions/connection-and-launch.md) | A                  |
| [创建会话与选择启动位置](user-stories/access-and-sessions/launch.md)              | B                  |
| [会话总览、搜索与提醒](user-stories/access-and-sessions/workspace.md)             | C                  |
| [终端、输入与移动端](user-stories/terminal.md)                                    | D–E、G–H           |
| [草稿编写、保存与发送确认](user-stories/terminal/drafts.md)                       | F                  |
| [代码工作区与工作树](user-stories/code-and-worktrees.md)                          | I                  |
| [仓库登记与 worktree 管理](user-stories/code-and-worktrees/worktrees.md)          | J                  |
| [配置、运行与贡献者工具](user-stories/operations-and-tooling.md)                  | K–L                |
| [原生客户端、隧道与托盘](user-stories/clients-network-tray.md)                    | M–P                |
| [终端分屏用户故事](user-stories/terminal-splits.md)                               | D13–D14            |
| [网络压缩与 P2P 直连用户故事](user-stories/terminal-network.md)                   | H10–H12            |
| [头显与 TV 工作区（已退役）](user-stories/terminal-headset-tv.md)                 | H13–H15            |
| [语音输入用户故事](user-stories/terminal-voice.md)                                | G07                |
| [混合会话](user-stories/hybrid-sessions.md)                                       | Q                  |
| [端口转发预览](user-stories/port-forwarding.md)                                   | R                  |
| [脚本与 agent 控制](user-stories/shell-control.md)                                | S                  |
| [功能覆盖核对与后续维护](user-stories/coverage.md)                                | 覆盖索引与维护约定 |

## 核对记录

各次局部核对的记录（对应的故事、已运行的验证、范围限制）按日期存放在以下文件，条目按日期升序。新的核对记录追加到最新的文件；超过文件上限时按“拆分过长文件”的办法按日期再拆。

| 文件                                                                                                   | 日期范围                          |
| ------------------------------------------------------------------------------------------------------ | --------------------------------- |
| [verification-log-2026-10-23-to-26](user-stories/verification-log-2026-10-23-to-26.md)                 | 2026-10-23 至 2026-10-26          |
| [verification-log-2026-10-27-to-31](user-stories/verification-log-2026-10-27-to-31.md)                 | 2026-10-27 至 2026-10-31          |
| [verification-log-2026-11](user-stories/verification-log-2026-11.md)                                   | 2026-11-01 至 2026-11-05          |
| [verification-log-2026-11-06](user-stories/verification-log-2026-11-06.md)                             | 2026-11-06 起                     |
| [verification-log-2026-11-handoff](user-stories/verification-log-2026-11-handoff.md)                   | 2026-11-11 起的交接增强与最终验收 |
| [verification-log-2026-11-shell](user-stories/verification-log-2026-11-shell.md)                       | 2026-11-05 起                     |
| [verification-log-2026-11-archived-history](user-stories/verification-log-2026-11-archived-history.md) | 2026-11-06 起                     |
| [verification-log-2026-11-12](user-stories/verification-log-2026-11-12.md)                             | 2026-11-12 起                     |
| [verification-log-2026-11-git-workbench](user-stories/verification-log-2026-11-git-workbench.md)       | 2026-11-12 起的 Git 工作台核对    |
| [verification-log-2026-11-15](user-stories/verification-log-2026-11-15.md)                             | 2026-11-15 起                     |
| [verification-log-2026-11-17](user-stories/verification-log-2026-11-17.md)                             | 2026-11-17 至 2026-11-18          |
| [verification-log-2026-11-18](user-stories/verification-log-2026-11-18.md)                             | 2026-11-18 起                     |
| [verification-log-2026-11-19](user-stories/verification-log-2026-11-19.md)                             | 2026-11-19 起                     |
