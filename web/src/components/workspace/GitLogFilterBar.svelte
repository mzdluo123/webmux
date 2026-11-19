<script lang="ts">
  import { useDockMotion } from "../../hooks/shared/dock-motion.svelte";
  import IconFilter from "~icons/tabler/filter";
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconRefresh from "~icons/tabler/refresh";
  import IconRestore from "~icons/tabler/restore";
  import IconX from "~icons/tabler/x";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import {
    SegmentedControl,
    SegmentedControlItem,
  } from "$lib/components/ui/segmented-control";
  import { m } from "$lib/paraglide/messages.js";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import type {
    GitLogFilterCallbacks,
    GitLogFilterFields,
  } from "./git-log-pane-types";

  let {
    scope,
    branchFilter,
    searchQuery,
    authorFilter = "",
    sinceFilter = "",
    untilFilter = "",
    pathFilter,
    refsChanged = false,
    selectionNotice = null,
    compareBaseMarkOid = null,
    onOpenRefDrawer,
    onResetLayout,
    onRefreshHistory,
    onScopeChange,
    onSearchChange,
    onAuthorChange,
    onSinceChange,
    onUntilChange,
    onPathFilterChange,
    onClearFilters,
    onJumpToHead,
    onDismissSelectionNotice,
    onCancelCompareMark,
  }: GitLogFilterFields &
    GitLogFilterCallbacks & {
      onOpenRefDrawer?: () => void;
      onResetLayout?: () => void;
      onCancelCompareMark?: () => void;
    } = $props();

  let manualAdvancedOpen = $state(false);
  const hasAdvancedFilter = $derived(
    Boolean(authorFilter.trim() || sinceFilter.trim() || untilFilter.trim()),
  );
  const showAdvanced = $derived(manualAdvancedOpen || hasAdvancedFilter);
  let advancedPanel = $state<HTMLDivElement | null>(null);
  let manualChange = false;
  useDockMotion(
    () => advancedPanel,
    () => showAdvanced,
    undefined,
    () => {
      const animate = manualChange;
      manualChange = false;
      return animate;
    },
  );

  function toggleAdvanced() {
    manualChange = !hasAdvancedFilter;
    manualAdvancedOpen = !showAdvanced;
  }
  const hasAnyActiveFilter = $derived(
    scope !== "all" ||
      branchFilter !== null ||
      Boolean(searchQuery.trim()) ||
      Boolean(pathFilter.trim()) ||
      hasAdvancedFilter,
  );
</script>

<div class="space-y-2 border-b bg-background px-2.5 py-2">
  <div class="flex min-w-0 flex-wrap items-center justify-between gap-1.5">
    <div class="flex min-w-0 flex-wrap items-center gap-1.5">
      {#if onOpenRefDrawer}
        <Button
          size="sm"
          variant="outline"
          aria-label={m.s6_filter_open_ref_drawer_aria()}
          onclick={onOpenRefDrawer}
        >
          <IconGitBranch aria-hidden="true" />
          <span class="max-w-28 truncate">
            {branchFilter ?? m.s6_filter_ref_short()}
          </span>
        </Button>
      {/if}

      <SegmentedControl
        wrap
        value={scope}
        onValueChange={(val) => {
          if (
            val === "all" ||
            val === "branch" ||
            val === "task" ||
            val === "to-push" ||
            val === "incoming"
          ) {
            onScopeChange(val);
          }
        }}
        aria-label={m.s6_filter_scope_aria()}
      >
        <SegmentedControlItem value="all">
          {m.s6_filter_scope_all()}
        </SegmentedControlItem>
        <SegmentedControlItem value="branch">
          {m.s6_log_tree_current_branch()}
        </SegmentedControlItem>
        <SegmentedControlItem value="task">
          {m.s6_filter_scope_task()}
        </SegmentedControlItem>
        <SegmentedControlItem value="to-push">
          {m.s6_filter_scope_to_push()}
        </SegmentedControlItem>
        <SegmentedControlItem value="incoming">
          {m.s6_filter_scope_incoming()}
        </SegmentedControlItem>
      </SegmentedControl>
    </div>

    <div class="flex items-center gap-1">
      <Button
        size="icon-sm"
        variant="ghost"
        aria-pressed={showAdvanced}
        aria-label={m.s6_filter_author_date_label()}
        title={m.s6_filter_author_date_label()}
        onclick={toggleAdvanced}
      >
        <IconFilter aria-hidden="true" />
      </Button>

      {#if onRefreshHistory}
        <Button
          size="icon-sm"
          variant="ghost"
          aria-label={m.s6_filter_refresh_history()}
          title={m.s6_filter_refresh_history()}
          onclick={onRefreshHistory}
        >
          <IconRefresh aria-hidden="true" />
        </Button>
      {/if}

      {#if onResetLayout}
        <Button
          size="icon-sm"
          variant="ghost"
          aria-label={m.s6_filter_reset_layout()}
          title={m.s6_filter_reset_layout()}
          onclick={onResetLayout}
        >
          <IconRestore aria-hidden="true" />
        </Button>
      {/if}
    </div>
  </div>

  <div>
    <div class="grid min-w-0 grid-cols-1 gap-1.5 sm:grid-cols-2">
      <Input
        size="sm"
        value={searchQuery}
        placeholder={m.s6_filter_search_placeholder()}
        aria-label={m.s6_filter_search_aria()}
        oninput={(e) => onSearchChange(e.currentTarget.value)}
      />
      <Input
        size="sm"
        mono
        value={pathFilter}
        placeholder={m.s6_filter_path_placeholder()}
        aria-label={m.s6_filter_path_aria()}
        oninput={(e) => onPathFilterChange(e.currentTarget.value)}
      />
    </div>

    <div bind:this={advancedPanel} hidden={!showAdvanced} inert={!showAdvanced}>
      <div class="grid min-w-0 grid-cols-1 gap-1.5 pt-2 sm:grid-cols-3">
        <Input
          size="sm"
          value={authorFilter}
          placeholder={m.s6_filter_author_placeholder()}
          aria-label={m.s6_filter_author_aria()}
          oninput={(e) => onAuthorChange?.(e.currentTarget.value)}
        />
        <Input
          size="sm"
          type="date"
          value={sinceFilter}
          aria-label={m.s6_filter_since_aria()}
          title={m.s6_filter_since_tooltip()}
          oninput={(e) => onSinceChange?.(e.currentTarget.value)}
        />
        <Input
          size="sm"
          type="date"
          value={untilFilter}
          aria-label={m.s6_filter_until_aria()}
          title={m.s6_filter_until_tooltip()}
          oninput={(e) => onUntilChange?.(e.currentTarget.value)}
        />
      </div>
    </div>
  </div>

  {#if hasAnyActiveFilter && onClearFilters}
    <div class="flex min-w-0 flex-wrap items-center justify-between gap-2">
      <span class="min-w-0 truncate text-xs text-muted-foreground">
        {branchFilter
          ? m.s6_filter_active_with_ref({ ref: branchFilter })
          : m.s6_filter_active_plain()}
      </span>
      <Button size="sm" variant="ghost" onclick={onClearFilters}>
        <IconX aria-hidden="true" />
        {m.s6_filter_reset_all()}
      </Button>
    </div>
  {/if}

  {#if refsChanged && onRefreshHistory}
    <InlineNotice tone="info">
      <span class="mr-2">{m.s6_filter_refs_changed_notice()}</span>
      <Button size="sm" variant="outline" onclick={onRefreshHistory}>
        {m.s6_filter_refresh_btn()}
      </Button>
    </InlineNotice>
  {/if}

  {#if selectionNotice}
    <InlineNotice tone="warning">
      <span class="mr-2">{selectionNotice.message}</span>
      {#if selectionNotice.canJumpHead && onJumpToHead}
        <Button size="sm" variant="outline" onclick={onJumpToHead}>
          {m.s6_filter_jump_to_head()}
        </Button>
      {/if}
      {#if onDismissSelectionNotice}
        <Button size="sm" variant="ghost" onclick={onDismissSelectionNotice}>
          {m.s6_close()}
        </Button>
      {/if}
    </InlineNotice>
  {/if}

  {#if compareBaseMarkOid}
    <InlineNotice tone="info">
      <span class="mr-2">
        {m.s6_filter_compare_mark_before()}<code class="font-mono"
          >{compareBaseMarkOid.slice(0, 7)}</code
        >{m.s6_filter_compare_mark_after()}
      </span>
      {#if onCancelCompareMark}
        <Button size="sm" variant="ghost" onclick={onCancelCompareMark}>
          {m.s6_cancel()}
        </Button>
      {/if}
    </InlineNotice>
  {/if}
</div>
