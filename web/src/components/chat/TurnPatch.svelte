<script lang="ts">
  import type { SelectedLineRange } from "@pierre/diffs";
  import { m } from "$lib/paraglide/messages.js";
  import { calculateDiff } from "../../domains/code/code-diff-client";
  import type { DiffResult } from "../../domains/code/code-diff-model";
  import { errorMessage } from "../../platform/error";
  import type { ChatContent } from "../../contracts/schemas";
  import CodeDiffReader from "../code/CodeDiffReader.svelte";

  let { patch }: { patch: ChatContent } = $props();
  let result = $state.raw<DiffResult>();
  let error = $state("");
  const hasSnapshot = $derived(
    patch.oldText !== undefined || patch.newText !== undefined,
  );
  const defaultSide = $derived<"old" | "new">(
    patch.oldText && !patch.newText ? "old" : "new",
  );
  let pickedSide = $state.raw<{
    patch: ChatContent;
    side: "old" | "new";
  } | null>(null);
  const side = $derived(
    pickedSide?.patch === patch ? pickedSide.side : defaultSide,
  );

  function selectDiff(range: SelectedLineRange | null) {
    if (range?.endSide && range.endSide !== range.side) return;
    pickedSide = range
      ? { patch, side: range.side === "deletions" ? "old" : "new" }
      : null;
  }

  $effect(() => {
    result = undefined;
    error = "";
    if (!hasSnapshot) return;
    const { path = "", oldText = "", newText = "" } = patch;
    const controller = new AbortController();
    void calculateDiff(path, oldText, newText, controller.signal).then(
      (diff) => {
        if (!controller.signal.aborted) result = diff;
      },
      (err: unknown) => {
        if (!controller.signal.aborted) error = errorMessage(err);
      },
    );
    return () => controller.abort();
  });
  function stripFullFilesHint(reason: string): string {
    return reason
      .replace(m.s3_diff_full_files_suffix(), "")
      .replace(
        /(?:\uff0c\u8bf7\u67e5\u770b\u4e24\u4fa7\u5b8c\u6574\u6587\u4ef6|[;,]?\s*(?:please\s+)?view both full files(?:\s+instead)?)\.?$/i,
        "",
      );
  }
</script>

{#if patch.truncated}<p class="text-xs">{m.s3_turn_patch_truncated()}</p>{/if}
{#if !hasSnapshot}
  {#if patch.text}
    <pre
      class="overflow-x-auto rounded-md bg-muted px-3 py-2 font-mono text-xs break-all whitespace-pre-wrap">{patch.text}</pre>
  {:else}
    <p class="text-xs text-muted-foreground" role="status">
      {m.s3_turn_patch_no_snapshot()}
    </p>
  {/if}
{:else if result?.kind === "ready"}
  <div
    class="flex h-64 min-w-0 flex-col"
    role="region"
    aria-label={m.s3_turn_diff_region_label({ path: String(patch.path ?? "") })}
  >
    <CodeDiffReader
      fileDiff={{
        ...result.fileDiff,
        name: patch.path ?? result.fileDiff.name,
      }}
      {side}
      expired={false}
      draftEnabled={false}
      emptyMessage={m.s3_turn_patch_unchanged()}
      onselect={selectDiff}
    />
  </div>
{:else}
  <p class="text-xs" role="status">
    {error ||
      (result?.kind === "limited"
        ? stripFullFilesHint(result.reason)
        : m.s3_turn_patch_computing())}
  </p>
{/if}
