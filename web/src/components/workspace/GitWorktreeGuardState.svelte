<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import IconAlertTriangle from "~icons/tabler/alert-triangle";
  import IconFolderPlus from "~icons/tabler/folder-plus";
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconHistory from "~icons/tabler/history";
  import IconPlus from "~icons/tabler/plus";
  import IconSettings from "~icons/tabler/settings";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import type { Repository, Worktree } from "../../contracts/schemas";
  import { worktreeTitle } from "../../domains/workspace/home-worktrees";

  let {
    kind,
    repositories = [],
    repository,
    worktree,
    canAddRepository = true,
    onAddRepository,
    onSelectRepository,
    onSelectWorktree,
    onNewWorktree,
    onManageWorktrees,
    onBackToLog,
  }: {
    kind:
      | "empty-registry"
      | "missing-repository"
      | "unavailable-repository"
      | "pick-worktree"
      | "broken-worktree";
    repositories?: readonly Repository[];
    repository?: Repository;
    worktree?: Worktree;
    canAddRepository?: boolean;
    onAddRepository: () => void;
    onSelectRepository: (repositoryId: string) => void;
    onSelectWorktree: (worktreeId?: string) => void;
    onNewWorktree: () => void;
    onManageWorktrees: () => void;
    onBackToLog?: () => void;
  } = $props();

  const writableWorktrees = $derived(
    (repository?.worktrees ?? []).filter(
      (wt) => wt.available && !wt.prunable && !wt.locked && wt.kind !== "bare",
    ),
  );
</script>

{#snippet writableList(showDirty: boolean)}
  <div class="space-y-1">
    {#each writableWorktrees as wt (wt.id)}
      <ListButton variant="compact-row" onclick={() => onSelectWorktree(wt.id)}>
        <span
          class={`min-w-0 flex-1 truncate ${showDirty ? "font-medium" : ""}`}
        >
          {worktreeTitle(wt)}
        </span>
        {#if showDirty && wt.dirty}
          <Badge variant="warning">{m.s7_guard_dirty_badge()}</Badge>
        {/if}
        <span class="max-w-48 truncate font-mono text-xs text-muted-foreground">
          {wt.path}
        </span>
      </ListButton>
    {/each}
  </div>
{/snippet}

<ScrollArea class="h-full min-h-8 flex-1" data-testid={`git-guard-${kind}`}>
  <div class="flex min-h-full items-center justify-center p-4 sm:p-8">
    <div
      class="w-full max-w-lg space-y-4 rounded-lg border bg-background p-5 shadow-xs"
    >
      {#if kind === "empty-registry"}
        <div class="space-y-1.5">
          <div class="flex items-center gap-2 text-sm font-semibold">
            <IconFolderPlus
              class="size-5 text-muted-foreground"
              aria-hidden="true"
            />
            <h2>{m.s7_guard_empty_title()}</h2>
          </div>
          <p class="text-xs leading-relaxed text-muted-foreground">
            {m.s7_guard_empty_desc()}
          </p>
        </div>
        {#if canAddRepository}
          <div class="flex flex-wrap gap-2">
            <Button size="sm" variant="default" onclick={onAddRepository}>
              <IconPlus aria-hidden="true" />
              <span>{m.s7_register_existing_repo_btn()}</span>
            </Button>
          </div>
        {/if}
      {:else if kind === "missing-repository"}
        <div class="space-y-1.5">
          <div class="flex items-center gap-2 text-sm font-semibold">
            <IconAlertTriangle class="size-5 text-warning" aria-hidden="true" />
            <h2>{m.s7_guard_missing_repo_title()}</h2>
          </div>
          <p class="text-xs leading-relaxed text-muted-foreground">
            {m.s7_guard_missing_repo_desc()}
          </p>
        </div>
        {#if repositories.length > 0}
          <div class="space-y-1">
            {#each repositories as repo (repo.id)}
              <ListButton
                variant="compact-row"
                onclick={() => onSelectRepository(repo.id)}
              >
                <span class="min-w-0 flex-1 truncate">{repo.name}</span>
                <span class="truncate font-mono text-xs text-muted-foreground">
                  {repo.anchorPath}
                </span>
              </ListButton>
            {/each}
          </div>
        {/if}
        {#if canAddRepository}
          <div class="flex flex-wrap gap-2">
            <Button size="sm" variant="outline" onclick={onAddRepository}>
              <IconPlus aria-hidden="true" />
              <span>{m.s7_register_existing_repo_btn()}</span>
            </Button>
          </div>
        {/if}
      {:else if kind === "unavailable-repository"}
        <div class="space-y-1.5">
          <div class="flex items-center gap-2 text-sm font-semibold">
            <IconAlertTriangle
              class="size-5 text-destructive"
              aria-hidden="true"
            />
            <h2>
              {m.s7_guard_unavail_repo_title({ name: repository?.name ?? "" })}
            </h2>
          </div>
          <p class="text-xs leading-relaxed text-muted-foreground">
            {repository?.error || m.s7_guard_unavail_repo_desc()}
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <Button size="sm" variant="outline" onclick={onManageWorktrees}>
            <IconSettings aria-hidden="true" />
            <span>{m.s7_guard_view_repo_details_btn()}</span>
          </Button>
        </div>
      {:else if kind === "pick-worktree"}
        <div class="space-y-1.5">
          <div class="flex items-center gap-2 text-sm font-semibold">
            <IconGitBranch class="size-5 text-info" aria-hidden="true" />
            <h2>{m.s7_guard_pick_wt_title()}</h2>
          </div>
          <p class="text-xs leading-relaxed text-muted-foreground">
            {#if worktree?.kind === "bare"}
              {m.s7_guard_pick_wt_bare_desc()}
            {:else}
              {m.s7_guard_pick_wt_readonly_desc()}
            {/if}
          </p>
        </div>
        {#if writableWorktrees.length > 0}
          {@render writableList(true)}
        {:else}
          <p class="text-xs text-muted-foreground">
            {m.s7_guard_no_writable_wt_hint()}
          </p>
        {/if}
        <div class="flex flex-wrap gap-2">
          <Button size="sm" variant="default" onclick={onNewWorktree}>
            <IconPlus aria-hidden="true" />
            <span>{m.s7_new_worktree_btn()}</span>
          </Button>
          {#if onBackToLog}
            <Button size="sm" variant="outline" onclick={onBackToLog}>
              <IconHistory aria-hidden="true" />
              <span>{m.s7_guard_back_to_log_btn()}</span>
            </Button>
          {/if}
        </div>
      {:else}
        <div class="space-y-1.5">
          <div class="flex items-center gap-2 text-sm font-semibold">
            <IconAlertTriangle class="size-5 text-warning" aria-hidden="true" />
            <h2>{m.s7_guard_broken_wt_title()}</h2>
          </div>
          <p class="text-xs leading-relaxed text-muted-foreground">
            {#if !worktree}
              {m.s7_guard_broken_wt_missing_desc()}
            {:else if worktree.locked}
              {m.s7_guard_broken_wt_locked_desc({ path: worktree.path })}
            {:else}
              {m.s7_guard_broken_wt_prunable_desc({ path: worktree.path })}
            {/if}
          </p>
        </div>
        {#if writableWorktrees.length > 0}
          {@render writableList(false)}
        {/if}
        <div class="flex flex-wrap gap-2">
          <Button size="sm" variant="outline" onclick={onManageWorktrees}>
            <IconSettings aria-hidden="true" />
            <span>{m.s7_guard_manage_repair_wt_btn()}</span>
          </Button>
          <Button
            size="sm"
            variant="outline"
            onclick={() => onSelectWorktree(undefined)}
          >
            <IconHistory aria-hidden="true" />
            <span>{m.s7_guard_switch_readonly_btn()}</span>
          </Button>
        </div>
      {/if}
    </div>
  </div>
</ScrollArea>
