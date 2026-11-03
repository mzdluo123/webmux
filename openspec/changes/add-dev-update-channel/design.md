# Design

## Context

现有更新链路（`signed-release-updates`）：

- 客户端读 `releases/latest/download/SHA256SUMS` 与签名；可信注释 `webmux <版本>` 决定版本；产物从 `releases/download/v<版本>/` 以固定文件名下载。
- 只有一组内置公钥，`ParseVersion` 只接受 `MAJOR.MINOR.PATCH`；托盘的版本对应判定与自更新也只认这种格式。
- 发布 workflow 由 `v*` 标签触发，分为 build、sign、publish 三个 job，签名 job 只拿到 `SHA256SUMS`。
- 现有正式标签：`v0.10.2`。

## Goals / Non-Goals

**Goals:**

- main 上每次通过 CI 的提交都能以开发版发布，开发版用户按现有更新节奏（Linux 每日、托盘每 24 小时）自动收到。
- 正式版用户在任何情况下都不会收到开发版，即使开发版签名密钥泄露。
- GitHub 上不累积 Release 或标签。

**Non-Goals:**

- beta、rc 等第三个通道；开发版的发布说明；回滚到更早的开发版。
- 为 macOS 或 `install.sh` 安装提供自动更新（它们仍只检查不替换）。
- 让 `webmux.service` 自动重启以应用开发版。

## Decisions

### D1 版本号

开发版版本为 `X.Y.Z-dev.N`：

- `X.Y.Z` 为最近一个正式标签 `vA.B.C` 的补丁号加 1，即 `A.B.(C+1)`；
- `N` 为该标签之后的提交数，正整数、无前导零；
- 由 `git describe --tags --match 'v[0-9]*.[0-9]*.[0-9]*' --exclude '*-*'` 推导，不需要手工维护。

比较规则遵循 semver 预发布优先级，并只接受 `dev` 这一种后缀：

```text
0.10.2 < 0.10.3-dev.1 < 0.10.3-dev.2 < 0.10.3 < 0.11.0-dev.1 < 0.11.0
```

下一个正式版无论是补丁、次版本还是主版本，都高于此前所有开发版；新标签后 `N` 从 1 重新计数，但基数升高，整体仍单调递增。

deb/rpm 的包版本写成 `X.Y.Z~dev.N`，`~` 使它排在 `X.Y.Z` 之前；nfpm 以 semver 解析 `X.Y.Z-dev.N` 时会自动生成该形式。

### D2 单个滚动 Release，产物名带版本号

开发版只使用一个 Release 与标签 `dev`，标记为预发布且不设为 latest。每次构建：

1. 把标签 `dev` 移到本次提交；
2. 上传本次的产物，文件名为 `webmux-<版本>-<正式版文件名去掉 webmux- 前缀>`，例如 `webmux-0.10.3-dev.2-linux-amd64`、`webmux-0.10.3-dev.2-setup-windows-amd64.exe`；deb/rpm 使用包文件名 `webmux_0.10.3~dev.2-1_amd64.deb`、`webmux-0.10.3~dev.2-1.x86_64.rpm`；
3. 以 `--clobber` 替换 `SHA256SUMS` 与 `SHA256SUMS.minisig`，清单只列本次的产物；
4. 删除既不属于本次、也不属于上一次构建的产物。

客户端先读清单再下载产物；上一次构建的产物保留到下一次构建，所以读到旧清单的客户端仍能下载完成。

比较过的方案：

- 每次构建一个预发布并定期清理：Release 与标签会累积，清理与正在下载的客户端存在竞态。
- 覆盖同名产物：清单与产物可能在下载途中错位。
- 只发 nightly：不满足“每次提交可用”。

### D3 签名密钥按通道分组

- 新增 Actions 密钥 `WEBMUX_DEV_SIGNING_KEY`，与正式版私钥不同。
- 发布构建内置两组公钥：stable 组与 dev 组。
- 验证清单后检查版本与密钥组是否匹配：stable 组只能签不带后缀的版本，dev 组只能签 `-dev.N` 版本，不匹配视为签名无效。
- 正式版通道只信任 stable 组。开发版密钥泄露时，攻击者签出的任何清单都会被正式版客户端拒绝；开发版用户受影响，此时轮换 dev 密钥即可。

签名环境沿用正式版的隔离方式：独立 job 只接收 `SHA256SUMS`，只运行仓库自带的签名工具。

### D4 清单发现

| 通道 | 读取的清单 | 产物来源 |
|---|---|---|
| stable | `releases/latest/download/` | `releases/download/v<版本>/` 固定文件名 |
| dev | `releases/download/dev/` 与 `releases/latest/download/` | 版本带 `-dev.N` 时为 `releases/download/dev/` 带版本文件名；否则同 stable |

dev 通道对两个清单分别按现有来源顺序与镜像规则取得已验证结果，再取版本较高的一个；一个清单失败不影响另一个。正式版发布后，开发版用户因此自动升到正式版。

### D5 通道的选择与保存

- 默认 stable；未配置、文件缺失或内容无法识别时都按 stable 处理，无法识别时记录警告。
- Linux：`/etc/webmux/update-channel`，内容为 `stable` 或 `dev`，不属于任何软件包，升级与卸载都不改动它。`webmux update channel` 显示当前通道；`webmux update channel stable|dev` 需要 root，写入后立即生效于下一次检查。`webmux update check` 也读取该文件。
- 托盘：`tray.json` 的 `updateChannel`，菜单“更新通道”下为“正式版”“开发版”两个单选项；切换后立即写入设置并发起一次检查。
- `install.sh`：`WEBMUX_CHANNEL=dev` 时从 `dev` Release 读清单并安装开发版二进制；只影响本次安装，不写配置。

### D6 切换通道只升级不降级

从 dev 切回 stable 时，若当前开发版高于最新正式版则不做任何安装，状态中说明“当前开发版 X 高于最新正式版 Y，正式版超过它后自动更新”。这样不需要降级路径，也不会出现 dpkg/rpm 降级的特殊处理。

### D7 托盘与 WSL 服务端

- 开发版托盘内置同一次构建的服务端摘要，安装 WSL 服务端时从 `dev` Release 下载带版本号的文件；本地缓存与安装程序预置文件按同一文件名查找。
- 版本对应判定按 D1 的规则比较，两侧都是开发版或一侧是开发版时也能判定较旧或较新；字面 `dev` 等无法解析的版本仍归为“无法判断”。
- 托盘切到开发版后，自动升级 WSL 服务端的规则不变，因此服务端会跟随到同一次开发版构建。

### D8 开发版发布 workflow

新增 `dev-release.yml`：

- 在 `test.yml` 对 main 的推送成功后触发（`workflow_run`），只在本仓库的 main 上运行，不处理 PR；
- `concurrency` 按 `dev-release` 分组并取消进行中的运行，连续推送时只发布最后一次；
- 复用 `make release-build`，以 `VERSION=<开发版版本>` 与带版本号的产物命名构建；sign 与 publish 结构与正式版相同，publish 按 D2 的顺序操作。

## Risks / Trade-offs

- [每次合入都跑完整的发布构建] → 约 10 分钟；`concurrency` 合并连续推送，只改文档的提交由 `test.yml` 的变更分类跳过。
- [开发版质量] → 只发布通过 `test.yml` 的提交；开发版用户主动选择，切回正式版不降级。
- [镜像缓存 `dev` 清单] → 沿用“多个来源取最高已验证版本”；最坏情况是开发版用户晚一天收到。
- [dev 密钥泄露] → 正式版用户不受影响（D3）；文档写明轮换 dev 密钥的流程。
- [两次构建间隔很短] → 只保留当前与上一次构建的产物，读到更早清单的客户端下载失败，下一次检查重试。

## Migration Plan

1. 维护者生成 dev 密钥，配置 `WEBMUX_DEV_SIGNING_KEY`，把公钥加入 dev 组。
2. 合入后，带有通道支持的第一个正式版发布前，开发版就会出现在 `dev` Release；更早的客户端只认 stable，不受影响。
3. 用户按 D5 选择 dev 通道即可加入；退出只需切回 stable。
