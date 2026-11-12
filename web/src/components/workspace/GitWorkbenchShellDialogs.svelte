<script lang="ts">
  import type { GitWorkbenchViewState } from "../../contracts/git-workbench-schemas";
  import type { GitReworkItem, SessionInfo } from "../../contracts/schemas";
  import type { GitSessionHandoffMode } from "../../hooks/workspace/git-rework-controller.svelte";
  import GitCommitPreviewDialog from "./GitCommitPreviewDialog.svelte";
  import GitPushDialog from "./GitPushDialog.svelte";
  import GitReworkDialog from "./GitReworkDialog.svelte";

  let {
    view,
    reworkOpen,
    handoffMode,
    reworkItems,
    activeSessions,
    selectedSessionId = $bindable(""),
    onConfirmCommit,
    onCloseCommitPreview,
    onConfirmPush,
    onClosePushPreview,
    onUpdateReworkNote,
    onRemoveReworkItem,
    onClearRework,
    onCopyReworkMarkdown,
    onDeliverToSession,
    onStartNewSessionWithDraft,
    onCloseRework,
  }: {
    view: GitWorkbenchViewState;
    reworkOpen: boolean;
    handoffMode: GitSessionHandoffMode;
    reworkItems: readonly GitReworkItem[];
    activeSessions: readonly SessionInfo[];
    selectedSessionId?: string;
    onConfirmCommit: () => void;
    onCloseCommitPreview: () => void;
    onConfirmPush: () => void;
    onClosePushPreview: () => void;
    onUpdateReworkNote: (id: string, note: string) => void;
    onRemoveReworkItem: (id: string) => void;
    onClearRework: () => void;
    onCopyReworkMarkdown: () => void;
    onDeliverToSession: (session: SessionInfo) => void;
    onStartNewSessionWithDraft: () => void;
    onCloseRework: () => void;
  } = $props();
</script>

{#if view.commitPreview}
  <GitCommitPreviewDialog
    model={view.commitPreview}
    commitMessage={view.commitMessage}
    onConfirm={onConfirmCommit}
    onClose={onCloseCommitPreview}
  />
{/if}

{#if view.pushPreview}
  <GitPushDialog
    model={view.pushPreview}
    onConfirm={onConfirmPush}
    onClose={onClosePushPreview}
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
    onUpdateNote={onUpdateReworkNote}
    onRemoveItem={onRemoveReworkItem}
    onClearAll={onClearRework}
    onCopyMarkdown={onCopyReworkMarkdown}
    onConfirmStale={() => {}}
    {onDeliverToSession}
    {onStartNewSessionWithDraft}
    onClose={onCloseRework}
  />
{/if}
