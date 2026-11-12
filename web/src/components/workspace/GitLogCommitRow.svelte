<script lang="ts">
  import IconTag from "~icons/tabler/tag";
  import { Badge } from "$lib/components/ui/badge";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import { ListButton } from "$lib/components/ui/list-button";
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
    variant="git-row"
    role="option"
    data-commit-row="true"
    data-has-action="true"
    aria-selected={selected}
    onclick={() => onSelect(row.oid)}
  >
    {#if layout}
      <svg
        width={layout.width}
        height={layout.height}
        viewBox={`0 0 ${layout.width} ${layout.height}`}
        class="shrink-0 overflow-visible"
        aria-hidden="true"
        data-graph-oid={row.oid}
      >
        {#each layout.rails as rail (rail.lane)}
          <line
            x1={rail.x}
            y1={rail.y1}
            x2={rail.x}
            y2={rail.y2}
            stroke-width="1.75"
            class={laneStrokeClass(rail.tone)}
          />
        {/each}
        {#each layout.segments as seg (seg.key)}
          <path
            d={seg.d}
            fill="none"
            stroke-width="1.75"
            stroke-dasharray={seg.dashed
              ? "3 2"
              : seg.isBoundary
                ? "2 2"
                : undefined}
            data-edge-kind={seg.kind}
            class={laneStrokeClass(seg.tone)}
          />
        {/each}
        <circle
          cx={layout.nodeX}
          cy={layout.nodeY}
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
          title="筛选条件省略了中间提交，虚线表示间接祖先关系"
        >
          <span class="truncate">筛选省略中间节点</span>
        </Badge>
      {/if}

      {#if taskBaselineOid === row.oid}
        <Badge variant="info">任务起点</Badge>
      {/if}

      {#if compareBaseMarkOid === row.oid}
        <Badge variant="info">比较起点</Badge>
      {/if}

      {#if compareMode?.baseOid === row.oid}
        <Badge variant="info">对比起点</Badge>
      {:else if compareMode?.targetOid === row.oid}
        <Badge variant="info">对比终点</Badge>
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

  <GitRowActionMenu label={`提交操作 ${row.shortOid}`} bind:open={menuOpen}>
    <DropdownMenu.Item onclick={() => onSelect(row.oid)}>
      查看提交详情
    </DropdownMenu.Item>
    <DropdownMenu.Item onclick={() => copyOid(row.oid)}>
      复制提交 OID ({row.shortOid})
    </DropdownMenu.Item>
    {#if onSetTaskBaseline && taskBaselineOid !== row.oid}
      <DropdownMenu.Item onclick={() => onSetTaskBaseline(row.oid)}>
        设为当前工作树任务基线 ({row.shortOid})
      </DropdownMenu.Item>
    {/if}
    {#if onCherryPickCommit || onRevertCommit}
      <DropdownMenu.Separator />
      {#if onCherryPickCommit}
        <DropdownMenu.Item onclick={() => onCherryPickCommit(row.oid)}>
          遴选此提交到当前分支… (Cherry-pick)
        </DropdownMenu.Item>
      {/if}
      {#if onRevertCommit}
        <DropdownMenu.Item onclick={() => onRevertCommit(row.oid)}>
          还原此提交… (Revert)
        </DropdownMenu.Item>
      {/if}
    {/if}
    {#if onMarkCompareBase || onStartCompare}
      <DropdownMenu.Separator />
      {#if onMarkCompareBase}
        {#if compareBaseMarkOid === row.oid}
          <DropdownMenu.Item onclick={() => onMarkCompareBase(null)}>
            取消比较起点标记
          </DropdownMenu.Item>
        {:else}
          <DropdownMenu.Item onclick={() => onMarkCompareBase(row.oid)}>
            设为比较起点 ({row.shortOid})
          </DropdownMenu.Item>
        {/if}
      {/if}
      {#if onStartCompare && compareBaseMarkOid && compareBaseMarkOid !== row.oid}
        <DropdownMenu.Item
          onclick={() => onStartCompare(compareBaseMarkOid, row.oid)}
        >
          与起点 ({compareBaseMarkOid.slice(0, 7)}) 比较
        </DropdownMenu.Item>
      {/if}
      {#if onStartCompare && selectedCommitOid && selectedCommitOid !== row.oid}
        <DropdownMenu.Item
          onclick={() => onStartCompare(row.oid, selectedCommitOid)}
        >
          与当前选中 ({selectedCommitOid.slice(0, 7)}) 比较
        </DropdownMenu.Item>
      {/if}
    {/if}
  </GitRowActionMenu>
</div>
