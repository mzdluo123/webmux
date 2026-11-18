<script lang="ts">
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import WorktreeUpstreamDialog from "../shared/WorktreeUpstreamDialog.svelte";

  let {
    ready,
    uploadNotice = $bindable(""),
  }: { ready: boolean; uploadNotice?: string } = $props();
</script>

<WorktreeUpstreamDialog />

{#if ready && uploadNotice}
  <div
    class="fixed inset-x-2 top-2 z-50 flex items-start gap-2 rounded-md border bg-card p-3 text-sm shadow-lg"
    role="alert"
  >
    <p class="min-w-0 flex-1 break-all">{uploadNotice}</p>
    <Button variant="outline" size="sm" onclick={() => (uploadNotice = "")}
      >{m.s1_close()}</Button
    >
  </div>
{/if}
