<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
  import type { WorktreeBatchFilter } from "../../domains/workspace/worktree-batch";
  import { Button } from "$lib/components/ui/button";
  import * as SegmentedControl from "$lib/components/ui/segmented-control";
  import ListSearchInput from "../shared/ListSearchInput.svelte";

  let {
    stage,
    query = $bindable(""),
    statusFilter = $bindable<WorktreeBatchFilter>("all"),
    counts,
    selectedCount,
    totalCount,
    staleCount,
    busy = false,
    onselectallsafe,
    onclearselection,
    onstopselecting,
    onprunestale,
    onstartselecting,
  }: {
    stage: "browse" | "select";
    query: string;
    statusFilter: WorktreeBatchFilter;
    counts: { all: number; safe: number; blocked: number; protected: number };
    selectedCount: number;
    totalCount: number;
    staleCount: number;
    busy?: boolean;
    onselectallsafe: () => void;
    onclearselection: () => void;
    onstopselecting: () => void;
    onprunestale: () => void;
    onstartselecting: () => void;
  } = $props();

  const filters = $derived([
    { id: "all", label: m.s8_filter_all(), count: counts.all },
    { id: "safe", label: m.s8_wt_filter_safe(), count: counts.safe },
    { id: "blocked", label: m.s8_wt_filter_blocked(), count: counts.blocked },
    {
      id: "protected",
      label: m.s8_wt_filter_protected(),
      count: counts.protected,
    },
  ] as const);
</script>

<div class="space-y-3">
  <ListSearchInput
    bind:value={query}
    label={m.s8_wt_search_label()}
    placeholder={m.s8_wt_search_placeholder()}
  />

  <SegmentedControl.Root
    value={statusFilter}
    onValueChange={(value) => {
      const next = filters.find((item) => item.id === value);
      if (next) statusFilter = next.id;
    }}
    size="form"
    aria-label={m.s8_wt_filter_aria()}
    wrap
  >
    {#each filters as filter (filter.id)}
      <SegmentedControl.Item value={filter.id} class="min-h-9">
        {filter.label}
        <span
          class="inline-block w-[3ch] text-right font-mono text-xs tabular-nums opacity-60"
          >{filter.count}</span
        >
      </SegmentedControl.Item>
    {/each}
  </SegmentedControl.Root>

  <div class="flex flex-wrap items-center justify-between gap-2 text-xs">
    <span class="text-muted-foreground tabular-nums">
      {stage === "select"
        ? m.s8_wt_selected_count({
            selected: String(selectedCount),
            total: String(totalCount),
          })
        : m.s8_wt_total_count({ count: String(totalCount) })}
    </span>
    <div class="flex flex-wrap items-center gap-1.5">
      {#if stage === "select"}
        <Button
          type="button"
          variant="outline"
          size="sm"
          class="min-h-9 sm:min-h-8"
          disabled={busy || counts.safe === 0}
          onclick={onselectallsafe}
        >
          {m.s8_wt_select_all_safe({ count: String(counts.safe) })}
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          class="min-h-9 sm:min-h-8"
          disabled={busy || selectedCount === 0}
          onclick={onclearselection}
        >
          {m.s8_wt_clear_selection()}
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          class="min-h-9 sm:min-h-8"
          disabled={busy}
          onclick={onstopselecting}
        >
          {m.s8_wt_exit_batch()}
        </Button>
      {:else}
        {#if staleCount > 0}
          <Button
            type="button"
            variant="outline"
            size="sm"
            class="min-h-9 sm:min-h-8"
            disabled={busy}
            onclick={onprunestale}
          >
            {busy
              ? m.s8_loading_preview()
              : m.s8_wt_prune_stale_count({ count: String(staleCount) })}
          </Button>
        {/if}
        <Button
          type="button"
          variant="outline"
          size="sm"
          class="min-h-9 sm:min-h-8"
          disabled={busy || totalCount === 0}
          onclick={onstartselecting}
        >
          {m.s8_wt_batch_clean()}
        </Button>
      {/if}
    </div>
  </div>
</div>
