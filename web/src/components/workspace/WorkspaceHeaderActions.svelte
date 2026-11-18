<script lang="ts">
  import type { Worktree } from "../../contracts/schemas";
  import { getShell, openGitWorkbench } from "../../hooks/shared/shell.svelte";
  import { featureAvailable } from "../../platform/feature-host.svelte";
  import WorkspaceActions from "./WorkspaceActions.svelte";

  let {
    refreshing,
    selectedWorktree,
  }: {
    refreshing: boolean;
    selectedWorktree: Worktree | undefined;
  } = $props();

  const shell = getShell();
  const repo = $derived(shell.settingsRepository);
  const git = $derived(featureAvailable("workspace.git-workbench"));
</script>

<WorkspaceActions
  {refreshing}
  onrefresh={() => void shell.repositories?.refresh(true)}
  ongit={git
    ? () =>
        openGitWorkbench({
          repositoryId: repo?.id ?? "",
          worktreeId: repo ? selectedWorktree?.id : undefined,
        })
    : undefined}
/>
