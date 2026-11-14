<script lang="ts">
  import * as Card from "$lib/components/ui/card";
  import { Label } from "$lib/components/ui/label";
  import { Separator } from "$lib/components/ui/separator";
  import { Switch } from "$lib/components/ui/switch";
  import { getPluginContributions } from "../../plugins/context";
  import { setWsCompression, wsCompression } from "../../platform/ws";

  let compression = $state(wsCompression());
  const providers = getPluginContributions().network;
</script>

<Card.Root flat id="network" class="scroll-mt-4">
  <Card.Header>
    <Card.Title>网络</Card.Title>
    <Card.Description
      >调整实时连接压缩。已启用的端口转发与 P2P 插件可在此配置；
      连接压缩仅对当前浏览器生效，插件设置在服务端持久化。</Card.Description
    >
  </Card.Header>
  <Card.Content class="space-y-6">
    <div class="space-y-2">
      <div class="flex items-center justify-between gap-4">
        <Label for="ws-compression">压缩实时连接（permessage-deflate）</Label>
        <Switch
          id="ws-compression"
          checked={compression}
          aria-describedby="ws-compression-help"
          onCheckedChange={(checked) => {
            setWsCompression(checked);
            compression = wsCompression();
          }}
        />
      </div>
      <p id="ws-compression-help" class="text-xs text-muted-foreground">
        切换后已打开的终端与列表连接会立即重连，输出从断点续传。小于 128
        字节的消息（如按键回显）不压缩，不增加输入延迟；每个连接在服务端多占约
        1.3 MB 内存。手机与平板在未选择时默认开启。
      </p>
    </div>
    {#if providers.ForwardSettings}<providers.ForwardSettings />{/if}
    {#if providers.P2PSettings}
      <Separator />
      <providers.P2PSettings />
    {/if}
  </Card.Content>
</Card.Root>
