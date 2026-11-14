<script lang="ts">
  import { untrack } from "svelte";
  import type { SelectedLineRange } from "@pierre/diffs";
  import IconRefresh from "~icons/tabler/refresh";
  import { Button } from "$lib/components/ui/button";
  import type {
    GitWorkingChange,
    GitWorktreeContext,
  } from "../../contracts/git-workbench-schemas";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import type { CodeDiffReaderState } from "../../contracts/schemas";
  import {
    mapDiffRangeToLineSelections,
    type GitHunkAndLineMutationProps,
  } from "../../domains/workspace/git-line-selection";
  import {
    type ActiveDiffReworkSelection,
    buildHunkReworkItemInput,
    createReworkDraftItem,
    extractDiffRangeSnippet,
    resolveStagingVersionDescriptor,
  } from "../../domains/workspace/git-rework-model";
  import { createOfflineHunkDiffState } from "../../hooks/workspace/git-offline-diff.svelte";
  import type { GitStagingDiffController } from "../../hooks/workspace/git-staging-diff.svelte";
  import CodeDiffReader from "../code/CodeDiffReader.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import GitConflictTerminalNotice from "./GitConflictTerminalNotice.svelte";
  import GitDiffSelectionBar from "./GitDiffSelectionBar.svelte";
  import GitStagingDiffHeader from "./GitStagingDiffHeader.svelte";
  import GitStagingHunkStrip from "./GitStagingHunkStrip.svelte";
  import GitStagingLineSelector from "./GitStagingLineSelector.svelte";

  let {
    change,
    context,
    stagingDiff,
    diffMode = "unified",
    readonlyMode = false,
    busy = false,
    stale = false,
    staleReason = "",
    hasActiveWorktreeSession = false,
    defaultLinePickerOpen = false,
    liveWorktreeIdentity,
    onDiffModeChange,
    onStageChange,
    onUnstageChange,
    onStageHunk,
    onUnstageHunk,
    onStageLines,
    onUnstageLines,
    onDiscardFile,
    onDiscardHunk,
    onDiscardLines,
    onAddReworkItem,
    onRefreshSnapshot,
    onOpenConflictResolver,
    onOpenWorktreeTerminal,
    onNewWorktreeTerminal,
  }: {
    change: GitWorkingChange;
    context: GitWorktreeContext;
    stagingDiff?: GitStagingDiffController;
    diffMode?: "unified" | "split";
    readonlyMode?: boolean;
    busy?: boolean;
    stale?: boolean;
    staleReason?: string;
    hasActiveWorktreeSession?: boolean;
    defaultLinePickerOpen?: boolean;
    liveWorktreeIdentity?: string;
    onDiffModeChange?: (mode: "unified" | "split") => void;
    onStageChange: (changeId: string) => void;
    onUnstageChange: (changeId: string) => void;
    onDiscardFile?: (changeId: string, path: string) => void;
    onAddReworkItem?: (
      input: Parameters<typeof createReworkDraftItem>[0],
      options?: { openSheet?: boolean },
    ) => void;
    onRefreshSnapshot?: () => void;
    onOpenConflictResolver?: (path?: string) => void;
    onOpenWorktreeTerminal?: () => void;
    onNewWorktreeTerminal?: () => void;
  } & GitHunkAndLineMutationProps = $props();

  let readerRef = $state<ReturnType<typeof CodeDiffReader>>();
  let linePickerOpen = $state(untrack(() => defaultLinePickerOpen));
  let navState = $state<{
    changeId: string;
    hunkIndex: number;
    jumpSeq: number;
    jumpTarget: CodeDiffReaderState["top"];
  }>({ changeId: "", hunkIndex: 0, jumpSeq: 0, jumpTarget: undefined });
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
  const jumpTarget = $derived(
    navState.changeId === change.id ? navState.jumpTarget : undefined,
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
        reason:
          "该条目存在未解决的合并或变基冲突，不能直接暂存或伪装为可提交文件；请前往工作树终端解决冲突。",
      });
      return;
    }
    return offline.load({
      disabled: false,
      hasTarget: true,
      binary: change.binary,
      binaryReason:
        change.degradeReason ||
        "二进制文件不展开文本行级差异，仅支持整文件暂存或取消暂存。",
      emptyReason:
        change.degradeReason ||
        "当前条目仅包含重命名、模式变更或空内容变化，支持整文件暂存。",
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
  const readerLineSelections = $derived(
    canStageLines
      ? mapDiffRangeToLineSelections(change.hunks, selectedRange)
      : [],
  );

  const resolvedSelection = $derived.by<ActiveDiffReworkSelection | null>(
    () => {
      if (!onAddReworkItem || !fileDiff || !selectedRange) return null;
      const extracted = extractDiffRangeSnippet(fileDiff, selectedRange);
      if (!extracted) return null;
      const desc = resolveStagingVersionDescriptor({
        group: change.group,
        side: extracted.side,
        headOid: context.headOid,
        liveWorktreeIdentity,
      });
      return {
        path: change.path,
        oldPath: change.oldPath,
        ...extracted,
        ...desc,
      };
    },
  );

  function clearStagingSelection() {
    selectionSlot = null;
    readerRef?.clearLineAction();
  }

  function addHunkToRework(hunk: GitWorkingChange["hunks"][number]) {
    if (!onAddReworkItem) return;
    onAddReworkItem(
      buildHunkReworkItemInput({
        change,
        hunk,
        headOid: context.headOid,
        liveWorktreeIdentity,
      }),
    );
  }

  function jumpToHunk(nextIdx: number) {
    if (change.hunks.length === 0) return;
    const clamped = Math.max(0, Math.min(change.hunks.length - 1, nextIdx));
    const hunk = change.hunks[clamped];
    if (!hunk) return;
    const line =
      hunk.newLines > 0
        ? Math.max(1, hunk.newStart)
        : Math.max(1, hunk.oldStart);
    const side = hunk.newLines > 0 ? "additions" : "deletions";
    navState = {
      changeId: change.id,
      hunkIndex: clamped,
      jumpSeq: jumpSeq + 1,
      jumpTarget: { line, side },
    };
  }
</script>

<ScrollArea class="h-full min-h-8 w-full" data-testid="git-staging-diff-panel">
  <div class="flex min-h-full w-full min-w-0 flex-col bg-background">
    <GitStagingDiffHeader
      {change}
      {hunkIndex}
      {phase}
      {diffMode}
      {stageMode}
      {canStageLines}
      {linePickerOpen}
      {canDiscardCurrentFile}
      {busy}
      {stale}
      onJumpToHunk={jumpToHunk}
      onToggleLinePicker={() => (linePickerOpen = !linePickerOpen)}
      {onDiffModeChange}
      {onDiscardFile}
      {onStageChange}
      {onUnstageChange}
    />

    {#if stale}
      <div class="border-b bg-background px-3 py-2">
        <InlineNotice tone="warning">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <span>
              {staleReason ||
                "文件或暂存区已在外部变更，当前阅读快照已过期；暂存按钮已停用，请刷新后再操作。"}
            </span>
            {#if onRefreshSnapshot}
              <Button size="sm" variant="outline" onclick={onRefreshSnapshot}>
                <IconRefresh aria-hidden="true" />
                刷新快照
              </Button>
            {/if}
          </div>
        </InlineNotice>
      </div>
    {/if}

    {#if change.group === "conflicted"}
      <GitConflictTerminalNotice
        message={`存在未解决冲突${
          context.operationInProgress
            ? `（正在进行：${context.operationInProgress}）`
            : ""
        }。冲突文件不可伪装为普通可提交文件，请打开三方冲突解决器或在终端处理。`}
        {hasActiveWorktreeSession}
        newLabel="在此工作树新建终端"
        onOpenConflictResolver={onOpenConflictResolver
          ? () => onOpenConflictResolver(change.path)
          : undefined}
        {onOpenWorktreeTerminal}
        {onNewWorktreeTerminal}
      />
    {:else if !change.hunkStageAvailable && change.degradeReason}
      <div class="border-b bg-background px-3 py-1.5">
        <InlineNotice tone="warning">
          逐块暂存已降级：{change.degradeReason}（仍可使用右上角或文件行的整文件暂存/取消暂存）。
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
        canAddRework={Boolean(onAddReworkItem && !readonlyMode)}
        onJumpToHunk={jumpToHunk}
        onAddHunkToRework={addHunkToRework}
        {onStageHunk}
        {onUnstageHunk}
        onDiscardHunk={change.group === "unstaged" ? onDiscardHunk : undefined}
      />
    {/if}

    {#if !(linePickerOpen && canStageLines)}
      <GitDiffSelectionBar
        selection={resolvedSelection}
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
        {onAddReworkItem}
        onClear={clearStagingSelection}
      />
    {/if}

    {#if linePickerOpen && canStageLines}
      <GitStagingLineSelector
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
    {:else}
      <div
        class="code-panel flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden"
      >
        {#if phase === "loading"}
          <p class="p-4 text-xs text-muted-foreground" role="status">
            正在加载语法高亮差异快照…
          </p>
        {:else if phase === "ready" && fileDiff}
          {#key `${change.id}:${jumpSeq}`}
            <CodeDiffReader
              bind:this={readerRef}
              {fileDiff}
              side="new"
              expired={stale}
              draftEnabled={false}
              preferredDiffStyle={diffMode}
              restore={jumpTarget
                ? {
                    selected: null,
                    expanded: [],
                    search: {
                      search: "",
                      caseSensitive: false,
                      regexp: false,
                      wholeWord: false,
                    },
                    top: jumpTarget,
                  }
                : undefined}
              onselect={(range) => {
                selectionSlot = range ? { changeId: change.id, range } : null;
              }}
            />
          {/key}
        {:else}
          <div class="p-4">
            <InlineNotice tone={phase === "error" ? "danger" : "info"}>
              {reason || "当前条目无文本行级差异预览。"}
            </InlineNotice>
          </div>
        {/if}
      </div>
    {/if}
  </div>
</ScrollArea>
