<script lang="ts">
  import IconMessagePlus from "~icons/tabler/message-plus";
  import IconMinus from "~icons/tabler/minus";
  import IconPlus from "~icons/tabler/plus";
  import IconTrash from "~icons/tabler/trash";
  import IconX from "~icons/tabler/x";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import type { GitLineSelection } from "../../contracts/schemas";
  import { countSelectedLines } from "../../domains/workspace/git-line-selection";
  import {
    type ActiveDiffReworkSelection,
    createReworkDraftItem,
    formatLineRangeLabel,
    toReworkItemInput,
  } from "../../domains/workspace/git-rework-model";

  let {
    selection,
    stageMode = "none",
    lineSelections = [],
    busy = false,
    stale = false,
    onStageSelectedLines,
    onUnstageSelectedLines,
    onDiscardSelectedLines,
    onAddReworkItem,
    onClear,
  }: {
    selection: ActiveDiffReworkSelection | null;
    stageMode?: "stage" | "unstage" | "none";
    lineSelections?: readonly GitLineSelection[];
    busy?: boolean;
    stale?: boolean;
    onStageSelectedLines?: (selections: readonly GitLineSelection[]) => void;
    onUnstageSelectedLines?: (selections: readonly GitLineSelection[]) => void;
    onDiscardSelectedLines?: (selections: readonly GitLineSelection[]) => void;
    onAddReworkItem?: (
      input: Parameters<typeof createReworkDraftItem>[0],
      options?: { openSheet?: boolean },
    ) => void;
    onClear: () => void;
  } = $props();

  let note = $state("");
  const changedCount = $derived(countSelectedLines(lineSelections));
  const hasLineMutation = $derived(
    changedCount > 0 &&
      ((stageMode === "stage" &&
        Boolean(onStageSelectedLines || onDiscardSelectedLines)) ||
        (stageMode === "unstage" && Boolean(onUnstageSelectedLines))),
  );

  function submitAdd(openSheet: boolean) {
    if (!selection || !onAddReworkItem) return;
    const clean = note.trim();
    note = "";
    onAddReworkItem(toReworkItemInput(selection, clean), { openSheet });
    onClear();
  }
</script>

{#if (selection && onAddReworkItem) || hasLineMutation}
  <div
    class="flex min-w-0 flex-col gap-1.5 border-b bg-background px-2.5 py-2 text-xs"
    role="region"
    aria-label="选中代码行加入审阅意见"
    data-testid="git-diff-selection-bar"
  >
    <div class="flex min-w-0 flex-wrap items-center justify-between gap-1.5">
      <div class="flex min-w-0 flex-wrap items-center gap-1.5">
        {#if selection}
          <Badge variant="info" class="shrink-0">
            {selection.versionLabel}
          </Badge>
          <span class="min-w-0 truncate font-mono font-medium text-foreground">
            {selection.path}
          </span>
          <Badge variant="outline" class="shrink-0">
            <span class="font-mono">
              {formatLineRangeLabel(selection.startLine, selection.endLine)}
            </span>
          </Badge>
          {#if selection.sourceKind !== "worktree"}
            <span class="text-muted-foreground">
              （将明确标注版本身份，不伪装为实时工作区行号）
            </span>
          {/if}
        {/if}
        {#if hasLineMutation}
          <Badge variant="outline" class="shrink-0">
            含 {changedCount} 行可操作改动
          </Badge>
        {/if}
      </div>

      <Button
        size="icon-xs"
        variant="ghost"
        aria-label="取消行选择"
        title="取消行选择"
        onclick={() => {
          note = "";
          onClear();
        }}
      >
        <IconX aria-hidden="true" />
      </Button>
    </div>

    {#if hasLineMutation}
      <div class="flex min-w-0 flex-wrap items-center gap-1.5">
        {#if stageMode === "stage" && onStageSelectedLines}
          <Button
            size="xs"
            variant="outline"
            wrap
            disabled={busy || stale}
            onclick={() => {
              onStageSelectedLines(lineSelections);
              onClear();
            }}
            data-testid="git-selection-stage-lines"
          >
            <IconPlus aria-hidden="true" />
            暂存所选行 ({changedCount})
          </Button>
        {/if}
        {#if stageMode === "unstage" && onUnstageSelectedLines}
          <Button
            size="xs"
            variant="outline"
            wrap
            disabled={busy || stale}
            onclick={() => {
              onUnstageSelectedLines(lineSelections);
              onClear();
            }}
            data-testid="git-selection-unstage-lines"
          >
            <IconMinus aria-hidden="true" />
            取消暂存所选行 ({changedCount})
          </Button>
        {/if}
        {#if stageMode === "stage" && onDiscardSelectedLines}
          <Button
            size="xs"
            variant="destructive-outline"
            wrap
            disabled={busy || stale}
            onclick={() => {
              onDiscardSelectedLines(lineSelections);
              onClear();
            }}
            data-testid="git-selection-discard-lines"
          >
            <IconTrash aria-hidden="true" />
            丢弃所选行… ({changedCount})
          </Button>
        {/if}
      </div>
    {/if}

    {#if selection && onAddReworkItem}
      <div class="flex min-w-0 flex-wrap items-center gap-1.5">
        <div class="min-w-[10rem] flex-1">
          <Input
            size="sm"
            value={note}
            placeholder="输入针对所选代码行的修改意见（可留空后续在草稿箱补充）…"
            aria-label="审阅修改意见备注"
            oninput={(e) => (note = e.currentTarget.value)}
            onkeydown={(e) => {
              if (e.key === "Enter" && !e.isComposing) {
                e.preventDefault();
                submitAdd(false);
              }
            }}
          />
        </div>
        <Button
          size="xs"
          variant="outline"
          onclick={() => submitAdd(true)}
          data-testid="git-selection-add-open"
        >
          加入并查看草稿箱
        </Button>
        <Button
          size="xs"
          variant="default"
          onclick={() => submitAdd(false)}
          data-testid="git-selection-add-rework"
        >
          <IconMessagePlus aria-hidden="true" />
          加入审阅意见
        </Button>
      </div>
    {/if}
  </div>
{/if}
