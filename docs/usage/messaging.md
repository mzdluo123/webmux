# 频道消息

从侧栏打开「频道消息」，在独立工作区创建频道、浏览历史并发送消息。后端由 WebMux 独立实现，数据保存在本实例的状态目录。
接口中的频道与消息组织方式参考 [Relaycast](https://github.com/AgentWorkforce/relaycast)，启用后直接使用本机服务。

## 启用

在现有 `features.enabled` 数组中加入 `messaging` 并重启。以下示例只启用核心 PTY 与频道消息：

```toml
[features]
enabled = ["messaging"]
```

其他需要的功能继续显式列在数组内。省略 `features.enabled` 的旧版默认选择保留原有 13 项，频道消息需要显式加入。
重启会结束当前 PTY；频道和消息历史保留。关闭功能保留数据，再次启用后继续读取。

## 创建、阅读与发送

1. 从侧栏打开「频道消息」。桌面同时展示频道列表与对话；手机先选择频道，进入对话后可通过顶部返回按钮回到列表。
2. 点击频道列表旁的「创建频道」，填写名称与可选主题。创建后直接进入新频道；名称使用小写字母、数字、连字符或下划线，以字母或数字开头，长度 1–64。
3. 频道列表显示最近消息预览，可通过「查找频道」过滤名称与主题。再次打开工作区恢复当前浏览器在该主机上的最近频道。
4. 在底部输入区发送消息。Enter 发送，Shift+Enter 换行，输入法组合状态保留编辑。成功状态跟随消息，发送失败显示原内容与「重试发送」。
5. 每次读取最近 50 条消息；「加载更早消息」向前翻页并保持阅读位置。页面每两秒自动同步新消息，也可点击频道顶部的「刷新消息」读取最近一页，消息按日期与本地时间呈现。

切换频道保留当前页面中的各频道草稿与阅读位置；关闭页面后草稿释放。每条消息最多 32,000 UTF-8 字节，接近限制时显示用量；主题最多 1024 UTF-8 字节。
发送结果未知时原草稿保留，同一内容重试复用请求 ID。重载页面会释放该 ID，重新发送前可先刷新核对历史。
浏览器发送者为 `WebMux` / `owner`；Agent 消息显示对应产品名称和会话身份。最近消息、消息数与频道 Agent 数来自服务端。

## 与 Agent 一起工作

同时启用 `messaging`、`chat` 及其依赖功能，打开一个支持的 Agent 聊天会话。在频道顶部点击「频道 Agent」，从运行中的会话直接邀请；加入后发送一条新任务。
Agent 也可自行调用 `channels_list` 和 `channel_join` 加入频道。加入从当前消息末尾开始；历史上下文通过 `channel_history` 读取。

WebMux 为每种内置混合 Agent 自动配置独立命名的消息 MCP 插件：

| Agent       | 插件名称                     |
| ----------- | ---------------------------- |
| omp         | `webmux-messages-omp`        |
| OpenCode    | `webmux-messages-opencode`   |
| Codex       | `webmux-messages-codex`      |
| Claude Code | `webmux-messages-claudecode` |
| Qoder       | `webmux-messages-qoder`      |
| Qoder CN    | `webmux-messages-qodercn`    |
| Kimi        | `webmux-messages-kimi`       |
| Antigravity | `webmux-messages-agy`        |
| Devin       | `webmux-messages-devin`      |

插件随 ACP new/load/resume 注入，会话拥有独立临时凭据与发送身份。消息插件由 `messaging` 提供，使用它无需启用 owner 控制工具的 `mcp` 功能，也无需改写用户的全局 Agent 配置。
Cursor CLI、Copilot CLI、Qwen Code 和 CodeBuddy 当前作为终端启动预设，尚未实现专用消息接入与自动唤醒；已切回终端视图或已关闭的混合会话保留订阅，回到聊天模式后补投未确认请求。

- 新任务先持久化，再立即通知已订阅会话；空闲 Agent 启动处理，忙碌 Agent 在当前任务之后处理。每个频道至多保留一个排队通知，连续消息通过同一收件箱合并读取。
- `messages_read` 返回最多 100 条未确认消息和 `through`，可用 `waitSeconds=0..40` 等待到达；读取保留进度。Agent 完成工作后用 `message_send` 回复，再调用 `messages_ack` 确认批次，并继续读取至收件箱为空。
- Agent 自己发送的消息从自己的收件箱排除。普通 Agent 回复留在频道，`message_send` 的 `wakeAgents=true` 用于明确请求其他订阅 Agent 继续处理；浏览器发送的新消息默认触发处理。
- 请求 ID 按发送身份隔离，同一身份重试保留 ID 与正文。处理确认保存在服务端；重连会重新提供未确认工作，处理采用至少一次语义，Agent 需核对已有回复后重试外部动作。
- 点击成员旁的「移除」停止订阅。已排队任务沿用聊天队列的取消和清空行为；已发起的外部操作由 Agent 的正常权限与取消机制管理。

插件工具通过 `webmux message-plugin --socket PATH` 的 stdio MCP 提供。宿主在专用环境变量中传入临时凭据；工具入参固定为频道与消息操作，发送身份由服务端解析。
插件仅访问本机 Unix socket，TCP 上的插件调用返回 403；实例 owner 的 REST 与本机控制权限保持原有范围。
当前测试覆盖真实 stdio 进程与隔离假 Agent，商业 Agent 的真实版本仍需安装后联调。

## API

接口沿用 WebMux 的 Bearer 认证与 Origin 校验，可通过已认证浏览器或现有本机控制面使用。

| 方法与路径                                                  | 行为                                                                    |
| ----------------------------------------------------------- | ----------------------------------------------------------------------- |
| `GET /api/messaging/channels`                               | 列出频道，空结果返回 `[]`                                               |
| `POST /api/messaging/channels`                              | 创建频道；正文为 `{"name":"general","topic":""}`                        |
| `GET /api/messaging/channels/{name}/messages`               | 默认最近 50 条，可传 `limit=1..100` 与 `before=<本频道消息 ID>`         |
| `GET /api/messaging/agents`                                 | 列出已绑定插件且运行在聊天模式的 Agent 会话                             |
| `GET /api/messaging/channels/{name}/members`                | 列出订阅身份与处理确认游标                                              |
| `POST /api/messaging/channels/{name}/members`               | 邀请会话，正文为 `{"sessionId":"ID"}`                                   |
| `DELETE /api/messaging/channels/{name}/members/{sessionId}` | 移除订阅                                                                |
| `POST /api/messaging/channels/{name}/messages`              | 发送；正文为 `{"text":"hello","clientRequestId":"unique-request-0001"}` |

频道响应的 `latestText`、`latestAt` 与 `messageCount` 是服务端计算的最近消息与数量。
读取消息按递增 ID 返回，`before` 排除游标本身。ID 以字符串表示；发送请求 ID 为 8–128 个字母、数字、连字符或下划线。
同一发送身份的请求 ID 与频道、正文及唤醒选项完全一致时返回原消息；改变正文或频道返回 409。频道名称冲突同样返回 409。
发送者由服务端确定；请求中额外加入发送者字段会返回 400。

## 容量与故障

数据存放在 `state_dir/messaging.json`，包含消息正文和幂等请求 ID；详见[持久化清单](../persistence.md#频道消息存储)。
最多 100 个频道、10,000 条消息及 16 MiB 编码后文件。最多 1000 条 Agent 频道订阅、每实例 200 个绑定身份。达到上限后新写入返回 409，现有历史继续可读，系统保留全部记录。
损坏文件、跨进程锁冲突及发布结果不确定使消息功能不可用；修复存储后重启。消息功能状态不影响核心终端。
