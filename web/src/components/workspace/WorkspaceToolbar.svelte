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
    refreshing,
    onstart,
    startDisabled = false,
    meta,
    children,
  }: {
    title?: string;
    mainWorktree?: boolean;
    /** Routes and controls for the current scope. */
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
  const scopeCounts = $derived(
    m.s8_toolbar_repo_session_counts({
      repos: String(repositoryCount),
      sessions: String(sessionCount),
    }),
  );
</script>

<div class="mb-6 space-y-3 border-b pb-4 sm:mb-8 sm:space-y-4">
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
    <div
      class="min-w-[min(100%,10rem)] flex-1 sm:mr-3 [[data-ui-mode=tv]_&]:flex-initial"
    >
      <div class="flex min-w-0 items-center gap-1.5">
        <h1
          class="text-lg leading-tight font-semibold tracking-tight wrap-anywhere sm:text-3xl"
        >
          {resolvedTitle}
        </h1>
        {#if mainWorktree}<MainWorktreeMark srLabel size="md" />{/if}
      </div>
      {#if meta}<div class="empty:hidden sm:mt-2">{@render meta()}</div>{/if}
    </div>
    <Button
      class="shrink-0"
      aria-label={m.s8_toolbar_new_session_plus()}
      disabled={startDisabled}
      onclick={() => onstart()}
      ><IconPlus aria-hidden="true" />{m.s8_new_session()}</Button
    >
  </div>
  <div class="flex min-w-0 flex-wrap items-center gap-1 border-b">
    <span
      aria-current="page"
      class="flex min-h-9 items-center gap-2 border-b-2 border-foreground px-2 text-sm font-medium"
    >
      {m.s1_sessions()}
      <span class="font-mono text-xs text-muted-foreground tabular-nums">
        {sessionCount}
      </span>
    </span>
    {@render actions()}
  </div>
  {@render children()}
</div>
