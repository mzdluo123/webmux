## 1. 插件选择与协议基础

- [x] 1.1 在 config 与 cmd 静态目录实现 missing/空数组、稳定 ID 和硬依赖解析；验证旧配置、仅核心和缺依赖时的真实启动结果。
- [x] 1.2 将插件解析前移到 server_config 打开任何业务 store 之前；验证禁用模块的损坏/锁定状态不阻止核心启动。
- [x] 1.3 增加 contract/wire 和前端 Zod 能力 DTO；验证未知版本/非法结构被客户端拒绝且不启动插件请求。
- [x] 1.4 实现 TCP/本机 GET /api/capabilities 及安全来源诊断；验证两种传输事实一致、鉴权仍独立且 configured 不产生业务文件。

## 2. 核心会话创建与输入

- [x] 2.1 从 Workspace.Create 抽取 sessioncreate 通用协调并迁移普通 cmd/cwd/env 调用；验证无 Workspace 实例时真实 PTY 创建成功。
- [x] 2.2 将 configId、workspace、chat、附件等接入显式创建端口；验证禁用能力在 worktree/进程副作用前被拒绝。
- [x] 2.3 保留创建去重、断连后完成和部分成功结果；用现有 create_session/create API 场景验证不重复创建 worktree。
- [x] 2.4 将普通输入/中断结果、错误和送达编排移出 hybrid；验证零插件提交去重、取消、Ctrl-C 和退出码。
- [x] 2.5 把 agent trust/profile 与启动延迟钩子归 launch；验证 launch 关闭时普通 argv 可运行且不预写 agent 信任文件。

## 3. 核心投影与传输边界

- [x] 3.1 从 session.Info/Overlay 移出仓库、聊天和端口业务语义，接入类型化投影；验证详情、初始快照和增量状态一致。
- [x] 3.2 解耦 hub 的 hybrid 视图 gate、归档筛选和 forward 帧；验证 core-only 控制权、replay、退出前排空及慢消费者行为。
- [x] 3.3 统一增强列表快照与订阅顺序，消除迁移窗口；沿用 continuity 场景验证并发更新无丢失且不阻塞 PTY。
- [x] 3.4 抽取 Controller transport 基础设施并实现带 owner/暴露策略的路由贡献；验证冲突及核心保留路由在 ready 前失败。
- [x] 3.5 TCP、本机与 P2P 复用已绑定流处理器；验证三种身份和暴露矩阵，不把远程调用提升成本机权限。

## 4. 业务后端插件装配

- [x] 4.1 为 launch、workspace、code、files 建立 cmd 装配与独立 Controller 贡献；分别运行单插件，验证不构造其他业务服务。
- [x] 4.2 将 hybrid、ACP、conversation、history/handoff 归 chat，声明 launch/files 依赖；验证无 workspace 的直接目录聊天和恢复。
- [x] 4.3 拆出 notifications、quick-commands 与 voice 的生产者、设置和路由；验证关闭后不扫描/推送/启动识别进程。
- [x] 4.4 拆出 forward 及端口发现、Cloudflare、Tailscale 和 P2P 装配；验证各自单独启用不扩大其他传输的能力或 Origin。
- [x] 4.5 将 stdio、ACP 注入、历史 reader 与 owner TCP 入口归 mcp；验证仅核心加 mcp 的工具发现不要求 launch/chat。
- [x] 4.6 在 cmd 显式连接跨插件组合，移除旧无条件装配及重复分派；验证完整组合保留原接口与业务结果。

## 5. 生命周期与持久化

- [x] 5.1 按入口/生产者、任务/agent、核心 PTY、store 分阶段关闭；验证交接/恢复任务退出后不再创建子进程。
- [x] 5.2 为部分初始化失败释放已构造资源；以失败启动验证锁、socket 和子进程均不残留。
- [x] 5.3 禁用插件不运行 seed、migration、恢复和 sweep；验证既有数据字节不变，重新启用后记录仍可见。
- [x] 5.4 files 在 chat 关闭时使用保守 retention；验证无法确认归属的归档产物不被删除且不开聊天数据库。
- [x] 5.5 保留共享偏好内禁用插件字段并拒绝其业务写操作；验证修改核心主题后再启用插件仍保留其偏好。
- [x] 5.6 配置诊断只接入启用模块的健康贡献；验证 disabled、业务 off、环境不支持和存储不可用不混淆且错误脱敏。

## 6. 浏览器插件宿主

- [x] 6.1 在 App 健康检查后、enterHome/业务启动前获取能力；验证发现失败不全开插件且保留重试入口。
- [x] 6.2 建立固定本地动态 import 映射及贡献类型；验证禁用模块的 chunk、REST、WS 和周期请求均未加载或启动。
- [x] 6.3 抽出核心列表、导航和 shell 上下文，插件拥有自己的控制器；验证仓库/聊天不就绪时仍能打开普通终端。
- [x] 6.4 按同一贡献表组合页面、导航、面板、操作和设置 section；验证禁用深链接显示原因、设置锚点不引用空节点。
- [x] 6.5 提取 SessionTerminalStage、Header、Switcher 的聊天/代码/上传/语音引用；验证旧偏好不能遮挡核心终端或触发插件加载。
- [x] 6.6 提取 P2P、推送、归档 fallback、沉浸式专用入口及后台资源；验证认证失败/宿主销毁时全部订阅和计时器释放。
- [x] 6.7 插件模块加载失败显示局部错误，不阻断核心；验证 terminal 输入输出仍有效且错误入口可以恢复。

## 7. 创建界面与外围客户端

- [x] 7.1 NewSessionDialog 支持无 launch/workspace/files 的已有 cwd+自定义命令；验证手机/桌面均可创建且不请求被禁用来源。
- [x] 7.2 区分来源禁用、加载故障、配置删除，归一化旧 configId/chat/code 偏好；验证不自动执行猜测命令或跳转禁用页。
- [x] 7.3 接入 workspace+chat 快速开始、代码引用与快捷指令的组合贡献；验证单边禁用时不提供不可执行操作。
- [x] 7.4 迁移 clicmd/localclient/Windows 控制台的能力消费；验证 run/attach 不读取 launch/repositories，禁用业务不误报旧版本。
- [x] 7.5 托盘菜单、MCP 发现、help/completion 和 agent guide 按实际能力组合；验证仅核心仍可打开终端和管理基础服务。

## 8. 架构约束与行为回归

- [x] 8.1 登记新包、编码/请求体汇点和持久化 owner，增加核心禁止依赖插件实现的规则；验证 archtest 拒绝反向依赖而非扩大豁免。
- [x] 8.2 更新前端依赖规则，禁止核心静态导入插件和远程模块；运行 pnpm --dir web check:arch 验证无新增 baseline。
- [x] 8.3 为 core-only 建立不自动补业务服务的夹具，扩展 E2E server 配置及按能力清理；验证测试不访问现有开发服务或禁用接口。
- [x] 8.4 迁移受影响契约/行为测试，删除只锁定文案或实现的断言，并重录 TestWireSnapshots；验证既有消费者和新增能力契约一致。

## 9. 端到端验收与交付

- [x] 9.1 运行隔离真实服务和浏览器，验证零插件创建、输入、重连、回放、控制、分屏、移动输入及 native attach，并记录实际输出。
- [x] 9.2 验证 code-only、launch-only、workspace-only、chat+依赖、mcp-only 和完整组合；记录禁用路径 404、零业务文件/进程和网络请求证据。
- [x] 9.3 验证缺依赖失败、旧模式迁移、重启采用新启用集、产物保留及安全拒绝；保留能捕捉消费者可见边界错误的回归覆盖。
- [x] 9.4 运行 make check 与 make test，补桌面/平板/手机前后截图及交互 GIF；未在原生 Windows 点击验证的部分明确标注限制。
- [x] 9.5 更新配置示例、协议、架构和持久化说明；更新 US-B01–B03、B08、C01、C08、C12、C16–C18、K04、K10、M01–M02、M05、M08、Q01、Q12–Q13、Q18–Q20、S07、S20、S24 的适用条件并新增插件故事，验证索引/计数一致。
- [ ] 9.6 运行 make docs-check 和 OpenSpec strict validation，确保故事仅声明实际验收；通过专用分支 PR 交付并跟进 CI/机器人检查，不自动合并。
