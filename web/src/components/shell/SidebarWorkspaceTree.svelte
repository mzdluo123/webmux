<script lang="ts">
  import type {
    ArchivedSession,
    SessionInfo,
    Worktree,
  } from "../../contracts/schemas";
  import { featureContributions } from "../../features/registry.svelte";
  import { featureAvailable } from "../../platform/feature-host.svelte";
  import {
    getShell,
    openGitWorkbench,
    openSession,
  } from "../../hooks/shared/shell.svelte";

  let {
    scopesOnly,
    sessions,
    showScope,
    current,
    configIcons,
    onscope,
    resuming,
    onresume,
    onremove,
  }: {
    scopesOnly: boolean;
    sessions: readonly SessionInfo[];
    showScope: boolean;
    current: string;
    configIcons: ReadonlyMap<string, string>;
    onscope: (repositoryId: string, worktreeId?: string) => void;
    resuming: string;
    onresume: (record: ArchivedSession) => void;
    onremove: (record: ArchivedSession) => void;
  } = $props();
  const workspace = featureContributions.entries.workspace;
  const shell = getShell();
  const setup = $derived(shell.setup.value);
  // Runs a worktree action of the tree, by the IDs its rows carry.
  function withTree(action: (tree: Worktree) => void) {
    return (repositoryId: string, worktreeId: string) => {
      const tree = shell.repositories?.list
        .find((r) => r.id === repositoryId)
        ?.worktrees.find((w) => w.id === worktreeId);
      if (tree) action(tree);
    };
  }
</script>

{#if workspace && shell.repositories}<workspace.Tree
    repositories={shell.repositories.list}
    {scopesOnly}
    {sessions}
    home={workspace.homeWorktrees.value}
    scope={showScope ? (shell.repositories.choice ?? "all") : ""}
    worktree={showScope ? (shell.repositories.worktree ?? "") : ""}
    {current}
    canCreate={setup.gitReady && setup.registryReady}
    {configIcons}
    {onscope}
    onopen={openSession}
    onnewworktree={(repositoryId) =>
      shell.start({ kind: "new-worktree", repositoryId })}
    onhomeworktrees={(repositoryId) => {
      const repo = shell.repositories?.list.find((r) => r.id === repositoryId);
      if (repo) shell.chooseHomeWorktrees(repo);
    }}
    onmanageworktrees={(repositoryId) => {
      const repo = shell.repositories?.list.find((r) => r.id === repositoryId);
      if (repo) shell.repositoryDetails(repo, "worktrees");
    }}
    onrepositorysettings={(repositoryId) => {
      const repo = shell.repositories?.list.find((r) => r.id === repositoryId);
      if (repo) shell.openRepositorySettings(repo);
    }}
    onrepositorydetails={(repositoryId) => {
      const repo = shell.repositories?.list.find((r) => r.id === repositoryId);
      if (repo) shell.repositoryDetails(repo);
    }}
    onworktreedetails={withTree((tree) => shell.worktreeDetails(tree))}
    onremoveworktree={withTree((tree) => shell.removeWorktree(tree))}
    onswitchbranch={withTree((tree) => shell.switchBranch(tree))}
    onbranchoperation={shell.branchOperation}
    onopengit={featureAvailable("workspace.git-workbench")
      ? (repositoryId, worktreeId, view) =>
          openGitWorkbench({ repositoryId, worktreeId, view })
      : undefined}
    onaddrepository={() => shell.addRepository()}
    onrename={shell.rename}
    onclose={shell.close}
    records={shell.archived?.records ?? []}
    {resuming}
    {onresume}
    {onremove}
  />{/if}
