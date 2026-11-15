<script lang="ts">
  import { ItemGroup } from "$lib/components/ui/item";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import { m } from "$lib/paraglide/messages.js";
  import type { GitAbortOperationPreview } from "../../contracts/schemas";
  import { gitSequencerKindLabel } from "../../domains/workspace/git-divergence-model";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";
  import GitOperationSessionAck from "./GitOperationSessionAck.svelte";
  import PreviewIssues from "./PreviewIssues.svelte";

  let {
    abortPreview,
    busy = false,
    fetching = false,
    errorMessage = "",
    acknowledged = false,
    onAcknowledgeChange,
    onConfirmAbort,
    onDismiss,
  }: {
    abortPreview: GitAbortOperationPreview | null;
    busy?: boolean;
    fetching?: boolean;
    errorMessage?: string;
    acknowledged?: boolean;
    onAcknowledgeChange?: (checked: boolean) => void;
    onConfirmAbort: () => void;
    onDismiss: () => void;
  } = $props();

  const runningSessions = $derived(abortPreview?.sessions ?? []);
  const readyToAbort = $derived(
    Boolean(
      abortPreview?.confirmationId &&
      abortPreview.blockers.length === 0 &&
      (runningSessions.length === 0 || acknowledged) &&
      !fetching &&
      !busy,
    ),
  );
</script>

<ResponsiveDialog
  title={abortPreview
    ? m.s6_abort_dialog_title_kind({
        kind: gitSequencerKindLabel(abortPreview.kind),
      })
    : m.s6_abort_dialog_title_generic()}
  description={abortPreview
    ? abortPreview.directionSummary
    : m.s6_abort_dialog_loading_desc()}
  drawerDescription={m.s6_abort_dialog_drawer_desc()}
  drawerFullScreen
  truncateDrawerTitle
  {busy}
  class="sm:max-w-[min(640px,calc(100vw-2rem))]"
  onclose={onDismiss}
>
  <div class="min-w-0 space-y-3" data-testid="git-abort-operation-dialog">
    {#if errorMessage}
      <InlineNotice tone="danger">{errorMessage}</InlineNotice>
    {/if}

    {#if fetching && !abortPreview}
      <p class="py-4 text-xs text-muted-foreground" role="status">
        {m.s6_abort_loading_orig_head()}
      </p>
    {:else if abortPreview}
      <section
        class="min-w-0 rounded-lg border border-destructive/30 bg-background p-3"
      >
        <div class="mb-2 flex min-w-0 flex-wrap items-center gap-1.5">
          <Badge variant="destructive" class="max-w-full">
            <span class="truncate">
              {m.s6_abort_badge_kind({
                kind: gitSequencerKindLabel(abortPreview.kind),
              })}
            </span>
          </Badge>
          {#if abortPreview.currentBranch}
            <Badge variant="info" class="max-w-full">
              <span class="truncate" title={abortPreview.currentBranch}>
                {abortPreview.currentBranch}
              </span>
            </Badge>
          {/if}
        </div>
        <p class="mb-2 text-sm font-semibold wrap-anywhere text-foreground">
          {abortPreview.directionSummary}
        </p>
        <p class="font-mono text-xs break-all text-muted-foreground">
          {m.s6_worktree_path_line({ path: abortPreview.worktreePath })}
        </p>
        {#if abortPreview.restoreOid}
          <p class="mt-1 text-xs wrap-anywhere text-foreground">
            {m.s6_abort_restore_anchor_label()}<span class="font-mono"
              >{abortPreview.restoreOid.slice(0, 7)}</span
            >
            <span class="font-mono text-muted-foreground"
              >({abortPreview.restoreOid})</span
            >
            {#if abortPreview.restoreSubject}
              <span class="text-muted-foreground"
                >· {abortPreview.restoreSubject}</span
              >
            {/if}
          </p>
        {/if}
      </section>

      <PreviewIssues
        blockers={abortPreview.blockers}
        warnings={abortPreview.warnings}
      />

      {#if abortPreview.affectedPaths.length > 0}
        <ItemGroup variant="panel" class="space-y-1.5">
          <span class="font-semibold text-foreground">
            {m.s6_abort_affected_paths_heading({
              count: abortPreview.affectedPaths.length,
            })}
          </span>
          <ScrollArea viewportClasses="max-h-44">
            <ul class="space-y-1 font-mono">
              {#each abortPreview.affectedPaths as path (path)}
                <li class="break-all text-muted-foreground">{path}</li>
              {/each}
            </ul>
          </ScrollArea>
        </ItemGroup>
      {/if}

      <GitOperationSessionAck
        sessions={runningSessions}
        acknowledgeSessions={acknowledged}
        disabled={busy}
        acknowledgementLabel={m.s6_abort_session_ack({
          count: runningSessions.length,
        })}
        onAcknowledgeSessionsChange={onAcknowledgeChange}
      />
    {/if}

    <footer class="flex flex-wrap justify-end gap-2 pt-1">
      <Button variant="outline" disabled={busy} onclick={onDismiss}>
        {m.s6_cancel()}
      </Button>
      <Button
        variant="destructive"
        wrap
        disabled={!readyToAbort}
        onclick={onConfirmAbort}
        data-testid="git-abort-confirm-btn"
      >
        {busy ? m.s6_abort_confirm_submitting() : m.s6_abort_confirm_action()}
      </Button>
    </footer>
  </div>
</ResponsiveDialog>
