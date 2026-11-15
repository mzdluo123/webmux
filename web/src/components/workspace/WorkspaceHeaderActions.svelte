<script lang="ts">
  import IconFolderPlus from "~icons/tabler/folder-plus";
  import * as m from "$lib/paraglide/messages.js";
  import { Button } from "$lib/components/ui/button";
  import type { Repository, Worktree } from "../../contracts/schemas";
  import type { BranchOperation } from "../../domains/workspace/branch-actions";
  import SessionSwitcherTrigger from "../shell/SessionSwitcherTrigger.svelte";
  import SessionListScopeMenu from "./SessionListScopeMenu.svelte";
  import WorkspaceActions from "./WorkspaceActions.svelte";

  let {
    canAddRepository,
    compact,
    refreshing,
    repo,
    selectedWorktree,
    selectedPinned,
    onaddrepository,
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
    canAddRepository: boolean;
    compact: boolean;
    refreshing: boolean;
    repo: Repository | undefined;
    selectedWorktree: Worktree | undefined;
    selectedPinned: boolean;
    onaddrepository: () => void;
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

<Button
  variant="outline"
  size="sm"
  class="max-sm:size-(--control-default)"
  aria-label={m.s1_add_repository()}
  title={m.s1_add_repository()}
  disabled={!canAddRepository}
  onclick={onaddrepository}
  ><IconFolderPlus aria-hidden="true" /><span class="hidden sm:inline"
    >{m.s1_add_repository()}</span
  ></Button
>
<SessionSwitcherTrigger size="icon-sm" />
<WorkspaceActions
  {compact}
  {refreshing}
  onchange={oncompactchange}
  {onrefresh}
  {ongit}
  menu={repo ? menu : undefined}
/>
