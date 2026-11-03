# Proposal

## Why

自动更新目前只有正式版一个来源：只在推送 `v*` 标签时发布，客户端只读 `releases/latest`，版本号只接受 `MAJOR.MINOR.PATCH`。维护者和少数尝鲜用户想在每次合入 main 后就能自动拿到新构建，在正式发布前发现问题，又不能让正式版用户收到未经发布流程的构建。

## What Changes

- 新增开发版通道：main 上的提交通过 CI 后自动构建并发布开发版，版本号为“最近正式标签的补丁号加 1”加 `-dev.<距该标签的提交数>`，例如 v0.10.2 之后的第 2 个提交为 `0.10.3-dev.2`。
- 开发版只发布到一个固定的预发布 Release `dev`：产物名带版本号，先上传新产物，再替换签名清单，最后删除更早的产物，只保留当前与上一次构建。不会累积 Release 或标签，也不会成为 `latest`。
- 开发版使用独立的签名密钥。正式版客户端只信任正式版密钥、只接受不带后缀的版本；开发版密钥只能签开发版版本号。
- 安装可以选择通道，默认正式版：
  - Linux 安装包：`webmux update channel stable|dev`，写入 root 拥有的 `/etc/webmux/update-channel`；
  - Windows 托盘：设置项 `updateChannel` 与菜单“更新通道”；
  - `install.sh`：`WEBMUX_CHANNEL=dev` 安装开发版二进制。
- 开发版通道同时读取开发版与正式版清单，取其中最高的已验证版本；正式版发布后开发版用户自动升到正式版。切换通道只升级不降级。
- 版本比较、deb/rpm 版本号、托盘与 WSL 服务端的版本对应判定都支持 `-dev.N`。
- `webmux update check --json` 增加可选字段 `channel`。

## Capabilities

### New Capabilities

- `update-channels`：正式版与开发版通道的定义、各安装方式的通道选择与持久化、切换通道的升级规则。

### Modified Capabilities

- `signed-release-updates`：版本号格式支持 `-dev.N`；按通道分组的签名密钥；按通道读取清单与下载产物；预发布版本的比较规则；检查命令输出通道。
- `release-distribution`：新增开发版发布流程、产物命名、保留策略与独立签名。
- `linux-package-auto-update`：开发版安装包的文件名与 deb/rpm 版本号。
- `windows-tray-wsl-install`：开发版托盘从 `dev` Release 下载同一次构建的服务端。
- `windows-tray-server-compat`：版本对应判定支持 `-dev.N`。
- `windows-tray-self-update`：便携版托盘的“下载托盘”按版本打开正式版或开发版的 Release 页面。

## Impact

- 代码：`internal/update`（版本解析与比较、通道、密钥分组、清单与产物路径）、`cmd/webmux/updatecmd.go`（`channel` 子命令、`--json` 字段）、`internal/tray`（设置、菜单、自更新、WSL 安装、兼容判定）、`install.sh`、`cmd/internal/releasesign`（按通道签名与验证）。
- 契约：`contract.UpdateCheckV1` 增加可选 `channel`；`docs/protocol/control-json.md` 同步。
- 发布：新增 `.github/workflows/dev-release.yml`；`Makefile` 支持带版本号的产物名；新增 Actions 密钥 `WEBMUX_DEV_SIGNING_KEY`。
- 文档：`docs/deployment/updates.md`、`docs/deployment/install-and-upgrade.md`、`docs/deployment/windows-wsl.md`，以及相关用户故事。
