<script lang="ts">
  import type { Snippet } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
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
</script>

<div class="mb-5 space-y-3 border-b pb-4">
  <div
    class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 sm:gap-x-1"
  >
    <p role="status" class="sr-only">
      {#if loading}{m.s8_toolbar_loading()}
      {:else if filtering}{m.s8_toolbar_filtering_sessions({
          visible: String(visibleCount),
          total: String(sessionCount),
        })}
      {:else}{m.s8_toolbar_repo_session_counts({
          repos: String(repositoryCount),
          sessions: String(sessionCount),
        })}{/if}{#if refreshing}
        {m.s8_toolbar_refreshing_git()}{/if}
    </p>
    <div class="min-w-0 sm:mr-3">
      <!-- Phones already show the title in the top bar. -->
      <div class="hidden min-w-0 items-center gap-1.5 sm:flex">
        <h1 class="min-w-0 text-lg font-semibold tracking-tight break-all">
          {resolvedTitle}
        </h1>
        {#if mainWorktree}<MainWorktreeMark srLabel size="md" />{/if}
      </div>
      {#if meta}<div class="sm:mt-1">{@render meta()}</div>{/if}
    </div>
    <div
      class="ml-auto hidden max-w-full items-center justify-end gap-1 sm:flex"
    >
      {@render actions()}
    </div>
    <Button
      size="sm"
      class="order-last min-h-9 w-full sm:order-none sm:min-h-8 sm:w-auto"
      disabled={startDisabled}
      onclick={() => onstart()}>{m.s8_toolbar_new_session_plus()}</Button
    >
  </div>
  {@render children()}
</div>
