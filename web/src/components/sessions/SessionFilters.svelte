<script lang="ts">
  import type { Snippet } from "svelte";
  import { MediaQuery } from "svelte/reactivity";
  import SessionSearch from "./SessionSearch.svelte";
  import { Button } from "$lib/components/ui/button";
  import {
    SegmentedControl,
    SegmentedControlItem,
  } from "$lib/components/ui/segmented-control";
  import { m } from "$lib/paraglide/messages.js";
  import type { SessionInfo } from "../../contracts/schemas";
  import {
    countSessions,
    type StatusFilter,
  } from "../../domains/sessions/session-status";

  let {
    sessions,
    archivedCount,
    query = $bindable(),
    searchInput = $bindable(null),
    statusFilter = $bindable(),
    filtered,
    shortcutDisabled,
    onclear,
    searchAction,
  }: {
    // Unfiltered list: the filter counts never depend on the active filter.
    sessions: readonly SessionInfo[];
    // Archived hybrid records, also unfiltered.
    archivedCount: number;
    query: string;
    searchInput?: HTMLInputElement | null;
    statusFilter: StatusFilter;
    // Filters are active and at least one session still matches.
    filtered: boolean;
    // A dialog or sheet owns the keyboard.
    shortcutDisabled: boolean;
    onclear: () => void;
    searchAction?: Snippet;
  } = $props();

  const wide = new MediaQuery("min-width: 768px");
  let group = $state<HTMLElement | null>(null);
  // The single-row filter scrolls sideways on phones; keep the choice visible.
  $effect(() => {
    void statusFilter;
    group
      ?.querySelector('[role="radio"][aria-checked="true"]')
      ?.scrollIntoView({ inline: "nearest", block: "nearest" });
  });
  const filters = $derived.by(() => {
    const counts = countSessions(sessions);
    return [
      { id: "all", label: m.s3_filter_all(), count: sessions.length },
      { id: "active", label: m.s3_filter_active(), count: counts.active },
      {
        id: "waiting",
        label: m.s3_tool_status_pending(),
        count: counts.waiting,
      },
      { id: "exited", label: m.s3_filter_exited(), count: counts.exited },
      { id: "archived", label: m.s3_archived(), count: archivedCount },
    ] as const;
  });
</script>

<div class="@container">
  <div class="flex flex-col gap-2 @3xl:flex-row @3xl:items-center @3xl:gap-4">
    <div class="flex min-w-0 items-center gap-3 @3xl:flex-1">
      <div class="min-w-0 flex-1">
        <SessionSearch
          bind:query
          bind:input={searchInput}
          {shortcutDisabled}
          {onclear}
          placeholder={wide.current
            ? m.s3_search_sessions_wide_placeholder()
            : m.s3_search_sessions_label()}
        />
      </div>
      {#if searchAction}{@render searchAction()}{/if}
    </div>
    <SegmentedControl
      bind:ref={group}
      value={statusFilter}
      onValueChange={(value) => {
        const next = filters.find((filter) => filter.id === value);
        if (next) statusFilter = next.id;
      }}
      size="form"
      aria-label={m.s3_filter_by_status_label()}
      class="no-scrollbar w-full justify-start overflow-x-auto overflow-y-hidden @3xl:w-fit"
    >
      {#each filters as filter (filter.id)}
        <SegmentedControlItem value={filter.id} class="min-h-9 shrink-0">
          {filter.label}
          <span
            class="inline-block w-[3ch] text-right font-mono text-xs tabular-nums opacity-60"
            >{filter.count}</span
          >
        </SegmentedControlItem>
      {/each}
    </SegmentedControl>
  </div>
</div>
{#if filtered}
  <div class="flex items-center gap-2 text-xs text-muted-foreground">
    <span>{m.s3_sessions_filtered()}</span>
    <Button
      variant="link"
      size="sm"
      class="min-h-9 sm:min-h-8"
      onclick={onclear}>{m.s3_clear_filters()}</Button
    >
  </div>
{/if}
