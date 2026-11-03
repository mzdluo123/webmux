# Tasks

## 1. 版本号与签名

- [ ] 1.1 `internal/update` 的版本解析与比较支持 `X.Y.Z-dev.N`（无前导零），拒绝其他后缀；`Newer` 按 semver 预发布优先级比较。
  - 验证：单元测试覆盖 `0.10.2 < 0.10.3-dev.1 < 0.10.3-dev.2 < 0.10.3 < 0.11.0-dev.1`、`-dev.0`、`-dev.01`、`-rc.1` 被拒绝。
- [ ] 1.2 内置公钥分为 stable 与 dev 两组；验证清单后检查版本与密钥组匹配，`stable` 通道只信任 stable 组。
  - 验证：单元测试覆盖 dev 密钥签 `webmux 9.9.9` 被拒、stable 通道拒绝 dev 清单、dev 通道接受两组各自的版本。
- [ ] 1.3 `cmd/internal/releasesign` 的 `sign`/`verify` 按版本选择并校验密钥组；生成 dev 密钥并把公钥加入 dev 组。
  - 验证：Go 测试覆盖用错组签名时 `verify` 失败；`TestSignerLinksOnlyStandardLibrary` 仍通过。

## 2. 通道与清单发现

- [ ] 2.1 新增通道读取：Linux 读 `/etc/webmux/update-channel`，缺失或无法识别按 `stable` 并记录警告。
  - 验证：单元测试覆盖缺失、`stable`、`dev`、无法识别四种文件内容。
- [ ] 2.2 `Client.Latest` 按通道读取清单：`dev` 同时读 `dev` 与 `latest` 并取较高版本，一个失败不影响另一个；`Fetch` 按版本选择产物来源与带版本号的文件名。
  - 验证：httptest 覆盖 dev 较高、正式版较高、dev 清单全部失败三种情况，以及开发版产物 URL。
- [ ] 2.3 `PackageAsset` 为开发版生成 `X.Y.Z~dev.N` 的 deb/rpm 文件名。
  - 验证：单元测试覆盖 deb 与 rpm、amd64 与 arm64。

## 3. CLI

- [ ] 3.1 新增 `webmux update channel [stable|dev]`：无参数显示，带参数要求 root 并写入文件。
  - 验证：`updatecmd_test.go` 覆盖显示、非 root 拒绝、写入、无效参数退出码 2。
- [ ] 3.2 `update check` 输出通道；`contract.UpdateCheckV1` 增加 `channel`，并更新 `docs/protocol/control-json.md`。
  - 验证：JSON 输出测试包含 `channel`；切回 stable 且当前开发版更高时提示“正式版超过它后自动更新”。

## 4. 托盘

- [ ] 4.1 `tray.json` 增加 `updateChannel`（缺省 `stable`），菜单增加“更新通道”单选；切换后立即检查，非发布构建禁用。
  - 验证：`internal/tray` 测试覆盖旧设置默认值、菜单状态与切换后发起检查。
- [ ] 4.2 自更新按通道检查并下载带版本号的开发版安装程序；便携版“下载托盘”按版本打开 `v<版本>` 或 `dev` 页面。
  - 验证：核心测试覆盖 dev 通道安装开发版、切回 stable 不降级、便携版链接。
- [ ] 4.3 WSL 服务端安装按托盘版本从 `v<版本>` 或 `dev` 下载对应文件名；兼容判定支持 `-dev.N`。
  - 验证：`install_test.go` 与 `compat_test.go` 覆盖开发版下载路径、开发版之间与开发版对正式版的判定。

## 5. 发布流水线

- [ ] 5.1 `Makefile` 支持开发版版本与带版本号的产物名（二进制、托盘、安装程序），deb/rpm 文件名由 nfpm 生成 `~dev.N`。
  - 验证：本地以 `VERSION=0.10.3-dev.2` 运行发布构建，产物名与 `SHA256SUMS` 符合 `release-distribution`。
- [ ] 5.2 新增 `.github/workflows/dev-release.yml`：`test.yml` 对 main 推送成功后触发，`concurrency` 取消进行中的运行，build/sign/publish 与正式版相同的隔离，publish 按“上传产物、替换清单、删除旧产物”的顺序操作 Release `dev`。
  - 验证：`actionlint` 通过；在测试仓库或 fork 中连续推送两次，Release `dev` 只剩两次构建的产物，`latest` 不变。
- [ ] 5.3 `install.sh` 支持 `WEBMUX_CHANNEL=dev|stable`，开发版从 Release `dev` 读取已签名清单。
  - 验证：本地签名的测试 Release 上安装开发版，`webmux version` 报告开发版版本；无效取值报错且不改动文件。

## 6. 文档与用户故事

- [ ] 6.1 更新 `docs/deployment/updates.md`（通道、切换、dev 公钥与轮换）、`install-and-upgrade.md`、`windows-wsl.md`。
  - 验证：`make docs-check` 通过。
- [ ] 6.2 按 `docs/agent-rules/user-stories-maintenance.md` 新增或更新用户故事（Linux 通道、托盘通道、开发版发布），同步两个索引与核对记录。
  - 验证：故事 ID 唯一，`make docs-check` 通过。

## 7. 集成验证

- [ ] 7.1 `make check` 与 `make test-fast` 通过。
- [ ] 7.2 端到端：以本地签名的测试 Release 在 systemd 容器中把 deb 从正式版切到开发版再切回，验证升级、不降级与正式版追上；在 Windows 上验证托盘切换通道后自更新到开发版。

## Workflow follow-up

- 维护者生成 dev 签名密钥并配置 Actions 密钥 `WEBMUX_DEV_SIGNING_KEY`。
- 审查通过后归档本变更。
