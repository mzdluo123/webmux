<script lang="ts">
  import { pluginContributions } from "../../plugins/registry.svelte";
  import { relayAuth } from "../../platform/relay-auth.svelte";
  import { relayDiagnostic } from "../../domains/settings/relay-diagnostics";
  import InlineNotice from "../shared/InlineNotice.svelte";

  const optimizer = $derived(pluginContributions.entries.p2p?.p2pState);
  const labels = {
    terminal: "终端",
    chat: "聊天",
    list: "会话列表",
    other: "其他业务",
  };
  const pathLabels = {
    ws: "WS",
    wss: "WSS",
    "webrtc-direct": "WebRTC 直连",
    "webrtc-turn": "WebRTC TURN",
  };
</script>

<section class="space-y-3" aria-label="当前浏览器的实际交互路径">
  <h2 class="text-base font-semibold">当前浏览器的交互路径</h2>
  <p class="text-sm text-muted-foreground">
    登录、设置、文件、语音、全部转发及未分类请求保持
    WSS。只有终端、聊天与列表可按实际测量优化；同一浏览器可能混合使用多条路径。
  </p>
  {#if !optimizer}
    <InlineNotice>交互优化实现尚不可用；基础请求不会等待 ICE。</InlineNotice>
  {:else}
    <p class="text-sm" role="status">
      WSS 基线 · {relayDiagnostic(optimizer.reason) ||
        "等待稳定健康与延迟收益采样"}
    </p>
    {#if !relayAuth.authenticated}
      <p class="text-xs text-muted-foreground">
        当前没有有效的 Relay 所有者浏览器登录；恢复 Token 或设备身份不能领取
        TURN 凭据。
      </p>
    {/if}
    {#if optimizer.inputUncertain}
      <InlineNotice tone="warning"
        >路径已变化；部分已发送输入尚未确认是否接受。系统不会自动重发，请检查终端回显或提交状态再决定下一步。</InlineNotice
      >
    {/if}
    <ul class="space-y-2 text-sm">
      {#each optimizer.streams as stream (stream.id)}
        <li>
          {labels[stream.kind]} #{stream.id} · {pathLabels[stream.path]} · {stream.state ===
          "connected"
            ? "已连接"
            : stream.state === "reconnecting"
              ? "重新附加中"
              : "连接中"}
        </li>
      {:else}
        <li>尚无活动交互流；候选连接成功不代表已切换业务。</li>
      {/each}
    </ul>
  {/if}
</section>
