<script lang="ts">
  import GitFileStatusIcon from "../shared/GitFileStatusIcon.svelte";
  import IconArrowsExchange from "~icons/tabler/arrows-exchange";
  import IconFileSearch from "~icons/tabler/file-search";
  import IconHistory from "~icons/tabler/history";
  import IconX from "~icons/tabler/x";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { ListButton } from "$lib/components/ui/list-button";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import { m } from "$lib/paraglide/messages.js";
  import type {
    GitCommitFile,
    GitCompareViewState,
  } from "../../contracts/git-workbench-schemas";
  import type { GitFileInspectorTab } from "../../hooks/workspace/git-file-inspector-controller.svelte";
  import { InlineNotice } from "$lib/components/ui/inline-notice";

  let {
    files,
    selectedFilePath,
    leftEndpoint = "",
    rightEndpoint = "",
    loading = false,
    compareMode = null,
    onSelectFile,
    onPreviewFile,
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
    onSelectFile: (path: string) => void;
    onPreviewFile?: (path: string) => void;
    onInspectFile?: (path: string, tab: GitFileInspectorTab) => void;
    onSwapCompareDirection?: () => void;
    onExitCompare?: () => void;
    onLoadMoreCompareFiles?: () => void;
  } = $props();

  let fileQuery = $state("");
  let listEl = $state<HTMLElement | null>(null);

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
  const inspectPath = $derived(
    loading || compareMode?.loading || compareMode?.error ? null : activePath,
  );

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
      focusRowAt(nextIdx);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      const prevIdx = Math.max(0, currentIdx - 1);
      const prevPath = fileKey(visibleFiles[prevIdx]!);
      onSelectFile(prevPath);
      focusRowAt(prevIdx);
    } else if (e.key === "Home") {
      e.preventDefault();
      const firstPath = fileKey(visibleFiles[0]!);
      onSelectFile(firstPath);
      focusRowAt(0);
    } else if (e.key === "End") {
      e.preventDefault();
      const lastIdx = visibleFiles.length - 1;
      const lastPath = fileKey(visibleFiles[lastIdx]!);
      onSelectFile(lastPath);
      focusRowAt(lastIdx);
    } else if (e.key === "Enter" && activePath) {
      e.preventDefault();
      onSelectFile(activePath);
      onPreviewFile?.(activePath);
    }
  }
</script>

<section
  class="flex h-full min-h-0 min-w-0 flex-col bg-background"
  aria-label={m.s6_log_files_section_aria()}
>
  <div class="space-y-1.5 border-b bg-background px-2.5 py-2">
    {#if compareMode}
      <div class="flex min-w-0 flex-wrap items-center justify-between gap-1.5">
        <div class="flex min-w-0 items-center gap-1.5">
          <Badge variant="info">{m.s6_log_files_compare_badge()}</Badge>
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
              size="sm"
              variant="outline"
              aria-label={m.s6_log_files_swap_aria()}
              onclick={onSwapCompareDirection}
            >
              <IconArrowsExchange aria-hidden="true" />
              {m.s6_log_files_swap_btn()}
            </Button>
          {/if}
          {#if onExitCompare}
            <Button
              size="sm"
              variant="ghost"
              aria-label={m.s6_log_files_exit_compare_aria()}
              onclick={onExitCompare}
            >
              <IconX aria-hidden="true" />
              {m.s6_log_files_exit_compare_btn()}
            </Button>
          {/if}
        </div>
      </div>
    {/if}

    <div class="flex min-w-0 flex-wrap items-center justify-between gap-2">
      <span class="min-w-0 truncate text-xs font-semibold text-foreground">
        {m.s6_log_files_heading_count({ count: String(sourceFiles.length) })}
        {#if !compareMode && rightEndpoint}
          <span class="ml-1 font-mono font-normal text-muted-foreground">
            {leftEndpoint || m.s6_empty_tree()} → {rightEndpoint}
          </span>
        {/if}
      </span>
      <div class="flex shrink-0 items-center gap-1">
        {#if onInspectFile && (compareMode || inspectPath)}
          <Button
            size="sm"
            variant="ghost"
            disabled={!inspectPath}
            onclick={() => inspectPath && onInspectFile(inspectPath, "history")}
            data-testid="git-log-file-history-btn"
          >
            <IconHistory aria-hidden="true" />
            {m.s6_log_files_history_btn()}
          </Button>
          <Button
            size="sm"
            variant="ghost"
            disabled={!inspectPath}
            onclick={() => inspectPath && onInspectFile(inspectPath, "blame")}
            data-testid="git-log-file-blame-btn"
          >
            <IconFileSearch aria-hidden="true" />
            Blame
          </Button>
        {/if}
      </div>
    </div>

    {#if compareMode || sourceFiles.length > 0 || fileQuery}
      <Input
        size="sm"
        mono
        value={fileQuery}
        placeholder={m.s6_log_files_filter_placeholder()}
        aria-label={m.s6_log_files_filter_aria()}
        oninput={(e) => (fileQuery = e.currentTarget.value)}
      />
    {/if}
  </div>

  <!-- Interactive listbox delegates ArrowUp/ArrowDown/Enter across child option buttons. -->
  <ScrollArea
    bind:viewportRef={listEl}
    class={`min-h-8 flex-1 ${compareMode ? "max-sm:h-[min(35dvh,16rem)] max-sm:flex-none" : ""}`}
    viewportProps={{
      role: "listbox",
      "aria-label": m.s6_log_files_listbox_aria(),
      onkeydown: handleListKeydown,
    }}
  >
    <div class="p-1.5">
      {#if compareMode?.loading || loading}
        <p class="p-2.5 text-xs text-muted-foreground" role="status">
          {m.s6_log_files_loading()}
        </p>
      {:else if compareMode?.error}
        <div class="p-2">
          <InlineNotice tone="danger">{compareMode.error}</InlineNotice>
        </div>
      {:else if visibleFiles.length === 0}
        <p class="p-2.5 text-xs text-muted-foreground">
          {normalizedQuery
            ? m.s6_log_files_no_filter_matches()
            : compareMode
              ? m.s6_log_files_no_compare_diff()
              : m.s6_log_files_no_commit_changes()}
        </p>
      {:else}
        {#each visibleFiles as file (`${file.status}:${file.oldPath}:${fileKey(file)}`)}
          {@const pathKey = fileKey(file)}
          <ListButton
            variant="compact-row"
            role="option"
            data-commit-file-row="true"
            aria-selected={activePath === pathKey}
            onclick={() => {
              onSelectFile(pathKey);
              onPreviewFile?.(pathKey);
            }}
          >
            <GitFileStatusIcon status={file.status} />

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
              <Badge variant="secondary">{m.s6_log_files_binary_badge()}</Badge>
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
              size="sm"
              variant="outline"
              disabled={compareMode.loadingMore}
              onclick={onLoadMoreCompareFiles}
            >
              {compareMode.loadingMore
                ? m.s6_loading_ellipsis()
                : m.s6_log_files_load_more()}
            </Button>
          </div>
        {/if}
      {/if}
    </div>
  </ScrollArea>
</section>
