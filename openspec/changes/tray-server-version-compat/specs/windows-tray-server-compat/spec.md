# Spec Delta

## Purpose

定义 Windows 托盘如何判断 WSL 中的 `webmux`（已安装文件与正在运行的实例）与托盘是否兼容、是否对应，何时检查，以及如何向用户展示结果并提供修复入口。

## ADDED Requirements

### Requirement: 检查时机

托盘 SHALL 在以下时机检查所选发行版中配置路径上的 `webmux`：托盘启动后首次取得状态时、切换发行版或修改 `webmux` 路径后、安装或升级完成后、以及处于缺失或不兼容状态时打开菜单。检查 SHALL 在后台进行，SHALL NOT 阻塞菜单弹出，单次检查 SHALL NOT 超过 30 秒。检查结果 SHALL 写入托盘日志，包括托盘与 WSL 两侧的版本、构建提交与契约版本。

#### Scenario: 启动时检查

- **WHEN** 托盘 0.3.0 启动，WSL 中的 `webmux` 为 0.2.0
- **THEN** 托盘取得状态后完成检查，日志中出现两侧版本，菜单在检查期间仍可打开

#### Scenario: 切换发行版

- **WHEN** 用户把目标发行版从 Ubuntu 切换到 Debian，Debian 中的 `webmux` 版本不同
- **THEN** 托盘重新检查并以 Debian 中的结果更新状态行

### Requirement: 兼容判定

托盘 SHALL 内置自己支持的控制契约版本集合。WSL 中的 `webmux` 报告的契约版本在该集合内即为兼容；低于集合即为“不兼容：WSL 过旧”，高于集合即为“不兼容：WSL 过新”。不支持机器可读版本输出的 `webmux` SHALL 沿用现有探测：支持本机 API 且状态契约为 v1 时视为契约 v1，否则为“不兼容：WSL 过旧”。无法执行时为“缺失”。

#### Scenario: 契约版本过高

- **WHEN** 托盘支持契约 v1，WSL 中的 `webmux` 报告契约 v2
- **THEN** 托盘判定“不兼容：WSL 过新”

#### Scenario: 旧版本没有机器可读版本输出

- **WHEN** WSL 中的 `webmux` 不认识 `version --json`，但支持本机 API，`status --json` 的 `v` 为 1
- **THEN** 托盘视其为契约 v1，判定为兼容，并以 `webmux version` 的文本输出取得版本号

### Requirement: 对应判定

兼容时，两侧版本号相同且构建提交相同（任一侧未知提交时只比较版本号）即为“对应”。否则 SHALL 归为以下之一：“WSL 较旧”“WSL 较新”（按 `MAJOR.MINOR.PATCH` 比较）、“同版本不同构建”（版本号相同而提交不同）。任一侧版本号不是 `MAJOR.MINOR.PATCH`（如 `dev`）且提交未知时 SHALL 归为“无法判断”，按兼容处理且不提示升级。

#### Scenario: 发布版本对应

- **WHEN** 托盘与 WSL 中的 `webmux` 都报告 0.3.0 且提交相同
- **THEN** 托盘判定“对应”，不显示版本提示，不发通知

#### Scenario: 本地构建来自不同提交

- **WHEN** 两侧都是本地 `make` 构建、都报告 0.1.0，但构建提交不同
- **THEN** 托盘判定“同版本不同构建”，状态行显示两侧的短提交号

#### Scenario: 开发构建

- **WHEN** WSL 中的 `webmux` 报告版本 `dev` 且没有构建提交
- **THEN** 托盘判定“无法判断”，照常工作，不提示升级

### Requirement: 不兼容时的行为

判定为不兼容时，托盘 SHALL 禁用启动、重启以及依赖本机 API 的管理操作，SHALL NOT 进入自动重启循环，并在状态行说明方向：“WSL 中的 webmux 版本过旧，请升级”或“WSL 中的 webmux <版本> 比托盘新，请升级托盘”。不兼容的实例已在运行时，打开 WebMux 与复制链接 SHALL 仍然可用。问题解决后的下一次检查通过时，托盘 SHALL 自动恢复正常状态。

#### Scenario: WSL 过新且正在运行

- **WHEN** WSL 中运行着契约 v2 的 WebMux，托盘只支持 v1
- **THEN** 状态行提示升级托盘，“启动 / 重启”禁用，“打开 WebMux”仍可用

#### Scenario: 升级后恢复

- **WHEN** WSL 过旧导致不兼容，用户点击“升级 WSL 中的 webmux”并安装成功
- **THEN** 托盘重新检查，判定为对应，状态回到“已停止”，“启动”可用

### Requirement: 修复入口

托盘 SHALL 按方向提供修复入口：WSL 过旧或较旧时提供“升级 WSL 中的 webmux”（安装与托盘同版本的 `webmux`，行为见 `windows-tray-wsl-install`）；WSL 过新或较新且其版本为发布版本时提供“下载托盘 <版本>”，用默认浏览器打开该版本的 Release 页面。托盘 SHALL NOT 在后台自动安装、自动更新自身或自动重启 WebMux。无法判断或同版本不同构建时 SHALL NOT 提供下载入口。

#### Scenario: WSL 较新

- **WHEN** 托盘为 0.3.0，WSL 中的 `webmux` 为 0.4.0 且兼容
- **THEN** 菜单出现“下载托盘 0.4.0”，点击后浏览器打开 `v0.4.0` 的 Release 页面

#### Scenario: 本地构建不提供下载

- **WHEN** 判定为“同版本不同构建”
- **THEN** 菜单没有升级或下载入口，只在状态行说明

### Requirement: 兼容但不对应时的提示

兼容但不对应时，托盘 SHALL 照常管理 WebMux，并在状态行显示“托盘 <版本> · WSL <版本>”及较旧的一方。每次托盘启动后，对同一发行版、同一组版本 SHALL 最多发出一次系统通知；之后的重复检查结果不变时 SHALL NOT 再次通知。判定为不兼容时 SHALL 同样发出一次通知。

#### Scenario: 只通知一次

- **WHEN** 托盘启动后判定“WSL 较旧”，用户随后多次打开菜单
- **THEN** 只出现一次通知，状态行持续显示两侧版本

#### Scenario: 对应时不打扰

- **WHEN** 判定为“对应”
- **THEN** 状态行只显示 WebMux 版本，不发通知

### Requirement: 运行实例与已安装文件不一致

WebMux 正在运行时，托盘 SHALL 以本机状态中的版本与构建信息识别运行中的实例。运行中的实例与已安装文件不是同一构建时，托盘 SHALL 在状态行提示“WSL 中已安装 <版本>，重启 WebMux 后生效，重启会结束所有会话”；仅当实例由托盘管理时 SHALL 提供重启，SHALL NOT 自动重启。兼容与对应判定 SHALL 对运行中的实例同样适用。

#### Scenario: 升级后未重启

- **WHEN** 由 systemd 管理的 WebMux 0.2.0 正在运行，用户已把 WSL 中的文件升级到 0.3.0
- **THEN** 状态行提示重启后生效，菜单不提供由托盘重启
