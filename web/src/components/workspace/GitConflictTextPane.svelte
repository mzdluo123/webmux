<script lang="ts">
  import IconCheck from "~icons/tabler/check";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import { Textarea } from "$lib/components/ui/textarea";
  import * as m from "$lib/paraglide/messages.js";
  import type { GitConflictFileDetail } from "../../contracts/schemas";
  import type {
    GitBaseCompareMode,
    GitConflictChunkChoice,
    GitConflictDraftState,
    GitConflictProgressSummary,
  } from "../../domains/workspace/git-conflict-model";
  import GitConflictSourceColumn from "./GitConflictSourceColumn.svelte";

  let {
    detail,
    draft,
    progress,
    baseCompareMode = "off",
    activeConflictSegmentId = null,
    onChooseChunk,
    onToggleNonConflicting,
    onConfirmManualEdits,
    onChangeResultText,
  }: {
    detail: GitConflictFileDetail;
    draft: GitConflictDraftState;
    progress: GitConflictProgressSummary;
    baseCompareMode?: GitBaseCompareMode;
    activeConflictSegmentId?: string | null;
    onChooseChunk: (segmentId: string, choice: GitConflictChunkChoice) => void;
    onToggleNonConflicting: (segmentId: string) => void;
    onConfirmManualEdits: () => void;
    onChangeResultText: (nextText: string) => void;
  } = $props();
</script>

<div
  class="grid h-full min-h-0 min-w-0 grid-cols-3 divide-x divide-border overflow-hidden"
  data-testid="git-conflict-desktop-grid"
>
  <GitConflictSourceColumn
    side="left"
    info={detail.left}
    segments={detail.segments}
    {draft}
    showBase={baseCompareMode === "left_base"}
    {activeConflictSegmentId}
    {onChooseChunk}
    {onToggleNonConflicting}
  />

  <!-- Column 2: Center Editable Result (NEVER replaced by Base) -->
  <section
    class="flex h-full min-h-0 min-w-0 flex-col bg-background"
    aria-label={m.s6_conflict_center_result_aria()}
    data-testid="git-conflict-center-result"
  >
    <div class="space-y-1 border-b bg-background px-2.5 py-2 text-xs">
      <div class="flex min-w-0 flex-wrap items-center justify-between gap-1.5">
        <div class="flex min-w-0 items-center gap-1.5">
          <Badge variant="success">{m.s6_conflict_result_badge()}</Badge>
          <span class="min-w-0 truncate font-mono" title={detail.path}>
            {detail.path}
          </span>
        </div>

        <div class="flex flex-wrap items-center gap-1">
          {#if progress.hasConflictMarkers}
            <Badge variant="destructive">
              {m.s6_conflict_has_markers_badge()}
            </Badge>
          {:else if progress.hasSavedToDisk}
            <Badge variant="success">{m.s6_conflict_saved_to_worktree()}</Badge>
          {:else}
            <Badge variant="warning">{m.s6_conflict_unsaved_draft()}</Badge>
          {/if}
          <Badge
            variant={progress.unresolvedConflicts === 0
              ? "secondary"
              : "outline"}
          >
            {m.s6_conflict_resolved_progress({
              resolved: String(
                progress.totalConflicts - progress.unresolvedConflicts,
              ),
              total: String(progress.totalConflicts),
            })}
          </Badge>
        </div>
      </div>
      <p class="text-xs text-muted-foreground">
        {m.s6_conflict_editor_help()}
      </p>
    </div>

    {#if draft.manualEdited && progress.unresolvedConflicts > 0 && !progress.hasConflictMarkers}
      <div
        class="flex flex-wrap items-center justify-between gap-2 border-b bg-background px-2.5 py-1.5 text-xs"
      >
        <span class="text-muted-foreground">
          {m.s6_conflict_manual_edit_detected({
            count: String(progress.unresolvedConflicts),
          })}
        </span>
        <Button
          size="sm"
          variant="outline"
          onclick={onConfirmManualEdits}
          data-testid="git-conflict-confirm-manual-btn"
        >
          <IconCheck aria-hidden="true" />
          {m.s6_conflict_confirm_manual_btn()}
        </Button>
      </div>
    {/if}

    <div
      class="flex min-h-0 flex-1 flex-col focus-within:ring-2 focus-within:ring-ring/40"
    >
      <ScrollArea class="min-h-8 flex-1">
        <div class="flex min-h-full flex-col p-2">
          <Textarea
            variant="bare"
            code
            aria-label={m.s6_conflict_result_editor_aria()}
            value={draft.resultText}
            oninput={(e) => onChangeResultText(e.currentTarget.value)}
            class="min-h-full flex-1"
            data-testid="git-conflict-result-textarea"
          />
        </div>
      </ScrollArea>
    </div>
  </section>

  <GitConflictSourceColumn
    side="right"
    info={detail.right}
    segments={detail.segments}
    {draft}
    showBase={baseCompareMode === "right_base"}
    {activeConflictSegmentId}
    {onChooseChunk}
    {onToggleNonConflicting}
  />
</div>
