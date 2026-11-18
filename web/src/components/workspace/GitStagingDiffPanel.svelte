<script lang="ts">
  import { tick, untrack } from "svelte";
  import type { SelectedLineRange } from "@pierre/diffs";
  import IconRefresh from "~icons/tabler/refresh";
  import { Button } from "$lib/components/ui/button";
  import type {
    GitWorkingChange,
    GitWorktreeContext,
  } from "../../contracts/git-workbench-schemas";
  import { m } from "$lib/paraglide/messages.js";
  import type { CodeDiffReaderState } from "../../contracts/schemas";
  import {
    mapDiffRangeToLineSelections,
    type GitHunkAndLineMutationProps,
  } from "../../domains/workspace/git-line-selection";
  import { createOfflineHunkDiffState } from "../../hooks/workspace/git-offline-diff.svelte";
  import type { GitStagingDiffController } from "../../hooks/workspace/git-staging-diff.svelte";
  import CodeDiffReader from "../code/CodeDiffReader.svelte";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import GitConflictTerminalNotice from "./GitConflictTerminalNotice.svelte";
  import GitDiffSelectionBar from "./GitDiffSelectionBar.svelte";
  import GitStagingDiffHeader from "./GitStagingDiffHeader.svelte";
  import GitStagingHunkStrip from "./GitStagingHunkStrip.svelte";
  import GitStagingLineSelector from "./GitStagingLineSelector.svelte";

  let {
    change,
    context,
    stagingDiff,
    readonlyMode = false,
    busy = false,
    stale = false,
    staleReason = "",
    hasActiveWorktreeSession = false,
    defaultLinePickerOpen = false,
    onStageChange,
    onUnstageChange,
    onStageHunk,
    onUnstageHunk,
    onStageLines,
    onUnstageLines,
    onDiscardFile,
    onDiscardHunk,
    onDiscardLines,
    onRefreshSnapshot,
    onOpenConflictResolver,
    onOpenWorktreeTerminal,
    onNewWorktreeTerminal,
  }: {
    change: GitWorkingChange;
    context: GitWorktreeContext;
    stagingDiff?: GitStagingDiffController;
    readonlyMode?: boolean;
    busy?: boolean;
    stale?: boolean;
    staleReason?: string;
    hasActiveWorktreeSession?: boolean;
    defaultLinePickerOpen?: boolean;
    onStageChange: (changeId: string) => void;
    onUnstageChange: (changeId: string) => void;
    onDiscardFile?: (changeId: string, path: string) => void;
    onRefreshSnapshot?: () => void;
    onOpenConflictResolver?: (path?: string) => void;
    onOpenWorktreeTerminal?: () => void;
    onNewWorktreeTerminal?: () => void;
  } & GitHunkAndLineMutationProps = $props();

  type LineAnchor = NonNullable<CodeDiffReaderState["top"]>;
  let readerRef = $state<ReturnType<typeof CodeDiffReader>>();
  let lineSelector = $state<ReturnType<typeof GitStagingLineSelector>>();
  let beforePicker:
    | { file: typeof fileDiff; state: CodeDiffReaderState }
    | undefined;
  let linePickerOpen = $state(untrack(() => defaultLinePickerOpen));
  let navState = $state<{
    changeId: string;
    hunkIndex: number;
    jumpSeq: number;
    restore?: CodeDiffReaderState;
  }>({ changeId: "", hunkIndex: 0, jumpSeq: 0 });
  let selectionSlot = $state.raw<{
    changeId: string;
    range: SelectedLineRange | null;
  } | null>(null);
  const hunkIndex = $derived(
    navState.changeId === change.id ? navState.hunkIndex : 0,
  );
  const jumpSeq = $derived(
    navState.changeId === change.id ? navState.jumpSeq : 0,
  );
  const readerRestore = $derived(
    navState.changeId === change.id ? navState.restore : undefined,
  );
  const selectedRange = $derived(
    selectionSlot?.changeId === change.id ? selectionSlot.range : null,
  );
  const offline = createOfflineHunkDiffState();

  $effect(() => {
    if (stagingDiff) return;
    if (change.group === "conflicted") {
      offline.set({
        phase: "limited",
        fileDiff: null,
        reason: m.s6_staging_conflicted_offline_reason(),
      });
      return;
    }
    return offline.load({
      disabled: false,
      hasTarget: true,
      binary: change.binary,
      binaryReason:
        change.degradeReason || m.s6_staging_binary_offline_reason(),
      emptyReason: change.degradeReason || m.s6_staging_empty_offline_reason(),
      snapshotKey: `offline-stage:${change.id}:${change.hunks.length}`,
      path: change.path,
      oldPath: change.oldPath ?? "",
      status: change.status,
      hunks: change.hunks,
    });
  });

  const phase = $derived(
    stagingDiff ? stagingDiff.state.phase : offline.state.phase,
  );
  const fileDiff = $derived(
    stagingDiff ? stagingDiff.state.fileDiff : offline.state.fileDiff,
  );
  const reason = $derived(
    stagingDiff ? stagingDiff.state.reason : offline.state.reason,
  );

  const stageMode = $derived<"stage" | "unstage" | "none">(
    readonlyMode
      ? "none"
      : change.group === "staged"
        ? "unstage"
        : change.group === "unstaged" || change.group === "untracked"
          ? "stage"
          : "none",
  );
  const canStageHunks = $derived(
    (stageMode === "stage" || stageMode === "unstage") &&
      change.group !== "untracked" &&
      change.hunkStageAvailable &&
      change.hunks.length > 0,
  );
  const canStageLines = $derived(
    canStageHunks && Boolean(onStageLines || onUnstageLines),
  );
  const canDiscardCurrentFile = $derived(
    !readonlyMode &&
      (change.group === "unstaged" || change.group === "untracked") &&
      Boolean(onDiscardFile),
  );
  function lineNumbers(side: LineAnchor["side"]): number[] {
    return change.hunks.flatMap((hunk) =>
      hunk.lines.flatMap((line) => {
        const number = side === "deletions" ? line.oldLine : line.newLine;
        return number == null ? [] : [number];
      }),
    );
  }

  // The hunk-only fallback joins the patch lines into compact files.
  function mapReaderAnchor(anchor: LineAnchor | undefined, toReader: boolean) {
    if (!anchor || stagingDiff?.state.rawDiff) return anchor;
    const numbers = lineNumbers(anchor.side);
    const line = toReader
      ? numbers.indexOf(anchor.line) + 1
      : numbers[anchor.line - 1];
    return line ? { line, side: anchor.side } : undefined;
  }

  const mappedSelectedRange = $derived.by<SelectedLineRange | null>(() => {
    if (!selectedRange) return null;
    if (selectedRange.endSide && selectedRange.endSide !== selectedRange.side) {
      return null;
    }
    const side: LineAnchor["side"] =
      selectedRange.side === "deletions" ? "deletions" : "additions";
    const start = mapReaderAnchor(
      { line: Math.min(selectedRange.start, selectedRange.end), side },
      false,
    );
    const end = mapReaderAnchor(
      { line: Math.max(selectedRange.start, selectedRange.end), side },
      false,
    );
    if (!start || !end) return null;
    return { side, start: start.line, end: end.line };
  });

  const readerLineSelections = $derived(
    canStageLines
      ? mapDiffRangeToLineSelections(change.hunks, mappedSelectedRange)
      : [],
  );

  function clearStagingSelection() {
    selectionSlot = null;
    readerRef?.clearLineAction();
  }

  function hunkStart(index: number): LineAnchor | undefined {
    const hunk = change.hunks[index];
    if (!hunk) return;
    return hunk.newLines > 0
      ? { line: Math.max(1, hunk.newStart), side: "additions" }
      : { line: Math.max(1, hunk.oldStart), side: "deletions" };
  }

  function restoreFrom(top: LineAnchor | undefined): CodeDiffReaderState {
    return {
      selected: null,
      expanded: [],
      search: {
        search: "",
        caseSensitive: false,
        regexp: false,
        wholeWord: false,
      },
      top,
    };
  }

  function jumpToHunk(nextIdx: number) {
    const index = Math.max(0, Math.min(change.hunks.length - 1, nextIdx));
    const anchor = hunkStart(index);
    if (!anchor) return;
    lineSelector?.scrollToAnchor(anchor);
    navState = {
      changeId: change.id,
      hunkIndex: index,
      jumpSeq: jumpSeq + 1,
      restore: restoreFrom(mapReaderAnchor(anchor, true)),
    };
  }

  async function toggleLinePicker() {
    if (linePickerOpen) {
      const anchor = lineSelector?.captureAnchor();
      const top = mapReaderAnchor(anchor, true);
      const saved =
        beforePicker?.file === fileDiff ? beforePicker?.state : undefined;
      const index = anchor
        ? change.hunks.findIndex((hunk) => {
            const [start, count] =
              anchor.side === "deletions"
                ? [hunk.oldStart, hunk.oldLines]
                : [hunk.newStart, hunk.newLines];
            return anchor.line >= start && anchor.line < start + count;
          })
        : -1;
      navState = {
        changeId: change.id,
        hunkIndex: index >= 0 ? index : hunkIndex,
        jumpSeq,
        restore: { ...(saved ?? restoreFrom(top)), top: top ?? saved?.top },
      };
      beforePicker = undefined;
      linePickerOpen = false;
      return;
    }

    const state = readerRef?.capture();
    beforePicker = state && { file: fileDiff, state };
    const anchor = mapReaderAnchor(state?.top, false) ?? hunkStart(hunkIndex);
    const changeId = change.id;
    linePickerOpen = true;
    await tick();
    if (linePickerOpen && changeId === change.id && anchor) {
      lineSelector?.scrollToAnchor(anchor);
    }
  }
</script>

<div
  class="flex h-full min-h-0 w-full min-w-0 flex-col overflow-hidden bg-background"
  data-testid="git-staging-diff-panel"
>
  <GitStagingDiffHeader
    {change}
    {hunkIndex}
    {stageMode}
    {canStageLines}
    {linePickerOpen}
    {canDiscardCurrentFile}
    {busy}
    {stale}
    onJumpToHunk={jumpToHunk}
    onToggleLinePicker={() => void toggleLinePicker()}
    {onDiscardFile}
    {onStageChange}
    {onUnstageChange}
  />

  {#if stale}
    <div class="shrink-0 border-b bg-background px-3 py-2">
      <InlineNotice tone="warning">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span>
            {staleReason || m.s6_staging_stale_fallback()}
          </span>
          {#if onRefreshSnapshot}
            <Button size="sm" variant="outline" onclick={onRefreshSnapshot}>
              <IconRefresh aria-hidden="true" />
              {m.s6_staging_refresh_snapshot()}
            </Button>
          {/if}
        </div>
      </InlineNotice>
    </div>
  {/if}

  {#if change.group === "conflicted"}
    <GitConflictTerminalNotice
      message={context.operationInProgress
        ? m.s6_staging_conflict_msg_in_progress({
            op: context.operationInProgress,
          })
        : m.s6_staging_conflict_msg_plain()}
      {hasActiveWorktreeSession}
      newLabel={m.s6_staging_new_terminal_here()}
      onOpenConflictResolver={onOpenConflictResolver
        ? () => onOpenConflictResolver(change.path)
        : undefined}
      {onOpenWorktreeTerminal}
      {onNewWorktreeTerminal}
    />
  {:else if !change.hunkStageAvailable && change.degradeReason}
    <div class="shrink-0 border-b bg-background px-3 py-1.5">
      <InlineNotice tone="warning">
        {m.s6_staging_hunk_degraded({ reason: change.degradeReason })}
      </InlineNotice>
    </div>
  {/if}

  {#if canStageHunks && !(linePickerOpen && canStageLines)}
    <GitStagingHunkStrip
      changeId={change.id}
      hunks={change.hunks}
      {hunkIndex}
      {stageMode}
      {busy}
      {stale}
      onJumpToHunk={jumpToHunk}
      {onStageHunk}
      {onUnstageHunk}
      onDiscardHunk={change.group === "unstaged" ? onDiscardHunk : undefined}
    />
  {/if}

  {#if !(linePickerOpen && canStageLines)}
    <GitDiffSelectionBar
      path={change.path}
      range={mappedSelectedRange}
      {stageMode}
      lineSelections={readerLineSelections}
      {busy}
      {stale}
      onStageSelectedLines={onStageLines
        ? (sel) => onStageLines(change.id, sel)
        : undefined}
      onUnstageSelectedLines={onUnstageLines
        ? (sel) => onUnstageLines(change.id, sel)
        : undefined}
      onDiscardSelectedLines={change.group === "unstaged" && onDiscardLines
        ? (sel) => onDiscardLines(change.id, sel)
        : undefined}
      onClear={clearStagingSelection}
    />
  {/if}

  {#if linePickerOpen && canStageLines}
    {#key change.id}
      <GitStagingLineSelector
        bind:this={lineSelector}
        changeId={change.id}
        hunks={change.hunks}
        {stageMode}
        canDiscardLines={change.group === "unstaged" && Boolean(onDiscardLines)}
        {busy}
        {stale}
        {onStageLines}
        {onUnstageLines}
        {onDiscardLines}
      />
    {/key}
  {:else}
    <div
      class="code-panel flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden"
    >
      {#if phase === "loading"}
        <p class="p-4 text-xs text-muted-foreground" role="status">
          {m.s6_staging_loading_diff()}
        </p>
      {:else if phase === "ready" && fileDiff}
        {#key `${change.id}:${jumpSeq}`}
          <CodeDiffReader
            bind:this={readerRef}
            {fileDiff}
            side="new"
            expired={stale}
            draftEnabled={false}
            preferredDiffStyle="split"
            restore={readerRestore}
            onselect={(range) => {
              selectionSlot = range ? { changeId: change.id, range } : null;
            }}
          />
        {/key}
      {:else}
        <div class="p-4">
          <InlineNotice tone={phase === "error" ? "danger" : "info"}>
            {reason || m.s6_staging_no_text_diff()}
          </InlineNotice>
        </div>
      {/if}
    </div>
  {/if}
</div>
