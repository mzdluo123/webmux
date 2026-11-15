<script lang="ts">
  import * as m from "$lib/paraglide/messages.js";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import type { GitStashCreatePreview } from "../../contracts/schemas";
  import {
    canConfirmPreviewModel,
    type StashPreviewDialogBaseProps,
  } from "../../domains/workspace/git-v2-adapter";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";
  import GitDialogFooter from "./GitDialogFooter.svelte";
  import GitIssueList from "./GitIssueList.svelte";
  import GitOperationSessionAck from "./GitOperationSessionAck.svelte";

  let {
    message,
    includeUntracked,
    keepIndex,
    preview,
    loading = false,
    submitting = false,
    acknowledgeSessions = false,
    dialogError = "",
    onMessageChange,
    onIncludeUntrackedChange,
    onKeepIndexChange,
    onAcknowledgeSessionsChange,
    onConfirm,
    onClose,
  }: StashPreviewDialogBaseProps & {
    message: string;
    includeUntracked: boolean;
    keepIndex: boolean;
    preview: GitStashCreatePreview | null;
    onMessageChange: (value: string) => void;
    onIncludeUntrackedChange: (checked: boolean) => void;
    onKeepIndexChange: (checked: boolean) => void;
  } = $props();

  const canConfirm = $derived(
    canConfirmPreviewModel(preview, loading, submitting, acknowledgeSessions),
  );
</script>

<ResponsiveDialog
  title={m.s7_stash_create_title()}
  description={m.s7_stash_create_desc()}
  drawerDescription={m.s7_stash_create_drawer_desc()}
  busy={submitting}
  class="sm:max-w-lg"
  onclose={onClose}
>
  <div class="space-y-3" data-testid="git-stash-create-dialog">
    <div class="space-y-1.5">
      <Label for="git-stash-message">{m.s7_stash_create_msg_label()}</Label>
      <Input
        id="git-stash-message"
        value={message}
        placeholder={m.s7_stash_create_msg_placeholder()}
        disabled={submitting}
        oninput={(e) => onMessageChange(e.currentTarget.value)}
      />
    </div>

    <div class="space-y-2 rounded-lg border bg-background p-3">
      <Label class="items-start">
        <Checkbox
          checked={includeUntracked}
          disabled={submitting}
          onCheckedChange={(val) => onIncludeUntrackedChange(Boolean(val))}
          class="mt-0.5"
          data-testid="git-stash-include-untracked-checkbox"
        />
        <span class="text-xs leading-snug">
          {m.s7_stash_include_untracked_pre()}<span class="font-mono"
            >--include-untracked</span
          >{m.s7_stash_include_untracked_mid()}<span class="font-mono">S^3</span
          >)
        </span>
      </Label>

      <Label class="items-start">
        <Checkbox
          checked={keepIndex}
          disabled={submitting}
          onCheckedChange={(val) => onKeepIndexChange(Boolean(val))}
          class="mt-0.5"
          data-testid="git-stash-keep-index-checkbox"
        />
        <span class="text-xs leading-snug">
          {m.s7_stash_keep_index_pre()}<span class="font-mono"
            >--keep-index</span
          >{m.s7_stash_keep_index_post()}
        </span>
      </Label>
    </div>

    {#if loading && !preview}
      <p class="text-xs text-muted-foreground" role="status">
        {m.s7_stash_create_loading()}
      </p>
    {:else if preview}
      <div
        class="grid grid-cols-1 gap-1.5 rounded-lg border bg-background p-3 text-xs sm:grid-cols-2"
      >
        <p class="wrap-anywhere">
          <span class="text-muted-foreground"
            >{m.s7_stash_create_branch_label()}</span
          >
          <span class="font-mono text-foreground">
            {preview.detached
              ? m.s7_detached_head()
              : preview.currentBranch || m.s7_unnamed_branch()}
          </span>
        </p>
        <p class="wrap-anywhere">
          <span class="text-muted-foreground"
            >{m.s7_stash_create_counts_label()}</span
          >
          <span class="text-foreground">
            {m.s7_stash_create_counts_value({
              staged: preview.stagedCount,
              unstaged: preview.unstagedCount,
              untracked: preview.untrackedCount,
            })}
          </span>
        </p>
        <p class="wrap-anywhere sm:col-span-2">
          <span class="text-muted-foreground"
            >{m.s7_stash_create_files_label()}</span
          >
          <span class="font-medium text-foreground">
            {m.s7_file_count_badge({ count: preview.affectedPaths.length })}
          </span>
        </p>
      </div>

      <GitIssueList blockers={preview.blockers} warnings={preview.warnings} />

      <GitOperationSessionAck
        sessions={preview.sessions}
        {acknowledgeSessions}
        disabled={submitting}
        acknowledgementLabel={m.s7_stash_create_session_ack()}
        {onAcknowledgeSessionsChange}
      />
    {/if}

    <GitDialogFooter
      {dialogError}
      {submitting}
      {canConfirm}
      confirmLabel={m.s7_stash_create_confirm()}
      submittingLabel={m.s7_stash_saving_phase()}
      confirmTestId="git-stash-create-confirm-btn"
      {onConfirm}
      {onClose}
    />
  </div>
</ResponsiveDialog>
