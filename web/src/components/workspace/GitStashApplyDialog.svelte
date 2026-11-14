<script lang="ts">
  import { Badge } from "$lib/components/ui/badge";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { Label } from "$lib/components/ui/label";
  import * as SegmentedControl from "$lib/components/ui/segmented-control";
  import type {
    GitStashApplyPreview,
    GitStashEntry,
  } from "../../contracts/schemas";
  import {
    canConfirmPreviewModel,
    type StashPreviewDialogBaseProps,
  } from "../../domains/workspace/git-v2-adapter";
  import InlineNotice from "../shared/InlineNotice.svelte";
  import ResponsiveDialog from "../shared/ResponsiveDialog.svelte";
  import GitDialogFooter from "./GitDialogFooter.svelte";
  import GitIssueList from "./GitIssueList.svelte";
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
    ? "安全弹出贮藏 (Safe Pop)"
    : "应用贮藏保留条目 (Apply)"}
  description="先将贮藏改动应用到当前工作树；Pop 模式下仅当无任何冲突完全成功时才移除原贮藏条目。"
  drawerDescription="核对目标工作树与冲突安全策略后应用贮藏。"
  busy={submitting}
  class="sm:max-w-lg"
  onclose={onClose}
>
  <div class="space-y-3" data-testid="git-stash-apply-dialog">
    <SegmentedControl.Root
      wrap
      value={mode}
      onValueChange={(val) => {
        if (val === "apply" || val === "pop") onModeChange(val);
      }}
      aria-label="贮藏恢复方式"
    >
      <SegmentedControl.Item value="pop">
        弹出并清理 (Safe Pop)
      </SegmentedControl.Item>
      <SegmentedControl.Item value="apply">
        仅应用保留贮藏 (Apply)
      </SegmentedControl.Item>
    </SegmentedControl.Root>

    {#if entry}
      <div class="space-y-1 rounded-lg border bg-background p-3 text-xs">
        <p class="font-mono text-foreground">
          {entry.stashRef} · {entry.shortOid}
          {#if entry.hasUntracked}
            <Badge variant="outline" class="ml-1">含未跟踪文件</Badge>
          {/if}
        </p>
        <p class="font-medium wrap-anywhere text-foreground">{entry.summary}</p>
      </div>
    {/if}

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
          尝试恢复原始暂存状态 (<span class="font-mono">--index</span>)
        </span>
      </Label>
    </div>

    {#if mode === "pop"}
      <InlineNotice tone="info">
        安全两步法保护：系统先执行 <span class="font-mono">git stash apply</span
        >，仅当退出码为 0
        且索引中无任何未合并冲突条目时才丢弃该贮藏；一旦发生冲突，原贮藏条目将完整保留并直接进入三方冲突解决器。
      </InlineNotice>
    {/if}

    {#if loading && !preview}
      <p class="text-xs text-muted-foreground" role="status">
        正在检查目标工作树重叠路径与安全条件…
      </p>
    {:else if preview}
      {#if preview.overlappingPaths.length > 0}
        <InlineNotice tone="warning">
          当前工作树有 {preview.overlappingPaths.length} 个已修改路径与该贮藏重叠：{preview.overlappingPaths
            .slice(0, 4)
            .join(", ")}{preview.overlappingPaths.length > 4 ? " …" : ""}
        </InlineNotice>
      {/if}

      <GitIssueList blockers={preview.blockers} warnings={preview.warnings} />

      <GitOperationSessionAck
        sessions={preview.sessions}
        {acknowledgeSessions}
        disabled={submitting}
        acknowledgementLabel="我已知晓当前工作树有活跃会话，恢复贮藏将修改工作树文件"
        {onAcknowledgeSessionsChange}
      />
    {/if}

    <GitDialogFooter
      {dialogError}
      {submitting}
      {canConfirm}
      confirmLabel={mode === "pop" ? "确认安全弹出 (Pop)" : "确认应用 (Apply)"}
      submittingLabel={mode === "pop" ? "正在安全弹出…" : "正在应用…"}
      confirmTestId="git-stash-apply-confirm-btn"
      {onConfirm}
      {onClose}
    />
  </div>
</ResponsiveDialog>
