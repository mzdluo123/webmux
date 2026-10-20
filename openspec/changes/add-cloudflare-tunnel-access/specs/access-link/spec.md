## Purpose

提供携带 Token 的 webmux 访问链接：服务启动时在终端打印可直接打开的 URL，浏览器打开该 URL 即完成认证，无需手动输入 Token，且 Token 不经由 URL 发送到服务器。

## ADDED Requirements

### Requirement: 启动时打印本地访问地址

开始监听后，系统 SHALL 在标准输出打印一次本地访问 URL：`http://<主机>:<实际端口>/`。监听地址为通配地址时，主机使用 `127.0.0.1`。IPv6 主机 MUST 加方括号。启用隧道时，URL SHALL 追加 `#token=<URL 编码的 Token>`；未启用隧道时 MUST NOT 追加片段，以免固定的 Token 被 service manager 捕获并写入 journal 或日志文件。非隧道模式 MUST NOT 打印二维码。

#### Scenario: 隧道模式的本地地址

- **WHEN** 以 `listen = "127.0.0.1:7681"` 启用隧道，且未配置 `token`（自动生成）
- **THEN** 标准输出包含 `http://127.0.0.1:7681/#token=<生成的 Token>`，且不打印二维码

#### Scenario: 非隧道模式不打印 Token

- **WHEN** 以 `listen = "127.0.0.1:7681"`、`token = "s3cret"` 启动，且未启用隧道
- **THEN** 标准输出包含 `http://127.0.0.1:7681/`，不含 `#token=`
- **AND** 不打印二维码

#### Scenario: 无 Token

- **WHEN** 以 `listen = "127.0.0.1:7681"`、空 Token 启动，且未启用隧道
- **THEN** 标准输出包含 `http://127.0.0.1:7681/`，不含 `#token=`

#### Scenario: 通配监听与动态端口

- **WHEN** 以 `listen = "0.0.0.0:0"` 启动，操作系统分配端口 `40123`
- **THEN** 打印的 URL 主机为 `127.0.0.1`，端口为 `40123`

#### Scenario: 隧道模式的特殊字符 Token

- **WHEN** 启用隧道，Token 为 `a b&c#d-0123456789abcdefghijklmnopqrstuvwxyz`
- **THEN** 打印的片段为 `#token=a%20b%26c%23d-0123456789abcdefghijklmnopqrstuvwxyz`，浏览器打开后得到原始 Token

### Requirement: 从 URL 片段认证

前端 SHALL 在首次请求 API 之前检查 `location.hash`。若片段为 `#token=<值>`，前端 SHALL 解码该值并作为当前 Token 持久化（与手动输入 Token 存入同一位置），同步写入 `webmux_token` Cookie（百分号编码），然后用该 Token 进行首次认证。其他形式的片段 MUST 保持原样，不被解释为 Token。

#### Scenario: 扫码直接进入

- **WHEN** 新设备首次打开 `https://abc.trycloudflare.com/#token=<有效 Token>`
- **THEN** 页面不显示 Token 输入表单，直接进入会话列表

#### Scenario: 覆盖旧 Token

- **WHEN** 浏览器已保存旧 Token，用户打开带新的有效 `#token=` 的链接
- **THEN** 新 Token 替换旧 Token，后续 HTTP 与 WebSocket 请求均使用新 Token

#### Scenario: 片段 Token 无效

- **WHEN** 用户打开带无效 `#token=` 的链接
- **THEN** 页面显示现有的 Token 输入表单及认证失败提示，用户可手动输入正确 Token

#### Scenario: 无关片段保持不变

- **WHEN** 用户打开 `/settings#quick-commands`
- **THEN** 前端不修改已保存的 Token，片段保持不变，设置页照常定位到快捷命令

### Requirement: WebSocket 认证不经过 URL

前端 MUST 在 WebSocket 握手前把当前 Token 写入 `webmux_token` Cookie：值使用百分号编码，设置 `Path=/` 与 `SameSite=Strict`，页面为 HTTPS 时加 `Secure`。WebSocket 请求 URL MUST NOT 携带 `?token=`。服务端 SHALL 在比较前对 Cookie 值做百分号解码。

#### Scenario: 片段登录后建立 WebSocket

- **WHEN** 用户通过带 `#token=` 的链接登录后，前端连接 `/ws/sessions`
- **THEN** 握手携带 `webmux_token` Cookie，请求 URL 不含 `token`，连接成功

#### Scenario: 手动输入 Token 后建立 WebSocket

- **WHEN** 用户在 Token 表单输入有效 Token 并进入会话列表
- **THEN** 后续 WebSocket 握手同样使用 `webmux_token` Cookie，不使用 query 参数

### Requirement: 认证片段不残留

前端 MUST 在读取 `#token=` 片段后立即用替换方式移除地址栏与当前历史记录中的片段，保留路径与查询参数，且 MUST NOT 新增历史记录项。`#token=` 只能作为片段使用；系统 MUST NOT 新增通过页面查询参数 `?token=` 认证 HTTP 页面或 `/api/` 请求的入口。

#### Scenario: 地址栏不再显示 Token

- **WHEN** 打开 `https://host/s/abcdefgh#token=xyz`
- **THEN** 认证后地址栏显示 `https://host/s/abcdefgh`，按浏览器“后退”不会回到带 Token 的地址

#### Scenario: 片段不发送到服务器

- **WHEN** 浏览器加载 `http://127.0.0.1:7681/#token=xyz`
- **THEN** 服务器收到的请求 URL 不包含 `token`
