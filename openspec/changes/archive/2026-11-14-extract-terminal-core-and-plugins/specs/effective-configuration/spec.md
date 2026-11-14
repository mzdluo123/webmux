## ADDED Requirements

### Requirement: 插件选择与实际运行状态

配置诊断 SHALL 展示插件选择的字段来源，并明确区分未声明的既有功能组合与显式空启用集。
configured SHALL 只读解析磁盘声明及依赖，不打开插件业务存储；effective SHALL 使用当前实例实际解析并采用的启用结果，不以磁盘新值替代。
禁用、配置关闭、环境不支持、存储不可用 SHALL 分别报告安全状态与受控原因，不把禁用插件的状态伪报为已初始化可用。
诊断 SHALL 不为报告健康状态而构造插件服务、打开其数据库或获取写锁。

#### Scenario: 修改插件集尚未重启

- **WHEN** 运行实例启用了聊天，而用户在磁盘改为显式空启用集
- **THEN** configured 显示仅核心，effective 仍显示当前聊天模块启用，并指出重启后生效

#### Scenario: 禁用模块的损坏状态文件

- **WHEN** 用户查询纯核心实例的 effective 配置，而磁盘有损坏的聊天数据库
- **THEN** 报告聊天模块禁用，不尝试读取数据库，不把损坏内容或凭据写入诊断

#### Scenario: 省略与显式空数组

- **WHEN** 比较未声明 plugins.enabled 与显式 plugins.enabled=[] 的配置
- **THEN** 前者显示既有功能默认组合及 default 来源，后者显示仅核心及 toml 来源
