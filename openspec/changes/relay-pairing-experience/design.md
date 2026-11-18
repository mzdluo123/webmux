# Design

## Context

现有 relayController 保存待配对 seed 和句柄摘要，但创建结果及轮询只属于浏览器。
组件卸载发送取消；临时轮询失败未安排下一次查询；Delete 丢弃 accessClosed。
继续使用 v2 新绑定与 v3 设置文件，主进程拥有活动流程，浏览器只观察。
现行客户端规格接近 400 行治理上限，配对、机器身份和设置要求迁移到 official-relay-pairing；同机丢失绑定场景遵循已有全新绑定约束。

## Goals / Non-Goals

**Goals:** 页面重开恢复同一尝试，有限等待，可核对未知结果，清晰区分删除与关闭。

**Non-Goals:** 旧绑定恢复、rebind、自动公网启用、生产授权、持久化设备码或轮询句柄。

## Decisions

### 本机流程接口

GET /api/relay/enrollments 返回 RelayPairing；POST 同路径接受 {confirmPublic:true} 并返回该对象；DELETE 同路径接受空对象并返回该对象。
删除浏览器 POST /enrollments/poll；控制面协议不变。
RelayPairing 字段为 phase、userCode、verificationUrl、expiresAt、pollIntervalSeconds、retryAt、error。
phase 枚举 idle/preparing/waiting/retrying/uncertain/cancelling/cancelled/paired/denied/expired/interrupted/failed；时间为 Unix 秒，空字段为零或空串。
专用接口保留现有认证、Origin 与会话内进程限制并禁缓存；GET /api/relay 与 config/status 不携带用户码或轮询句柄。

### 宿主授权所有权

单次创建先占用流程再执行网络调用，重复创建返回同一活动流程。宿主在生命周期上下文上执行有界请求，浏览器离开不取消。
句柄只留宿主内存；按照返回间隔、Retry-After 和有界退避继续轮询，同一次尝试不换钥。
明确拒绝和过期清理待配对 seed；成功保存绑定但保持关闭；取消与成功串行核对权威结果，迟到结果不能覆盖新尝试。
上游创建结果未知且无句柄时如实展示 uncertain，禁止自动重发，保留取消出口；用户取消或有效期结束才可重开。
重启发现残留 pending seed 清理并报告 interrupted，不声称恢复远端授权上下文。

### 其他操作回执

RelayStatus 增加 operation 对象：kind 为 idle/enable/disable/reconnect/delete，phase 为 idle/submitting/completed/failed/uncertain，error 为脱敏代码，deleted/accessClosed 为布尔值。
开始、完成与失败由宿主记录；已受理操作使用宿主有界上下文完成，浏览器超时后只 GET 核对，不重放。
删除已确认则清除身份；deleted=true/accessClosed=false 维持完成回执与准确警告，允许新配对。全局退出严格撤权保持原实现。

### 页内任务面板

未配对仅说明用途、安全边界和「使用 GitHub 配对」。授权中原位显示用户码、用户触发复制、原始 verificationUrl、剩余有效期、等待说明和取消。
已配对显示账号及公网开关；就绪才复制/打开地址；启用确认说明公网 Shell 与 Relay 可读取转发内容。
连接诊断按连接健康、路由分配、会话候选分组渐进展开，不重复主流程状态；删除放次级危险操作区。
采用现有 Button/Card/InlineNotice/Collapsible/Skeleton；双语消息、键盘与读屏保持可用。
参考 GitHub CLI 的设备码动作、Carbon 原位反馈与渐进披露信息架构。

## Risks / Trade-offs

- 无恢复协议的上游创建响应丢失无法证明成功；保留真实未知状态，用户取消后由明确新动作重开。
- 进程重启仅恢复持久绑定；活动授权中断是明确可见边界。
- 浏览器 API 模拟验证 UI；宿主隔离 fake client 验证真实状态机与竞争，均不访问生产。
- 截图与交互 GIF 使用会话 artifactsDir；模拟手机不能证明真机键盘或触控。
