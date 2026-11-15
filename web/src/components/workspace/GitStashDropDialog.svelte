<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
  import { ItemGroup } from "$lib/components/ui/item";
  import { Badge } from "$lib/components/ui/badge";
  import type {
    GitStashDropPreview,
    GitStashEntry,
  } from "../../contracts/schemas";
  import { canExecutePreviewedMutation } from "../../domains/workspace/git-v2-adapter";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";
  import GitDialogFooter from "./GitDialogFooter.svelte";
  import GitIssueList from "./GitIssueList.svelte";
  import GitRecoveryCommandBox from "./GitRecoveryCommandBox.svelte";

  let {
    target,
    preview,
    loading = false,
    submitting = false,
    dialogError = "",
    onConfirm,
    onClose,
  }: {
    target: GitStashEntry | null;
    preview: GitStashDropPreview | null;
    loading?: boolean;
    submitting?: boolean;
    dialogError?: string;
    onConfirm: () => void;
    onClose: () => void;
  } = $props();

  const entry = $derived(preview?.entry ?? target);
  const recoveryCmd = $derived(
    preview?.recoveryCommand ?? (entry ? `git stash apply ${entry.oid}` : ""),
  );
  const canConfirm = $derived(
    canExecutePreviewedMutation({
      loading,
      submitting,
      confirmationId: preview?.confirmationId,
      blockerCount: preview?.blockers.length ?? 0,
    }),
  );
</script>

<ResponsiveDialog
  title={m.s7_stash_drop_title()}
  description={m.s7_stash_drop_desc()}
  drawerDescription={m.s7_stash_drop_drawer_desc()}
  busy={submitting}
  class="sm:max-w-lg"
  onclose={onClose}
>
  <div class="space-y-3" data-testid="git-stash-drop-dialog">
    {#if entry}
      <ItemGroup variant="panel" class="space-y-1.5">
        <div class="flex flex-wrap items-center gap-1.5">
          <Badge variant="destructive">
            <span class="font-mono">{entry.stashRef}</span>
          </Badge>
          {#if preview}
            <Badge variant="outline">
              {m.s7_file_count_badge({ count: preview.fileCount })}
            </Badge>
          {/if}
        </div>
        <p class="font-medium wrap-anywhere text-foreground">{entry.summary}</p>
        <p class="font-mono break-all text-muted-foreground">
          {m.s7_stash_drop_oid_label({ oid: entry.oid })}
        </p>
      </ItemGroup>
    {/if}

    {#if recoveryCmd}
      <GitRecoveryCommandBox
        label={m.s7_stash_drop_recovery_label()}
        copyLabel={m.s7_stash_drop_copy_recovery()}
        command={recoveryCmd}
        testId="git-stash-drop-recovery-cmd"
      />
    {/if}

    {#if loading && !preview}
      <p class="text-xs text-muted-foreground" role="status">
        {m.s7_stash_drop_loading()}
      </p>
    {:else if preview}
      <GitIssueList blockers={preview.blockers} warnings={preview.warnings} />
    {/if}

    <GitDialogFooter
      {dialogError}
      {submitting}
      {canConfirm}
      confirmLabel={m.s7_stash_drop_confirm()}
      submittingLabel={m.s7_stash_drop_submitting()}
      confirmVariant="destructive"
      confirmTestId="git-stash-drop-confirm-btn"
      {onConfirm}
      {onClose}
    />
  </div>
</ResponsiveDialog>
