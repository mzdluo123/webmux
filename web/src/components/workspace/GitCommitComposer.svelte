<script lang="ts" module>
  import type { GitWorktreeContext } from "../../contracts/git-workbench-schemas";
  import type { GitCommitDraftConflictState } from "../../domains/workspace/git-commit-draft-model";
  import type { GitAmendDraftPrompt } from "../../hooks/workspace/git-commit-options.svelte";

  export interface GitCommitComposerProps {
    onCommitMessageChange: (msg: string) => void;
    onOpenCommitPreview: (followUpPush: boolean) => void;
    onOpenPush?: () => void;
    onCreateBranchFromHead?: () => void;
    onResolveDraftKeepLocal?: () => void;
    onResolveDraftAdoptRemote?: () => void;
    onResolveDraftMergeBoth?: () => void;
    onToggleCommitAmend?: (checked: boolean) => void;
    onResolveAmendDraft?: (choice: "replace" | "merge" | "keep") => void;
    onCommitAuthorOverrideChange?: (value: string) => void;
    onToggleCommitSignoff?: (checked: boolean) => void;
    context: GitWorktreeContext;
    stagedCount: number;
    conflictedCount?: number;
    commitMessage: string;
    readonlyMode?: boolean;
    busy?: boolean;
    uncertainBlocked?: boolean;
    draftSaving?: boolean;
    draftRevision?: number;
    draftConflict?: GitCommitDraftConflictState | null;
    commitAmend?: boolean;
    commitAmendPushed?: boolean;
    commitAuthorOverride?: string;
    commitSignoff?: boolean;
    commitSignoffTrailer?: string;
    amendDraftPrompt?: GitAmendDraftPrompt | null;
  }
</script>

<script lang="ts">
  import { tick } from "svelte";
  import IconArrowUp from "~icons/tabler/arrow-up";
  import IconCheck from "~icons/tabler/check";
  import IconGitBranch from "~icons/tabler/git-branch";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Textarea } from "$lib/components/ui/textarea";
  import { m } from "$lib/paraglide/messages.js";
  import { validateAuthorOverrideInput } from "../../domains/workspace/git-v2-adapter";
  import { allowAutomaticInputFocus } from "../../hooks/shared/input-focus";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import GitCommitAdvancedOptions from "./GitCommitAdvancedOptions.svelte";
  import GitCommitDraftConflictCard from "./GitCommitDraftConflictCard.svelte";

  let {
    context,
    stagedCount,
    conflictedCount = 0,
    commitMessage,
    readonlyMode = false,
    busy = false,
    uncertainBlocked = false,
    draftSaving = false,
    draftRevision = 0,
    draftConflict = null,
    commitAmend = false,
    commitAmendPushed = false,
    commitAuthorOverride = "",
    commitSignoff = false,
    commitSignoffTrailer = "",
    amendDraftPrompt = null,
    onCommitMessageChange,
    onOpenCommitPreview,
    onOpenPush,
    onCreateBranchFromHead,
    onResolveDraftKeepLocal,
    onResolveDraftAdoptRemote,
    onResolveDraftMergeBoth,
    onToggleCommitAmend,
    onResolveAmendDraft,
    onCommitAuthorOverrideChange,
    onToggleCommitSignoff,
  }: GitCommitComposerProps = $props();

  const parsed = $derived.by(() => {
    const idx = commitMessage.indexOf("\n");
    if (idx === -1) return { subject: commitMessage, body: "" };
    const subject = commitMessage.slice(0, idx);
    const rest = commitMessage.slice(idx + 1).replace(/^\n/, "");
    return { subject, body: rest };
  });

  function updateSubject(nextSubject: string) {
    const clean = nextSubject.replace(/\r?\n/g, " ");
    if (!parsed.body) {
      onCommitMessageChange(clean);
    } else {
      onCommitMessageChange(`${clean}\n\n${parsed.body}`);
    }
  }

  function updateBody(nextBody: string) {
    if (!nextBody) {
      onCommitMessageChange(parsed.subject);
    } else {
      onCommitMessageChange(`${parsed.subject}\n\n${nextBody}`);
    }
  }

  const authorValidation = $derived(
    validateAuthorOverrideInput(commitAuthorOverride),
  );
  const identityMissing = $derived(
    !readonlyMode &&
      !commitAuthorOverride.trim() &&
      (!context.authorName.trim() || !context.authorEmail.trim()),
  );
  const detachedHead = $derived(context.headState === "detached");
  const unbornHead = $derived(context.headState === "unborn");
  const canPreviewCommit = $derived(
    !readonlyMode &&
      !busy &&
      !uncertainBlocked &&
      !draftConflict &&
      (stagedCount > 0 || (commitAmend && !unbornHead)) &&
      conflictedCount === 0 &&
      authorValidation.valid &&
      Boolean(parsed.subject.trim()),
  );
  const canDirectPush = $derived(
    !readonlyMode &&
      !uncertainBlocked &&
      !commitAmend &&
      stagedCount === 0 &&
      context.headState === "branch" &&
      Boolean(context.checkedOutBranch) &&
      (context.ahead > 0 || !context.upstream || Boolean(context.upstreamGone)),
  );

  const disabledHint = $derived.by(() => {
    if (readonlyMode) return m.s6_commit_hint_readonly();
    if (draftConflict) return m.s6_commit_hint_draft_conflict();
    if (uncertainBlocked) return m.s6_commit_hint_uncertain();
    if (conflictedCount > 0) return m.s6_commit_hint_conflicted();
    if (!authorValidation.valid) return authorValidation.error;
    if (commitAmend && unbornHead) return m.s6_commit_hint_unborn_amend();
    if (stagedCount === 0 && !commitAmend)
      return m.s6_commit_hint_empty_staged();
    if (!parsed.subject.trim()) return m.s6_commit_hint_missing_subject();
    return "";
  });

  let bodyEditor = $state<HTMLTextAreaElement | null>(null);

  async function resolveDraft(
    action: () => void,
    mode: "keep" | "adopt" | "merge",
  ) {
    const editor = bodyEditor;
    const reading = editor && {
      body: editor.value,
      start: editor.selectionStart,
      end: editor.selectionEnd,
      direction: editor.selectionDirection,
      top: editor.scrollTop,
    };
    action();
    await tick();
    if (!editor?.isConnected || editor.disabled || !reading) return;

    if (allowAutomaticInputFocus()) {
      editor.focus({ preventScroll: true });
    }
    if (mode === "adopt") {
      editor.setSelectionRange(0, 0);
      editor.scrollTop = 0;
    } else {
      // A merged body can introduce a heading before the original text.
      const prefixLength =
        mode === "merge" && reading.body
          ? editor.value.indexOf(reading.body)
          : 0;
      const offset = Math.max(0, prefixLength);
      const precedingLines =
        editor.value.slice(0, offset).split("\n").length - 1;
      const lineHeight =
        Number.parseFloat(getComputedStyle(editor).lineHeight) || 20;
      editor.setSelectionRange(
        reading.start + offset,
        reading.end + offset,
        reading.direction,
      );
      editor.scrollTop = reading.top + precedingLines * lineHeight;
    }
    editor.scrollIntoView({ block: "nearest", inline: "nearest" });
  }

  function handleKeydown(e: KeyboardEvent) {
    if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
      e.preventDefault();
      if (canPreviewCommit) {
        // Safety invariant UI-10: Ctrl/Cmd+Enter ONLY opens preview, never direct-commits!
        onOpenCommitPreview(false);
      }
    }
  }
</script>

<div
  class="space-y-1.5 border-t bg-background p-2.5"
  data-testid="git-commit-composer"
>
  <div
    class="flex min-w-0 flex-wrap items-center justify-between gap-1.5 text-xs text-muted-foreground"
  >
    <span class="min-w-0 truncate">
      {#if context.authorName && context.authorEmail}
        {m.s6_commit_author_display({
          name: context.authorName,
          email: context.authorEmail,
        })}
      {:else}
        {m.s6_commit_author_unconfigured()}
      {/if}
    </span>

    <div class="flex flex-wrap items-center gap-1">
      {#if draftSaving}
        <Badge variant="outline">{m.s6_commit_draft_syncing()}</Badge>
      {:else if draftRevision > 0 && commitMessage.trim()}
        <Badge variant="outline" data-testid="git-commit-draft-synced">
          <IconCheck class="size-3" aria-hidden="true" />
          {m.s6_commit_draft_synced({ revision: draftRevision })}
        </Badge>
      {/if}
      {#if context.signEnabled}
        <Badge variant="info">
          {context.signFormat
            ? m.s6_commit_sign_format_badge({
                format: context.signFormat.toUpperCase(),
              })
            : m.s6_commit_sign_enabled_badge()}
        </Badge>
      {/if}
      {#if context.executableHooks && context.executableHooks.length > 0}
        <Badge
          variant="outline"
          title={m.s6_commit_hooks_tooltip({
            hooks: context.executableHooks.join(", "),
          })}
        >
          {m.s6_commit_hooks_badge({ count: context.executableHooks.length })}
        </Badge>
      {/if}
    </div>
  </div>

  {#if draftConflict && onResolveDraftKeepLocal && onResolveDraftAdoptRemote && onResolveDraftMergeBoth}
    <GitCommitDraftConflictCard
      conflict={draftConflict}
      saving={draftSaving}
      onKeepLocal={() => void resolveDraft(onResolveDraftKeepLocal, "keep")}
      onAdoptRemote={() =>
        void resolveDraft(onResolveDraftAdoptRemote, "adopt")}
      onMergeBoth={() => void resolveDraft(onResolveDraftMergeBoth, "merge")}
    />
  {/if}

  {#if detachedHead}
    <InlineNotice tone="warning">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <span>{m.s6_commit_detached_notice()}</span>
        {#if onCreateBranchFromHead}
          <Button size="sm" variant="outline" onclick={onCreateBranchFromHead}>
            <IconGitBranch aria-hidden="true" />
            {m.s6_commit_new_branch_from_head()}
          </Button>
        {/if}
      </div>
    </InlineNotice>
  {:else if identityMissing}
    <InlineNotice tone="warning">
      {m.s6_commit_identity_missing_notice()}
    </InlineNotice>
  {/if}

  <Input
    value={parsed.subject}
    placeholder={m.s6_commit_subject_placeholder()}
    aria-label={m.s6_commit_subject_aria()}
    disabled={readonlyMode}
    oninput={(e) => updateSubject(e.currentTarget.value)}
    onkeydown={handleKeydown}
  />

  <Textarea
    bind:ref={bodyEditor}
    class="max-sm:max-h-40"
    rows={2}
    value={parsed.body}
    placeholder={m.s6_commit_body_placeholder()}
    aria-label={m.s6_commit_body_aria()}
    disabled={readonlyMode}
    oninput={(e) => updateBody(e.currentTarget.value)}
    onkeydown={handleKeydown}
  />

  {#if onToggleCommitAmend || onCommitAuthorOverrideChange || onToggleCommitSignoff}
    <GitCommitAdvancedOptions
      {context}
      {readonlyMode}
      amend={commitAmend}
      amendPushed={commitAmendPushed}
      authorOverride={commitAuthorOverride}
      signoff={commitSignoff}
      signoffTrailer={commitSignoffTrailer}
      draftPrompt={amendDraftPrompt}
      onToggleAmend={onToggleCommitAmend}
      {onResolveAmendDraft}
      onAuthorOverrideChange={onCommitAuthorOverrideChange}
      onToggleSignoff={onToggleCommitSignoff}
    />
  {/if}

  {#if disabledHint}
    <p class="text-xs text-muted-foreground" data-testid="git-commit-hint">
      {disabledHint}
    </p>
  {/if}

  <div class="flex flex-wrap items-center justify-end gap-2">
    {#if canDirectPush && onOpenPush}
      <Button variant="outline" onclick={onOpenPush}>
        <IconArrowUp aria-hidden="true" />
        {m.s6_commit_push_ahead_btn()}
      </Button>
    {/if}
    <Button
      variant="outline"
      disabled={!canPreviewCommit ||
        detachedHead ||
        (commitAmend && commitAmendPushed)}
      title={detachedHead
        ? m.s6_commit_and_push_detached_tooltip()
        : commitAmend && commitAmendPushed
          ? m.s6_commit_and_push_amend_pushed_tooltip()
          : disabledHint || undefined}
      onclick={() => onOpenCommitPreview(true)}
      data-testid="git-commit-and-push-btn"
    >
      {m.s6_commit_and_push_btn()}
    </Button>
    <Button
      variant="default"
      disabled={!canPreviewCommit}
      title={disabledHint || undefined}
      onclick={() => onOpenCommitPreview(false)}
    >
      {m.s6_commit_preview_btn()}
    </Button>
  </div>
</div>
