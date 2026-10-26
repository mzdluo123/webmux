## Why

[#146](https://github.com/mzdluo123/webmux/issues/146) 的根因是持久化策略、来源解析和目录清单分别由各功能自行维护。只改文件名或抽取原子写不能阻止新功能再次复制存储逻辑。当前代码还有替换后目录同步失败、持久入口所有权与只读诊断之间的边界，不能用统一“失败就回滚/继续启动”处理。

本提案基于主分支 `677c1a94dedf461487867e6b99c1cbba670208a0`，逐项调查与外部依据见 [research.md](research.md)。本次 draft PR 交付规格与可运行防漂移守卫；运行时实现分阶段执行。当前真实行为见 [持久化清单](../../../docs/persistence.md)。

## What Changes

- 保留引导 TOML、通用偏好、领域凭据/所有权/数据的边界。公网访问设置走专用安全流程，不进入通用偏好快照。
- 在 `internal/statefile` 建立已注册存储描述符、严格有界读取、独立锁、原子提交与类型化故障状态；领域服务保留自身数据校验与业务副作用。
- **BREAKING**：统一锁名为完整数据文件名加 `.lock`；将 `tunnel.json` 改为 `tunnel-settings.json`、`tailscale.json` 改为 `tailscale-ownership.json`，不提供双读、双写或自动迁移。
- 旧 Tailscale 所有权文件出现时阻止启动并要求先清理/核实外部入口；破坏性升级不等于可以忘记已有入口。
- **BREAKING**：push 损坏后保留原文件并禁用推送，删除静默重建密钥的例外。不存在文件仍允许首次初始化。
- 区分 configured（磁盘解析）与 effective（活进程快照），字段级来源、可用性与脱敏由同一解析模型提供。
- 本 PR 立即增加 `TestPersistence*`、精确源码引用清单、文档一致性检查、负向样例和 `make persistence-check` / CI 接入。
- 后续通过 typed handle、统一适配器契约测试、故障注入与脱敏测试逐步替代临时调用点登记。
- 不重复实现 #119 的偏好同步；`add-server-settings-sync` 消费本变更的持久化基础设施，并在实施时更新其中旧锁名、回滚和兼容迁移假设。

## Capabilities

### New Capabilities

- `state-persistence`：存储登记、加锁/严格解码/提交、故障分类、破坏性升级及外部所有权保护。
- `effective-configuration`：磁盘与活进程配置快照、字段来源、降级原因和安全脱敏。
- `persistence-governance`：当前防漂移校验及后续统一入口、契约测试和文档完整性约束。

### Modified Capabilities

无。本 draft 新增横切治理能力；在实施相关运行时任务时再同步现有能力 delta，避免把尚未实施的接口变化记作现状。

## Impact

- 当前 PR：`internal/archtest`、Makefile、CI、部署/架构导航、持久化清单与 US-L09。
- 后续：`internal/statefile`、launch/workspace/push/tunnelsettings/tailscalesettings/runtime、Tailscale 所有权接线、config/client/localclient/contract/wire/api 与配置 CLI。
- 不变更 PTY、终端帧、会话生命周期；不新增网页编辑启动安全项的能力。
- 已确认：JSON 基础设施；push 保留损坏文件并禁用；本 PR 只做 OpenSpec 与守卫；旧所有权阻止启动。具体设计与待确认 CLI 默认入口见 [design.md](design.md)。
