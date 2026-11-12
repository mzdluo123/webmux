<script lang="ts">
  import type { SelectedLineRange } from "@pierre/diffs";
  import IconArrowDown from "~icons/tabler/arrow-down";
  import IconArrowUp from "~icons/tabler/arrow-up";
  import IconMinus from "~icons/tabler/minus";
  import IconPlus from "~icons/tabler/plus";
  import IconRefresh from "~icons/tabler/refresh";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import type {
    GitWorkingChange,
    GitWorktreeContext,
  } from "../../contracts/git-workbench-schemas";
  import type { CodeDiffReaderState } from "../../contracts/schemas";
  import {
    type ActiveDiffReworkSelection,
    createReworkDraftItem,
    extractDiffRangeSnippet,
    resolveStagingVersionDescriptor,
  } from "../../domains/workspace/git-rework-model";
  import { createOfflineHunkDiffState } from "../../hooks/workspace/git-offline-diff.svelte";
  import {
    formatStagingComparisonEndpoints,
    type GitStagingDiffController,
  } from "../../hooks/workspace/git-staging-diff.svelte";
  import CodeDiffReader from "../code/CodeDiffReader.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import GitConflictTerminalNotice from "./GitConflictTerminalNotice.svelte";
  import GitDiffSelectionBar from "./GitDiffSelectionBar.svelte";
  import GitDiffStyleSwitch from "./GitDiffStyleSwitch.svelte";
  import GitStagingHunkStrip from "./GitStagingHunkStrip.svelte";

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
    liveWorktreeIdentity,
    onDiffModeChange,
    onStageChange,
    onUnstageChange,
    onStageHunk,
    onUnstageHunk,
    onAddReworkItem,
    onRefreshSnapshot,
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
    liveWorktreeIdentity?: string;
    onDiffModeChange?: (mode: "unified" | "split") => void;
    onStageChange: (changeId: string) => void;
    onUnstageChange: (changeId: string) => void;
    onStageHunk: (changeId: string, hunkId: string) => void;
    onUnstageHunk: (changeId: string, hunkId: string) => void;
    onAddReworkItem?: (
      input: Parameters<typeof createReworkDraftItem>[0],
      options?: { openSheet?: boolean },
    ) => void;
    onRefreshSnapshot?: () => void;
    onOpenWorktreeTerminal?: () => void;
    onNewWorktreeTerminal?: () => void;
  } = $props();

  let readerRef = $state<ReturnType<typeof CodeDiffReader>>();
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

  const endpoints = $derived(formatStagingComparisonEndpoints(change));
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
    const useNewSide = hunk.newLines > 0;
    const startLine = Math.max(1, useNewSide ? hunk.newStart : hunk.oldStart);
    const span = Math.max(1, useNewSide ? hunk.newLines : hunk.oldLines);
    const endLine = startLine + span - 1;
    const desc = resolveStagingVersionDescriptor({
      group: change.group,
      side: useNewSide ? "new" : "old",
      headOid: context.headOid,
      liveWorktreeIdentity,
    });
    const snippet = hunk.lines
      .map(
        (l) =>
          `${l.kind === "add" ? "+" : l.kind === "delete" ? "-" : " "}${l.text}`,
      )
      .join("\n");
    onAddReworkItem({
      path: change.path,
      oldPath: change.oldPath,
      sourceKind: desc.sourceKind,
      versionLabel: desc.versionLabel,
      startLine,
      endLine,
      codeSnippet: snippet,
      capturedWorktreeIdentity: desc.capturedWorktreeIdentity,
    });
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

<div
  class="flex h-full min-h-0 min-w-0 flex-col bg-git-surface"
  data-testid="git-staging-diff-panel"
>
  <header
    class="flex min-w-0 flex-wrap items-center justify-between gap-2 border-b border-git-border bg-git-panel px-2.5 py-1.5"
  >
    <div class="flex min-w-[12rem] flex-1 flex-wrap items-center gap-1.5">
      <Badge
        variant={change.group === "conflicted"
          ? "destructive"
          : change.group === "staged"
            ? "info"
            : "outline"}
      >
        {change.status}
      </Badge>
      <Badge variant="outline">
        <span class="font-mono">{endpoints.left} → {endpoints.right}</span>
      </Badge>
      <span
        class="min-w-0 truncate font-mono text-xs font-medium text-foreground"
        title={change.oldPath && change.oldPath !== change.path
          ? `${change.oldPath} → ${change.path}`
          : change.path}
      >
        {change.oldPath && change.oldPath !== change.path
          ? `${change.oldPath} → ${change.path}`
          : change.path}
      </span>
    </div>

    <div class="flex shrink-0 flex-wrap items-center gap-1.5">
      {#if change.hunks.length > 0}
        <div class="flex items-center gap-1">
          <Button
            size="icon-sm"
            variant="ghost"
            disabled={hunkIndex <= 0}
            aria-label="上一变更块"
            title="上一变更块"
            onclick={() => jumpToHunk(hunkIndex - 1)}
          >
            <IconArrowUp aria-hidden="true" />
          </Button>
          <span class="font-mono text-xs text-muted-foreground">
            {Math.min(hunkIndex + 1, change.hunks.length)}/{change.hunks.length}
          </span>
          <Button
            size="icon-sm"
            variant="ghost"
            disabled={hunkIndex >= change.hunks.length - 1}
            aria-label="下一变更块"
            title="下一变更块"
            onclick={() => jumpToHunk(hunkIndex + 1)}
          >
            <IconArrowDown aria-hidden="true" />
          </Button>
        </div>
      {/if}

      {#if phase === "ready"}
        <GitDiffStyleSwitch value={diffMode} onChange={onDiffModeChange} />
      {/if}

      {#if stageMode === "stage"}
        <Button
          size="xs"
          variant="outline"
          disabled={busy || stale}
          onclick={() => onStageChange(change.id)}
        >
          <IconPlus aria-hidden="true" />
          暂存整文件
        </Button>
      {:else if stageMode === "unstage"}
        <Button
          size="xs"
          variant="outline"
          disabled={busy || stale}
          onclick={() => onUnstageChange(change.id)}
        >
          <IconMinus aria-hidden="true" />
          取消暂存整文件
        </Button>
      {/if}
    </div>
  </header>

  {#if stale}
    <div class="border-b border-git-border bg-git-panel px-3 py-2">
      <InlineNotice tone="warning">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span>
            {staleReason ||
              "文件或暂存区已在外部变更，当前阅读快照已过期；暂存按钮已停用，请刷新后再操作。"}
          </span>
          {#if onRefreshSnapshot}
            <Button size="xs" variant="outline" onclick={onRefreshSnapshot}>
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
      }。冲突文件不可伪装为普通可提交文件，请在工作树终端完成冲突解决。`}
      {hasActiveWorktreeSession}
      newLabel="在此工作树新建终端"
      {onOpenWorktreeTerminal}
      {onNewWorktreeTerminal}
    />
  {:else if !change.hunkStageAvailable && change.degradeReason}
    <div class="border-b border-git-border bg-git-panel px-3 py-1.5">
      <InlineNotice tone="warning">
        逐块暂存已降级：{change.degradeReason}（仍可使用右上角或文件行的整文件暂存/取消暂存）。
      </InlineNotice>
    </div>
  {/if}

  {#if canStageHunks}
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
    />
  {/if}

  <GitDiffSelectionBar
    selection={resolvedSelection}
    {onAddReworkItem}
    onClear={clearStagingSelection}
  />

  <div class="code-panel flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
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
</div>
