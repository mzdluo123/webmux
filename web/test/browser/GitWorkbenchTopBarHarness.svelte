<script lang="ts">
  import GitOperationsPopover from "../../src/components/workspace/GitOperationsPopover.svelte";
  import GitWorkbenchTopBar from "../../src/components/workspace/GitWorkbenchTopBar.svelte";
  import type {
    GitBranchItem,
    GitOperation,
    GitWorktreeContext,
  } from "../../src/contracts/git-workbench-schemas";
  import type {
    Repository,
    SessionInfo,
    Worktree,
  } from "../../src/contracts/schemas";

  let {
    repositories,
    repository,
    worktreeId,
    worktree,
    context,
    branches = [],
    operations = [],
    returnSession,
    reworkCount = 0,
    onSelectRepository,
    onSelectWorktree,
  }: {
    repositories: readonly Repository[];
    repository: Repository | undefined;
    worktreeId?: string;
    worktree: Worktree | undefined;
    context: GitWorktreeContext;
    branches?: readonly GitBranchItem[];
    operations?: readonly GitOperation[];
    returnSession?: SessionInfo;
    reworkCount?: number;
    onSelectRepository: (repositoryId: string) => void;
    onSelectWorktree: (worktreeId?: string) => void;
  } = $props();
</script>

<div class="git-workbench bg-background text-foreground">
  {#snippet operationLedger()}
    <GitOperationsPopover worktreePath={context.worktreePath} {operations} />
  {/snippet}

  <GitWorkbenchTopBar
    operations={operationLedger}
    {repositories}
    {repository}
    {worktreeId}
    {worktree}
    {context}
    {branches}
    branchFilter={null}
    returnSessionId={returnSession?.id}
    {returnSession}
    sessionsReady={true}
    {reworkCount}
    onOpenRework={() => {}}
    {onSelectRepository}
    {onSelectWorktree}
    onAddRepository={() => {}}
    onNewWorktree={() => {}}
    onManageWorktrees={() => {}}
    onBranchFilterChange={() => {}}
    onBranchAction={() => {}}
    onFetch={() => {}}
    onUpdate={() => {}}
    onPush={() => {}}
    onReturnSession={() => {}}
  />
</div>
