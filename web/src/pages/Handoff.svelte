<script lang="ts">
  import { untrack } from "svelte";
  import { Button } from "$lib/components/ui/button";
  import * as m from "$lib/paraglide/messages.js";
  import { createHandoffEditor } from "../hooks/sessions/handoff.svelte";
  import { navigate } from "../platform/api";
  import HandoffForm from "../components/chat/HandoffForm.svelte";
  import SidebarButton from "../components/shell/SidebarButton.svelte";
  let { sourceId }: { sourceId: string } = $props();
  const editor = createHandoffEditor(
    untrack(() => sourceId),
    true,
  );
</script>

<div class="flex h-full min-h-0 min-w-0 flex-col">
  <header
    class="app-header flex shrink-0 items-center gap-2 border-b bg-card px-2"
  >
    <SidebarButton class="sm:hidden" />
    <h1 class="min-w-0 flex-1 truncate text-base font-medium">
      {m.s1_handoff_to_new_chat()}
    </h1>
    <Button variant="ghost" size="sm" onclick={() => navigate("/new")}
      >{m.s1_back_to_new_chat()}</Button
    >
  </header>
  <main class="min-h-0 flex-1 overflow-y-auto p-4 sm:p-6">
    <div class="mx-auto max-w-2xl space-y-4">
      <p class="text-sm text-muted-foreground">
        {m.s1_handoff_description()}
      </p>
      {#if editor.loading}<p role="status" class="text-sm">
          {m.s1_restoring_handoff_draft()}
        </p>{:else}<HandoffForm {editor} fork />{/if}
    </div>
  </main>
</div>
