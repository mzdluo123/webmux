# Spec Delta

## ADDED Requirements

### Requirement: WebMux 自有的会话变量

服务端 SHALL 在每个会话的环境中设置以下变量，使会话内的程序不依赖 `PATH` 或默认配置就能找到并连接本实例：

- `WEBMUX_SOCKET`：本实例本机 API 套接字的绝对路径。本机 API 不可用时 SHALL NOT 设置。
- `WEBMUX_BIN`：可以执行 `webmux` 子命令的可执行文件的绝对路径。按下文规则解析不到时 SHALL NOT 设置。

注入要求：

- 注入 SHALL 发生在构造会话进程环境的唯一入口：PTY 主进程，以及混合会话聊天端的 ACP agent 等辅助进程，都 SHALL 得到相同的值。
- 两个变量 SHALL 属于服务端自有变量：
  - 创建请求的 `env` 包含它们时，SHALL 以 400 拒绝；
  - 保存包含它们的启动配置时，SHALL 以 400 拒绝；
  - `webmux run` 继承调用方环境时，SHALL 跳过它们。
- 它们 SHALL NOT 出现在会话信息中。
- 服务端 SHALL NOT 为此修改会话的 `PATH`。

`WEBMUX_BIN` 的解析在服务启动时完成一次，结果保存在内存中：

1. 配置项 `cli_path` 非空时，使用它：展开 `~`，转为绝对路径；
2. 否则使用服务自身可执行文件的路径（`os.Executable`），解析符号链接，并去掉 Linux 上替换文件后出现的 ` (deleted)` 后缀；
3. 得到的路径 SHALL 是存在、可执行的普通文件，否则不设置变量，并记录一条警告日志。日志说明原因，并提示可以设置 `cli_path`。

`cli_path` 用于以下情形：服务由 `go run`、临时构建目录或包装脚本启动，自身路径对会话没有意义。
配置校验 SHALL 拒绝指向不存在或不可执行文件的 `cli_path`。
软件包原地升级后，同一路径指向新版本的可执行文件，变量 SHALL 继续有效。

#### Scenario: PATH 中没有 webmux

- **WHEN** 服务以 `/opt/webmux/bin/webmux serve` 启动，会话的 `PATH` 不含 `/opt/webmux/bin`
- **THEN** 会话中 `"$WEBMUX_BIN" ls` 能列出本实例的会话，`PATH` 保持原样

#### Scenario: 聊天端 agent 同样可见

- **WHEN** 混合会话处于聊天端，ACP agent 执行 `echo "$WEBMUX_BIN" "$WEBMUX_SOCKET"`
- **THEN** 输出与同一会话终端端相同的两个绝对路径

#### Scenario: 开发构建

- **WHEN** 服务由 `go run ./cmd/webmux` 启动，配置了 `cli_path = "~/src/webmux/bin/webmux"`
- **THEN** 会话中 `WEBMUX_BIN` 是 `cli_path` 展开后的绝对路径，而不是 Go 构建缓存中的临时文件

#### Scenario: 请求覆盖被拒绝

- **WHEN** 创建请求的 `env` 包含 `WEBMUX_SOCKET=/tmp/other.sock` 或 `WEBMUX_BIN=/tmp/fake`
- **THEN** 服务端返回 400，不创建会话

#### Scenario: 跨实例继承被跳过

- **WHEN** 在实例 A 的会话里，用户用 `webmux run --config b.toml -- bash` 在实例 B 创建会话
- **THEN** 新会话的 `WEBMUX_SOCKET` 与 `WEBMUX_BIN` 都是实例 B 的值
