<script lang="ts">
  import IconPlus from "~icons/tabler/plus";
  import IconTrash from "~icons/tabler/trash";
  import IconX from "~icons/tabler/x";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import type { GitWorkingChange } from "../../contracts/git-workbench-schemas";
  import GitPanelToolbar from "./GitPanelToolbar.svelte";

  let {
    selectedItems,
    busy = false,
    onStageSelected,
    onDiscardSelected,
    onClearSelection,
  }: {
    selectedItems: readonly GitWorkingChange[];
    busy?: boolean;
    onStageSelected: (items: readonly GitWorkingChange[]) => void;
    onDiscardSelected: (items: readonly GitWorkingChange[]) => void;
    onClearSelection: () => void;
  } = $props();
</script>

{#if selectedItems.length > 0}
  <div
    class="shrink-0 text-xs"
    role="region"
    aria-label="已选未暂存条目批量操作"
    data-testid="git-discard-batch-bar"
  >
    <GitPanelToolbar>
      <div class="flex min-w-0 items-center gap-1.5">
        <Badge variant="info">已选 {selectedItems.length} 项</Badge>
        <Button
          size="xs"
          variant="ghost"
          disabled={busy}
          onclick={onClearSelection}
        >
          <IconX aria-hidden="true" />
          取消选择
        </Button>
      </div>

      <div class="flex min-w-0 flex-wrap items-center gap-1">
        <Button
          size="xs"
          variant="outline"
          disabled={busy}
          onclick={() => {
            onStageSelected(selectedItems);
            onClearSelection();
          }}
          data-testid="git-batch-stage-btn"
        >
          <IconPlus aria-hidden="true" />
          暂存已选 ({selectedItems.length})
        </Button>
        <Button
          size="xs"
          variant="destructive-outline"
          disabled={busy}
          onclick={() => {
            onDiscardSelected(selectedItems);
            onClearSelection();
          }}
          data-testid="git-batch-discard-btn"
        >
          <IconTrash aria-hidden="true" />
          丢弃已选… ({selectedItems.length})
        </Button>
      </div>
    </GitPanelToolbar>
  </div>
{/if}
