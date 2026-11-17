<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { worktreeTitle } from "../../domains/workspace/home-worktrees";
  import type { WorktreePreset } from "../../domains/sessions/worktree-preset";
  import type { Repository, Worktree } from "../../contracts/schemas";
  import { ResponsiveDialog } from "$lib/components/ui/responsive-dialog";
  import SwitchBranch, { type SwitchSource } from "./SwitchBranch.svelte";

  // The switch of a worktree's branch outside the repository panel (home
  // page, sidebar, Git workbench): a dialog on desktop, a bottom drawer on
  // phones. Every way out also closes it, so only one dialog is ever on screen;
  // the host reads its own state in the handler before the close clears it.
  let {
    repository,
    worktree,
    initialBranch,
    initialMode,
    initialStartPoint,
    source,
    onchange,
    onnewworktree,
    ongoto,
    onopenconflict,
    onclose,
  }: {
    repository: Repository;
    worktree: Worktree;
    initialBranch?: string;
    initialMode?: "existing" | "new";
    initialStartPoint?: string;
    source?: SwitchSource;
    /** A switch ran; the repositories are read again. */
    onchange: () => void;
    /** Opens the new-worktree dialog with the chosen target. */
    onnewworktree: (preset: WorktreePreset) => void;
    /** Opens the details of the worktree at a path. */
    ongoto: (path: string) => void;
    /** Opens the 3-way conflict resolver when Smart Checkout encounters conflicts. */
    onopenconflict?: (tree: Worktree, path?: string) => void;
    onclose: () => void;
  } = $props();
  let busy = $state(false);
</script>

<ResponsiveDialog
  title={m.s8_switch_branch_heading({ title: worktreeTitle(worktree) })}
  description={m.s8_switch_dialog_desc()}
  drawerDescription={m.s8_switch_dialog_drawer_desc()}
  truncateDrawerTitle
  {busy}
  {onclose}
>
  <SwitchBranch
    {repository}
    {worktree}
    {initialBranch}
    {initialMode}
    {initialStartPoint}
    {source}
    bind:busy
    {onchange}
    ondone={onclose}
    onnewworktree={(preset) => {
      onnewworktree(preset);
      onclose();
    }}
    ongoto={(path) => {
      ongoto(path);
      onclose();
    }}
    onopenconflict={onopenconflict
      ? (tree, path) => {
          onopenconflict(tree, path);
          onclose();
        }
      : undefined}
    oncancel={onclose}
  />
</ResponsiveDialog>
