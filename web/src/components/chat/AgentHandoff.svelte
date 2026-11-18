<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import { untrack } from "svelte";
  import { createHandoffEditor } from "../../hooks/sessions/handoff.svelte";
  import HandoffForm from "./HandoffForm.svelte";
  import { ResponsiveDialog } from "$lib/components/ui/responsive-dialog";
  let { sourceId, onclose }: { sourceId: string; onclose: () => void } =
    $props();
  const editor = createHandoffEditor(
    untrack(() => sourceId),
    false,
  );
  let closing = false;
  function close() {
    if (closing || editor.busy) return;
    closing = true;
    onclose();
  }
</script>

<ResponsiveDialog
  title={m.s3_agent_handoff_title()}
  description={m.s3_agent_handoff_desc()}
  onclose={close}
  busy={editor.busy}
  class="sm:max-w-2xl"
>
  {#if editor.loading}<p role="status" class="text-sm">
      {m.handoff_loading_source()}
    </p>{:else}<HandoffForm {editor} fork={false} />{/if}
  <div class="flex justify-end pt-4">
    <Button variant="outline" disabled={editor.busy} onclick={close}
      >{m.s1_close()}</Button
    >
  </div>
</ResponsiveDialog>
