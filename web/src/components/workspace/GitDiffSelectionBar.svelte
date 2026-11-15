<script lang="ts">
  import IconMessagePlus from "~icons/tabler/message-plus";
  import IconMinus from "~icons/tabler/minus";
  import IconPlus from "~icons/tabler/plus";
  import IconTrash from "~icons/tabler/trash";
  import IconX from "~icons/tabler/x";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import * as m from "$lib/paraglide/messages.js";
  import type { GitLineSelection } from "../../contracts/schemas";
  import { countSelectedLines } from "../../domains/workspace/git-line-selection";
  import {
    type ActiveDiffReworkSelection,
    createReworkDraftItem,
    formatLineRangeLabel,
    toReworkItemInput,
  } from "../../domains/workspace/git-rework-model";

  let {
    selection,
    stageMode = "none",
    lineSelections = [],
    busy = false,
    stale = false,
    onStageSelectedLines,
    onUnstageSelectedLines,
    onDiscardSelectedLines,
    onAddReworkItem,
    onClear,
  }: {
    selection: ActiveDiffReworkSelection | null;
    stageMode?: "stage" | "unstage" | "none";
    lineSelections?: readonly GitLineSelection[];
    busy?: boolean;
    stale?: boolean;
    onStageSelectedLines?: (selections: readonly GitLineSelection[]) => void;
    onUnstageSelectedLines?: (selections: readonly GitLineSelection[]) => void;
    onDiscardSelectedLines?: (selections: readonly GitLineSelection[]) => void;
    onAddReworkItem?: (
      input: Parameters<typeof createReworkDraftItem>[0],
      options?: { openSheet?: boolean },
    ) => void;
    onClear: () => void;
  } = $props();

  let note = $state("");
  const changedCount = $derived(countSelectedLines(lineSelections));
  const hasLineMutation = $derived(
    changedCount > 0 &&
      ((stageMode === "stage" &&
        Boolean(onStageSelectedLines || onDiscardSelectedLines)) ||
        (stageMode === "unstage" && Boolean(onUnstageSelectedLines))),
  );

  function submitAdd(openSheet: boolean) {
    if (!selection || !onAddReworkItem) return;
    const clean = note.trim();
    note = "";
    onAddReworkItem(toReworkItemInput(selection, clean), { openSheet });
    onClear();
  }
</script>

{#if (selection && onAddReworkItem) || hasLineMutation}
  <div
    class="flex min-w-0 flex-col gap-1.5 border-b bg-background px-2.5 py-2 text-xs"
    role="region"
    aria-label={m.s6_diff_selection_region_aria()}
    data-testid="git-diff-selection-bar"
  >
    <div class="flex min-w-0 flex-wrap items-center justify-between gap-1.5">
      <div class="flex min-w-0 flex-wrap items-center gap-1.5">
        {#if selection}
          <Badge variant="info" class="shrink-0">
            {selection.versionLabel}
          </Badge>
          <span class="min-w-0 truncate font-mono font-medium text-foreground">
            {selection.path}
          </span>
          <Badge variant="outline" class="shrink-0">
            <span class="font-mono">
              {formatLineRangeLabel(selection.startLine, selection.endLine)}
            </span>
          </Badge>
          {#if selection.sourceKind !== "worktree"}
            <span class="text-muted-foreground">
              {m.s6_diff_selection_non_worktree_hint()}
            </span>
          {/if}
        {/if}
        {#if hasLineMutation}
          <Badge variant="outline" class="shrink-0">
            {m.s6_diff_selection_changed_count({
              count: String(changedCount),
            })}
          </Badge>
        {/if}
      </div>

      <Button
        size="icon-sm"
        variant="ghost"
        aria-label={m.s6_diff_selection_clear_aria()}
        title={m.s6_diff_selection_clear_aria()}
        onclick={() => {
          note = "";
          onClear();
        }}
      >
        <IconX aria-hidden="true" />
      </Button>
    </div>

    {#if hasLineMutation}
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
    {/if}

    {#if selection && onAddReworkItem}
      <div class="flex min-w-0 flex-wrap items-center gap-1.5">
        <div class="min-w-[10rem] flex-1">
          <Input
            size="sm"
            value={note}
            placeholder={m.s6_diff_selection_note_placeholder()}
            aria-label={m.s6_diff_selection_note_aria()}
            oninput={(e) => (note = e.currentTarget.value)}
            onkeydown={(e) => {
              if (e.key === "Enter" && !e.isComposing) {
                e.preventDefault();
                submitAdd(false);
              }
            }}
          />
        </div>
        <Button
          size="sm"
          variant="outline"
          onclick={() => submitAdd(true)}
          data-testid="git-selection-add-open"
        >
          {m.s6_diff_selection_add_and_open()}
        </Button>
        <Button
          size="sm"
          variant="default"
          onclick={() => submitAdd(false)}
          data-testid="git-selection-add-rework"
        >
          <IconMessagePlus aria-hidden="true" />
          {m.s6_diff_selection_add_rework()}
        </Button>
      </div>
    {/if}
  </div>
{/if}
