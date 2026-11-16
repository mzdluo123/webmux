# 交接实现与检查

交接采用原生只读范围、Snapcompact 文本预览和普通 prompt 单次发送。
测试按内容身份、并发消费、操作状态和旧文件保留组织。

## 故障矩阵

- 同一预览并发确认：目标创建和 prompt 各得到一次执行权。
- 相同操作重复请求：返回进程内已有结果；修改预览引用产生冲突。
- 来源范围后追加：预览沿用原边界；范围内替换产生来源变化错误。
- 目标原生身份或配置变化：发送在 actor admission 阶段拒绝。
- 准备阶段取消：操作结束；已进入外部调用的结果按实际确认信息记录。
- prompt 确认丢失：unknown 状态显示「可能已发送，请检查目标会话」。

## 缓存与生命周期

预览限五分钟、32 项、64 MiB；操作结果限 256 项、完成后十五分钟。
发送中的 scope 计入预算，缓存清理按请求执行。
服务重启后旧句柄失效；普通 hybrid 恢复以原生 session 身份启动。
旧数据库及 journal/WAL/SHM/lock 保留原件。

## 测试入口

`internal/handoff/service_test.go` 覆盖并发、操作冲突、结果不明、取消、容量与资源信息。
`internal/historyimport/scope_test.go` 覆盖追加、替换、实例隔离和缓存限制。
`internal/api/native_handoff_test.go` 覆盖认证、Origin、旧协议和遗留文件。
九个原生读取器的版本夹具位于各 Agent 的 history 包中。
浏览器流程位于 `web/test/e2e/handoff.spec.ts`，wire fixture 由 API 测试生成。
