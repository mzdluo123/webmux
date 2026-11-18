<script lang="ts">
  import type { Repository, Worktree } from "../../contracts/schemas";
  import { getShell, openGitWorkbench } from "../../hooks/shared/shell.svelte";
  import { featureAvailable } from "../../platform/feature-host.svelte";
  import { homeSessionView } from "../../domains/sessions/home-session-view.svelte";
  import SessionSwitcherTrigger from "../shell/SessionSwitcherTrigger.svelte";
  import SessionListScopeMenu from "./SessionListScopeMenu.svelte";
  import WorkspaceActions from "./WorkspaceActions.svelte";

  let {
    refreshing,
    selectedWorktree,
    selectedPinned,
    onworktreedetails,
    ontogglepin,
    onremove,
  }: {
    refreshing: boolean;
    selectedWorktree: Worktree | undefined;
    selectedPinned: boolean;
    onworktreedetails: () => void;
    ontogglepin: (repo: Repository) => void;
    onremove: (tree: Worktree) => void;
  } = $props();

  const shell = getShell();
  const repo = $derived(shell.settingsRepository);
  const git = $derived(featureAvailable("workspace.git-workbench"));
</script>

{#snippet menu()}
  {#if repo}
    <SessionListScopeMenu
      {repo}
      tree={selectedWorktree}
      pinned={selectedPinned}
      onnewworktree={() =>
        shell.start({ kind: "new-worktree", repositoryId: repo.id })}
      onhome={() => shell.chooseHomeWorktrees(repo)}
      onmanage={() => shell.repositoryDetails(repo, "worktrees")}
      onsettings={shell.openRepositorySettings}
      ondetails={() => shell.repositoryDetails(repo)}
      ongit={git
        ? (worktreeId, view) =>
            openGitWorkbench({ repositoryId: repo.id, worktreeId, view })
        : undefined}
      {onworktreedetails}
      onswitch={shell.switchBranch}
      ontogglepin={() => ontogglepin(repo)}
      {onremove}
      onoperation={(operation) => shell.branchOperation(repo.id, operation)}
    />
  {/if}
{/snippet}

<SessionSwitcherTrigger size="icon-sm" />
<WorkspaceActions
  compact={homeSessionView.compact}
  {refreshing}
  onchange={(compact) => (homeSessionView.compact = compact)}
  onrefresh={() => void shell.repositories?.refresh(true)}
  ongit={git
    ? () =>
        openGitWorkbench({
          repositoryId: repo?.id ?? "",
          worktreeId: repo ? selectedWorktree?.id : undefined,
        })
    : undefined}
  menu={repo ? menu : undefined}
/>
