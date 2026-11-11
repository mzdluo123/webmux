<script lang="ts">
  import { Badge } from "$lib/components/ui/badge";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import { ListButton } from "$lib/components/ui/list-button";
  import {
    laneFillClass,
    laneStrokeClass,
    type GitGraphRowLayout,
  } from "../../domains/workspace/git-graph-layout";
  import type { GitHistoryRow } from "../../contracts/git-workbench-schemas";
  import GitRowActionMenu from "./GitRowActionMenu.svelte";

  let {
    row,
    layout,
    selected = false,
    showAuthorColumn = true,
    onSelect,
  }: {
    row: GitHistoryRow;
    layout?: GitGraphRowLayout;
    selected?: boolean;
    showAuthorColumn?: boolean;
    onSelect: (oid: string) => void;
  } = $props();

  function copyOid(oid: string) {
    if (typeof navigator !== "undefined" && navigator.clipboard) {
      void navigator.clipboard.writeText(oid);
    }
  }
</script>

<div class="group/row relative">
  <ListButton
    variant="git-row"
    role="option"
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
          <span class="truncate" title={refBadge.name}>
            {refBadge.name}
          </span>
        </Badge>
      {/each}
      {#if row.directParentsMissing}
        <Badge variant="warning" class="max-w-full">
          <span class="truncate">筛选省略中间节点</span>
        </Badge>
      {/if}
      <span
        class="min-w-[6ch] max-w-full flex-1 truncate font-medium"
        title={row.subject}
      >
        {row.subject}
      </span>
      <span class="shrink-0 font-mono text-xs text-muted-foreground">
        {row.shortOid}
      </span>
      {#if showAuthorColumn}
        <span class="hidden shrink-0 text-xs text-muted-foreground sm:inline">
          {row.authorName}
        </span>
      {/if}
    </div>
  </ListButton>

  <GitRowActionMenu label={`提交操作 ${row.shortOid}`}>
    <DropdownMenu.Item onclick={() => copyOid(row.oid)}>
      复制提交 OID ({row.shortOid})
    </DropdownMenu.Item>
    <DropdownMenu.Item onclick={() => onSelect(row.oid)}>
      查看提交详情
    </DropdownMenu.Item>
  </GitRowActionMenu>
</div>
