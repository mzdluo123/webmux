# 配置与持久化清单

本页记录当前实现，不把 OpenSpec 的目标行为当成已实现功能。清单核对基于主分支 `677c1a94dedf461487867e6b99c1cbba670208a0`；后续变更必须同步清单和源码引用。

## 当前服务端存储

以下路径除 `config.toml` 外均相对于 `state_dir`；`none` 表示没有对应跨进程锁。TOML 位置由 `-config` 或默认配置目录决定。运行时目录中的 socket 不是持久化 JSON，不能用备份记录替代活进程身份验证。

<!-- persistence-inventory:start -->

| ID                  | 路径                    | 锁                      | 所有者                       | 损坏或冲突                             | 网页可改       |
| ------------------- | ----------------------- | ----------------------- | ---------------------------- | -------------------------------------- | -------------- |
| bootstrap           | config.toml             | none                    | internal/config + cmd/webmux | 配置错误阻止启动                       | 否             |
| launch              | launch-configs.json     | launch-configs.lock     | internal/launch              | 保留原文件；启动配置不可用             | 是             |
| repositories        | repositories.json       | repositories.lock       | internal/workspace           | 保留原文件；仓库管理不可用             | 是             |
| tunnel              | tunnel.json             | tunnel-config.lock      | internal/tunnelsettings      | 保留原文件；关闭入口并禁写             | 是             |
| tailscale-settings  | tailscale-settings.json | tailscale-settings.lock | internal/tailscalesettings   | 保留原文件；mode off 并禁写            | 是             |
| tailscale-ownership | tailscale.json          | tailscale.lock          | cmd/webmux                   | 保留原文件；无法确认入口安全时拒绝启动 | 否             |
| push                | push.json               | none                    | internal/push                | 当前改名后重建；设备需重新订阅         | 是（设备订阅） |
| runtime             | run/server.json         | run/server.lock         | internal/runtime             | 记录损坏或锁冲突使本机控制退化         | 否             |

<!-- persistence-inventory:end -->

这份表的机器来源是 `internal/archtest/persistence_inventory.json`；`make persistence-check` 精确比较表格各单元格（允许 Markdown 对齐空格变化），拒绝遗漏、重复或过时条目。

## 当前行为与缺口

- `internal/statefile` 提供同目录临时文件、0600、文件同步、原子替换和父目录同步；只有 `RequireDirSync` 调用者把目录同步失败作为错误返回。它还没有统一读取、大小限制或加锁接口。
- 仓库清单、Cloudflare 与 Tailscale 网页覆盖使用强制目录同步；仓库清单已明确处理替换后同步失败，禁止旧内存再次覆盖磁盘。launch、push、Tailscale 所有权和运行时记录仍使用尽力目录同步。
- launch、tunnel 与 tailscalesettings 使用未知字段拒绝和尾随内容检查；repositories、push、Tailscale 所有权、运行时记录使用宽松 JSON 解码。它们都还没有共享的有界严格解码器。
- push 当前仅有进程内 mutex，损坏会改名重建。统一方案已经选择保留原文件并禁用推送，但本次规格与守卫 PR 不改变其运行时行为。
- Tailscale 所有权丢失或不可确认时可能拒绝启动，避免遗留入口继续暴露本地服务；这不是普通偏好服务的可用性降级。
- `run/server.lock` 只保护运行时记录与本机 socket；锁冲突时第二个实例仍可提供 TCP 终端，但没有本机控制面。Tailscale 启动前所有权检查另外决定是否必须阻止启动；不能把所有锁冲突混为同一种行为。
- `settings.json` 尚在 `add-server-settings-sync` 规划中；`tailscale-settings.json` 已在调查期间的新提交中实现，现已纳入清单；它是安全访问覆盖，不是普通偏好。新增时登记真实实现，不提前登记虚构文件。
- Windows `tray.json`、`keepalive.json`、日志、下载缓存、上传文件与 baseline 结果不属于服务端配置清单；它们的文件名和写入入口仍逐项登记为有原因的例外。

## 机械校验

```sh
make persistence-check
```

守卫位于现有 `internal/archtest`，直接解析 `cmd/` 与 `internal/` 所有平台的生产 Go 文件，不受本机 GOOS 限制。测试文件、testdata 和前端输出不参与生产扫描。

- 每个 `.json` / `.lock` 字符串及选定的文件创建、替换、删除、底层打开和锁函数引用，都登记到具体文件、函数、符号与出现次数。
- 解析导入别名；引用函数值也计数；拒绝对受检查包使用点导入。新增入口、重复调用、删除后遗留登记、无主登记、无理由例外或缺失来源文件都会失败。
- 每个存储的文件名和锁名必须在该存储登记的源码引用中实际出现。临时 JSON 模式与设备文件也需要登记，不能凭命名自动豁免。
- 负向样例验证新存储、旧函数中的新增调用、别名、函数值、Windows 文件、清单漏行及错误策略漂移确实被拒绝。
- `make check` 和 CI 的格式/构建 job 显式执行，常规 Go 测试也会运行。失败时根据诊断更新具体登记与本表，不能扩大目录级白名单。

## 校验边界

这是当前架构的防漂移入口守卫，不是完整的数据流证明。它无法推断已经登记函数内部任意动态路径的新含义，也不能证明 chmod、fsync、锁时序、错误策略描述与所有执行路径一致；外部进程和反射写文件不在扫描能力内。更换未覆盖的 I/O 库必须扩展扫描规则与负向样例。

后续统一持久化实现需要收紧为“领域服务只能取得已注册的 typed handle”，撤销各领域原始 I/O 登记，并补齐故障注入、权限、跨进程锁、严格解码与脱敏契约测试。当前清单是精确调用点登记，不能当作永久允许新领域自建存储的通行证。

## 变更流程

新增存储前明确所属层、生命周期、数据/锁名、所有者、敏感性、失败后果和网页可编辑边界。登记真实读写入口及对应测试后同步清单；仍在规划的能力只写 OpenSpec，不登记为已实现。

详细目标、破坏性改名和分阶段验收见 [配置与状态治理提案](../openspec/changes/govern-config-state/proposal.md)。部署总览保留在 [部署指南](deployment.md)，架构分层见 [架构说明](architecture.md)。
