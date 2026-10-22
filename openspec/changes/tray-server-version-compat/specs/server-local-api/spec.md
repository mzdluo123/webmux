# Spec Delta

## ADDED Requirements

### Requirement: 状态中的构建信息

实例运行中时，`/local/v1/status` 与 `webmux status --json` SHALL 包含 `build` 对象：`revision`（运行中实例的构建提交，未知时为空字符串）、`modified`（构建时是否有未提交改动）与 `contract`（该实例支持的控制契约版本）。该字段 SHALL 为可选新增字段，契约版本保持 v1；旧实例 SHALL 省略该字段，客户端 SHALL 把缺失视为未知。实例未运行时 SHALL 省略该字段。

#### Scenario: 运行中的实例

- **WHEN** 由提交 `abc123…` 构建的 WebMux 正在运行，客户端请求 `/local/v1/status`
- **THEN** 响应中 `version` 为该实例的版本，`build.revision` 为 `abc123…`，`build.contract` 为 1，`v` 仍为 1

#### Scenario: 未运行

- **WHEN** WebMux 未运行，用户执行 `webmux status --json`
- **THEN** 输出不包含 `build` 字段
