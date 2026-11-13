<script lang="ts">
  import IconArrowsSplit2 from "~icons/tabler/arrows-split-2";
  import IconCheck from "~icons/tabler/check";
  import IconHistory from "~icons/tabler/history";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
  import GitRowActionMenu from "./GitRowActionMenu.svelte";
  import IconMinus from "~icons/tabler/minus";
  import IconPlus from "~icons/tabler/plus";
  import IconTrash from "~icons/tabler/trash";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { Checkbox } from "$lib/components/ui/checkbox";
  import { ListButton } from "$lib/components/ui/list-button";
  import type { GitWorkingChange } from "../../contracts/git-workbench-schemas";
  import type { GitFileInspectorTab } from "../../hooks/workspace/git-file-inspector-controller.svelte";

  let {
    item,
    isReviewed = false,
    isReading = false,
    isSelectedForBatch = false,
    batchSelectable = false,
    readonlyMode = false,
    stagingBusy = false,
    onSelect,
    onToggleBatchSelect,
    onToggleReviewed,
    onStageChange,
    onUnstageChange,
    onDiscardChange,
    onOpenConflictResolver,
    onInspectFile,
  }: {
    item: GitWorkingChange;
    isReviewed?: boolean;
    isReading?: boolean;
    isSelectedForBatch?: boolean;
    batchSelectable?: boolean;
    readonlyMode?: boolean;
    stagingBusy?: boolean;
    onSelect: () => void;
    onToggleBatchSelect?: (change: GitWorkingChange) => void;
    onToggleReviewed: (change: GitWorkingChange) => void;
    onStageChange: (changeId: string) => void;
    onUnstageChange: (changeId: string) => void;
    onDiscardChange?: (change: GitWorkingChange) => void;
    onOpenConflictResolver?: (path: string) => void;
    onInspectFile?: (path: string, tab: GitFileInspectorTab) => void;
  } = $props();
</script>

<div
  class="group/row relative flex min-w-0 items-center gap-1 rounded-sm pr-7 max-sm:pr-(--control-touch-compact)"
  data-change-id={item.id}
  data-reviewed={isReviewed}
  data-group={item.group}
  data-testid={`git-change-row-${item.id}`}
>
  <div class="flex min-w-[6rem] flex-1 items-center gap-1">
    {#if batchSelectable && !readonlyMode && onToggleBatchSelect}
      <Checkbox
        checked={isSelectedForBatch}
        disabled={stagingBusy}
        aria-label={`选择未暂存条目 ${item.path}`}
        onCheckedChange={() => onToggleBatchSelect(item)}
        class="shrink-0"
      />
    {/if}

    <Button
      size="icon-sm"
      variant="ghost"
      aria-pressed={isReviewed}
      aria-label={`标记已审阅 ${item.path}`}
      title={isReviewed ? `已审阅 ${item.path}` : `标记已审阅 ${item.path}`}
      onclick={() => onToggleReviewed(item)}
    >
      <IconCheck aria-hidden="true" />
    </Button>

    <div class="min-w-0 flex-1">
      <ListButton
        variant="compact-row"
        role="option"
        data-working-change-row="true"
        aria-selected={isReading}
        onclick={onSelect}
      >
        <div class="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
          <Badge
            variant={item.group === "conflicted"
              ? "destructive"
              : item.group === "staged"
                ? "info"
                : "outline"}
          >
            {item.status}
          </Badge>
          <span
            class="min-w-[4ch] max-w-full flex-1 truncate font-mono"
            title={item.oldPath && item.oldPath !== item.path
              ? `${item.oldPath} → ${item.path}`
              : item.path}
          >
            {item.oldPath && item.oldPath !== item.path
              ? `${item.oldPath} → ${item.path}`
              : item.path}
          </span>
          {#if isReviewed}
            <Badge variant="secondary" class="max-w-full">
              <span class="truncate">已审阅</span>
            </Badge>
          {/if}
          {#if item.preStagedFromCli}
            <Badge variant="warning" class="max-w-full">
              <span class="truncate">CLI 预暂存</span>
            </Badge>
          {/if}
        </div>
      </ListButton>
    </div>
  </div>

  <div class="ml-auto flex shrink-0 items-center gap-0.5">
    {#if !readonlyMode}
      {#if item.group === "staged"}
        <Button
          size="icon-sm"
          variant="ghost"
          disabled={stagingBusy}
          aria-label={`取消暂存文件 ${item.path}`}
          title={`取消暂存文件 ${item.path}`}
          onclick={() => onUnstageChange(item.id)}
        >
          <IconMinus aria-hidden="true" />
        </Button>
      {:else if item.group === "unstaged" || item.group === "untracked"}
        <Button
          size="icon-sm"
          variant="ghost"
          disabled={stagingBusy}
          aria-label={`暂存文件 ${item.path}`}
          title={`暂存文件 ${item.path}`}
          onclick={() => onStageChange(item.id)}
        >
          <IconPlus aria-hidden="true" />
        </Button>
      {/if}
    {/if}
  </div>

  <GitRowActionMenu label={`文件操作 ${item.path}`}>
    {#if item.group !== "untracked" && onInspectFile}
      <DropdownMenu.Item onclick={() => onInspectFile(item.path, "history")}>
        <IconHistory aria-hidden="true" />文件历史与 Blame
      </DropdownMenu.Item>
    {/if}
    {#if !readonlyMode && item.group === "conflicted" && onOpenConflictResolver}
      <DropdownMenu.Item onclick={() => onOpenConflictResolver(item.path)}>
        <IconArrowsSplit2 aria-hidden="true" />打开三方冲突解决器
      </DropdownMenu.Item>
    {/if}
    {#if !readonlyMode && (item.group === "unstaged" || item.group === "untracked") && onDiscardChange}
      <DropdownMenu.Item
        disabled={stagingBusy}
        onclick={() => onDiscardChange(item)}
      >
        <IconTrash aria-hidden="true" />丢弃更改
      </DropdownMenu.Item>
    {/if}
  </GitRowActionMenu>
</div>
