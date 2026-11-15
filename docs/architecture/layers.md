# 架构分层与线上契约

修改包依赖、构造入口或线上 JSON 时，以本页的分层规则和架构守卫为准；总体结构见[架构设计](../architecture.md)。

## 分层与架构测试

后端分为五层，`internal/archtest` 用 `golang.org/x/tools/go/packages` 加载带类型信息的源码检查规则；违规让 `go test ./...` 失败，一次列出全部违规，每条形如 `文件:行 [Rn] 消息 — 修复提示`。规则基于类型解析，导入别名、点导入和函数值都绕不过。

| 层             | 包                                                                                                         | 职责                                                                |
| -------------- | ---------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------- |
| 装配根         | `cmd/...`                                                                                                  | 唯一构造 Service、把配置转换为 Service 选项并注入 Controller 的位置 |
| Controller     | `api`、`hub`、`wire`、`transport`、`pluginapi`、`pluginhub`、`pluginwire`、`mcphttp`                       | 路由、认证、状态映射、传输和投影                                    |
| Service        | `session`、`sessioncreate`、`hybrid`、`conversation`、`workspace`、`launch`、`settings`、`relaysetting` 等 | 业务规则、状态、文件系统与进程                                      |
| 共享           | `builtin`、`contract`、`streamproto`、`config`、`runtime`、`statefile`、`acp`、`agentprofile/...` 等       | 契约与基础设施，不依赖 Service 或 Controller                        |
| 服务器外客户端 | `client`、`localclient`、`attach`、`clicmd`、`tray/...`、`wslclient`、`tunnel`、`relay`、`service`         | 只通过线上协议访问服务器，只依赖共享层和其他客户端                  |

| 规则 | 检查                                                                                                                                                                                                                                                                                      |
| ---- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| R0   | `internal/` 下每个包都在清单中，清单没有过期条目；出站与编码登记指向 Service 包；纯函数登记具有理由、真实包级函数及 Controller 使用点                                                                                                                                                     |
| R1   | 依赖方向：Service 不导入 Controller、`cmd/`、WebSocket 库或 `net/http/httptest`；共享层与客户端不导入 Service 或 Controller                                                                                                                                                               |
| R2   | 只有 `cmd/` 导入 `internal/api`                                                                                                                                                                                                                                                           |
| R3   | Controller 不导入 `os/exec`、`syscall`、`golang.org/x/sys/...`、PTY 库或 `internal/statefile`                                                                                                                                                                                             |
| R4   | Controller 不使用读写、创建或删除文件系统条目的 `os` 函数（`os.Hostname` 等不受限）                                                                                                                                                                                                       |
| R5   | Controller 的构造、状态和 I/O 操作只调用注入实例的方法；只允许显式登记并附理由的 Service 包级纯函数（目前为 `push.Validate`、`hybrid.FormValue`）；可以引用 Service 的类型、常量与哨兵错误                                                                                                |
| R6   | Service 不使用服务端 HTTP 符号，不声明以 `*http.Request` 为参数的函数；只有登记为出站的 `push` 可以使用 HTTP 客户端符号                                                                                                                                                                   |
| R7   | Service 错误不携带整数 HTTP 状态字段或映射方法；允许大小、行号、退出码等业务整数及非错误对象的 `Status() int`。状态映射归 Controller 的 `api/errors.go` 与 `pluginapi/errors*.go`                                                                                                         |
| R8   | Controller 编解码的每个 JSON 值都是 `contract` 中的具名类型（可经指针、切片、映射）；汇点是 `encoding/json` 的编解码函数和登记的包装函数（见 `layers_test.go` 的 `sinks`，如 `api.reply`、`api.serve`）；接口类型的值只能出现在登记的包装函数内；`contract` 类型的字段不引用 Service 类型 |
| R9   | Service 不声明 `MarshalJSON`；JSON 编码器只能在登记函数体内直接调用（Web Push 载荷与 VAPID 头、代码视图摘要）。检查整个源码，包括包级初始化；禁止保存编码器函数值或方法值，即使位于登记函数中                                                                                             |
| R10  | `contract` 不导入 Service 或 Controller                                                                                                                                                                                                                                                   |
| R11  | Controller 只在登记函数（`layers_test.go` 的 `bodyReaders`：`decode`、`decodeOptional`、`decodeStrict`、`receiveUpload`）内读取请求体；`r.Body` 与 `ParseForm`、`MultipartReader` 等消费请求体的方法其他位置都不允许，新增 JSON 接口必须经这些函数，大小上限与未知字段策略因此集中        |

**维护方式**

- 新增 `internal/` 包时，在 `internal/archtest/layers_test.go` 的清单中归类；删除包时同时删除条目。
- 例外只能写进清单的登记表（`outbound`、`sinks`、`bodyReaders`、`encoders`、`pure`），并附理由，评审时可见。纯函数登记不能用于构造、状态访问或 I/O；缺失、未使用或指向非函数的条目使测试失败。依赖方向没有例外。
- 每条规则由 `internal/archtest/testdata/fixture` 中带 `// want Rn` 注释的故意违规验证；修改规则时同步增加违规与合规样例。
- `AGENTS.md` 有 10 KiB 体积预算，由 `TestAgentsGuideSize`（`internal/archtest/agents_size_test.go`）检查；超限时把最不常用的规则原文迁入 `docs/agent-rules/`，规则索引见 [Agent 规则](../agent-rules.md)。
- Service 以哨兵错误报告错误种类（`workspace.ErrConflict`、`codeview.ErrGone`、`files.ErrTooLarge` 等），Controller 用 `errors.Is` 映射状态码，错误文本不变。
- 线上类型集中在 `internal/contract`；核心转换在 `wire`，可选转换在 `pluginwire`，保持 nil 与空集合区别（`null` 与 `[]`/`{}`）。持久化类型保留自己的 json 标签但不直接上线。

**线上快照**

`internal/api/testdata/wire/*.json` 是每个 REST 端点、`/ws/sessions` 事件和终端 JSON 帧的规范化快照。

- 动态时间、随机 ID、主机名及临时路径只按端点/帧的明确 JSON 路径处理，路径替换检查边界。
- 业务正文、确定性 Git OID、版本摘要和固定输入保留。
- 九组场景（sessions-terminal、basics-errors-fs-upload、launch、workspace、codeview、push、settings、hybrid、hybrid-records）各自持有夹具、期限和归一化状态，可用 `-run '^TestWireSnapshots$/codeview$'` 等单独运行；隧道另有 `TestWireSnapshotsTunnel`。

线上形状变化会给出 JSON 差异；有意的协议变更同时修改 `web/src/contracts/schemas.ts` 并重新录制：

```sh
go test ./internal/api -run TestWireSnapshots -update
```

快照文件的差异随变更一起评审。

真实终端输出分配基准：`go test ./internal/hub -run '^$' -bench '^BenchmarkAttachOutput$' -benchmem`。覆盖连续输出、输出间的控制消息与大块输出，通过真实 PTY 和 WebSocket 校验字节及顺序；比较前后版本时使用同一份基准和参数，不能把短采样中的耗时波动直接解释为吞吐提升。

## Relay 公开协议与宿主边界

`protocol/relay` 是独立公开 Go 子模块，只包含版本化 DTO、校验、签名 transcript 与有界 framing；不包含私有控制面、PostgreSQL、账号授权或数据节点业务实现。两个服务端仓库通过固定发布版本依赖，不用本地 `replace`。

- Controller 的 JSON 仍须使用 `internal/contract` 具名类型；只有受审白名单中的 Relay DTO 可以通过显式契约别名复用公开子模块，不能推广为任意外部类型或 Service 类型透传。
- `internal/relay` 属于服务器外客户端：只负责宿主出站连接、协议与受信固定回环地址，不导入宿主 Service/Controller，也不自行构造它们。
- `internal/relaysetting` 是宿主服务：保存设置与 Ed25519 身份，管理浏览器会话及所有者 Peer 授权。Cookie 返回中性值，由 API 添加 HTTP 属性；身份头不能建立受信连接器上下文。
- 只有 `cmd/` 构造连接器、认证服务和短命 mTLS 回环入口，并把已认证所有者权限映射给 Pion。未选择 P2P 不构造或启动它，Relay 的 WSS 基线独立可用。
- 子模块有单独的 `go.mod`，因此 Make/CI 必须显式检查其 vet/race；主模块的 `go test ./...` 不会自动递归进入子模块。
