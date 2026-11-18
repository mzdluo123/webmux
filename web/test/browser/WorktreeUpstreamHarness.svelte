<script lang="ts">
  import { onMount } from "svelte";
  import WorktreeUpstreamDialog from "../../src/components/shared/WorktreeUpstreamDialog.svelte";
  import {
    createHandoffEditor,
    type HandoffEditor,
  } from "../../src/hooks/sessions/handoff.svelte";
  let { onready }: { onready: (editor: HandoffEditor) => void } = $props();
  const editor = createHandoffEditor("source", false);
  onMount(() => onready(editor));
</script>

<WorktreeUpstreamDialog />
<output aria-label="handoff state">{editor.operation?.state ?? "idle"}</output>
