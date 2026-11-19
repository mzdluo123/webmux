<script lang="ts">
  import IconRefresh from "~icons/tabler/refresh";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import { m } from "$lib/paraglide/messages.js";
  import type { GitDiscardPreview } from "../../contracts/schemas";
  import { canConfirmPreviewModel } from "../../domains/workspace/git-v2-adapter";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import { ResponsiveDialog } from "$lib/components/ui/responsive-dialog";
  import GitDialogFooter from "./GitDialogFooter.svelte";
  import GitIssueList from "./GitIssueList.svelte";
  import GitOperationSessionAck from "./GitOperationSessionAck.svelte";

  let {
    preview,
    loading = false,
    submitting = false,
    staleRejected = false,
    acknowledgeSessions = false,
    dialogError = "",
    onAcknowledgeSessionsChange,
    onRefreshAfterStale,
    onConfirm,
    onClose,
  }: {
    preview: GitDiscardPreview | null;
    loading?: boolean;
    submitting?: boolean;
    staleRejected?: boolean;
    acknowledgeSessions?: boolean;
    dialogError?: string;
    onAcknowledgeSessionsChange: (checked: boolean) => void;
    onRefreshAfterStale?: () => void;
    onConfirm: () => void;
    onClose: () => void;
  } = $props();

  const scopeLabel = $derived.by(() => {
    switch (preview?.scope) {
      case "lines":
        return m.s6_discard_scope_lines({ count: String(preview.lineCount) });
      case "hunks":
        return m.s6_discard_scope_hunks({ count: String(preview.hunkCount) });
      default:
        return m.s6_discard_scope_files({
          count: String(preview?.items.length ?? 0),
        });
    }
  });

  const canConfirm = $derived(
    !staleRejected &&
      canConfirmPreviewModel(preview, loading, submitting, acknowledgeSessions),
  );
</script>

<ResponsiveDialog
  title={m.s6_discard_dialog_title()}
  description={m.s6_discard_dialog_desc()}
  drawerDescription={m.s6_discard_dialog_drawer_desc()}
  busy={submitting}
  class="sm:max-w-lg"
  onclose={onClose}
>
  <div class="space-y-3" data-testid="git-discard-preview-dialog">
    <InlineNotice tone="warning">
      {m.s6_discard_notice_before()}
      <span class="font-mono">refs/webmux/discard-recovery/</span>
      {m.s6_discard_notice_after()}
    </InlineNotice>

    {#if staleRejected}
      <InlineNotice tone="warning">
        {m.s6_discard_stale_rejected()}
        {#snippet actions()}
          {#if onRefreshAfterStale}
            <Button
              size="sm"
              variant="outline"
              disabled={loading || submitting}
              onclick={onRefreshAfterStale}
              data-testid="git-discard-refresh-stale-btn"
            >
              <IconRefresh aria-hidden="true" />
              {m.s6_discard_refresh_stale_btn()}
            </Button>
          {/if}
        {/snippet}
      </InlineNotice>
    {/if}

    {#if loading && !preview}
      <p class="text-xs text-muted-foreground" role="status">
        {m.s6_discard_loading_hashes()}
      </p>
    {:else if preview}
      <div class="space-y-2 rounded-lg border bg-background p-3 text-xs">
        <div class="flex min-w-0 flex-wrap items-center gap-1.5">
          <Badge variant="destructive" class="max-w-full">
            <span class="truncate">{m.s6_discard_destructive_badge()}</span>
          </Badge>
          <Badge variant="outline" class="max-w-full">
            <span class="truncate">{scopeLabel}</span>
          </Badge>
        </div>
        <p class="font-medium wrap-anywhere text-foreground">
          {preview.summary}
        </p>

        {#if preview.trackedOverwritePaths.length > 0}
          <div class="space-y-1">
            <p class="font-semibold text-foreground">
              {m.s6_discard_tracked_heading({
                count: String(preview.trackedOverwritePaths.length),
              })}
            </p>
            <div
              class="rounded-sm border bg-background"
              data-testid="git-discard-tracked-list"
            >
              <ScrollArea viewportClasses="max-h-28">
                <ul class="space-y-0.5 p-2 font-mono">
                  {#each preview.trackedOverwritePaths as p (p)}
                    <li class="break-all text-foreground">{p}</li>
                  {/each}
                </ul>
              </ScrollArea>
            </div>
          </div>
        {/if}

        {#if preview.untrackedDeletePaths.length > 0}
          <div class="space-y-1">
            <p class="font-semibold text-destructive">
              {m.s6_discard_untracked_heading({
                count: String(preview.untrackedDeletePaths.length),
              })}
            </p>
            <div
              class="rounded-sm border bg-background"
              data-testid="git-discard-untracked-list"
            >
              <ScrollArea viewportClasses="max-h-28">
                <ul class="space-y-0.5 p-2 font-mono">
                  {#each preview.untrackedDeletePaths as p (p)}
                    <li class="break-all text-destructive">{p}</li>
                  {/each}
                </ul>
              </ScrollArea>
            </div>
          </div>
        {/if}
      </div>

      {#if preview.blockers.length > 0 || preview.warnings.length > 0}
        <GitIssueList blockers={preview.blockers} warnings={preview.warnings} />
      {/if}

      <p class="text-muted-foreground" data-testid="git-discard-recovery-note">
        {m.s6_discard_recovery_note()}
      </p>

      {#if preview.sessions.length > 0}
        <GitOperationSessionAck
          sessions={preview.sessions}
          {acknowledgeSessions}
          disabled={submitting}
          acknowledgementLabel={m.s6_discard_session_ack()}
          {onAcknowledgeSessionsChange}
        />
      {/if}
    {/if}

    <GitDialogFooter
      {dialogError}
      {submitting}
      {canConfirm}
      wrap
      confirmLabel={m.s6_discard_confirm_btn()}
      submittingLabel={m.s6_discard_submitting_btn()}
      confirmVariant="destructive"
      confirmTestId="git-discard-confirm-btn"
      {onConfirm}
      {onClose}
    />
  </div>
</ResponsiveDialog>
