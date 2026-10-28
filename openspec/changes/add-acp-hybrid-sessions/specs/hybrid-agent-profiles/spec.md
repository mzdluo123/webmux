## Purpose

内置 agent 档案描述 webmux 如何识别、启动、恢复和中断某个支持 ACP 的 agent，让混合会话在 TUI 与 ACP 两端使用一致的命令、参数与环境。新增 agent 时只需新增档案，不改变会话编排。

## ADDED Requirements

### Requirement: 内置档案与可识别的命令写法

系统 SHALL 内置 OpenCode 与 omp 两个档案，第一期 SHALL NOT 开放用户自定义档案。启动配置的 argv 只有在以下两种写法之一中，且可执行文件的基本名与某个档案匹配时，才 SHALL 视为可用于混合模式：

1. 直接执行：`[<可执行文件>, 参数…]`；
2. webmux 的 bashrc 包装：`["bash", "-ic", "exec -- \"$@\"", "webmux", <可执行文件>, 参数…]`。

其他任何写法（例如其他 shell 包装、`env` 前缀）SHALL 视为不可用，并给出原因“命令写法不受支持”。

#### Scenario: 预置的 bashrc 包装

- **WHEN** 配置的 argv 为 `["bash","-ic","exec -- \"$@\"","webmux","omp"]`
- **THEN** 判定为可用，档案为 omp

#### Scenario: 自定义包装

- **WHEN** 配置的 argv 为 `["sh","-c","omp"]`
- **THEN** 判定为不可用，原因为命令写法不受支持

### Requirement: 恢复命令的生成

切换或恢复到 TUI 时，系统 SHALL 按以下方式生成恢复命令：

- 由可执行文件、原 argv 中位于档案白名单内的参数、档案的恢复参数（OpenCode 为 `-s <id>`，omp 为 `--resume <id>`）组成，并套用与原命令相同的写法（直接执行或 bashrc 包装）。
- 白名单参数 SHALL 同时识别 `--flag value` 与 `--flag=value` 两种形式，并按档案声明的参数个数解析。
- 系统 SHALL 丢弃以下参数：位置参数（例如提示词）、会话类参数（`-c`、`--continue`、`-r`、`--resume`、`-s`、`--session`、`--fork` 等）、白名单之外的参数。
- 被丢弃的参数 SHALL 在切换结果中列出，界面 SHALL 提示“以下参数在恢复时未保留”。

#### Scenario: 丢弃提示词与会话参数

- **WHEN** 原 argv 为 `["omp","--model","haiku","--continue","修 bug"]`，agent 会话 ID 为 `01abc`
- **THEN** 恢复命令为 `["omp","--model","haiku","--resume","01abc"]`，界面列出未保留的 `--continue` 与 `修 bug`

### Requirement: ACP 启动命令

系统 SHALL 按档案启动 ACP 进程：OpenCode 为 `opencode acp`，omp 为 `omp acp`，并套用与原命令相同的写法。

档案声明的自动批准类参数 SHALL 从原 argv 透传给 ACP 进程（omp 为 `--approval-mode`、`--yolo`、`--auto-approve`、`--config`）。档案没有对应 ACP 参数的（例如 OpenCode 的 `--auto`）SHALL 被丢弃，并在界面上提示。

使用 bashrc 包装时，`.bashrc` 执行期间写到 stdout 的内容 SHALL NOT 进入 ACP 协议通道。

#### Scenario: bashrc 有输出

- **WHEN** 用户的 `.bashrc` 会 `echo` 一行欢迎语，混合会话从聊天开始
- **THEN** ACP 握手成功，欢迎语不导致协议错误

### Requirement: 两端环境一致

ACP 进程、恢复后的 TUI 与最初的 TUI SHALL 使用同一套环境：

- 启动配置保存的环境变量；
- 服务进程环境，按现有规则移除宿主终端变量；
- webmux 注入的变量，包括同一个 `WEBMUX_SESSION_ID`。

`webmux run --launch-config` 创建的混合会话同样 SHALL NOT 继承调用方 shell 的环境。

#### Scenario: 配置文件目录变量

- **WHEN** 混合配置设置了 `PI_CODING_AGENT_DIR=/tmp/agentdir`
- **THEN** ACP 进程与恢复后的 TUI 都能看到同样的值，并能列出同一目录中的会话

### Requirement: 档案与混合开关的校验

保存启动配置时，若 `hybrid: true` 而 argv 不满足可识别的写法，服务端 SHALL 返回 400，并给出不可用原因。启动配置摘要与完整配置 SHALL 提供只读字段 `hybridSupport`，内容为 `{supported, agent?, reason?}`，供设置页决定勾选框是否可用及显示原因。

#### Scenario: 对 shell 配置勾选

- **WHEN** 客户端保存 `{name: "Shell", cmd: ["/bin/bash","-l"], hybrid: true}`
- **THEN** 服务端返回 400，原因为未识别到支持的 agent

### Requirement: 版本提示

agent 的版本来自终端信号或 ACP `initialize` 返回的 `agentInfo.version`。版本低于档案记录的实测版本（OpenCode 2.1.4、omp 18.13.1）时，系统 SHALL 在会话页显示提示，SHALL NOT 因此阻止切换；能否切换只由资格判定决定。版本无法解析时 SHALL 不提示。

#### Scenario: 旧版本

- **WHEN** ACP `initialize` 返回 omp 版本 18.0.0
- **THEN** 聊天视图显示“低于已验证版本 18.13.1”的提示，其余功能照常
