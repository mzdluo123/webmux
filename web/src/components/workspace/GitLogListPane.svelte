<script lang="ts">
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import * as m from "$lib/paraglide/messages.js";
  import { computeGitGraphLayout } from "../../domains/workspace/git-graph-layout";
  import { syncElementScrollTop } from "../../domains/workspace/git-workbench-memory";
  import type {
    GitCompareViewState,
    GitHistoryRow,
    GitHistoryStatus,
    GitTaskBaseline,
  } from "../../contracts/git-workbench-schemas";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import GitLogCommitRow from "./GitLogCommitRow.svelte";
  import GitLogFilterBar from "./GitLogFilterBar.svelte";
  import type {
    GitLogFilterCallbacks,
    GitLogFilterFields,
  } from "./git-log-pane-types";

  let {
    desktop = true,
    historyState,
    historyReason = "",
    taskBaseline = null,
    nextCursor = "",
    historyLoadingMore = false,
    visibleCommits,
    selectedCommitOid,
    compareMode = null,
    showBranchesPanel = true,
    showAuthorColumn = true,
    listScrollTop = 0,
    onListScrollChange,
    onOpenRefDrawer,
    onToggleBranchesPanel,
    onResetLayout,
    onLoadMoreHistory,
    onSelectCommit,
    onMarkCompareBase,
    onStartCompare,
    onSetTaskBaseline,
    onCherryPickCommit,
    onRevertCommit,
    ...filterProps
  }: GitLogFilterFields &
    GitLogFilterCallbacks & {
      desktop?: boolean;
      historyState: GitHistoryStatus;
      historyReason?: string;
      taskBaseline?: GitTaskBaseline | null;
      nextCursor?: string;
      historyLoadingMore?: boolean;
      visibleCommits: readonly GitHistoryRow[];
      selectedCommitOid: string | null;
      compareMode?: GitCompareViewState | null;
      showBranchesPanel?: boolean;
      showAuthorColumn?: boolean;
      listScrollTop?: number;
      onListScrollChange?: (top: number) => void;
      onOpenRefDrawer?: () => void;
      onToggleBranchesPanel?: () => void;
      onResetLayout?: () => void;
      onLoadMoreHistory?: () => void;
      onSelectCommit: (oid: string) => void;
      onMarkCompareBase?: (oid: string | null) => void;
      onStartCompare?: (baseOid: string, targetOid: string) => void;
      onSetTaskBaseline?: (baseOid?: string) => void;
      onCherryPickCommit?: (oid: string) => void;
      onRevertCommit?: (oid: string) => void;
    } = $props();

  let listScrollEl = $state<HTMLElement | null>(null);
  $effect(() => syncElementScrollTop(listScrollEl, listScrollTop));

  const graphLayouts = $derived(computeGitGraphLayout(visibleCommits));

  function focusCommitRow(index: number) {
    const rows =
      listScrollEl?.querySelectorAll<HTMLButtonElement>("[data-commit-row]");
    rows?.[index]?.focus();
  }

  function handleListKeydown(e: KeyboardEvent) {
    if (visibleCommits.length === 0) return;
    if (!(e.target instanceof HTMLElement)) return;
    if (!e.target.hasAttribute("data-commit-row")) return;

    const currentIdx = Math.max(
      0,
      visibleCommits.findIndex((c) => c.oid === selectedCommitOid),
    );
    if (e.key === "ArrowDown") {
      e.preventDefault();
      const nextIdx = Math.min(visibleCommits.length - 1, currentIdx + 1);
      onSelectCommit(visibleCommits[nextIdx]!.oid);
      focusCommitRow(nextIdx);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      const prevIdx = Math.max(0, currentIdx - 1);
      onSelectCommit(visibleCommits[prevIdx]!.oid);
      focusCommitRow(prevIdx);
    } else if (e.key === "Home") {
      e.preventDefault();
      onSelectCommit(visibleCommits[0]!.oid);
      focusCommitRow(0);
    } else if (e.key === "End") {
      e.preventDefault();
      const lastIdx = visibleCommits.length - 1;
      onSelectCommit(visibleCommits[lastIdx]!.oid);
      focusCommitRow(lastIdx);
    }
  }

  const listboxViewportProps = $derived({
    role: "listbox" as const,
    "aria-label": m.s6_log_list_listbox_aria(),
    onkeydown: handleListKeydown,
  });

  function describeHistoryStatus(status: GitHistoryStatus): string {
    if (historyReason) return historyReason;
    switch (status) {
      case "no_worktree":
        return m.s6_history_status_no_worktree();
      case "no_task_baseline":
        return m.s6_history_status_no_task_baseline();
      case "detached_no_branch":
        return m.s6_history_status_detached();
      case "no_upstream":
        return m.s6_history_status_no_upstream();
      case "upstream_gone":
        return m.s6_history_status_upstream_gone();
      case "unborn":
        return m.s6_history_status_unborn();
      default:
        return m.s6_history_status_empty();
    }
  }
</script>

<section
  class="flex h-full min-h-0 min-w-0 flex-col bg-background"
  aria-label={m.s6_log_list_section_aria()}
>
  <GitLogFilterBar
    {desktop}
    {...filterProps}
    {showBranchesPanel}
    {onOpenRefDrawer}
    {onToggleBranchesPanel}
    {onResetLayout}
    onCancelCompareMark={onMarkCompareBase
      ? () => onMarkCompareBase(null)
      : undefined}
  />

  {#if filterProps.scope === "task" && taskBaseline}
    <div
      class="flex min-w-0 flex-wrap items-center justify-between gap-1.5 border-b bg-background px-2.5 py-1.5 text-xs"
    >
      <div class="flex min-w-0 flex-wrap items-center gap-1.5">
        <span class="text-muted-foreground"
          >{m.s6_log_list_task_baseline_label()}</span
        >
        <Badge variant="info">
          <span class="font-mono">
            {taskBaseline.unborn || !taskBaseline.baseOid
              ? m.s6_task_baseline_unborn()
              : taskBaseline.baseOid.slice(0, 7)}
          </span>
        </Badge>
        <Badge variant="outline">
          {taskBaseline.source === "worktree_create"
            ? m.s6_log_list_baseline_auto()
            : m.s6_log_list_baseline_manual()}
        </Badge>
        {#if !taskBaseline.cleanAtStart}
          <Badge variant="warning">{m.s6_log_list_baseline_dirty()}</Badge>
        {/if}
      </div>
    </div>
  {/if}

  <!-- Interactive listbox delegates ArrowUp/ArrowDown/Home/End across child option buttons. -->
  <ScrollArea
    bind:viewportRef={listScrollEl}
    class="min-h-8 flex-1"
    viewportProps={listboxViewportProps}
    onscroll={(e) => {
      const el = e.currentTarget;
      onListScrollChange?.(el.scrollTop);
      if (
        nextCursor &&
        !historyLoadingMore &&
        onLoadMoreHistory &&
        el.scrollHeight - el.scrollTop - el.clientHeight < 120
      ) {
        onLoadMoreHistory();
      }
    }}
  >
    <div class="p-1.5">
      {#if historyState !== "ready"}
        <div class="p-2">
          <InlineNotice
            tone={historyState === "upstream_gone" ? "warning" : "info"}
          >
            <span class="mr-2">{describeHistoryStatus(historyState)}</span>
            {#if historyState === "no_task_baseline" && onSetTaskBaseline}
              <Button
                size="sm"
                variant="outline"
                onclick={() => onSetTaskBaseline()}
              >
                {m.s6_log_list_set_head_baseline()}
              </Button>
            {/if}
          </InlineNotice>
        </div>
      {:else if visibleCommits.length === 0}
        <p class="p-3 text-xs text-muted-foreground">
          {m.s6_log_list_no_filter_matches()}
        </p>
      {:else}
        {#each visibleCommits as row, idx (row.oid)}
          <GitLogCommitRow
            {row}
            layout={graphLayouts[idx]}
            selected={selectedCommitOid === row.oid}
            {selectedCommitOid}
            compareBaseMarkOid={filterProps.compareBaseMarkOid}
            taskBaselineOid={taskBaseline && !taskBaseline.unborn
              ? (taskBaseline.baseOid ?? null)
              : null}
            {compareMode}
            {showAuthorColumn}
            onSelect={onSelectCommit}
            {onMarkCompareBase}
            {onStartCompare}
            {onSetTaskBaseline}
            {onCherryPickCommit}
            {onRevertCommit}
          />
        {/each}
      {/if}
    </div>
  </ScrollArea>

  {#if historyState === "ready" && nextCursor && onLoadMoreHistory}
    <div
      class="flex shrink-0 items-center justify-between gap-2 border-t bg-background px-2.5 py-1.5"
    >
      <span class="text-xs text-muted-foreground">
        {m.s6_log_list_loaded_count({ count: String(visibleCommits.length) })}
      </span>
      <Button
        size="sm"
        variant="outline"
        disabled={historyLoadingMore}
        onclick={onLoadMoreHistory}
      >
        {historyLoadingMore
          ? m.s6_log_list_loading_more()
          : m.s6_log_list_load_more()}
      </Button>
    </div>
  {/if}
</section>
