<script lang="ts">
  import type { SelectedLineRange } from "@pierre/diffs";
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
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
    backLabel: rawBackLabel,
    presentation = "docked",
    isTaskScope = false,
    liveWorktreeIdentity,
    onAddReworkItem,
    onBack,
  }: {
    logDiff?: GitDiffPanelReader;
    fallbackFile?: GitCommitFile | null;
    fallbackLeftLabel?: string;
    fallbackRightLabel?: string;
    backLabel?: string;
    presentation?: "docked" | "mobile";
    isTaskScope?: boolean;
    liveWorktreeIdentity?: string;
    onAddReworkItem?: (
      input: Parameters<typeof createReworkDraftItem>[0],
      options?: { openSheet?: boolean },
    ) => void;
    onBack?: () => void;
  } = $props();

  const backLabel = $derived(rawBackLabel ?? m.s6_back_to_commit_detail());

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
      binaryReason: m.s6_binary_no_text_diff(),
      emptyReason: m.s6_log_diff_mode_rename_only(),
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
  const effectiveDiffStyle = $derived(
    presentation === "mobile" ? "unified" : "split",
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

{#snippet fileIdentitySummary()}
  {#if leftEndpoint || rightEndpoint}
    <Badge variant="outline" class="shrink-0">
      <span class="font-mono"
        >{leftEndpoint || m.s6_empty_tree()} → {rightEndpoint}</span
      >
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
      {m.s6_log_diff_preview_placeholder()}
    {/if}
  </span>
{/snippet}

<section
  class="flex h-full min-h-0 min-w-0 flex-col bg-background"
  aria-label={m.s6_log_diff_section_aria()}
  data-testid="git-log-diff-panel"
  data-diff-presentation={presentation}
>
  {#if presentation === "mobile"}
    <header
      class="flex min-w-0 flex-col gap-1.5 border-b bg-background px-2.5 py-1.5"
    >
      {#if onBack}
        <div class="flex min-w-0 items-center">
          <Button size="sm" variant="ghost" class="shrink-0" onclick={onBack}>
            <IconArrowLeft aria-hidden="true" />
            {backLabel}
          </Button>
        </div>
      {/if}
      <div class="flex min-w-0 items-center gap-1.5 overflow-hidden">
        {@render fileIdentitySummary()}
      </div>
    </header>
  {:else}
    <GitPanelToolbar as="header">
      <div class="flex min-w-0 flex-1 items-center gap-1.5 overflow-hidden">
        {@render fileIdentitySummary()}
      </div>
    </GitPanelToolbar>
  {/if}

  <GitDiffSelectionBar
    selection={resolvedSelection}
    {onAddReworkItem}
    onClear={clearLogSelection}
  />

  <div class="code-panel flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
    {#if phase === "ready" && activeFileDiff && filePath}
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
</section>
