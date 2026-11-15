<script lang="ts">
  import { MediaQuery } from "svelte/reactivity";
  import IconSearch from "~icons/tabler/search";
  import { Button } from "$lib/components/ui/button";
  import * as InputGroup from "$lib/components/ui/input-group";
  import * as SegmentedControl from "$lib/components/ui/segmented-control";
  import * as m from "$lib/paraglide/messages.js";
  import type { SessionInfo } from "../../contracts/schemas";
  import {
    countSessions,
    type StatusFilter,
  } from "../../domains/sessions/session-status";

  let {
    sessions,
    archivedCount,
    query = $bindable(),
    statusFilter = $bindable(),
    filtered,
    shortcutDisabled,
    onclear,
  }: {
    // Unfiltered list: the filter counts never depend on the active filter.
    sessions: readonly SessionInfo[];
    // Archived hybrid records, also unfiltered.
    archivedCount: number;
    query: string;
    statusFilter: StatusFilter;
    // Filters are active and at least one session still matches.
    filtered: boolean;
    // A dialog or sheet owns the keyboard.
    shortcutDisabled: boolean;
    onclear: () => void;
  } = $props();

  let searchInput = $state<HTMLInputElement | null>(null);
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

  function searchShortcut(event: KeyboardEvent) {
    if (
      event.defaultPrevented ||
      event.isComposing ||
      event.metaKey ||
      event.ctrlKey ||
      event.altKey ||
      shortcutDisabled
    )
      return;
    const target = event.target;
    if (
      event.key !== "/" ||
      (target instanceof HTMLElement &&
        (target.isContentEditable ||
          target.closest('input, textarea, select, [role="dialog"]')))
    )
      return;
    event.preventDefault();
    searchInput?.focus();
  }
</script>

<svelte:window onkeydown={searchShortcut} />

<div class="flex flex-col gap-2 md:flex-row md:items-center md:gap-4">
  <InputGroup.Root class="min-w-0 md:flex-1">
    <InputGroup.Addon>
      <IconSearch aria-hidden="true" />
    </InputGroup.Addon>
    <InputGroup.Input
      type="search"
      aria-label={m.s3_search_sessions_label()}
      aria-keyshortcuts="/"
      title={m.s3_search_sessions_shortcut_title()}
      placeholder={wide.current
        ? m.s3_search_sessions_wide_placeholder()
        : m.s3_search_sessions_label()}
      bind:ref={searchInput}
      bind:value={query}
      onkeydown={(event) => {
        if (event.key === "Escape" && !event.isComposing) {
          event.preventDefault();
          onclear();
        }
      }}
    />
    <InputGroup.Addon align="inline-end" class="hidden md:flex">
      <kbd class="border px-1" aria-hidden="true">/</kbd>
    </InputGroup.Addon>
  </InputGroup.Root>
  <SegmentedControl.Root
    bind:ref={group}
    value={statusFilter}
    onValueChange={(value) => {
      const next = filters.find((filter) => filter.id === value);
      if (next) statusFilter = next.id;
    }}
    size="form"
    aria-label={m.s3_filter_by_status_label()}
    class="no-scrollbar w-full justify-start overflow-x-auto overflow-y-hidden md:w-fit md:overflow-visible"
  >
    {#each filters as filter (filter.id)}
      <SegmentedControl.Item value={filter.id} class="min-h-9 shrink-0">
        {filter.label}
        <span
          class="inline-block w-[3ch] text-right font-mono text-xs tabular-nums opacity-60"
          >{filter.count}</span
        >
      </SegmentedControl.Item>
    {/each}
  </SegmentedControl.Root>
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
