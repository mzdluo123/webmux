<script lang="ts">
  import { Switch } from "$lib/components/ui/switch";
  import { m } from "$lib/paraglide/messages.js";

  let {
    byBranch,
    controls,
    onchange,
  }: {
    byBranch: boolean;
    controls?: string;
    onchange: (checked: boolean) => void;
  } = $props();
</script>

<div
  class="grid min-h-10 grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-center gap-4 px-4 py-1 text-center text-xs"
>
  <span class={byBranch ? "text-muted-foreground" : "font-medium"}>
    {m.s8_sidebar_by_conversation()}
  </span>
  <Switch
    checked={byBranch}
    onCheckedChange={onchange}
    aria-label={m.s8_sidebar_by_branch()}
    aria-controls={controls}
  />
  <span class={byBranch ? "font-medium" : "text-muted-foreground"}>
    {m.s8_sidebar_by_branch()}
  </span>
</div>
