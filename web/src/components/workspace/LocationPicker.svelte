<script lang="ts">
  import { tick } from "svelte";
  import * as m from "$lib/paraglide/messages.js";
  import IconCirclePlus from "~icons/tabler/circle-plus";
  import IconFolder from "~icons/tabler/folder";
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconPencil from "~icons/tabler/pencil";
  import IconTrash from "~icons/tabler/trash";
  import { Button } from "$lib/components/ui/button";
  import * as Command from "$lib/components/ui/command";
  import { Label } from "$lib/components/ui/label";
  import ChoiceDetails from "../shared/ChoiceDetails.svelte";
  import DirectoryInput from "../shared/DirectoryInput.svelte";
  import {
    getPluginContributions,
    type CreationLocationProps,
  } from "../../plugins/context";
  import PickerField from "../shared/PickerField.svelte";
  import {
    directoryName,
    selectableWorktree,
    worktreeLabel,
  } from "../../domains/sessions/new-session-defaults";
  import type { Repository, Worktree } from "../../contracts/schemas";
  import { pluginAvailable } from "../../platform/plugin-host.svelte";

  // The "where" row of the new-session dialog: recent directories, worktrees
  // of every repository, a typed or browsed directory, and "new worktree".
  let {
    repositories,
    recent,
    preferredRepositoryId,
    createdTree,
    newWorktreeSummary,
    onremove,
    scope,
    kind = $bindable(),
    cwd = $bindable(),
    repositoryId = $bindable(),
    worktreeId = $bindable(),
    disabled = false,
    projectScoped = false,
  }: CreationLocationProps = $props();
  const providers = getPluginContributions().creation;
  let open = $state(false);
  let search = $state("");
  // The desktop list stays mounted while closed; each opening starts unfiltered.
  function setOpen(next: boolean) {
    open = next;
    if (!next) search = "";
  }
  let cwdInput = $state<HTMLInputElement | null>(null);

  function treesOf(repo: Repository) {
    return createdTree?.repositoryId === repo.id &&
      !repo.worktrees.some((tree) => tree.id === createdTree?.id)
      ? [...repo.worktrees, createdTree]
      : repo.worktrees;
  }
  function scopedTrees(repo: Repository) {
    const trees = treesOf(repo);
    return scope?.worktreeId
      ? trees.filter((tree) => tree.id === scope.worktreeId)
      : trees;
  }
  const ordered = $derived(
    scope || projectScoped
      ? repositories.filter(
          (repo) => repo.id === (scope?.repositoryId ?? repositoryId),
        )
      : [
          ...repositories.filter((repo) => repo.id === preferredRepositoryId),
          ...repositories.filter((repo) => repo.id !== preferredRepositoryId),
        ],
  );
  const repository = $derived(
    repositories.find((repo) => repo.id === repositoryId),
  );
  const worktree = $derived(
    repository && treesOf(repository).find((tree) => tree.id === worktreeId),
  );

  function chooseTree(repo: Repository, tree: Worktree) {
    kind = "worktree";
    repositoryId = repo.id;
    worktreeId = tree.id;
    setOpen(false);
  }
  function chooseDirectory(dir: string) {
    kind = "directory";
    cwd = dir;
    setOpen(false);
  }
  async function typeDirectory() {
    kind = "directory";
    setOpen(false);
    await tick();
    cwdInput?.focus();
    cwdInput?.select();
  }
  function chooseNewWorktree(repo: Repository) {
    kind = "new-worktree";
    repositoryId = repo.id;
    setOpen(false);
  }
</script>

{#snippet treeGroup(repo: Repository)}
  <Command.Group
    heading={projectScoped ? m.s8_loc_project_workspaces() : repo.name}
    value="repo:{repo.id}"
  >
    {#each scopedTrees(repo) as tree (tree.id)}
      <Command.Item
        value="tree:{repo.id}:{tree.id}"
        keywords={[repo.name, worktreeLabel(tree), tree.path]}
        disabled={!selectableWorktree(tree) ||
          repo.availability !== "available"}
        data-checked={kind === "worktree" &&
          repositoryId === repo.id &&
          worktreeId === tree.id}
        onSelect={() => chooseTree(repo, tree)}
      >
        <IconGitBranch class="text-muted-foreground" aria-hidden="true" />
        <ChoiceDetails primary={worktreeLabel(tree)} secondary={tree.path} />
        {#if onremove && tree.kind === "linked"}
          <Button
            type="button"
            variant="quiet"
            size="icon-sm"
            class="pointer-events-auto"
            title={m.s8_loc_delete_worktree({ label: worktreeLabel(tree) })}
            aria-label={m.s8_loc_delete_worktree({
              label: worktreeLabel(tree),
            })}
            onkeydown={(event) => {
              // The command root rewrites Enter into selecting the row.
              if (event.key === "Enter") event.stopPropagation();
            }}
            onclick={(event) => {
              event.stopPropagation();
              setOpen(false);
              onremove(tree);
            }}><IconTrash class="size-4" aria-hidden="true" /></Button
          >
        {/if}
      </Command.Item>
    {/each}
    <Command.Item
      value="new:{repo.id}"
      keywords={[repo.name, m.s8_loc_new_worktree(), "worktree"]}
      disabled={repo.availability !== "available"}
      data-checked={kind === "new-worktree" && repositoryId === repo.id}
      onSelect={() => chooseNewWorktree(repo)}
    >
      <IconCirclePlus class="text-muted-foreground" aria-hidden="true" />
      <span class="text-sm"
        >{scope?.worktreeId
          ? m.s8_loc_new_from_current_wt()
          : m.s8_menu_new_worktree()}</span
      >
    </Command.Item>
  </Command.Group>
{/snippet}

<div class="space-y-2">
  <Label for="session-location"
    >{projectScoped
      ? m.s8_loc_work_location_label()
      : m.s8_loc_location_label()}</Label
  >
  <PickerField id="session-location" bind:open={() => open, setOpen} {disabled}>
    {#snippet trigger()}
      {#if kind === "directory"}
        <IconFolder
          class="size-4 shrink-0 text-muted-foreground"
          aria-hidden="true"
        />
        <ChoiceDetails
          primary={m.s8_loc_direct_dir()}
          secondary={projectScoped ? cwd : ""}
        />
      {:else if kind === "new-worktree"}
        <IconCirclePlus
          class="size-4 shrink-0 text-muted-foreground"
          aria-hidden="true"
        />
        <ChoiceDetails
          primary={repository && !projectScoped
            ? m.s8_loc_new_worktree_in_repo({ name: repository.name })
            : m.s8_loc_new_worktree()}
          secondary={newWorktreeSummary}
        />
      {:else}
        <IconGitBranch
          class="size-4 shrink-0 text-muted-foreground"
          aria-hidden="true"
        />
        {#if worktree}
          <ChoiceDetails
            primary={`${projectScoped ? "" : (repository?.name ?? "") + " · "}${worktreeLabel(worktree)}`}
            secondary={worktree.path}
          />
        {:else}
          <span class="grid min-w-0 flex-1 gap-0.5">
            <span class="text-sm text-muted-foreground"
              >{m.s8_loc_select_worktree()}</span
            >
            {#if repository}<span class="truncate text-xs text-muted-foreground"
                >{m.s8_loc_no_available_worktrees({
                  name: repository.name,
                })}</span
              >{/if}
          </span>
        {/if}
      {/if}
    {/snippet}
    <Command.Root label={m.s8_loc_location_label()} class="h-auto min-w-0">
      <Command.Input
        bind:value={search}
        placeholder={m.s8_loc_search_placeholder()}
        aria-label={m.s8_loc_search_aria()}
        autocomplete="off"
        autocapitalize="off"
        autocorrect="off"
        spellcheck={false}
      />
      <Command.List
        aria-label={m.s8_loc_list_aria()}
        class="max-h-[min(20rem,50dvh)] p-1"
      >
        <Command.Empty>{m.s8_loc_empty()}</Command.Empty>
        {#if ordered[0]?.id === preferredRepositoryId}
          {@render treeGroup(ordered[0])}
        {/if}
        {#if recent.length && !scope && !projectScoped}
          <Command.Group heading={m.s8_loc_recent_dirs()} value="recent">
            {#each recent as dir (dir)}
              <Command.Item
                value="dir:{dir}"
                keywords={[dir]}
                data-checked={kind === "directory" && cwd === dir}
                onSelect={() => chooseDirectory(dir)}
              >
                <IconFolder class="text-muted-foreground" aria-hidden="true" />
                <ChoiceDetails primary={directoryName(dir)} secondary={dir} />
              </Command.Item>
            {/each}
          </Command.Group>
        {/if}
        {#each ordered.filter((repo) => repo.id !== preferredRepositoryId) as repo (repo.id)}
          {@render treeGroup(repo)}
        {/each}
        {#if !scope && !projectScoped}
          <Command.Group heading={m.s8_loc_other()} value="other">
            <Command.Item
              value="action:directory"
              keywords={[m.s8_loc_dir_keyword(), "directory"]}
              onSelect={typeDirectory}
            >
              <IconPencil class="text-muted-foreground" aria-hidden="true" />
              <span class="text-sm">{m.s8_loc_enter_or_browse_dir()}</span>
            </Command.Item>
          </Command.Group>
        {/if}
      </Command.List>
    </Command.Root>
  </PickerField>
  {#if kind === "directory" && !projectScoped}
    <DirectoryInput
      bind:ref={cwdInput}
      bind:value={cwd}
      placeholder="~/projects"
      Picker={pluginAvailable("files.upload")
        ? providers.DirectoryPicker
        : undefined}
    />
  {/if}
</div>
