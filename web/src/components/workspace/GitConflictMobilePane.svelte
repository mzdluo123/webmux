<script lang="ts">
  import IconCheck from "~icons/tabler/check";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import {
    SegmentedControl,
    SegmentedControlItem,
  } from "$lib/components/ui/segmented-control";
  import { Textarea } from "$lib/components/ui/textarea";
  import { m } from "$lib/paraglide/messages.js";
  import type { GitConflictFileDetail } from "../../contracts/schemas";
  import type {
    GitConflictChunkChoice,
    GitConflictDraftState,
    GitConflictProgressSummary,
  } from "../../domains/workspace/git-conflict-model";

  let {
    detail,
    draft,
    progress,
    onChooseChunk,
    onApplyAllNonConflicting,
    onConfirmManualEdits,
    onChangeResultText,
  }: {
    detail: GitConflictFileDetail;
    draft: GitConflictDraftState;
    progress: GitConflictProgressSummary;
    onChooseChunk: (segmentId: string, choice: GitConflictChunkChoice) => void;
    onApplyAllNonConflicting: () => void;
    onConfirmManualEdits: () => void;
    onChangeResultText: (nextText: string) => void;
  } = $props();

  let sourceSide = $state<"left" | "right" | "base">("left");

  const activeSide = $derived(
    sourceSide === "left"
      ? detail.left
      : sourceSide === "right"
        ? detail.right
        : detail.base,
  );
  const conflictSegs = $derived(
    detail.segments.filter((s) => s.kind === "conflict"),
  );
</script>

<ScrollArea
  class="min-h-8 min-w-0 flex-1"
  data-testid="git-conflict-mobile-pane"
>
  <div class="space-y-3 p-2.5 text-xs">
    <!-- 1. Single-source read-only switcher (Left / Right / Common Ancestor Base) -->
    <div class="space-y-2 rounded-lg border bg-background p-2.5">
      <SegmentedControl
        wrap
        value={sourceSide}
        onValueChange={(val) => {
          if (val === "left" || val === "right" || val === "base") {
            sourceSide = val;
          }
        }}
        aria-label={m.s6_conflict_source_switch_aria()}
      >
        <SegmentedControlItem value="left">
          {m.s6_conflict_left_source()}
        </SegmentedControlItem>
        <SegmentedControlItem value="right">
          {m.s6_conflict_right_source()}
        </SegmentedControlItem>
        <SegmentedControlItem value="base">
          {m.s6_conflict_base_source()}
        </SegmentedControlItem>
      </SegmentedControl>

      <div class="space-y-1">
        <p class="font-semibold wrap-anywhere text-foreground">
          {activeSide.label}
        </p>
        <p class="wrap-anywhere text-muted-foreground">{activeSide.role}</p>
      </div>

      <div class="rounded-md border bg-background">
        <ScrollArea viewportClasses="max-h-36">
          <pre
            class="p-2 font-mono text-xs whitespace-pre-wrap break-all text-foreground">{activeSide.present
              ? activeSide.text || m.s6_conflict_empty_file()
              : m.s6_conflict_missing_on_side()}</pre>
        </ScrollArea>
      </div>
    </div>

    <!-- 2. Non-conflicting quick action + Per-conflict decision cards -->
    {#if progress.pendingNonConflicting > 0}
      <div
        class="flex flex-wrap items-center justify-between gap-2 rounded-lg border bg-background p-2.5"
      >
        <span class="min-w-0 flex-1 wrap-anywhere text-foreground">
          {m.s6_conflict_nonconflicting_detected({
            count: progress.pendingNonConflicting,
          })}
        </span>
        <Button
          size="sm"
          variant="outline"
          wrap
          onclick={onApplyAllNonConflicting}
        >
          {m.s6_conflict_apply_all_nonconflicting()}
        </Button>
      </div>
    {/if}

    {#if conflictSegs.length > 0}
      <div class="space-y-2">
        {#each conflictSegs as seg, idx (seg.id)}
          {@const choice = draft.choices[seg.id] ?? "unresolved"}
          <div
            class="space-y-2 rounded-lg border bg-background p-2.5"
            data-mobile-conflict-card={seg.id}
          >
            <div class="flex flex-wrap items-center justify-between gap-1.5">
              <span class="font-semibold text-foreground">
                {m.s6_conflict_block_index({
                  current: idx + 1,
                  total: conflictSegs.length,
                })}
              </span>
              <Badge
                variant={choice === "unresolved" ? "destructive" : "success"}
              >
                {choice === "unresolved"
                  ? m.s6_conflict_status_unresolved()
                  : m.s6_conflict_status_chosen()}
              </Badge>
            </div>

            <div class="space-y-1.5">
              <div class="rounded-md border bg-background p-2">
                <div class="mb-1 font-medium text-muted-foreground">
                  {m.s6_conflict_left_ref_heading({
                    ref:
                      detail.left.refName ||
                      m.s6_conflict_default_current_branch(),
                  })}
                </div>
                <pre
                  class="font-mono text-xs whitespace-pre-wrap break-all text-foreground">{seg
                    .leftLines.length > 0
                    ? seg.leftLines.join("\n")
                    : m.s6_conflict_deleted_section()}</pre>
              </div>

              <div class="rounded-md border bg-background p-2">
                <div class="mb-1 font-medium text-muted-foreground">
                  {m.s6_conflict_right_ref_heading({
                    ref:
                      detail.right.refName || m.s6_conflict_default_incoming(),
                  })}
                </div>
                <pre
                  class="font-mono text-xs whitespace-pre-wrap break-all text-foreground">{seg
                    .rightLines.length > 0
                    ? seg.rightLines.join("\n")
                    : m.s6_conflict_deleted_section()}</pre>
              </div>
            </div>

            <div class="flex flex-wrap items-center gap-1.5">
              <Button
                size="sm"
                variant="outline"
                wrap
                aria-pressed={choice === "left"}
                onclick={() => onChooseChunk(seg.id, "left")}
              >
                {m.s6_conflict_accept_left()}
              </Button>
              <Button
                size="sm"
                variant="outline"
                wrap
                aria-pressed={choice === "right"}
                onclick={() => onChooseChunk(seg.id, "right")}
              >
                {m.s6_conflict_accept_right()}
              </Button>
              <Button
                size="sm"
                variant="ghost"
                wrap
                aria-pressed={choice === "both_lr"}
                onclick={() => onChooseChunk(seg.id, "both_lr")}
              >
                {m.s6_conflict_both_lr()}
              </Button>
              <Button
                size="sm"
                variant="ghost"
                wrap
                aria-pressed={choice === "both_rl"}
                onclick={() => onChooseChunk(seg.id, "both_rl")}
              >
                {m.s6_conflict_both_rl()}
              </Button>
            </div>
          </div>
        {/each}
      </div>
    {/if}

    <!-- 3. Continuously visible Editable Result area (never hidden behind tabs) -->
    <section
      class="space-y-2 rounded-lg border bg-background p-2.5"
      aria-label={m.s6_conflict_mobile_result_aria()}
      data-testid="git-conflict-mobile-result"
    >
      <div class="flex flex-wrap items-center justify-between gap-1.5">
        <div class="flex flex-wrap items-center gap-1.5">
          <Badge variant="success">{m.s6_conflict_result_badge()}</Badge>
          <span class="font-mono break-all text-foreground">{detail.path}</span>
        </div>
        {#if progress.hasSavedToDisk}
          <Badge variant="success">{m.s6_conflict_saved_to_worktree()}</Badge>
        {:else}
          <Badge variant="warning">{m.s6_conflict_unsaved_draft()}</Badge>
        {/if}
      </div>

      {#if draft.manualEdited && progress.unresolvedConflicts > 0 && !progress.hasConflictMarkers}
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span class="text-muted-foreground">
            {m.s6_conflict_manual_edited_label()}
          </span>
          <Button
            size="sm"
            variant="outline"
            wrap
            onclick={onConfirmManualEdits}
          >
            <IconCheck aria-hidden="true" />
            {m.s6_conflict_confirm_manual_btn()}
          </Button>
        </div>
      {/if}

      <div
        class="rounded-md border bg-background p-1.5 focus-within:ring-2 focus-within:ring-ring/40"
      >
        <Textarea
          variant="bare"
          code
          aria-label={m.s6_conflict_result_editor_aria()}
          value={draft.resultText}
          oninput={(e) => onChangeResultText(e.currentTarget.value)}
          class="min-h-40"
          data-testid="git-conflict-mobile-textarea"
        />
      </div>
    </section>
  </div>
</ScrollArea>
