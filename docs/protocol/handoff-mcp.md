# MCP：冻结原文补查

`thread_read` 通过当前 WebMux 服务读取已交接给目标聊天的短期原生范围。
每个 MCP 子进程由目标 runtime ID 绑定，来源 scope 在工具请求中明确提供。

## 读取与分页

请求字段为 view=activity、snapshotId，以及可选 afterPosition、itemId、textOffset、limit、maxBytesPerItem。
snapshotId 来自交接文本中的 Frozen scope；首次位置为字符串 0。
返回原生 records、nextPosition、nextTextOffset、more、done 和来源信息。
长项使用同一 snapshotId 与返回的 UTF-8 字节偏移续读。
原文范围包括预算省略的可导出记录，范围边界与预览一致。

## 配置与生命周期

```sh
webmux mcp-serve --socket /path/webmux.sock --session <runtime-id>
```

选择 `mcp` 功能时，普通 ACP 启动配置加入 `webmux-history` 只读工具。
服务端按目标运行身份与已登记的 scope 检查访问，正文通过本机 API 返回。
预览五分钟后失效，服务重启后对应句柄失效。
目标 Agent 的原生会话恢复继续使用已有 hybrid 恢复流程。

## 协议与实现依据

传输使用现有 MCP stdio registry、帧大小限制和调用期限。
实现位于 `internal/threadread`；认证与来源校验由 `internal/featureapi` 承接。
网络控制 MCP 的 session_read 使用同一[原生历史分页](http-conversation.md)，后续页携带 snapshotId。
