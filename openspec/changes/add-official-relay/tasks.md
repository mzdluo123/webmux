# Tasks

> UI 任务开始前先读 `webmux-ui` 技能与 `docs/design-system.md`、`docs/frontend-style.md`。所有自动化测试使用独立临时实例，不连接开发者现有 WebMux 或终端会话。

## 1. 协议骨架与依赖验证

- [ ] 1.1 引入 `github.com/hashicorp/yamux`，新增 `internal/relayproto` 的版本、签名 transcript 和 WSS/`net.Conn`/yamux 适配；用真实双端测试证明双向 stream、半关闭、保活和窗口背压，并运行 `go test -race ./internal/relayproto`。
- [ ] 1.2 为 Relay 控制面、连接器和宿主 API 建立 `internal/contract` DTO，登记 `relayproto`、`relaycontrol`、`relay`、`relaysettings` 的架构层级与出站用途；运行 `go test -race ./internal/archtest`。
- [ ] 1.3 用表驱动测试固定协议主版本不兼容、错误私钥、nonce 重放、错误受众和单次消费行为，并验证日志与错误中不含签名和配对材料。

## 2. Relay 注册库与配对控制面

- [ ] 2.1 在 `internal/relaycontrol` 建立 SQLite migration、accounts/hosts/enrollments 表和事务接口；测试账号唯一绑定、固定主机名、并发重新绑定、撤销、重启恢复及数据库故障 readiness，运行 `go test -race ./internal/relaycontrol`。
- [ ] 2.2 实现可注入 HTTP 客户端的 GitHub Device Flow 与账号查询，使用假服务验证授权等待、过期、拒绝、邀请名单和访问凭据不落盘；不得连接真实 GitHub 做单元测试。
- [ ] 2.3 实现配对创建、轮询、取消和显式重新绑定 API，覆盖一次性用户码、句柄摘要、速率限制、未邀请账号、过期清理和原子换钥；运行对应 `internal/api` 测试。
- [ ] 2.4 实现连接器 WSS 质询签名认证和每主机单活动会话，验证未认证连接不能顶掉旧连接、合法新连接替换旧连接、撤销立即关闭入口。

## 3. 公共反向代理与故障隔离

- [ ] 3.1 实现 Relay 活动会话注册与 yamux stream 拨号、宿主固定回环拨号，使用临时回环服务验证未知 Host、离线主机、任意回源拒绝和 502/503 映射。
- [ ] 3.2 用 `httputil.ReverseProxy` 接入 REST、流式响应、二进制下载和 multipart 上传，测试原始 Host/Origin、转发头清理、状态码与正文一致，并断言不整包缓冲。
- [ ] 3.3 用隔离的真实 WebMux 验证 `/ws/sessions`、终端回放/offset 续接、聊天、设置、语音和 `/forward/*`，覆盖 WebSocket 关闭码、Token 子协议、Cookie 与长连接不被普通 HTTP 超时截断。
- [ ] 3.4 实现配对/IP 速率、每主机 stream、连接缓冲和保活配置，使用慢浏览器、慢宿主和大上传并发终端测试证明内存有界、超限返回 429/503 且其他主机不受影响。
- [ ] 3.5 实现 Relay 重启、绑定撤销、换钥和签名删除的控制面行为，端到端验证旧入口立即失效、数据库恢复后原地址可由合法宿主重新连接。

## 4. WebMux 宿主设置与连接器

- [ ] 4.1 新增 `[relay]` TOML 配置与 `relay-settings.json` 受控存储，登记敏感级别和 persistence inventory；测试默认关闭、0600/0700、原子写、损坏 fail-closed、重启恢复和关闭落盘失败。
- [ ] 4.2 在 `cmd/webmux` 实现 Relay 控制器与 `internal/relay` 连接器，复用代际启停模型；测试未配对/弱 Token 拒绝、自动退避重连、迟到事件、唯一 worker 和关闭时 goroutine 回收。
- [ ] 4.3 在每次状态变化中维护 `PublicOrigins` 的独立 `relay` 来源，验证只有就绪才授权、断开立即撤权、Cloudflare/Tailscale 来源不受影响。
- [ ] 4.4 实现宿主配对、轮询取消、启停、重新绑定和删除逻辑，验证远端删除未确认时保留本地绑定，所有错误均脱敏且不返回私钥或 GitHub 凭据。
- [ ] 4.5 实现 `/api/relay*` 路由、Zod/Go 契约、统一错误映射及会话内进程 403 限制，运行 `go test -race ./internal/api ./cmd/webmux` 并重录 wire 快照。

## 5. 状态、推送与 P2P 集成

- [ ] 5.1 将 Relay 加入 `urls.access`、本机状态可选 `relay` 对象和 configured/effective 脱敏诊断，测试多来源并列、字段来源追踪、存储降级和无敏感值输出。
- [ ] 5.2 将固定 Relay HTTPS 来源接入现有推送登记，验证新订阅 Host/Origin 前提、重启地址稳定和离线不删除旧订阅，并运行推送相关 Go/Vitest 测试。
- [ ] 5.3 回归 Windows 托盘通用访问入口与本机状态，验证 Relay 无需专属菜单协议即可显示，旧服务端缺少 `relay` 字段时保持兼容。
- [ ] 5.4 用真实浏览器经 Relay 执行 P2P 信令、直连升级和断线回落，断言首屏不等待 P2P、候选限制不变、offset 无缺口且回落 origin 为 Relay；不修改 P2P 帧协议。

## 6. 设置界面与浏览器验收

- [ ] 6.1 新增 `/settings/relay` 路由、设置入口、脱敏 schema 与状态 hook，显示配对状态、期望开关、连接状态、固定地址和错误；运行路由、schema 与 hook 单元测试。
- [ ] 6.2 实现配对码、验证链接、有效期、轮询取消和重新绑定界面，测试配对码不进入 localStorage/sessionStorage/Cookie/日志，离开页面取消请求并清除组件状态。
- [ ] 6.3 实现启停、刷新和删除绑定的风险确认与当前公网页面断线提示，覆盖弱 Token、未配对、存储失败、远端删除未确认等状态，并验证桌面和手机可访问性。
- [ ] 6.4 新增 Storybook 与模拟 API 的 Playwright 流程，覆盖首次配对、启用、断线恢复、重新绑定和删除；运行相关 Vitest/Storybook/E2E，并采集桌面、平板和手机截图供实现 PR 使用。
- [ ] 6.5 运行 `pnpm --dir web check:arch`、类型检查、Oxlint 和相关格式检查，修复新增违规而不扩大 baseline。

## 7. 单节点部署、运维与用户故事

- [ ] 7.1 实现 `cmd/webmux-relay` 的 TOML/环境装配、公共泛域名、TLS 证书文件、readiness、优雅关闭和证书重载，使用临时证书与随机端口完成启动/重启测试，并验证 `CGO_ENABLED=0 go build ./cmd/webmux-relay`。
- [ ] 7.2 新增 Relay 部署示例和运维指南，覆盖 GitHub OAuth Device Flow、邀请名单、DNS/证书、配额、备份恢复、单节点无 SLA、信任边界和日志脱敏；运行 `make docs-check`。
- [ ] 7.3 按用户故事维护流程新增官方 Relay 用户与部署者故事，更新索引、计数和代码依据，明确未执行的真机/公网验证，不宣称未完成的容量或隐私保证。
- [ ] 7.4 对配置样例、架构、持久化、远程访问、安全策略和协议说明做必要同步，验证全部相对链接与锚点，并确认不更新 `AGENTS.md` 中未改变的既有规则。

## 8. 全量验证与邀请制发布准备

- [ ] 8.1 运行所有 Relay 相关 Go race、Vitest、浏览器和 E2E 用例，汇总协议、认证、背压、撤权和恢复矩阵；任何失败先修复再进入全量门禁。
- [ ] 8.2 在隔离环境启动真实 Relay、WebMux 和浏览器，完成从 GitHub 假授权到公网路径登录、终端、上传、推送来源、P2P、Relay 重启和换钥的完整场景，并记录实际 RTT、内存和 yamux 参数依据。
- [ ] 8.3 运行 `make check` 与 `make test-fast`，合并前运行完整 `make test`；检查 `git status`，删除测试产生的临时文件，保留可复现的验证结果供实现 PR 摘要引用。
