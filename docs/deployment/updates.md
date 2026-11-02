# 自动更新

WebMux 的更新只替换磁盘上的文件，从不自动重启 WebMux：**重启会结束所有终端会话**，由你决定何时重启。

| 安装方式                      | 更新方式                                                                                |
| ----------------------------- | --------------------------------------------------------------------------------------- |
| deb / rpm 安装包              | 系统定时器 `webmux-update.timer` 每天以 root 检查并通过 `dpkg` / `rpm` 安装新版本       |
| Windows 安装程序安装的托盘    | 托盘每 24 小时检查，下载新安装程序后静默升级自身，再把 WSL 中较旧的 webmux 升级到同版本 |
| 便携版托盘                    | 只提示新版本，菜单“下载托盘 <版本>”打开 Release 页面                                    |
| `install.sh`、手动下载、macOS | 不自动替换；`webmux update check` 查看是否有新版本，重新运行 `install.sh` 升级          |

本地 `make build` / `make tray` 构建的程序不是发布构建，不参与更新：`webmux update check` 会报告“此构建不是发布构建，不支持更新”，托盘的“检查更新”不可用。

## 发布签名

每个 Release 附带 `SHA256SUMS.minisig`：用发布私钥对 `SHA256SUMS` 做的 [minisign](https://jedisct1.github.io/minisign/) 签名，可信注释为 `webmux <版本>`。更新程序只信任内置的公钥：

```text
untrusted comment: minisign public key 4AF7A1E8058C841C
RWQchIwF6KH3SuZyvOfqZZAoobboXxcEAyyLB3YptnKdLs7vUvxLHm4e
```

- 来源顺序为 GitHub，然后是 `ghfast.top`、`gh-proxy.com` 镜像。每个来源的 `SHA256SUMS` 都必须通过签名验证，所以镜像只影响能否下载，不能改变内容。
- 签名里的版本号决定清单的版本，安装产物从 `v<该版本>` 标签下载；只安装严格高于当前版本的发布，不会降级。

手动验证下载（任意平台安装 minisign 0.11 及以上）：

```sh
minisign -Vm SHA256SUMS -P RWQchIwF6KH3SuZyvOfqZZAoobboXxcEAyyLB3YptnKdLs7vUvxLHm4e
sha256sum -c --ignore-missing SHA256SUMS
```

第一条输出 `Trusted comment: webmux <版本>`，应与你下载的 Release 一致。

## 检查更新

```sh
webmux update check          # 当前版本、最新版本、是否有可用更新
webmux update check --json   # {"v":1,"current":…,"latest":…,"available":…,"installMethod":"deb|rpm|other"}
```

该命令不下载安装产物、不改写文件；检查失败时退出码非零。

## Linux 安装包的自动更新

deb / rpm 首次安装时启用 `webmux-update.timer`（没有运行 systemd 的容器中只安装文件）。定时器每天触发一次，带最长 6 小时随机延迟，关机错过的会在下次开机后补上。

- 服务 `webmux-update.service` 以 root 运行 `webmux update apply`：确认 `/usr/bin/webmux` 属于 `webmux` 软件包，下载并校验同格式、同架构的安装包，再用 `dpkg -i` 或 `rpm -U` 安装。它不能访问 `/home` 与 `/root`。
- 同一时刻只有一个更新在运行；`dpkg` / `rpm` 被占用（例如你正在 `apt upgrade`）时本次失败，原安装不变，次日重试。
- 查看记录：`journalctl -u webmux-update.service`，其中有当前版本、最新版本、来源与结果。
- 立即更新：`sudo systemctl start webmux-update.service`（或 `sudo webmux update apply`）。

关闭自动更新，之后的升级会保留这一选择：

```sh
sudo systemctl disable --now webmux-update.timer
```

需要代理时为服务添加环境变量：

```sh
sudo systemctl edit webmux-update.service
# [Service]
# Environment=HTTPS_PROXY=http://proxy.example:3128
```

安装、升级与卸载都不会为任何用户启用、启动、停止或重启 `webmux.service`。

## 重启后生效的提示

正在运行的 WebMux 发现自己的可执行文件被换成了其他版本（例如定时器刚升级了软件包），会在以下位置提示“已安装 <版本>，重启后生效，重启会结束所有会话”：

- `webmux status` 增加一行，`webmux status --json` 与本机状态 API 增加 `installedVersion`；
- Web UI 主页顶部与设置页，并按启动方式给出重启方法：systemd 为 `systemctl --user restart webmux`，launchd 为 `webmux service restart`，Windows 托盘为“在托盘菜单中重启”，手动运行为“停止后重新运行 webmux”。

主页上的提示可以关闭，关闭只记在当前设备上，出现更新的版本时会再次显示；设置页中的提示始终保留。

## Windows 托盘

托盘菜单中的“自动更新”默认勾选，对应 `%APPDATA%\WebMux\tray.json` 中的 `autoUpdate`。

- 开启时，托盘启动 2 分钟后检查一次，之后每 24 小时检查一次；“检查更新”随时手动检查。
- 安装版托盘发现新版本后，把 `webmux-setup-windows-<架构>.exe` 下载到 `%LOCALAPPDATA%\WebMux\updates`，校验后以 `/S` 静默运行。安装程序沿用原安装目录与登录启动设置。旧托盘按“仅退出托盘”退出，WebMux 与会话不受影响，转发在新托盘启动后恢复。
- 托盘正在安装 WSL 中的 webmux、启动、停止、重启 WebMux 或显示对话框时，等这些操作结束后再升级。
- 新托盘在状态行显示一次“已更新到 <版本>”。之后它发现 WSL 中的 webmux 较旧时会自动升级该文件，并提示重启 WebMux 后生效。
- 关闭“自动更新”后不再自动检查；手动检查发现新版本时，菜单提供“更新到 <版本>”。

## 密钥轮换（维护者）

发布私钥是 GitHub Actions 密钥 `WEBMUX_RELEASE_SIGNING_KEY`，内容为 `go run ./cmd/internal/releasesign keygen -secret <文件>` 生成的一行 Base64，需要离线备份；缺失时 `make release` 直接失败。

更换密钥分两步：先发布一个同时内置新旧公钥（`internal/update` 的 `releaseKeys`）、仍用旧私钥签名的版本，下一个版本再改用新私钥。私钥丢失后，已安装的版本无法再自动更新，用户需要手动升级一次。
