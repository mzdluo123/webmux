<script lang="ts">
  import { Label } from "$lib/components/ui/label";
  import { Switch } from "$lib/components/ui/switch";
  import { HIGH_PORT_MIN } from "../../domains/workspace/forwards.svelte";
  import {
    changeSettings,
    settings,
  } from "../../domains/settings/settings.svelte";

  const ignoreHighPorts = $derived(settings.values.ignoreHighPorts ?? true);
</script>

<div class="space-y-2">
  <div class="flex items-center justify-between gap-4">
    <Label for="ignore-high-ports"
      >忽略 {HIGH_PORT_MIN} 及以上端口的转发提示</Label
    >
    <Switch
      id="ignore-high-ports"
      checked={ignoreHighPorts}
      disabled={!settings.ready}
      aria-describedby="ignore-high-ports-help"
      onCheckedChange={(checked) =>
        changeSettings({ ignoreHighPorts: checked ? null : false })}
    />
  </div>
  <p id="ignore-high-ports-help" class="text-xs text-muted-foreground">
    默认开启。不为 {HIGH_PORT_MIN} 及以上的监听端口显示会话页转发提示，也不在端口转发对话框的“检测到的端口”中列出，避免临时通信端口干扰；仍可手动输入端口添加转发。
  </p>
</div>
