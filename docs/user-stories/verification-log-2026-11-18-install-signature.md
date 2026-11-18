# `install.sh` 镜像签名校验与信任根核对记录（2026-11-18）

收录 `install.sh` 镜像回退签名校验、非 GitHub 信任根收敛、旧版本固定安装提示与测试前置检查（US-K11、US-L13）的局部核对；主索引见 [WebMux User Stories](../user-stories.md#核对记录) 与 [配置、运行与贡献者工具](operations-and-tooling.md)。

## 范围

- 故事编号与总数（264）保持不变，更新 US-K11、US-L13 及 `update-channels` 规格：
  - `install.sh` 内置与 `internal/update` 一致的正式版、开发版 Ed25519 minisign 公钥，启动时以内置正反自检向量探测本机 `minisign` 或支持 Ed25519 `-rawin` 的 `openssl`；
  - 未指定 `WEBMUX_SHA256` 且具备本地验签能力时，所有来源均须提供有效的 `SHA256SUMS.minisig`，同时校验文件签名、可信注释全局签名、通道公钥组隔离与请求版本一致性；
  - 固定安装 `v0.11.0` 之前未附带 `SHA256SUMS.minisig` 的早期正式版时，错误提示明确告知使用 `WEBMUX_SHA256` 固定二进制摘要，并将安装脚本与文档中的版本示例更新为已签名的 `0.16.0`；
  - 未指定 `WEBMUX_SHA256` 且缺少本地验签能力（如默认仅含 LibreSSL 的 macOS 或精简容器）时，仅接受直接从 `https://github.com/mzdluo123/webmux` 获取的 `SHA256SUMS`（GitHub 已返回清单而二进制中断时允许镜像提供匹配摘要的二进制），拒绝非 GitHub 来源的未验证清单；
  - `cmd/internal/releasesign` 增加 `TestInstallScriptPublicKeysMatchBuiltIn`，防止密钥轮换时 `install.sh` 与 `internal/update` 公钥漂移；
  - `.github/scripts/test-install.sh` 启动时预检本机 `minisign` 或支持 Ed25519 的 `openssl`，缺失时直接输出安装提示，并在 `docs/agent-rules/testing.md` 与 `conventions.md` 登记 `make test-install` 的本地依赖；
  - `docs/user-stories/operations-and-tooling.md` 接近 48 KiB 上限，按 K、L 分组原文拆分为 `operations-and-tooling/operations.md` 与 `operations-and-tooling/tooling.md`，原路径保留树状索引并同步主线最新变更。

## 已执行验证与局限

- 安装脚本与签名回归：
  - `make test-install` 通过，覆盖以下场景：
    - `openssl` 与 `minisign` 两条本地验签路径、`shasum` 摘要回退，以及缺少验签器时的前置报错检查；
    - 篡改二进制配无签名清单拒绝、伪造清单配真实签名拒绝、攻击者自签密钥拒绝，以及内置生产公钥拒绝测试密钥；
    - 可信注释篡改拒绝、开发版密钥跨通道混用拒绝、`WEBMUX_VERSION` 旧版重放拒绝，以及毒化首镜像自动切换次镜像；
    - `v0.11.0` 之前无签名版本固定安装的 `WEBMUX_SHA256` 提示；
    - 无本地验签器时拒绝非 GitHub 镜像、允许 `WEBMUX_SHA256` 固定摘要安装，以及复用官方 GitHub 清单摘要校验镜像二进制；
  - 隔离临时目录实测直连 GitHub 正式版安装：`WEBMUX_INSTALL_DIR=<tmp> WEBMUX_MIRRORS= sh ./install.sh` 通过内置公钥验签并完成安装。
- 静态与治理门禁：
  - `go test -race ./cmd/internal/releasesign ./internal/update ./internal/archtest`、`go vet`（含 `GOOS=darwin`）与 `make go-lint`（`golangci-lint` v2.15.0，0 issues）通过；
  - `make docs-check`、`make persistence-check`、`make format-check`、`openspec validate update-channels --strict` 与 `bash .github/scripts/test-classify-changes.sh` 通过。
- 局限：未在无 Homebrew 的原生 macOS（LibreSSL 环境）真机上执行实机联网回归，无本地验签器分支与测试前置报错通过受控隔离 `PATH` 及本地 HTTP 夹具验证。
