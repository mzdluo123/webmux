# Tasks

## 1. 隔离环境与协议契约

- [x] 1.1 固定两端 frp 版本，在隔离宿主和节点完成原生 QUIC 工作连接、双向认证回源 204、应用 HTTP 和 WebSocket；保存脱敏结果并确认业务数据经过 UDP。
  `TestNativeQUICOrigin` 使用原版 0.72.0、真实 OIDC 与回源 mTLS；TCP 同端口由无 frps 的监听占用。`go test -race -count=1` 通过，回源 HEAD 204、HTTP 正文和 WebSocket 双向标记均匹配。
- [x] 1.2 为控制协议加入能力协商和限时候选分配；双版本组合测试确认旧宿主或旧节点只使用 WSS，未知端点无法触发拨号。
  `TestTransportCapabilityNegotiation` 覆盖旧版、已协商、未知和重复响应；`TestTransportLegacyRecovery` 覆盖旧节点显式恢复，`TestStockProductionQUICSwitch` 覆盖未协商宿主，`TestRelayTransportRejectsUnassignedDialTargets` 拒绝自定义地址。
- [x] 1.3 在两端验证节点证书、宿主分配、连接代际与共享配额；覆盖证书不符、跨宿主、过期和撤销，证明均无法访问回源。

## 2. 节点与宿主切换

- [x] 2.1 节点实现独立候选注册和仅探针可达的目标；令旧路径健康而候选失败，验证不会借用旧 204 提交。
  隔离测试 `TestStockProductionQUICSwitch` 使用真实 frpc/frps 和节点入口，在 `-race -count=1` 下通过；候选探针失败后原 WSS 继续服务，提交成功后 HTTP 和 WebSocket 经 QUIC 回源。
- [x] 2.2 实现限时操作与条件提交；测试并发、响应丢失、取消及撤销竞态，验证至多一个生效协议且临时资源全部释放。
  `TestTransportSelectionAuthority`、`TestTransportCandidateReconciliation` 与 `TestStockProductionQUICSwitch` 验证已提交结果可查、取消与撤销不复活旧连接。`TestRealHostTransportSwitch` 通过受管宿主子进程往返切换并等待旧注册释放，两条路径均取得应用正文。无法证明已释放的注册隔离保留。
- [x] 2.3 宿主保存已确认的选择并实现重启恢复；测试 UDP 不可达时无虚假生效协议，本机可切回 WSS。
- [x] 2.4 更新私有节点部署说明与 US-A09 验收；在隔离节点演练能力发布、停止签发、切回 WSS 和关闭 UDP。

## 3. 设置与诊断

- [x] 3.1 扩展设置 API、状态 DTO、CLI JSON 与前端校验；用同一次试连核对设备间生效协议、节点、耗时、阶段、时间和回源观测。
- [x] 3.2 在官方中转设置实现双语「测试并应用」，覆盖未支持、进行中、成功及失败保留状态；验证键盘、手机触控和桌面截图。
- [x] 3.3 更新 US-A10 与客户端使用说明；按文档从远端试连失败恢复到原链路，并验证本机恢复入口。

## 4. 端到端对照

- [x] 4.1 固定宿主、节点及浏览器路径，在正常、限速丢包和 UDP 不通三档交替测试；分别记录冷启动资源成功率、真实 PTY 按键 p95/p99、超时数和重连时间。
- [x] 4.2 使用隔离 PTY 在两方向切换期间发送可计数输入；验证进程存活、能重新附着、输入不重复，并在撤权后确认两协议均停止访问。
  节点 `scripts/test-host-integration.sh <宿主工作区> --quic` 通过。最终 11 轮使用同一 PTY、每轮 60 次输入、2500 ms 回显期限；受损档 WSS 超时 0/120、QUIC 超时 2/120。
  全部 610 条已执行的完整命令各执行一次，阻断期间的 60 条完整命令均未执行。隔离环境验证了两个协议撤权、绑定删除后连接和监听关闭，以及本机恢复、节点回滚；没有改动生产。
