# 安装与升级

部署 WebMux 的运行要求、安装方式、升级与持久化升级诊断。

## 运行要求

- Linux 或 macOS（amd64 / arm64）。
- 一个可执行的 shell，以及打算使用的 CLI agent（Claude Code、Codex CLI 等）。
- 可选：Git，用于仓库与 worktree 管理（需支持 `git worktree list --porcelain -z`）。

运行端不需要 Node.js、Go、tmux 或数据库。例外：Codex CLI 与 Claude Code 的混合模式（聊天 ↔ 终端）分别需要第三方适配器 codex-acp、claude-agent-acp；它们运行在 Node 上，Claude 连接器要求 Node.js 22 或更高版本，见 [混合模式连接器](configuration.md#混合模式连接器)。

## 安装

Linux/macOS 推荐使用仓库根目录的 [`install.sh`](../../install.sh)。

- 依赖：POSIX `sh`、`curl`、`uname`、`mktemp`、`awk`，以及 `sha256sum` 或 `shasum`（任一即可）；验签优先使用 `minisign`，未安装时使用支持 Ed25519 的 `openssl`（如 Linux 自带 OpenSSL 1.1.1+ / 3.x）。
- 流程：自动识别 Linux/macOS 与 amd64/arm64（Apple Silicon 的 Rosetta 终端优先选择 arm64），验证 `SHA256SUMS.minisig` 发布签名后再下载对应二进制并校验 SHA-256，通过后才原子替换目标文件。
- 默认安装到 `~/.local/bin/webmux`，不需要 sudo，也不会执行 `config init`、`service install` 或改写 shell 配置。

安装脚本本身不依赖 `raw.githubusercontent.com`。任选一个可访问的入口：

```sh
# GitHub 直连；install.sh 随 Release 一起发布
curl -fsSL https://github.com/mzdluo123/webmux/releases/latest/download/install.sh | sh

# 国内网络：ghfast.top 代理同一 Release 资产
curl -fsSL https://ghfast.top/https://github.com/mzdluo123/webmux/releases/latest/download/install.sh | sh

# jsDelivr 读取仓库中的脚本；脚本随后仍从 GitHub 或镜像下载二进制
curl -fsSL https://cdn.jsdelivr.net/gh/mzdluo123/webmux@main/install.sh | sh
```

| 变量                 | 默认值                                    | 说明                                                                                                                                                  |
| -------------------- | ----------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| `WEBMUX_VERSION`     | 最新 Release                              | 指定 `x.y.z`（可带 `v` 前缀，从 `releases/download/v...` 下载；`v0.11.0` 之前的旧版无签名，固定安装需搭配 `WEBMUX_SHA256`）或当前开发版 `x.y.z-dev.N` |
| `WEBMUX_CHANNEL`     | `stable`                                  | `dev` 安装 `dev` Release 中的当前开发版；只影响本次安装，不写配置                                                                                     |
| `WEBMUX_INSTALL_DIR` | `~/.local/bin`                            | 目标目录；不可写时明确报错，不自动 sudo                                                                                                               |
| `WEBMUX_MIRRORS`     | `https://ghfast.top https://gh-proxy.com` | 空格分隔的 gh-proxy 前缀；空值表示只用 GitHub                                                                                                         |
| `WEBMUX_SHA256`      | 空                                        | 固定二进制 SHA-256，跳过 `SHA256SUMS` 与签名验证（也用于固定安装 `v0.11.0` 之前未附带 `SHA256SUMS.minisig` 的旧版）                                   |
| `WEBMUX_BASE_URL`    | `https://github.com/mzdluo123/webmux`     | GitHub 仓库地址，主要用于测试和自建镜像                                                                                                               |

```sh
curl -fsSL https://github.com/mzdluo123/webmux/releases/latest/download/install.sh \
  | WEBMUX_VERSION=0.16.0 WEBMUX_INSTALL_DIR="$HOME/bin" sh
```

### 安装开发版

开发版发布在预发布 Release `dev` 中（见[自动更新](updates.md#更新通道)），它自带同一份 `install.sh`：

```sh
curl -fsSL https://github.com/mzdluo123/webmux/releases/download/dev/install.sh | WEBMUX_CHANNEL=dev sh

# 国内网络
curl -fsSL https://ghfast.top/https://github.com/mzdluo123/webmux/releases/download/dev/install.sh | WEBMUX_CHANNEL=dev sh
```

- 脚本读取 `dev` Release 的 `SHA256SUMS` 与 `SHA256SUMS.minisig`，以内置开发版公钥验证签名与版本后，安装匹配的 `webmux-<x.y.z-dev.N>-<系统>-<架构>`。
- `WEBMUX_VERSION=x.y.z-dev.N` 隐含开发版，且只接受 `dev` Release 当前清单中的版本；其他版本报错并给出当前开发版版本。
- `WEBMUX_CHANNEL=dev` 搭配 `x.y.z`，或 `WEBMUX_CHANNEL=stable` 搭配开发版版本时报错退出。
- 要让之后的 `webmux update check` 也跟随开发版，执行 `sudo webmux update channel dev`。

下载顺序为 GitHub 直连，然后依次尝试镜像（地址为 `<前缀>/https://github.com/...`），每个来源都有连接、低速和总时限。

- 未设置 `WEBMUX_SHA256` 且本机具备 `minisign` 或支持 Ed25519 的 `openssl` 时，每个来源都须返回有效的 `SHA256SUMS.minisig`（匹配内置正式版或开发版公钥及可信注释版本），验证通过后才按清单校验二进制；失败时保留原有二进制、清理临时文件，并汇总各来源的错误。
  - 未设置 `WEBMUX_VERSION` 且 GitHub 不可达时，安装首个通过签名校验的镜像所提供的已签名版本（`v<签名版本>`）；需要限定精确版本时请显式设置 `WEBMUX_VERSION`。
  - `v0.11.0` 之前的早期版本未发布 `SHA256SUMS.minisig`，固定安装这些版本时脚本提示设置 `WEBMUX_SHA256` 校验二进制摘要。
- 本机既无 `minisign` 也无支持 Ed25519 的 `openssl`（如 macOS 默认自带的 LibreSSL）且未设置 `WEBMUX_SHA256` 时，脚本只接受直接从 `https://github.com/mzdluo123/webmux` 下载的 `SHA256SUMS`。
  - 若 GitHub 已返回清单而二进制下载失败，镜像可仅提供匹配该摘要的二进制；若 GitHub 清单也不可达，脚本拒绝来自非 GitHub 来源的未验证清单，安装 `minisign`、支持 Ed25519 的 OpenSSL 或设置 `WEBMUX_SHA256` 后方可经镜像安装。
- 镜像是第三方服务，能看到请求者 IP；直接经镜像 URL 拉取 `install.sh` 执行（`curl https://ghfast.top/.../install.sh | sh`）会信任该镜像返回的脚本内容，高保证场景请从 GitHub 获取脚本或先核对脚本摘要再运行。

脚本只安装二进制。完成后按提示运行 `webmux`（默认监听 `127.0.0.1:7681`）。

- 需要后台服务时再执行 `webmux service install`，首次启动会自动生成配置与 Token。
- 新生成配置默认启用全部 13 项内置功能。要调整默认组合，请 [选择对应功能](configuration.md#内置功能选择) 并重启服务。
- 重复执行脚本会升级二进制；正在运行的服务需手动重启，重启会结束现有会话。

### 手动下载

也可以不用脚本，手动下载并校验 Release 资产：

- 服务端：`webmux-{linux,darwin}-{amd64,arm64}`。
- Windows 托盘：`webmux-tray-windows-{amd64,arm64}.exe`；控制台客户端：`webmux-windows-{amd64,arm64}.exe`，见 [Windows（WSL）部署](windows-wsl.md)。

```sh
curl -LO https://github.com/mzdluo123/webmux/releases/latest/download/webmux-linux-amd64
curl -LO https://github.com/mzdluo123/webmux/releases/latest/download/SHA256SUMS
sha256sum --check --ignore-missing SHA256SUMS   # macOS: shasum -a 256 -c --ignore-missing SHA256SUMS
install -m 755 webmux-linux-amd64 ~/.local/bin/webmux
```

### Linux 安装包（deb / rpm）

Release 中提供 amd64、arm64 的 deb 与 rpm 包，二进制与同一 Release 的 `webmux-linux-<架构>` 相同，安装需要 sudo，之后由 apt / dnf 升级与卸载。

```sh
# Debian / Ubuntu（arm64 换成 _arm64.deb）
sudo apt install ./webmux_0.2.0-1_amd64.deb
# Fedora / RHEL / openSUSE（arm64 换成 .aarch64.rpm）
sudo dnf install ./webmux-0.2.0-1.x86_64.rpm
```

- 包内有 `/usr/bin/webmux`、systemd 用户单元 `/usr/lib/systemd/user/webmux.service` 与自动更新的系统单元 `webmux-update.service` / `webmux-update.timer`，没有依赖；第一次装上带定时器的版本时启用该定时器，见[自动更新](updates.md)。
- 安装、升级、卸载都不会为任何用户启用、启动或重启服务。需要开机自启的用户执行一次 `systemctl --user enable --now webmux`；首次启动会在 `~/.config/webmux/config.toml` 生成带随机 Token 的配置。
- 升级后正在运行的服务继续使用旧进程，执行 `systemctl --user restart webmux`（或 `webmux service restart`）后生效，重启会结束现有会话。
- `webmux service status|start|stop|restart|logs` 同样作用于包内单元；`webmux service uninstall` 只禁用并停止它，删除请卸载软件包。也可以改用 `webmux service install`，它写入的 `~/.config/systemd/user/webmux.service` 优先于包内单元。

Windows 不使用 `install.sh` 或这些安装包：请使用 `webmux-setup-windows-*.exe` 安装托盘，或下载 `webmux-windows-*.exe` / 独立托盘程序，或按 [Windows（WSL）](windows-wsl.md) 在 WSL 中运行 `install.sh` 或安装 deb / rpm。

## 升级

替换二进制后重启服务才会加载新版本，**重启会结束所有终端会话**；仓库清单与 worktree 保留，混合会话可在首页所属工作树的“已归档”中恢复（排队消息不保留）。WebMux 会在状态与 Web UI 中提示已安装但尚未生效的版本。

从 v0.16.0 升级时，启动会自动将配置中的 `[plugins]` 迁移为 `[features]`，保留原功能选择；原文备份到配置路径加 `.pre-features.bak`。新旧两表并存时停止启动，需先整理配置，详见[改名迁移](configuration.md#内置功能选择)。

能力快照自 `version: 2` 起改名（`features` 字段），服务端不再发送 v1；旧版本地客户端（`webmux` CLI、托盘、Windows 控制台）连接 v2 服务端会按“不支持的能力响应”关闭可选入口，升级时请与服务器同步更新本机二进制。新版客户端仍可解码旧服务端的 v1 快照，先升级客户端再升级服务端不受影响。

deb / rpm 与安装版托盘的自动更新、`webmux update check` 与发布签名验证见[自动更新](updates.md)。

## 持久化升级与诊断

完整路径、锁与故障策略见 [持久化清单](../persistence.md)。新版统一使用数据文件名加 `.lock`，包含新增 `settings.json` 与 `settings.json.lock`。升级前停止旧进程并备份，不支持新旧版本共用状态目录。旧 Tailscale 所有权文件须先核实其外部入口；不能仅删除记录后继续暴露服务。

`config show --json` 查看磁盘配置，`config show --live --json` 查看运行进程实际采用值。输出包含字段来源与脱敏的存储健康；离线检查不创建或修复文件。推送文件损坏保留原位并禁用推送，修复后重启；不会自动丢弃订阅并生成新密钥。
