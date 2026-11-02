# Spec Delta

## Purpose

让通过 deb/rpm 安装到系统的 WebMux 以系统级定时任务自动安装经签名验证的新版本安装包，并保证更新不启动、停止或重启任何用户的 WebMux 服务。

## ADDED Requirements

### Requirement: 系统级更新定时器

安装包 SHALL 提供系统单元 `webmux-update.timer` 与 `webmux-update.service`。定时器 SHALL 每天触发一次，带最长 6 小时的随机延迟，错过的触发 SHALL 在下次开机后补执行。服务 SHALL 以 root 运行 `/usr/bin/webmux update apply`，类型为一次性任务，SHALL NOT 访问任何用户的主目录。

#### Scenario: 每日检查

- **WHEN** deb 安装后系统持续运行两天
- **THEN** `journalctl -u webmux-update.service` 中出现两次检查记录，触发时间各带随机延迟

#### Scenario: 不读取用户目录

- **WHEN** 更新服务运行
- **THEN** 它无法读取 `/home` 与 `/root` 下的文件，检查与安装仍正常完成

### Requirement: 定时器启用与保留

若系统由 systemd 管理，安装脚本 SHALL 在本机首次获得定时器时启用并启动 `webmux-update.timer`，并留下标记。卸载 SHALL 停止并禁用定时器并删除标记。

- 首次获得：新安装，或从尚未包含定时器的版本升级。
- 已有标记的升级：只重新加载单元定义，SHALL 保留定时器当时的启用或禁用状态。
- 没有运行 systemd：安装 SHALL 成功且不报错，也不留下标记。

#### Scenario: 用户关闭自动更新

- **WHEN** 管理员执行 `systemctl disable --now webmux-update.timer`，之后手动安装新版本 deb
- **THEN** 升级后定时器仍为禁用状态

#### Scenario: 从没有定时器的版本升级

- **WHEN** 系统上装有不含定时器的 0.4.0 deb，管理员升级到包含定时器的 0.5.0
- **THEN** 升级后定时器为启用状态，之后的升级不再改变它

#### Scenario: 容器内安装

- **WHEN** 在未运行 systemd 的容器中执行 `dpkg -i webmux_0.5.0-1_amd64.deb`
- **THEN** 安装成功，`/usr/bin/webmux` 可用，没有 systemctl 错误

### Requirement: 安装方式识别

`webmux update apply` SHALL 只在 `/usr/bin/webmux` 属于已安装的 `webmux` 软件包时执行更新，并选择与当前二进制同架构的安装包：

- 由 dpkg 管理：`webmux_<版本>-1_<amd64|arm64>.deb`；
- 由 rpm 管理：`webmux-<版本>-1.<x86_64|aarch64>.rpm`；
- 不属于任何软件包：SHALL 报告“当前安装方式不支持自动更新”并以非零退出码结束，SHALL NOT 下载。

#### Scenario: rpm 系统

- **WHEN** 在 aarch64 的 Fedora 上通过 rpm 安装 0.4.0，最新版本为 0.5.0
- **THEN** 更新服务下载并校验 `webmux-0.5.0-1.aarch64.rpm`

#### Scenario: 非包安装

- **WHEN** 用户以 root 运行 `install.sh` 安装到 `/usr/local/bin` 后执行 `webmux update apply`
- **THEN** 命令报告当前安装方式不支持自动更新并失败，没有网络下载

### Requirement: 权限要求

`webmux update apply` SHALL 要求以 root 运行；非 root 调用 SHALL 报告“需要 root 权限”并以非零退出码结束，SHALL NOT 发起网络请求。

#### Scenario: 普通用户执行

- **WHEN** 普通用户执行 `webmux update apply`
- **THEN** 命令报告需要 root 权限并失败

### Requirement: 尊重版本锁定

`webmux` 软件包被管理员锁定版本时（dpkg 的 `apt-mark hold`，或 rpm 系统上 `dnf versionlock` 中的 `webmux` 条目），`webmux update apply` SHALL 在发起网络请求前跳过本次更新，记录“已被锁定版本”并以退出码 0 结束。锁定工具或插件不存在时 SHALL 视为未锁定。

#### Scenario: apt-mark hold

- **WHEN** 管理员执行过 `apt-mark hold webmux`，最新版本高于已安装版本，更新服务触发
- **THEN** 日志记录已被锁定版本，没有下载，`/usr/bin/webmux` 不变

#### Scenario: dnf versionlock

- **WHEN** Fedora 上 `dnf versionlock list` 列出 `webmux`，更新服务触发
- **THEN** 日志记录已被锁定版本，没有下载，安装包不变

### Requirement: 通过包管理器安装

校验通过的安装包 SHALL 以 argv 形式交给 `dpkg -i` 或 `rpm -U` 安装，SHALL NOT 经 shell。安装成功或失败后 SHALL 删除下载的安装包。

- 并发：同一时刻 SHALL 只有一个 `update apply` 在运行，后来者 SHALL 立即退出并报告已有更新在进行。
- 失败：包管理器失败（含数据库被占用）时 SHALL 记录其输出并以非零退出码结束；原有安装 SHALL 保持可用，下次触发时重试。

#### Scenario: dpkg 被占用

- **WHEN** 管理员正在运行 `apt upgrade`，更新服务触发并尝试安装
- **THEN** 更新服务失败并记录 dpkg 锁被占用，`/usr/bin/webmux` 仍为旧版本，次日再次尝试

#### Scenario: 安装成功

- **WHEN** 最新版本为 0.5.0，安装包校验通过且 dpkg 成功
- **THEN** `/usr/bin/webmux version` 输出 `webmux 0.5.0`，缓存目录中不留下安装包

### Requirement: 更新不改动用户服务

自动更新 SHALL 遵守“安装与升级不改动用户服务”：SHALL NOT 为任何用户启用、启动、停止或重启 `webmux.service`。正在运行的服务 SHALL 继续使用旧进程，直到用户自行重启。

#### Scenario: 运行中自动更新

- **WHEN** 某用户的 `webmux.service` 正在运行并有 3 个会话，更新服务把软件包升级到 0.5.0
- **THEN** 服务进程与 3 个会话保持不变，`webmux status` 提示已安装 0.5.0、重启后生效

### Requirement: 更新日志

每次运行 SHALL 在系统日志中记录：当前版本、发现的最新版本、使用的来源、结果（已是最新、已安装 <版本>、失败及原因）。日志 SHALL NOT 包含任何用户数据或令牌。

#### Scenario: 已是最新

- **WHEN** 当前版本即为最新版本
- **THEN** 日志记录“已是最新版本 0.5.0”，没有下载安装包
