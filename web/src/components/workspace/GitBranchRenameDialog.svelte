<script lang="ts">
  import { Badge } from "$lib/components/ui/badge";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import { m } from "$lib/paraglide/messages.js";
  import type { GitBranchRenamePreview } from "../../contracts/schemas";
  import { canExecutePreviewedMutation } from "../../domains/workspace/git-v2-adapter";
  import { ResponsiveDialog } from "$lib/components/ui/responsive-dialog";
  import GitDialogFooter from "./GitDialogFooter.svelte";
  import GitIssueList from "./GitIssueList.svelte";
  import GitOperationSessionAck from "./GitOperationSessionAck.svelte";

  let {
    oldName,
    newName,
    preview,
    loading = false,
    submitting = false,
    acknowledgeSessions = false,
    dialogError = "",
    onNewNameChange,
    onAcknowledgeSessionsChange,
    onConfirm,
    onClose,
  }: {
    oldName: string;
    newName: string;
    preview: GitBranchRenamePreview | null;
    loading?: boolean;
    submitting?: boolean;
    acknowledgeSessions?: boolean;
    dialogError?: string;
    onNewNameChange: (val: string) => void;
    onAcknowledgeSessionsChange: (checked: boolean) => void;
    onConfirm: () => void;
    onClose: () => void;
  } = $props();

  const canConfirm = $derived(
    Boolean(newName.trim()) &&
      newName.trim() !== oldName &&
      canExecutePreviewedMutation({
        loading,
        submitting,
        confirmationId: preview?.confirmationId,
        blockerCount: preview?.blockers.length ?? 0,
        sessionCount: preview?.sessions.length ?? 0,
        acknowledgeSessions,
      }),
  );
</script>

<ResponsiveDialog
  title={m.s6_branch_rename_title({ name: oldName })}
  description={m.s6_branch_rename_desc()}
  drawerDescription={m.s6_branch_rename_drawer_desc()}
  busy={submitting}
  class="sm:max-w-lg"
  onclose={onClose}
>
  <div class="space-y-3" data-testid="git-branch-rename-dialog">
    <div class="space-y-1.5">
      <Label for="git-branch-rename-input">
        {m.s6_branch_rename_input_label()}
      </Label>
      <Input
        id="git-branch-rename-input"
        mono
        value={newName}
        placeholder="feat/new-branch-name"
        disabled={submitting}
        oninput={(e) => onNewNameChange(e.currentTarget.value)}
        data-testid="git-branch-rename-input"
      />
    </div>

    {#if loading && !preview}
      <p class="text-xs text-muted-foreground" role="status">
        {m.s6_branch_rename_validating()}
      </p>
    {:else if preview}
      <div class="space-y-1.5 rounded-lg border bg-background p-3 text-xs">
        <p class="wrap-anywhere">
          <span class="text-muted-foreground">
            {m.s6_branch_rename_change_label()}
          </span>
          <span class="font-mono text-foreground">{preview.oldName}</span>
          →
          <span class="font-mono font-semibold text-foreground">
            {preview.newName}
          </span>
        </p>
        <p class="wrap-anywhere">
          <span class="text-muted-foreground">
            {m.s6_branch_points_to_label()}
          </span>
          <span class="font-mono text-foreground">
            {preview.headOid.slice(0, 7)}
          </span>
          · {preview.subject}
        </p>
        {#if preview.upstream}
          <p class="wrap-anywhere">
            <span class="text-muted-foreground">
              {m.s6_branch_rename_upstream_label()}
            </span>
            {m.s6_branch_rename_upstream_before()}
            <span class="font-mono text-foreground">{preview.upstream}</span>
            {m.s6_branch_rename_upstream_after()}
          </p>
        {/if}
        {#if preview.affectedWorktrees.length > 0}
          <div class="pt-1">
            <span class="text-muted-foreground">
              {m.s6_branch_rename_worktrees_label()}
            </span>
            <div class="mt-1 flex flex-wrap gap-1">
              {#each preview.affectedWorktrees as wt (wt.id)}
                <Badge variant="info">
                  <span class="font-mono">{wt.path}</span>
                </Badge>
              {/each}
            </div>
          </div>
        {/if}
      </div>

      <GitIssueList blockers={preview.blockers} warnings={preview.warnings} />

      <GitOperationSessionAck
        sessions={preview.sessions}
        {acknowledgeSessions}
        disabled={submitting}
        acknowledgementLabel={m.s6_branch_rename_session_ack()}
        {onAcknowledgeSessionsChange}
      />
    {/if}

    <GitDialogFooter
      {dialogError}
      {submitting}
      {canConfirm}
      confirmLabel={m.s6_branch_rename_confirm()}
      submittingLabel={m.s6_branch_rename_submitting()}
      confirmTestId="git-branch-rename-confirm-btn"
      {onConfirm}
      {onClose}
    />
  </div>
</ResponsiveDialog>
