<script lang="ts">
  import IconCheck from "~icons/tabler/check";
  import IconMinus from "~icons/tabler/minus";
  import IconPlus from "~icons/tabler/plus";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { ListButton } from "$lib/components/ui/list-button";
  import type { GitWorkingChange } from "../../contracts/git-workbench-schemas";

  let {
    item,
    isReviewed = false,
    isReading = false,
    readonlyMode = false,
    stagingBusy = false,
    onSelect,
    onToggleReviewed,
    onStageChange,
    onUnstageChange,
  }: {
    item: GitWorkingChange;
    isReviewed?: boolean;
    isReading?: boolean;
    readonlyMode?: boolean;
    stagingBusy?: boolean;
    onSelect: () => void;
    onToggleReviewed: (change: GitWorkingChange) => void;
    onStageChange: (changeId: string) => void;
    onUnstageChange: (changeId: string) => void;
  } = $props();
</script>

<div
  class="flex min-w-0 items-center gap-1 rounded-sm"
  data-change-id={item.id}
  data-reviewed={isReviewed}
  data-group={item.group}
>
  <Button
    size="icon-xs"
    variant="ghost"
    aria-pressed={isReviewed}
    aria-label={`标记已审阅 ${item.path}`}
    title={isReviewed ? `已审阅 ${item.path}` : `标记已审阅 ${item.path}`}
    onclick={() => onToggleReviewed(item)}
  >
    <IconCheck aria-hidden="true" />
  </Button>

  <div class="min-w-0 flex-1">
    <ListButton variant="git-row" aria-selected={isReading} onclick={onSelect}>
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
          class="min-w-[6ch] max-w-full flex-1 truncate font-mono"
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

  {#if !readonlyMode}
    {#if item.group === "staged"}
      <Button
        size="icon-xs"
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
        size="icon-xs"
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
