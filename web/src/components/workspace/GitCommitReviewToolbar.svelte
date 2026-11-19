<script lang="ts">
  import IconPlayerTrackNext from "~icons/tabler/player-track-next";
  import IconRestore from "~icons/tabler/restore";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import {
    SegmentedControl,
    SegmentedControlItem,
  } from "$lib/components/ui/segmented-control";
  import { m } from "$lib/paraglide/messages.js";
  import type { GitReviewScope } from "../../contracts/git-workbench-schemas";
  import type { GitReviewProgressSummary } from "../../hooks/workspace/git-review-sync.svelte";
  import GitPanelToolbar from "./GitPanelToolbar.svelte";

  let {
    scope,
    workingChangeCount,
    taskFileCount,
    progress,
    onScopeChange,
    onNextUnreviewed,
    onResetLayout,
  }: {
    scope: GitReviewScope;
    workingChangeCount: number;
    taskFileCount?: number;
    progress?: GitReviewProgressSummary;
    onScopeChange: (scope: GitReviewScope) => void;
    onNextUnreviewed: () => void;
    onResetLayout?: () => void;
  } = $props();
</script>

<GitPanelToolbar>
  <SegmentedControl
    wrap
    value={scope}
    onValueChange={(value) => {
      if (value === "uncommitted" || value === "task") onScopeChange(value);
    }}
    aria-label={m.s6_review_scope_aria()}
  >
    <SegmentedControlItem value="uncommitted">
      {m.s6_uncommitted_changes_tab({ count: workingChangeCount })}
    </SegmentedControlItem>
    <SegmentedControlItem
      value="task"
      aria-label={taskFileCount !== undefined
        ? m.s6_task_changes_with_count({ count: taskFileCount })
        : m.s6_task_changes_label()}
      title={m.s6_task_changes_label()}
    >
      <span class="sm:hidden">{m.s6_task_changes_short()}</span>
      <span class="max-sm:hidden">{m.s6_task_changes_label()}</span>
      {#if taskFileCount !== undefined}
        <span>({taskFileCount})</span>
      {/if}
    </SegmentedControlItem>
  </SegmentedControl>

  <div class="flex min-w-0 flex-wrap items-center gap-1.5">
    {#if progress && progress.total > 0}
      <Badge
        variant={progress.remaining === 0 ? "secondary" : "outline"}
        data-testid="git-review-progress-badge"
      >
        {m.s6_reviewed_progress_badge({
          reviewed: progress.reviewed,
          total: progress.total,
        })}
      </Badge>
      <Button
        size="sm"
        variant="outline"
        disabled={progress.remaining === 0}
        onclick={onNextUnreviewed}
        aria-label={m.s6_next_unreviewed_label()}
        title={m.s6_next_unreviewed_tooltip()}
        data-testid="git-next-unreviewed"
      >
        <IconPlayerTrackNext aria-hidden="true" />
        <span class="sm:hidden">{m.s6_next_unreviewed_short()}</span>
        <span class="max-sm:hidden">{m.s6_next_unreviewed_label()}</span>
      </Button>
    {/if}

    {#if onResetLayout}
      <Button
        size="icon-sm"
        variant="ghost"
        aria-label={m.s6_reset_commit_pane_layout()}
        title={m.s6_reset_commit_pane_layout()}
        onclick={onResetLayout}
      >
        <IconRestore aria-hidden="true" />
      </Button>
    {/if}
  </div>
</GitPanelToolbar>
