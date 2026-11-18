<script lang="ts">
  import { Badge } from "$lib/components/ui/badge";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Label } from "$lib/components/ui/label";
  import {
    SegmentedControl,
    SegmentedControlItem,
  } from "$lib/components/ui/segmented-control";
  import { m } from "$lib/paraglide/messages.js";
  import type {
    GitStashApplyPreview,
    GitStashEntry,
  } from "../../contracts/schemas";
  import {
    canConfirmPreviewModel,
    type StashPreviewDialogBaseProps,
  } from "../../domains/workspace/git-v2-adapter";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import { ResponsiveDialog } from "$lib/components/ui/responsive-dialog";
  import GitDialogFooter from "./GitDialogFooter.svelte";
  import GitIssueList from "./GitIssueList.svelte";
  import GitReviewBody from "./GitReviewBody.svelte";
  import GitOperationSessionAck from "./GitOperationSessionAck.svelte";

  let {
    mode,
    reinstateIndex,
    target,
    preview,
    loading = false,
    submitting = false,
    acknowledgeSessions = false,
    dialogError = "",
    onModeChange,
    onReinstateIndexChange,
    onAcknowledgeSessionsChange,
    onConfirm,
    onClose,
  }: StashPreviewDialogBaseProps & {
    mode: "apply" | "pop";
    reinstateIndex: boolean;
    target: GitStashEntry | null;
    preview: GitStashApplyPreview | null;
    onModeChange: (mode: "apply" | "pop") => void;
    onReinstateIndexChange: (checked: boolean) => void;
  } = $props();

  const canConfirm = $derived(
    canConfirmPreviewModel(preview, loading, submitting, acknowledgeSessions),
  );
  const entry = $derived(preview?.entry ?? target);
</script>

<ResponsiveDialog
  title={mode === "pop"
    ? m.s6_stash_apply_title_pop()
    : m.s6_stash_apply_title_apply()}
  description={m.s6_stash_apply_desc()}
  drawerDescription={m.s6_stash_apply_drawer_desc()}
  pinDrawerHeader
  busy={submitting}
  class="sm:max-w-lg"
  onclose={onClose}
>
  <GitReviewBody
    testId="git-stash-apply-dialog"
    resetScrollOn={`${mode}:${dialogError}`}
  >
    <SegmentedControl
      wrap
      disabled={submitting}
      value={mode}
      onValueChange={(val) => {
        if (val === "apply" || val === "pop") onModeChange(val);
      }}
      aria-label={m.s6_stash_apply_mode_aria()}
    >
      <SegmentedControlItem value="pop">
        {m.s6_stash_mode_pop()}
      </SegmentedControlItem>
      <SegmentedControlItem value="apply">
        {m.s6_stash_mode_apply()}
      </SegmentedControlItem>
    </SegmentedControl>

    <div class="rounded-lg border bg-background p-3">
      <Label class="items-start">
        <Checkbox
          checked={reinstateIndex}
          disabled={submitting}
          onCheckedChange={(val) => onReinstateIndexChange(Boolean(val))}
          class="mt-0.5"
          data-testid="git-stash-reinstate-index-checkbox"
        />
        <span class="text-xs leading-snug">
          {m.s6_stash_reinstate_index_before()}<span class="font-mono"
            >--index</span
          >)
        </span>
      </Label>
    </div>

    {#if dialogError}
      <InlineNotice tone="danger">{dialogError}</InlineNotice>
    {/if}

    {#if entry}
      <div class="space-y-1 rounded-lg border bg-background p-3 text-xs">
        <p class="font-mono text-foreground">
          {entry.stashRef} · {entry.shortOid}
          {#if entry.hasUntracked}
            <Badge variant="outline" class="ml-1">
              {m.s6_stash_has_untracked_badge()}
            </Badge>
          {/if}
        </p>
        <p class="font-medium wrap-anywhere text-foreground">{entry.summary}</p>
      </div>
    {/if}

    {#if mode === "pop"}
      <InlineNotice tone="info">
        {m.s6_stash_safe_pop_before()}<span class="font-mono"
          >git stash apply</span
        >{m.s6_stash_safe_pop_after()}
      </InlineNotice>
    {/if}

    {#if loading && !preview}
      <p class="text-xs text-muted-foreground" role="status">
        {m.s6_stash_loading_preview()}
      </p>
    {:else if preview}
      {#if preview.overlappingPaths.length > 0}
        <InlineNotice tone="warning">
          {m.s6_stash_overlapping_paths({
            count: String(preview.overlappingPaths.length),
            paths: `${preview.overlappingPaths.slice(0, 4).join(", ")}${preview.overlappingPaths.length > 4 ? " …" : ""}`,
          })}
        </InlineNotice>
      {/if}

      <GitIssueList blockers={preview.blockers} warnings={preview.warnings} />

      <GitOperationSessionAck
        sessions={preview.sessions}
        {acknowledgeSessions}
        disabled={submitting}
        acknowledgementLabel={m.s6_stash_session_ack()}
        {onAcknowledgeSessionsChange}
      />
    {/if}
  </GitReviewBody>

  {#snippet footer()}
    <GitDialogFooter
      {submitting}
      {canConfirm}
      confirmLabel={mode === "pop"
        ? m.s6_stash_confirm_pop()
        : m.s6_stash_confirm_apply()}
      submittingLabel={mode === "pop"
        ? m.s6_stash_submitting_pop()
        : m.s6_stash_submitting_apply()}
      confirmTestId="git-stash-apply-confirm-btn"
      {onConfirm}
      {onClose}
    />
  {/snippet}
</ResponsiveDialog>
