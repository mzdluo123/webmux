<script lang="ts">
  import IconAlertTriangle from "~icons/tabler/alert-triangle";
  import IconArrowsSplit2 from "~icons/tabler/arrows-split-2";
  import IconCheck from "~icons/tabler/check";
  import IconPlayerPlay from "~icons/tabler/player-play";
  import IconTerminal2 from "~icons/tabler/terminal-2";
  import IconX from "~icons/tabler/x";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { m } from "$lib/paraglide/messages.js";
  import type { GitInProgressOperation } from "../../contracts/schemas";
  import {
    formatInProgressHeadline,
    gitSequencerKindLabel,
  } from "../../domains/workspace/git-divergence-model";

  let {
    operation,
    busy = false,
    conflictResolverOpen = false,
    hasActiveWorktreeSession = false,
    onOpenConflictResolver,
    onContinue,
    onOpenAbortPreview,
    onOpenWorktreeTerminal,
  }: {
    operation: GitInProgressOperation;
    busy?: boolean;
    conflictResolverOpen?: boolean;
    hasActiveWorktreeSession?: boolean;
    onOpenConflictResolver?: (path?: string) => void;
    onContinue?: () => void;
    onOpenAbortPreview?: () => void;
    onOpenWorktreeTerminal?: () => void;
  } = $props();

  const shortOid = (oid: string | undefined) => (oid ? oid.slice(0, 7) : "");
  const primaryBlocker = $derived(operation.continueBlockers[0]?.message ?? "");
</script>

{#if operation.active}
  <section
    class="border-b bg-background px-3 py-2 text-xs"
    aria-label={m.s6_in_progress_banner_aria()}
    data-testid="git-in-progress-banner"
  >
    <div class="flex min-w-0 flex-wrap items-center justify-between gap-2">
      <div class="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
        <Badge
          variant={operation.conflictedCount > 0 ? "destructive" : "warning"}
          class="max-w-full min-w-0"
        >
          {#if operation.conflictedCount > 0}
            <IconAlertTriangle aria-hidden="true" />
            <span class="truncate">
              {m.s6_in_progress_conflict_count({
                kind: gitSequencerKindLabel(operation.kind),
                count: String(operation.conflictedCount),
              })}
            </span>
          {:else}
            <IconCheck aria-hidden="true" />
            <span class="truncate">
              {m.s6_in_progress_conflicts_cleared({
                kind: gitSequencerKindLabel(operation.kind),
              })}
            </span>
          {/if}
        </Badge>

        {#if operation.stepTotal && operation.stepTotal > 0 && operation.stepCurrent}
          <Badge variant="info" class="max-w-full min-w-0">
            <span class="truncate">
              {m.s6_in_progress_step({
                current: String(operation.stepCurrent),
                total: String(operation.stepTotal),
              })}
            </span>
          </Badge>
        {/if}

        <span
          class={conflictResolverOpen
            ? "min-w-0 font-medium wrap-anywhere text-foreground max-xl:hidden"
            : "min-w-0 font-medium wrap-anywhere text-foreground"}
        >
          {formatInProgressHeadline(operation)}
        </span>

        {#if operation.stoppedCommitOid}
          <span
            class={conflictResolverOpen
              ? "font-mono text-muted-foreground max-xl:hidden"
              : "font-mono text-muted-foreground"}
          >
            {m.s6_in_progress_stopped_at({
              oid: shortOid(operation.stoppedCommitOid),
            })}
            {#if operation.stoppedSubject}
              · {operation.stoppedSubject}
            {/if}
          </span>
        {:else if operation.origHeadOid}
          <span
            class={conflictResolverOpen
              ? "font-mono text-muted-foreground max-xl:hidden"
              : "font-mono text-muted-foreground"}
          >
            {m.s6_in_progress_orig_head({
              oid: shortOid(operation.origHeadOid),
            })}
          </span>
        {/if}
      </div>

      <div class="flex min-w-0 flex-wrap items-center gap-1.5">
        {#if operation.conflictedCount > 0 && onOpenConflictResolver && !conflictResolverOpen}
          <Button
            size="sm"
            variant="default"
            wrap
            disabled={busy}
            onclick={() =>
              onOpenConflictResolver(operation.conflictedFiles[0]?.path)}
            data-testid="git-open-conflict-resolver-btn"
          >
            <IconArrowsSplit2 aria-hidden="true" />
            {m.s6_in_progress_open_resolver({
              count: String(operation.conflictedCount),
            })}
          </Button>
        {/if}

        {#if onContinue && operation.kind !== "unstash"}
          <Button
            size="sm"
            variant="success"
            wrap
            disabled={!operation.canContinue || busy}
            title={!operation.canContinue
              ? primaryBlocker || m.s6_conflict_continue_blocked_tooltip()
              : m.s6_in_progress_continue_ready_tooltip()}
            onclick={onContinue}
            data-testid="git-continue-operation-btn"
          >
            <IconPlayerPlay aria-hidden="true" />
            {busy ? m.s6_continuing_ellipsis() : m.s6_continue_operation_btn()}
          </Button>
        {/if}

        {#if operation.canAbort && onOpenAbortPreview}
          <Button
            size="sm"
            variant="destructive-outline"
            wrap
            disabled={busy}
            onclick={onOpenAbortPreview}
            data-testid="git-abort-operation-btn"
          >
            <IconX aria-hidden="true" />
            {m.s6_in_progress_abort_btn()}
          </Button>
        {/if}

        {#if hasActiveWorktreeSession && onOpenWorktreeTerminal}
          <Button
            size="sm"
            variant="ghost"
            wrap
            onclick={onOpenWorktreeTerminal}
          >
            <IconTerminal2 aria-hidden="true" />
            {m.s6_terminal_short()}
          </Button>
        {/if}
      </div>
    </div>

    {#if operation.conflictedCount === 0 && !operation.canContinue && primaryBlocker}
      <p class="mt-1 wrap-anywhere text-muted-foreground">
        {m.s6_in_progress_cannot_continue({ reason: primaryBlocker })}
      </p>
    {/if}
  </section>
{/if}
