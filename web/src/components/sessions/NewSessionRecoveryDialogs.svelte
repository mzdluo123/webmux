<script lang="ts">
  import type { Worktree } from "../../contracts/schemas";
  import * as AlertDialog from "$lib/components/ui/alert-dialog";
  import { m } from "$lib/paraglide/messages.js";
  import LazyDialog from "../shared/LazyDialog.svelte";
  let {
    workspaceEnabled,
    filesEnabled,
    removing,
    onchange,
    onremoved,
    onremovalclose,
    mkdirOpen = $bindable(false),
    mkdirMounted,
    cwd,
    onmkdir,
  }: {
    workspaceEnabled: boolean;
    filesEnabled: boolean;
    removing: Worktree | null;
    onchange: () => void;
    onremoved: (tree: Worktree) => void;
    onremovalclose: () => void;
    mkdirOpen: boolean;
    mkdirMounted: boolean;
    cwd: string;
    onmkdir: () => void;
  } = $props();
</script>

{#if workspaceEnabled && removing}
  {@const target = removing}
  <LazyDialog
    load={() => import("../workspace/RemoveWorktreeDialog.svelte")}
    label={m.s3_delete_confirmation_label()}
    onclose={onremovalclose}
  >
    {#snippet children(page)}<page.default
        worktree={target}
        {onchange}
        onremoved={() => onremoved(target)}
        onclose={onremovalclose}
      />{/snippet}
  </LazyDialog>
{/if}

{#if filesEnabled && mkdirMounted}<AlertDialog.Root bind:open={mkdirOpen}>
    <AlertDialog.Content>
      <AlertDialog.Header
        ><AlertDialog.Title>{m.s3_mkdir_confirm_title()}</AlertDialog.Title
        ><AlertDialog.Description class="break-all"
          >{m.s3_mkdir_confirm_desc({ cwd })}</AlertDialog.Description
        ></AlertDialog.Header
      >
      <AlertDialog.Footer>
        <AlertDialog.Cancel>{m.s3_cancel()}</AlertDialog.Cancel>
        <AlertDialog.Action onclick={onmkdir}
          >{m.s3_mkdir_and_start()}</AlertDialog.Action
        >
      </AlertDialog.Footer>
    </AlertDialog.Content>
  </AlertDialog.Root>{/if}
