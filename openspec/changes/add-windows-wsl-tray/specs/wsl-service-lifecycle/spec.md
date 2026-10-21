## Purpose

规定托盘程序如何在 WSL 发行版中启动、保持、监控、停止和重启 WebMux，使 Windows 用户登录后即可获得一个持续运行、可安全停止的 WebMux 服务。

## ADDED Requirements

### Requirement: 在 WSL 中启动 WebMux

托盘 SHALL 在目标发行版中以当前 WSL 默认用户启动设置中指定的 `webmux` 可执行文件，并以 `--config <配置路径>` 传入设置中的配置文件。启动命令 SHALL 以参数数组传递，用户提供的路径 SHALL NOT 被 shell 解释为命令或通配符；仅允许 `~/` 前缀展开为 WSL 用户主目录。

启动前托盘 SHALL 检查可执行文件存在且可执行；不存在时 SHALL 进入异常状态并在状态行说明缺失的路径，SHALL NOT 反复重试。

WebMux 的标准输出与标准错误 SHALL 写入 WSL 内的服务端日志文件（与 WebMux 状态目录同级的固定位置），每次启动前保留上一次日志为历史文件；托盘退出或断开 SHALL NOT 导致 WebMux 因输出管道关闭而退出。

#### Scenario: 正常启动

- **WHEN** 已停止状态下用户点击“启动”
- **THEN** 图标进入启动中，WebMux 在目标发行版中运行，健康检查成功后图标变为运行中

#### Scenario: 可执行文件缺失

- **WHEN** 设置中的 `webmux` 路径在发行版中不存在
- **THEN** 图标显示异常，状态行提示“找不到 ~/.local/bin/webmux”，托盘不进入自动重启循环

#### Scenario: 路径包含空格与特殊字符

- **WHEN** 配置路径为 `~/my configs/webmux;$(id).toml`
- **THEN** WebMux 收到的 `--config` 参数值为该路径的字面值（主目录展开后），不执行任何额外命令

### Requirement: 保持发行版存活

在 WebMux 由托盘启动并运行期间，目标 WSL 发行版 SHALL NOT 因“没有活动的 WSL 客户端”而被自动关闭；浏览器断开或关闭所有 Windows 终端窗口 SHALL NOT 结束 WebMux。

#### Scenario: 关闭所有终端窗口

- **WHEN** WebMux 运行中，用户关闭所有 Windows Terminal 与 WSL 窗口并等待 5 分钟
- **THEN** WebMux 仍在运行，已有会话可从浏览器重新连接

### Requirement: 就绪判定

启动后托盘 SHALL 从 Windows 侧按配置推导的本机地址轮询 `/api/health`（配置了 Token 时携带 Bearer 认证），在返回成功后判定为运行中。在 60 秒内未就绪时 SHALL 进入异常状态并在状态行给出最后一次失败原因（连接被拒、认证失败、超时等）及“打开日志”提示，但 SHALL NOT 自动结束已启动的进程。

健康检查返回 401 时 SHALL 报告“Token 不匹配”，而不是“未运行”。

#### Scenario: 启动超时

- **WHEN** WebMux 进程已启动但 60 秒内健康检查始终连接失败
- **THEN** 图标显示异常，状态行提示“WebMux 未在 60 秒内就绪：连接被拒绝”，菜单“打开日志”可用

#### Scenario: 运行中定期检查

- **WHEN** WebMux 运行中
- **THEN** 托盘至少每 10 秒检查一次健康状态并更新会话数，且每次检查的超时不超过 3 秒

### Requirement: 接管已运行的实例与端口冲突

托盘启动或用户点击“启动”时，若配置地址上已有健康检查成功的 WebMux（例如由 systemd 用户服务或手动启动），托盘 SHALL 直接显示运行中并以“外部实例”标记，SHALL NOT 再启动第二个实例。对外部实例，托盘 SHALL 只提供访问类操作和状态监控；“停止”“重启” SHALL 禁用并在状态行说明“由外部管理”。

若配置地址已被占用但不是 WebMux（健康检查返回非预期响应），托盘 SHALL 进入异常状态并提示端口被占用，SHALL NOT 启动新进程。

托盘重新启动后，SHALL 能识别此前由托盘启动且仍在运行的实例，并恢复对其的停止与重启控制。

#### Scenario: systemd 已在运行

- **WHEN** 发行版内 `systemctl --user` 已运行 WebMux，用户启动托盘
- **THEN** 托盘显示运行中（外部实例），“打开 WebMux”可用，“停止”“重启”禁用

#### Scenario: 托盘重启后恢复控制

- **WHEN** 用户“仅退出托盘”后重新启动托盘
- **THEN** 托盘显示运行中，“停止”“重启”可用

### Requirement: 停止与重启

停止 SHALL 先向 WebMux 发送 SIGTERM，让其自行清理会话与子进程；15 秒内未退出时 SHALL 强制结束。停止前托盘 SHALL 校验目标进程确实是本托盘启动的 WebMux，SHALL NOT 向无关进程发送信号。

当前会话数大于 0 时，停止与重启 SHALL 先弹出确认，说明将结束的会话数量且不可恢复；会话数为 0 时 SHALL 直接执行。重启 SHALL 等价于停止完成后再启动。

用户主动停止后，托盘 SHALL NOT 自动重启 WebMux。

#### Scenario: 有会话时停止

- **WHEN** WebMux 有 4 个会话，用户点击“停止”
- **THEN** 托盘提示“停止 WebMux 将结束 4 个终端会话”，确认后 WebMux 正常退出、会话进程被清理，图标变为已停止

#### Scenario: 停止超时

- **WHEN** WebMux 收到 SIGTERM 后 15 秒仍未退出
- **THEN** 托盘强制结束该进程，图标变为已停止，托盘日志记录强制结束

### Requirement: 意外退出与自动重启

WebMux 非因用户停止而退出（崩溃、被杀、`wsl --shutdown`、发行版终止）时，托盘 SHALL 发出 Windows 通知说明 WebMux 已退出及会话已丢失。设置允许自动重启时，托盘 SHALL 以指数退避（首次 2 秒，上限 60 秒）重新启动；10 分钟内连续失败 5 次后 SHALL 停止自动重启、进入异常状态，直到用户手动启动。

#### Scenario: wsl --shutdown

- **WHEN** WebMux 运行中，用户在 PowerShell 执行 `wsl --shutdown`
- **THEN** 托盘通知 WebMux 已退出，约 2 秒后重新启动，图标恢复为运行中

#### Scenario: 持续崩溃

- **WHEN** WebMux 每次启动后立即以非零码退出
- **THEN** 托盘在 5 次失败后停止重试，图标显示异常，状态行提示“多次启动失败，请查看日志”

### Requirement: 托盘启动时自动启动 WebMux

托盘启动时，若设置“托盘启动时自动启动 WebMux”为是且未检测到已运行的实例，托盘 SHALL 自动执行启动流程。WSL 尚在初始化（例如刚登录时）导致的首次失败 SHALL 计入自动重启退避，而不是直接放弃。

#### Scenario: 登录后 WSL 冷启动较慢

- **WHEN** Windows 登录后托盘立即启动，WSL 虚拟机需要 20 秒才能就绪
- **THEN** 托盘显示启动中直到 WebMux 健康检查成功，不出现失败通知
