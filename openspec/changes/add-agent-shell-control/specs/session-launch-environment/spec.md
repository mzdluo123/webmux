# Spec Delta

## ADDED Requirements

### Requirement: 本机 API 套接字变量

服务端 SHALL 在每个会话的环境中设置 `WEBMUX_SOCKET`，值为本实例本机 API 套接字的绝对路径。本机 API 不可用时 SHALL NOT 设置该变量。

- 会话创建请求的 `env` 包含 `WEBMUX_SOCKET` 时，SHALL 以 400 拒绝且不创建会话，与 `WEBMUX_SESSION_ID` 一致。
- `WEBMUX_SOCKET` SHALL 属于服务端自有变量：`webmux run` 继承调用方环境时 SHALL 跳过它，因此从一个实例的会话里创建的会话总是拿到目标实例的路径。
- 启动配置的环境变量 SHALL NOT 覆盖它；保存包含该名称的启动配置时 SHALL 以 400 拒绝。
- 该变量 SHALL NOT 出现在会话信息中。

#### Scenario: 会话中可见

- **WHEN** 用户在 WebMux 会话中执行 `echo "$WEBMUX_SOCKET"`
- **THEN** 输出本实例套接字的绝对路径，`webmux ls` 不带参数即可连接该实例

#### Scenario: 请求覆盖被拒绝

- **WHEN** 创建请求的 `env` 包含 `WEBMUX_SOCKET=/tmp/other.sock`
- **THEN** 服务端返回 400，不创建会话

#### Scenario: 跨实例继承被跳过

- **WHEN** 在实例 A 的会话里，用户用 `webmux run --config b.toml -- bash` 在实例 B 创建会话
- **THEN** 新会话的 `WEBMUX_SOCKET` 是实例 B 的套接字
