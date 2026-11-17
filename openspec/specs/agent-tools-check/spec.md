# agent-tools-check Specification

## Purpose

定义 WebMux 对服务运行环境下内置 CLI Agent 工具与第三方 ACP 适配器的本机安装路径解析、本地版本采集、按需公开上游版本检查，以及在 Web 设置页与命令行客户端中的一致状态呈现与升级指引行为。

## Requirements

### Requirement: 工具检测目录与内置功能门控

系统 SHALL 按 `launch` 内置功能控制内置 Agent 工具检测能力的装配与暴露：

- 启用 `launch` 时，系统 SHALL 提供检测能力，覆盖全部已注册的混合模式 Agent、其声明的第三方 ACP 适配器及内置纯终端 Agent 预设；
- 禁用 `launch` 时，系统 SHALL NOT 构造检测服务或挂载相关路由，受鉴权请求访问检测接口 SHALL 返回 404，Web 设置页与命令行客户端 SHALL NOT 暴露该入口。

#### Scenario: 启用 launch 时返回完整工具目录

- **WHEN** 实例启用 `launch` 功能且客户端请求 `GET /api/agent-tools`
- **THEN** 响应按稳定顺序包含全部内置混合 Agent（含附属 ACP 适配器信息）与纯终端预设工具条目，不含通用登录 Shell 预设

#### Scenario: 禁用 launch 时路由关闭

- **WHEN** 实例以 `features.enabled = []` 启动且已鉴权客户端访问 `GET /api/agent-tools` 或 `POST /api/agent-tools/check`
- **THEN** 服务端返回 404，不启动任何 Shell 或外部命令探测进程

### Requirement: 登录启动文件一致的可执行文件定位

本机扫描 SHALL 使用与默认预设会话一致的启动 Shell 解析规则定位候选可执行文件：

- 若服务默认 Shell 为受支持的交互式启动文件包装（`bash` 或 `zsh`），系统 SHALL 在交互式启动文件生效后的 `PATH` 中查找；否则在服务进程当前 `PATH` 中查找；
- 系统 SHALL 在有界时间内完成批量定位，且启动文件向标准输出打印的欢迎语 SHALL NOT 干扰路径提取。

#### Scenario: 仅在启动文件中追加的用户级路径

- **WHEN** 服务默认 Shell 为 `bash` 且某 Agent 仅安装在 `~/.bashrc` 追加到 `PATH` 的目录下
- **THEN** 本机扫描判定该工具为已安装，并返回解析后的绝对路径与匹配的可执行文件名

#### Scenario: 多候选可执行文件名按优先级命中

- **WHEN** 某 Agent 声明多个可执行文件名（如 `qoder` 与 `qodercli`）且仅第二个存在于 `PATH` 中
- **THEN** 本机扫描判定为已安装，并报告实际命中的可执行文件名及路径

### Requirement: 安全受界的本地版本号采集

对已定位到绝对路径的工具与 ACP 适配器，系统 SHALL 直接执行该可执行文件的版本查询参数（无隐式 Shell 展开），关闭标准输入并置于独立进程组中，限制单进程执行时间与输出字节上限。
超时或非零退出时，系统 SHALL 终止残留进程组，仍保留工具已安装的事实并将已安装版本置为空字符串。

#### Scenario: 提取带前后缀的版本输出

- **WHEN** 已安装可执行文件输出 `2.1.316 (Claude Code)`、`codex-cli 0.165.0` 或 `kimi, version 2.5.2`
- **THEN** 系统提取规范化的点分数字版本号（如 `2.1.316`、`0.165.0`、`2.5.2`）填入 `installedVersion`

#### Scenario: 版本子进程超时阻塞不卡死接口

- **WHEN** 某可执行文件在 `--version` 参数下阻塞超过单进程时限
- **THEN** 系统清理其进程组并按期返回响应，该条目 `installed` 为 `true`、`installedVersion` 为空，其余工具结果正常返回

### Requirement: 零静默出站与按需上游更新检查

系统 SHALL 保持零静默出站并仅按需执行只读上游版本检查：

- 服务启动及 `GET /api/agent-tools` 请求 SHALL NOT 主动发起任何外网 HTTP 请求；未执行过在线检查的已安装工具状态 SHALL 标记为 `unchecked`（无公开上游版本源的工具标记为 `source_unavailable`）；
- 仅当客户端显式调用 `POST /api/agent-tools/check` 时，系统 SHALL 通过受信任的只读 HTTPS 包仓库源（如 npm Registry、PyPI、GitHub Releases）查询已安装工具与适配器的最新稳定版，且 SHALL NOT 携带凭据或跟随指向私网/环回的重定向。

#### Scenario: 首次读取仅执行本机探测

- **WHEN** 服务启动后客户端首次调用 `GET /api/agent-tools`
- **THEN** 服务端不产生任何外部网络请求，具备公开版本源的已安装工具返回 `updateStatus: "unchecked"`，专有分发工具返回 `updateStatus: "source_unavailable"`

#### Scenario: 显式检查返回可用更新与最新状态

- **WHEN** 客户端调用 `POST /api/agent-tools/check`，本机工具 A 版本 `2.1.300` 低于上游 `2.1.320`，工具 B 版本 `0.165.0` 等于上游 `0.165.0`
- **THEN** 工具 A 返回 `latestVersion: "2.1.320"` 与 `updateStatus: "update_available"`，工具 B 返回 `latestVersion: "0.165.0"` 与 `updateStatus: "up_to_date"`

#### Scenario: 单个上游源失败隔离

- **WHEN** `POST /api/agent-tools/check` 期间某一个包仓库源返回非 2xx 或超时
- **THEN** 仅对应工具标记为 `updateStatus: "check_failed"` 并附简短错误摘要，其余工具的检查结果正常返回并写入内存缓存

### Requirement: 内存缓存、手动刷新与实测基线区分

系统 SHALL 在内存中分别缓存本机扫描结果与上游版本结果（不新增持久化状态文件），并对并发请求执行合并去重；当客户端传入强制刷新标记时 SHALL 重新扫描本机环境。
响应 SHALL 同时区分上游最新版本 `latestVersion` 与 WebMux 混合会话已验证基线版本 `verifiedVersion`，SHALL NOT 将基线版本伪报为上游最新版。

#### Scenario: 新安装工具后强制重新扫描

- **WHEN** 用户在宿主机新安装某工具后请求 `GET /api/agent-tools?refresh=1`
- **THEN** 系统绕过本机扫描内存缓存重新定位可执行文件，立即返回新安装的条目及版本号

#### Scenario: 低于已验证基线的兼容性提示

- **WHEN** 某混合 Agent 本地已安装版本低于其档案声明的 `verifiedVersion`
- **THEN** 响应携带 `belowVerified: true` 与 `verifiedVersion`，供客户端展示兼容性提示

### Requirement: Web 设置页 Agent 工具常驻卡片

Web 设置页（`/settings`）SHALL 在启用 `launch` 时紧邻「启动配置」下方提供唯一的「Agent 工具」（`#agent-tools`）常驻分区与导航项，不在应用侧栏或终端顶栏放置常驻更新徽标：

- 卡片首次打开时 SHALL 仅请求本机状态并保持骨架占位零布局跳动；
- 默认展开已安装工具（含附属 ACP 适配器状态、版本号、更新状态及可一键复制的升级或安装命令），底部使用可折叠区域收纳未安装工具。

#### Scenario: 展示已安装工具、缺失适配器与折叠未安装项

- **WHEN** 用户打开 `/settings#agent-tools`，本机已安装 Claude Code（有可用更新）与 Codex CLI（缺少 `codex-acp`），其余工具未安装
- **THEN** 卡片顶部显示已安装与可更新计数及「检查更新」主按钮；主列表展示上述两项、更新提示、`codex-acp` 未安装警告与复制命令按钮；其余未安装工具默认收起在底部折叠项中

#### Scenario: 窄屏与多主题下无横向溢出

- **WHEN** 在桌面（1280px）、平板（768px）与手机（390px / 320px）视口下查看该卡片并展开未安装列表
- **THEN** 长路径安全截断并保留完整提示，状态徽标与操作按钮自然排版，页面不出现横向滚动条

### Requirement: CLI agents 子命令查询

命令行客户端 SHALL 提供 `webmux agents` 子命令访问同一服务端接口：默认输出已安装工具与附属 ACP 适配器的状态表格，`--all` 包含未安装工具，`--refresh` 强制重新扫描本机，`--check` 触发上游版本检查，`--json` 输出带协议版本号的结构化 JSON。

#### Scenario: 命令行检查更新与 JSON 输出

- **WHEN** 用户执行 `webmux agents --check --json`
- **THEN** 客户端调用 `POST /api/agent-tools/check` 并向标准输出打印包含协议版本字段 `v`、`tools`、版本号与 `updateStatus` 的有效 JSON，退出码为 0
