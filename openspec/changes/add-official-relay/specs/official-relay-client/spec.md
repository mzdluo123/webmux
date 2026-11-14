# official-relay-client Specification

## Purpose

WebMux 宿主通过独立中继密钥与官方 Relay 配对，以固定 HTTPS 地址提供低配置公网访问，并在连接断开时立即撤销入口授权、自动恢复连接，同时保持现有 Cloudflare、Tailscale、Web Push 和 P2P 能力的独立行为。

## ADDED Requirements

### Requirement: 默认关闭与公网启用前提

官方 Relay SHALL 默认关闭，且与 Cloudflare Tunnel、Tailscale 的开关和凭据相互独立。只有完成主机配对、设置存储可用且当前 WebMux 登录 Token 不少于 43 字符时，用户才允许启用 Relay。

启用前 SHALL 显示公网 Shell 风险确认。启用失败 MUST NOT 阻止本机 HTTP 服务或现有终端会话；运行中启用或关闭 MUST NOT 重启 WebMux。

#### Scenario: 未配对时尝试启用

- **WHEN** 主机尚未完成 Relay 配对，用户请求启用
- **THEN** 系统拒绝启动连接器并说明需要先配对，不建立任何公网连接

#### Scenario: 登录 Token 过弱

- **WHEN** 已配对主机当前登录 Token 少于 43 字符
- **THEN** 启用被拒绝并给出修复方式，本机服务继续运行

### Requirement: 本地密钥与账号配对

WebMux SHALL 在本机生成并保存独立的 Ed25519 中继密钥对。配对接口 SHALL 返回官方验证地址、一次性用户码和剩余有效期，供用户在浏览器完成 GitHub 授权；WebMux 登录 Token MUST NOT 出现在配对请求中。

私钥、轮询句柄和 GitHub 凭据 MUST NOT 出现在状态响应、诊断、日志或浏览器持久化中。配对失败或过期 SHALL 允许重试，不得改写 WebMux 登录 Token。

同一账号的重新绑定 SHALL 需要显式确认；成功后旧私钥被替换，公共地址保持不变。

#### Scenario: 从设置页完成配对

- **WHEN** 用户打开 Relay 设置、开始配对并在官方页面完成 GitHub 授权
- **THEN** WebMux 显示已配对和固定公共地址，本机仅保存中继私钥与绑定信息，不保存 GitHub 访问凭据

#### Scenario: 配对接口不包含登录 Token

- **WHEN** 宿主创建或轮询配对请求
- **THEN** 请求只包含中继公钥或配对材料，不包含 WebMux 登录 Token

### Requirement: 受控持久化与安全降级

Relay 的启用意图、服务端点、主机标识、公共地址和私钥 SHALL 通过登记的受控状态文件持久化，目录权限为 0700、文件权限为 0600，并使用原子写入和独立锁。

状态文件损坏、不可读或锁冲突时，系统 SHALL 保留原文件、报告存储不可用并禁止启动新的 Relay 公网连接，不得回退到 TOML 意外开启。关闭操作即使落盘失败也 SHALL 立即撤销当前公网授权并停止连接，同时返回未持久化错误。

重启后 SHALL 按持久化启用意图恢复连接；TOML 只提供默认开关与运维端点，网页绑定与私钥不得写入 `config.toml`。

#### Scenario: 损坏设置阻止自动暴露

- **WHEN** Relay 状态文件损坏且 TOML 配置为启用
- **THEN** 本机服务正常启动，Relay 保持关闭并显示存储错误，不尝试连接公网

#### Scenario: 关闭时存储写失败

- **WHEN** 用户关闭 Relay 但状态文件无法写入
- **THEN** 当前连接和公共来源授权立即撤销，响应指出关闭状态未持久化，修复后需重新保存

### Requirement: 连接状态机与公网来源撤权

Relay 客户端 SHALL 区分关闭、连接中、就绪、bulk 降级和失败状态，并单独表示未配对、配对中与已配对。只有 control 认证和 stream 服务均就绪后，客户端才将固定 HTTPS 来源登记为允许的 Host/Origin 并公开地址；bulk 未就绪时 SHALL 显示降级并拒绝新批量请求。

control 断开、绑定撤销、关闭或服务退出时，客户端 SHALL 立即撤销该来源的 Host/Origin 授权。新连接认证失败 MUST NOT 替换或撤销既有合法 control；仅 bulk 失败 SHALL 按同代际降级重连。迟到的连接事件 MUST NOT 在关闭或新配置后重新授权旧地址。

启用状态下发生意外断线时，客户端 SHALL 按有界指数退避和随机抖动自动重连；本地会话和其他远程入口不受影响。代际或等效机制 SHALL 防止并行重连产生多个活动连接器。

#### Scenario: 断线后恢复

- **WHEN** 就绪的 Relay 连接因网络中断失败，随后网络恢复
- **THEN** 中断期间公共来源被撤销，客户端自动重连，成功后使用原固定地址重新授权

#### Scenario: 关闭期间收到迟到就绪事件

- **WHEN** 用户已关闭 Relay，先前连接随后报告就绪
- **THEN** 该事件被丢弃，公共来源保持未授权，不创建新连接器

### Requirement: 经 Relay 的完整应用行为与 P2P 基线

Relay 就绪后，浏览器 SHALL 能通过公共地址使用现有登录、会话管理、终端回放与续接、聊天、语音、设置、上传下载和 `/forward/*` 端口转发。Relay 传输 MUST NOT 绕过 WebMux 的认证、Host、Origin 或会话内进程限制；认证可使用既有 Token 或下述绑定 Relay Origin 的有效登录会话。

现有 P2P SHALL 把 Relay 视为新的 HTTP/WebSocket 基线：首屏不等待 P2P，仍按既有候选与 RTT 策略机会性升级和回落。MVP MUST NOT 为 Relay 放宽直连候选限制、引入 TURN 或修改 P2P 帧协议。

关闭 Relay SHALL 撤销 Relay 基线入口，但不得暗中修改独立的 P2P 配置；P2P 活动连接的既有处理保持与 Cloudflare 基线断开时一致。

#### Scenario: Relay 上完成 P2P 升级与回落

- **WHEN** 浏览器经 Relay 打开 WebMux，后台 P2P 直连满足现有升级条件，随后 P2P 断开
- **THEN** 首屏先经 Relay 可用，升级和回落沿用原有无缺口续接，回落目标为同一 Relay 地址

#### Scenario: Relay 不绕过认证

- **WHEN** 未携带有效 WebMux Token 或 Relay 登录会话的浏览器经 Relay 请求 `/api/sessions` 或 `/ws/sessions`
- **THEN** WebMux 返回与直连公网入口一致的 401，Relay 不以代理头或注入长期 Token 代替宿主认证

### Requirement: 固定访问地址、状态与推送来源

Relay 就绪后，固定公共地址 SHALL 出现在现有访问 URL 汇总、本机状态和 Relay 设置页，且地址与二维码 MUST NOT 自动附带 WebMux Token。Windows 托盘 SHALL 通过现有通用访问入口展示该地址，无需新增 Relay 专属菜单协议。

配对的固定 HTTPS 来源 SHALL 纳入现有 Web Push 来源管理。新订阅只能从当前通过 Host/Origin 校验的入口开启；Relay 离线期间已有通知可以保留，点击后等待入口恢复。

状态与错误输出 MUST NOT 包含私钥、配对材料、GitHub 凭据或 WebMux Token。

#### Scenario: 查看远程访问入口

- **WHEN** Relay 与 Cloudflare Tunnel 同时就绪
- **THEN** 访问 URL 汇总分别列出两个固定来源，撤销其中一个不会移除另一个

#### Scenario: 从 Relay 地址开启推送

- **WHEN** 手机通过就绪的 Relay HTTPS 地址打开 WebMux 并开启通知
- **THEN** 订阅关联该固定来源，重启 WebMux 并重连 Relay 后通知仍指向同一地址

### Requirement: 设置操作与删除绑定

WebMux SHALL 在公网访问设置中提供开始配对、启停、刷新状态、重新绑定和删除绑定操作，并显示期望开关、运行状态、固定地址及失败原因。

删除绑定 SHALL 先停止当前连接并由官方 Relay 确认撤销，再删除本地绑定与私钥。官方服务不可达时，系统 SHALL 保留本地绑定信息以便重试，明确提示远端尚未撤销，不得谎称删除完成。

设置操作 SHALL 保留现有 API 认证和 Origin 校验；界面在桌面与手机布局可用，并在当前操作会断开正在使用的公网页面时给出确认提示。

#### Scenario: 成功删除绑定

- **WHEN** 用户确认删除且官方 Relay 完成撤销
- **THEN** 当前连接停止、公共来源撤权、本地私钥与绑定被删除，公共地址不再接受该主机连接

#### Scenario: 删除时官方服务不可达

- **WHEN** 用户删除绑定但 Relay 控制面无法确认撤销
- **THEN** 本地连接保持关闭，绑定信息保留用于重试，界面明确显示远端尚未撤销

### Requirement: GitHub 跳转与宿主浏览器会话

宿主 SHALL 显式信任已配对 Relay 的所有者认证，并保存所有者 GitHub 数字 ID 与绑定代际。独立直连、Cloudflare 与 Tailscale 入口 SHALL 保持既有 Token 认证；Relay 会话 MUST NOT 在其他 Origin 使用。

宿主 SHALL 为后台跳转创建绑定浏览器的单次 state 和 PKCE S256 事务，有效期不超过 5 分钟；回调必须校验事务，并通过宿主密钥认证的控制通道携 verifier 兑换 code。账号、host ID、受众或绑定代际不符 SHALL 拒绝登录，MUST NOT 仅相信浏览器携带的身份声明。

成功后宿主 SHALL 创建绝对有效期不超过 12 小时的内存会话，使用 host-only、Secure、HttpOnly、SameSite=Lax、Path=/ 的 `__Host-` Cookie；MUST NOT 写入 localStorage、sessionStorage 或 URL。回调 SHALL 禁缓存、禁止 Referer 泄露并跳至无凭据 URL。长期 WebMux Token MUST NOT 参与跳转或自动下发。

#### Scenario: 新设备免 Token 打开面板

- **WHEN** 所有者在新设备登录官方后台并点击自己的在线主机
- **THEN** 宿主验证一次性授权并打开可操作的面板，不要求输入 WebMux Token，不向浏览器下发该 Token

#### Scenario: 回调被其他浏览器或主机使用

- **WHEN** code 在没有对应临时 Cookie/state 的浏览器使用，或兑换身份与本机绑定不符
- **THEN** 宿主拒绝建立会话且不改变任何绑定，用户可重新开始登录

### Requirement: 会话认证覆盖与撤销

Relay 会话 SHALL 覆盖 HTTP、WebSocket、P2P 信令和 `/forward/*`，前端 SHALL 通过现有 API/WS 封装识别登录态。Cookie 写请求 MUST 校验同源 Origin 与 CSRF token，转发上游前 MUST 剥离宿主认证 Cookie 和 CSRF 材料。

宿主注销、会话过期、对应后台会话撤销、绑定撤销、换钥、移出邀请名单、Relay 关闭或断线 SHALL 撤销相关宿主会话。宿主重启 SHALL 清空会话。撤销 SHALL 拒绝后续请求并关闭该会话建立的 WebSocket、P2P 和转发连接，但 MUST NOT 终止持久 PTY。

#### Scenario: 关闭 Relay 后旧会话不能复用

- **WHEN** 用户关闭后重新启用 Relay，再使用旧会话 Cookie 访问
- **THEN** 旧会话被拒绝，用户需重新登录 GitHub 后台，主机配对和持久 PTY 保留

#### Scenario: 端口转发不泄露登录会话

- **WHEN** 已登录用户通过 `/forward/*` 访问目标应用
- **THEN** 目标应用收不到宿主认证 Cookie 或 CSRF 材料，跨源伪造写请求被拒绝

#### Scenario: 已升级 P2P 的会话过期

- **WHEN** Relay 登录会话过期且浏览器仍有活动 P2P 或 WebSocket 连接
- **THEN** 相关访问连接被关闭、后续请求被拒绝，持久 PTY 继续运行

### Requirement: WSS 连接隔离与重连诊断

宿主 SHALL 使用运维配置的 HTTPS endpoint 与 WSS + smux v2，MVP 传输固定 wss，不提供 auto/quic 或 UDP endpoint，不接受浏览器指定拨号地址。

宿主 SHALL 在同一逻辑代际内认证 control/bulk 两条物理连接，角色纳入签名并共用配额。control 断开 SHALL 撤销来源及派生会话并关闭 bulk；仅 bulk 断开 SHALL 保留交互、显示降级和 bulkReady=false，有界重连 bulk，新批量请求返回不可用，不挤占 control。

证书、pin、主机身份与协议错误 MUST NOT 通过重试绕过。网络重连 SHALL 使用有界退避，报告实际传输、角色就绪数与脱敏 reconnectReason；MUST NOT 自动重放非幂等业务请求。

#### Scenario: UDP 黑洞不延迟 WSS 建连

- **WHEN** UDP 不可达而目标 TCP 443 可达
- **THEN** 客户端直接建立 WSS，不等待 QUIC 超时；实际可达性仍以 WSS 握手结果为准

#### Scenario: 错误证书保持失败

- **WHEN** WSS 服务证书或 pin 校验失败
- **THEN** 连接失败并报告安全错误，不切换信任配置或其他传输

### Requirement: 内置官方端点与公开服务身份

开源客户端 SHALL 内置官方 HTTPS/WSS 地址、Relay 受众和官方 ECDSA P-256 TLS 服务身份公钥或对应 SHA-256 SPKI pin 集合。用户 SHALL 无需填写官方地址、公钥或 OAuth 应用配置即可发起 GitHub 配对；内置默认 MUST NOT 自动启用 Relay 或替代公网确认。

所有到 Relay 的 HTTPS/WSS 连接 SHALL 同时通过正常 TLS 证书链、有效期、主机名及公钥 pin 校验；MUST NOT 跳过 TLS 校验、自动学习未知公钥或在校验失败时降级绕过。公钥仅用于认证服务身份，宿主私钥 SHALL 继续逐机生成并用于主机认证。

官方私钥、OAuth Client Secret 和用户 GitHub access token MUST NOT 进入开源仓库、客户端二进制或公开 API。OAuth Client ID SHALL 视为公开标识，不作为客户端认证秘密。

#### Scenario: 使用官方默认配置配对

- **WHEN** 用户安装带有有效官方信任配置的开源客户端并选择官方 Relay
- **THEN** 客户端使用内置地址与公钥校验服务，用户无需填写地址或 OAuth 凭据，仍须完成 GitHub 配对与启用确认

#### Scenario: 证书有效但官方公钥不匹配

- **WHEN** 官方连接返回有效证书但其公钥不在内置信任集合
- **THEN** 客户端拒绝连接，不提交配对材料或宿主身份凭据，不通过重连绕过错误

### Requirement: 公钥轮换与自建信任隔离

官方公钥轮换 SHALL 支持当前与下一公钥的重叠信任集合，先发布客户端再换服务端公钥；未知公钥 SHALL 拒绝。公钥泄露后的恢复 SHALL 允许要求客户端升级，MUST NOT 声称未经更新的客户端能够立即撤销旧公钥。

自建端点覆盖 SHALL 显式配置独立服务受众与公钥集合，并保留标准 TLS 校验。变更信任配置 SHALL 先撤销旧连接、公共来源和会话，确认后重新配对；旧绑定、宿主凭据及未完成授权材料 MUST NOT 发送到新服务。

#### Scenario: 官方公钥正常轮换

- **WHEN** 客户端已安装包含当前与下一公钥的版本，官方服务器切换到下一公钥
- **THEN** 客户端继续连接；只有旧公钥的客户端明确报告升级要求，不自动接受新公钥

#### Scenario: 从官方切换到自建服务

- **WHEN** 用户将官方默认配置覆盖为自建地址和公钥
- **THEN** 旧连接和会话失效，新服务要求独立配对，不能取得旧服务的绑定凭据或使用官方信任身份
