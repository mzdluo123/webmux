## Purpose

让 Windows 用户从托盘快速获取 WSL 中 WebMux 的访问方式：打开浏览器、复制本机与远程链接、按需复制 Token，同时不降低 WebMux 既有的认证与暴露面约束。

## ADDED Requirements

### Requirement: 读取 WebMux 访问配置

托盘 SHALL 以 WSL 中设置指定的 WebMux 配置文件为唯一来源读取 `listen` 与 `token`，规则与 WebMux 启动时一致：文件不存在时使用默认 `127.0.0.1:7681` 且无 Token；文件存在但无法解析时 SHALL 报告配置错误且不生成链接。托盘 SHALL NOT 将 Token 写入任何 Windows 侧文件或日志。

配置文件变化 SHALL 在下一次打开菜单、复制或健康检查时生效；对运行中实例，托盘 SHALL 提示“配置已修改，重启 WebMux 后生效”，直到实例重启。

#### Scenario: 默认配置

- **WHEN** WSL 中不存在 WebMux 配置文件
- **THEN** 本机链接为 `http://127.0.0.1:7681`，菜单不显示“复制 Token”和“复制远程链接”

#### Scenario: 运行中修改监听地址

- **WHEN** WebMux 运行中，用户把配置中的 `listen` 改为 `100.64.0.5:7681`
- **THEN** 状态行提示配置已修改需重启生效，复制的链接仍对应当前运行实例的地址

### Requirement: 本机链接

托盘 SHALL 生成一个可在 Windows 浏览器中直接打开的本机链接：

- `listen` 为回环地址或 `localhost` 时，链接主机为 `127.0.0.1`（IPv6 回环为 `[::1]`），端口与 `listen` 一致；
- `listen` 为通配地址（`0.0.0.0`、`::` 或空主机）时，链接主机为 `127.0.0.1`；
- `listen` 为具体非回环地址时，本机链接使用该地址。

链接 SHALL 使用 `http` 协议、无路径和查询参数，且 SHALL NOT 包含 Token。“打开 WebMux”SHALL 用 Windows 默认浏览器打开本机链接。

托盘 SHALL 用 Windows 侧对本机链接的健康检查验证可达性；WebMux 进程在 WSL 中仍在运行但 Windows 侧不可达时，SHALL 提示检查 WSL localhost 转发或网络模式设置，而不是显示为已停止。

#### Scenario: 默认回环监听

- **WHEN** `listen = "127.0.0.1:7681"`
- **THEN** “复制本机链接”写入剪贴板 `http://127.0.0.1:7681`

#### Scenario: Windows 侧不可达

- **WHEN** WebMux 进程在发行版内运行，但 Windows 侧访问 `127.0.0.1:7681` 连接失败
- **THEN** 状态行提示“WebMux 已在 WSL 中运行，但 Windows 无法访问 127.0.0.1:7681，请检查 WSL localhostForwarding / networkingMode”

### Requirement: 远程链接

当 `listen` 为具体的非回环地址时，菜单 SHALL 显示“复制远程链接”，内容为 `http://<该地址>:<端口>`。当 `listen` 为通配地址时，“复制远程链接” SHALL 展开为子菜单，列出 WSL 发行版内与 Windows 主机上可用于远程访问的非回环 IPv4 地址（标注所属网卡，Tailscale 地址置顶）；没有可用地址时子菜单显示“无可用地址”。

`listen` 为通配地址且未配置 Token 时，菜单 SHALL 在远程链接条目旁显示“未设置 Token，任何能访问该端口的人都可获得 shell”的警告。

#### Scenario: Tailscale 地址监听

- **WHEN** `listen = "100.64.0.5:7681"` 且配置了 Token
- **THEN** “复制远程链接”写入 `http://100.64.0.5:7681`，本机链接也为该地址

#### Scenario: 通配监听且无 Token

- **WHEN** `listen = "0.0.0.0:7681"` 且 `token = ""`
- **THEN** 远程链接子菜单顶部显示无 Token 警告，其下列出可用地址

### Requirement: 复制 Token

仅当配置了非空 Token 时，菜单 SHALL 显示“复制 Token”。复制的内容 SHALL 只有 Token 本身。写入剪贴板时 SHALL 标记为不进入 Windows 剪贴板历史与跨设备云剪贴板；托盘 SHALL 在 60 秒后若剪贴板内容仍为该 Token 则清空剪贴板。

托盘 SHALL NOT 在菜单、悬停提示、通知或日志中显示 Token 明文。

#### Scenario: 复制 Token 后自动清除

- **WHEN** 用户点击“复制 Token”且 60 秒内未复制其他内容
- **THEN** 60 秒后剪贴板为空，Windows 剪贴板历史（Win+V）中不出现该 Token

#### Scenario: 期间复制了其他内容

- **WHEN** 用户复制 Token 后 30 秒内又复制了一段普通文本
- **THEN** 60 秒到期时剪贴板中的普通文本保持不变

### Requirement: 复制反馈

每次复制操作成功后，托盘 SHALL 给出简短的 Windows 通知（例如“已复制本机链接”），复制 Token 的通知 SHALL NOT 包含 Token；剪贴板被其他程序占用导致写入失败时 SHALL 通知失败原因。

#### Scenario: 复制成功提示

- **WHEN** 用户点击“复制本机链接”
- **THEN** 出现“已复制 http://127.0.0.1:7681”的通知
