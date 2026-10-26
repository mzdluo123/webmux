## 1. 本次 draft：调查与可运行守卫

- [x] 1.1 对照主分支与 issue，记录现有存储、错误策略、运行时发现边界与尚未实现能力。
- [x] 1.2 调查 XDG、fsync/flock、SQLite 故障测试、Go JSON 与 Tailscale 生命周期原始资料，写入 research.md。
- [x] 1.3 记录用户选择：统一 JSON、push 保留损坏文件、仅规格与守卫、旧所有权阻止启动。
- [x] 1.4 新增全平台 AST 引用守卫、精确清单、文档对照及故意违规样例。
- [x] 1.5 接入 make persistence-check / make check / CI，增加 US-L09 与文档导航。
- [x] 1.6 执行 OpenSpec 严格校验、定向 Go race、文档守卫、格式与故事索引检查并记录真实结果。
- [ ] 1.7 确认 config show 默认入口（不阻塞规格与守卫 draft PR，阻塞后续 CLI 实施）。

## 2. 统一基础设施（后续实施）

- [ ] 2.1 在 statefile 定义 descriptor、typed handle、故障 kind 与提交阶段；登记共享层边界。
- [ ] 2.2 实现生命周期 flock、0600/0700、符号链接拒绝、有界严格读取；区分不存在与损坏。
- [ ] 2.3 实现提交与删除的强制目录同步；commit_uncertain 禁止后续写，不回滚虚构状态。
- [ ] 2.4 实现纯只读 ReadSnapshot，不加写锁、不初始化/恢复、不 chmod。
- [ ] 2.5 建立 descriptor 与契约适配器一一对应测试；注入各写入/删除阶段失败并重新打开验证。
- [ ] 2.6 Linux/macOS 原生运行跨进程锁、权限、严格解析与故障测试；记录文件系统保证边界。

## 3. 领域迁移与命名（后续实施）

- [ ] 3.1 迁移 launch/repositories；移除原始锁与写入口登记，保留领域验证与故障隔离。
- [ ] 3.2 迁移 push，取消损坏改名重建；验证原文件、密钥和订阅保留以及正常缺失初始化。
- [ ] 3.3 迁移 tunnelsettings/tailscalesettings，覆盖 Save/Reset、关闭未持久化、Funnel 确认与旧覆盖 fail-closed。
- [ ] 3.4 迁移 Tailscale ownership，持久确认 pending 后才执行外部操作；验证 active、撤销和启动前对账。
- [ ] 3.5 实现破坏性文件/锁改名与旧 tailscale.json 阻止启动；测试新旧并存、损坏、不可读、无匹配入口和未验证入口。
- [ ] 3.6 runtime 采用统一写原语，保留发现记录与 TCP 降级边界；不把领域锁策略误套为全局拒绝启动。
- [ ] 3.7 同步真实清单、受影响能力 delta 与 US-K08/US-N/US-P 等已有故事；新增行为另补稳定 ID。

## 4. 配置来源与诊断（后续实施）

- [ ] 4.1 经用户确认 CLI 默认后，实现字段存在性元数据与共享纯 resolver。
- [ ] 4.2 实现 configured 只读模式与 effective 同 UID socket 快照；声明实例身份与分组观察时间。
- [ ] 4.3 增加安全 DTO、contract/wire、本机客户端和 CLI 人类/JSON 输出；API 不直接读盘。
- [ ] 4.4 测试显式默认值、CLI 覆盖、生成值、web 覆盖、磁盘漂移、活进程不可达、锁占用与安全降级。
- [ ] 4.5 在所有诊断路径植入秘密哨兵，证明成功/错误/日志均不泄漏 token、密钥、env、端点或正文。
- [ ] 4.6 同步 CLI 契约、用法与相关故事；如另做网页来源 UI，提交真实截图和浏览器测试。

## 5. 与偏好同步衔接（后续实施，不重复 #119）

- [ ] 5.1 更新 add-server-settings-sync 的锁名、提交不确定语义、共享限长读取及无兼容迁移约定。
- [ ] 5.2 settings 服务通过 descriptor/typed handle 接入；revision 与广播只在 durable commit 后前进。
- [ ] 5.3 按该变更交付偏好 API/WebSocket/前端统一入口，移除独立本地偏好写入；保留设备数据与只读缓存边界。
- [ ] 5.4 新存储与测试适配器自动进入清单；所有领域完成迁移后删除临时原始 I/O 例外。
