<script lang="ts">
  import IconArrowsExchange from "~icons/tabler/arrows-exchange";
  import IconFileSearch from "~icons/tabler/file-search";
  import IconHistory from "~icons/tabler/history";
  import IconMaximize from "~icons/tabler/maximize";
  import IconX from "~icons/tabler/x";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { ListButton } from "$lib/components/ui/list-button";
  import type {
    GitCommitFile,
    GitCompareViewState,
  } from "../../contracts/git-workbench-schemas";
  import type { GitFileInspectorTab } from "../../hooks/workspace/git-file-inspector-controller.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";

  let {
    files,
    selectedFilePath,
    leftEndpoint = "",
    rightEndpoint = "",
    loading = false,
    compareMode = null,
    phone = false,
    onSelectFile,
    onPreviewFile,
    onExpandFile,
    onInspectFile,
    onSwapCompareDirection,
    onExitCompare,
    onLoadMoreCompareFiles,
  }: {
    files: readonly GitCommitFile[];
    selectedFilePath: string | null;
    leftEndpoint?: string;
    rightEndpoint?: string;
    loading?: boolean;
    compareMode?: GitCompareViewState | null;
    phone?: boolean;
    onSelectFile: (path: string) => void;
    onPreviewFile?: (path: string) => void;
    onExpandFile?: (path: string) => void;
    onInspectFile?: (path: string, tab: GitFileInspectorTab) => void;
    onSwapCompareDirection?: () => void;
    onExitCompare?: () => void;
    onLoadMoreCompareFiles?: () => void;
  } = $props();

  let fileQuery = $state("");
  let listEl = $state<HTMLDivElement>();

  const sourceFiles = $derived(compareMode ? compareMode.files : files);
  const normalizedQuery = $derived(fileQuery.trim().toLowerCase());
  const visibleFiles = $derived(
    normalizedQuery
      ? sourceFiles.filter(
          (f) =>
            f.newPath.toLowerCase().includes(normalizedQuery) ||
            f.oldPath.toLowerCase().includes(normalizedQuery),
        )
      : sourceFiles,
  );

  const activePath = $derived.by(() => {
    if (
      selectedFilePath &&
      visibleFiles.some((f) => (f.newPath || f.oldPath) === selectedFilePath)
    ) {
      return selectedFilePath;
    }
    const first = visibleFiles[0];
    return first ? first.newPath || first.oldPath : null;
  });

  function fileKey(file: GitCommitFile): string {
    return file.newPath || file.oldPath;
  }

  function focusRowAt(index: number) {
    const buttons = listEl?.querySelectorAll<HTMLButtonElement>(
      "[data-commit-file-row]",
    );
    buttons?.[index]?.focus();
  }

  function handleListKeydown(e: KeyboardEvent) {
    if (visibleFiles.length === 0) return;
    const currentIdx = Math.max(
      0,
      visibleFiles.findIndex((f) => fileKey(f) === activePath),
    );

    if (e.key === "ArrowDown") {
      e.preventDefault();
      const nextIdx = Math.min(visibleFiles.length - 1, currentIdx + 1);
      const nextPath = fileKey(visibleFiles[nextIdx]!);
      onSelectFile(nextPath);
      onPreviewFile?.(nextPath);
      focusRowAt(nextIdx);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      const prevIdx = Math.max(0, currentIdx - 1);
      const prevPath = fileKey(visibleFiles[prevIdx]!);
      onSelectFile(prevPath);
      onPreviewFile?.(prevPath);
      focusRowAt(prevIdx);
    } else if (e.key === "Home") {
      e.preventDefault();
      const firstPath = fileKey(visibleFiles[0]!);
      onSelectFile(firstPath);
      onPreviewFile?.(firstPath);
      focusRowAt(0);
    } else if (e.key === "End") {
      e.preventDefault();
      const lastIdx = visibleFiles.length - 1;
      const lastPath = fileKey(visibleFiles[lastIdx]!);
      onSelectFile(lastPath);
      onPreviewFile?.(lastPath);
      focusRowAt(lastIdx);
    } else if (e.key === "Enter" && activePath) {
      e.preventDefault();
      onExpandFile?.(activePath);
    }
  }
</script>

<section
  class="flex h-full min-h-0 min-w-0 flex-col bg-git-surface"
  aria-label="变更文件列表"
>
  <div class="space-y-1.5 border-b border-git-border bg-git-panel px-2.5 py-2">
    {#if compareMode}
      <div class="flex min-w-0 flex-wrap items-center justify-between gap-1.5">
        <div class="flex min-w-0 items-center gap-1.5">
          <Badge variant="info">两版本比较</Badge>
          <span class="truncate font-mono text-xs text-foreground">
            {compareMode.baseOid.slice(0, 7)} → {compareMode.targetOid.slice(
              0,
              7,
            )}
          </span>
        </div>
        <div class="flex items-center gap-1">
          {#if onSwapCompareDirection}
            <Button
              size="xs"
              variant="outline"
              aria-label="交换比较方向"
              onclick={onSwapCompareDirection}
            >
              <IconArrowsExchange aria-hidden="true" />
              交换方向
            </Button>
          {/if}
          {#if onExitCompare}
            <Button
              size="xs"
              variant="ghost"
              aria-label="退出两版本比较"
              onclick={onExitCompare}
            >
              <IconX aria-hidden="true" />
              退出比较
            </Button>
          {/if}
        </div>
      </div>
    {/if}

    <div class="flex min-w-0 flex-wrap items-center justify-between gap-2">
      <span class="min-w-0 truncate text-xs font-semibold text-foreground">
        变更文件 ({sourceFiles.length})
        {#if !compareMode && rightEndpoint}
          <span class="ml-1 font-mono font-normal text-muted-foreground">
            {leftEndpoint || "空树"} → {rightEndpoint}
          </span>
        {/if}
      </span>
      <div class="flex shrink-0 items-center gap-1">
        {#if activePath && onInspectFile}
          <Button
            size="xs"
            variant="ghost"
            onclick={() => activePath && onInspectFile(activePath, "history")}
            data-testid="git-log-file-history-btn"
          >
            <IconHistory aria-hidden="true" />
            历史
          </Button>
          <Button
            size="xs"
            variant="ghost"
            onclick={() => activePath && onInspectFile(activePath, "blame")}
            data-testid="git-log-file-blame-btn"
          >
            <IconFileSearch aria-hidden="true" />
            Blame
          </Button>
        {/if}
        {#if !phone && activePath && onExpandFile}
          <Button
            size="icon-sm"
            variant="ghost"
            aria-label="放大查看当前文件差异"
            title="放大查看当前文件差异 (Enter / 双击)"
            onclick={() => activePath && onExpandFile(activePath)}
          >
            <IconMaximize aria-hidden="true" />
          </Button>
        {/if}
      </div>
    </div>

    {#if sourceFiles.length > 0 || fileQuery}
      <Input
        size="sm"
        mono
        value={fileQuery}
        placeholder="按路径过滤变更文件…"
        aria-label="筛选变更文件"
        oninput={(e) => (fileQuery = e.currentTarget.value)}
      />
    {/if}
  </div>

  <!-- Interactive listbox delegates ArrowUp/ArrowDown/Enter across child option buttons. -->
  <!-- svelte-ignore a11y_interactive_supports_focus -->
  <div
    bind:this={listEl}
    class="min-h-0 flex-1 overflow-y-auto p-1.5"
    role="listbox"
    aria-label="提交变更文件"
    onkeydown={handleListKeydown}
  >
    {#if compareMode?.loading || loading}
      <p class="p-2.5 text-xs text-muted-foreground" role="status">
        正在加载变更文件列表…
      </p>
    {:else if compareMode?.error}
      <div class="p-2">
        <InlineNotice tone="danger">{compareMode.error}</InlineNotice>
      </div>
    {:else if visibleFiles.length === 0}
      <p class="p-2.5 text-xs text-muted-foreground">
        {normalizedQuery
          ? "没有匹配过滤路径的变更文件。"
          : compareMode
            ? "这两个提交版本之间没有文件差异。"
            : "当前提交在此父节点对比下没有文件变更。"}
      </p>
    {:else}
      {#each visibleFiles as file (`${file.status}:${file.oldPath}:${fileKey(file)}`)}
        {@const pathKey = fileKey(file)}
        <ListButton
          variant="git-row"
          role="option"
          data-commit-file-row="true"
          aria-selected={activePath === pathKey}
          onclick={() => {
            onSelectFile(pathKey);
            onPreviewFile?.(pathKey);
          }}
          ondblclick={() => {
            if (!phone) onExpandFile?.(pathKey);
          }}
        >
          <Badge
            variant={file.status === "D"
              ? "destructive"
              : file.status === "A"
                ? "info"
                : file.status === "R"
                  ? "warning"
                  : "outline"}
          >
            {file.status}
          </Badge>

          <span
            class="min-w-0 flex-1 truncate font-mono text-xs"
            title={file.oldPath && file.oldPath !== file.newPath
              ? `${file.oldPath} → ${file.newPath}`
              : pathKey}
          >
            {file.oldPath && file.oldPath !== file.newPath
              ? `${file.oldPath} → ${file.newPath}`
              : pathKey}
          </span>

          {#if file.binary}
            <Badge variant="secondary">二进制</Badge>
          {/if}

          {#if file.additions > 0 || file.deletions > 0}
            <span class="shrink-0 font-mono text-xs text-success">
              +{file.additions}
            </span>
            <span class="shrink-0 font-mono text-xs text-destructive">
              -{file.deletions}
            </span>
          {/if}
        </ListButton>
      {/each}

      {#if compareMode?.nextCursor && onLoadMoreCompareFiles}
        <div class="pt-1.5 text-center">
          <Button
            size="xs"
            variant="outline"
            disabled={compareMode.loadingMore}
            onclick={onLoadMoreCompareFiles}
          >
            {compareMode.loadingMore ? "加载中…" : "加载更多变更文件"}
          </Button>
        </div>
      {/if}
    {/if}
  </div>
</section>
