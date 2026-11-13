<script lang="ts">
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { ListButton } from "$lib/components/ui/list-button";
  import * as NativeSelect from "$lib/components/ui/native-select";
  import type {
    GitCommitDetail,
    GitIssue,
    GitPushPreviewModel,
  } from "../../contracts/git-workbench-schemas";
  import { formatPushRemoteTrackingRef } from "../../domains/workspace/git-divergence-model";
  import type { GitPushController } from "../../hooks/workspace/git-push-controller.svelte";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";
  import GitDialogFooter from "./GitDialogFooter.svelte";
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
  title={`推送确认 · ${sourceBranch} → ${remoteName}/${targetBranch || sourceBranch}`}
  description={pushUrl
    ? `仓库 ${repositoryName} · 目标远端 ${pushUrl} · 共 ${totalCommits} 个待推提交`
    : `仓库 ${repositoryName} · 核对待推提交与文件差异后确认推送`}
  drawerDescription={`共 ${totalCommits} 个待推提交 · 核对差异后确认推送`}
  drawerFullScreen
  truncateDrawerTitle
  busy={push?.running ?? false}
  class="sm:max-w-[min(900px,calc(100vw-2rem))]"
  onclose={handleClose}
>
  <div class="space-y-3" data-testid="git-push-dialog">
    {#if push?.rejected || push?.conflictError}
      <GitPushRejectedBanner
        {precedingCommit}
        reason={push?.rejected?.message || push?.conflictError || ""}
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
        第一步已完成：本地提交 <span class="font-mono"
          >{precedingCommit.shortOid}</span
        >（{precedingCommit.subject}）已创建。请核对本分支全部待推提交后确认推送。
      </InlineNotice>
    {/if}

    {#if push?.loadError}
      <InlineNotice tone="danger">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span>加载推送预览失败：{push.loadError}</span>
          <Button
            size="sm"
            variant="outline"
            onclick={() => void push.refreshPreview()}
          >
            重试加载
          </Button>
        </div>
      </InlineNotice>
    {/if}

    <div
      class="flex min-w-0 flex-col gap-2 rounded-lg border bg-background p-2.5 text-xs"
    >
      <div class="flex min-w-0 flex-wrap items-center justify-between gap-2">
        <div class="flex min-w-0 max-w-full flex-wrap items-center gap-1.5">
          <span class="font-semibold text-foreground">{repositoryName}</span>
          <Badge variant="info" class="max-w-full">
            <span class="truncate" title={sourceBranch}>{sourceBranch}</span>
          </Badge>
          <span class="text-muted-foreground">→</span>
          <Badge variant="outline" class="max-w-full">
            <span
              class="truncate"
              title={`${remoteName}/${targetBranch || sourceBranch}`}
            >
              {remoteName}/{targetBranch || sourceBranch}
            </span>
          </Badge>
          {#if publishMode}
            <Badge variant="warning" class="max-w-full">
              <span class="truncate">首次发布上游 (Publish)</span>
            </Badge>
          {/if}
        </div>

        {#if pushUrl}
          <span class="font-mono break-all text-muted-foreground">
            {pushUrl}
          </span>
        {/if}
      </div>

      {#if push && publishMode}
        <div class="grid min-w-0 grid-cols-1 gap-2 pt-1 sm:grid-cols-2">
          <div class="space-y-1">
            <span class="text-xs text-muted-foreground">发布远端 (Remote)</span>
            {#if push.remotes.length > 0}
              <NativeSelect.Root
                size="sm"
                class="w-full"
                value={push.remote}
                aria-label="选择发布远端"
                onchange={(e) => push.setPublishRemote(e.currentTarget.value)}
              >
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
                aria-label="远端名称"
                oninput={(e) => push.setPublishRemote(e.currentTarget.value)}
              />
            {/if}
          </div>

          <div class="space-y-1">
            <span class="text-xs text-muted-foreground">
              远端分支名 (Target Branch)
            </span>
            <Input
              size="sm"
              mono
              value={push.target}
              placeholder={sourceBranch}
              aria-label="远端目标分支名"
              oninput={(e) => push.setPublishTarget(e.currentTarget.value)}
            />
          </div>
        </div>
      {/if}
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
            待推送提交 ({totalCommits})
          </span>
          {#if push?.loading}
            <span class="text-xs text-muted-foreground">正在计算…</span>
          {/if}
        </div>

        {#if push?.loading && commitRows.length === 0}
          <p class="px-2 py-3 text-xs text-muted-foreground">
            正在读取待推送提交列表…
          </p>
        {:else if commitRows.length === 0}
          <p class="px-2 py-3 text-xs text-muted-foreground">
            {#if publishMode}
              当前分支与远端共享提交基底，首次发布将在远端创建分支引用并配置上游跟踪。
            {:else}
              当前分支没有领先远端上游的待推提交。
            {/if}
          </p>
        {:else}
          <div
            class="max-h-64 space-y-0.5 overflow-y-auto"
            data-testid="git-push-commits-list"
          >
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
                <span class="shrink-0 font-mono text-xs text-muted-foreground">
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

    <GitDialogFooter
      submitting={push?.running ?? false}
      {canConfirm}
      confirmLabel={publishMode ? "发布并推送" : "确认推送"}
      submittingLabel="正在推送…"
      onConfirm={handleConfirm}
      onClose={handleClose}
    />
  </div>
</ResponsiveDialog>
