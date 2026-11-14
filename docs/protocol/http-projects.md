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
