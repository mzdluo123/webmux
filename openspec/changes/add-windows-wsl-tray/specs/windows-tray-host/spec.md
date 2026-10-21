## Purpose

提供运行在 Windows 通知区域的 WebMux 托盘程序：以单实例常驻，用图标和菜单呈现 WSL 中 WebMux 的状态，并承载托盘自身的设置、日志、登录自动启动和退出行为。

## ADDED Requirements

### Requirement: 支持平台与运行形态

托盘程序 SHALL 作为独立的 Windows 图形子系统可执行文件提供，支持 Windows 10 22H2 及以上与 Windows 11 的 amd64、arm64 架构，并要求系统已安装 WSL 2。托盘程序运行时 SHALL NOT 显示控制台窗口；其调用的所有 WSL 命令 SHALL NOT 弹出控制台窗口。

托盘程序 SHALL NOT 要求管理员权限，SHALL 以当前登录用户身份运行，且 SHALL NOT 依赖 Node.js、Go 或其他运行时。

#### Scenario: 普通用户启动

- **WHEN** 非管理员用户双击 `webmux-tray.exe`
- **THEN** 通知区域出现 WebMux 图标，期间不出现任何控制台窗口或 UAC 提示

#### Scenario: 未安装 WSL

- **WHEN** 系统中找不到可用的 `wsl.exe` 或没有任何已安装的发行版
- **THEN** 托盘图标显示异常状态，菜单状态行说明“未检测到 WSL 发行版”，启动类操作禁用，“打开日志”和“退出”保持可用

### Requirement: 单实例

同一 Windows 用户会话中 SHALL 至多运行一个托盘实例。再次启动托盘程序时，新进程 SHALL 通知已运行实例弹出状态提示后立即退出，不创建第二个图标。

#### Scenario: 重复启动

- **WHEN** 托盘已在运行，用户再次从开始菜单启动托盘程序
- **THEN** 仍只有一个托盘图标，已有实例显示一条包含当前状态的通知，新进程以退出码 0 结束

### Requirement: 状态图标与悬停提示

托盘图标 SHALL 区分四种状态：已停止、启动中（含停止中/重启中）、运行中、异常。悬停提示 SHALL 显示“WebMux”、当前状态，以及运行中时的会话数量和本机链接地址，且 SHALL NOT 包含 Token。

状态 SHALL 以 WebMux 健康检查结果为准，而不是仅以进程是否存在为准：进程存在但健康检查连续失败 SHALL 显示为异常。

#### Scenario: 运行中的悬停提示

- **WHEN** WebMux 健康运行且有 3 个会话，监听 `127.0.0.1:7681`
- **THEN** 悬停提示显示 “WebMux · 运行中 · 3 个会话 · http://127.0.0.1:7681”

#### Scenario: 健康检查失败

- **WHEN** WebMux 进程仍在运行，但健康检查连续 3 次失败
- **THEN** 图标切换为异常状态，菜单状态行显示最近一次失败原因

### Requirement: 托盘菜单结构

左键单击图标 SHALL 打开 WebMux（与菜单“打开 WebMux”相同），右键单击 SHALL 弹出菜单。菜单 SHALL 按以下分组依次包含：

1. 不可点击的状态行：状态、会话数、WebMux 版本、发行版名称。
2. 访问：打开 WebMux、复制本机链接、复制远程链接（仅在存在远程链接时显示）、复制 Token（仅在配置了 Token 时显示）。
3. 服务：启动、停止、重启。
4. 维护：打开日志、打开 WebMux 配置文件、发行版（子菜单）、编辑托盘设置。
5. 登录时自动启动（复选项）。
6. 退出。

菜单项 SHALL 按当前状态启用或禁用：运行中禁用“启动”；已停止禁用“停止”“重启”和访问类条目；启动中/停止中禁用所有服务类条目。菜单打开时 SHALL 反映最新状态，不要求用户重新打开菜单。

#### Scenario: 已停止时的菜单

- **WHEN** WebMux 已停止，用户右键托盘图标
- **THEN** “启动”可用，“停止”“重启”“打开 WebMux”“复制本机链接”禁用，“打开日志”“退出”可用

#### Scenario: 左键打开

- **WHEN** WebMux 运行中，用户左键单击托盘图标
- **THEN** 默认浏览器打开 WebMux 本机链接

### Requirement: 托盘设置

托盘 SHALL 在当前用户的应用数据目录中保存自身设置，至少包含：目标 WSL 发行版（空表示系统默认发行版）、WSL 内 `webmux` 可执行文件路径（默认 `~/.local/bin/webmux`）、WSL 内 WebMux 配置文件路径（默认 `~/.config/webmux/config.toml`）、托盘启动时是否自动启动 WebMux（默认是）、意外退出时是否自动重启（默认是）。

托盘设置 SHALL NOT 保存 WebMux Token。设置文件缺失时 SHALL 使用默认值；内容损坏或不符合约束时 SHALL 使用默认值运行、在状态行和托盘日志中说明，并 SHALL NOT 覆盖原文件，直到用户通过托盘修改设置。

“编辑托盘设置” SHALL 用系统文本编辑器打开设置文件；托盘 SHALL 在文件保存后自动重新加载，无需重启托盘。路径类设置变更 SHALL 在下次启动 WebMux 时生效，并 SHALL NOT 自动停止正在运行的实例。

#### Scenario: 首次运行

- **WHEN** 设置文件不存在，用户首次启动托盘
- **THEN** 托盘使用默认发行版、`~/.local/bin/webmux` 和 `~/.config/webmux/config.toml` 启动 WebMux，并且不在磁盘上写入 Token

#### Scenario: 设置文件损坏

- **WHEN** 用户把设置文件改成无法解析的内容并保存
- **THEN** 托盘继续以默认值运行，状态行提示“托盘设置无效，已使用默认值”，原文件内容保持不变

### Requirement: 发行版选择

“发行版”子菜单 SHALL 列出当前用户已安装的 WSL 发行版，并以单选标记当前目标发行版；首项为“系统默认（<名称>）”。切换发行版时，若 WebMux 正在运行，托盘 SHALL 先询问是否停止当前实例（说明会结束所有会话）；用户取消则不切换。

托盘 SHALL 正确识别包含非 ASCII 字符的发行版名称。

#### Scenario: 切换发行版

- **WHEN** WebMux 在 `Ubuntu` 中运行且有 2 个会话，用户在子菜单选择 `Debian`
- **THEN** 托盘提示停止将结束 2 个会话；确认后停止 `Ubuntu` 中的实例并在 `Debian` 中启动，取消则保持原状

### Requirement: 登录时自动启动

“登录时自动启动”复选项 SHALL 在当前 Windows 用户的登录启动项中注册或注销托盘程序，不需要管理员权限。复选状态 SHALL 反映启动项实际是否存在且指向当前托盘可执行文件；指向其他路径时（例如移动了可执行文件）SHALL 显示为未勾选，再次勾选时更新为当前路径。

自动启动只启动托盘；托盘随后是否启动 WebMux 由托盘设置中的“托盘启动时自动启动 WebMux”决定。

#### Scenario: 启用自动启动

- **WHEN** 用户勾选“登录时自动启动”后注销并重新登录 Windows
- **THEN** 托盘自动出现，并在 WSL 中启动 WebMux，期间无控制台窗口

#### Scenario: 可执行文件被移动

- **WHEN** 启动项指向的旧路径已不是当前托盘程序
- **THEN** 菜单中复选项显示为未勾选；用户勾选后启动项指向当前路径

### Requirement: 托盘日志

托盘 SHALL 将自身运行事件（状态变化、WSL 命令的退出码与错误、健康检查失败原因、自动重启）写入当前用户本地应用数据目录下的日志文件，单文件大小 SHALL 有上限并保留至少一个历史文件。托盘日志 SHALL NOT 记录 Token、请求认证头或任何终端输入输出。

#### Scenario: 日志中不含 Token

- **WHEN** WebMux 配置了 Token，托盘完成多次启动、健康检查和复制 Token 操作
- **THEN** 托盘日志文件中不出现 Token 字符串

### Requirement: 退出托盘

选择“退出”时，若 WebMux 由本托盘启动且正在运行，托盘 SHALL 让用户在“仅退出托盘（WebMux 继续运行）”“停止 WebMux 并退出”“取消”之间选择；选择停止时 SHALL 按停止流程完成后再退出。WebMux 未运行或不是由本托盘启动时，托盘 SHALL 直接退出且不影响 WebMux。

Windows 注销或关机时，托盘 SHALL 直接退出，不弹出询问。

#### Scenario: 仅退出托盘

- **WHEN** WebMux 运行中，用户选择“退出 → 仅退出托盘”
- **THEN** 托盘图标消失，浏览器中已打开的 WebMux 会话继续可用；再次启动托盘后显示为运行中且不重复启动
