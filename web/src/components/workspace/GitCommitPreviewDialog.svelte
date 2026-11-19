<script lang="ts">
  import type { FileDiffMetadata } from "@pierre/diffs";
  import IconChevronDown from "~icons/tabler/chevron-down";
  import IconChevronRight from "~icons/tabler/chevron-right";
  import IconGitBranch from "~icons/tabler/git-branch";
  import IconRefresh from "~icons/tabler/refresh";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import { m } from "$lib/paraglide/messages.js";
  import { errorMessage } from "../../platform/error";
  import {
    gitChanges,
    gitContext,
    gitDiff,
    gitStagingDiff,
  } from "../../platform/git-api";
  import type {
    GitCommitPreviewModel,
    GitWorkingChange,
  } from "../../contracts/git-workbench-schemas";
  import {
    resolveCalculatedDiff,
    resolveHunksCalculatedDiff,
  } from "../../domains/code/code-diff-client";
  import CodeDiffReader from "../code/CodeDiffReader.svelte";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import GitCommitPreviewMetaCard from "./GitCommitPreviewMetaCard.svelte";
  import GitDialogFooter from "./GitDialogFooter.svelte";
  import GitIssueList from "./GitIssueList.svelte";
  import { ResponsiveDialog } from "$lib/components/ui/responsive-dialog";

  let {
    model,
    commitMessage,
    repositoryId = "",
    worktreeId,
    submitting = false,
    uncertainBlocked = false,
    acknowledgePushedAmend: controlledAckPushedAmend,
    onAcknowledgePushedAmendChange,
    onConfirm,
    onRefreshPreview,
    onCreateBranchFromHead,
    onClose,
  }: {
    model: GitCommitPreviewModel;
    commitMessage: string;
    repositoryId?: string;
    worktreeId?: string;
    submitting?: boolean;
    uncertainBlocked?: boolean;
    acknowledgePushedAmend?: boolean;
    onAcknowledgePushedAmendChange?: (checked: boolean) => void;
    onConfirm: () => void;
    onRefreshPreview?: () => void;
    onCreateBranchFromHead?: () => void;
    onClose: () => void;
  } = $props();

  let localAckPushedAmend = $state(false);
  const effectiveAckPushedAmend = $derived(
    controlledAckPushedAmend ?? localAckPushedAmend,
  );
  function handleAckPushedAmend(checked: boolean) {
    localAckPushedAmend = checked;
    onAcknowledgePushedAmendChange?.(checked);
  }

  let expandedFileId = $state<string | null>(null);
  let inspectState = $state.raw<{
    phase: "idle" | "loading" | "ready" | "limited" | "error";
    fileDiff: FileDiffMetadata | null;
    reason: string;
  }>({
    phase: "idle",
    fileDiff: null,
    reason: "",
  });

  const expandedFile = $derived(
    expandedFileId
      ? (model.stagedFiles.find((f) => f.id === expandedFileId) ?? null)
      : null,
  );

  // Load real HEAD -> index diff when user expands any staged file in preview
  $effect(() => {
    const file: GitWorkingChange | null = expandedFile;
    if (!file) {
      inspectState = { phase: "idle", fileDiff: null, reason: "" };
      return;
    }
    if (file.binary) {
      inspectState = {
        phase: "limited",
        fileDiff: null,
        reason: file.degradeReason || m.s6_commit_preview_binary_reason(),
      };
      return;
    }
    const ctrl = new AbortController();
    inspectState = { phase: "loading", fileDiff: null, reason: "" };

    void (async () => {
      try {
        if (repositoryId && worktreeId) {
          const ctx = await gitContext(repositoryId, worktreeId, ctrl.signal);
          const list = await gitChanges(
            repositoryId,
            {
              contextId: ctx.contextId,
              worktreeId,
              group: "staged",
              q: file.path,
            },
            ctrl.signal,
          );
          const match = list.entries.find(
            (e) =>
              e.group === "staged" &&
              (e.newPath === file.path || e.oldPath === file.path),
          );
          if (match) {
            const raw = await gitDiff(
              repositoryId,
              {
                contextId: ctx.contextId,
                worktreeId,
                changeId: match.changeId,
              },
              ctrl.signal,
            );
            if (ctrl.signal.aborted) return;
            if (raw.kind === "text") {
              const calc = await resolveCalculatedDiff(raw, ctrl.signal);
              if (ctrl.signal.aborted) return;
              if (calc.calculation === "ready" && calc.fileDiff) {
                inspectState = {
                  phase: "ready",
                  fileDiff: calc.fileDiff,
                  reason: "",
                };
                return;
              }
            }
          }

          // Fallback via staging/diff endpoint when >200 changed files
          const stDiff = await gitStagingDiff(
            repositoryId,
            worktreeId,
            {
              area: "staged",
              path: file.path,
              ...(file.oldPath ? { oldPath: file.oldPath } : {}),
            },
            ctrl.signal,
          );
          if (ctrl.signal.aborted) return;
          if (stDiff.hunks.length > 0) {
            const calc = await resolveHunksCalculatedDiff(
              `commit-preview:${file.id}:${stDiff.hunks.length}`,
              file.path,
              file.oldPath ?? "",
              file.status,
              stDiff.hunks,
              ctrl.signal,
            );
            if (!ctrl.signal.aborted && calc.fileDiff) {
              inspectState = {
                phase: "ready",
                fileDiff: calc.fileDiff,
                reason: "",
              };
              return;
            }
          }
        }

        if (file.hunks.length > 0) {
          const calc = await resolveHunksCalculatedDiff(
            `commit-preview-fixture:${file.id}:${file.hunks.length}`,
            file.path,
            file.oldPath ?? "",
            file.status,
            file.hunks,
            ctrl.signal,
          );
          if (!ctrl.signal.aborted && calc.fileDiff) {
            inspectState = {
              phase: "ready",
              fileDiff: calc.fileDiff,
              reason: "",
            };
            return;
          }
        }

        if (!ctrl.signal.aborted) {
          inspectState = {
            phase: "limited",
            fileDiff: null,
            reason:
              file.degradeReason || m.s6_commit_preview_empty_diff_reason(),
          };
        }
      } catch (err) {
        if (ctrl.signal.aborted) return;
        inspectState = {
          phase: "error",
          fileDiff: null,
          reason: errorMessage(err),
        };
      }
    })();

    return () => ctrl.abort();
  });

  const needsPushedAck = $derived(
    Boolean(model.amend && model.amendPushed && !effectiveAckPushedAmend),
  );
  const canExecute = $derived(
    !submitting &&
      !uncertainBlocked &&
      !model.stale &&
      model.blockers.length === 0 &&
      (model.stagedFiles.length > 0 || Boolean(model.amend)) &&
      !needsPushedAck &&
      commitMessage.trim().length > 0,
  );
  const effectivePreviewMessage = $derived.by(() => {
    const trimmed = commitMessage.trim();
    if (!model.signoff || !model.signoffTrailer) return commitMessage;
    if (trimmed.includes(model.signoffTrailer)) return commitMessage;
    return `${trimmed}\n\n${model.signoffTrailer}`;
  });

  function toggleInspectFile(id: string) {
    expandedFileId = expandedFileId === id ? null : id;
  }
</script>

<ResponsiveDialog
  title={model.followUpPush
    ? m.s6_commit_preview_title_push({ repo: model.repositoryName })
    : model.amend
      ? m.s6_commit_preview_title_amend({ repo: model.repositoryName })
      : m.s6_commit_preview_title_local({ repo: model.repositoryName })}
  description={model.amend
    ? m.s6_commit_preview_desc_amend()
    : m.s6_commit_preview_desc_local()}
  drawerDescription={m.s6_commit_preview_drawer_desc()}
  truncateDrawerTitle
  busy={submitting}
  pinDrawerHeader
  drawerFullScreen
  class="h-[min(90dvh,34rem)] sm:max-w-3xl"
  onclose={onClose}
>
  <div class="space-y-3">
    <GitCommitPreviewMetaCard
      {model}
      acknowledgePushedAmend={effectiveAckPushedAmend}
      {submitting}
      onAcknowledgePushedAmendChange={handleAckPushedAmend}
    />

    {#if model.headState === "detached"}
      <InlineNotice tone="warning">
        {m.s6_commit_preview_detached_notice()}
        {#snippet actions()}
          {#if onCreateBranchFromHead}
            <Button
              size="sm"
              variant="outline"
              wrap
              onclick={() => {
                onClose();
                onCreateBranchFromHead();
              }}
            >
              <IconGitBranch aria-hidden="true" />
              {m.s6_commit_preview_create_branch_btn()}
            </Button>
          {/if}
        {/snippet}
      </InlineNotice>
    {/if}

    {#if model.followUpPush}
      <InlineNotice tone="info">
        {m.s6_commit_preview_two_step_notice()}
      </InlineNotice>
    {/if}

    <GitIssueList blockers={model.blockers} warnings={model.warnings} />

    <div class="space-y-1.5">
      <div class="flex flex-wrap items-center justify-between gap-1">
        <span class="text-xs font-semibold text-foreground">
          {model.amend
            ? m.s6_commit_preview_entries_amend({
                count: model.stagedFiles.length,
              })
            : m.s6_commit_preview_entries_local({
                count: model.stagedFiles.length,
              })}
        </span>
        <span class="text-xs text-muted-foreground">
          {m.s6_commit_preview_excluded_counts({
            unstaged: model.unstagedCount,
            untracked: model.untrackedCount,
          })}
        </span>
      </div>

      <div class="rounded-lg border bg-background">
        <ScrollArea viewportClasses="max-h-44">
          <div class="space-y-0.5 p-1.5">
            {#if model.stagedFiles.length === 0}
              <p class="px-2 py-2 text-xs text-muted-foreground">
                {m.s6_commit_preview_no_staged()}
              </p>
            {:else}
              {#each model.stagedFiles as file (file.id)}
                {@const isExpanded = expandedFileId === file.id}
                <ListButton
                  variant="compact-row"
                  class="overflow-hidden"
                  aria-expanded={isExpanded}
                  aria-selected={isExpanded}
                  onclick={() => toggleInspectFile(file.id)}
                >
                  {#if isExpanded}
                    <IconChevronDown
                      class="size-3.5 shrink-0"
                      aria-hidden="true"
                    />
                  {:else}
                    <IconChevronRight
                      class="size-3.5 shrink-0"
                      aria-hidden="true"
                    />
                  {/if}
                  <Badge variant="info">{file.status}</Badge>
                  <span
                    class="min-w-0 flex-1 truncate font-mono"
                    title={file.path}
                  >
                    {file.oldPath && file.oldPath !== file.path
                      ? `${file.oldPath} → ${file.path}`
                      : file.path}
                  </span>
                  {#if file.preStagedFromCli}
                    <Badge variant="warning" class="max-w-full min-w-0 shrink">
                      <span class="truncate">
                        {m.s6_commit_preview_cli_prestaged()}
                      </span>
                    </Badge>
                  {/if}
                </ListButton>
              {/each}
            {/if}
          </div>
        </ScrollArea>
      </div>

      {#if expandedFile}
        <div
          class="code-panel flex h-52 min-w-0 flex-col overflow-hidden rounded-lg border bg-background"
          data-testid="git-commit-preview-diff"
        >
          {#if inspectState.phase === "loading"}
            <p class="p-3 text-xs text-muted-foreground">
              {m.s6_commit_preview_loading_diff({ path: expandedFile.path })}
            </p>
          {:else if inspectState.phase === "ready" && inspectState.fileDiff}
            <CodeDiffReader
              fileDiff={inspectState.fileDiff}
              side={expandedFile.status === "D" ||
              (inspectState.fileDiff.additionLines.length === 0 &&
                inspectState.fileDiff.deletionLines.length > 0)
                ? "old"
                : "new"}
              expired={false}
              draftEnabled={false}
              preferredDiffStyle="split"
              onselect={() => {}}
            />
          {:else}
            <div class="p-3">
              <InlineNotice
                tone={inspectState.phase === "error" ? "danger" : "info"}
              >
                {inspectState.reason}
              </InlineNotice>
            </div>
          {/if}
        </div>
      {/if}
    </div>

    <div class="rounded-lg border bg-background p-2.5">
      <span class="text-xs font-semibold text-muted-foreground">
        {m.s6_commit_preview_message_label()}
      </span>
      <ScrollArea viewportClasses="mt-1 max-h-24">
        <p
          class="font-mono text-xs whitespace-pre-wrap wrap-anywhere text-foreground"
          data-testid="git-commit-preview-message"
        >
          {effectivePreviewMessage}
        </p>
      </ScrollArea>
    </div>
  </div>
  {#snippet footer()}
    <div class="space-y-2">
      {#if model.stale || model.dialogError}
        <InlineNotice tone={model.stale ? "warning" : "danger"} size="compact">
          <ScrollArea
            class="min-w-0 flex-1"
            viewportClasses="max-h-[min(4rem,12dvh)] overscroll-contain"
            viewportProps={{ tabindex: 0 }}
          >
            <p class="wrap-anywhere pr-2">
              {model.dialogError || m.s6_commit_preview_stale_fallback()}
            </p>
          </ScrollArea>
          {#snippet actions()}
            {#if model.stale && onRefreshPreview}
              <Button
                size="sm"
                variant="default"
                wrap
                disabled={submitting}
                onclick={onRefreshPreview}
              >
                <IconRefresh aria-hidden="true" />
                {m.s6_commit_preview_regenerate_btn()}
              </Button>
            {/if}
          {/snippet}
        </InlineNotice>
      {/if}

      <GitDialogFooter
        {submitting}
        canConfirm={canExecute}
        wrap
        confirmLabel={model.followUpPush
          ? m.s6_commit_preview_confirm_push()
          : model.amend
            ? m.s6_commit_preview_confirm_amend()
            : m.s6_commit_preview_confirm_local()}
        submittingLabel={m.s6_commit_preview_submitting()}
        confirmTestId="git-commit-preview-confirm-btn"
        {onConfirm}
        {onClose}
      />
    </div>
  {/snippet}
</ResponsiveDialog>
