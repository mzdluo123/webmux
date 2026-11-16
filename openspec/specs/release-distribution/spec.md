# release-distribution Specification

## Purpose

定义 WebMux 正式版本的发布方式：一次标签发布产出所有平台二进制与校验和，并让 Windows 托盘内置同版本 Linux 二进制的摘要，使托盘可以安全地安装服务端。

## Requirements

### Requirement: 标签触发发布

推送形如 `v<版本>` 的 Git 标签时，发布流水线 SHALL 以该版本号构建全部发布产物，并创建同名 GitHub Release 上传它们。每个二进制 SHALL 报告该版本号，并 SHALL 标记为发布构建。构建、测试或签名失败时 SHALL NOT 创建或更新 Release。

#### Scenario: 发布新版本

- **WHEN** 维护者推送标签 `v0.2.0`
- **THEN** Release `v0.2.0` 包含全部发布产物，`webmux version` 输出 `webmux 0.2.0`

#### Scenario: 构建失败

- **WHEN** 推送标签后某个平台构建失败
- **THEN** 不出现该标签的 Release，也不上传任何部分产物

#### Scenario: 安装程序构建失败

- **WHEN** 推送标签后 Windows 安装程序打包失败
- **THEN** 不出现该标签的 Release，也不上传任何部分产物

### Requirement: 发布产物

每个标签发布的 Release SHALL 包括以下产物（Linux 安装包见“Linux 安装包”；Release `dev` 的产物见“开发版产物命名”）：

- Linux：`webmux-linux-amd64`、`webmux-linux-arm64`；
- macOS：`webmux-darwin-amd64`、`webmux-darwin-arm64`；
- Windows：`webmux-windows-amd64.exe`、`webmux-windows-arm64.exe`；
- Windows 托盘：`webmux-tray-windows-amd64.exe`、`webmux-tray-windows-arm64.exe`；
- Windows 安装程序：`webmux-setup-windows-amd64.exe`、`webmux-setup-windows-arm64.exe`；
- 校验和：`SHA256SUMS`；
- 校验和签名：`SHA256SUMS.minisig`。

#### Scenario: 产物齐全

- **WHEN** 标签 `v0.2.0` 的发布流水线成功
- **THEN** Release 中包含上述每个文件，`SHA256SUMS.minisig` 能以内置公钥验证 `SHA256SUMS`

### Requirement: 校验和文件

`SHA256SUMS` 的格式 SHALL 与 `sha256sum` 输出一致，可用 `sha256sum -c` 直接校验。

- 标签发布：SHALL 为 Release 中除 `SHA256SUMS.minisig` 外的每个其他产物各列一行。
- Release `dev`：SHALL 只为本次构建的产物（包括 `install.sh`）各列一行。

#### Scenario: 用户校验下载

- **WHEN** 用户下载 `webmux-linux-amd64` 与 `SHA256SUMS` 后执行 `sha256sum -c --ignore-missing SHA256SUMS`
- **THEN** 该文件校验通过

#### Scenario: 开发版清单不列旧产物

- **WHEN** Release `dev` 中有 0.10.3-dev.2 与 0.10.3-dev.3 的产物，当前清单为 dev.3
- **THEN** `SHA256SUMS` 只列出 dev.3 的产物

### Requirement: 托盘内置服务端摘要

发布构建的托盘 SHALL 内置同一次构建产出的 `webmux-linux-amd64` 与 `webmux-linux-arm64` 的 SHA-256。托盘 SHALL 只信任内置摘要，SHALL NOT 以下载得到的 `SHA256SUMS` 作为校验依据。未内置摘要的托盘（如开发构建）SHALL 报告“此版本托盘不支持安装 webmux”，且不提供下载。

#### Scenario: 发布版托盘

- **WHEN** 用户运行 Release 中的托盘
- **THEN** 托盘内置的两个摘要与同一 Release 的 `SHA256SUMS` 中对应行一致

#### Scenario: 开发构建

- **WHEN** 用户运行本地 `make tray` 构建、未注入摘要的托盘，且 WSL 中缺少 webmux
- **THEN** 托盘显示缺少 webmux 及“此版本托盘不支持安装 webmux”，菜单没有可用的安装动作

### Requirement: 安装程序内容来自同一次构建

每个 Windows 安装程序 SHALL 只包含同一次发布构建产出、同架构的托盘（与 Release 中 `webmux-tray-windows-<架构>.exe` 字节相同）以及同架构的 Linux 服务端（与 `webmux-linux-<架构>` 字节相同）。因此安装后的托盘 SHALL 内置与预置服务端相符的摘要。安装程序登记的版本号 SHALL 与该发布版本相同。

#### Scenario: 安装后文件一致

- **WHEN** 用户安装 `v0.2.0` 的 `webmux-setup-windows-amd64.exe`
- **THEN** 安装目录中 `webmux-tray.exe` 的 SHA-256 与同一 Release `SHA256SUMS` 中 `webmux-tray-windows-amd64.exe` 一行一致，预置的服务端与 `webmux-linux-amd64` 一行一致

### Requirement: Linux 安装包

标签发布 SHALL 另外产出 Linux 安装包并上传到同一 Release：`webmux_<版本>-1_amd64.deb`、`webmux_<版本>-1_arm64.deb`、`webmux-<版本>-1.x86_64.rpm`、`webmux-<版本>-1.aarch64.rpm`。

安装包 SHALL 由同一次构建的 `webmux-linux-<架构>` 打包，并各自在 `SHA256SUMS` 中占一行。安装包构建失败时 SHALL NOT 创建 Release。

#### Scenario: 发布包含安装包

- **WHEN** 维护者推送标签 `v0.2.0`
- **THEN** Release 中包含上述四个安装包，`sha256sum -c SHA256SUMS` 对它们全部通过

### Requirement: 发布签名

发布流水线 SHALL 用发布私钥对 `SHA256SUMS` 生成 minisign 格式的 `SHA256SUMS.minisig`，可信注释为 `webmux <版本>`，并 SHALL 在发布前以内置公钥验证该签名。

- 私钥 SHALL 只来自流水线密钥配置，SHALL NOT 出现在仓库、构建日志或产物中。
- 构建、测试与打包 SHALL 在看不到私钥的环境中完成；签名 SHALL 在独立环境中进行，该环境只接收 `SHA256SUMS`，SHALL NOT 运行前端依赖、第三方 Go 模块或打包工具。
- 密钥缺失或签名失败时 SHALL NOT 创建 Release。
- 公钥 SHALL 内置于发布构建并在仓库文档中公布，使用户可以用 `minisign -Vm SHA256SUMS -P <公钥>` 手动验证。

#### Scenario: 用户手动验证

- **WHEN** 用户下载 `SHA256SUMS` 与 `SHA256SUMS.minisig`，以文档中的公钥执行 `minisign -Vm SHA256SUMS -P <公钥>`
- **THEN** 验证通过，输出的可信注释为 `webmux 0.5.0`

#### Scenario: 缺少签名密钥

- **WHEN** 推送标签时流水线没有配置发布私钥
- **THEN** 发布失败，不出现该标签的 Release

#### Scenario: 构建环境拿不到私钥

- **WHEN** 发布构建中的某个依赖在构建时读取环境变量
- **THEN** 它读不到发布私钥，签名环境只运行仓库自带的签名工具

### Requirement: 开发版发布

main 上的提交通过 CI 后，开发版流水线 SHALL 以开发版版本构建与正式版相同的二进制、托盘、安装程序与 Linux 安装包，并发布到唯一的 Release `dev`。

- 版本 SHALL 为最近一个 `vX.Y.Z` 正式标签的补丁号加 1，加 `-dev.N`，`N` 为该标签之后的提交数；`N` 为 0（提交本身即正式标签）时 SHALL NOT 发布。
- 流水线 SHALL 构建通过 CI 的那个提交。发布前该提交已不是 main 的最新提交，或其版本不高于 Release `dev` 当前清单的版本时，SHALL NOT 发布。
- Release `dev` SHALL 标记为预发布，SHALL NOT 成为 `latest`；替换清单后标签 `dev` SHALL 指向本次提交。
- 连续推送时 SHALL 只发布最后一次；拉取请求、非 main 分支与其他仓库 SHALL NOT 触发开发版发布。
- 替换清单的步骤一旦开始 SHALL NOT 被新的运行取消。

#### Scenario: 合入后发布开发版

- **WHEN** v0.10.2 之后的第 2 个提交合入 main 且 CI 通过
- **THEN** Release `dev` 的签名清单可信注释为 `webmux 0.10.3-dev.2`，`latest` 仍指向 v0.10.2

#### Scenario: 较早的提交后完成测试

- **WHEN** 提交 A、B 依次合入 main，B 的 CI 先通过并发布为 0.10.3-dev.5，随后 A 的 CI 通过
- **THEN** A 不发布，Release `dev` 的清单仍为 0.10.3-dev.5，标签 `dev` 仍指向 B

#### Scenario: 正式标签提交

- **WHEN** 打上 `v0.10.3` 的提交推送到 main 且 CI 通过
- **THEN** 开发版流水线不发布，Release `dev` 不变

### Requirement: 开发版跳过只改文档的变更

自 Release `dev` 当前发布的提交以来只改了文档时，开发版流水线 SHALL NOT 构建与发布。

- 判定 SHALL 使用当前发布提交中的变更分类规则，SHALL NOT 采用待发布提交中的规则。
- 分类器要求完整运行的变更（工具链、未知路径等）SHALL 照常发布。
- 当前发布的提交不存在或不是本次提交的祖先时 SHALL 照常发布。

#### Scenario: 只改文档

- **WHEN** Release `dev` 当前为 0.10.3-dev.5，之后合入 main 的提交只修改了 `docs/` 与 `openspec/` 下的 Markdown，CI 通过
- **THEN** 开发版流水线不构建也不发布，Release `dev` 仍为 0.10.3-dev.5

#### Scenario: 文档之后的代码提交

- **WHEN** 接上例，再合入一个修改 `internal/` 的提交，它是 v0.10.2 之后的第 7 个提交
- **THEN** Release `dev` 发布 0.10.3-dev.7

### Requirement: 开发版产物命名

开发版产物文件名 SHALL 带版本号，且 SHALL NOT 含有 GitHub 会改写的字符：

- 二进制、托盘与安装程序：`webmux-<版本>-<正式版文件名去掉 webmux- 前缀>`；
- deb：`webmux_<版本>-1_<架构>.deb`；rpm：`webmux-<版本>-1.<架构>.rpm`；包内版本 SHALL 为 `X.Y.Z~dev.N`。

Release `dev` SHALL 以固定文件名包含 `install.sh`，内容与同一提交中的脚本相同；它 SHALL 列在本次清单中，SHALL 随清单一起替换。

#### Scenario: 产物名

- **WHEN** 开发版 0.10.3-dev.2 发布完成
- **THEN** Release `dev` 包含 `webmux-0.10.3-dev.2-linux-amd64`、`webmux-0.10.3-dev.2-setup-windows-amd64.exe` 与 `webmux_0.10.3-dev.2-1_amd64.deb`，均列在 `SHA256SUMS` 中
- **AND** 该 deb 的包内版本为 `0.10.3~dev.2-1`

### Requirement: 开发版发布顺序与保留

开发版发布 SHALL 依次：上传本次带版本号的产物；替换 `install.sh`、`SHA256SUMS` 与签名；移动标签 `dev`；删除 Release 中不在本次清单中的文件，Release `dev` 因此只保留当前构建。替换清单前读到旧清单的客户端可能下载失败，SHALL 在下一次检查时重试。

- 替换清单之前任何一步失败时 SHALL NOT 替换清单，已列在当前清单中的产物 SHALL 保持不变；上传了一部分的本次产物 SHALL 由下一次成功的发布删除。
- 替换清单之后删除失败时，SHALL 只留下多余的文件，由下一次成功的发布删除。
- 判断“版本不高于当前清单”时 SHALL 只采用能以内置公钥验证的现有清单与签名；缺失、不配对或无法验证时 SHALL 视为未发布并照常发布。

#### Scenario: 只保留当前构建

- **WHEN** 开发版 0.10.3-dev.3 发布完成，此前 Release `dev` 中有 dev.1 与 dev.2 的产物，当前清单为 dev.2
- **THEN** Release `dev` 中只剩 dev.3 的产物以及 `install.sh`，签名清单列出 dev.3

#### Scenario: 上次替换清单中途失败

- **WHEN** 发布 0.10.3-dev.3 时签名已替换为 dev.3 而 `SHA256SUMS` 仍是 dev.2 的，随后重跑 0.10.3-dev.3 的发布
- **THEN** 重跑判定现有清单无法验证，重新上传 dev.3 的清单与签名，Release `dev` 恢复为可验证的 dev.3

#### Scenario: 发布中途失败

- **WHEN** 上传 0.10.3-dev.3 的产物时失败，之后 0.10.3-dev.4 发布成功
- **THEN** 失败后签名清单仍为 dev.2，dev.2 的产物全部保留；dev.4 发布后 Release `dev` 中只剩 dev.4 的产物

### Requirement: 开发版签名

开发版 SHALL 使用与正式版不同的私钥签名，私钥 SHALL 只来自流水线密钥配置。开发版的构建、签名与发布 SHALL 采用与“发布签名”相同的隔离：构建环境看不到私钥，签名环境只接收 `SHA256SUMS` 并只运行仓库自带的签名工具。

- 发布前 SHALL 以内置的开发版公钥组验证签名。
- 开发版公钥 SHALL 内置于发布构建，并在仓库文档中与正式版公钥分开公布。

#### Scenario: 缺少开发版密钥

- **WHEN** main 推送后流水线没有配置开发版私钥
- **THEN** 开发版发布失败，Release `dev` 保持上一次的内容
