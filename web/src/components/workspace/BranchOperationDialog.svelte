<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
  import {
    operationTitle,
    type BranchOperation,
  } from "../../domains/workspace/branch-actions";
  import type { BranchOperations } from "../../domains/workspace/branch-operations";
  import BranchOperationFlow from "./BranchOperationFlow.svelte";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";

  // A branch operation started outside the repository panel (home page,
  // sidebar, worktree details): a dialog on desktop, a bottom drawer on
  // phones, kept open while its request runs.
  let {
    repositoryId,
    operation,
    source,
    onchange,
    onclose,
  }: {
    repositoryId: string;
    operation: BranchOperation;
    source?: BranchOperations;
    onchange: () => void;
    onclose: () => void;
  } = $props();
  let busy = $state(false);
  const descriptions: Record<BranchOperation["kind"], string> = $derived({
    "fast-forward": m.s8_op_desc_fast_forward(),
    push: m.s8_op_desc_push(),
    delete: m.s8_op_desc_delete(),
  });
</script>

<ResponsiveDialog
  title={operationTitle(operation)}
  description={descriptions[operation.kind]}
  {busy}
  {onclose}
>
  <BranchOperationFlow
    {repositoryId}
    {operation}
    bind:busy
    {source}
    {onchange}
    {onclose}
  />
</ResponsiveDialog>
