# hybrid-agent-profiles Specification Delta

## ADDED Requirements

### Requirement: Antigravity CLI 的 TUI 会话标识注入

Antigravity CLI（`agy`）的 TUI 原生通过 `OSC 7501` 根记录上报 `app=antigravity` 与运行状态，但不在根记录中携带会话 ID。混合会话中 `agy` 的每一次 TUI 启动（包括首次启动与恢复）SHALL 注入 `--settings` 会话级设置，配置 `title` 命令在状态变化时将 `conversation_id` 作为 `OSC 7501` 子记录写入控制终端，且不向标准输出打印内容以免覆盖终端标题；原 argv 中的 `--settings` SHALL 作为丢弃参数省略并在切换结果中提示。首次启动时注入的参数 SHALL 在会话信息的命令中可见。

#### Scenario: agy 首次启动与恢复均注入会话级设置

- **WHEN** 混合配置的 argv 为 `["agy","--dangerously-skip-permissions","--settings","{}"]`，agent 会话 ID 为 `89c9a933-60f6-46ad-977b-a8daf22f8e4d`，用户从聊天切到终端
- **THEN** 恢复命令包含注入的 `--settings <title 配置>`、保留的 `--dangerously-skip-permissions` 与 `--conversation 89c9a933-60f6-46ad-977b-a8daf22f8e4d`，界面列出未保留的 `--settings {}`

## MODIFIED Requirements

### Requirement: 内置档案与可识别的命令写法

系统 SHALL 内置 OpenCode、omp 与 Antigravity CLI（`agy`）三个档案，第一期 SHALL NOT 开放用户自定义档案。启动配置的 argv 只有在以下两种写法之一中，且可执行文件的基本名与某个档案匹配时，才 SHALL 视为可用于混合模式：

1. 直接执行：`[<可执行文件>, 参数…]`；
2. webmux 的 bashrc 包装：`["bash", "-ic", "exec -- \"$@\"", "webmux", <可执行文件>, 参数…]`。

其他任何写法（例如其他 shell 包装、`env` 前缀）SHALL 视为不可用，并给出原因“命令写法不受支持”。

#### Scenario: 预置的 bashrc 包装

- **WHEN** 配置的 argv 为 `["bash","-ic","exec -- \"$@\"","webmux","omp"]`
- **THEN** 判定为可用，档案为 omp

#### Scenario: agy 的 bashrc 包装

- **WHEN** 配置的 argv 为 `["bash","-ic","exec -- \"$@\"","webmux","agy","--dangerously-skip-permissions"]`
- **THEN** 判定为可用，档案为 `agy`

#### Scenario: 自定义包装

- **WHEN** 配置的 argv 为 `["sh","-c","omp"]`
- **THEN** 判定为不可用，原因为命令写法不受支持

### Requirement: 使两端看到不同会话库或目录的参数

以下参数 SHALL 使配置不可用于混合模式：

- 让 TUI 与 ACP 看到不同会话库的参数，原因为“该参数使聊天视图无法找到会话”：
  - omp 的 `--session-dir` 只对 ACP 新建的会话生效，ACP 的列出、加载与恢复始终使用默认目录；`--no-session` 不保存会话；
  - OpenCode 的 `--server` 使 TUI 连接另一个服务及其会话库；
  - `agy` 的 `--project` 与 `--new-project` 使用云端项目会话，忽略本地 `--conversation` 会话恢复。
- 在命令中指定工作目录的参数，原因为“请在新建会话时选择目录，不要在命令中写目录”：OpenCode 的位置参数，以及 omp 的 `--cwd`。

omp 的 `--profile` SHALL NOT 视为不可用：它同时透传给 ACP 进程，两端使用同一个配置档的会话库。

#### Scenario: OpenCode 命令中写了目录

- **WHEN** 用户对 argv 为 `["opencode","/home/u/proj"]` 的配置勾选混合模式
- **THEN** 保存返回 400，原因提示在新建会话时选择目录

#### Scenario: agy 使用云端项目参数

- **WHEN** 用户对 argv 为 `["agy","--project","my-cloud-proj"]` 的配置勾选混合模式
- **THEN** 保存返回 400，原因为该参数使聊天视图无法找到会话

### Requirement: 不启动交互界面的参数

以下参数 SHALL 使配置不可用于混合模式，原因为“该参数不启动交互界面”：

- omp 的 `-p`/`--print`、`--mode`、`--export`、`--alias`、`-h`/`--help`、`-v`/`--version`，以及首个位置参数为 omp 子命令（例如 `commit`、`acp`）的写法；
- OpenCode 的 `-h`/`--help`、`-v`/`--version`、`--completions`、`--wizard`；
- `agy` 的 `-p`/`--print`/`--prompt`、`--output-format`、`--input-format`、`--json-schema`、`--print-timeout`、`--disable-slash-commands`、`-h`/`--help`、`-v`/`--version`，以及首个位置参数为 `agy` 子命令（`acp`、`agent`、`agents`、`changelog`、`help`、`install`、`mcp`、`mic-serve`、`models`、`plugin`、`plugins`、`remote-control`、`update`）的写法。

#### Scenario: 非交互用法

- **WHEN** 用户对 argv 为 `["omp","-p","总结改动"]` 的配置勾选混合模式
- **THEN** 保存返回 400，原因为该参数不启动交互界面

#### Scenario: agy 非交互或子命令用法

- **WHEN** 用户对 argv 为 `["agy","-p","总结改动"]` 或 `["agy","mcp"]` 的配置勾选混合模式
- **THEN** 保存返回 400，原因为该参数不启动交互界面

### Requirement: 恢复命令的生成

切换或恢复到 TUI 时，系统 SHALL 按以下方式生成恢复命令：

- 由可执行文件、档案注入的 TUI 参数、原 argv 中位于档案白名单内的参数、档案的恢复参数（OpenCode 为 `-s <id>`，omp 为 `--resume <id>`，`agy` 为 `--conversation <id>`）组成，并套用与原命令相同的写法（直接执行或 bashrc 包装）。
- 白名单参数 SHALL 同时识别 `--flag value` 与 `--flag=value` 两种形式，并按档案声明的参数个数解析。`agy` 的白名单参数为开关 `--dangerously-skip-permissions`、`--sandbox`，以及带值参数 `--agent`、`--add-dir`、`--log-file`、`--model`、`--effort`、`--mode`。
- 系统 SHALL 丢弃以下参数：
  - omp 与 `agy` 的位置参数（提示词）；
  - 提示词参数（例如 OpenCode 的 `--prompt`、`agy` 的 `-i`/`--prompt-interactive`）；
  - 会话类与会话级覆盖参数（`-c`、`--continue`、`-r`、`--resume`、`--restore`、`-s`、`--session`、`--conversation`、`--fork`、`agy` 的 `--settings` 与 `--remote-control` 等）；
  - 白名单之外的参数。
- 被丢弃的参数 SHALL 在切换结果中列出，界面 SHALL 提示“以下参数在恢复时未保留”。

#### Scenario: 丢弃提示词与会话参数

- **WHEN** 原 argv 为 `["omp","--model","haiku","--continue","修 bug"]`，agent 会话 ID 为 `01abc`
- **THEN** 恢复命令为 `["omp","--model","haiku","--resume","01abc"]`，界面列出未保留的 `--continue` 与 `修 bug`

#### Scenario: agy 生成恢复命令

- **WHEN** 原 argv 为 `["agy","--dangerously-skip-permissions","--model","gemini-3.8-flash","-c","修 bug"]`，agent 会话 ID 为 `89c9a933-60f6-46ad-977b-a8daf22f8e4d`
- **THEN** 恢复命令保留 `--dangerously-skip-permissions` 与 `--model gemini-3.8-flash` 并追加 `--conversation 89c9a933-60f6-46ad-977b-a8daf22f8e4d`，界面列出未保留的 `-c` 与 `修 bug`

### Requirement: ACP 启动命令

系统 SHALL 按档案启动 ACP 进程：OpenCode 为 `opencode acp`，omp 为 `omp acp`，`agy` 为 `agy acp`，并套用与原命令相同的写法。

- omp 的 `omp acp` 解析完整 argv，并静默忽略未知参数。因此原 argv 中位于 omp 白名单内的参数 SHALL 全部透传给 ACP 进程（例如 `--model`、`--thinking`、`--profile`、`--config`、`--approval-mode`、`--auto-approve`），使两端使用相同的模型、配置档与批准策略。
- `opencode acp` 只接受 `--login`，OpenCode 的白名单参数 SHALL NOT 透传；`--auto` 也 SHALL NOT 在聊天视图提示不支持，聊天端的工具批准由聊天视图的权限模式决定（默认自动批准）。
- `agy acp` 校验其子命令标志，原 argv 中位于 `agy` 白名单且 `agy acp` 接受的参数（`--dangerously-skip-permissions`、`--sandbox`、`--agent`、`--add-dir`、`--log-file`）SHALL 透传给 `agy acp`；仅 TUI 接受的 `--model`、`--effort`、`--mode` SHALL NOT 透传给 `agy acp`，其中 `--mode` SHALL 在聊天视图的 `unsupported` 列表中提示（模型与推理等级由 ACP `configOptions` 控制）。

使用 bashrc 包装时，`.bashrc` 执行期间写到 stdout 的内容 SHALL NOT 进入 ACP 协议通道。

#### Scenario: bashrc 有输出

- **WHEN** 用户的 `.bashrc` 会 `echo` 一行欢迎语，混合会话从聊天开始
- **THEN** ACP 握手成功，欢迎语不导致协议错误

#### Scenario: omp 参数透传

- **WHEN** 混合配置的 argv 为 `["omp","--profile","work","--model","haiku","修 bug"]`，会话从聊天开始
- **THEN** ACP 进程的 argv 为 `["omp","acp","--profile","work","--model","haiku"]`

#### Scenario: agy 参数按子集透传

- **WHEN** 混合配置的 argv 为 `["agy","--dangerously-skip-permissions","--sandbox","--model","gemini-3.8-flash","--mode","plan"]`，会话从聊天开始
- **THEN** ACP 进程的 argv 为 `["agy","acp","--dangerously-skip-permissions","--sandbox"]`，聊天状态中的 `unsupported` 包含 `["--mode"]`

### Requirement: 版本提示

agent 的版本来自终端信号或 ACP `initialize` 返回的 `agentInfo.version`。版本低于档案记录的实测版本（OpenCode 2.1.4、omp 18.13.1、`agy` 1.4.12）时，系统 SHALL 在会话页显示提示，SHALL NOT 因此阻止切换；能否切换只由资格判定决定。版本无法解析时 SHALL 不提示。

#### Scenario: 旧版本

- **WHEN** ACP `initialize` 返回 omp 版本 18.0.0
- **THEN** 聊天视图显示“低于已验证版本 18.13.1”的提示，其余功能照常

#### Scenario: agy 旧版本

- **WHEN** ACP `initialize` 返回 `agy` 版本 1.3.0
- **THEN** 聊天视图显示“低于已验证版本 1.4.12”的提示，其余功能照常
