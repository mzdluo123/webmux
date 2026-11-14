<script lang="ts">
  import type { Repository, Worktree } from "../../contracts/schemas";
  import type { BranchOperation } from "../../domains/workspace/branch-actions";
  import { worktreeTitle } from "../../domains/workspace/home-worktrees";
  import {
    hasWorktreeChanges,
    switchableWorktree,
  } from "../../domains/workspace/worktree-sync";
  import WorkspaceMenu from "./WorkspaceMenu.svelte";

  let {
    repo,
    tree,
    pinned = false,
    onnewworktree,
    onhome,
    onmanage,
    onsettings,
    ondetails,
    ongit,
    onworktreedetails,
    onswitch,
    ontogglepin,
    onremove,
    onoperation,
  }: {
    repo: Repository;
    tree?: Worktree;
    pinned?: boolean;
    onnewworktree: () => void;
    onhome: () => void;
    onmanage: () => void;
    onsettings: () => void;
    ondetails: () => void;
    ongit?: (worktreeId?: string, view?: "commit") => void;
    onworktreedetails: () => void;
    onswitch: (tree: Worktree) => void;
    ontogglepin: () => void;
    onremove: (tree: Worktree) => void;
    onoperation: (operation: BranchOperation) => void;
  } = $props();
</script>

<WorkspaceMenu
  repository={repo.name}
  unavailable={repo.availability !== "available"}
  {onnewworktree}
  {onhome}
  {onmanage}
  {onsettings}
  {ondetails}
  worktree={tree
    ? {
        title: worktreeTitle(tree),
        ondetails: onworktreedetails,
        onswitch: switchableWorktree(tree) ? () => onswitch(tree) : undefined,
        pinned,
        ontogglepin,
        onremove: tree.kind === "linked" ? () => onremove(tree) : undefined,
        tree,
        onoperation,
        oncommit:
          ongit &&
          tree.available &&
          !tree.prunable &&
          !tree.locked &&
          tree.kind !== "bare" &&
          hasWorktreeChanges(tree)
            ? () => ongit(tree.id, "commit")
            : undefined,
      }
    : undefined}
/>
