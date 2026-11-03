# Spec Delta

## ADDED Requirements

### Requirement: 开发版发布

main 上的提交通过 CI 后，开发版流水线 SHALL 以开发版版本构建与正式版相同的全部产物，并发布到唯一的 Release `dev`。

- 版本 SHALL 为最近一个 `vX.Y.Z` 正式标签的补丁号加 1，加 `-dev.N`，`N` 为该标签之后的提交数。
- Release `dev` SHALL 标记为预发布，SHALL NOT 成为 `latest`；标签 `dev` SHALL 指向本次提交。
- 连续推送时 SHALL 只发布最后一次；拉取请求与非 main 分支 SHALL NOT 触发开发版发布。

#### Scenario: 合入后发布开发版

- **WHEN** v0.10.2 之后的第 2 个提交合入 main 且 CI 通过
- **THEN** Release `dev` 的签名清单可信注释为 `webmux 0.10.3-dev.2`，`latest` 仍指向 v0.10.2

### Requirement: 开发版产物命名

开发版产物文件名 SHALL 带版本号，`SHA256SUMS` SHALL 只列本次构建的产物。

- 二进制、托盘与安装程序：`webmux-<版本>-<正式版文件名去掉 webmux- 前缀>`；
- deb：`webmux_<X.Y.Z~dev.N>-1_<架构>.deb`；rpm：`webmux-<X.Y.Z~dev.N>-1.<架构>.rpm`。

#### Scenario: 产物名

- **WHEN** 开发版 0.10.3-dev.2 发布完成
- **THEN** Release `dev` 包含 `webmux-0.10.3-dev.2-linux-amd64`、`webmux-0.10.3-dev.2-setup-windows-amd64.exe` 与 `webmux_0.10.3~dev.2-1_amd64.deb`，均列在 `SHA256SUMS` 中

### Requirement: 开发版发布顺序与保留

开发版发布 SHALL 先上传本次产物，再替换 `SHA256SUMS` 与签名，最后删除既不属于本次、也不属于上一次构建的产物。任何一步失败时 SHALL NOT 替换签名清单，已有产物 SHALL 保持不变。

#### Scenario: 只保留两次构建

- **WHEN** 开发版 0.10.3-dev.3 发布完成，此前 Release `dev` 中有 dev.1 与 dev.2 的产物
- **THEN** Release `dev` 中只剩 dev.2 与 dev.3 的产物，签名清单列出 dev.3

#### Scenario: 发布中途失败

- **WHEN** 上传 0.10.3-dev.3 的产物时失败
- **THEN** 签名清单仍为 dev.2，dev.2 的产物全部保留

### Requirement: 开发版签名

开发版 SHALL 使用与正式版不同的私钥签名，私钥 SHALL 只来自流水线密钥配置。开发版的构建、签名与发布 SHALL 采用与“发布签名”相同的隔离：构建环境看不到私钥，签名环境只接收 `SHA256SUMS` 并只运行仓库自带的签名工具。开发版公钥 SHALL 内置于发布构建，并在仓库文档中与正式版公钥分开公布。

#### Scenario: 缺少开发版密钥

- **WHEN** main 推送后流水线没有配置开发版私钥
- **THEN** 开发版发布失败，Release `dev` 保持上一次的内容
