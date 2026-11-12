<script lang="ts">
  import IconMessagePlus from "~icons/tabler/message-plus";
  import IconMinus from "~icons/tabler/minus";
  import IconPlus from "~icons/tabler/plus";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import type { GitWorkingChange } from "../../contracts/git-workbench-schemas";

  type GitChangeHunk = GitWorkingChange["hunks"][number];

  let {
    changeId,
    hunks,
    hunkIndex,
    stageMode,
    busy,
    stale,
    canAddRework,
    onJumpToHunk,
    onAddHunkToRework,
    onStageHunk,
    onUnstageHunk,
  }: {
    changeId: string;
    hunks: readonly GitChangeHunk[];
    hunkIndex: number;
    stageMode: "stage" | "unstage" | "none";
    busy: boolean;
    stale: boolean;
    canAddRework: boolean;
    onJumpToHunk: (index: number) => void;
    onAddHunkToRework?: (hunk: GitChangeHunk) => void;
    onStageHunk: (changeId: string, hunkId: string) => void;
    onUnstageHunk: (changeId: string, hunkId: string) => void;
  } = $props();
</script>

<div
  class="border-b border-git-border bg-git-panel px-2.5 py-1.5"
  aria-label="代码块逐块暂存栏"
  data-testid="git-hunk-action-strip"
>
  <div class="mb-1 flex items-center justify-between text-xs">
    <span class="font-semibold text-muted-foreground">
      代码块逐块操作（常驻可见 · 支持桌面与触屏）
    </span>
    <span class="font-mono text-muted-foreground">共 {hunks.length} 块</span>
  </div>
  <div class="max-h-28 space-y-1 overflow-y-auto">
    {#each hunks as hunk, idx (hunk.id)}
      <div
        class="flex min-w-0 flex-wrap items-center justify-between gap-1.5 rounded-sm border border-git-border bg-git-surface px-2 py-1"
        data-hunk-id={hunk.id}
      >
        <div class="min-w-0 flex-1">
          <ListButton
            variant="git-row"
            aria-selected={hunkIndex === idx}
            onclick={() => onJumpToHunk(idx)}
          >
            <span class="shrink-0 font-mono text-xs text-muted-foreground">
              #{idx + 1}
            </span>
            <span
              class="min-w-0 flex-1 truncate font-mono text-xs"
              title={hunk.header}
            >
              {hunk.header}
            </span>
          </ListButton>
        </div>
        <div class="flex shrink-0 flex-wrap items-center gap-1">
          {#if canAddRework && onAddHunkToRework}
            <Button
              size="xs"
              variant="ghost"
              onclick={() => onAddHunkToRework(hunk)}
              data-testid="git-hunk-add-rework"
            >
              <IconMessagePlus aria-hidden="true" />
              加入意见
            </Button>
          {/if}
          {#if stageMode === "stage"}
            <Button
              size="xs"
              variant="outline"
              disabled={busy || stale}
              onclick={() => onStageHunk(changeId, hunk.id)}
            >
              <IconPlus aria-hidden="true" />
              暂存此块
            </Button>
          {:else}
            <Button
              size="xs"
              variant="outline"
              disabled={busy || stale}
              onclick={() => onUnstageHunk(changeId, hunk.id)}
            >
              <IconMinus aria-hidden="true" />
              取消暂存此块
            </Button>
          {/if}
        </div>
      </div>
    {/each}
  </div>
</div>
