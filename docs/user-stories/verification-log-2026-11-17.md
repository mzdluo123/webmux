# User Stories 局部核对记录（2026-11-17 起）

按日期记录各次局部核对的范围、已运行的验证与限制，索引见 [WebMux User Stories](../user-stories.md#核对记录)。

## `install.sh` 镜像回退签名校验与非 GitHub 信任根收敛（2026-11-17，US-K11、US-L13）

- 现象：`install.sh` 默认回退到 `ghfast.top`、`gh-proxy.com` 时，在未指定 `WEBMUX_SHA256` 的情况下直接信任同一镜像返回的未签名 `SHA256SUMS`，可被恶意或被劫持镜像同时替换二进制与摘要。
- 修复：
  - `install.sh` 内置与 `internal/update` 一致的正式版、开发版 Ed25519 minisign 公钥，启动时以内置自检向量探测本机 `minisign` 或支持 Ed25519 `-rawin` 的 `openssl`；
  - 未指定 `WEBMUX_SHA256` 且具备本地验签能力时，所有来源都须提供有效的 `SHA256SUMS.minisig`，同时校验文件签名、可信注释全局签名、通道公钥组隔离与请求版本一致性；
  - 未指定 `WEBMUX_SHA256` 且缺少本地验签能力（如默认仅含 LibreSSL 的 macOS 或精简容器）时，仅接受直接从 `https://github.com/mzdluo123/webmux` 获取的 `SHA256SUMS`（GitHub 已返回清单而二进制中断时允许镜像提供匹配摘要的二进制），拒绝非 GitHub 来源的未验证清单；
  - `cmd/internal/releasesign` 增加 `TestInstallScriptPublicKeysMatchBuiltIn`，防止密钥轮换时 `install.sh` 与 `internal/update` 公钥漂移；
  - `docs/user-stories/operations-and-tooling.md` 接近 48 KiB 上限，按 K、L 分组原文拆分为 `operations-and-tooling/operations.md` 与 `operations-and-tooling/tooling.md`，原路径保留树状索引。
- 回归测试与验证结果：
  - `make test-install` 通过，覆盖以下场景：
    - `openssl` 与 `minisign` 两条本地验签路径及 `shasum` 摘要回退；
    - 镜像返回篡改二进制与无签名伪造清单、伪造清单配真实签名、攻击者自签密钥及内置生产公钥拒绝测试密钥；
    - 可信注释篡改、开发版密钥跨通道混用、`WEBMUX_VERSION` 旧版重放、开发版签名版本与清单资产不一致及毒化首镜像自动切换次镜像；
    - 无本地验签器时拒绝非 GitHub 镜像、允许 `WEBMUX_SHA256` 固定摘要，以及复用官方 GitHub 清单摘要校验镜像二进制；
  - 隔离临时目录实测直连 GitHub 最新正式版安装：`WEBMUX_INSTALL_DIR=<tmp> WEBMUX_MIRRORS= sh ./install.sh` 通过内置公钥验签并安装 `webmux 0.16.0`；
  - `go test -race ./cmd/internal/releasesign ./internal/update ./internal/archtest`、`go vet`（含 `GOOS=darwin`）与 `make go-lint`（golangci-lint v2.15.0，0 issues）通过；
  - `make docs-check`、`make persistence-check`、`make format-check`、`openspec validate update-channels --strict` 与 `bash .github/scripts/test-classify-changes.sh` 通过。
- 局限：未在无 Homebrew 的原生 macOS（LibreSSL 环境）真机上执行实机联网回归，无本地验签器分支通过受控隔离 `PATH` 与本地 HTTP 夹具验证。故事编号与总数不变。
