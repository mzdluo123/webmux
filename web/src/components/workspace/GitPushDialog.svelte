<script lang="ts">
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { ListButton } from "$lib/components/ui/list-button";
  import * as NativeSelect from "$lib/components/ui/native-select";
  import { ScrollArea } from "$lib/components/ui/scroll-area";
  import { m } from "$lib/paraglide/messages.js";
  import type {
    GitCommitDetail,
    GitIssue,
    GitPushPreviewModel,
  } from "../../contracts/git-workbench-schemas";
  import { formatPushRemoteTrackingRef } from "../../domains/workspace/git-divergence-model";
  import type { GitPushController } from "../../hooks/workspace/git-push-controller.svelte";
  import { InlineNotice } from "$lib/components/ui/inline-notice";
  import { ResponsiveDialog } from "$lib/components/ui/responsive-dialog";
  import GitDialogFooter from "./GitDialogFooter.svelte";
  import GitReviewBody from "./GitReviewBody.svelte";
  import GitIssueList from "./GitIssueList.svelte";
  import GitPushCommitInspector from "./GitPushCommitInspector.svelte";
  import GitPushRejectedBanner, {
    type GitPushRecoveryCallbacks,
  } from "./GitPushRejectedBanner.svelte";

  let {
    push,
    model,
    onConfirm,
    onClose,
    onFetchRemote,
    onFastForwardBranch,
    onMergeRemoteBranch,
    onRebaseOntoRemoteBranch,
  }: GitPushRecoveryCallbacks & {
    push?: GitPushController;
    model?: GitPushPreviewModel;
    onConfirm?: () => void;
    onClose?: () => void;
  } = $props();

  let fixtureCommitOid = $state<string | null>(null);
  let fixtureParentOid = $state("");
  let fixtureFilePath = $state<string | null>(null);

  const repositoryName = $derived(
    push ? push.repositoryName : (model?.repositoryName ?? ""),
  );
  const sourceBranch = $derived(
    push ? push.branch : (model?.sourceBranch ?? ""),
  );
  const remoteName = $derived(
    push
      ? push.publishing
        ? push.remote || push.preview?.remote || "origin"
        : (push.preview?.remote ?? "origin")
      : (model?.remote ?? "origin"),
  );
  const targetBranch = $derived(
    push
      ? push.publishing
        ? push.target || push.preview?.target || sourceBranch
        : (push.preview?.target ?? sourceBranch)
      : (model?.targetBranch ?? sourceBranch),
  );
  const publishMode = $derived(
    push
      ? Boolean(push.publishing || push.preview?.publish)
      : Boolean(model?.publish),
  );
  const pushUrl = $derived(
    push ? (push.preview?.pushUrl ?? "") : (model?.pushUrl ?? ""),
  );
  const precedingCommit = $derived(
    push ? push.precedingCommit : (model?.precedingCommit ?? null),
  );
  const remoteTrackingRef = $derived(
    formatPushRemoteTrackingRef(remoteName, targetBranch || sourceBranch),
  );

  const rejection = $derived(
    push?.rejected
      ? push.rejected.message || m.s7_push_remote_rejected()
      : (push?.conflictError ?? ""),
  );

  const commitRows = $derived.by(() => {
    if (push) {
      const items = push.preview?.commits.items ?? [];
      return items.map((c) => ({
        oid: c.hash,
        shortOid: c.hash.slice(0, 7),
        subject: c.subject,
        author: c.author,
      }));
    }
    const items = model?.commits ?? [];
    return items.map((c) => ({
      oid: c.oid,
      shortOid: c.shortOid,
      subject: c.subject,
      author: c.authorName,
    }));
  });

  const totalCommits = $derived(
    push
      ? (push.preview?.commits.total ?? commitRows.length)
      : (model?.totalCommits ?? commitRows.length),
  );

  const blockers = $derived<GitIssue[]>(
    push
      ? (push.preview?.blockers ?? []).map((b) => ({
          code: b.code,
          message: b.message,
        }))
      : (model?.blockers ?? []),
  );
  const warnings = $derived<GitIssue[]>(
    push
      ? (push.preview?.warnings ?? []).map((w) => ({
          code: w.code,
          message: w.message,
        }))
      : (model?.warnings ?? []),
  );

  const activeCommitOid = $derived(
    push
      ? (push.selectedCommitOid ?? commitRows[0]?.oid ?? null)
      : (fixtureCommitOid ?? commitRows[0]?.oid ?? null),
  );
  const activeDetail = $derived<GitCommitDetail | null>(
    push
      ? push.activeDetail
      : ((model?.commits.find((c) => c.oid === activeCommitOid) ??
          model?.commits[0] ??
          null) as GitCommitDetail | null),
  );
  const selectedParent = $derived(
    push ? push.selectedParentOid : fixtureParentOid,
  );
  const selectedFile = $derived(push ? push.selectedFilePath : fixtureFilePath);

  const canConfirm = $derived.by(() => {
    if (push) {
      const cur = push.preview;
      if (!cur || !cur.confirmationId || push.loading || push.running) {
        return false;
      }
      if (blockers.length > 0) return false;
      if (
        push.publishing &&
        (cur.remote !== push.remote || cur.target !== push.target.trim())
      ) {
        return false;
      }
      return true;
    }
    if (!model) return false;
    if (model.blockers.length > 0) return false;
    return model.publish || model.commits.length > 0;
  });

  function handleClose() {
    if (push) push.closeDialog();
    else onClose?.();
  }

  function handleConfirm() {
    if (push) void push.confirmPush();
    else onConfirm?.();
  }
</script>

<ResponsiveDialog
  title={m.s6_push_dialog_title()}
  description={m.s6_push_dialog_drawer_desc({ count: String(totalCommits) })}
  drawerFullScreen
  pinDrawerHeader
  busy={push?.running ?? false}
  class="sm:max-w-[min(900px,calc(100vw-2rem))]"
  onclose={handleClose}
>
  <GitReviewBody
    fullScreen
    testId="git-push-dialog"
    resetScrollOn={rejection || push?.loadError}
  >
    {#if push?.rejected || push?.conflictError}
      <GitPushRejectedBanner
        {precedingCommit}
        reason={rejection}
        {sourceBranch}
        {remoteTrackingRef}
        onCloseDialog={handleClose}
        {onFetchRemote}
        {onFastForwardBranch}
        {onMergeRemoteBranch}
        {onRebaseOntoRemoteBranch}
      />
    {:else if precedingCommit}
      <InlineNotice tone="success">
        {m.s6_push_step1_done_before()}<span class="font-mono"
          >{precedingCommit.shortOid}</span
        >{m.s6_push_step1_done_after({ subject: precedingCommit.subject })}
      </InlineNotice>
    {/if}

    {#if push?.loadError}
      <InlineNotice tone="danger">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span>{m.s6_push_load_error({ error: push.loadError })}</span>
          <Button
            size="sm"
            variant="outline"
            onclick={() => void push.refreshPreview()}
          >
            {m.s6_push_retry_btn()}
          </Button>
        </div>
      </InlineNotice>
    {/if}

    <div
      class="flex min-w-0 flex-col gap-2 rounded-lg border bg-background p-2.5 text-xs"
    >
      <div class="flex min-w-0 items-center gap-2">
        <span
          class="min-w-0 flex-1 truncate font-semibold text-foreground"
          title={repositoryName}>{repositoryName}</span
        >
        {#if publishMode}
          <Badge variant="warning" class="max-w-full">
            <span class="truncate">{m.s6_push_publish_badge()}</span>
          </Badge>
        {/if}
      </div>
      <div class="flex min-w-0 items-center gap-1.5">
        <Badge variant="info" class="min-w-0 shrink">
          <span class="truncate" title={sourceBranch}>{sourceBranch}</span>
        </Badge>
        <span class="shrink-0 text-muted-foreground">→</span>
        <Badge variant="outline" class="min-w-0 shrink">
          <span
            class="truncate"
            title={`${remoteName}/${targetBranch || sourceBranch}`}
          >
            {remoteName}/{targetBranch || sourceBranch}
          </span>
        </Badge>
      </div>

      {#if push && publishMode}
        <div class="grid min-w-0 grid-cols-1 gap-2 pt-1 sm:grid-cols-2">
          <div class="space-y-1">
            <span class="text-xs text-muted-foreground">
              {m.s6_push_publish_remote_label()}
            </span>
            {#if push.remotesLoading || push.remotes.length > 0}
              <NativeSelect.Root
                size="sm"
                disabled={push.running || push.remotesLoading}
                class="w-full"
                value={push.remote}
                aria-label={m.s6_push_select_remote_aria()}
                onchange={(e) => push.setPublishRemote(e.currentTarget.value)}
              >
                {#if push.remotesLoading}
                  <NativeSelect.Option value={push.remote}
                    >{push.remote}</NativeSelect.Option
                  >
                {/if}
                {#each push.remotes as r (r.name)}
                  <NativeSelect.Option value={r.name}>
                    {r.name} ({r.pushUrl})
                  </NativeSelect.Option>
                {/each}
              </NativeSelect.Root>
            {:else}
              <Input
                size="sm"
                mono
                value={push.remote}
                disabled={push.running}
                aria-label={m.s6_push_remote_name_aria()}
                oninput={(e) => push.setPublishRemote(e.currentTarget.value)}
              />
            {/if}
          </div>

          <div class="space-y-1">
            <span class="text-xs text-muted-foreground">
              {m.s6_push_target_branch_label()}
            </span>
            <Input
              size="sm"
              mono
              value={push.target}
              disabled={push.running}
              placeholder={sourceBranch}
              aria-label={m.s6_push_target_branch_aria()}
              oninput={(e) => push.setPublishTarget(e.currentTarget.value)}
            />
          </div>
        </div>
      {/if}
      <span
        class="min-h-4 truncate font-mono text-muted-foreground"
        title={pushUrl || undefined}>{pushUrl}</span
      >
    </div>

    <GitIssueList {blockers} {warnings} />

    <div
      class="grid min-w-0 grid-cols-1 gap-3 lg:grid-cols-12"
      data-slot="git-push-body"
    >
      <div
        class="min-w-0 space-y-2 rounded-lg border bg-background p-2 lg:col-span-5"
      >
        <div class="flex items-center justify-between px-1">
          <span class="text-xs font-semibold text-foreground">
            {m.s6_push_commits_heading({ count: String(totalCommits) })}
          </span>
          {#if push?.loading}
            <span class="text-xs text-muted-foreground">
              {m.s6_push_calculating()}
            </span>
          {/if}
        </div>

        {#if push?.loading && commitRows.length === 0}
          <p class="px-2 py-3 text-xs text-muted-foreground">
            {m.s6_push_loading_commits()}
          </p>
        {:else if commitRows.length === 0}
          <p class="px-2 py-3 text-xs text-muted-foreground">
            {#if publishMode}
              {m.s6_push_publish_shared_base()}
            {:else}
              {m.s6_push_nothing_ahead()}
            {/if}
          </p>
        {:else}
          <ScrollArea
            viewportClasses="max-h-64"
            data-testid="git-push-commits-list"
          >
            <div class="space-y-0.5">
              {#each commitRows as commit (commit.oid)}
                <ListButton
                  variant="compact-row"
                  aria-selected={activeCommitOid === commit.oid}
                  onclick={() => {
                    if (push) {
                      void push.selectCommit(commit.oid);
                    } else {
                      fixtureCommitOid = commit.oid;
                      fixtureParentOid = "";
                      fixtureFilePath = null;
                    }
                  }}
                >
                  <span
                    class="shrink-0 font-mono text-xs text-muted-foreground"
                  >
                    {commit.shortOid}
                  </span>
                  <span
                    class="min-w-0 flex-1 truncate font-medium"
                    title={commit.subject}
                  >
                    {commit.subject}
                  </span>
                </ListButton>
              {/each}
            </div>
          </ScrollArea>
        {/if}
      </div>

      <div class="min-w-0 lg:col-span-7">
        <GitPushCommitInspector
          detail={activeDetail}
          detailLoading={push?.detailLoading ?? false}
          selectedParentOverride={selectedParent}
          selectedFilePath={selectedFile}
          liveDiffState={push ? push.fileDiffState : null}
          onSelectParent={(pOid) => {
            if (push) push.selectParent(pOid);
            else fixtureParentOid = pOid;
          }}
          onSelectFile={(fPath) => {
            if (push) push.selectFile(fPath);
            else fixtureFilePath = fPath;
          }}
        />
      </div>
    </div>

    {#snippet footer()}
      <GitDialogFooter
        submitting={push?.running ?? false}
        {canConfirm}
        confirmLabel={publishMode
          ? m.s6_push_confirm_publish()
          : m.s6_push_confirm_push()}
        submittingLabel={m.s6_push_submitting()}
        onConfirm={handleConfirm}
        onClose={handleClose}
      />
    {/snippet}
  </GitReviewBody>
</ResponsiveDialog>
