<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
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
      >{m.s2_forward_ignore_high({ port: String(HIGH_PORT_MIN) })}</Label
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
    {m.s2_forward_ignore_high_help({ port: String(HIGH_PORT_MIN) })}
  </p>
</div>
