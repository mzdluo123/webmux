# 交接的原生身份与文本发送

交接来源使用 Agent 原生历史中的 session、分支与记录边界。
目标通过 hybrid 的普通 prompt 接收机械历史投影和当前用户任务。

## 原生身份

运行身份来自 hybrid 记录中的 runtime ID、Agent、native ID、HistoryRoot 与 cwd。
来源发现按原生 ID 精确匹配，读取器验证 cursor、格式、解析版本和内容 hash。
旧记录缺少原生身份或历史根目录时，接口返回来源不可用。

## 发送过程

确认前保存进程内预览，确认后原子消费并登记 operationId。
新目标通过普通 sessioncreate 流程取得目录、worktree 和聊天会话。
已有目标继续使用其当前原生身份和配置。
发送时在目标 actor 队列中再次核对预览绑定的目标身份。
Agent 返回已确认内容或 prompt 响应后，操作记录成功状态。

## 结果与生命周期

明确拒绝记录失败原因；调用结果无法确认时记录 unknown 并提示检查目标聊天。
页面提供目标会话和已创建资源的位置，以及同一操作的查询入口。
关闭服务取消生产者并等待退出，旧交接句柄随服务实例结束。
下一次启动从 hybrid 记录恢复普通原生会话。
协议字段见[HTTP 交接接口](http-handoff.md)，原文补查见[thread_read](handoff-mcp.md)。
