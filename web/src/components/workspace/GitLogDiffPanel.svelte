<script lang="ts">
  import type { FileDiffMetadata } from "@pierre/diffs";
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import IconChevronDown from "~icons/tabler/chevron-down";
  import IconChevronUp from "~icons/tabler/chevron-up";
  import IconDownload from "~icons/tabler/download";
  import IconFileText from "~icons/tabler/file-text";
  import IconMaximize from "~icons/tabler/maximize";
  import IconMinimize from "~icons/tabler/minimize";
  import IconRefresh from "~icons/tabler/refresh";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as SegmentedControl from "$lib/components/ui/segmented-control";
  import type { GitCommitFile } from "../../contracts/git-workbench-schemas";
  import { resolveHunksCalculatedDiff } from "../../domains/code/code-diff-client";
  import type { GitLogDiffController } from "../../hooks/workspace/git-log-diff.svelte";
  import CodeDiffReader from "../code/CodeDiffReader.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";

  let {
    logDiff,
    fallbackFile = null,
    fallbackLeftLabel = "",
    fallbackRightLabel = "",
    presentation = "docked",
    collapsed = false,
    diffMode = "unified",
    onDiffModeChange,
    onToggleCollapsed,
    onToggleMaximized,
    onBack,
  }: {
    logDiff?: GitLogDiffController;
    fallbackFile?: GitCommitFile | null;
    fallbackLeftLabel?: string;
    fallbackRightLabel?: string;
    presentation?: "docked" | "maximized" | "mobile";
    collapsed?: boolean;
    diffMode?: "unified" | "split";
    onDiffModeChange?: (mode: "unified" | "split") => void;
    onToggleCollapsed?: () => void;
    onToggleMaximized?: () => void;
    onBack?: () => void;
  } = $props();

  let readerRef = $state<ReturnType<typeof CodeDiffReader>>();
  let offlineState = $state.raw<{
    phase: "idle" | "loading" | "ready" | "binary" | "limited" | "error";
    fileDiff: FileDiffMetadata | null;
    reason: string;
  }>({
    phase: "idle",
    fileDiff: null,
    reason: "",
  });

  export function hasOpenReaderPanel(): boolean {
    return readerRef?.hasOpenPanel() ?? false;
  }

  $effect(() => {
    if (logDiff) return;
    const file = fallbackFile;
    if (!file) {
      offlineState = { phase: "idle", fileDiff: null, reason: "" };
      return;
    }
    if (file.binary) {
      offlineState = {
        phase: "binary",
        fileDiff: null,
        reason: "二进制文件不直接展开文本行级差异。",
      };
      return;
    }
    if (file.hunks.length === 0) {
      offlineState = {
        phase: "limited",
        fileDiff: null,
        reason: "此文件仅包含模式或重命名变更，无文本差异。",
      };
      return;
    }

    const ctrl = new AbortController();
    offlineState = { phase: "loading", fileDiff: null, reason: "" };
    const snapKey = `fixture:${fallbackLeftLabel}:${fallbackRightLabel}:${file.newPath}`;
    void resolveHunksCalculatedDiff(
      snapKey,
      file.newPath,
      file.oldPath,
      file.status,
      file.hunks,
      ctrl.signal,
    ).then((res) => {
      if (ctrl.signal.aborted) return;
      if (res.calculation === "ready" && res.fileDiff) {
        offlineState = {
          phase: "ready",
          fileDiff: res.fileDiff,
          reason: "",
        };
      } else {
        offlineState = {
          phase: res.calculation === "error" ? "error" : "limited",
          fileDiff: null,
          reason: res.calculationReason,
        };
      }
    });
    return () => ctrl.abort();
  });

  const phase = $derived(logDiff ? logDiff.state.phase : offlineState.phase);
  const activeFileDiff = $derived(
    logDiff ? logDiff.state.fileDiff : offlineState.fileDiff,
  );
  const activeReason = $derived(
    logDiff ? logDiff.state.reason : offlineState.reason,
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
</script>

{#snippet downloadSideButton(side: "before" | "after", label: string)}
  {#if logDiff}
    <Button
      size="xs"
      variant="outline"
      disabled={logDiff.state.downloadingSide !== null}
      onclick={() => void logDiff.downloadSnapshotSide(side)}
    >
      <IconDownload aria-hidden="true" />
      {label}
    </Button>
  {/if}
{/snippet}

{#snippet snapshotSideButtons()}
  {#if sideDoc && logDiff}
    <Button
      size="xs"
      variant="outline"
      onclick={() => logDiff.closeSnapshotSide()}
    >
      返回差异对比
    </Button>
  {:else if logDiff && rawDiff && !collapsed}
    {#if rawDiff.old.source !== "empty" && rawDiff.old.kind === "text"}
      <Button
        size="xs"
        variant="ghost"
        onclick={() => void logDiff.openSnapshotSide("before")}
      >
        <IconFileText aria-hidden="true" />
        变更前全文
      </Button>
    {/if}
    {#if rawDiff.new.source !== "empty" && rawDiff.new.kind === "text"}
      <Button
        size="xs"
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
    <Badge variant="secondary" class="shrink-0">{fileStatus}</Badge>
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
  class="flex h-full min-h-0 min-w-0 flex-col bg-git-surface"
  aria-label="历史文件差异视图"
  data-testid="git-log-diff-panel"
  data-diff-presentation={presentation}
>
  {#if presentation === "mobile"}
    <header
      class="flex min-w-0 flex-col gap-1.5 border-b border-git-border bg-git-panel px-2.5 py-1.5"
    >
      <div class="flex min-w-0 flex-wrap items-center justify-between gap-1.5">
        {#if onBack}
          <Button size="xs" variant="ghost" class="shrink-0" onclick={onBack}>
            <IconArrowLeft aria-hidden="true" />
            返回提交详情
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
    <header
      class="flex min-w-0 flex-wrap items-center justify-between gap-2 border-b border-git-border bg-git-panel px-2.5 py-1.5"
    >
      <div
        class="flex min-w-[15rem] flex-1 items-center gap-1.5 overflow-hidden"
      >
        {#if presentation === "maximized" && onBack}
          <Button size="xs" variant="ghost" class="shrink-0" onclick={onBack}>
            <IconArrowLeft aria-hidden="true" />
            返回日志列表
          </Button>
        {/if}
        {@render fileIdentitySummary()}
      </div>

      <div class="flex shrink-0 flex-wrap items-center gap-1.5">
        {@render snapshotSideButtons()}

        {#if !collapsed && phase === "ready"}
          <SegmentedControl.Root
            value={diffMode}
            onValueChange={(val) => {
              if (val === "unified" || val === "split") {
                onDiffModeChange?.(val);
              }
            }}
            aria-label="历史差异布局模式"
          >
            <SegmentedControl.Item value="unified"
              >Unified</SegmentedControl.Item
            >
            <SegmentedControl.Item value="split">Split</SegmentedControl.Item>
          </SegmentedControl.Root>
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
    </header>
  {/if}

  {#if !collapsed}
    <div
      class="code-panel flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden"
    >
      {#if sideDoc}
        <div class="min-h-0 flex-1 overflow-auto p-3">
          {#if sideDoc.loading}
            <p class="text-xs text-muted-foreground" role="status">
              正在读取快照完整文件…
            </p>
          {:else if sideDoc.error}
            <InlineNotice tone="danger">{sideDoc.error}</InlineNotice>
          {:else if sideDoc.file}
            <div
              class="mb-2 flex items-center justify-between gap-2 text-xs text-muted-foreground"
            >
              <span class="font-mono">
                {sideDoc.side === "before" ? "变更前快照" : "变更后快照"} · {sideDoc
                  .file.document.path}
              </span>
              <span>{sideDoc.file.document.lineCount} 行</span>
            </div>
            {#if sideDoc.file.document.kind === "text"}
              <pre
                class="overflow-x-auto rounded-md border border-git-border bg-git-panel p-3 font-mono text-xs leading-relaxed text-foreground">{sideDoc
                  .file.document.text}</pre>
            {:else}
              <InlineNotice tone="info">
                {sideDoc.file.document.reason ||
                  "该快照不是可直接预览的文本文件。"}
              </InlineNotice>
            {/if}
          {/if}
        </div>
      {:else if phase === "idle" || !filePath}
        <div
          class="flex flex-1 items-center justify-center p-4 text-xs text-muted-foreground"
        >
          在右上「变更文件」中单击文件即可在此预览历史差异；双击或按 Enter
          可放大至主工作区。
        </div>
      {:else if phase === "loading"}
        <div
          class="flex flex-1 items-center justify-center p-4 text-xs text-muted-foreground"
          role="status"
        >
          正在加载并计算历史差异快照…
        </div>
      {:else if phase === "ready" && activeFileDiff}
        {#key activeFileDiff.cacheKey ?? `${leftEndpoint}:${rightEndpoint}:${filePath}`}
          <CodeDiffReader
            bind:this={readerRef}
            fileDiff={activeFileDiff}
            side={fileStatus === "D" ? "old" : "new"}
            expired={false}
            draftEnabled={false}
            preferredDiffStyle={effectiveDiffStyle}
            onselect={() => {}}
          />
        {/key}
      {:else if phase === "binary"}
        <div class="space-y-2.5 p-4 text-xs">
          <InlineNotice tone="info">
            {activeReason || "二进制文件不直接展开文本行级差异。"}
          </InlineNotice>
          {#if rawDiff && logDiff}
            <div
              class="flex flex-wrap items-center gap-2 text-muted-foreground"
            >
              {#if rawDiff.old.source !== "empty"}
                <span
                  >变更前：{rawDiff.old.size} B ({rawDiff.old.mediaType ||
                    "binary"})</span
                >
                {#if rawDiff.old.downloadable}
                  {@render downloadSideButton("before", "下载变更前文件")}
                {/if}
              {/if}
              {#if rawDiff.new.source !== "empty"}
                <span
                  >变更后：{rawDiff.new.size} B ({rawDiff.new.mediaType ||
                    "binary"})</span
                >
                {#if rawDiff.new.downloadable}
                  {@render downloadSideButton("after", "下载变更后文件")}
                {/if}
              {/if}
            </div>
          {/if}
        </div>
      {:else if phase === "limited"}
        <div class="space-y-2.5 p-4 text-xs">
          <InlineNotice tone="warning">
            {activeReason || "该文件超过内联差异预算或仅包含元数据变更。"}
          </InlineNotice>
          {#if rawDiff && logDiff}
            <div class="flex flex-wrap items-center gap-2">
              {#if rawDiff.old.source !== "empty" && rawDiff.old.downloadable}
                {@render downloadSideButton("before", "下载变更前原始文件")}
              {/if}
              {#if rawDiff.new.source !== "empty" && rawDiff.new.downloadable}
                {@render downloadSideButton("after", "下载变更后原始文件")}
              {/if}
            </div>
          {/if}
        </div>
      {:else if phase === "error"}
        <div class="space-y-2 p-4 text-xs">
          <InlineNotice tone="danger">
            差异读取失败：{activeReason}
          </InlineNotice>
          {#if logDiff}
            <Button
              size="xs"
              variant="outline"
              onclick={() => logDiff.retryActiveDiff()}
            >
              <IconRefresh aria-hidden="true" />
              重试加载差异
            </Button>
          {/if}
        </div>
      {/if}
    </div>
  {/if}
</section>
