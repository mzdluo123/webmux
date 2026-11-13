<script lang="ts">
  import type {
    GitCommitOutcome,
    GitPrecedingCommit,
    GitWorkbenchViewState,
  } from "../../contracts/git-workbench-schemas";
  import type { GitReworkItem, SessionInfo } from "../../contracts/schemas";
  import { applyShellCommitConfirm } from "../../domains/workspace/git-shell-actions";
  import type { GitSessionHandoffMode } from "../../hooks/workspace/git-rework-controller.svelte";
  import GitAbortOperationDialog from "./GitAbortOperationDialog.svelte";
  import GitCommitPreviewDialog from "./GitCommitPreviewDialog.svelte";
  import GitDivergencePreviewDialog from "./GitDivergencePreviewDialog.svelte";
  import GitPushDialog from "./GitPushDialog.svelte";
  import GitReworkDialog from "./GitReworkDialog.svelte";

  let {
    view = $bindable(),
    reworkOpen = $bindable(false),
    reworkItems = $bindable([]),
    handoffMode,
    activeSessions,
    simulatedCommitOutcome = null,
    selectedSessionId = $bindable(""),
    acknowledgeSessions = $bindable(false),
    onOpenFollowUpPush,
  }: {
    view: GitWorkbenchViewState;
    reworkOpen?: boolean;
    reworkItems?: GitReworkItem[];
    handoffMode: GitSessionHandoffMode;
    activeSessions: readonly SessionInfo[];
    simulatedCommitOutcome?: GitCommitOutcome | null;
    selectedSessionId?: string;
    acknowledgeSessions?: boolean;
    onOpenFollowUpPush: (branch: string, preceding: GitPrecedingCommit) => void;
  } = $props();

  function confirmCommit() {
    const res = applyShellCommitConfirm(view, simulatedCommitOutcome);
    if (!res) return;
    view = res.nextView;
    if (res.followUpPush) {
      onOpenFollowUpPush(res.followUpPush.branch, res.followUpPush.preceding);
    }
  }
</script>

{#if view.commitPreview}
  <GitCommitPreviewDialog
    model={view.commitPreview}
    commitMessage={view.commitMessage}
    onConfirm={confirmCommit}
    onClose={() => (view = { ...view, commitPreview: null })}
  />
{/if}

{#if view.pushPreview}
  <GitPushDialog
    model={view.pushPreview}
    onConfirm={() => {
      const push = view.pushPreview;
      if (!push) return;
      view = {
        ...view,
        pushPreview: null,
        statusBanner: `已完成推送确认：${push.sourceBranch} → ${push.remote}/${push.targetBranch}`,
      };
    }}
    onClose={() => {
      const hadPreceding = view.pushPreview?.precedingCommit;
      view = {
        ...view,
        pushPreview: null,
        statusBanner: hadPreceding
          ? `本地已提交（${hadPreceding.shortOid}），已取消推送；本地提交已保留。`
          : view.statusBanner,
      };
    }}
    onMergeRemoteBranch={(remoteRef) =>
      (view = {
        ...view,
        pushPreview: null,
        statusBanner: `已打开合并远端分支预览：${remoteRef}`,
      })}
    onRebaseOntoRemoteBranch={(remoteRef) =>
      (view = {
        ...view,
        pushPreview: null,
        statusBanner: `已打开变基到远端分支预览：${remoteRef}`,
      })}
  />
{/if}

{#if view.divergencePreview}
  <GitDivergencePreviewDialog
    kind={view.divergencePreview.kind}
    preview={view.divergencePreview}
    {acknowledgeSessions}
    onAcknowledgeSessionsChange={(checked) => (acknowledgeSessions = checked)}
    onConfirm={() =>
      (view = {
        ...view,
        divergencePreview: null,
        statusBanner: "已确认执行发散合并/变基/遴选/还原操作",
      })}
    onClose={() => (view = { ...view, divergencePreview: null })}
  />
{/if}

{#if view.abortPreview}
  <GitAbortOperationDialog
    abortPreview={view.abortPreview}
    acknowledged={acknowledgeSessions}
    onAcknowledgeChange={(checked) => (acknowledgeSessions = checked)}
    onConfirmAbort={() =>
      (view = {
        ...view,
        abortPreview: null,
        inProgress: null,
        conflictOpen: false,
        statusBanner: "已中止进行中的操作并回退到操作前提交",
      })}
    onDismiss={() => (view = { ...view, abortPreview: null })}
  />
{/if}

{#if reworkOpen}
  <GitReworkDialog
    mode={handoffMode}
    items={reworkItems}
    stagedChanges={view.workingChanges.filter((c) => c.group === "staged")}
    {activeSessions}
    bind:selectedSessionId
    hasCommitMessageDraft={Boolean(view.commitMessage.trim())}
    onUpdateNote={(id, note) =>
      (reworkItems = reworkItems.map((it) =>
        it.id === id ? { ...it, userNote: note } : it,
      ))}
    onRemoveItem={(id) =>
      (reworkItems = reworkItems.filter((it) => it.id !== id))}
    onClearAll={() => (reworkItems = [])}
    onCopyMarkdown={() =>
      (view = { ...view, statusBanner: "已复制结构化审阅意见 Markdown" })}
    onConfirmStale={() => {}}
    onDeliverToSession={(session) => {
      reworkOpen = false;
      if (handoffMode === "rework") reworkItems = [];
      view = {
        ...view,
        statusBanner: `已将意见放入会话「${session.title || session.id}」输入草稿（未自动执行）`,
      };
    }}
    onStartNewSessionWithDraft={() => {
      reworkOpen = false;
      view = {
        ...view,
        statusBanner: "已打开带预填草稿的新建工作树会话面板",
      };
    }}
    onClose={() => (reworkOpen = false)}
  />
{/if}
