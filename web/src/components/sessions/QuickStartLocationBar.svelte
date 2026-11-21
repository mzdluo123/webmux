<script lang="ts">
  import { untrack } from "svelte";
  import IconBook2 from "~icons/tabler/book-2";
  import IconCirclePlus from "~icons/tabler/circle-plus";
  import IconGitBranch from "~icons/tabler/git-branch";
  import * as Command from "$lib/components/ui/command";
  import * as Popover from "$lib/components/ui/popover";
  import {
    SegmentedControl,
    SegmentedControlItem,
  } from "$lib/components/ui/segmented-control";
  import { m } from "$lib/paraglide/messages.js";
  import { api } from "../../platform/api";
  import ChoiceDetails from "../shared/ChoiceDetails.svelte";
  import ChatSelect from "../chat/ChatSelect.svelte";
  import QuietTrigger from "../chat/QuietTrigger.svelte";
  import BranchCommandList from "../workspace/BranchCommandList.svelte";
  import PinnedWorktreeMark from "../workspace/PinnedWorktreeMark.svelte";
  import { errorMessage } from "../../platform/error";
  import {
    selectableWorktree,
    worktreeLabel,
  } from "../../domains/sessions/new-session-defaults";
  import { isWorktreePinned } from "../../domains/workspace/worktree-pins";
  import { worktreePins } from "../../domains/workspace/worktree-pins.svelte";
  import {
    resolveQuickStartBranches,
    resolveQuickStartWorktree,
    type QuickStartBranches,
    type QuickStartTargetMode,
  } from "../../hooks/sessions/quick-start";
  import {
    BranchesSchema,
    type Repository,
    type Worktree,
  } from "../../contracts/schemas";

  let {
    compact = false,
    repositories,
    repository,
    sending,
    targetModeChoice,
    worktreeChoice,
    baseChoice,
    uploadError,
    targetMode = $bindable<QuickStartTargetMode>("new"),
    selectedWorktree = $bindable<Worktree | undefined>(),
    branchData = $bindable<QuickStartBranches | undefined>(),
    branchError = $bindable<string | undefined>(),
    base = $bindable(""),
    onrepositorychange,
    ontargetmodechange,
    onselectworktree,
    onselectbranch,
  }: {
    compact?: boolean;
    repositories: readonly Repository[];
    repository?: Repository;
    sending: boolean;
    targetModeChoice?: QuickStartTargetMode;
    worktreeChoice?: { repositoryId: string; worktreeId: string };
    baseChoice?: { repositoryId: string; name: string };
    uploadError: string;
    targetMode?: QuickStartTargetMode;
    selectedWorktree?: Worktree;
    branchData?: QuickStartBranches;
    branchError?: string;
    base?: string;
    onrepositorychange: (id: string) => void;
    ontargetmodechange: (mode: QuickStartTargetMode) => void;
    onselectworktree: (worktreeId: string) => void;
    onselectbranch: (name: string) => void;
  } = $props();

  let branches = $state<{
    id: string;
    data?: QuickStartBranches;
    error?: string;
  }>();
  const repositoryId = $derived(repository?.id);
  $effect(() => {
    const id = repositoryId;
    if (!id || untrack(() => branches?.id) === id) return;
    branches = { id };
    const controller = new AbortController();
    api(
      `/repositories/${id}/branches`,
      BranchesSchema,
      "GET",
      undefined,
      controller.signal,
    )
      .then((data) => {
        if (branches?.id === id) branches = { id, data };
      })
      .catch((err: unknown) => {
        if (branches?.id === id && !controller.signal.aborted)
          branches = { id, error: errorMessage(err) };
      });
    return () => controller.abort();
  });

  const resolvedBranchData = $derived(
    branches?.id === repository?.id ? branches?.data : undefined,
  );
  const resolvedBranches = $derived(
    resolveQuickStartBranches(resolvedBranchData, repository?.id, baseChoice),
  );
  const localBranches = $derived(resolvedBranches.localBranches);
  const remoteBranches = $derived(resolvedBranches.remoteBranches);
  const baseName = $derived(resolvedBranches.baseName);
  const resolvedWorktree = $derived(
    resolveQuickStartWorktree(repository, worktreeChoice, worktreePins.value),
  );
  const worktrees = $derived(resolvedWorktree.worktrees);
  const defaultWorktree = $derived(resolvedWorktree.defaultWorktree);
  const resolvedSelectedWorktree = $derived(resolvedWorktree.selectedWorktree);
  const resolvedTargetMode = $derived<QuickStartTargetMode>(
    targetModeChoice === "existing" && resolvedSelectedWorktree
      ? "existing"
      : "new",
  );
  const branchLoading = $derived(!resolvedBranchData && !branches?.error);
  const branchFailed = $derived(!!branches?.error);

  $effect.pre(() => {
    targetMode = resolvedTargetMode;
    selectedWorktree = resolvedSelectedWorktree;
    branchData = resolvedBranchData;
    branchError = branches?.id === repository?.id ? branches?.error : undefined;
    base = resolvedBranches.base;
  });

  let basePickerOpen = $state(false);
  let baseSearch = $state("");
  let worktreeSearch = $state("");
  const baseBranchesCount = $derived(
    localBranches.length + remoteBranches.length,
  );
  const usableRepository = (repo: Repository) =>
    repo.availability === "available";

  function setBasePickerOpen(next: boolean) {
    basePickerOpen = next;
    if (!next) {
      baseSearch = "";
      worktreeSearch = "";
    }
  }
</script>

<div
  class={[
    "-my-1 flex min-w-0 flex-nowrap items-center gap-0.5 overflow-x-auto px-1 py-1 [scrollbar-width:none]",
    compact && "composer-mobile:hidden",
  ]}
  role="toolbar"
  aria-label={m.s3_qs_repo_and_branch_label()}
>
  <ChatSelect
    labeled
    name={m.s3_repository_label()}
    value={repository?.id ?? ""}
    choices={repositories.map((repo) => ({
      value: repo.id,
      name: repo.name,
      reason: usableRepository(repo)
        ? undefined
        : repo.error || m.s3_repo_currently_unavailable(),
    }))}
    disabled={sending}
    onchange={onrepositorychange}
  >
    {#snippet icon()}<IconBook2 aria-hidden="true" />{/snippet}
  </ChatSelect>
  <Popover.Root bind:open={() => basePickerOpen, setBasePickerOpen}>
    <Popover.Trigger
      disabled={sending || (!baseBranchesCount && !defaultWorktree)}
    >
      {#snippet child({ props })}
        {@const treeLabel = selectedWorktree
          ? worktreeLabel(selectedWorktree)
          : ""}
        <QuietTrigger
          {props}
          label={targetMode === "existing"
            ? m.s3_qs_worktree_value({ label: treeLabel })
            : m.s3_qs_base_branch_value({ branch: baseName })}
          title={targetMode === "existing"
            ? selectedWorktree
              ? m.s3_qs_worktree_title_full({
                  label: treeLabel,
                  path: selectedWorktree.path,
                })
              : m.s3_worktree_label()
            : baseName
              ? m.s3_qs_base_branch_title_full({ branch: baseName })
              : m.s3_base_branch_label()}
          text={targetMode === "existing" ? treeLabel : baseName}
        >
          {#snippet icon()}{#if targetMode === "existing"}<IconGitBranch
                aria-hidden="true"
              />{:else}<IconCirclePlus aria-hidden="true" />{/if}{/snippet}
        </QuietTrigger>
      {/snippet}
    </Popover.Trigger>
    <Popover.Content
      align="start"
      class="max-h-(--bits-popover-content-available-height) w-72 max-w-(--bits-popover-content-available-width) gap-0 overflow-hidden p-0 [&>[data-slot=command]]:min-h-0"
    >
      <div class="shrink-0 border-b px-2 py-1.5">
        <SegmentedControl
          value={targetMode}
          onValueChange={(next) => {
            if (next === "new") ontargetmodechange("new");
            else if (next === "existing" && defaultWorktree)
              ontargetmodechange("existing");
          }}
          aria-label={m.s3_qs_worktree_mode_label()}
          class="w-full"
          wrap
        >
          <SegmentedControlItem value="new" disabled={sending}
            >{m.s3_qs_new_worktree_tab()}</SegmentedControlItem
          >
          <SegmentedControlItem
            value="existing"
            disabled={sending || !defaultWorktree}
            >{m.s3_qs_existing_worktree_tab()}</SegmentedControlItem
          >
        </SegmentedControl>
      </div>
      {#if targetMode === "existing"}
        <Command.Root
          label={m.s3_worktree_label()}
          class="h-auto min-w-0"
          filter={(value, query, keywords) =>
            [value, ...(keywords ?? [])].some((item) =>
              item.toLowerCase().includes(query.trim().toLowerCase()),
            )
              ? 1
              : 0}
          disableInitialScroll
        >
          <Command.Input
            bind:value={worktreeSearch}
            placeholder={m.s3_search_worktrees_placeholder()}
            aria-label={m.s3_search_worktrees_label()}
            autocomplete="off"
            autocapitalize="off"
            autocorrect="off"
            spellcheck={false}
          />
          <Command.List
            aria-label={m.s3_worktree_list_label()}
            class="max-h-[min(14rem,50dvh)] p-1"
          >
            <Command.Empty>{m.s3_no_matching_worktrees()}</Command.Empty>
            {#each worktrees as tree (tree.id)}
              {@const label = worktreeLabel(tree)}
              {@const usable = selectableWorktree(tree)}
              <Command.Item
                value={tree.id}
                keywords={[label, tree.path]}
                disabled={sending || !usable}
                data-checked={selectedWorktree?.id === tree.id}
                onSelect={() => {
                  onselectworktree(tree.id);
                  setBasePickerOpen(false);
                }}
              >
                <IconGitBranch
                  class="text-muted-foreground"
                  aria-hidden="true"
                />
                <ChoiceDetails
                  primary={label}
                  secondary={usable
                    ? tree.path
                    : tree.error || m.s3_worktree_unavailable()}
                />
                {#if repository && isWorktreePinned(worktreePins.value, repository.id, tree.id)}<PinnedWorktreeMark
                    srLabel
                  />{/if}
              </Command.Item>
            {/each}
          </Command.List>
        </Command.Root>
      {:else}
        <BranchCommandList
          label={m.s3_base_branch_label()}
          branches={localBranches}
          {remoteBranches}
          selected={baseName}
          loading={branchLoading}
          failed={branchFailed}
          disabled={sending}
          bind:search={baseSearch}
          onselect={(name) => {
            onselectbranch(name);
            setBasePickerOpen(false);
          }}
        />
      {/if}
    </Popover.Content>
  </Popover.Root>
</div>

{#if uploadError}<p role="alert" class="px-1 text-sm text-destructive">
    {uploadError}
  </p>{/if}
