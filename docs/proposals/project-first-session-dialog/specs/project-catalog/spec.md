## Purpose

为会话提供持久、统一的项目目录入口，让已有 Git 仓库和普通文件夹都能作为项目，并在新建会话流程内完成新建、本地打开、远程克隆和 GitHub 仓库发现。项目存在不依赖会话进程存活，也不改变会话的服务进程内生命周期。

## ADDED Requirements

### Requirement: 持久项目目录

系统 SHALL 保存稳定项目标识、名称、规范目录及可选仓库关联。已登记仓库 SHALL 映射为项目并保留原仓库标识。相同规范项目目录 SHALL 去重；非 Git 文件夹 SHALL 无需 Git 也可作为项目。服务重启 SHALL 恢复项目清单但不恢复会话。

#### Scenario: 普通项目重启恢复

- **WHEN** 用户登记非 Git 文件夹并重启服务
- **THEN** 项目仍在列表，可在该目录启动会话，旧 PTY 不恢复

#### Scenario: 重复添加

- **WHEN** 用户通过符号链接再次选择同一项目目录
- **THEN** 返回已有项目，不产生重复条目，不覆盖已有名称或仓库初始化指令

### Requirement: 四种来源逐层接入

添加项目 SHALL 提供新建项目、本地文件夹、Git URL 和 GitHub 仓库四种来源。新建项目 SHALL 默认只创建目录，明确勾选初始化 Git 才执行初始化。本地文件夹 SHALL 浏览服务端文件系统，不上传浏览器设备文件。成功添加 SHALL 进入会话设置，不直接启动 agent。

#### Scenario: 新项目不初始化 Git

- **WHEN** 用户输入名称及父目录并保持初始化 Git 关闭
- **THEN** 创建普通项目目录并进入会话设置，不生成 Git 元数据

#### Scenario: 打开文件夹

- **WHEN** 用户确认一个可访问的服务端文件夹
- **THEN** 系统登记项目，识别其可用 Git 归属，进入会话设置并显示实际路径

### Requirement: 安全且可恢复的克隆

Git URL 与 GitHub 来源 SHALL 在提交前展示远程仓库与目标目录。系统 SHALL 拒绝覆盖既有目标、带明文凭据的 URL 和不支持的协议，不将输入作为 shell 命令。操作 SHALL 返回可查询的稳定操作标识，重复请求不得重复克隆。
失败 SHALL 保留输入并解释原因；只清理该操作自己的临时资源。克隆完成而登记失败 SHALL 保留目录并允许重新登记，不要求再次克隆。完成添加后关闭弹窗 SHALL NOT 删除目录。

#### Scenario: 目标已存在

- **WHEN** 用户提交克隆到已存在目录
- **THEN** 提示冲突并保留远程地址与目录，既有文件不变

#### Scenario: 断线后恢复

- **WHEN** 克隆进行中浏览器断连后重连
- **THEN** 查询既有操作状态，不重新启动克隆；成功后可进入会话设置

#### Scenario: 登记失败

- **WHEN** 克隆成功而项目持久登记失败
- **THEN** 显示已克隆目录与重新登记动作，保留磁盘内容

### Requirement: GitHub 来源局部降级

GitHub 来源 SHALL 使用服务端已配置的 GitHub 身份展示有权访问的仓库，支持搜索和分页，条目显示 owner/name、描述及私有状态。缺少工具、未登录或权限不足 SHALL 显示具体配置原因及重试，不将错误表示为空列表；其他来源仍可使用。

#### Scenario: 未配置 GitHub

- **WHEN** 服务端缺少 gh 或未登录
- **THEN** GitHub 来源显示需要配置及原因，不阻止本地文件夹和 Git URL 操作

#### Scenario: 选择私有仓库

- **WHEN** 当前服务端身份有权访问一个私有仓库且用户选择它
- **THEN** 显示其名称、私有标记及目标目录，确认后使用服务端凭据克隆，不把 token 返回浏览器

### Requirement: 沿用服务安全与错误约定

项目读取、目录创建、登记、克隆和 GitHub 发现 SHALL 服从既有认证与 Origin 规则。系统 SHALL 对外部调用施加期限和取消，SHALL NOT 日志记录凭据或在配置响应中暴露凭据。移除项目记录 SHALL NOT 自动删除目录、仓库、分支或结束会话。

#### Scenario: 非授权来源写入

- **WHEN** 未认证或非法 Origin 请求创建项目
- **THEN** 请求被拒绝，项目清单、文件与会话不变

#### Scenario: 移除项目

- **WHEN** 用户移除项目记录
- **THEN** 目录和运行会话保持不变

### Requirement: 项目能力服从功能选择

项目目录能力 SHALL 属于 workspace 可选功能，不新增核心必选依赖。未选择 workspace 时 SHALL NOT 加载项目模块、请求项目接口、打开项目存储或探测 Git/gh。默认纯核心及固定 legacy 功能选择语义 SHALL 保持不变。
本地项目能力 SHALL 独立于 Git 可用性；克隆与 GitHub 发现 SHALL 分别报告可用性。workspace SHALL NOT 新增对 files、launch 或 chat 的硬依赖。

#### Scenario: 纯核心启动

- **WHEN** features.enabled 为显式空列表
- **THEN** 用户仍能以目录和完整命令启动终端，不要求先添加项目，也不打开项目存储

#### Scenario: 只有 workspace

- **WHEN** 仅选择 workspace 且 Git/gh 不可用
- **THEN** 本地项目目录仍可登记和使用，Git 来源解释不可用原因，不要求启用 files、launch、chat

#### Scenario: 项目存储不可用

- **WHEN** workspace 已选择但项目清单读取失败
- **THEN** 项目区域显示错误并提供核心目录＋命令创建回退，不报告空列表
