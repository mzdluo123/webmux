<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";

  // A picker row or trigger: a title over a monospace command or path, and
  // an optional environment variable count.
  let {
    primary,
    secondary = "",
    envCount = 0,
  }: { primary: string; secondary?: string; envCount?: number } = $props();
</script>

<span class="grid min-w-0 flex-1 gap-0.5 text-left">
  <span class="truncate text-sm font-medium" title={primary}>{primary}</span>
  {#if secondary}<span
      class="truncate font-mono text-xs text-muted-foreground"
      title={secondary}>{secondary}</span
    >{/if}
  {#if envCount}<span class="text-xs text-muted-foreground"
      >{m.s1_env_var_count({ count: envCount })}</span
    >{/if}
</span>
