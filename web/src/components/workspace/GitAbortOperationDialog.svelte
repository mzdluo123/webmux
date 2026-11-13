<script lang="ts">
  import { ItemGroup } from "$lib/components/ui/item";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
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
    ? `中止 ${gitSequencerKindLabel(abortPreview.kind)} 并回退工作树`
    : "中止进行中的 Git 操作"}
  description={abortPreview
    ? abortPreview.directionSummary
    : "正在读取回退目标与受影响文件列表…"}
  drawerDescription="确认回退目标 OID 与将被丢弃的冲突改动"
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
        正在读取操作前锚点（ORIG_HEAD）与未提交冲突文件…
      </p>
    {:else if abortPreview}
      <section
        class="min-w-0 rounded-lg border border-destructive/30 bg-background p-3"
      >
        <div class="mb-2 flex min-w-0 flex-wrap items-center gap-1.5">
          <Badge variant="destructive" class="max-w-full">
            <span class="truncate">
              中止 {gitSequencerKindLabel(abortPreview.kind)}
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
          工作树：{abortPreview.worktreePath}
        </p>
        {#if abortPreview.restoreOid}
          <p class="mt-1 text-xs wrap-anywhere text-foreground">
            回退锚点：<span class="font-mono"
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
            将被重置或丢弃冲突修改的路径 ({abortPreview.affectedPaths.length})
          </span>
          <ul class="max-h-44 space-y-1 overflow-y-auto font-mono">
            {#each abortPreview.affectedPaths as path (path)}
              <li class="break-all text-muted-foreground">{path}</li>
            {/each}
          </ul>
        </ItemGroup>
      {/if}

      <GitOperationSessionAck
        sessions={runningSessions}
        acknowledgeSessions={acknowledged}
        disabled={busy}
        acknowledgementLabel={`我已知晓：中止操作将使以上 ${runningSessions.length} 个会话的工作目录回退至操作前状态。`}
        onAcknowledgeSessionsChange={onAcknowledgeChange}
      />
    {/if}

    <footer class="flex flex-wrap justify-end gap-2 pt-1">
      <Button variant="outline" disabled={busy} onclick={onDismiss}>取消</Button
      >
      <Button
        variant="destructive"
        wrap
        disabled={!readyToAbort}
        onclick={onConfirmAbort}
        data-testid="git-abort-confirm-btn"
      >
        {busy ? "正在中止并回退…" : "确认中止并回退"}
      </Button>
    </footer>
  </div>
</ResponsiveDialog>
