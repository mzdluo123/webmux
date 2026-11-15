<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
  import { ItemGroup } from "$lib/components/ui/item";
  import { Badge } from "$lib/components/ui/badge";
  import type { GitBranchItem } from "../../contracts/git-workbench-schemas";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";
  import GitDialogFooter from "./GitDialogFooter.svelte";
  import GitRecoveryCommandBox from "./GitRecoveryCommandBox.svelte";

  let {
    target,
    submitting = false,
    dialogError = "",
    onConfirm,
    onClose,
  }: {
    target: GitBranchItem;
    submitting?: boolean;
    dialogError?: string;
    onConfirm: () => void;
    onClose: () => void;
  } = $props();

  const recoveryCommand = $derived(`git tag ${target.name} ${target.head}`);
</script>

<ResponsiveDialog
  title={m.s7_tag_delete_title({ name: target.name })}
  description={m.s7_tag_delete_desc()}
  drawerDescription={m.s7_tag_delete_drawer_desc()}
  busy={submitting}
  class="sm:max-w-lg"
  onclose={onClose}
>
  <div class="space-y-3" data-testid="git-tag-delete-dialog">
    <ItemGroup variant="panel" class="space-y-1.5">
      <div class="flex flex-wrap items-center gap-1.5">
        <Badge variant="destructive">
          <span class="font-mono">{target.name}</span>
        </Badge>
        <Badge variant="outline">
          {target.tagKind === "annotated"
            ? m.s7_tag_annotated_badge()
            : m.s7_tag_lightweight_badge()}
        </Badge>
      </div>
      <p class="font-mono break-all text-muted-foreground">
        {m.s7_tag_target_oid_line({ oid: target.head })}
      </p>
      {#if target.tagMessage}
        <p class="whitespace-pre-wrap wrap-anywhere text-foreground">
          {target.tagMessage}
        </p>
      {/if}
    </ItemGroup>

    <GitRecoveryCommandBox
      label={m.s7_tag_delete_recovery_label()}
      copyLabel={m.s7_tag_delete_copy_recovery()}
      command={recoveryCommand}
      testId="git-tag-delete-recovery-cmd"
    />

    <GitDialogFooter
      {dialogError}
      {submitting}
      confirmLabel={m.s7_tag_delete_confirm()}
      submittingLabel={m.s7_tag_delete_submitting()}
      confirmVariant="destructive"
      confirmTestId="git-tag-delete-confirm-btn"
      {onConfirm}
      {onClose}
    />
  </div>
</ResponsiveDialog>
