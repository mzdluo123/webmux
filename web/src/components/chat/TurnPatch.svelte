<script lang="ts">
  import type { SelectedLineRange } from "@pierre/diffs";
  import { untrack } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import { calculateDiff } from "../../domains/code/code-diff-client";
  import type { DiffResult } from "../../domains/code/code-diff-model";
  import { errorMessage } from "../../platform/error";
  import type {
    ChatContent,
    CodeDiffReaderState,
  } from "../../contracts/schemas";
  import CodeDiffReader from "../code/CodeDiffReader.svelte";

  let { patch }: { patch: ChatContent } = $props();
  let result = $state.raw<DiffResult>();
  let error = $state("");
  let reader = $state.raw<ReturnType<typeof CodeDiffReader>>();
  let restore = $state.raw<CodeDiffReaderState>();
  let previousPath: string | undefined;
  const path = $derived(patch.path ?? "");
  const oldText = $derived(patch.oldText);
  const newText = $derived(patch.newText);
  const hasSnapshot = $derived(oldText !== undefined || newText !== undefined);
  const defaultSide = $derived<"old" | "new">(
    oldText && !newText ? "old" : "new",
  );
  let pickedSide = $state<"old" | "new">();
  const side = $derived(pickedSide ?? defaultSide);

  function selectDiff(range: SelectedLineRange | null) {
    if (range?.endSide && range.endSide !== range.side) return;
    pickedSide = range
      ? range.side === "deletions"
        ? "old"
        : "new"
      : undefined;
  }

  $effect.pre(() => {
    if (previousPath === path) {
      const captured = untrack(() => reader?.capture());
      if (captured) restore = captured;
    } else restore = undefined;
    previousPath = path;
    pickedSide = undefined;
    result = undefined;
    error = "";
    if (!hasSnapshot) return;
    const controller = new AbortController();
    void calculateDiff(
      path,
      oldText ?? "",
      newText ?? "",
      controller.signal,
    ).then(
      (diff) => {
        if (controller.signal.aborted) return;
        if (diff.kind === "ready" && restore?.selected) {
          const { selected } = restore;
          const lineCount = (side: SelectedLineRange["side"]) =>
            side === "deletions"
              ? diff.fileDiff.deletionLines.length
              : diff.fileDiff.additionLines.length;
          if (
            selected.start > lineCount(selected.side) ||
            selected.end > lineCount(selected.endSide ?? selected.side)
          )
            restore = { ...restore, selected: null };
        }
        result = diff;
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

<div class="flex h-64 min-w-0 flex-col overflow-hidden">
  {#if patch.truncated}
    <p class="shrink-0 text-xs">{m.s3_turn_patch_truncated()}</p>
  {/if}
  {#if hasSnapshot && result?.kind === "ready"}
    <div
      class="flex min-h-0 min-w-0 flex-1 flex-col"
      role="region"
      aria-label={m.s3_turn_diff_region_label({ path })}
    >
      <CodeDiffReader
        bind:this={reader}
        fileDiff={{ ...result.fileDiff, name: path || result.fileDiff.name }}
        {side}
        {restore}
        expired={false}
        draftEnabled={false}
        emptyMessage={m.s3_turn_patch_unchanged()}
        onselect={selectDiff}
      />
    </div>
  {:else if !hasSnapshot && patch.text}
    <pre
      class="min-h-0 overflow-auto rounded-md bg-muted px-3 py-2 font-mono text-xs break-all whitespace-pre-wrap">{patch.text}</pre>
  {:else}
    <p
      class="min-h-0 overflow-y-auto text-xs text-muted-foreground"
      role="status"
    >
      {!hasSnapshot
        ? m.s3_turn_patch_no_snapshot()
        : error ||
          (result?.kind === "limited"
            ? stripFullFilesHint(result.reason)
            : m.s3_turn_patch_computing())}
    </p>
  {/if}
</div>
