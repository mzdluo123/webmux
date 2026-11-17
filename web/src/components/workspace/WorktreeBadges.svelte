<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import type { Worktree } from "../../contracts/schemas";
  import { Badge } from "$lib/components/ui/badge";
  import { syncNotable } from "../../domains/workspace/worktree-sync";
  import WorktreeSyncSummary from "./WorktreeSyncSummary.svelte";

  // Undefined `pinned` means the list is not curated (no pin badge).
  let { worktree, pinned }: { worktree: Worktree; pinned?: boolean } = $props();
  const shown = $derived(
    worktree.kind === "main" ||
      pinned === false ||
      worktree.kind === "bare" ||
      worktree.locked ||
      worktree.prunable ||
      !worktree.available ||
      worktree.dirty !== false ||
      worktree.hasUntracked ||
      syncNotable(worktree),
  );
</script>

{#if shown}
  <div class="flex max-w-full flex-wrap items-center gap-1.5">
    {#if worktree.kind === "main"}<Badge variant="outline"
        >{m.s8_badge_main_worktree()}</Badge
      >{/if}
    {#if pinned === false}<Badge variant="outline"
        >{m.s8_badge_not_on_home()}</Badge
      >{/if}
    {#if worktree.kind === "bare"}<Badge variant="secondary"
        >{m.s8_badge_bare_no_start()}</Badge
      >{/if}
    {#if worktree.locked}<Badge variant="outline">{m.s8_badge_locked()}</Badge
      >{/if}
    {#if worktree.prunable}<Badge variant="outline"
        >{m.s8_badge_prunable()}</Badge
      >{/if}
    {#if !worktree.available}<Badge variant="destructive"
        >{m.s8_badge_unavailable()}</Badge
      >
    {:else if worktree.dirty === null}<Badge variant="outline"
        >{m.s8_badge_status_unknown()}</Badge
      >
    {:else if worktree.dirty}<Badge variant="warning"
        >{m.s8_badge_modified()}</Badge
      >
    {/if}
    {#if worktree.hasUntracked}<Badge variant="outline"
        >{m.s8_batch_untracked_files()}</Badge
      >{/if}
    <WorktreeSyncSummary {worktree} variant="indicator" />
  </div>
{/if}
