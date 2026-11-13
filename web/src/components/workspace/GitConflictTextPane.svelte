<script lang="ts">
  import IconCheck from "~icons/tabler/check";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { Textarea } from "$lib/components/ui/textarea";
  import type { GitConflictFileDetail } from "../../contracts/schemas";
  import type {
    GitBaseCompareMode,
    GitConflictChunkChoice,
    GitConflictDraftState,
    GitConflictProgressSummary,
  } from "../../domains/workspace/git-conflict-model";
  import GitConflictSourceColumn from "./GitConflictSourceColumn.svelte";

  let {
    detail,
    draft,
    progress,
    baseCompareMode = "off",
    activeConflictSegmentId = null,
    onChooseChunk,
    onToggleNonConflicting,
    onConfirmManualEdits,
    onChangeResultText,
  }: {
    detail: GitConflictFileDetail;
    draft: GitConflictDraftState;
    progress: GitConflictProgressSummary;
    baseCompareMode?: GitBaseCompareMode;
    activeConflictSegmentId?: string | null;
    onChooseChunk: (segmentId: string, choice: GitConflictChunkChoice) => void;
    onToggleNonConflicting: (segmentId: string) => void;
    onConfirmManualEdits: () => void;
    onChangeResultText: (nextText: string) => void;
  } = $props();
</script>

<div
  class="grid h-full min-h-0 min-w-0 grid-cols-3 divide-x divide-border overflow-hidden"
  data-testid="git-conflict-desktop-grid"
>
  <GitConflictSourceColumn
    side="left"
    info={detail.left}
    segments={detail.segments}
    {draft}
    showBase={baseCompareMode === "left_base"}
    {activeConflictSegmentId}
    {onChooseChunk}
    {onToggleNonConflicting}
  />

  <!-- Column 2: Center Editable Result (NEVER replaced by Base) -->
  <section
    class="flex h-full min-h-0 min-w-0 flex-col bg-background"
    aria-label="中间可编辑合并结果 Result"
    data-testid="git-conflict-center-result"
  >
    <div class="space-y-1 border-b bg-background px-2.5 py-2 text-xs">
      <div class="flex min-w-0 flex-wrap items-center justify-between gap-1.5">
        <div class="flex min-w-0 items-center gap-1.5">
          <Badge variant="success">合并结果 Result</Badge>
          <span class="min-w-0 truncate font-mono" title={detail.path}>
            {detail.path}
          </span>
        </div>

        <div class="flex flex-wrap items-center gap-1">
          {#if progress.hasConflictMarkers}
            <Badge variant="destructive">含冲突标记</Badge>
          {:else if progress.hasSavedToDisk}
            <Badge variant="success">已保存至工作树</Badge>
          {:else}
            <Badge variant="warning">草稿未保存</Badge>
          {/if}
          <Badge
            variant={progress.unresolvedConflicts === 0
              ? "secondary"
              : "outline"}
          >
            已决 {progress.totalConflicts -
              progress.unresolvedConflicts}/{progress.totalConflicts}
          </Badge>
        </div>
      </div>
      <p class="text-xs text-muted-foreground">
        可直接编辑合并文本；按序执行「保存结果」写入工作树后，再点击「标记已解决」写入暂存索引。
      </p>
    </div>

    {#if draft.manualEdited && progress.unresolvedConflicts > 0 && !progress.hasConflictMarkers}
      <div
        class="flex flex-wrap items-center justify-between gap-2 border-b bg-background px-2.5 py-1.5 text-xs"
      >
        <span class="text-muted-foreground">
          检测到你已直接编辑结果文本（剩余 {progress.unresolvedConflicts}
          处冲突块待确认）
        </span>
        <Button
          size="xs"
          variant="outline"
          onclick={onConfirmManualEdits}
          data-testid="git-conflict-confirm-manual-btn"
        >
          <IconCheck aria-hidden="true" />
          确认手动编辑已解决冲突
        </Button>
      </div>
    {/if}

    <div
      class="flex min-h-0 flex-1 flex-col overflow-y-auto p-2 focus-within:ring-2 focus-within:ring-ring/40"
    >
      <Textarea
        variant="bare"
        code
        aria-label="合并结果文本编辑器"
        value={draft.resultText}
        oninput={(e) => onChangeResultText(e.currentTarget.value)}
        class="min-h-full flex-1"
        data-testid="git-conflict-result-textarea"
      />
    </div>
  </section>

  <GitConflictSourceColumn
    side="right"
    info={detail.right}
    segments={detail.segments}
    {draft}
    showBase={baseCompareMode === "right_base"}
    {activeConflictSegmentId}
    {onChooseChunk}
    {onToggleNonConflicting}
  />
</div>
