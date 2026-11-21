<script lang="ts">
  import { useViewport } from "../hooks/shared/viewport.svelte";
  import { untrack } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import { createHandoffEditor } from "../hooks/sessions/handoff.svelte";
  import { navigate } from "../platform/api";
  import HandoffForm from "../components/chat/HandoffForm.svelte";
  import SidebarButton from "../components/shell/SidebarButton.svelte";
  let { sourceId, fork = true }: { sourceId: string; fork?: boolean } =
    $props();
  const viewport = useViewport();
  const editor = createHandoffEditor(
    untrack(() => sourceId),
    untrack(() => fork),
  );
</script>

<div
  class="safe-area-page flex h-(--viewport-height) min-w-0 flex-col overflow-hidden"
  style:--viewport-height={`${viewport.height}px`}
>
  <header
    class="app-header flex shrink-0 items-center gap-2 border-b bg-card px-2"
  >
    <SidebarButton class="sm:hidden" />
    <h1 class="min-w-0 flex-1 truncate text-base font-medium">
      {fork ? m.s1_handoff_to_new_chat() : m.s3_agent_handoff_title()}
    </h1>
    <Button variant="ghost" size="sm" onclick={() => navigate(`/s/${sourceId}`)}
      >{m.handoff_return_source()}</Button
    >
  </header>
  {#if editor.loading}<p class="p-4" role="status">
      {m.handoff_loading_source()}
    </p>{:else}<HandoffForm {editor} {fork} />{/if}
</div>
