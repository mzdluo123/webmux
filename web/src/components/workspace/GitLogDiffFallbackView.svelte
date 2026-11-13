<script lang="ts">
  import IconDownload from "~icons/tabler/download";
  import IconRefresh from "~icons/tabler/refresh";
  import { Button } from "$lib/components/ui/button";
  import type { GitDiffPanelReader } from "../../hooks/workspace/git-log-diff.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";

  let {
    phase,
    reason,
    logDiff,
  }: {
    phase: "idle" | "loading" | "binary" | "limited" | "error";
    reason: string;
    logDiff?: GitDiffPanelReader;
  } = $props();

  const rawDiff = $derived(logDiff?.state.rawDiff ?? null);
  const sideDoc = $derived(logDiff?.state.snapshotSideDoc ?? null);
</script>

{#snippet downloadSideButton(side: "before" | "after", label: string)}
  {#if logDiff}
    <Button
      size="sm"
      variant="outline"
      disabled={logDiff.state.downloadingSide !== null}
      onclick={() => void logDiff.downloadSnapshotSide(side)}
    >
      <IconDownload aria-hidden="true" />
      {label}
    </Button>
  {/if}
{/snippet}

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
          class="overflow-x-auto rounded-md border bg-background p-3 font-mono text-xs leading-relaxed text-foreground">{sideDoc
            .file.document.text}</pre>
      {:else}
        <InlineNotice tone="info">
          {sideDoc.file.document.reason || "该快照不是可直接预览的文本文件。"}
        </InlineNotice>
      {/if}
    {/if}
  </div>
{:else if phase === "idle"}
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
{:else if phase === "binary"}
  <div class="space-y-2.5 p-4 text-xs">
    <InlineNotice tone="info">
      {reason || "二进制文件不直接展开文本行级差异。"}
    </InlineNotice>
    {#if rawDiff && logDiff}
      <div class="flex flex-wrap items-center gap-2 text-muted-foreground">
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
      {reason || "该文件超过内联差异预算或仅包含元数据变更。"}
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
      差异读取失败：{reason}
    </InlineNotice>
    {#if logDiff}
      <Button
        size="sm"
        variant="outline"
        onclick={() => logDiff.retryActiveDiff()}
      >
        <IconRefresh aria-hidden="true" />
        重试加载差异
      </Button>
    {/if}
  </div>
{/if}
