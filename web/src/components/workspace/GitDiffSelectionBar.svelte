<script lang="ts">
  import type { SelectedLineRange } from "@pierre/diffs";
  import IconMinus from "~icons/tabler/minus";
  import IconPlus from "~icons/tabler/plus";
  import IconTrash from "~icons/tabler/trash";
  import IconX from "~icons/tabler/x";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import type { GitLineSelection } from "../../contracts/schemas";
  import {
    countSelectedLines,
    formatLineRangeLabel,
  } from "../../domains/workspace/git-line-selection";

  let {
    path,
    range = null,
    stageMode = "none",
    lineSelections = [],
    busy = false,
    stale = false,
    onStageSelectedLines,
    onUnstageSelectedLines,
    onDiscardSelectedLines,
    onClear,
  }: {
    path: string;
    range?: SelectedLineRange | null;
    stageMode?: "stage" | "unstage" | "none";
    lineSelections?: readonly GitLineSelection[];
    busy?: boolean;
    stale?: boolean;
    onStageSelectedLines?: (selections: readonly GitLineSelection[]) => void;
    onUnstageSelectedLines?: (selections: readonly GitLineSelection[]) => void;
    onDiscardSelectedLines?: (selections: readonly GitLineSelection[]) => void;
    onClear: () => void;
  } = $props();

  const changedCount = $derived(countSelectedLines(lineSelections));
  const hasLineMutation = $derived(
    changedCount > 0 &&
      ((stageMode === "stage" &&
        Boolean(onStageSelectedLines || onDiscardSelectedLines)) ||
        (stageMode === "unstage" && Boolean(onUnstageSelectedLines))),
  );
  const rangeLabel = $derived(
    range
      ? formatLineRangeLabel(
          Math.min(range.start, range.end),
          Math.max(range.start, range.end),
        )
      : "",
  );
</script>

{#if hasLineMutation}
  <div
    class="flex min-w-0 flex-col gap-1.5 border-b bg-background px-2.5 py-2 text-xs"
    role="region"
    aria-label={m.s6_diff_selection_region_aria()}
    data-testid="git-diff-selection-bar"
  >
    <div class="flex min-w-0 flex-wrap items-center justify-between gap-1.5">
      <div class="flex min-w-0 flex-wrap items-center gap-1.5">
        <span class="min-w-0 truncate font-mono font-medium text-foreground">
          {path}
        </span>
        {#if rangeLabel}
          <Badge variant="outline" class="shrink-0">
            <span class="font-mono">{rangeLabel}</span>
          </Badge>
        {/if}
        <Badge variant="outline" class="shrink-0">
          {m.s6_diff_selection_changed_count({
            count: String(changedCount),
          })}
        </Badge>
      </div>

      <Button
        size="icon-sm"
        variant="ghost"
        aria-label={m.s6_diff_selection_clear_aria()}
        title={m.s6_diff_selection_clear_aria()}
        onclick={onClear}
      >
        <IconX aria-hidden="true" />
      </Button>
    </div>

    <div class="flex min-w-0 flex-wrap items-center gap-1.5">
      {#if stageMode === "stage" && onStageSelectedLines}
        <Button
          size="sm"
          variant="outline"
          wrap
          disabled={busy || stale}
          onclick={() => {
            onStageSelectedLines(lineSelections);
            onClear();
          }}
          data-testid="git-selection-stage-lines"
        >
          <IconPlus aria-hidden="true" />
          {m.s6_stage_selected_lines({ count: String(changedCount) })}
        </Button>
      {/if}
      {#if stageMode === "unstage" && onUnstageSelectedLines}
        <Button
          size="sm"
          variant="outline"
          wrap
          disabled={busy || stale}
          onclick={() => {
            onUnstageSelectedLines(lineSelections);
            onClear();
          }}
          data-testid="git-selection-unstage-lines"
        >
          <IconMinus aria-hidden="true" />
          {m.s6_unstage_selected_lines({ count: String(changedCount) })}
        </Button>
      {/if}
      {#if stageMode === "stage" && onDiscardSelectedLines}
        <Button
          size="sm"
          variant="destructive-outline"
          wrap
          disabled={busy || stale}
          onclick={() => {
            onDiscardSelectedLines(lineSelections);
            onClear();
          }}
          data-testid="git-selection-discard-lines"
        >
          <IconTrash aria-hidden="true" />
          {m.s6_discard_selected_lines({ count: String(changedCount) })}
        </Button>
      {/if}
    </div>
  </div>
{/if}
