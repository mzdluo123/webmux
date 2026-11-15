<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
  import IconChevronDown from "~icons/tabler/chevron-down";
  import IconFolder from "~icons/tabler/folder";
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconPlus from "~icons/tabler/plus";
  import IconSettings from "~icons/tabler/settings";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import type { Repository, Worktree } from "../../contracts/schemas";
  import { worktreeTitle } from "../../domains/workspace/home-worktrees";

  const READONLY_WORKTREE = "__readonly__";

  let {
    repositories,
    repository,
    worktreeId,
    worktree,
    canAddRepository = true,
    onSelectRepository,
    onSelectWorktree,
    onAddRepository,
    onNewWorktree,
    onManageWorktrees,
  }: {
    repositories: readonly Repository[];
    repository: Repository | undefined;
    worktreeId?: string;
    worktree: Worktree | undefined;
    canAddRepository?: boolean;
    onSelectRepository: (repositoryId: string) => void;
    onSelectWorktree: (worktreeId?: string) => void;
    onAddRepository: () => void;
    onNewWorktree: () => void;
    onManageWorktrees: () => void;
  } = $props();

  const repoAvailable = $derived(repository?.availability === "available");
  const repositoryButtonLabel = $derived(
    repository?.name ??
      (repositories.length === 0
        ? m.s7_switcher_no_repo()
        : m.s7_switcher_select_repo()),
  );
  const worktreeButtonLabel = $derived(
    !worktreeId
      ? m.s7_readonly_repo_no_wt()
      : worktree
        ? worktreeTitle(worktree)
        : m.s7_switcher_wt_missing_label(),
  );
</script>

<DropdownMenu.Root>
  <DropdownMenu.Trigger>
    {#snippet child({ props })}
      <Button
        {...props}
        size="sm"
        variant="outline"
        aria-label={m.s7_switcher_switch_repo_aria()}
        title={repository?.anchorPath ?? m.s7_switcher_switch_repo_aria()}
      >
        <IconFolder aria-hidden="true" />
        <span class="max-w-28 truncate sm:max-w-44">
          {repositoryButtonLabel}
        </span>
        <IconChevronDown class="text-muted-foreground" aria-hidden="true" />
      </Button>
    {/snippet}
  </DropdownMenu.Trigger>
  <DropdownMenu.Content
    align="start"
    class="w-64"
    data-testid="git-repository-switcher-popover"
  >
    <DropdownMenu.Label>{m.s7_switcher_registered_repos()}</DropdownMenu.Label>
    {#if repositories.length === 0}
      <DropdownMenu.Item disabled
        >{m.s7_switcher_no_registered_repos()}</DropdownMenu.Item
      >
    {:else}
      <DropdownMenu.RadioGroup
        value={repository?.id ?? ""}
        onValueChange={(id) => {
          if (id) onSelectRepository(id);
        }}
      >
        {#each repositories as repo (repo.id)}
          <DropdownMenu.RadioItem value={repo.id} closeOnSelect>
            <span class="min-w-0 flex-1 truncate">{repo.name}</span>
            {#if repo.availability !== "available"}
              <Badge variant="destructive">{m.s7_unavailable_badge()}</Badge>
            {/if}
          </DropdownMenu.RadioItem>
        {/each}
      </DropdownMenu.RadioGroup>
    {/if}
    {#if canAddRepository}
      <DropdownMenu.Separator />
      <DropdownMenu.Item onSelect={onAddRepository}>
        <IconPlus aria-hidden="true" />
        <span>{m.s7_register_existing_repo_btn()}</span>
      </DropdownMenu.Item>
    {/if}
  </DropdownMenu.Content>
</DropdownMenu.Root>

{#if repository}
  <DropdownMenu.Root>
    <DropdownMenu.Trigger>
      {#snippet child({ props })}
        <Button
          {...props}
          size="sm"
          variant="outline"
          aria-label={m.s7_switcher_switch_wt_aria()}
          title={worktree?.path ?? m.s7_switcher_switch_wt_aria()}
        >
          <IconGitBranch aria-hidden="true" />
          <span class="max-w-28 truncate sm:max-w-44">
            {worktreeButtonLabel}
          </span>
          <IconChevronDown class="text-muted-foreground" aria-hidden="true" />
        </Button>
      {/snippet}
    </DropdownMenu.Trigger>
    <DropdownMenu.Content
      align="start"
      class="w-72"
      data-testid="git-worktree-switcher-popover"
    >
      <DropdownMenu.Label
        >{m.s7_switcher_target_wt_heading()}</DropdownMenu.Label
      >
      <DropdownMenu.RadioGroup
        value={worktreeId ?? READONLY_WORKTREE}
        onValueChange={(value) =>
          onSelectWorktree(value === READONLY_WORKTREE ? undefined : value)}
      >
        <DropdownMenu.RadioItem value={READONLY_WORKTREE} closeOnSelect>
          <span class="min-w-0 flex-1 truncate"
            >{m.s7_switcher_readonly_view_item()}</span
          >
          <Badge variant="outline">{m.s7_readonly_badge()}</Badge>
        </DropdownMenu.RadioItem>
        {#each repository.worktrees as wt (wt.id)}
          <DropdownMenu.RadioItem value={wt.id} closeOnSelect>
            <span class="min-w-0 flex-1 truncate">{worktreeTitle(wt)}</span>
            {#if !wt.available || wt.prunable}
              <Badge variant="destructive">{m.s7_wt_missing_badge()}</Badge>
            {:else if wt.kind === "bare"}
              <Badge variant="warning">{m.s7_wt_bare_badge()}</Badge>
            {:else if wt.locked}
              <Badge variant="warning">{m.s7_wt_locked_badge()}</Badge>
            {:else if wt.kind === "main"}
              <Badge variant="outline">{m.s7_wt_main_badge()}</Badge>
            {/if}
          </DropdownMenu.RadioItem>
        {/each}
      </DropdownMenu.RadioGroup>
      <DropdownMenu.Separator />
      {#if repoAvailable}
        <DropdownMenu.Item onSelect={onNewWorktree}>
          <IconPlus aria-hidden="true" />
          <span>{m.s7_new_worktree_btn()}</span>
        </DropdownMenu.Item>
      {/if}
      <DropdownMenu.Item onSelect={onManageWorktrees}>
        <IconSettings aria-hidden="true" />
        <span>{m.s7_manage_worktrees_btn()}</span>
      </DropdownMenu.Item>
    </DropdownMenu.Content>
  </DropdownMenu.Root>
{/if}
