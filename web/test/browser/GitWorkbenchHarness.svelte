<script lang="ts">
  import { untrack } from "svelte";
  import GitWorkbenchShell from "../../src/components/workspace/GitWorkbenchShell.svelte";
  import type {
    GitCommitOutcome,
    GitWorkbenchViewState,
  } from "../../src/contracts/git-workbench-schemas";

  let {
    initialState,
    simulatedCommitOutcome = null,
    onOpenWorktree,
  }: {
    initialState: GitWorkbenchViewState;
    simulatedCommitOutcome?: GitCommitOutcome | null;
    onOpenWorktree?: (worktreeId: string, path: string) => void;
  } = $props();

  let view = $state<GitWorkbenchViewState>(untrack(() => initialState));
</script>

<div class="h-[820px] w-full">
  <GitWorkbenchShell bind:view {simulatedCommitOutcome} {onOpenWorktree} />
</div>
