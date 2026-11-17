<script lang="ts">
  import { m } from "$lib/paraglide/messages.js";
  import type { ArchivedSession, Worktree } from "../../contracts/schemas";
  import LazyDialog from "../shared/LazyDialog.svelte";

  let {
    worktree,
    record,
    onremoved,
    onchange,
    onclose,
  }: {
    worktree: Worktree;
    record: ArchivedSession | null;
    onremoved: () => void;
    onchange: () => void;
    onclose: () => void;
  } = $props();
</script>

<LazyDialog
  load={() => import("../workspace/RemoveWorktreeDialog.svelte")}
  label={m.s3_delete_confirmation_label()}
  {onclose}
>
  {#snippet children(page)}<page.default
      {worktree}
      forgetRecords={record ? [record] : []}
      {onremoved}
      {onchange}
      {onclose}
    />{/snippet}
</LazyDialog>
