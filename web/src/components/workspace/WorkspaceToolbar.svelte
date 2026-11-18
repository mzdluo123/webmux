<script lang="ts">
  import type { Snippet } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import IconPlus from "~icons/tabler/plus";
  import { Button } from "$lib/components/ui/button";
  import MainWorktreeMark from "./MainWorktreeMark.svelte";

  let {
    title,
    mainWorktree = false,
    actions,
    loading,
    filtering,
    visibleCount,
    sessionCount,
    repositoryCount,
    showRepositoryCount,
    refreshing,
    onstart,
    startDisabled = false,
    meta,
    children,
  }: {
    title?: string;
    mainWorktree?: boolean;
    /** Icon controls; phones show them in the top bar instead. */
    actions: Snippet;
    loading: boolean;
    filtering: boolean;
    visibleCount: number;
    sessionCount: number;
    repositoryCount: number;
    showRepositoryCount: boolean;
    refreshing: boolean;
    // Opens the new-session dialog; the old header's primary action.
    onstart: () => void;
    startDisabled?: boolean;
    /** Status line under the title, e.g. a worktree's badges. */
    meta?: Snippet;
    // Search and status filters below the title row.
    children: Snippet;
  } = $props();
  const resolvedTitle = $derived(title ?? m.s8_workspace_title());
  const scopeCounts = $derived(
    m.s8_toolbar_repo_session_counts({
      repos: String(repositoryCount),
      sessions: String(sessionCount),
    }),
  );
</script>

<div class="mb-6 space-y-4 border-b pb-4 sm:mb-8 sm:space-y-6">
  <div class="flex flex-wrap items-center gap-x-3 gap-y-2">
    <p role="status" class="sr-only">
      {#if loading}{m.s8_toolbar_loading()}
      {:else if filtering}{m.s8_toolbar_filtering_sessions({
          visible: String(visibleCount),
          total: String(sessionCount),
        })}
      {:else}{scopeCounts}{/if}{#if refreshing}
        {m.s8_toolbar_refreshing_git()}{/if}
    </p>
    <div class="min-w-0 flex-1 sm:mr-3">
      <div class="flex min-w-0 items-center gap-1.5">
        <h1
          class="text-lg leading-tight font-semibold tracking-tight wrap-anywhere sm:text-3xl"
        >
          {resolvedTitle}
        </h1>
        {#if mainWorktree}<MainWorktreeMark srLabel size="md" />{/if}
      </div>
      <p
        aria-hidden="true"
        class="mt-1 hidden text-sm text-muted-foreground sm:block"
      >
        {showRepositoryCount
          ? scopeCounts
          : m.s8_details_total_sessions({ count: String(sessionCount) })}
      </p>
      {#if meta}<div class="empty:hidden sm:mt-2">{@render meta()}</div>{/if}
    </div>
    <div
      class="ml-auto hidden max-w-full flex-wrap items-center justify-end gap-1 sm:flex"
    >
      {@render actions()}
    </div>
    <Button
      class="shrink-0"
      aria-label={m.s8_toolbar_new_session_plus()}
      disabled={startDisabled}
      onclick={() => onstart()}
      ><IconPlus aria-hidden="true" />{m.s8_new_session()}</Button
    >
  </div>
  {@render children()}
</div>
