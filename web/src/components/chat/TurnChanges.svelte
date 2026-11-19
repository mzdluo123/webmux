<script lang="ts">
  import { SvelteMap } from "svelte/reactivity";
  import { ListButton } from "$lib/components/ui/list-button";
  import { m } from "$lib/paraglide/messages.js";
  import type { ChatContent } from "../../contracts/schemas";
  import { diffStats } from "../../domains/chat/chat-model";
  import { holdTranscriptPosition } from "../../hooks/chat/transcript-resize";
  import DiffCount from "./DiffCount.svelte";
  import ChatDisclosure from "./ChatDisclosure.svelte";
  import TurnPatch from "./TurnPatch.svelte";

  let { changes }: { changes: readonly ChatContent[] } = $props();
  const previewId = $props.id();
  let selectedPath = $state<string>();
  const byFile = $derived.by(() => {
    const files = new SvelteMap<string, ChatContent[]>();
    for (const change of changes) {
      const path = change.path!;
      const patches = files.get(path) ?? [];
      patches.push(change);
      files.set(path, patches);
    }
    return files;
  });
  const files = $derived(
    [...byFile].map(([path, patches]) => ({
      path,
      patches,
      hasLineStats: patches.some(
        (patch) => patch.oldText !== undefined || patch.newText !== undefined,
      ),
      stats: patches.reduce(
        (total, patch) => {
          const stats = diffStats(patch);
          return {
            added: total.added + stats.added,
            removed: total.removed + stats.removed,
          };
        },
        { added: 0, removed: 0 },
      ),
    })),
  );
  const selected = $derived(files.find(({ path }) => path === selectedPath));
</script>

<ChatDisclosure>
  {#snippet summary()}{m.s3_turn_changes_summary({
      count: String(byFile.size),
    })}{/snippet}
  <p class="text-xs">{m.s3_turn_changes_desc()}</p>
  <ul>
    {#each files as { path, hasLineStats, stats } (path)}
      <li>
        <ListButton
          variant="row"
          class="w-full min-w-0"
          aria-pressed={selectedPath === path}
          aria-controls={selectedPath === path ? previewId : undefined}
          onclick={(event) => {
            event.currentTarget.dispatchEvent(
              new Event(holdTranscriptPosition, { bubbles: true }),
            );
            selectedPath = selectedPath === path ? undefined : path;
          }}
        >
          <span class="truncate" title={path}>{path}</span>
          {#if hasLineStats}
            <DiffCount added={stats.added} removed={stats.removed} />
          {/if}
        </ListButton>
      </li>
    {/each}
  </ul>
  {#if selected}
    {#key selected.path}
      <div id={previewId} class="min-w-0 space-y-1.5">
        {#each selected.patches as patch, i (i)}
          {#if selected.patches.length > 1}
            <p class="text-xs text-muted-foreground">
              {m.s3_turn_patch_index({
                current: String(i + 1),
                total: String(selected.patches.length),
              })}
            </p>
          {/if}
          <TurnPatch {patch} />
        {/each}
      </div>
    {/key}
  {/if}
</ChatDisclosure>
