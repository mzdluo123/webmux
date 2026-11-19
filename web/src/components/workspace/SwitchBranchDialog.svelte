<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import { worktreeTitle } from "../../domains/workspace/home-worktrees";
  import type { WorktreePreset } from "../../domains/sessions/worktree-preset";
  import type { Repository, Worktree } from "../../contracts/schemas";
  import { Button } from "$lib/components/ui/button";
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
  const formId = $props.id();
  let busy = $state(false);
  let switchView = $state<SwitchBranch>();
</script>

<ResponsiveDialog
  title={m.s8_switch_branch_heading({ title: worktreeTitle(worktree) })}
  description={m.s8_switch_dialog_desc()}
  drawerDescription={m.s8_switch_dialog_drawer_desc()}
  truncateDrawerTitle
  pinDrawerHeader
  drawerFullScreen
  class="h-[min(90dvh,36rem)]"
  {busy}
  {onclose}
>
  <SwitchBranch
    bind:this={switchView}
    {formId}
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
  {#snippet footer()}
    <div class="flex flex-wrap justify-end gap-2 border-t pt-3">
      <Button type="button" variant="outline" disabled={busy} onclick={onclose}>
        {m.s8_cancel()}
      </Button>
      <Button
        type="submit"
        form={formId}
        disabled={!(switchView?.isSwitchReady() ?? false)}
      >
        {busy ? m.s8_switching() : m.s8_switch_btn()}
      </Button>
    </div>
  {/snippet}
</ResponsiveDialog>
