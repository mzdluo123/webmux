<script lang="ts">
  import IconMessagePlus from "~icons/tabler/message-plus";
  import IconX from "~icons/tabler/x";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import {
    type ActiveDiffReworkSelection,
    createReworkDraftItem,
    formatLineRangeLabel,
    toReworkItemInput,
  } from "../../domains/workspace/git-rework-model";

  let {
    selection,
    onAddReworkItem,
    onClear,
  }: {
    selection: ActiveDiffReworkSelection | null;
    onAddReworkItem?: (
      input: Parameters<typeof createReworkDraftItem>[0],
      options?: { openSheet?: boolean },
    ) => void;
    onClear: () => void;
  } = $props();

  let note = $state("");

  function submitAdd(openSheet: boolean) {
    if (!selection || !onAddReworkItem) return;
    const clean = note.trim();
    note = "";
    onAddReworkItem(toReworkItemInput(selection, clean), { openSheet });
    onClear();
  }
</script>

{#if selection && onAddReworkItem}
  <div
    class="flex min-w-0 flex-col gap-1.5 border-b border-git-border bg-git-panel px-2.5 py-2 text-xs"
    role="region"
    aria-label="选中代码行加入审阅意见"
    data-testid="git-diff-selection-bar"
  >
    <div class="flex min-w-0 flex-wrap items-center justify-between gap-1.5">
      <div class="flex min-w-0 flex-wrap items-center gap-1.5">
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
        variant="git-primary"
        onclick={() => submitAdd(false)}
        data-testid="git-selection-add-rework"
      >
        <IconMessagePlus aria-hidden="true" />
        加入审阅意见
      </Button>
    </div>
  </div>
{/if}
