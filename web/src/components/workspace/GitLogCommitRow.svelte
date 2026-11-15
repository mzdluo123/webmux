<script lang="ts">
  import IconTag from "~icons/tabler/tag";
  import { Badge } from "$lib/components/ui/badge";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import { ListButton } from "$lib/components/ui/list-button";
  import * as m from "$lib/paraglide/messages.js";
  import {
    laneFillClass,
    laneStrokeClass,
    type GitGraphRowLayout,
  } from "../../domains/workspace/git-graph-layout";
  import type {
    GitCompareViewState,
    GitHistoryRow,
  } from "../../contracts/git-workbench-schemas";
  import GitRowActionMenu from "./GitRowActionMenu.svelte";

  let {
    row,
    layout,
    selected = false,
    selectedCommitOid = null,
    compareBaseMarkOid = null,
    taskBaselineOid = null,
    compareMode = null,
    showAuthorColumn = true,
    onSelect,
    onMarkCompareBase,
    onStartCompare,
    onSetTaskBaseline,
    onCherryPickCommit,
    onRevertCommit,
  }: {
    row: GitHistoryRow;
    layout?: GitGraphRowLayout;
    selected?: boolean;
    selectedCommitOid?: string | null;
    compareBaseMarkOid?: string | null;
    taskBaselineOid?: string | null;
    compareMode?: GitCompareViewState | null;
    showAuthorColumn?: boolean;
    onSelect: (oid: string) => void;
    onMarkCompareBase?: (oid: string | null) => void;
    onStartCompare?: (baseOid: string, targetOid: string) => void;
    onSetTaskBaseline?: (baseOid: string) => void;
    onCherryPickCommit?: (oid: string) => void;
    onRevertCommit?: (oid: string) => void;
  } = $props();

  function formatShortDate(iso: string): string {
    if (!iso) return "";
    const idx = iso.indexOf("T");
    if (idx === 10) {
      return `${iso.slice(0, 10)} ${iso.slice(11, 16)}`;
    }
    return iso.slice(0, 16);
  }

  function copyOid(oid: string) {
    if (typeof navigator !== "undefined" && navigator.clipboard) {
      void navigator.clipboard.writeText(oid);
    }
  }

  let menuOpen = $state(false);

  function handleContextMenu(e: MouseEvent) {
    e.preventDefault();
    menuOpen = true;
  }

  function handleRowKeydown(e: KeyboardEvent) {
    if (e.key === "ContextMenu" || (e.shiftKey && e.key === "F10")) {
      e.preventDefault();
      menuOpen = true;
    }
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="group/row relative"
  oncontextmenu={handleContextMenu}
  onkeydown={handleRowKeydown}
>
  <ListButton
    variant="compact-row"
    role="option"
    data-commit-row="true"
    data-has-action="true"
    aria-selected={selected}
    onclick={() => onSelect(row.oid)}
  >
    {#if layout}
      <span
        class="w-(--graph-width) shrink-0"
        style:--graph-width={`${layout.width}px`}
        aria-hidden="true"
      ></span>
      <svg
        width={layout.width}
        height="100%"
        class="pointer-events-none absolute inset-y-0 left-2 h-full w-(--graph-width) overflow-visible"
        style:--graph-width={`${layout.width}px`}
        aria-hidden="true"
        data-graph-oid={row.oid}
      >
        {#each layout.rails as rail (rail.lane)}
          <line
            x1={rail.x}
            y1={rail.y1 === 0 ? "0%" : "50%"}
            x2={rail.x}
            y2={rail.y2 === layout.height ? "100%" : "50%"}
            stroke-width="1.75"
            class={laneStrokeClass(rail.tone)}
          />
        {/each}
        <svg
          x="0"
          y="0"
          width={layout.width}
          height="100%"
          viewBox={`0 0 ${layout.width} ${layout.height}`}
          preserveAspectRatio="none"
          class="overflow-visible"
        >
          {#each layout.segments as seg (seg.key)}
            <path
              d={seg.d}
              fill="none"
              stroke-width="1.75"
              vector-effect="non-scaling-stroke"
              stroke-dasharray={seg.dashed
                ? "3 2"
                : seg.isBoundary
                  ? "2 2"
                  : undefined}
              data-edge-kind={seg.kind}
              class={laneStrokeClass(seg.tone)}
            />
          {/each}
        </svg>
        <circle
          cx={layout.nodeX}
          cy="50%"
          r={layout.isMerge ? 4.2 : 3.4}
          class={laneFillClass(layout.nodeTone)}
        />
      </svg>
    {/if}

    <div class="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
      {#each row.refs as refBadge (`${refBadge.kind}:${refBadge.name}`)}
        <Badge
          variant={refBadge.kind === "head"
            ? "info"
            : refBadge.kind === "remote"
              ? "secondary"
              : "outline"}
          class="max-w-full"
        >
          {#if refBadge.kind === "tag"}
            <IconTag class="size-3 shrink-0" aria-hidden="true" />
          {/if}
          <span class="truncate" title={refBadge.name}>
            {refBadge.name}
          </span>
        </Badge>
      {/each}

      {#if row.directParentsMissing}
        <Badge
          variant="warning"
          class="max-w-full"
          title={m.s6_commit_row_missing_parents_tooltip()}
        >
          <span class="truncate">{m.s6_commit_row_missing_parents_badge()}</span
          >
        </Badge>
      {/if}

      {#if taskBaselineOid === row.oid}
        <Badge variant="info">{m.s6_commit_row_task_baseline_badge()}</Badge>
      {/if}

      {#if compareBaseMarkOid === row.oid}
        <Badge variant="info">{m.s6_commit_row_compare_mark_badge()}</Badge>
      {/if}

      {#if compareMode?.baseOid === row.oid}
        <Badge variant="info">{m.s6_commit_row_compare_start_badge()}</Badge>
      {:else if compareMode?.targetOid === row.oid}
        <Badge variant="info">{m.s6_commit_row_compare_end_badge()}</Badge>
      {/if}

      <span
        class="min-w-[6ch] max-w-full flex-1 truncate font-medium"
        title={row.subject}
      >
        {row.subject}
      </span>

      {#if showAuthorColumn}
        <span
          class="hidden max-w-28 shrink-0 truncate text-xs text-muted-foreground md:inline"
          title={`${row.authorName} <${row.authorEmail}>`}
        >
          {row.authorName}
        </span>
      {/if}

      {#if row.committerAt}
        <span
          class="hidden shrink-0 font-mono text-xs text-muted-foreground lg:inline"
          title={row.committerAt}
        >
          {formatShortDate(row.committerAt)}
        </span>
      {/if}

      <span class="shrink-0 font-mono text-xs text-muted-foreground">
        {row.shortOid}
      </span>
    </div>
  </ListButton>

  <GitRowActionMenu
    label={m.s6_commit_row_menu_label({ shortOid: row.shortOid })}
    bind:open={menuOpen}
  >
    <DropdownMenu.Item onclick={() => onSelect(row.oid)}>
      {m.s6_commit_row_view_details()}
    </DropdownMenu.Item>
    <DropdownMenu.Item onclick={() => copyOid(row.oid)}>
      {m.s6_commit_row_copy_oid({ shortOid: row.shortOid })}
    </DropdownMenu.Item>
    {#if onSetTaskBaseline && taskBaselineOid !== row.oid}
      <DropdownMenu.Item onclick={() => onSetTaskBaseline(row.oid)}>
        {m.s6_commit_row_set_task_baseline({ shortOid: row.shortOid })}
      </DropdownMenu.Item>
    {/if}
    {#if onCherryPickCommit || onRevertCommit}
      <DropdownMenu.Separator />
      {#if onCherryPickCommit}
        <DropdownMenu.Item onclick={() => onCherryPickCommit(row.oid)}>
          {m.s6_commit_row_cherry_pick()}
        </DropdownMenu.Item>
      {/if}
      {#if onRevertCommit}
        <DropdownMenu.Item onclick={() => onRevertCommit(row.oid)}>
          {m.s6_commit_row_revert()}
        </DropdownMenu.Item>
      {/if}
    {/if}
    {#if onMarkCompareBase || onStartCompare}
      <DropdownMenu.Separator />
      {#if onMarkCompareBase}
        {#if compareBaseMarkOid === row.oid}
          <DropdownMenu.Item onclick={() => onMarkCompareBase(null)}>
            {m.s6_commit_row_clear_compare_mark()}
          </DropdownMenu.Item>
        {:else}
          <DropdownMenu.Item onclick={() => onMarkCompareBase(row.oid)}>
            {m.s6_commit_row_mark_compare_base({ shortOid: row.shortOid })}
          </DropdownMenu.Item>
        {/if}
      {/if}
      {#if onStartCompare && compareBaseMarkOid && compareBaseMarkOid !== row.oid}
        <DropdownMenu.Item
          onclick={() => onStartCompare(compareBaseMarkOid, row.oid)}
        >
          {m.s6_commit_row_compare_with_marked({
            shortOid: compareBaseMarkOid.slice(0, 7),
          })}
        </DropdownMenu.Item>
      {/if}
      {#if onStartCompare && selectedCommitOid && selectedCommitOid !== row.oid}
        <DropdownMenu.Item
          onclick={() => onStartCompare(row.oid, selectedCommitOid)}
        >
          {m.s6_commit_row_compare_with_selected({
            shortOid: selectedCommitOid.slice(0, 7),
          })}
        </DropdownMenu.Item>
      {/if}
    {/if}
  </GitRowActionMenu>
</div>
