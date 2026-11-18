<script lang="ts">
  import type { Repository, Worktree } from "../../contracts/schemas";
  import type { BranchOperation } from "../../domains/workspace/branch-actions";
  import SessionSwitcherTrigger from "../shell/SessionSwitcherTrigger.svelte";
  import SessionListScopeMenu from "./SessionListScopeMenu.svelte";
  import WorkspaceActions from "./WorkspaceActions.svelte";

  let {
    compact,
    refreshing,
    repo,
    selectedWorktree,
    selectedPinned,
    oncompactchange,
    onrefresh,
    ongit,
    onnewworktree,
    onhome,
    onmanage,
    onsettings,
    ondetails,
    onscopegit,
    onworktreedetails,
    onswitch,
    ontogglepin,
    onremove,
    onoperation,
  }: {
    compact: boolean;
    refreshing: boolean;
    repo: Repository | undefined;
    selectedWorktree: Worktree | undefined;
    selectedPinned: boolean;
    oncompactchange: (compact: boolean) => void;
    onrefresh: () => void;
    ongit?: () => void;
    onnewworktree: (repo: Repository) => void;
    onhome: (repo: Repository) => void;
    onmanage: (repo: Repository) => void;
    onsettings: (repo: Repository) => void;
    ondetails: (repo: Repository) => void;
    onscopegit?: (
      repo: Repository,
      worktreeId?: string,
      view?: "commit",
    ) => void;
    onworktreedetails: () => void;
    onswitch: (tree: Worktree) => void;
    ontogglepin: (repo: Repository) => void;
    onremove: (tree: Worktree) => void;
    onoperation: (repo: Repository, operation: BranchOperation) => void;
  } = $props();
</script>

{#snippet menu()}
  {#if repo}
    <SessionListScopeMenu
      {repo}
      tree={selectedWorktree}
      pinned={selectedPinned}
      onnewworktree={() => onnewworktree(repo)}
      onhome={() => onhome(repo)}
      onmanage={() => onmanage(repo)}
      onsettings={() => onsettings(repo)}
      ondetails={() => ondetails(repo)}
      ongit={onscopegit
        ? (worktreeId, view) => onscopegit(repo, worktreeId, view)
        : undefined}
      {onworktreedetails}
      {onswitch}
      ontogglepin={() => ontogglepin(repo)}
      {onremove}
      onoperation={(operation) => onoperation(repo, operation)}
    />
  {/if}
{/snippet}

<SessionSwitcherTrigger size="icon-sm" />
<WorkspaceActions
  {compact}
  {refreshing}
  onchange={oncompactchange}
  {onrefresh}
  {ongit}
  menu={repo ? menu : undefined}
/>
