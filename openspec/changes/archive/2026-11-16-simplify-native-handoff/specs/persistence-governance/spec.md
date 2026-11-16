## MODIFIED Requirements

### Requirement: Reviewed inventory coverage

仓库 SHALL 有真实存储清单，覆盖文件、锁、所有者、故障行为与网页可写边界，并与文档表格机械比较。
尚未实现的 OpenSpec 存储 MUST 不冒充现有条目。非服务器状态 SHALL 以有原因的精确引用登记，而非目录级排除。
WebMux conversation SQLite 停用后，系统 SHALL 同步移除其运行时 descriptor、契约适配器和活跃存储清单行。
遗留数据库原件 SHALL 不作为当前存储登记，也 SHALL 不被启动流程打开、迁移或删除；废弃说明与活跃清单 SHALL 区分。
Agent 原生 SQLite 的只读访问 SHALL 继续准确登记，不得以移除 WebMux 状态库为由扩大例外或删除原生读取能力。

#### Scenario: Added persistence reference

- **WHEN** 生产 Go 源码新增未登记 JSON/锁名字或受监控文件 I/O 引用
- **THEN** make persistence-check 与 CI 失败，指出具体文件、函数或路径及登记要求

#### Scenario: Existing helper grows

- **WHEN** 已登记函数增加一次相同写函数引用
- **THEN** 引用计数变化导致失败，不能因函数已经被允许而静默放行

#### Scenario: Removed implementation or stale documentation

- **WHEN** 删除了存储源码引用，或文档遗漏、多出或改变清单行
- **THEN** 守卫失败，拒绝积累无效登记

#### Scenario: Retired conversation store

- **WHEN** 新版在存在旧 conversations.sqlite 的状态目录启动
- **THEN** 不构造旧存储，不要求其锁或契约适配器，也不将其列为活跃状态存储；其他存储守卫保持有效

#### Scenario: Native SQLite read remains reviewed

- **WHEN** Agent reader 访问受支持的原生 SQLite 历史
- **THEN** 只读访问仍受现有来源和持久化边界守卫约束，不获得 WebMux 状态写入豁免
