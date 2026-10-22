# Spec Delta

## ADDED Requirements

### Requirement: 机器可读的版本信息

`webmux version --json` SHALL 只向标准输出写入一个 JSON 对象，退出码 0，SHALL NOT 启动服务、读取配置或访问网络。对象 SHALL 包含：`v`（契约版本）、`version`（版本号）、`contract`（该构建支持的控制契约版本，整数）、`revision`（构建提交，未知时为空字符串）、`modified`（构建时工作区是否有未提交改动）。不带 `--json` 的 `webmux version` 与 `webmux --version` 输出 SHALL 保持 `webmux <版本>` 不变。

#### Scenario: 发布构建

- **WHEN** 用户对 Release `v0.3.0` 的二进制执行 `webmux version --json`
- **THEN** 输出 `{"v":1,"version":"0.3.0","contract":1,"revision":"<40 位提交>","modified":false}`，没有进程监听端口

#### Scenario: 没有构建信息

- **WHEN** 二进制在没有 Git 信息的目录中构建
- **THEN** `revision` 为空字符串，`modified` 为 `false`，其余字段照常输出

#### Scenario: 文本输出不变

- **WHEN** 用户执行 `webmux version`
- **THEN** 输出仍为 `webmux 0.3.0`
