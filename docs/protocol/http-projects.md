# HTTP API：本地项目

选择 workspace 插件后，可先登记或创建项目，再通过现有会话接口在项目目录启动命令。
本层提供 API；新建会话弹窗尚未接入。纯核心继续直接使用 cwd 与 cmd，不依赖项目记录。

## 项目清单

`GET /api/projects` 返回项目数组，空清单为 `[]`。
每项包含 `id`、`name`、`path`、`repositoryId`、`availability` 和 `error`。
普通目录的 repositoryId 为空字符串；Git 项目引用原仓库标识，不复制 Git 状态。
不可访问的目录保留为 unavailable，附带原因，不会静默从清单消失。

`POST /api/projects` 接受 `{path}`，打开已存在的服务端目录。
新登记返回 201，重复登记返回 200；Git 可用时识别未登记的仓库，普通目录不要求 Git。
已有普通目录项目不会因其父目录是 Git 仓库而在重复登记时换成父项目。

## 创建项目目录

`POST /api/projects/create` 接受 `{parent, name, initializeGit}`，成功返回 201 和项目。
父目录必须存在，名称必须是一个合法目录名，目标已存在返回 409，不覆盖或清空文件。
initializeGit 默认行为由请求明确指定：false 仅创建目录，true 初始化 main 分支的空仓库。
初始化不创建提交，不要求配置 Git 作者；Git 缺失时在创建目录前返回不可用错误。
创建目录后后续步骤失败，错误对象带 `{error, path}`，保留目录并允许通过登记接口恢复。

## 浏览与能力

`GET /api/projects/directories?prefix=...` 返回匹配前缀的服务端目录路径数组，支持 ~ 与目录符号链接。
此接口属于 workspace，不依赖 files 的上传或目录接口。
workspace.projects 表示项目清单存储是否可用；workspace.manage 仍反映既有 Git 管理可用性。
项目接口统一使用认证及 Origin 校验；workspace 禁用时不挂载，也不打开项目文件。

## Git URL 克隆

workspace.clone 可用时，POST /api/projects/clone 接受 `{requestId, url, path}`，返回 202 和操作对象。
requestId 为 1–120 个字母、数字、下划线或短横线，首字符必须是字母或数字；未确认提交结果或查询同一操作时必须保留标识和参数，参数冲突返回 409。failed/interrupted 是终态；重新下载必须使用新的 requestId，同一标识只返回原操作，不重新执行。
URL 支持 HTTPS、SSH 及 user@host:path；拒绝内嵌凭据、查询参数、本地/ext 协议及控制字符。path 的父目录必须存在，任何既有目标均拒绝覆盖。
操作对象包含 id、state、path、error、projectId；state 为 running、registering、succeeded、failed、interrupted 或 registration_failed。
GET /api/projects/operations/{id} 查询操作，成功后以 projectId 选择项目。浏览器断连不取消操作；并发上限 2，单次网络克隆最长 10 分钟，排队不占用此期限。
克隆成功但登记失败时，目标目录保留；POST /api/projects/operations/{id}/register 返回 202 并只重试登记。已经成功的重试返回同一结果。
错误不返回 Git 输出；服务停止会取消进程组并等待 worker，再关闭状态文件。服务崩溃后的未完成操作按持久账本标为中断，不自动重新下载。

## GitHub 仓库来源

`GET /api/projects/github?query=...&page=1` 使用服务用户已有的 gh 登录查询 github.com，不启动登录或更改认证配置。
返回 `{available, reason, login, repositories, page, hasMore}`；条目包含 fullName、description、private、cloneUrl。
不填 query 时列出本人及协作仓库；填写后搜索可访问仓库名称。每页 20 条，最多 50 页；query 最长 200 字节且不能含控制字符。
服务端 gh/Git 缺失、gh 克隆语法不支持 --no-upstream（需要 2.88+）、未登录、权限不足、网络错误与超时均返回 available=false 和可读原因，不伪装成空结果；repositories 始终为数组。
每次显式打开或刷新来源才读取身份；workspace.github 能力不发起网络请求，也不检查个人开发环境账号。
请求最长 20 秒，输出上限 4 MiB；错误不回传 gh 输出，取消时清理进程组。

选择 GitHub 仓库后，克隆请求额外传 `source: "github"`，URL 限 github.com 的 HTTPS 仓库地址。
该来源通过 gh repo clone 使用已有认证，适用于私有仓库；不要求先配置全局 Git credential helper。
source 参与幂等参数比较；省略它仍使用普通 Git URL 克隆，不自动切换认证方式。
所有克隆先持久化 registering 再原子发布；发布不支持或失败不会覆盖目标。重启后若目标存在可单独重试登记，否则重新发起。

克隆传输协议限制仅作用于子进程环境，不写入新仓库的 Git 配置；已有编号环境配置保留。
