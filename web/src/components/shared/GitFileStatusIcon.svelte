<script lang="ts">
  import { gitFileStatus } from "./git-file-status";
  let { status, group }: { status: string; group?: string } = $props();
  const presentation = $derived(gitFileStatus({ status, group }));
</script>

<presentation.icon
  class={`size-4 shrink-0 ${presentation.color}`}
  aria-label={presentation.label}
/>
