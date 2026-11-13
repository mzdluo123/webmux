<script lang="ts">
  import { MediaQuery } from "svelte/reactivity";
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import IconFileSearch from "~icons/tabler/file-search";
  import IconGitCommit from "~icons/tabler/git-commit";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import type { GitFileHistoryRow } from "../../contracts/schemas";
  import type { GitDiffPanelReader } from "../../hooks/workspace/git-log-diff.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import GitLogDiffPanel from "./GitLogDiffPanel.svelte";

  let {
    rows,
    selectedOid = null,
    nextCursor = "",
    loading = false,
    loadingMore = false,
    error = "",
    diffReader,
    onSelectRow,
    onLoadMore,
    onOpenBlameAt,
    onJumpToCommit,
  }: {
    rows: readonly GitFileHistoryRow[];
    selectedOid?: string | null;
    nextCursor?: string;
    loading?: boolean;
    loadingMore?: boolean;
    error?: string;
    diffReader?: GitDiffPanelReader;
    onSelectRow: (row: GitFileHistoryRow) => void;
    onLoadMore?: () => void;
    onOpenBlameAt?: (oid: string, path: string) => void;
    onJumpToCommit?: (oid: string) => void;
  } = $props();

  const desktop = new MediaQuery("(min-width: 1024px)");
  let mobileShowDiff = $state(false);

  const activeRow = $derived(
    rows.find((r) => r.oid === selectedOid) ?? rows[0] ?? null,
  );
</script>

{#snippet historyList(isMobile: boolean)}
  <div
    class="flex h-full min-h-0 min-w-0 flex-col border-git-border lg:w-80 lg:shrink-0 lg:border-r"
    data-testid="git-file-history-list"
  >
    <div class="min-h-0 flex-1 space-y-1 overflow-y-auto p-2">
      {#if loading && rows.length === 0}
        <p class="p-3 text-xs text-muted-foreground" role="status">
          正在读取单文件重命名追踪历史 (--follow)…
        </p>
      {:else if error}
        <div class="p-2">
          <InlineNotice tone="danger">{error}</InlineNotice>
        </div>
      {:else if rows.length === 0}
        <p class="p-3 text-xs text-muted-foreground">
          未找到该文件在当前修订下的提交历史。
        </p>
      {:else}
        {#each rows as row (`${row.oid}:${row.path}`)}
          {@const selected = activeRow?.oid === row.oid}
          {@const renamed = Boolean(row.oldPath && row.oldPath !== row.path)}
          <ListButton
            variant="git-row"
            aria-selected={selected}
            data-file-history-row={row.shortOid}
            onclick={() => {
              onSelectRow(row);
              if (isMobile) mobileShowDiff = true;
            }}
          >
            <div class="flex min-w-0 flex-1 flex-col gap-1">
              <div class="flex min-w-0 flex-wrap items-center gap-1.5">
                <Badge
                  variant={row.status.startsWith("R")
                    ? "warning"
                    : row.status === "A"
                      ? "info"
                      : "outline"}
                >
                  {row.status}
                </Badge>
                <span class="font-mono text-xs text-muted-foreground">
                  {row.shortOid}
                </span>
                <span class="truncate text-xs text-muted-foreground">
                  {row.authorName}
                </span>
              </div>
              <p class="truncate text-xs font-medium text-foreground">
                {row.subject}
              </p>
              {#if renamed}
                <span
                  class="truncate font-mono text-xs text-warning"
                  title={`${row.oldPath} → ${row.path}`}
                  data-testid="git-file-history-rename-badge"
                >
                  {row.oldPath} → {row.path}
                </span>
              {/if}
            </div>
          </ListButton>
        {/each}

        {#if nextCursor && onLoadMore}
          <div class="pt-1.5 text-center">
            <Button
              size="xs"
              variant="outline"
              disabled={loadingMore}
              onclick={onLoadMore}
            >
              {loadingMore ? "加载中…" : "加载更早提交"}
            </Button>
          </div>
        {/if}
      {/if}
    </div>
  </div>
{/snippet}

{#snippet historyDiffSection(isMobile: boolean)}
  <div class="flex h-full min-h-0 min-w-0 flex-1 flex-col bg-git-surface">
    {#if activeRow}
      <div
        class="flex min-w-0 flex-wrap items-center justify-between gap-2 border-b border-git-border bg-git-panel px-2.5 py-1.5 text-xs"
      >
        <div class="flex min-w-0 items-center gap-1.5">
          {#if isMobile}
            <Button
              size="xs"
              variant="ghost"
              onclick={() => (mobileShowDiff = false)}
            >
              <IconArrowLeft aria-hidden="true" />
              返回提交列表
            </Button>
          {/if}
          <span class="font-mono text-muted-foreground">
            {activeRow.shortOid}
          </span>
          <span class="truncate font-medium text-foreground">
            {activeRow.subject}
          </span>
        </div>

        <div class="flex flex-wrap items-center gap-1.5">
          {#if onOpenBlameAt}
            <Button
              size="xs"
              variant="outline"
              onclick={() => onOpenBlameAt(activeRow.oid, activeRow.path)}
              data-testid="git-file-history-blame-btn"
            >
              <IconFileSearch aria-hidden="true" />
              此版本 Blame
            </Button>
          {/if}
          {#if onJumpToCommit}
            <Button
              size="xs"
              variant="outline"
              onclick={() => onJumpToCommit(activeRow.oid)}
              data-testid="git-file-history-jump-log-btn"
            >
              <IconGitCommit aria-hidden="true" />
              在历史图中定位
            </Button>
          {/if}
        </div>
      </div>
    {/if}

    <div class="min-h-0 min-w-0 flex-1">
      <GitLogDiffPanel
        logDiff={diffReader}
        fallbackLeftLabel={activeRow?.comparedParentOid?.slice(0, 7) ?? "空树"}
        fallbackRightLabel={activeRow?.shortOid ?? ""}
        presentation="docked"
      />
    </div>
  </div>
{/snippet}

<div
  class="flex h-full min-h-0 min-w-0 flex-1 overflow-hidden bg-git-surface"
  data-testid="git-file-history-view"
>
  {#if desktop.current}
    {@render historyList(false)}
    {@render historyDiffSection(false)}
  {:else if !mobileShowDiff}
    {@render historyList(true)}
  {:else}
    {@render historyDiffSection(true)}
  {/if}
</div>
