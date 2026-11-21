<script lang="ts">
  import type { Snippet } from "svelte";
  import { m } from "$lib/paraglide/messages.js";
  import IconPlus from "~icons/tabler/plus";
  import { Button } from "$lib/components/ui/button";
  import MainWorktreeMark from "./MainWorktreeMark.svelte";
  import PinnedWorktreeMark from "./PinnedWorktreeMark.svelte";

  let {
    title,
    mainWorktree = false,
    pinnedWorktree = false,
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
    pinnedWorktree?: boolean;
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

<div class="mb-6 space-y-3 sm:mb-7 sm:space-y-4">
  <p role="status" class="sr-only">
    {#if loading}{m.s8_toolbar_loading()}
    {:else if filtering}{m.s8_toolbar_filtering_sessions({
        visible: String(visibleCount),
        total: String(sessionCount),
      })}
    {:else}{scopeCounts}{/if}{#if refreshing}
      {m.s8_toolbar_refreshing_git()}{/if}
  </p>
  <div class="flex flex-wrap items-center gap-3">
    <div class="min-w-[min(100%,6rem)] flex-1">
      <div class="flex min-w-0 items-center gap-1.5">
        <h1
          class="text-xl leading-tight font-semibold tracking-tight wrap-anywhere sm:text-2xl"
        >
          {resolvedTitle}
        </h1>
        {#if mainWorktree}
          <span class="hidden sm:inline-flex">
            <MainWorktreeMark srLabel size="md" />
          </span>
        {/if}
        {#if pinnedWorktree}<PinnedWorktreeMark srLabel size="md" />{/if}
      </div>
      {#if meta}<div class="empty:hidden sm:mt-2">{@render meta()}</div>{/if}
    </div>
    <div
      class="ml-auto flex max-w-full shrink-0 flex-wrap items-center justify-end gap-1"
    >
      {@render actions()}
      <Button
        size="sm-touch"
        aria-label={m.s8_toolbar_new_session_plus()}
        title={m.s8_new_session()}
        disabled={startDisabled}
        onclick={() => onstart()}
      >
        <IconPlus aria-hidden="true" />
        <span class="max-sm:sr-only">{m.s8_new_session()}</span>
      </Button>
    </div>
  </div>
  {@render children()}
</div>
