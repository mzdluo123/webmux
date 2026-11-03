# Spec Delta

## MODIFIED Requirements

### Requirement: 版本对应判定

兼容时，托盘 SHALL 只比较版本号：两侧相同为“对应”；否则按 `signed-release-updates` 的版本比较规则（`MAJOR.MINOR.PATCH`，开发版 `-dev.N` 低于同号正式版）归为“WSL 较旧”或“WSL 较新”。

任一侧版本号不是正式版或开发版格式（如 `dev`）时 SHALL 归为“无法判断”，按兼容处理且不提示更新。托盘 SHALL NOT 以构建提交区分版本号相同的构建。

#### Scenario: 发布版本对应

- **WHEN** 托盘与 WSL 中的 `webmux` 都报告 0.3.0
- **THEN** 托盘判定“对应”，状态行不显示版本提示

#### Scenario: 本地构建

- **WHEN** 两侧都是本地 `make` 构建、都报告 0.1.0，即使来自不同提交
- **THEN** 托盘判定“对应”

#### Scenario: 开发构建

- **WHEN** WSL 中的 `webmux` 报告版本 `dev`
- **THEN** 托盘判定“无法判断”，照常工作，不提示更新

#### Scenario: 开发版托盘与正式版服务端

- **WHEN** 托盘为 0.10.3-dev.4，WSL 中的 `webmux` 为 0.10.2
- **THEN** 托盘判定“WSL 较旧”

#### Scenario: 开发版之间

- **WHEN** 托盘为 0.10.3-dev.4，WSL 中的 `webmux` 为 0.10.3-dev.7
- **THEN** 托盘判定“WSL 较新”
