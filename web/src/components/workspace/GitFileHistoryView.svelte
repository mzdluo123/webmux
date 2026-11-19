<script lang="ts">
  import { tick } from "svelte";
  import { MediaQuery } from "svelte/reactivity";
  import IconArrowLeft from "~icons/tabler/arrow-left";
  import IconFileSearch from "~icons/tabler/file-search";
  import IconGitCommit from "~icons/tabler/git-commit";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import { m } from "$lib/paraglide/messages.js";
  import type { GitFileHistoryRow } from "../../contracts/schemas";
  import type { GitDiffPanelReader } from "../../hooks/workspace/git-log-diff.svelte";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
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
  let list = $state<HTMLDivElement | null>(null);
  let backButton = $state<HTMLButtonElement | null>(null);
  let opener: HTMLElement | null = null;

  async function showDiff(row: GitFileHistoryRow, button: HTMLElement) {
    onSelectRow(row);
    if (desktop.current) return;
    opener = button;
    mobileShowDiff = true;
    await tick();
    backButton?.focus({ preventScroll: true });
  }

  async function showList() {
    mobileShowDiff = false;
    await tick();
    if (opener?.isConnected) opener.focus({ preventScroll: true });
    else list?.focus({ preventScroll: true });
  }

  const activeRow = $derived(
    rows.find((r) => r.oid === selectedOid) ?? rows[0] ?? null,
  );
</script>

{#snippet historyList()}
  <div
    bind:this={list}
    tabindex="-1"
    class={[
      "h-full min-h-0 min-w-0 flex-col lg:w-80 lg:max-w-[42%] lg:shrink-0 lg:border-r",
      !desktop.current && mobileShowDiff ? "hidden" : "flex",
    ]}
    data-testid="git-file-history-list"
  >
    <ScrollArea class="min-h-8 flex-1">
      <div class="space-y-1 p-2">
        {#if loading && rows.length === 0}
          <p class="p-3 text-xs text-muted-foreground" role="status">
            {m.s6_file_history_loading()}
          </p>
        {:else if error && rows.length === 0}
          <div class="p-2">
            <InlineNotice tone="danger">{error}</InlineNotice>
          </div>
        {:else if rows.length === 0}
          <p class="p-3 text-xs text-muted-foreground">
            {m.s6_file_history_empty()}
          </p>
        {:else}
          {#each rows as row (`${row.oid}:${row.path}`)}
            {@const selected = activeRow?.oid === row.oid}
            {@const renamed = Boolean(row.oldPath && row.oldPath !== row.path)}
            <ListButton
              variant="compact-row"
              aria-selected={selected}
              data-file-history-row={row.shortOid}
              onclick={(event) => void showDiff(row, event.currentTarget)}
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

          {#if error}
            <InlineNotice tone="danger">{error}</InlineNotice>
          {/if}

          {#if nextCursor && onLoadMore}
            <div class="pt-1.5 text-center">
              <Button
                size="sm"
                variant="outline"
                disabled={loadingMore}
                onclick={onLoadMore}
              >
                {loadingMore
                  ? m.s6_loading_ellipsis()
                  : m.s6_file_history_load_older()}
              </Button>
            </div>
          {/if}
        {/if}
      </div>
    </ScrollArea>
  </div>
{/snippet}

{#snippet historyDiffSection(isMobile: boolean)}
  <div class="flex h-full min-h-0 min-w-0 flex-1 flex-col bg-background">
    {#if activeRow}
      <div
        class="flex min-w-0 flex-wrap items-center justify-between gap-2 border-b bg-background px-2.5 py-1.5 text-xs"
      >
        <div class="flex min-w-0 flex-wrap items-center gap-1.5">
          {#if isMobile}
            <Button
              size="sm"
              variant="ghost"
              bind:ref={backButton}
              onclick={() => void showList()}
            >
              <IconArrowLeft aria-hidden="true" />
              {m.s6_back_to_commit_list()}
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
              size="sm"
              variant="outline"
              onclick={() => onOpenBlameAt(activeRow.oid, activeRow.path)}
              data-testid="git-file-history-blame-btn"
            >
              <IconFileSearch aria-hidden="true" />
              {m.s6_file_history_blame_at_rev()}
            </Button>
          {/if}
          {#if onJumpToCommit}
            <Button
              size="sm"
              variant="outline"
              onclick={() => onJumpToCommit(activeRow.oid)}
              data-testid="git-file-history-jump-log-btn"
            >
              <IconGitCommit aria-hidden="true" />
              {m.s6_jump_to_commit_in_graph()}
            </Button>
          {/if}
        </div>
      </div>
    {/if}

    <div class="min-h-8 min-w-0 flex-1">
      <GitLogDiffPanel
        logDiff={diffReader}
        fallbackLeftLabel={activeRow?.comparedParentOid?.slice(0, 7) ??
          m.s6_empty_tree()}
        fallbackRightLabel={activeRow?.shortOid ?? ""}
        presentation="docked"
      />
    </div>
  </div>
{/snippet}

<div
  class="flex h-full min-h-0 min-w-0 flex-1 overflow-hidden bg-background"
  data-testid="git-file-history-view"
>
  {@render historyList()}
  {#if desktop.current || mobileShowDiff}
    {@render historyDiffSection(!desktop.current)}
  {/if}
</div>
