<script lang="ts">
  import type { SelectedLineRange } from "@pierre/diffs";
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import IconChevronDown from "~icons/tabler/chevron-down";
  import IconChevronUp from "~icons/tabler/chevron-up";
  import IconFileText from "~icons/tabler/file-text";
  import IconMaximize from "~icons/tabler/maximize";
  import IconMinimize from "~icons/tabler/minimize";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import GitDiffStyleSwitch from "./GitDiffStyleSwitch.svelte";
  import GitFileStatusIcon from "../shared/GitFileStatusIcon.svelte";
  import type { GitCommitFile } from "../../contracts/git-workbench-schemas";
  import {
    type ActiveDiffReworkSelection,
    createReworkDraftItem,
    extractDiffRangeSnippet,
    resolveSnapshotVersionDescriptor,
  } from "../../domains/workspace/git-rework-model";
  import type { GitDiffPanelReader } from "../../hooks/workspace/git-log-diff.svelte";
  import { createOfflineHunkDiffState } from "../../hooks/workspace/git-offline-diff.svelte";
  import CodeDiffReader from "../code/CodeDiffReader.svelte";
  import GitDiffSelectionBar from "./GitDiffSelectionBar.svelte";
  import GitLogDiffFallbackView from "./GitLogDiffFallbackView.svelte";
  import GitPanelToolbar from "./GitPanelToolbar.svelte";

  let {
    logDiff,
    fallbackFile = null,
    fallbackLeftLabel = "",
    fallbackRightLabel = "",
    backLabel = "返回提交详情",
    presentation = "docked",
    collapsed = false,
    diffMode = "unified",
    isTaskScope = false,
    liveWorktreeIdentity,
    onAddReworkItem,
    onDiffModeChange,
    onToggleCollapsed,
    onToggleMaximized,
    onBack,
  }: {
    logDiff?: GitDiffPanelReader;
    fallbackFile?: GitCommitFile | null;
    fallbackLeftLabel?: string;
    fallbackRightLabel?: string;
    backLabel?: string;
    presentation?: "docked" | "maximized" | "mobile";
    collapsed?: boolean;
    diffMode?: "unified" | "split";
    isTaskScope?: boolean;
    liveWorktreeIdentity?: string;
    onAddReworkItem?: (
      input: Parameters<typeof createReworkDraftItem>[0],
      options?: { openSheet?: boolean },
    ) => void;
    onDiffModeChange?: (mode: "unified" | "split") => void;
    onToggleCollapsed?: () => void;
    onToggleMaximized?: () => void;
    onBack?: () => void;
  } = $props();

  let readerRef = $state<ReturnType<typeof CodeDiffReader>>();
  let selectionSlot = $state.raw<{
    key: string;
    range: SelectedLineRange | null;
  } | null>(null);
  const offline = createOfflineHunkDiffState();

  export function hasOpenReaderPanel(): boolean {
    return readerRef?.hasOpenPanel() ?? false;
  }

  $effect(() => {
    const file = fallbackFile;
    return offline.load({
      disabled: Boolean(logDiff),
      hasTarget: Boolean(file),
      binary: Boolean(file?.binary),
      binaryReason: "二进制文件不直接展开文本行级差异。",
      emptyReason: "此文件仅包含模式或重命名变更，无文本差异。",
      snapshotKey: `fixture:${fallbackLeftLabel}:${fallbackRightLabel}:${file?.newPath ?? ""}`,
      path: file?.newPath ?? "",
      oldPath: file?.oldPath ?? "",
      status: file?.status ?? "M",
      hunks: file?.hunks ?? [],
    });
  });

  const phase = $derived(logDiff ? logDiff.state.phase : offline.state.phase);
  const activeFileDiff = $derived(
    logDiff ? logDiff.state.fileDiff : offline.state.fileDiff,
  );
  const activeReason = $derived(
    logDiff ? logDiff.state.reason : offline.state.reason,
  );
  const leftEndpoint = $derived(
    logDiff ? logDiff.state.leftLabel : fallbackLeftLabel,
  );
  const rightEndpoint = $derived(
    logDiff ? logDiff.state.rightLabel : fallbackRightLabel,
  );
  const filePath = $derived(
    logDiff
      ? logDiff.state.path
      : (fallbackFile?.newPath ?? fallbackFile?.oldPath ?? ""),
  );
  const oldFilePath = $derived(
    logDiff ? logDiff.state.oldPath : (fallbackFile?.oldPath ?? ""),
  );
  const fileStatus = $derived(
    logDiff ? logDiff.state.status : (fallbackFile?.status ?? ""),
  );
  const rawDiff = $derived(logDiff?.state.rawDiff ?? null);
  const sideDoc = $derived(logDiff?.state.snapshotSideDoc ?? null);
  const effectiveDiffStyle = $derived(
    presentation === "mobile" ? "unified" : diffMode,
  );

  const selectionKey = $derived(`${leftEndpoint}:${rightEndpoint}:${filePath}`);
  const selectedRange = $derived(
    selectionSlot?.key === selectionKey ? selectionSlot.range : null,
  );

  const resolvedSelection = $derived.by<ActiveDiffReworkSelection | null>(
    () => {
      if (!onAddReworkItem || !activeFileDiff || !selectedRange || !filePath) {
        return null;
      }
      const extracted = extractDiffRangeSnippet(activeFileDiff, selectedRange);
      if (!extracted) return null;
      const desc = resolveSnapshotVersionDescriptor({
        isTaskScope,
        side: extracted.side,
        leftLabel: leftEndpoint,
        rightLabel: rightEndpoint,
        baseOid: logDiff?.state.baseOid ?? "",
        targetOid: logDiff?.state.targetOid ?? "",
        liveWorktreeIdentity,
      });
      return {
        path: filePath,
        oldPath: oldFilePath || undefined,
        ...extracted,
        ...desc,
      };
    },
  );

  function clearLogSelection() {
    selectionSlot = null;
    readerRef?.clearLineAction();
  }
</script>

{#snippet snapshotSideButtons()}
  {#if sideDoc && logDiff}
    <Button
      size="sm"
      variant="outline"
      onclick={() => logDiff.closeSnapshotSide()}
    >
      返回差异对比
    </Button>
  {:else if logDiff && rawDiff && !collapsed}
    {#if rawDiff.old.source !== "empty" && rawDiff.old.kind === "text"}
      <Button
        size="sm"
        variant="ghost"
        onclick={() => void logDiff.openSnapshotSide("before")}
      >
        <IconFileText aria-hidden="true" />
        变更前全文
      </Button>
    {/if}
    {#if rawDiff.new.source !== "empty" && rawDiff.new.kind === "text"}
      <Button
        size="sm"
        variant="ghost"
        onclick={() => void logDiff.openSnapshotSide("after")}
      >
        <IconFileText aria-hidden="true" />
        变更后全文
      </Button>
    {/if}
  {/if}
{/snippet}

{#snippet fileIdentitySummary()}
  {#if leftEndpoint || rightEndpoint}
    <Badge variant="outline" class="shrink-0">
      <span class="font-mono">{leftEndpoint || "空树"} → {rightEndpoint}</span>
    </Badge>
  {/if}
  {#if fileStatus}
    <GitFileStatusIcon status={fileStatus} />
  {/if}
  <span
    class="min-w-0 truncate font-mono text-xs font-medium text-foreground"
    title={oldFilePath && oldFilePath !== filePath
      ? `${oldFilePath} → ${filePath}`
      : filePath}
  >
    {#if filePath}
      {oldFilePath && oldFilePath !== filePath
        ? `${oldFilePath} → ${filePath}`
        : filePath}
    {:else}
      文件差异预览
    {/if}
  </span>
{/snippet}

<section
  class="flex h-full min-h-0 min-w-0 flex-col bg-background"
  aria-label="历史文件差异视图"
  data-testid="git-log-diff-panel"
  data-diff-presentation={presentation}
>
  {#if presentation === "mobile"}
    <header
      class="flex min-w-0 flex-col gap-1.5 border-b bg-background px-2.5 py-1.5"
    >
      <div class="flex min-w-0 flex-wrap items-center justify-between gap-1.5">
        {#if onBack}
          <Button size="sm" variant="ghost" class="shrink-0" onclick={onBack}>
            <IconArrowLeft aria-hidden="true" />
            {backLabel}
          </Button>
        {/if}
        <div class="flex shrink-0 flex-wrap items-center gap-1">
          {@render snapshotSideButtons()}
        </div>
      </div>
      <div class="flex min-w-0 items-center gap-1.5 overflow-hidden">
        {@render fileIdentitySummary()}
      </div>
    </header>
  {:else}
    <GitPanelToolbar as="header">
      <div
        class="flex min-w-[15rem] flex-1 items-center gap-1.5 overflow-hidden"
      >
        {#if presentation === "maximized" && onBack}
          <Button size="sm" variant="ghost" class="shrink-0" onclick={onBack}>
            <IconArrowLeft aria-hidden="true" />
            返回日志列表
          </Button>
        {/if}
        {@render fileIdentitySummary()}
      </div>

      <div class="flex shrink-0 flex-wrap items-center gap-1.5">
        {@render snapshotSideButtons()}

        {#if !collapsed && phase === "ready"}
          <GitDiffStyleSwitch
            value={diffMode}
            ariaLabel="历史差异布局模式"
            onChange={(mode) => onDiffModeChange?.(mode)}
          />
        {/if}

        {#if onToggleMaximized && filePath}
          <Button
            size="icon-sm"
            variant="ghost"
            aria-pressed={presentation === "maximized"}
            aria-label={presentation === "maximized"
              ? "退出放大差异"
              : "放大差异到主工作区"}
            title={presentation === "maximized"
              ? "退出放大差异 (Esc)"
              : "放大差异到主工作区 (Enter / 双击文件)"}
            onclick={onToggleMaximized}
          >
            {#if presentation === "maximized"}
              <IconMinimize aria-hidden="true" />
            {:else}
              <IconMaximize aria-hidden="true" />
            {/if}
          </Button>
        {/if}

        {#if presentation === "docked" && onToggleCollapsed}
          <Button
            size="icon-sm"
            variant="ghost"
            aria-expanded={!collapsed}
            aria-label={collapsed ? "展开底部差异面板" : "折叠底部差异面板"}
            title={collapsed ? "展开底部差异面板" : "折叠底部差异面板"}
            onclick={onToggleCollapsed}
          >
            {#if collapsed}
              <IconChevronUp aria-hidden="true" />
            {:else}
              <IconChevronDown aria-hidden="true" />
            {/if}
          </Button>
        {/if}
      </div>
    </GitPanelToolbar>
  {/if}

  {#if !collapsed}
    <GitDiffSelectionBar
      selection={resolvedSelection}
      {onAddReworkItem}
      onClear={clearLogSelection}
    />

    <div
      class="code-panel flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden"
    >
      {#if !sideDoc && phase === "ready" && activeFileDiff && filePath}
        {#key activeFileDiff.cacheKey ?? `${leftEndpoint}:${rightEndpoint}:${filePath}`}
          <CodeDiffReader
            bind:this={readerRef}
            fileDiff={activeFileDiff}
            side={fileStatus === "D" ? "old" : "new"}
            expired={false}
            draftEnabled={false}
            preferredDiffStyle={effectiveDiffStyle}
            onselect={(range) => {
              selectionSlot = range ? { key: selectionKey, range } : null;
            }}
          />
        {/key}
      {:else}
        <GitLogDiffFallbackView
          phase={!filePath && phase !== "loading"
            ? "idle"
            : phase === "ready"
              ? "idle"
              : phase}
          reason={activeReason}
          {logDiff}
        />
      {/if}
    </div>
  {/if}
</section>
